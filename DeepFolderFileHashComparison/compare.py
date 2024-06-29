import argparse
import datetime
import os
import sqlite3
import hashlib

STATUS_TABLE_NAME = "status"
STATUS_TABLE_VERSION_FIELD = "app_version"
STATUS_TABLE_SOURCE_PATH_FIELD = "root_source_path"
STATUS_TABLE_TARGET_PATH_FIELD = "root_target_path"
STATUS_TABLE_FOLLOW_SYMLINKS_FIELD = "follow_symlinks"
STATUS_TABLE_COMPLETE_FIELD = "complete"
STATUS_TABLE_STARTED_FIELD = "started"
STATUS_TABLE_ID_FIELD = "id"

SOURCE_FILES_TABLE_NAME = "source_files"
TARGET_FILES_TABLE_NAME = "target_files"
FILES_TABLES_ID_FIELD = "id"
FILES_TABLES_HASH_FIELD = "hash"
FILES_TABLES_PATH_FIELD = "path"

TIMESTAMP_TIME_FORMAT = "%Y-%m-%d %H:%M:%S"

APP_VERSION_MAJOR = 1
APP_VERSION_MINOR = 0
APP_VERSION_PATCH = 0

SHA_256_SUM_EMPTY_FILE_HASH = b"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"


def check_db_status(client):
    is_compatible_version_or_irrelevant, previous_exists, completed, timestamp = False, False, False, None
    root_source_path, root_target_path, follow_symlinks = None, None, False
    cursor = client.execute(f"SELECT {STATUS_TABLE_VERSION_FIELD} FROM {STATUS_TABLE_NAME} LIMIT 1;")
    results = cursor.fetchall()
    previous_exists = len(results) > 0

    if previous_exists:
        is_compatible_version_or_irrelevant = results[0][0] == APP_VERSION_MAJOR

        results = client.execute(f"SELECT TOP 1 {STATUS_TABLE_COMPLETE_FIELD}, {STATUS_TABLE_STARTED_FIELD}, "
                                 f"{STATUS_TABLE_SOURCE_PATH_FIELD}, {STATUS_TABLE_TARGET_PATH_FIELD}, "
                                 f"{STATUS_TABLE_FOLLOW_SYMLINKS_FIELD} FROM {STATUS_TABLE_NAME};")
        if len(results) > 0:
            completed, timestamp, root_source_path, root_target_path, follow_symlinks = results[0]

    return is_compatible_version_or_irrelevant, previous_exists, completed, timestamp, \
        root_source_path, root_target_path, follow_symlinks


def setup_db_tables_non_destructively(connection, client):
    client.execute(
        f"CREATE TABLE IF NOT EXISTS {SOURCE_FILES_TABLE_NAME} "
        f"({FILES_TABLES_ID_FIELD} INTEGER PRIMARY KEY AUTOINCREMENT, "
        f"{FILES_TABLES_HASH_FIELD} BLOB, "
        f"{FILES_TABLES_PATH_FIELD} VARCHAR);"
    )

    client.execute(
        f"CREATE TABLE IF NOT EXISTS {TARGET_FILES_TABLE_NAME} "
        f"({FILES_TABLES_ID_FIELD} INTEGER PRIMARY KEY AUTOINCREMENT, "
        f"{FILES_TABLES_HASH_FIELD} BLOB, "
        f"{FILES_TABLES_PATH_FIELD} VARCHAR);"
    )

    client.execute(
        f"CREATE TABLE IF NOT EXISTS {STATUS_TABLE_NAME} "
        f"({STATUS_TABLE_ID_FIELD} INTEGER PRIMARY KEY AUTOINCREMENT, {STATUS_TABLE_COMPLETE_FIELD} BOOL, "
        f"{STATUS_TABLE_STARTED_FIELD} TIMESTAMP, {STATUS_TABLE_SOURCE_PATH_FIELD} VARCHAR, "
        f"{STATUS_TABLE_TARGET_PATH_FIELD} VARCHAR, {STATUS_TABLE_FOLLOW_SYMLINKS_FIELD} BOOL, "
        f"{STATUS_TABLE_VERSION_FIELD} VARCHAR);"
    )

    connection.commit()


def totally_reset_db(connection, client):
    client.execute(f"DROP TABLE {SOURCE_FILES_TABLE_NAME};")

    client.execute(f"DROP TABLE {TARGET_FILES_TABLE_NAME};")

    client.execute(f"DROP TABLE {STATUS_TABLE_NAME};")

    setup_db_tables_non_destructively(connection, client)

    connection.commit()


def walk_directories_and_subdirectories(
        directory,
        db_client,
        db_connection,
        checked_directories,
        table_name,
        follow_symlinks
):
    if os.path.join(directory) in checked_directories:
        return checked_directories

    for current_root_path, directory_names, file_names in os.walk(directory, followlinks=follow_symlinks):
        checked_directories.add(current_root_path)

        for name in file_names:
            hasher = hashlib.new('sha256')
            file_path = os.path.join(current_root_path, name)
            with open(file_path, 'rb') as f:
                # buffer the data in case it's a huge file
                while True:
                    file_data = f.read(1048576)  # 1 MB read
                    if not file_data:
                        break
                    hasher.update(file_data)
                print(f"Inserting ({hasher.digest()}, '{file_path}')")
                insert_result = db_client.execute(
                    f"INSERT INTO {table_name} ({FILES_TABLES_HASH_FIELD}, {FILES_TABLES_PATH_FIELD}) "
                    f"VALUES (zeroblob(32), ?)",
                    (file_path,)
                )
                with db_connection.blobopen(table_name, FILES_TABLES_HASH_FIELD, insert_result.lastrowid) as blob:
                    blob.write(hasher.digest())
        db_connection.commit()

        checked_directories_to_remove = checked_directories.intersection(
            {os.path.join(current_root_path, p) for p in directory_names}
        )
        for item in checked_directories_to_remove:
            # remove directories from the walk
            head, tail = os.path.split(item)
            directory_names.remove(tail)

    return checked_directories


def get_files_not_present_in_target_but_present_in_source(db_client):
    """
    Note: this excludes empty files
    :param db_client:
    :return:
    """
    data = []

    for item in db_client.execute(f"SELECT * FROM {SOURCE_FILES_TABLE_NAME} "
                                  f"NOT IN (SELECT {FILES_TABLES_HASH_FIELD} FROM {TARGET_FILES_TABLE_NAME})"):
                                  # f"WHERE {FILES_TABLES_HASH_FIELD} = {SHA_256_SUM_EMPTY_FILE_HASH} "
                                  # f"AND {FILES_TABLES_HASH_FIELD} "
                                  # f"NOT IN SELECT {FILES_TABLES_HASH_FIELD} FROM {TARGET_FILES_TABLE_NAME}"):
        data.append(item)

    return data


def get_empty_target_files(db_client):
    """
    :param db_client:
    :return:
    """
    data = []

    for item in db_client.execute(f"SELECT * FROM {TARGET_FILES_TABLE_NAME}"
                                  f" WHERE {FILES_TABLES_HASH_FIELD} = {SHA_256_SUM_EMPTY_FILE_HASH} "):
        data.append(item)

    return data


def get_empty_source_files(db_client):
    """
    :param db_client:
    :return:
    """
    data = []

    for item in db_client.execute(f"SELECT * FROM {SOURCE_FILES_TABLE_NAME}"
                                  f" WHERE {FILES_TABLES_HASH_FIELD} = {SHA_256_SUM_EMPTY_FILE_HASH} "):
        data.append(item)

    return data


def split_string_or_return_empty_list(string_to_split):
    try:
        return string_to_split.split(",")
    except AttributeError:
        return []


def verify_files_folders_are_reachable(path):
    if path is None:
        return False, "Path is None."
    elif os.path.isdir(path):
        return True, None
    elif not os.path.exists(path):
        return False, f"{path} does not exist on the current file system."
    else:
        return False, f"{path} is not a directory."

# todo, need to add warnings for empty files will not appear in list

# todo, need to multithread work on hashing
# todo include option to ignore symlinks
# todo, need to to track completions in another table...check versions as as part of open to make sure all is okay

# todo, should I let people checkpoint the DB and force a resume if they know files haven't changed? I think so, just with a warning.

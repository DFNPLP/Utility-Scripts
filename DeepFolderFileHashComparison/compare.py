import argparse
import datetime
import os
import sqlite3
import hashlib

DEFAULT_DB_FILE_NAME = "data_cache"


def main():
    pass


def walk_directories_and_subdirectories(directory, db_client, db_connection, checked_directories, table_name):
    if os.path.join(directory) in checked_directories:
        return checked_directories

    for current_root_path, directory_names, file_names in os.walk(directory, followlinks=True):
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
                    f"INSERT INTO {table_name} (hash, path) VALUES (zeroblob(32), ?)",
                    (file_path,)
                )
                with db_connection.blobopen(table_name, "hash", insert_result.lastrowid) as blob:
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


def split_string_or_return_empty_list(string_to_split):
    try:
        return string_to_split.split(",")
    except AttributeError:
        return []


def verify_files_folders_are_reachable(paths_list):
    valid_directory_paths, invalid_directory_paths = [], {}
    for item in paths_list:
        if os.path.isdir(item):
            valid_directory_paths.append(item)
        else:
            current_error = invalid_directory_paths.get(item, "")
            if os.path.exists(item):
                current_error += f"{item} does not exist on the current file system."
            else:
                current_error += f"{item} is not a directory."
            invalid_directory_paths[item] = current_error

    return valid_directory_paths, invalid_directory_paths


def request_paths(reason):
    new_paths = input(f"Please provide a single path for {reason}. ")
    new_paths_list = split_string_or_return_empty_list(new_paths)

    while not new_paths_list:
        new_paths = input(f"Sorry, that input wasn't understood. Please provide a single path for {reason}. ")
        new_paths_list = split_string_or_return_empty_list(new_paths)

    return new_paths_list


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        prog="DeepFolderFileHashComparison",
        description="Allows you to index files by hash in on-disk/in-memory database and "
                    "compare that to another folder."
    )

    parser.add_argument(
        '-d',
        '--db_name',
        action='store',
        dest="db_name",
        default=DEFAULT_DB_FILE_NAME
    )

    parser.add_argument(
        '-s',
        '--source',
        action='store',
        dest="source",
        default=None
    )

    parser.add_argument(
        '-t',
        '--target',
        action='store',
        dest="target",
        default=None
    )

    args = parser.parse_args()

    db_name = args.db_name if args.db_name else DEFAULT_DB_FILE_NAME
    db_connection = sqlite3.connect(f"{DEFAULT_DB_FILE_NAME}.db")
    client = db_connection.cursor()

    client.execute(
        "CREATE TABLE IF NOT EXISTS source_files "
        "(id INTEGER PRIMARY KEY AUTOINCREMENT, hash BLOB, path VARCHAR);"
    )

    client.execute(
        "CREATE TABLE IF NOT EXISTS target_files "
        "(id INTEGER PRIMARY KEY AUTOINCREMENT, hash BLOB, path VARCHAR);"
    )

    # client.execute(f"INSERT INTO files VALUES (0, 'c:/fake_file', '{datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S')}')")
    # um = client.execute(f"SELECT * FROM files")
    # for item in um:
    #     print(item)
    #
    # client.execute(
    #     f"DROP TABLE files;")

    for table_name, incoming_path, reason in \
            [("source_files", args.source, "source file checks"), ("target_files", args.target, "target file checks")]:
        paths_list = split_string_or_return_empty_list(incoming_path)
        if not paths_list or len(paths_list) != 1:
            paths_list = request_paths(reason)
        valid_paths, errors = verify_files_folders_are_reachable(paths_list)

        while errors or not valid_paths or len(valid_paths) != 1:
            print(f"There are either errors present or a number of paths which is not 1. Errors: {errors}")
            paths_list = request_paths(reason)
            valid_paths, errors = verify_files_folders_are_reachable(paths_list)

        checked_directories = set()
        for item in paths_list:
            checked_directories = walk_directories_and_subdirectories(
                item,
                client,
                db_connection,
                checked_directories,
                table_name
            )

    for item in client.execute("SELECT * FROM source_files WHERE hash NOT IN (SELECT hash FROM target_files)"):
        print(item)

# todo, need to add warnings for empty files will not appear in list
# hex hash for empty file: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

    # todo, need to multithread work on hashing


    #todo, figure out comparison interactions....also, what happens if old and new file caches are mixed?
    #todo, figue out how to specify file name of db
    # todo, give tools to inspect DB contents, compare old DB contents
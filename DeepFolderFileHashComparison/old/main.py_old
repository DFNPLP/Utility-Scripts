import argparse
import os
import sqlite3
from enum import auto, StrEnum

from compare import setup_db_tables_non_destructively, check_db_status, totally_reset_db, \
    verify_files_folders_are_reachable, walk_directories_and_subdirectories, \
    get_files_not_present_in_target_but_present_in_source, get_empty_source_files, get_empty_target_files
from fsm_menu import Fsm

DEFAULT_DB_FILE_NAME = "data_cache"


def check_file_exists(db_file_path):
    return os.path.exists(db_file_path)


def print_if_not_quiet(text, is_silent):
    if not is_silent:
        print(text)


def main(args):
    file_exists = check_file_exists(args.output_db_path)
    db_connection = sqlite3.connect(args.output_db_path)
    db_client = db_connection.cursor()
    if not file_exists:
        print_if_not_quiet(f"DB at path not found: {args.output_db_path}", args.quiet)
        if args.source_folder_path is None or args.target_folder_path is None or \
                not check_file_exists(args.source_folder_path) or not check_file_exists(args.target_folder_path):
            # todo, trigger help from arg_parser?
            print("Missing paths or invalid paths provided. Both the source and target paths must exist, "
                  f"be accessible, and be non-None. Source folder path provided: {args.source_folder_path} "
                  f"Target folder path provided: {args.target_folder_path}")
            return

        setup_db_tables_non_destructively(db_connection, db_client)
        for item, table_name in [
            (args.source_folder_path, "source_files"), (args.target_folder_path, "target_files")
        ]:
            walk_directories_and_subdirectories(
                item,
                db_client,
                db_connection,
                set(),
                table_name,
                args.follow_symlinks
            )

        #todo, spit out occaisional progress if not complete and killed early...maybe just warn if so?
        # todo, run multi threaded?
    else:
        # if check_file_exists(args.output_db_path):
        #     # todo, trigger help from arg_parser?
        #     print(f"DB at path already exists! Try running with \"python compare.py -i {args.output_db_path}\" "
        #           f"or deleting the file before specifying {args.output_db_path} as the output DB file.")
        #     return

        #todo, make sure DB run wasn't partial and is compatible...probably warn if necessary
        missing_in_target = get_files_not_present_in_target_but_present_in_source(db_client)

        # todo, flag enabling hiding these
        empties_source = get_empty_source_files(db_client)
        empties_target = get_empty_target_files(db_client)

        # todo option to print out to files
        print(missing_in_target, empties_source, empties_target)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        prog="DeepFolderFileHashComparison",
        description="Allows you to index files by hash in on-disk database and compare that to another folder."
    )

    #todo, check if I can specify required if another is specified on these

    parser.add_argument(
        '-o',
        '--output_db_path',
        action='store',
        dest="output_db_path",
        default=DEFAULT_DB_FILE_NAME
    )

    parser.add_argument(
        '-q',
        '--quiet',
        action="store_true",
        dest="quiet",
        default=False
    )

    parser.add_argument(
        '-s',
        '--source_folder_path',
        action='store',
        dest="source_folder_path",
        default=None
    )

    parser.add_argument(
        '-t',
        '--target_folder_path',
        action='store',
        dest="target_folder_path",
        default=None
    )

    parser.add_argument(
        '-l',
        '--follow_symlinks',
        action="store_true",
        dest="follow_symlinks",
        default=False
    )

    main(parser.parse_args())





#todo, rename file to compare.py


#todo, validate this al works...run in debugger with some comparison functions.
#todo.....need to deal with...what if multiple files with the same hash appear in target? what if multiple in source?
import argparse
from deep_folder_file_hash_comparison.constants import DEFAULT_DB_FILE_NAME


def get_parser():
    parser = argparse.ArgumentParser(
        prog="DeepFolderFileHashComparison",
        description="Allows you to index files by hash in on-disk database and compare that to another folder."
    )

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
        dest="source_folder_path"
    )

    parser.add_argument(
        '-t',
        '--target_folder_path',
        action='store',
        dest="target_folder_path"
    )

    parser.add_argument(
        '-l',
        '--follow_symlinks',
        action="store_true",
        dest="follow_symlinks",
        default=False
    )

    parser.add_argument(
        '-c',
        '--continue',
        action="store_true",
        dest="continue_if_halted",
        default=False
    )

    return parser

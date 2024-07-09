import os

from deep_folder_file_hash_comparison.dependencies.dependencies import get_parser
from deep_folder_file_hash_comparison.file_manipulation import check_path_exists


def main(parser, processed_args):
    if not processed_args.source_folder_path or not check_path_exists(processed_args.source_folder_path):
        raise ValueError("No path or invalid path provided for source_folder_path: "
                         f"{processed_args.target_folder_path}{os.linesep}{parser.format_help()}")


if __name__ == "__main__":
    parser = get_parser()
    processed_args = parser.parse_args()
    main(parser, processed_args)

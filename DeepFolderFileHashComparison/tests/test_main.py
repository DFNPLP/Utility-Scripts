import argparse
import os
import re

import pytest as pytest

from deep_folder_file_hash_comparison.dependencies.dependencies import get_parser
from deep_folder_file_hash_comparison.main import main


@pytest.mark.parametrize(
    "source_flag_data",
    [
        ["-s", "./tests/data/%%%"],
        ["--source_folder_path", "./tests/data/%%%"],
        []
    ]
)
def test_source_not_valid(source_flag_data):
    parser = get_parser()
    parsed_args = parser.parse_args(source_flag_data)

    with pytest.raises(
            ValueError,
            match=re.escape(
                "No path or invalid path provided for source_folder_path: "
                f"{parsed_args.target_folder_path}{os.linesep}{parser.format_help()}"
            )
    ):
        main(parser, parsed_args)


@pytest.skip("Needs to be worked on.....might not be testable if exits on parsing error....need to check")
@pytest.mark.parametrize(
    "source_flag_data",
    [
        ["-s"],
        ["--source_folder_path"]
    ]
)
def test_source_not_specified(capfd, source_flag_data):
    pass

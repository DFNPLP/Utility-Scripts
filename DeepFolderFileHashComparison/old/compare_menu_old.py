import argparse
import os
import sqlite3
from enum import auto, StrEnum

from compare import setup_db_tables_non_destructively, check_db_status, totally_reset_db, \
    verify_files_folders_are_reachable, walk_directories_and_subdirectories, \
    get_files_not_present_in_target_but_present_in_source
from fsm_menu import Fsm

DEFAULT_DB_FILE_NAME = "data_cache"


def check_file_exists(db_file_path):
    return os.path.exists(db_file_path)


def prompt_for_data(prompt, error_message, transform=lambda data: data, verify=lambda data: True):
    response = input(f"{prompt} ")
    transformed_response = transform(response)

    while not verify(transformed_response):
        response = input(f"{error_message} ")
        transformed_response = transform(response)

    return transformed_response


def prompt_delete_db(why_the_db_needs_to_be_deleted_text, confirm_only=False):
    delete_if_true_exit_if_false = False
    delete_old_db = False
    if not confirm_only:
        delete_old_db = prompt_for_data(
            f"A previous version of the DB exists and conflicts with this run because: "
            f"{why_the_db_needs_to_be_deleted_text} Would you like to delete the previous DB? (y/n)",
            "Response not recognized. A previous version of the DB exists and conflicts with this run because: "
            f"{why_the_db_needs_to_be_deleted_text} Would you like to delete the previous DB? (y/n)",
            lambda data: data.lower().trim() if type(data) is str else data,
            lambda data: data in ["y", "n"]
        ) == "y"

    if delete_old_db or confirm_only:
        confirm_delete = prompt_for_data(
            "Are you sure you wish to delete the old DB? (y/n)",
            "Response not recognized. Are you sure you wish to delete the old DB? (y/n)",
            lambda data: data.lower().trim() if type(data) is str else data,
            lambda data: data in ["y", "n"]
        ) == "y"
        if confirm_delete:
            delete_if_true_exit_if_false = True
    else:
        print("Please specify a new DB name on next run or move the current DB if you wish to save it.")
        delete_if_true_exit_if_false = False

    return delete_if_true_exit_if_false


def setup_db(db_file_name):
    db_connection = sqlite3.connect(db_file_name)
    client = db_connection.cursor()

    setup_db_tables_non_destructively(db_connection, client)

    return db_connection, client


def select_path(prompt, path=None):
    return get_data(
        path,
        prompt,
        validator_func=lambda data: verify_files_folders_are_reachable(data),
    )


def get_data(
    initial_data,
    prompt,
    error_prompt=None,
    validator_func=lambda data: (True, None),
    transformer_func=lambda data: data
):
    error_prompt_to_use = prompt if not error_prompt else error_prompt
    new_data = initial_data
    valid_data, error = validator_func(new_data)
    while not valid_data:
        if error:
            print(f"Error: {error}")
            data_candidate = input(error_prompt_to_use)
            valid_data, error = validator_func(data_candidate)
        else:
            data_candidate = input(prompt)
            valid_data, error = validator_func(data_candidate)

        if valid_data:
            new_data = data_candidate

    return transformer_func(new_data)


def get_inputs_state(data):
    data["compare_source"], data["compare_target_from_data"], data["follow_symlinks"] = \
        get_inputs_required_for_compare(
        data.get("compare_source", None),
        data.get("compare_target", None),
        data.get("follow_symlinks", None)
    )

    return States.PROCESS_DATA, data


def process_data_state(data):
    db_connection = data.get("db_connection")
    db_client = data.get("db_client")

    for item, table_name in [
        (data.get("compare_source"), "source_files"), (data.get("compare_target"), "target_files")
    ]:
        walk_directories_and_subdirectories(
            item,
            db_client,
            db_connection,
            set(),
            table_name,
            data.get("follow_symlinks")
        )

    return States.COMPARE_DATA, data


def get_inputs_required_for_compare(
    source_path,
    source_target,
    follow_symlinks
):
    new_source_path = select_path(
        "Please provide a single path for source file checks. ",
        source_path
    )
    new_target_path = select_path(
        "Please provide a single path for target file checks. ",
        source_target
    )
    follow_symlinks = get_data(
        follow_symlinks,
        "Should symlinks should be followed? (y/n?) ",
        validator_func=lambda data: (
            data is not None and data.lower() in {"y", "n"},
            None if data is not None and data.lower() in {"y", "n"} else "Must be 'y', or 'n'."
        ),
        transformer_func=lambda data: data.lower() == "y"
    )

    return new_source_path, new_target_path, follow_symlinks


def validate_and_set_up(data):
    db_file_name = data.get("db_file_name")
    db_already_existed = check_file_exists(db_file_name)
    db_connection, db_client = setup_db(db_file_name)
    (
        is_compatible_version_or_irrelevant,
        previous_exists,
        completed,
        previous_run_timestamp,
        db_source_path,
        db_target_path,
        db_follow_symlinks
    ) = check_db_status(db_client)

    return_data = {
        "db_file_name": db_file_name,
        "db_already_existed": db_already_existed,
        "db_connection": db_connection,
        "db_client": db_client,
        "previous_run_timestamp": previous_run_timestamp
    }

    if not is_compatible_version_or_irrelevant and previous_exists:
        return States.DELETE_DB_BECAUSE_VERSION_CONFLICT, return_data
    elif previous_exists and completed:
        return States.DELETE_DB_BECAUSE_PREVIOUS_RUN, return_data
    else:
        return States.REQUEST_PATHS, return_data


def delete_because_version_conflict(data):
    delete_if_true_exit_if_false = prompt_delete_db(
        "the DB was created with a version of this software which is incompatible with the current version"
    )
    if not delete_if_true_exit_if_false:
        print("Please specify a different db name on next run or move the current DB before running this "
              "program again.")
        return None, None

    else:
        db_connection = data.get("db_connection")
        db_client = data.get("db_client")
        totally_reset_db(db_connection, db_client)
        return States.REQUEST_PATHS, data


def delete_because_previous_run(data):
    previous_run_timestamp = data.get("previous_run_timestamp")
    choice = int(prompt_for_data(
        f"A previous run of this program from {previous_run_timestamp} was stored. Would you like to (1 or 2):\n1. "
        "Continue the previous analysis\n2. Delete the previous DB\n",
        f"Response not recognized. A previous run of this program from {previous_run_timestamp} was stored. "
        "Would you like to (1 or 2):\n1. Continue with the previous analysis "
        "(Warning: data could be out of date)\n2. Delete the previous DB\n",
        verify=lambda selection: selection in ["1", "2"]
    ))
    match choice:
        case 1:
            return States.COMPARE_DATA, data
        case 2:
            delete_if_true_exit_if_false = prompt_delete_db(
                "the DB has previous data in it",
                confirm_only=True
            )
            if not delete_if_true_exit_if_false:
                print("Please specify a different db name on next run or move the current DB before running this "
                      "program again.")
                return None, None
            else:
                db_connection = data.get("db_connection")
                db_client = data.get("db_client")
                totally_reset_db(db_connection, db_client)
                return States.REQUEST_PATHS, data


def compare_data_state(data):
    db_client = data.get("db_client")
    data = get_files_not_present_in_target_but_present_in_source(db_client)
    for item in data:
        # todo, allow file output
        print(item)


class States(StrEnum):
    VALIDATE_AND_SET_UP = auto()
    REQUEST_PATHS = auto()
    DELETE_DB_BECAUSE_PREVIOUS_RUN = auto()
    DELETE_DB_BECAUSE_VERSION_CONFLICT = auto()
    PROCESS_DATA = auto()
    COMPARE_DATA = auto()


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

    parser.add_argument(
        '-l',
        '--follow_symlinks',
        action="store_true",
        dest="follow_symlinks",
        default=False
    )

    args = parser.parse_args()

    db_file_name = args.db_name if args.db_name else DEFAULT_DB_FILE_NAME
    fsm_menu = Fsm()
    fsm_menu.register_states(
        [state for state in States]
    )

    fsm_menu.set_state_handler(States.VALIDATE_AND_SET_UP, validate_and_set_up)
    fsm_menu.set_state_handler(States.PROCESS_DATA, process_data_state)
    fsm_menu.set_state_handler(States.COMPARE_DATA, compare_data_state)
    fsm_menu.set_state_handler(States.DELETE_DB_BECAUSE_PREVIOUS_RUN, delete_because_previous_run)
    fsm_menu.set_state_handler(States.DELETE_DB_BECAUSE_VERSION_CONFLICT, delete_because_version_conflict)
    fsm_menu.set_state_handler(States.REQUEST_PATHS, get_inputs_state)

    fsm_menu.set_state(
        States.VALIDATE_AND_SET_UP,
        {
            "compare_source": args.source,
            "compare_target": args.target,
            "follow_symlinks": args.follow_symlinks,
            "db_file_name": args.db_name
        }
    )

    fsm_menu.execute_fsm()


#todo, validate this al works...run in debugger with some comparison functions.
#todo.....need to deal with...what if multiple files with the same hash appear in target? what if multiple in source?
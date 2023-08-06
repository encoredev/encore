#!/usr/bin/env bash
#
# This script will run the same checks as Encore's CI pipeline and report the same static analysis errors
# as the pipeline by default. It can be used to check for what errors might be reported by the pipeline
# before you commit and open a PR.
#
# Usage:
#   ./check.bash [options]
#
# Options:
#   --base <ref>         The merge base to compare against (default: origin/main)
#   --diff               Show the diff against base instead of running the checks
#   --filter-mode <mode> The filter mode to use for reviewdog; added, file, diff_context, nofilter (default: file)
#   --all                Alias for `--filter-mode nofilter` (runs checks against all files in the working directory)
#
# Examples:
#
#   # Run the checks against files changed since branching from origin/main
#   # (This is the default behavior and what our CI process does)
#   ./check.bash
#
#   # Show the diff between the current working directory and origin/main
#   ./check.bash --diff
#
#   # Run the checks against the entire working directory (regardless of changes made)
#   ./check.bash --all


##############################################################################################################################
# Step 0: Setup the script with basic error handling                                                                         #
##############################################################################################################################

  set -euo pipefail
  # nosemgrep
  IFS=$'\n\t'

  function errHandler() {
    echo "Exiting due to an error line $1" >&2
    echo "" >&2
    awk 'NR>L-4 && NR<L+4 { printf "%-5d%3s%s\n",NR,(NR==L?">> ":""),$0 }' L="$1" "$0" >&2
  }
  trap 'errHandler $LINENO' ERR


##############################################################################################################################
# Step 1: Configure the script with the parameters the use wants                                                             #
##############################################################################################################################

  # Parameters
  WORK_DIR=$( dirname "${BASH_SOURCE[0]}" ) # Get the directory this script is in
  BASE_REF="origin/main"                    # The merge base to compare against
  DIFF_ONLY="false"                         # If true, show the diff instead of running the checks
  FILTER_MODE="file"                        # The filter mode to use for reviewdog (added, file, diff_context, nofilter)

  # Parse the command line arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base)
        BASE_REF="$2"
        shift 2
        ;;
      --diff)
        DIFF_ONLY="true"
        shift 1
        ;;
      --filter-mode)
        FILTER_MODE="$2"
        shift 2
        ;;
      --all)
        FILTER_MODE="nofilter"
        shift 1
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
  done


##############################################################################################################################
# Step 2: Check for required tools and error out if anything is missing which we can't install for the user                  #
##############################################################################################################################

  # Check for tools we can't install using go
  command -v go >/dev/null 2>&1 || { echo >&2 "go is required but not installed. Aborting."; exit 1; }
  command -v git >/dev/null 2>&1 || { echo >&2 "git is required but not installed. Aborting."; exit 1; }
  command -v sed >/dev/null 2>&1 || { echo >&2 "sed is required but not installed. Aborting."; exit 1; }
  command -v semgrep >/dev/null 2>&1 || { echo >&2 "semgrep is required but not installed. Aborting."; exit 1; }

  # Now install all missing tools
  command -v reviewdog >/dev/null 2>&1 || go install github.com/reviewdog/reviewdog/cmd/reviewdog@latest || { echo >&2 "Unable to install reviewdog. Aborting."; exit 1; }
  command -v staticcheck >/dev/null 2>&1 || go install honnef.co/go/tools/cmd/staticcheck@latest || { echo >&2 "Unable to install staticcheck. Aborting."; exit 1; }
  command -v errcheck >/dev/null 2>&1 || go install github.com/kisielk/errcheck@latest || { echo >&2 "Unable to install errcheck. Aborting."; exit 1; }
  command -v ineffassign >/dev/null 2>&1 || go install github.com/gordonklaus/ineffassign@latest || { echo >&2 "Unable to install ineffassign. Aborting."; exit 1; }


##############################################################################################################################
# Step 3: Create a diff of the changes in the working directory against the common ancestor of the current branch and main   #
#         This will be used to run static analysis checks on only the files that have changed. This diff should mimic the    #
#         diff that would be created by Github when all current changes are comitted and pushed into a PR on github.         #
##############################################################################################################################

  # Don't generate the diff if we don't need it to filter!
  if [[ "$FILTER_MODE" != "nofilter" ]]; then

    # Create a temp directory to store the common ancestor commit
    TMP_DIR=$(mktemp -d)
    if [[ ! "$TMP_DIR" || ! -d "$TMP_DIR" ]]; then
      echo "Could not create temp dir"
      exit 1
    fi

    # Create a temp file to store the diff we need
    DIFF_FILE=$(mktemp)
    if [[ ! "$DIFF_FILE" || ! -f "$DIFF_FILE" ]]; then
      echo "Could not create temp diff file"
      exit 1
    fi

    # Create a blank file to use as a comparison when a file is missing because either it's new or been deleted
    BLANK_FILE=$(mktemp)
    if [[ ! "$BLANK_FILE" || ! -f "$BLANK_FILE" ]]; then
      echo "Could not create blank file"
      exit 1
    fi

    # Clean up on exit and delete all the temp files we just created
    function cleanup() {
      rm -rf "$TMP_DIR"
      rm -f "$DIFF_FILE"
      rm -f "$BLANK_FILE"
    }
    trap cleanup EXIT

    # Clone the repo into the temp directory
    git clone -q "$WORK_DIR" "$TMP_DIR"

    # Change our temp directory to be a clean copy of the common ancestor commit
    pushd "$TMP_DIR" > /dev/null
    git reset -q --hard HEAD
    git checkout -q "$(git merge-base "$BASE_REF" HEAD)"
    TRACKED_FILES_FROM_MAIN=$(git ls-files)
    popd > /dev/null

    # Create a list of files that we care about
    MODIFICATIONS_IN_WORKING_DIR=$(git status --short | awk '{print $2}')
    TRACKED_FILES_IN_WORKING_DIR=$(git ls-files)
    ALL_FILES=$(echo "$TRACKED_FILES_IN_WORKING_DIR $MODIFICATIONS_IN_WORKING_DIR $TRACKED_FILES_FROM_MAIN" | tr ' ' '\n' | sort -u)

    # Create a diff of the changes in the working directory against the common ancestor of the current branch and main
    for file in $ALL_FILES; do
      # If the original file doesn't exist, use a blank file instead
      # (This means it was a new file that was added in the current version of the code base)
      ORIGINAL_FILE="$TMP_DIR/$file"
      if [[ ! -f "$ORIGINAL_FILE" ]]; then
        ORIGINAL_FILE="$BLANK_FILE"
      fi

      # If the updated file doesn't exist, use a blank file instead
      # (This means the file was deleted in the current version of the code base)
      UPDATED_FILE="$WORK_DIR/$file"
      if [[ ! -f "$UPDATED_FILE" ]]; then
        UPDATED_FILE="$BLANK_FILE"
      fi

      # Run git diff between the original file and the updated file
      # Replace the file paths in the diff to match the relative path in the working directory
      # Then write the diff into our diff file
      git diff "$ORIGINAL_FILE" "$UPDATED_FILE" | sed "s|$ORIGINAL_FILE|/$file|g" | sed "s|$UPDATED_FILE|/$file|g" >> "$DIFF_FILE" || true # Suppress the exit code
    done

    if [[ "$DIFF_ONLY" == "true" ]]; then
      cat "$DIFF_FILE"
      exit 0
    fi
  fi


##############################################################################################################################
# Step 4: Run review dog using the diff we just created, allowing reviewdog to only show errors from changes we've made      #
##############################################################################################################################

  if [[ "$FILTER_MODE" == "nofilter" ]]; then
    reviewdog -filter-mode=nofilter
  else
    reviewdog -filter-mode="$FILTER_MODE" -diff="cat $DIFF_FILE"
  fi

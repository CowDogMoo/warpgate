#!/bin/bash

set -e

TESTS_TO_RUN=$1
RETURN_CODE=0

TIMESTAMP=$(date +"%Y%m%d%H%M%S")
LOGFILE="/tmp/goutils-unit-test-results-$TIMESTAMP.log"

if [[ -z "${TESTS_TO_RUN}" ]]; then
    echo "No tests input" | tee -a "$LOGFILE"
    echo "Example - Run all shorter collection of tests: bash run-go-tests.sh short" | tee -a "$LOGFILE"
    echo "Example - Run all tests: bash run-go-tests.sh all" | tee -a "$LOGFILE"
    echo "Example - Run coverage for a specific version: bash run-go-tests.sh coverage" | tee -a "$LOGFILE"
    exit 1
fi

run_tests() {
    local coverage_file=$1
    repo_root=$(git rev-parse --show-toplevel 2> /dev/null) || exit
    pushd "${repo_root}" || exit
    echo "Logging output to ${LOGFILE}" | tee -a "$LOGFILE"
    echo "Run the following command to see the output in real time:" | tee -a "$LOGFILE"
    echo "tail -f ${LOGFILE}" | tee -a "$LOGFILE"
    echo "Running tests..." | tee -a "$LOGFILE"

    if [[ "${TESTS_TO_RUN}" == 'coverage' ]]; then
        go test -v -race -failfast -tags=integration -coverprofile="${coverage_file}" ./... 2>&1 | tee -a "$LOGFILE"
    elif [[ "${TESTS_TO_RUN}" == 'all' ]]; then
        go test -v -race -failfast ./... 2>&1 | tee -a "$LOGFILE"
    elif [[ "${TESTS_TO_RUN}" == 'short' ]] && [[ "${GITHUB_ACTIONS}" != "true" ]]; then
        go test -v -short -failfast -race ./... 2>&1 | tee -a "$LOGFILE"
    else
        if [[ "${GITHUB_ACTIONS}" != 'true' ]]; then
            go test -v -failfast -race "./.../${TESTS_TO_RUN}" 2>&1 | tee -a "$LOGFILE"
        fi
    fi

    RETURN_CODE=$?
}

if [[ "${TESTS_TO_RUN}" == 'coverage' ]]; then
    run_tests 'coverage-all.out'
else
    run_tests
fi

if [[ "${RETURN_CODE}" -ne 0 ]]; then
    echo "unit tests failed" | tee -a "$LOGFILE"
    exit 1
fi

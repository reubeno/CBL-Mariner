#!/usr/bin/env python3
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import argparse
import json
import logging
import multiprocessing
import os
import subprocess
import sys
import time

logger = logging.getLogger(__name__)
self_path = os.path.abspath(__file__)
script_dir_path = os.path.dirname(self_path)
toolkit_dir_path = os.path.dirname(script_dir_path)
repo_root_dir_path = os.path.dirname(toolkit_dir_path)
build_dir_path = os.path.join(repo_root_dir_path, "build")
test_results_path = os.path.join(build_dir_path, "pkg_artifacts", "test_results.json")
logs_dir_path = os.path.join(build_dir_path, "logs")
ptest_logs_dir_path = os.path.join(logs_dir_path, "pkggen", "rpmbuilding")

parser = argparse.ArgumentParser(description="Run package tests")
parser.add_argument("-s", "--spec", dest="specs", action="append", help="Names of specs to run tests for")
parser.add_argument("-v", "--verbose", dest="verbose", action="store_true", help="Enable verbose output")
parser.add_argument("--markdown-report", dest="markdown_report", help="Path to output markdown report of test results")
parser.add_argument("--report-last-results-only", dest="report_last_results_only", action="store_true", help="Report only the last test results without re-running tests")

args = parser.parse_args()

if args.verbose:
    log_level = logging.DEBUG
else:
    log_level = logging.INFO

logging.basicConfig(format="%(levelname)s: %(message)s", level=log_level)

if not os.path.isfile(os.path.join(toolkit_dir_path, "Makefile")):
    logger.error("can't find toolkit path")
    sys.exit(1)

#
# Run.
#

if args.report_last_results_only:
    logger.info("reporting last test results only; skipping build/test")

else:
    if not args.specs:
        logger.error("no specs provided")
        sys.exit(1)

    toolkit_log_level = "info" if args.verbose else "warn"

    # N.B. We do *not* use TEST_RUN_LIST or TEST_RERUN_LIST, because those
    # fail in undesirable ways on specs with no %check sections. We instead
    # just set RUN_CHECK=y and figure out after the fact what happened.
    cmd = [
        "sudo",
        "make",
        "build-packages",
        "-j",
        str(multiprocessing.cpu_count()),
        "REBUILD_TOOLS=y",
        f"LOG_LEVEL={toolkit_log_level}",
        f"SRPM_PACK_LIST={' '.join(args.specs)}",
        f"PACKAGE_REBUILD_LIST={' '.join(args.specs)}",
        "RUN_CHECK=y",
        "DAILY_BUILD_ID=lkg"
    ]

    start_time = time.time()

    result = subprocess.run(cmd, cwd=toolkit_dir_path)
    if result.returncode != 0:
        logger.error("build tools invocation failed")
        sys.exit(1)

    elapsed_time = time.time() - start_time

    logger.info(f"Ran tests in {elapsed_time:.2f}s.")

#
# Now analyze
#

class ReadableTestReporter:
    def __init__(self, display_log_paths: bool = True):
        self._display_log_paths = display_log_paths

    def on_skipped(self, name: str):
        print(f"⏩ {srpm_name}: SKIPPED")

    def on_blocked(self, name: str):
        print(f"🚫 {srpm_name}: BLOCKED")

    def on_failed(self, name: str, expected_failure: bool, log_path: str):
        if expected_failure:
            print(f"🟡 {srpm_name}: FAILED (expected)")
        else:
            print(f"❌ {srpm_name}: FAILED")
        
        if self._display_log_paths:
            print(f"    Log: {log_path}")

    def on_succeeded(self, name: str, expected_failure: bool):
        if expected_failure:
            print(f"🔴 {srpm_name}: PASSED (unexpected)")
        else:
            print(f"✅ {srpm_name}: PASSED")

    def on_unknown_result(self, name):
        print(f"❓ {srpm_name}: {result}")

class MarkdownTestReporter:
    def __init__(self, report_path: str):
        self._report_file = open(report_path, "w")

    def close(self):
        self._report_file.close()

    def on_skipped(self, name: str):
        self._test_heading(name)
        self._write_line(f"⏩ {srpm_name}: SKIPPED")
        self._write_line("")

    def on_blocked(self, name: str):
        self._test_heading(name)
        self._write_line(f"🚫 {srpm_name}: BLOCKED")
        self._write_line("")

    def on_failed(self, name: str, expected_failure: bool, log_path: str):
        self._test_heading(name)

        if expected_failure:
            self._write_line(f"🟡 {srpm_name}: FAILED (expected)")
        else:
            self._write_line(f"❌ {srpm_name}: FAILED")

        LINES_TO_SHOW = 100

        self._write_line("")
        self._write_line(f"Last {LINES_TO_SHOW} lines of test output:\n")

        self._write_line("```")
        with open(log_path, "r") as log_file:
            for line in log_file.readlines()[-LINES_TO_SHOW:]:
                self._report_file.write(line)
        self._write_line("```")
        self._write_line("")

    def on_succeeded(self, name: str, expected_failure: bool):
        self._test_heading(name)

        if expected_failure:
            self._write_line(f"🔴 {srpm_name}: PASSED (unexpected)")
        else:
            self._write_line(f"✅ {srpm_name}: PASSED")

        self._write_line("")

    def on_unknown_result(self, name):
        self._test_heading(name)
        self._write_line(f"❓ {srpm_name}: {result}")
        self._write_line("")

    def _test_heading(self, name: str):
        self._write_line(f"## `{name}`\n\n")

    def _write_line(self, line: str):
        self._report_file.write(line)
        self._report_file.write("\n")

logger.debug(f"Analyzing test results: {test_results_path}")

with open(test_results_path, "r") as test_results:
    test_results_text = test_results.read()
    test_results = json.loads(test_results_text)

unexpected_fail_count = 0
expected_fail_count = 0
skip_count = 0
block_count = 0
expected_success_count = 0
unexpected_success_count = 0

reporters = [ReadableTestReporter()]

markdown_reporter = None
if args.markdown_report:
    markdown_reporter = MarkdownTestReporter(args.markdown_report)
    reporters.append(markdown_reporter)

#
# Report results.
#

# TODO: Figure out which components didn't *have* any checks to run.
for srpm_name, srpm_results in test_results.items():
    result = srpm_results["Result"]
    expected_failure = srpm_results["ExpectedFailure"]

    display_log_path = False
    if result == "skipped":
        for reporter in reporters:
            reporter.on_skipped(srpm_name)
        skip_count += 1
    elif result == "blocked":
        for reporter in reporters:
            reporter.on_blocked(srpm_name)
        block_count += 1
    elif result == "failed":
        if expected_failure:
            expected_fail_count += 1
        else:
            unexpected_fail_count += 1  
        for reporter in reporters:
            reporter.on_failed(srpm_name, expected_failure, srpm_results["LogPath"])
    elif result == "succeeded":
        if expected_failure:
            unexpected_success_count += 1
        else:
            expected_success_count += 1
        for reporter in reporters:
            reporter.on_succeeded(srpm_name, expected_failure)
    else:
        for reporter in reporters:
            reporter.on_unknown_result(srpm_name)

if markdown_reporter:
    markdown_reporter.close()

#
# Display a readable summary.
#

print("")
if expected_success_count > 0:
    print(f"Tests succeeded:              {expected_success_count}")
if unexpected_success_count > 0:
    print(f"Tests succeeded unexpectedly: {unexpected_success_count}")
if expected_fail_count > 0:
    print(f"Tests expected to fail:       {expected_fail_count}")
if unexpected_fail_count > 0:
    print(f"Tests failed:                 {unexpected_fail_count}")
if block_count > 0:
    print(f"Tests blocked:                {block_count}")
if skip_count > 0:
    print(f"Tests skipped:                {skip_count}")

if unexpected_fail_count > 0 or block_count > 0:
    logger.error("One or more tests were failed or blocked; exiting with error.")
    sys.exit(1)

#!/bin/bash
set -e

# Called with the problem directory as the first argument.
# e.g. /contest/A
PROBLEM_DIR="${1:?usage: entrypoint.sh <problem-dir>}"

cd "$PROBLEM_DIR"

echo "[builder] Compiling solution.cpp..."
g++ -O2 -o solution solution.cpp

echo "[builder] Building tcframe runner from spec.cpp..."
tcframe build

echo "[builder] Generating test cases..."
./runner --solution=./solution

echo "[builder] Done. Test cases written to tc/"

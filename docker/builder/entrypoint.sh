#!/bin/bash
set -e

PROBLEM_DIR="${1:?usage: entrypoint.sh <problem-dir>}"

cd "$PROBLEM_DIR"

echo "[builder] Compiling solution.cpp..."
g++ -O2 -std=c++20 -o solution solution.cpp

# Compile scorer/communicator before running the tcframe runner — it calls the
# scorer during test case generation to validate sample outputs.
if [ -f scorer.cpp ]; then
    echo "[builder] Compiling scorer.cpp..."
    g++ -O2 -std=c++20 -o scorer scorer.cpp
    echo "[builder] scorer compiled."
fi

if [ -f communicator.cpp ]; then
    echo "[builder] Compiling communicator.cpp..."
    g++ -O2 -std=c++20 -o communicator communicator.cpp
    echo "[builder] communicator compiled."
fi

echo "[builder] Building tcframe runner from spec.cpp..."
tcframe build

echo "[builder] Generating test cases..."
./runner --solution=./solution

echo "[builder] Done. Test cases written to tc/"

# Generate config.json from spec.cpp if not already present.
# This gives the judge the subtask→group structure for IOI scoring.
if [ ! -f config.json ]; then
    echo "[builder] Generating config.json from spec.cpp..."
    if python3 /parse_spec.py spec.cpp > config.json 2>/dev/null; then
        echo "[builder] config.json generated."
    else
        rm -f config.json
        echo "[builder] No subtask structure found in spec.cpp — skipping config.json."
    fi
else
    echo "[builder] config.json already exists — not overwriting."
fi

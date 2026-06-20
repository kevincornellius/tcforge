#!/usr/bin/env python3
"""
Parse a tcframe spec.cpp and extract subtask/test-group structure.
Outputs a config.json to stdout.

Structure detected:
  void SubtaskN() { Points(M); ... }
  void TestGroupN() { Subtasks({s1, s2, ...}); ... }
"""
import re, json, sys

def extract_body(text, pos):
    """Return the text of the { } block starting at pos (which should be a '{')."""
    start = text.find('{', pos)
    if start == -1:
        return ""
    depth = 1
    i = start + 1
    while i < len(text) and depth > 0:
        if text[i] == '{':
            depth += 1
        elif text[i] == '}':
            depth -= 1
        i += 1
    return text[start:i]


def parse(spec_path):
    with open(spec_path) as f:
        spec = f.read()

    # Subtask points: void SubtaskN() { ... Points(M) ... }
    subtask_points = {}
    for m in re.finditer(r'void\s+Subtask(\d+)\s*\(\s*\)', spec):
        body = extract_body(spec, m.end())
        pm = re.search(r'Points\s*\((\d+)\)', body)
        if pm:
            subtask_points[int(m.group(1))] = int(pm.group(1))

    # Test group → subtask assignments: void TestGroupN() { Subtasks({...}); ... }
    test_groups = {}
    for m in re.finditer(r'void\s+TestGroup(\d+)\s*\(\s*\)', spec):
        body = extract_body(spec, m.end())
        sm = re.search(r'Subtasks\s*\(\{([^}]+)\}\)', body)
        if sm:
            subs = [int(s.strip()) for s in sm.group(1).split(',') if s.strip().lstrip('-').isdigit()]
            test_groups[int(m.group(1))] = subs

    if not subtask_points or not test_groups:
        return None

    num_groups = max(test_groups.keys())
    num_subtasks = max(subtask_points.keys())

    return {
        "test_groups": [test_groups.get(i + 1, []) for i in range(num_groups)],
        "points": [subtask_points.get(i + 1, 0) for i in range(num_subtasks)],
    }


if __name__ == "__main__":
    spec_path = sys.argv[1] if len(sys.argv) > 1 else "spec.cpp"
    result = parse(spec_path)
    if result:
        print(json.dumps(result, indent=2))
    else:
        sys.exit(1)

#!/usr/bin/env python3
"""Mark slow top-level tests so the default `go test` run skips them.

Reads `go test -json` output, takes every top-level test that passed in at
least --seconds, and inserts `slowtest.Skip(t)` as its first statement. Tests
already marked are left alone, so the script can be rerun on newer timings.

    PLAINSOLID_SLOW=1 go test -json ./... > timings.jsonl
    python3 tools/ci/mark-slow-tests.py timings.jsonl
    gofmt -l . && go vet ./...
"""
import argparse
import json
import re
import subprocess
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path

MODULE = "plainsolid"
IMPORT = f'"{MODULE}/internal/slowtest"'


def timestamp(text):
    """Parse a go test -json time, which may carry nanoseconds."""
    text = text.replace("Z", "+00:00")
    head, zone = text[:-6], text[-6:]
    if "." in head:
        whole, fraction = head.split(".")
        head = whole + "." + (fraction + "000000")[:6]
    return datetime.fromisoformat(head + zone)


def slow_tests(paths, seconds):
    """Return {directory: {test: seconds}} for tests at least `seconds` long.

    A test's duration is the wall time from its start to its end, less the
    time it spent paused by t.Parallel. The Elapsed field alone is not enough:
    for a test whose subtests call t.Parallel it covers only the parent's own
    statements and can read 0.01 s for a test that takes a minute.
    """
    found = defaultdict(dict)
    for path in paths:
        started, paused_at, paused = {}, {}, defaultdict(float)
        with open(path, encoding="utf-8") as lines:
            for line in lines:
                if not line.startswith("{"):
                    continue
                event = json.loads(line)
                name, package, action = event.get("Test"), event.get("Package", ""), event.get("Action")
                if not name or "/" in name or not name.startswith("Test") or not package.startswith(MODULE):
                    continue
                key = (package, name)
                if action == "run":
                    started[key] = timestamp(event["Time"])
                elif action == "pause":
                    paused_at[key] = timestamp(event["Time"])
                elif action == "cont" and key in paused_at:
                    paused[key] += (timestamp(event["Time"]) - paused_at.pop(key)).total_seconds()
                elif action == "pass":
                    wall = event.get("Elapsed", 0)
                    if key in started:
                        wall = max(wall, (timestamp(event["Time"]) - started[key]).total_seconds() - paused[key])
                    if wall >= seconds:
                        directory = package[len(MODULE):].lstrip("/") or "."
                        found[directory][name] = max(wall, found[directory].get(name, 0))
    return found


def add_import(text):
    if IMPORT in text:
        return text
    block = re.search(r"^import \(\n", text, re.M)
    if block:
        return text[:block.end()] + f"\t{IMPORT}\n" + text[block.end():]
    single = re.search(r'^import ("[^"]+")\n', text, re.M)
    if not single:
        raise ValueError("no import declaration")
    return text[:single.start()] + f"import (\n\t{IMPORT}\n\t{single.group(1)}\n)\n" + text[single.end():]


def mark(root, directory, names):
    remaining, changed = set(names), []
    for path in sorted((root / directory).glob("*_test.go")):
        text = path.read_text(encoding="utf-8")
        edits = 0
        for name in sorted(remaining):
            # A body on the same line gets "Skip(t); " and gofmt splits the lines.
            head = re.search(rf"^func {name}\((\w+) \*testing\.T\) +\{{(\n| )", text, re.M)
            if not head:
                continue
            remaining.discard(name)
            call = f"slowtest.Skip({head.group(1)})"
            call = f"\t{call}\n" if head.group(2) == "\n" else f"{call}; "
            if text[head.end():head.end() + 80].lstrip().startswith("slowtest.Skip("):
                continue
            text = text[:head.end()] + call + text[head.end():]
            edits += 1
        if edits:
            path.write_text(add_import(text), encoding="utf-8")
            changed.append(path)
    return changed, remaining


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("timings", nargs="+", type=Path, help="go test -json output files")
    ap.add_argument("--seconds", type=float, default=2.0, help="mark tests at least this slow (default 2)")
    ap.add_argument("--root", type=Path, default=Path.cwd(), help="module root (default: current directory)")
    opts = ap.parse_args()
    changed, missing = [], []
    for directory, names in sorted(slow_tests(opts.timings, opts.seconds).items()):
        files, remaining = mark(opts.root, directory, names)
        changed += files
        missing += [f"{directory}.{name}" for name in sorted(remaining)]
    if changed:
        subprocess.run(["gofmt", "-w", *map(str, changed)], check=True)
    print(f"marked tests in {len(changed)} files; {len(missing)} timed tests no longer exist", file=sys.stderr)
    for name in missing:
        print(f"  missing: {name}", file=sys.stderr)


if __name__ == "__main__":
    main()

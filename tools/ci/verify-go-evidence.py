#!/usr/bin/env python3
"""Reconcile sharded Go test JSON against independently enumerated tests."""
import argparse
import collections
import hashlib
import json
from pathlib import Path
import re
import sys

TRIMMED = "plainsolid/brep/trimmed"
NAME = re.compile(r"(?:Test|Fuzz|Example)[A-Za-z0-9_]*")
TERMINAL = {"pass", "skip", "fail"}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def events(path, progress=False):
    with path.open(encoding="utf-8") as stream:
        for number, line in enumerate(stream, 1):
            if progress and not line.endswith("\n"):
                # A writer may be in the middle of this final line.
                break
            try:
                yield json.loads(line)
            except json.JSONDecodeError as error:
                raise ValueError(f"{path}:{number}: malformed Go JSON: {error}") from error


def artifact(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for data in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(data)
    return {"path": str(path), "bytes": path.stat().st_size, "sha256": h.hexdigest()}


def read_inventory(stage, packages, progress=False):
    files = sorted(stage.glob("inventory*.jsonl"))
    require(files or progress, "missing independently enumerated Go inventory")
    cases = set()
    terminal = {}
    for path in files:
        for event in events(path, progress):
            pkg, action = event.get("Package"), event["Action"]
            if action == "build-fail":
                raise ValueError(f"Go inventory build failed for {event.get('ImportPath')}")
            name = event.get("Output", "").strip()
            if pkg and action == "output" and NAME.fullmatch(name):
                require((pkg, name) not in cases, f"duplicate inventory test {pkg}/{name}")
                cases.add((pkg, name))
            if pkg and action in TERMINAL and not event.get("Test"):
                require(pkg not in terminal, f"duplicate inventory package {pkg}")
                terminal[pkg] = action
    require("fail" not in terminal.values(), "inventory command reported failed packages")
    unexpected = set(terminal) - packages
    missing = packages - set(terminal)
    require(not unexpected, f"inventory reported unexpected packages: {sorted(unexpected)}")
    if missing and progress:
        print(f"inventory: {len(terminal)}/{len(packages)} packages enumerated", file=sys.stderr)
        return files, cases, terminal
    require(not missing, f"inventory package mismatch: {sorted(missing)}")
    return files, cases, terminal


def read_shards(stage, inventory):
    paths = sorted(stage.glob("trimmed-*.list"))
    require(paths, "missing trimmed test selections")
    trimmed = {key for key in inventory if key[0] == TRIMMED}
    consumed = set()
    lanes = {"packages": inventory - trimmed}
    for selection in paths:
        names = selection.read_text().splitlines()
        require(names and len(names) == len(set(names)), f"empty or duplicate selection: {selection}")
        cases = {(TRIMMED, name) for name in names}
        require(not (consumed & cases), f"overlapping trimmed selections: {selection}")
        consumed.update(cases)
        lanes[selection.stem] = cases
    require(consumed == trimmed, f"trimmed selections disagree with inventory: {sorted(consumed ^ trimmed)}")
    return paths, lanes


def read_lane(stage, lane, cases, expected_packages, progress):
    path = stage / f"{lane}.jsonl"
    observed = {}
    packages = {}
    everything = collections.Counter()
    if path.exists() or not progress:
        for event in events(path, progress):
            action, pkg, name = event["Action"], event.get("Package"), event.get("Test")
            if action == "build-fail":
                raise ValueError(f"{lane}: Go build failed for {event.get('ImportPath')}")
            if action not in TERMINAL:
                continue
            if name:
                everything[action] += 1
                if "/" not in name:
                    require((pkg, name) not in observed, f"{lane}: duplicate test {pkg}/{name}")
                    observed[pkg, name] = action
            elif pkg:
                require(pkg not in packages, f"{lane}: duplicate package {pkg}")
                packages[pkg] = action
    require(not (set(observed) - cases), f"{lane}: tests outside inventory {sorted(set(observed) - cases)}")
    require(not (set(packages) - set(expected_packages)), f"{lane}: packages outside inventory")
    missing = cases - set(observed)
    counts = collections.Counter(observed.values())
    print(f"{lane}: {dict(counts)}, {len(missing)}/{len(cases)} cases and "
          f"{len(expected_packages) - len(packages)} packages pending", file=sys.stderr)
    require(not everything["fail"] and "fail" not in packages.values(), f"{lane}: failures in Go run")
    if progress:
        return None
    require(not missing, f"{lane}: missing tests: {sorted(missing)}")
    require(packages == expected_packages, f"{lane}: package exit statuses or inventory disagree")
    return {
        **artifact(path),
        "topLevel": dict(counts),
        "includingSubtests": dict(everything),
        "terminalPackages": packages,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("stage", type=Path)
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--progress", action="store_true")
    action.add_argument("--output", type=Path, help="write a manifest only for a complete successful run")
    opts = parser.parse_args()
    stage = opts.stage.resolve()
    package_list = stage / "packages.list"
    if opts.progress and stage.is_dir() and not package_list.is_file():
        print("inventory: preparing the source and Go package list", file=sys.stderr)
        return
    pkgs = package_list.read_text().splitlines()
    require(TRIMMED in pkgs and len(set(pkgs)) == len(pkgs), "invalid Go package list")
    files, cases, packages = read_inventory(stage, set(pkgs), opts.progress)
    if opts.progress:
        if set(packages) != set(pkgs):
            return
        if not (stage / "run-commands.json").is_file() and not (stage / "run.sh").is_file():
            print("inventory complete: preparing Go lanes", file=sys.stderr)
            return
    schedules, lanes = read_shards(stage, cases)
    status_path = stage / "status.tsv"
    statuses = {}
    if status_path.exists() or not opts.progress:
        for line in status_path.read_text().splitlines():
            lane, state = line.split()
            require(lane not in statuses and lane in lanes, f"unexpected lane status {lane}")
            statuses[lane] = int(state)
    require(not any(statuses.values()), f"nonzero lane status: {statuses}")
    reports = {}
    for lane, expected in lanes.items():
        active_pkgs = {p: status for p, status in packages.items() if (p == TRIMMED) == (lane != "packages")}
        reports[lane] = read_lane(stage, lane, expected, active_pkgs, opts.progress)
    if opts.progress:
        return
    require(set(statuses) == set(lanes), f"missing final lane exit status: {sorted(set(lanes) - set(statuses))}")
    revision = (stage / "revision").read_text().strip()
    require(re.fullmatch(r"[0-9a-f]{40}(?:[0-9a-f]{24})?", revision), "invalid Git revision")
    launcher = stage / "run-commands.json"
    source_input = None
    if launcher.is_file():
        run = json.loads(launcher.read_text())
        mode = run.get("mode")
        commands = run.get("commands")
        require(mode in {"ordinary", "race"} and isinstance(commands, dict) and set(commands) == set(lanes),
                "invalid qualification command record")
        for command in commands.values():
            require(isinstance(command, list) and command[:2] == ["go", "test"] and
                    ("-race" in command) == (mode == "race") and "-count=1" in command and "-json" in command,
                    "recorded Go lane command disagrees with qualification mode")
        source_input = run.get("sourceInput")
        if source_input is not None:
            require(isinstance(source_input, dict) and source_input.get("kind") == "git archive" and
                    re.fullmatch(r"[0-9a-f]{64}", str(source_input.get("sha256", ""))),
                    "invalid qualification source archive record")
    else:
        launcher = stage / "run.sh"
        mode = "race" if "go test -race " in launcher.read_text() else "ordinary"
    total = collections.Counter()
    trimmed = collections.Counter()
    for lane, record in reports.items():
        total.update(record["topLevel"])
        if lane != "packages":
            trimmed.update(record["topLevel"])
    result = {
        "snapshot": revision,
        "mode": mode,
        "toolchain": (stage / "toolchain.txt").read_text().strip(),
        "summary": {
            "packagesPassed": list(packages.values()).count("pass"),
            "packagesWithoutTests": list(packages.values()).count("skip"),
            "topLevelInventory": len(cases),
            "topLevelPasses": total["pass"],
            "topLevelSkips": total["skip"],
            "trimmedPasses": trimmed["pass"],
            "trimmedSkips": trimmed["skip"],
        },
        "sourceArtifacts": [artifact(p) for p in [package_list, status_path, launcher, *files, *schedules]],
        "lanes": reports,
    }
    if source_input is not None:
        result["sourceInput"] = source_input
    opts.output.write_text(json.dumps(result, indent=2) + "\n")
    print(result["summary"])


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        sys.exit(str(error))

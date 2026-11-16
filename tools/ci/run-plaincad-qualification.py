#!/usr/bin/env python3
"""Run all eight PlainCAD gates against an immutable PlainSolid revision."""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile


KERNEL = Path(__file__).resolve().parents[2]
APP_REVISION = "efae4d2ffa6065875d3a76fc5926f358b5936a29"
DRIVER = "node_modules/playwright-core/lib/coreBundle.js"
DRIVER_SHA = "549070af3acabb3efcc4f55bfe6210f9f7c2fcf633cf7eaa59bfe60719969171"
PATCHES = ("plaincad-fastener-preset-locator.patch", "plaincad-production-wasm-capture.patch")
GATES = ("wasm", "build", "types", "selected-kernel", "development", "production", "cross-browser", "ordinary-units")
BROWSER_COUNTS = {"development": 345, "production": 66, "cross-browser": 15}
UNIT_COUNTS = {"selected-kernel": 4, "ordinary-units": 1691}


def require(ok, message):
    if not ok:
        raise ValueError(message)


def artifact(path):
    with path.open("rb") as file:
        digest = hashlib.file_digest(file, "sha256").hexdigest()
    return {"path": str(path), "bytes": path.stat().st_size, "sha256": digest}


def git(directory, *args):
    return subprocess.check_output(["git", "-C", str(directory), *args])


def prepare(app, stage, revision, cache):
    require(not stage.is_relative_to(app), "the disposable stage must be outside the PlainCAD source checkout")
    require((app / DRIVER).is_file(), "PlainCAD requires preinstalled npm and Playwright dependencies")
    require(artifact(app / DRIVER)["sha256"] == DRIVER_SHA, "source Playwright driver differs from the qualified revision")
    require(git(app, "rev-parse", "--verify", APP_REVISION).decode().strip() == APP_REVISION,
            "pinned PlainCAD revision is missing")
    stage.mkdir(parents=True)
    with stage.joinpath("app-source.tar").open("wb") as archive:
        subprocess.run(["git", "-C", str(app), "archive", APP_REVISION], stdout=archive, check=True)
    with tarfile.open(stage / "app-source.tar") as archive:
        require(archive.pax_headers.get("comment") == APP_REVISION, "PlainCAD source archive has the wrong revision")
        archive.extractall(stage, filter="data")
    modules = stage / "node_modules"
    if sys.platform == "darwin":
        subprocess.run(["cp", "-cRp", str(app / "node_modules"), str(modules)], check=True)
    else:
        shutil.copytree(app / "node_modules", modules, symlinks=True)
    require(artifact(stage / DRIVER)["sha256"] == DRIVER_SHA, "staged Playwright driver does not match")
    inputs = stage / "qualification-input"
    inputs.mkdir()
    patches = []
    env = dict(os.environ, GIT_CEILING_DIRECTORIES=str(stage.parent))
    for name in PATCHES:
        path = inputs / name
        path.write_bytes(git(KERNEL, "show", f"{revision}:tools/plaincad/{name}"))
        subprocess.run(["git", "apply", "--no-index", str(path)], cwd=stage, env=env, check=True)
        subprocess.run(["git", "apply", "--no-index", "--reverse", "--check", str(path)], cwd=stage, env=env, check=True)
        patches.append(artifact(path))
    (stage / "kernel-revision").write_text(revision + "\n")
    (stage / "app-revision").write_text(APP_REVISION + "\n")
    commands = {
        "wasm": ["npm", "run", "kernel:plainsolid", "--", str(KERNEL), revision],
        "build": ["npm", "run", "build"],
        "types": ["npm", "run", "lint"],
        "selected-kernel": ["npm", "run", "test:plainsolid", "--", "--maxWorkers=1", "--reporter=json",
                            f"--outputFile={stage / 'selected-kernel.json'}"],
        "development": ["npm", "run", "test:browser", "--", "--reporter=list,json"],
        "production": ["npm", "run", "test:production", "--", "--reporter=list,json"],
        "cross-browser": ["node", "node_modules/@playwright/test/cli.js", "test", "-c",
                          "playwright.cross-browser.config.ts", "--reporter=list,json"],
        "ordinary-units": ["npm", "test", "--", "--maxWorkers=1", "--retry=2", "--testTimeout=15000",
                           "--reporter=json", f"--outputFile={stage / 'ordinary-units.json'}"],
    }
    base = {"CI": "1", "VITE_CAD_KERNEL": "plainsolid", "GOMAXPROCS": "2", "GOCACHE": str(cache)}
    command_env = {gate: {**base, **({"PLAYWRIGHT_JSON_OUTPUT_FILE": str(stage / f"{gate}.json")}
                                     if gate in BROWSER_COUNTS else {})} for gate in GATES}
    source = artifact(stage / "app-source.tar")
    (stage / "qualification-input.json").write_text(json.dumps({
        "appRevision": APP_REVISION, "kernelRevision": revision, "source": source,
        "driver": artifact(stage / DRIVER), "patches": patches, "commands": commands, "environment": command_env,
    }, indent=2) + "\n")
    for gate in GATES:
        print(f"PlainCAD: running {gate}", flush=True)
        with (stage / f"{gate}.log").open("wb") as log:
            result = subprocess.run(commands[gate], cwd=stage, env={**os.environ, **command_env[gate]},
                                    stdout=log, stderr=subprocess.STDOUT, check=False).returncode
        with (stage / "status.tsv").open("a") as log:
            log.write(f"{gate}\t{result}\n")
        require(gate not in ("wasm", "build") or result == 0, f"{gate} failed; see {stage / (gate + '.log')}")


def verify(stage, output):
    status = [s.split() for s in (stage / "status.tsv").read_text().splitlines()]
    require(len(status) == len(GATES) and dict(status) == dict.fromkeys(GATES, "0"), f"gate exits: {status}")
    app_revision = (stage / "app-revision").read_text().strip()
    kernel_revision = (stage / "kernel-revision").read_text().strip()
    require(app_revision == APP_REVISION, f"unqualified PlainCAD source revision {app_revision}")
    require(re.fullmatch(r"[a-f0-9]{40}", kernel_revision), "missing immutable PlainSolid revision")
    reports = {}
    for gate, count in BROWSER_COUNTS.items():
        p = stage / f"{gate}.json"
        stats = json.loads(p.read_text())["stats"]
        require(stats["expected"] == count and all(stats[k] == 0 for k in ("skipped", "unexpected", "flaky")),
                f"{gate} results differ from the gate: {stats}")
        reports[gate] = {**artifact(p), "summary": stats}
    for gate, count in UNIT_COUNTS.items():
        p = stage / f"{gate}.json"
        report = json.loads(p.read_text())
        want = dict(numTotalTests=count, numPassedTests=count, numFailedTests=0, numPendingTests=0)
        got = {key: report[key] for key in want}
        require(report["success"] and got == want, f"{gate} results differ from the gate: {got}")
        reports[gate] = {**artifact(p), "summary": got}
    wasm = artifact(stage / "src/cad/kernel/plainsolid-kernel.wasm")
    built = list((stage / "dist/assets").glob("plainsolid-kernel-*.wasm"))
    require(len(built) == 1, "production must contain exactly one PlainSolid WASM asset")
    production = artifact(built[0])
    require(wasm["sha256"] == production["sha256"], "production WASM does not match the staged kernel")
    driver = artifact(stage / DRIVER)
    require(driver["sha256"] == DRIVER_SHA, "Playwright driver changed")
    summary = re.search(r"JavaScript bundle budget passed \((\d+) bundles, largest ([\d.]+) kB; limit ([\d.]+) kB\)",
                        (stage / "build.log").read_text())
    require(summary is not None, "build did not report the JavaScript bundle budget")
    count, largest, limit = summary.groups()
    require(float(largest) <= float(limit), "production exceeded the reported JavaScript bundle budget")
    manifest = {
        "kernelRevision": kernel_revision, "appRevision": app_revision,
        "checkpointAt": datetime.datetime.now().astimezone().isoformat(timespec="seconds"),
        "tools": {"go": subprocess.check_output(["go", "version"], text=True).strip(),
                  "node": subprocess.check_output(["node", "--version"], text=True).strip(),
                  "playwright": json.loads((stage / "node_modules/playwright/package.json").read_text())["version"]},
        "exits": {k: int(v) for k, v in status}, "reports": reports, "wasm": wasm, "productionAsset": production,
        "driver": driver, "bundle": {"bundles": int(count), "largestKB": float(largest), "limitKB": float(limit)},
        "logs": {gate: artifact(stage / f"{gate}.log") for gate in GATES},
    }
    inputs = stage / "qualification-input.json"
    if inputs.is_file():
        data = json.loads(inputs.read_text())
        require(data["appRevision"] == app_revision and data["kernelRevision"] == kernel_revision
                and data["driver"] == driver and data["source"] == artifact(stage / "app-source.tar"),
                "immutable stage inputs changed")
        require(data["patches"] == [artifact(stage / "qualification-input" / name) for name in PATCHES],
                "staged test patches changed")
        for name, recorded in zip(PATCHES, data["patches"], strict=True):
            require(recorded["sha256"] == hashlib.sha256(git(KERNEL, "show", f"{kernel_revision}:tools/plaincad/{name}")).hexdigest(),
                    f"staged patch does not match the kernel revision: {name}")
        manifest["inputRecord"] = artifact(inputs)
        manifest["sourceArchive"] = data["source"]
        manifest["testOnlyPatches"] = data["patches"]
    else:
        # Earlier stages used a checksum list in place of the structured input record.
        entries = {}
        for line in (stage / "test-patches-and-driver.sha256").read_text().splitlines():
            digest, file = line.split(maxsplit=1)
            p = KERNEL / file
            info = artifact(p)
            require(digest == info["sha256"] and p.name not in entries, f"legacy staged input changed: {p}")
            entries[p.name] = info
        require(set(entries) == {*PATCHES, Path(DRIVER).name}, "incomplete legacy input record")
        manifest["testOnlyPatches"] = [entries[name] for name in PATCHES]
    with output.open("x") as file:
        file.write(json.dumps(manifest, indent=2) + "\n")
    print(f"PlainCAD: eight gates passed on {kernel_revision}", flush=True)


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("stage", type=Path, help="disposable PlainCAD copy")
    cli.add_argument("output", type=Path, help="new manifest path")
    cli.add_argument("--app", type=Path, help="PlainCAD Git checkout with preinstalled dependencies")
    cli.add_argument("--kernel-revision", default="HEAD", help="immutable commit to build; defaults to current HEAD")
    cli.add_argument("--cache", type=Path, help="Go build cache directory")
    cli.add_argument("--verify-only", action="store_true", help="reconcile an existing sealed run")
    args = cli.parse_args()
    stage, output = args.stage.resolve(), args.output.resolve()
    require(not output.exists(), f"refusing to overwrite {output}")
    if not args.verify_only:
        require(args.app is not None, "--app is required for a fresh run")
        revision = git(KERNEL, "rev-parse", "--verify", "--end-of-options", args.kernel_revision + "^{commit}").decode().strip()
        cache = args.cache.resolve() if args.cache else stage.parent / ".plaincad-go-cache"
        prepare(args.app.resolve(), stage, revision, cache)
    verify(stage, output)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, KeyError, subprocess.CalledProcessError, tarfile.TarError) as exc:
        sys.exit(f"PlainCAD qualification: {exc}")

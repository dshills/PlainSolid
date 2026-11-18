#!/usr/bin/env python3
"""Run and reconcile a complete, sharded Go qualification of a Git commit."""

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile

TRIMMED = "plainsolid/brep/trimmed"
NAME = re.compile(r"(?:Test|Fuzz|Example)[A-Za-z0-9_]*")


def run(args, directory, env, log=None):
    if log is None:
        return subprocess.check_output(args, cwd=directory, env=env, text=True)
    with log.open("wb") as output:
        return subprocess.run(args, cwd=directory, env=env, stdout=output,
                              stderr=subprocess.STDOUT, check=False).returncode


def archive(revision, repo, source):
    with subprocess.Popen(["git", "archive", revision], cwd=repo, stdout=subprocess.PIPE) as pack:
        try:
            unpack = subprocess.run(["tar", "-x", "-C", str(source)], stdin=pack.stdout, check=False)
        finally:
            pack.stdout.close()
        if unpack.returncode or pack.wait():
            raise ValueError("could not archive the requested commit")


def unpack_archive(revision, path, source):
    with path.open("rb") as raw:
        sha256 = hashlib.file_digest(raw, "sha256").hexdigest()
        raw.seek(0)
        with tarfile.open(fileobj=raw, mode="r:") as pack:
            if pack.pax_headers.get("comment") != revision:
                raise ValueError("archive commit does not match the requested revision")
            pack.extractall(source, filter="data")
    return {"kind": "git archive", "sha256": sha256}


def trimmed_inventory(path):
    names = []
    for line in path.read_text().splitlines():
        e = json.loads(line)
        name = e.get("Output", "").strip()
        if e.get("Package") == TRIMMED and e.get("Action") == "output" and NAME.fullmatch(name):
            names.append(name)
    if not names or len(names) != len(set(names)):
        raise ValueError("missing or duplicate trimmed test names")
    return names


def schedule(names, count, prior):
    seconds = {}
    if prior:
        for path in sorted(prior.glob("trimmed-*.jsonl")):
            for line in path.read_text().splitlines():
                e = json.loads(line)
                name = e.get("Test", "")
                if e.get("Package") == TRIMMED and e.get("Action") in {"pass", "skip"} and NAME.fullmatch(name):
                    seconds[name] = max(.02, e.get("Elapsed", 0))
    selections, totals = ([[] for _ in range(count)], [0.] * count)
    for name in sorted(names, key=lambda n: (-seconds.get(n, 8.), n)):
        i = min(range(count), key=lambda i: totals[i])
        selections[i].append(name)
        totals[i] += seconds.get(name, 8.)
    return selections


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("revision", help="Git commit, tag or branch (full commit hash with --archive)")
    ap.add_argument("stage", type=Path, help="new directory for source, logs and report")
    ap.add_argument("--archive", type=Path, help="trusted tar from git archive, for execution outside the checkout")
    ap.add_argument("--race", action="store_true")
    ap.add_argument("--shards", type=int, default=8)
    ap.add_argument("--jobs", type=int, default=min(9, os.cpu_count() or 1), help="parallel lanes")
    ap.add_argument("--go-workers", type=int, default=2, help="Go scheduler threads per lane")
    ap.add_argument("--cache", type=Path, help="Go build cache (default: GOCACHE or a shared cache beside the stage)")
    ap.add_argument("--prior", type=Path, help="completed stage used only for estimating shard durations")
    ap.add_argument("--timeout", default="240m", help="go test timeout per package")
    opts = ap.parse_args()
    if not 0 < opts.shards <= 32 or not 0 < opts.jobs <= 33 or not 0 < opts.go_workers <= 32:
        ap.error("shards, jobs and Go workers must be positive and bounded")
    parent = opts.stage.resolve()
    env = os.environ.copy()
    for key in ("GOOS", "GOARCH", "GOFLAGS"):
        env.pop(key, None)
    # Qualification covers the slow tier that a plain `go test` skips.
    env.update(GOMAXPROCS=str(opts.go_workers), CGO_ENABLED="1" if opts.race else "0", PLAINSOLID_SLOW="1")
    if opts.cache is not None:
        env["GOCACHE"] = str(opts.cache.resolve())
    elif not env.get("GOCACHE"):
        env["GOCACHE"] = str(parent.parent / ".go-qualification-cache")
    if opts.archive:
        revision = opts.revision
        if not re.fullmatch(r"[a-f0-9]{40}(?:[a-f0-9]{24})?", revision):
            ap.error("--archive requires a full Git commit hash")
    else:
        repo = Path(run(["git", "rev-parse", "--show-toplevel"], Path.cwd(), env).strip())
        revision = run(["git", "rev-parse", "--verify", "--end-of-options", f"{opts.revision}^{{commit}}"], repo, env).strip()
    parent.mkdir()
    source = parent / "src"
    source.mkdir()
    (parent / "revision").write_text(revision + "\n")
    source_input = unpack_archive(revision, opts.archive, source) if opts.archive else None
    if source_input is None:
        archive(revision, repo, source)
    version = run(["go", "version"], source, env)
    (parent / "toolchain.txt").write_text(version + f"{platform.system()} {platform.machine()}\n")
    packages = run(["go", "list", "./..."], source, env).splitlines()
    others = [p for p in packages if p != TRIMMED]
    if TRIMMED not in packages or not others:
        raise ValueError("qualification requires the trimmed package and the other PlainSolid packages")
    (parent / "packages.list").write_text("\n".join(packages) + "\n")
    inventory = ["go", "test", "-json", "-list", "."]
    with ThreadPoolExecutor(max_workers=min(2, opts.jobs)) as executor:
        results = [executor.submit(run, inventory + pkgs, source, env, parent / name) for name, pkgs in
                   (("inventory.jsonl", others), ("inventory-trimmed.jsonl", [TRIMMED]))]
        if any([job.result() != 0 for job in results]):
            raise ValueError(f"Go inventory failed; see {parent}")
    names = trimmed_inventory(parent / "inventory-trimmed.jsonl")
    selections = schedule(names, min(opts.shards, len(names)), opts.prior)
    base = ["go", "test"] + (["-race"] if opts.race else []) + ["-count=1", "-json", f"-timeout={opts.timeout}"]
    lanes = {"packages": base + ["-p", "2", *others]}
    for i, split in enumerate(selections):
        lane = f"trimmed-{i}"
        split.sort()
        (parent / f"{lane}.list").write_text("\n".join(split) + "\n")
        lanes[lane] = base + ["-p", "1", "./brep/trimmed", "-run", "^(" + "|".join(split) + ")$"]
    mode = "race" if opts.race else "ordinary"
    go_env = {name: env[name] for name in ("GOCACHE", "CGO_ENABLED", "GOMAXPROCS")}
    run_record = {"mode": mode, "commands": lanes, "goEnvironment": go_env}
    if source_input:
        run_record["sourceInput"] = source_input
    (parent / "run-commands.json").write_text(json.dumps(run_record, indent=2) + "\n")
    rc = 0
    with ThreadPoolExecutor(max_workers=opts.jobs) as executor, (parent / "status.tsv").open("w") as status:
        waiting = {executor.submit(run, argv, source, env, parent / f"{lane}.jsonl"): lane for lane, argv in lanes.items()}
        for result in as_completed(waiting):
            lane, code = waiting[result], result.result()
            status.write(f"{lane}\t{code}\n")
            status.flush()
            print(f"{lane}: {code}", flush=True)
            rc = rc or code
    if rc:
        raise ValueError(f"Go qualification failed; see {parent}")
    verifier = Path(__file__).resolve().with_name("verify-go-evidence.py")
    subprocess.run([sys.executable, str(verifier), str(parent), "--output", str(parent / "report.json")], check=True)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError, tarfile.TarError) as error:
        sys.exit(str(error))

# Go CI and qualification

Go CI runs on pull requests to `main`, pushes to `main`, and manual dispatch.
The Linux and Windows jobs compile all packages and run bounded tests selected
in [smoke.sh](smoke.sh). Stable Go on Linux also checks formatting, vet and the
PlainCAD WASM build. Superseded runs for the same pull request or ref are
canceled. Each job has an eight-minute timeout.

```sh
bash tools/ci/smoke.sh
```

Go qualification runs the full platform, toolchain, race and fuzz matrix on
demand. From GitHub Actions select **Go qualification**, then **Run workflow**
and select the ref to qualify. Qualify a release using its exact source commit.

```sh
gh workflow run go-qualification.yml --ref main
```

## Test tiers

A plain `go test ./...` runs the fast tier: every test except those whose
first statement is `slowtest.Skip(t)` (package `plainsolid/internal/slowtest`).
`PLAINSOLID_SLOW=1` enables the rest. The Go qualification workflow and
`run-go-qualification.py` set it, so both run the complete suite. `smoke.sh`
also sets it so that each case it names runs whichever tier it is in. The
fast tier takes about five minutes on a 14-core machine; the complete suite is
several hours of test time.

Use the fast tier for each change and the complete suite before a release or
when changing the code a slow test covers. To run one slow test:

```sh
PLAINSOLID_SLOW=1 go test ./brep/trimmed -run '^TestNestedQuarticTubeMaterial$'
```

A new test that takes about two seconds or more should call
`slowtest.Skip(t)` first. To re-tier from measurements, feed `go test -json`
output from a complete run to the marker; it only adds marks and is safe to
rerun. Fuzz targets are never marked.

```sh
python3 tools/ci/mark-slow-tests.py "$stage"/packages.jsonl "$stage"/trimmed-*.jsonl
python3 tools/ci/mark-slow-tests.py timings.jsonl --seconds 5
```

## Full test inventory and local evidence

`test-shard.sh` selects disjoint top-level tests, examples and fuzz seeds
from a package. Selecting a parent includes all its subtests. GitHub
Actions gives `exchange` and `exchange/step` dedicated shards so their
ordinary and race runs have separate package timeouts. The four STEP
shards also keep the test filter well within Windows' command line limit.
`trimmed-shard.sh` uses the same selector for the trimmed-solid suite and
its separate long cases. Fuzz jobs use `-run=^$` and an exact target name
so that ordinary tests run only in their assigned jobs.

The regular trimmed-solid suite tests Go 1.23 on Apple Silicon and the
current stable Go release on Intel macOS. The Intel runner avoids a
reproducible Go 1.27.4 garbage collection crash on macOS ARM64 while
keeping the regular suite's test inventory and race coverage. The other
macOS jobs continue to run on Apple Silicon with both Go versions.

An independent `go test -json -list '^(Test|Fuzz|Example)'` run
establishes the expected cases for local qualification.

Run all packages from an immutable archive of a committed tree. The launcher
uses previous test timings only to balance shards; the new independent
inventory determines every case. It writes `report.json` only after full
reconciliation. The output directory must be new.

```sh
python3 tools/ci/run-go-qualification.py HEAD .worktrees/qualification/go-current
python3 tools/ci/run-go-qualification.py HEAD .worktrees/qualification/go-current-race \
  --race --prior .worktrees/qualification/go-current --jobs 6
```

To qualify the same committed tree on another platform without transferring
the checkout or its history, run `git archive --format=tar <full-commit>`
and transfer the trusted tar and the two Python scripts. With Python 3.12+
use `run-go-qualification.py <full-commit> <new-stage> --archive <tar>`.
The launcher checks the Git commit recorded in the tar header and records
the archive SHA-256 with the run commands.

For an immutable qualification workspace, keep the following together:

- `revision`, `toolchain.txt`, and the recorded Go argument lists in
  `run-commands.json` (or the historical shell launcher `run.sh`).
- `packages.list` from `go list ./...` and one or more
  `inventory*.jsonl` listings that cover every package once.
- `packages.jsonl` for all packages except `plainsolid/brep/trimmed`,
  and a `trimmed-*.list` / `trimmed-*.jsonl` pair per disjoint group.
- `status.tsv` with one tab-delimited lane name and exit code per
  completed lane. The ordinary lane is named `packages`.

The verifier rejects duplicate, extra, missing and failed cases and mismatched
package statuses. It keeps skipped tests distinct from passes and only writes
a manifest after every required lane has exited successfully.

```sh
python3 tools/ci/verify-go-evidence.py "$stage" --progress
python3 tools/ci/verify-go-evidence.py "$stage" --output "$stage/manifest.json"
```

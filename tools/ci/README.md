# Go qualification evidence

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

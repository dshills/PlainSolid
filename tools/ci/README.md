# Go qualification evidence

`trimmed-shard.sh` selects disjoint top-level trimmed-solid tests. The
same parent selection includes all its subtests. An independent `go test
-json -list '^(Test|Fuzz|Example)'` run establishes the expected cases.

Run all packages from an immutable archive of a committed tree. The launcher
uses previous test timings only to balance shards; the new independent
inventory determines every case. It writes `report.json` only after full
reconciliation. The output directory must be new.

```sh
python3 tools/ci/run-go-qualification.py HEAD .worktrees/qualification/go-current
python3 tools/ci/run-go-qualification.py HEAD .worktrees/qualification/go-current-race \
  --race --prior .worktrees/qualification/go-current --jobs 6
```

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

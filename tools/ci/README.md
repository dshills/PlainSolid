# Go qualification evidence

`trimmed-shard.sh` selects disjoint top-level trimmed-solid tests. The
same parent selection includes all its subtests. An independent `go test
-json -list '^(Test|Fuzz|Example)'` run establishes the expected cases.

For an immutable qualification workspace, keep the following together:

- `revision`, `toolchain.txt`, and the actual test launcher `run.sh`.
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

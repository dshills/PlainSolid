# Original development acceptance stage

After `run.mjs` has staged the app and passed its typecheck and real Go/WASM
backend probe, verify or run original development tests without rebuilding:

```sh
node tools/plaincad/development.mjs --work /tmp/staged-app --check
node tools/plaincad/development.mjs --work /tmp/staged-app --run
```

Default port 5279 preserves fixture-local storage origins. The runner obtains
the original Playwright config from the pinned app revision and changes only
its port when explicitly requested. Both stages refuse a custom port for
selected fixed-origin fixtures. `--test` is a file regular expression; `--grep` selects
test titles. All original `.spec`/`.test` files are selected by default; the
extra generated backend probe is excluded. Original assertions, timeouts,
worker count and retry policy are preserved. Run expensive qualification jobs
sequentially to avoid resource contention.

Every staged source hash, supported runtime adaptation, adapter and WASM is
verified before launch. The existing production provenance module replays the
exact display patches against pinned originals. A preflight staged by a different
patch version must be restaged, not silently accepted. Output includes
`development-results.json`, `development-tests.log` and
`development-readiness.json` with per-case results, source/config/runner hashes
and runtime version. Zero cases, skipped cases, failures and retries that did
not all pass do not qualify as a pass. A selected run does not qualify other
tests. `--check` creates no runner configuration and launches no browser.

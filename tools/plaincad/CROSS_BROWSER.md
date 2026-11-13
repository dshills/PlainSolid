# Original cross-browser acceptance stage

Consume a passed Go/WASM preflight from `run.mjs`:

```sh
node tools/plaincad/cross-browser.mjs --work /tmp/staged-app --check
node tools/plaincad/cross-browser.mjs --work /tmp/staged-app --run
```

The default port is the original 5281. The original committed
`playwright.cross-browser.config.ts` supplies Chromium, Firefox and WebKit,
worker count, deadlines, retries and storage state. Only an explicitly
requested port changes the derived configuration; selected fixed-origin
fixtures forbid a nondefault port. `--test`, `--grep` and `--project` permit
narrow qualification and are recorded. A selected project pass never qualifies
other engines. The caller must install browser dependencies separately.

Before running, the stage verifies original fixtures, supported provider/runtime
adaptations, adapter, WASM and configuration using the shared provenance helpers.
`--run` builds the production app, confirms the deployed WASM hash, and then
runs original browser assertions. No browser is launched by `--check`.

Artifacts are `cross-browser-build.log`, `cross-browser-tests.log`,
`cross-browser-results.json` and `cross-browser-readiness.json`. Reports retain
project identities, expected status, annotations, each attempt and errors,
source/runner/config/adapter/WASM hashes and runtime identity. Missing engines,
skips, failures, unexpected passes and failed retries cannot become an overall
pass. Missing browser executables remain failures, not ignored projects.

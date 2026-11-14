# Direct suite delegation

`run.mjs --suite production` and `--suite cross-browser` stage/typecheck the app
and run the real backend probe, then delegate to `production.mjs` or
`cross-browser.mjs`. Their original config verification, runtime replay,
fixed-origin restrictions and failure policies are authoritative.

`--port` means acceptance port: defaults are 5280 (production), 5281
(cross-browser), and 5279 (development). Nondevelopment suites independently
use 5279 for their development backend probe; `--preflight-port` can select a
free probe port. For example:

```sh
node tools/plaincad/run.mjs --source /path/to/PlainCAD --work /tmp/cross-check \
  --suite cross-browser --port 5427 --preflight-port 5428
```

No fixture origins are rewritten. The second stage can reject a requested port
when selected originals contain fixed origins. Development preflight and
acceptance share one port. The child suite report remains separate, and final
`readiness.json` contains both probe and acceptance cases plus the child report
hash. Failure to run or qualify the delegated suite remains failure even when
the preflight passed. No browser engine is silently skipped.

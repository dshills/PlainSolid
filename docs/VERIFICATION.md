# Verification record

This page says how PlainSolid is verified and what the last complete runs
showed. Edit it in place; do not append dated entries. The per-feature
narrative that this page replaced is in git:
`git show c35d3358:docs/VERIFICATION.md`.

## How to verify

| When | Command | Cost |
|---|---|---|
| Every change | `go build ./... && go vet ./...`, then `go test` on the changed packages | Seconds to minutes |
| Before merging to `main` | `go test ./...` (the default tier), then `go build -tags research ./... && go vet -tags research ./...` | Under two minutes on a 14-core machine |
| A slow test your change touches | `PLAINSOLID_SLOW=1 go test -tags research ./pkg -run '^TestName$'` | Seconds to minutes each |
| Before a release, or when the user asks | `python3 tools/ci/run-go-qualification.py HEAD <new-stage>`, with `--race` for the race run | Hours of machine time |
| Before a release, or when the user asks | `python3 tools/ci/run-plaincad-qualification.py <stage> <manifest> --app <PlainCAD checkout>`; see [the PlainCAD harness](../tools/plaincad/README.md) | The eight PlainCAD gates |
| On demand | The **Go qualification** GitHub workflow | Full platform, toolchain, race and fuzz matrix |

The tiers, shard scripts, evidence layout and GitHub workflows are described
in [the CI notes](../tools/ci/README.md). The optional pinned OCCT oracle,
which the `PLAINSOLID_OCCT_*` tests need, is described in
[its README](../tools/occtref/README.md); without it those tests skip, which
accounts for most of the optional skips below.

## Last complete runs

| Run | Kernel commit | Platform | Result | Evidence |
|---|---|---|---|---|
| Go, ordinary | `9f95a188`, 2026-11-16 | macOS ARM64, Go 1.27.4 | 6,121 passed, 608 optional skips, 0 failed of 6,729 inventoried; 54 packages passed, 8 have no tests | [JSON](evidence/main-9f95a188-qualification-2026-11-16.json) |
| PlainCAD, app `efae4d2` | `9f95a188`, 2026-11-16 | Chromium, Firefox and WebKit | All eight gates: 345 development, 66 production, 15 cross-browser, 4 WASM contract, 1,691 unit | [JSON](evidence/plaincad-main-9f95a188-2026-11-16.json) |
| Go, ordinary | `c7afb429`, 2026-11-16 | Linux ARM64, Go 1.26.0 | 6,018 passed, 599 optional skips, 0 failed of 6,617 inventoried | [JSON](evidence/native-linux-c7afb429-qualification-2026-11-16.json) |
| Go, race detector | `ab33a412`, 2026-11-15 | macOS ARM64, Go 1.27.4 | 5,557 passed, 580 optional skips, 0 failed of 6,137 inventoried | [JSON](evidence/native-main-ab33a412-race-2026-11-15.json) |

Vet and the Linux and Windows amd64 cross-builds passed on the `9f95a188`
archive. The eight PlainCAD gates are the WASM build, the app build, the
TypeScript checks, the selected-kernel check, the development, production and
cross-browser suites, and the ordinary unit suite.

Since those runs, only the fast tier has been run: 5,293 passed, 1,615 skipped
and 0 failed at `c35d3358` on 2026-11-18 (macOS ARM64). The 1,615 skips are
the 991 tests in the slow tier plus the tests that need the OCCT oracle or
another opt-in setting.

## What these results do not establish

- Passing tests do not establish OCCT parity. Coverage is supporting evidence,
  never proof of geometric correctness.
- No complete run exists for the current commit, for native Windows, or for
  Go versions other than those listed.
- Each qualified operation is qualified on a stated input domain. A passing
  example does not widen that domain.
- OCCT comparisons cover selected modeling and exchange cases, and OCCT output
  is not an infallible oracle.
- The release requirements in [the specification](SPECIFICATION.md), including
  the adversarial corpus, numeric envelope and exchange security review, are
  open.

## Keeping this record

Update the table only after a complete run, and replace the row it supersedes.
Keep one evidence file per row under `docs/evidence` and remove the file it
replaces. The record of an ordinary change is its commit message: what
changed, what was run, and what was not.

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
| Go, ordinary, with `-tags research` | `ecaae04d`, 2026-11-18 | macOS ARM64, Go 1.23.12 | 6,285 passed, 624 optional skips, 0 failed of 6,909 inventoried; 55 packages passed, 9 have no tests | [JSON](evidence/main-ecaae04d-qualification-2026-11-18.json) |
| PlainCAD, app `efae4d2` | `9f95a188`, 2026-11-16 | Chromium, Firefox and WebKit | All eight gates: 345 development, 66 production, 15 cross-browser, 4 WASM contract, 1,691 unit | [JSON](evidence/plaincad-main-9f95a188-2026-11-16.json) |
| Go, ordinary | `c7afb429`, 2026-11-16 | Linux ARM64, Go 1.26.0 | 6,018 passed, 599 optional skips, 0 failed of 6,617 inventoried | [JSON](evidence/native-linux-c7afb429-qualification-2026-11-16.json) |
| Go, race detector | `ab33a412`, 2026-11-15 | macOS ARM64, Go 1.27.4 | 5,557 passed, 580 optional skips, 0 failed of 6,137 inventoried | [JSON](evidence/native-main-ab33a412-race-2026-11-15.json) |

The `ecaae04d` run is the first complete run since the research quarantine
and covers both halves: it sets `PLAINSOLID_SLOW=1` and `-tags research`. It
took 39 minutes of wall time on four lanes, and 9,220 s of package test time.
It used Go 1.23.12 on purpose, because Go 1.25 and later can abort the
`brep/trimmed` test binary on this platform (see the known defects on
[the status page](STATUS.md)); a complete run under the current Go release has
not been made since `9f95a188`. Vet and the Linux and Windows amd64
cross-builds last passed on the `9f95a188` archive and were not repeated. The eight PlainCAD gates are the WASM build, the app build, the
TypeScript checks, the selected-kernel check, the development, production and
cross-browser suites, and the ordinary unit suite.

The PlainCAD, Linux and race rows predate the quarantine and the parallel
tests in `exchange/step`. The default tier (`go test ./...`, Go 1.27.4) last
passed at the quarantine commit `6e5d1bda`: 4,781 passed, 1,217 skipped, 0
failed, in 78 s.

## What these results do not establish

- Passing tests do not establish OCCT parity. Coverage is supporting evidence,
  never proof of geometric correctness.
- No complete run exists under the current Go release since `9f95a188`, for
  native Windows, or for Go versions other than those listed. The PlainCAD
  suites have not been rerun since `9f95a188`.
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

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
| Go, ordinary, with `-tags research` | `93099dfe`, 2026-11-19 | macOS ARM64, Go 1.23.12 | 6,359 passed, 625 optional skips, 0 failed of 6,984 inventoried; 57 packages passed, 9 have no tests | [JSON](evidence/main-93099dfe-qualification-2026-11-19.json) |
| PlainCAD, app `efae4d2` | `b841cc75`, 2026-11-19 | Chromium, Firefox and WebKit; WASM built with Go 1.27.4 | All eight gates: 345 development, 66 production, 15 cross-browser, 4 WASM contract, 1,691 unit; none skipped, failed or flaky | [JSON](evidence/plaincad-main-b841cc75-2026-11-19.json) |
| Go, ordinary | `c7afb429`, 2026-11-16 | Linux ARM64, Go 1.26.0 | 6,018 passed, 599 optional skips, 0 failed of 6,617 inventoried | [JSON](evidence/native-linux-c7afb429-qualification-2026-11-16.json) |
| Go, race detector | `ab33a412`, 2026-11-15 | macOS ARM64, Go 1.27.4 | 5,557 passed, 580 optional skips, 0 failed of 6,137 inventoried | [JSON](evidence/native-main-ab33a412-race-2026-11-15.json) |

The first Go row and the PlainCAD row cover the Go compiler mitigation (see
the known defects on [the status page](STATUS.md)), the general Boolean of
`brep/boolean`, the band mesher of `brep/trimmed`, the STEP writer and the
STEP reader for boundaries of planes and cylinders, and the bridge's fallback
to the general Boolean where a legacy path declines a request. `93099dfe`
differs from `b841cc75`, the PlainCAD row, only in documents. Neither row
covers the five changes after them that [the status page](STATUS.md) lists,
the last of them `494b1a47`. The Go run sets
`PLAINSOLID_SLOW=1` and `-tags research`, so it covers the slow tier and the
research quarantine; it took 50 minutes of wall time on four lanes. It used Go
1.23.12 on purpose, because Go 1.25 and later can abort the research tests of
`brep/trimmed` on this platform; a complete Go run under the current Go
release has not been made since `9f95a188`. The PlainCAD run builds the WASM
kernel with Go 1.27.4, the release in use. Vet and the
Linux and Windows amd64 cross-builds last passed on the `9f95a188` archive and
were not repeated. The eight PlainCAD gates are the WASM build, the app build,
the TypeScript checks, the selected-kernel check, the development, production
and cross-browser suites, and the ordinary unit suite.

The Linux and race rows predate the quarantine, the parallel tests in
`exchange/step` and the mitigation. The default tier (`go test ./...`, Go
1.27.4) passes at `2c4d38c7` on all 58 packages with tests.

## What these results do not establish

- Passing tests do not establish OCCT parity. Coverage is supporting evidence,
  never proof of geometric correctness.
- No complete Go run exists under the current Go release since `9f95a188`,
  for native Windows, or for Go versions other than those listed.
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

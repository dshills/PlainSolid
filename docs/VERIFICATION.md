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
| Go, ordinary, with `-tags research` | `b26779db`, 2026-11-21 | macOS ARM64, Go 1.23.12 | 6,308 passed, 625 optional skips, 0 failed of 6,933 inventoried; 60 packages passed, 9 have no tests | [JSON](evidence/main-b26779db-qualification-2026-11-21.json) |
| PlainCAD, app `efae4d2` | `b26779db`, 2026-11-21 | Chromium, Firefox and WebKit; WASM built with Go 1.27.4 | All eight gates: 345 development, 66 production, 15 cross-browser, 4 WASM contract, 1,691 unit; none skipped, failed or flaky | [JSON](evidence/plaincad-main-b26779db-2026-11-21.json) |
| Go, ordinary | `c7afb429`, 2026-11-16 | Linux ARM64, Go 1.26.0 | 6,018 passed, 599 optional skips, 0 failed of 6,617 inventoried | [JSON](evidence/native-linux-c7afb429-qualification-2026-11-16.json) |
| Go, race detector | `ab33a412`, 2026-11-15 | macOS ARM64, Go 1.27.4 | 5,557 passed, 580 optional skips, 0 failed of 6,137 inventoried | [JSON](evidence/native-main-ab33a412-race-2026-11-15.json) |

The first Go row and the PlainCAD row are one commit, `b26779db`. It has
the Go compiler mitigation (see the known defects on
[the status page](STATUS.md)), the general Boolean of `brep/boolean` with
planes and cylinders, cylinders of unequal radius or with skew axes
included, the chart mesher of `brep/chartmesh`, the STEP writer and reader
for boundaries of planes and cylinders with the edges where two cylinders
meet, the bridge that asks the general Boolean first and has no legacy
bore body and no legacy family for a cylinder through a side or a corner
of the stock, and the chamfer and
fillet of a result of the general Boolean in `brep/blend`. The Go run sets
`PLAINSOLID_SLOW=1` and `-tags research`, so it covers the slow tier and the
research quarantine; it took 61 minutes of wall time on four lanes. It used Go
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
1.27.4) passes at `b26779db` on all 60 packages with tests. The
PlainCAD run on `deb6a124` failed 45 development and 26 production
cases on holes through all; `f8f22061`, on which both runs then passed,
is the correction, and the specification of the general Boolean has the
cause (RL-32).

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

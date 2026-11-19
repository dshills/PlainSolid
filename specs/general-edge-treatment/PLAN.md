# Plan: edge treatment of a general Boolean result

This plan implements [the specification](SPEC.md) in this directory, as
the owner decided it on 2026-11-19 (section 14 of the specification). A
reference such as "K3" or "Q6" names an entry of the specification.

## State

On 2026-11-19, with `main` at `4e960375`: no code of this plan exists.
The package `brep/blend` does not exist. The design was tried with a
scratch test that is not in the tree; its results are in section 15 of
the specification.

## Before Phase 1

The owner decided OD-E1 to OD-E8 as recommended, so no phase below
changes. With the decision the owner asked for every phase to be
implemented on the branch `claude/cranky-vaughan-ac5664`, one phase at a
time, each committed, without a stop between phases; and for the traces
under `internal/plaincadbridge/testdata/general_boolean` to be replayed
on a default bridge before the first phase and after the last, to show
that no response for other stock changed (OD-E2). R-1, R-2, R-4 and
Phase 6 below are written to that.

`speccritic`, `golangci-lint`, `prism` and `realitycheck` each start
(`<tool> --help` exits with status 0; confirmed on 2026-11-19). If one of
them fails later, the session stops under ST-4; it does not skip a check
silently.

## Rules for every phase

- R-1. Work is on the branch `claude/cranky-vaughan-ac5664` in its own
  worktree. `AGENTS.md` asks for a `codex/` branch per chat; this branch
  was made for this session by the application that runs it, and the
  owner asked for the work to be done on it. It is not merged to `main` and not pushed by the
  implementing session; the owner merges.
- R-2. Each phase ends with these checks, all passing, before its commit:
  1. `gofmt -l brep/blend internal/plaincadbridge` prints nothing;
  2. `go build ./...` and `go build -tags research ./...`;
  3. `go vet ./...` and `go vet -tags research ./...`;
  4. `go test ./brep/blend ./internal/plaincadbridge` (the bridge's
     default tier took 31 s on 2026-11-19);
  5. `golangci-lint run ./...` reports nothing in a file the phase added
     or edited;
  6. `git diff --stat main -- internal/plaincadbridge` lists no existing
     `_test.go` file (D4).
- R-3. Each phase is one commit, or several coherent ones. The commit
  message says what changed, what was run with its result, and what was
  not run.
- R-4. After a phase's commit the session runs `prism review` on the
  phase's diff, fixes what it finds that is a defect of the phase, and
  lists the findings it leaves with a reason. A fix is followed by the
  checks of R-2 again and by its own commit. `realitycheck check` against
  the specification and this plan is run once, after Phase 5, and handled
  the same way. `clarion pack` is not run: this repository has no
  `docs/clarion-meta.json`. The session then goes on to the next phase,
  as the owner asked (Before Phase 1); it stops between phases only for
  ST-1 to ST-5.
- R-5. Every new test finishes in under two seconds. No test is added to
  `brep/trimmed`, and no file of `brep/trimmed` or `brep/boolean` is
  edited (G4).
- R-6. A new exported identifier of `brep/blend` is added in the phase
  whose bridge code calls it, not earlier (G1).
- R-7. Not run in any phase: the complete Go suite (tests of the slow
  tier, `PLAINSOLID_SLOW=1`, and tests under the research tag), race
  runs, `tools/ci/run-go-qualification.py`, cross-platform builds, the
  PlainCAD gates. They are run when the owner asks. Not part of that:
  the build and the vet under the research tag of R-2, which run no
  test, and `go test ./...` without `PLAINSOLID_SLOW`, the default tier
  that `AGENTS.md` requires before a merge, which Phase 6 runs once.
- R-8. A phase that changes what a request returns on a general result
  (Phases 1 to 4) reads the capability summary of `README.md` and the
  matrix of `docs/STATUS.md` and edits, in the same commit, in place and
  in a line or two, every line that the phase makes untrue, so that both
  are true after every phase. Phase 1 changes only `edgeAvailable`; if
  neither record mentions it, its commit message says that no line
  needed a change.

## When a phase stops

- ST-1. `boolean.Combine` returns an error, or a result that fails a
  closed form of 11.1, for a row that the specification puts in scope.
  The session commits the failing row as a skipped test whose skip
  message states the input and the error, labels the commit work in
  progress, and reports to the owner. It does not edit `brep/boolean`.
- ST-2. The line bound L3: 1,200 lines of Go outside test files, counted
  over `brep/blend` and the new bridge file with `wc -l`. The session
  counts before every commit. A commit that would bring the count to
  1,200 or more is not made: the session stops there, reports the count
  and what remains, and continues only on the owner's word.
- ST-3. A check of R-2 fails for a reason the phase did not cause. The
  session reports it and leaves it alone (`AGENTS.md`).
- ST-4. A phase's own check cannot be met after two attempts at the same
  defect. The session commits what passes, labelled work in progress with
  what remains, and reports.
- ST-5. A phase shows that a decision of section 14 cannot be kept as
  decided, that the plan needs a phase it does not have, or that a file
  outside `brep/blend`, the new bridge files, the three dispatch points
  of D3, the two records of R-8 and the two documents of this directory
  has to change. The session stops and
  reports.

## Size estimate

Lines of Go outside test files. An estimate over by half is reported in
the phase's commit message; the bound is L3.

| Part | Estimate |
|---|---|
| Resolving and classifying an edge (Phase 1) | 220 |
| Tools, combining and the checks (Phases 2 to 4) | 380 |
| Bridge adapter and dispatch (Phases 1 to 4) | 170 |
| Total | 770 |

## Phase 1: resolving a reference, and `edgeAvailable`

Goal: a reference to an edge of a general result resolves to an edge
chain with its two faces, its in-face directions, its opening angle and
its convexity; `edgeAvailable` on a general result answers from that.

Work:

- New package `brep/blend`: a package comment that states the input
  domain; the type of a resolved edge; the function that resolves one
  reference in a `trimmed.Solid` under R1 to R8, given ε_p and τ; the
  error values that distinguish "lost", "on a curved face" and "between
  parallel faces".
- New file `internal/plaincadbridge/general_edge.go`: `edgeAvailable`
  for a general result (R11).
- Edit: the `edgeAvailable` dispatch (D3).

Tests, in `brep/blend`, each on a solid built in the test from
`brep.Prism`, `brep.Box` and `boolean.Combine`:

- a box edge resolves, with ψ = 90°, convex, and `a1`, `a2` as section 3
  defines them; either order of the two ends gives the same chain;
- the concave and the convex axial edge of the L-shaped prism of Q9; an
  axial edge of the hexagonal prism of Q7 (ψ = 120°) and of the
  triangular prism of Q8;
- a chain of two collinear edges resolves as one. The solid is the box
  (0,0,0)–(20,10,10) united with the cylinder of radius 2 and height 3
  standing on its top face at (10,2,10), which touches the top front
  edge at (10,0,10). The Boolean divides the top face in two there
  (Boolean M6), so the edge (0,0,10)–(20,0,10) is two edges with two
  different top faces in one plane (tried on 2026-11-19);
- the point tolerance: a reference whose ends are each 5e-8 from the
  ends of a box edge resolves; one whose end is 2e-7 away is "lost";
- "lost": a reference to half an edge; a reference between two vertices
  that no straight chain joins; an edge interrupted by a slot;
- "on a curved face": a straight edge between a plane and a cylinder,
  taken from a body cut by a cylinder that leaves through a side face;
- "between parallel faces": on the same solid, the edge by which the
  Boolean divided the top face. The test finds it by inspection, as the
  one straight edge whose two faces lie in the plane z = 10, and gives
  its two ends as the reference;
- "lost" again: the seam of the cylinder of that solid, a straight edge
  that one face uses twice (R3);
- a smooth edge of a fillet tool made in the test does not resolve (F4).

Tests, in `internal/plaincadbridge`: `edgeAvailable` on the slotted box
of Q1 is true for (0,0,0)–(20,0,0) and for (0,0,10)–(12,0,10), and false
for (0,0,10)–(20,0,10) and for (0,2,0)–(20,2,0).

Done when: R-2 passes; every existing test passes unedited.

Risk carried: `edgeAvailable` on a general result changes from true to
false for a straight edge between a plane and a cylinder. No existing
test issues `edgeAvailable` to a general result (searched on 2026-11-19).

## Phase 2: chamfer of convex edges

Goal: `chamfer` on a general result, for one edge and for several edges
that do not meet, with every check of 7.5.

Work:

- `brep/blend`: the chamfer tool (T1, T3, T4, T5); the union of tools
  with the roles of their faces (B1, B2); the cut (B3, B4); the checks K1
  to K7 in their order; the result type; the error values for X7 to X10.
  The treatment kind has one value, chamfer; the fillet value is added in
  Phase 4 (R-6).
- `general_edge.go`: reading the request, N1 and N2, R9 and R10, the call
  of the package, the error mapping of section 9, D6, D7, publication by
  `putGeneral` (U1).
- Edit: the dispatch in `treatEdge` (D3). In this phase it sends only
  `chamfer`; `fillet` on a general result still returns today's "stock
  type is unsupported".

Tests:

- Closed forms: Q1; Q4; the chamfer rows of Q5; the chamfer rows of Q7
  and Q8, below the bridge.
- Rejections, each through the bridge on a general result, with code,
  message and D6: X1 (twice the same chamfer; half an edge); X2; X3 (the
  divider edge of Phase 1, on that solid published with `putGeneral`);
  X4; X5; X6; X8 by K1, by K3 (the four K3
  rows of 11.3 that are chamfers) and by K4; X9 (the three rows of
  11.3).
- The accepted counterpart of the K3 rows (hole axes at 2.000001).
- The chain of Phase 1: `chamfer` 0.5 of (0,0,10)–(20,0,10) on the box
  with the touching cylinder is rejected with X9, by K2: the cylinder
  stands on the flank.
- A chain treated with success, below the bridge. The test builds the
  box (0,0,0)–(20,10,10) by hand with `trimmed.NewSolid`, with the edge
  (0,0,0)–(20,0,0) stored as two edges that meet at (8,0,0); `chamfer`
  0.5 of the chain must give the closed form of 11.1 with L = 20. If
  `trimmed.NewSolid` or the intake of `boolean.Combine` refuses a solid
  with that vertex, the row is not dropped: the session reports it under
  ST-4 with the refusal.
- Below the bridge, on tools made for the purpose: one test each for K1,
  K5, K6 and K7, each requiring the error value of its row of section 9
  (X8 for K1, X10 for the others).
- A request with a duplicate and a reversed reference treats one edge
  (R9).
- Order of failures (section 9), three requests: a lost reference
  together with a `normal` that matches no face returns X1; a `normal`
  that matches no face together with a `direction` across the edge
  returns X4; a chamfer that fails both K2 and K3, the half-edge wedge of
  a hand-made tool below the bridge beside a touching hole, returns X9.

Done when: R-2 passes, and each of K2, K3 and K4 has been removed in turn
from a working copy and a test seen to fail (11.3); the commit message
names the test that failed for each.

## Phase 3: concave edges and the cap perimeter

Goal: a concave edge is treated by union; a request with convex and
concave edges is treated in two calls; `capChamfer` with four edges on a
general result.

Work:

- `brep/blend`: the union branch of B3; the checks applied to each call.
- `general_edge.go`: `capChamfer` (C1 to C5) and the fixed answer X12 for
  `capFillet` with four edges.
- Edit: the dispatch in `treatCapPerimeter` (D3).

Tests:

- Closed forms: Q6; the chamfer rows of Q9, below the bridge.
- Rejections: X12; X13 with a reference that is lost, with four edges
  that do not close, and with a `normal` of the wrong sign.
- Atomicity of the second call (D6). The body is a general result: the
  box (0,0,0)–(20,20,10) less the box (10,10,0)–(20,20,10), which
  leaves a concave axial edge at (10,10), united with the box
  (10.2,10,0)–(10.6,10.6,10), which stands on one face of that edge
  inside the reach of a chamfer of 1 and leaves the edge in place. Three
  `chamfer` 1 requests with `direction`: the convex axial edge at
  (20,0) alone must succeed; the concave edge alone must fail; both in
  one request must fail with the error of the concave edge alone, and
  the number of handles and the handle counter must be what they were
  before it.

Done when: R-2 passes.

## Phase 4: fillet

Goal: `fillet` on a general result, convex and concave, for one edge and
for several that do not meet.

Work:

- `brep/blend`: the fillet tool (T2) with its working faces taken from
  the sources of its `Combine` call; the fillet value of the treatment
  kind.
- `general_edge.go` and the `treatEdge` dispatch: `fillet` is sent to the
  mechanism.

Tests:

- Closed forms: Q2; Q3; the fillet rows of Q5; the fillet rows of Q7, Q8
  and Q9, below the bridge; Q10.
- Rejections: X7 (a radius below the working tolerance); X8 by K3 (a
  fillet as wide as its face); X11 (a fillet of an edge that a bore
  crosses at right angles; two fillets whose tools would meet are already
  rejected by X6, so the corner case is tested below the bridge).
- F4 on the published body: `edgeAvailable` is false for the two smooth
  edges of the fillet of Q2.

Done when: R-2 passes; L1 is confirmed by a test that counts the calls
for a request of three fillets.

## Phase 5: the result stays usable, and agreement with the legacy path

Goal: section 10 and section 11.2 of the specification as tests. No
production code is expected.

Work:

- Tests of F1, F2 and F3 on the chamfered body of Q1 and the filleted
  body of Q2.
- Tests of 11.2: each listed stock is treated by the legacy path on a
  default bridge and by the mechanism applied to the stock lifted with
  `generalOperand`, and the two shapes are compared.

Done when: R-2 passes. A row that fails is handled by ST-1 if the failure
is in `boolean.Combine`, in the mesher or in the STEP writer or reader:
those are outside this plan, and the session reports the request and the
error instead of changing them. A failure in the code of Phases 1 to 4 is
fixed in this phase.

## Phase 6: records and the checks before a merge

Work:

- `README.md` and `docs/STATUS.md` are read against what Phases 2 to 4
  left in them (R-8) and corrected where a line is no longer true: the
  Phase 4 cell of the matrix must not say that general results take no
  edge treatment, and the Phase 5 cell names the treatment with its
  domain.
- The package comment of `brep/blend` states the input domain and the
  number of `Combine` calls (L1).
- Section 14 of the specification is checked against what was built; the
  status line at its head is updated.
- The session proposes, and does not make, a note in the Boolean
  specification that the consequence of OD-8 for treatments after a
  Boolean is lifted; that document is the owner's record.

- The trace replay (OD-E2). Before Phase 1 the session replays every
  trace under `internal/plaincadbridge/testdata/general_boolean` on a
  default bridge, with a probe that is not committed, and keeps the
  response text of every request. After Phase 5 it replays them again
  and compares.

Done when: R-2 passes; `go test ./...` passes (the default tier, R-7);
every response of the trace replay is byte for byte what it was before
Phase 1, and the report gives the count; `README.md` and
`docs/STATUS.md` name the treatment, and the status page is still under
150 lines. Then the session stops and reports. The merge
is the owner's.

What this plan does not establish, to be said in that report: no PlainCAD
gate was run, and on the default bridge PlainCAD reaches the new path
only for a body that a legacy Boolean declined, so the PlainCAD suites do
not exercise it until a family is moved to the general Boolean.

## Where each requirement is implemented

| Requirement | Phase |
|---|---|
| D1, D2, D4, D5, D6, D7 | 2, and held by R-2 in every phase |
| D3 | 1 (`edgeAvailable`), 2 (`chamfer`), 3 (`capChamfer`, `capFillet`), 4 (`fillet`) |
| R1 to R8, R11 | 1 |
| R9, R10, N1, N2, N3 | 2 |
| T1, T3, T4, T5, B1, B2, B4, K1 to K7, U1, U2 | 2 |
| B3 | 2 (cut), 3 (union) |
| C1 to C5 | 3 |
| T2 | 4 |
| L1 | 4 |
| L2 | 2 |
| L3 | every phase (ST-2) |
| X1, X2, X4 to X10 | 2 |
| X3 | 1 (the classification), 2 (the request) |
| X11 | 4 |
| X12, X13 | 3 |
| X14, X15 | none: today's code, unchanged |
| F1, F2, F3 | 5 |
| F4 | 1, 4 |
| 11.1 | 2 (Q1, Q4, Q5, Q7, Q8 chamfers), 3 (Q6, Q9 chamfers), 4 (Q2, Q3, Q5, Q7 to Q10 fillets) |
| 11.2 | 5 |
| 11.3 | 2, 3, 4 |
| P1 to P6 | 1 to 4 |
| OD-E7 (N3) | 2 |
| OD-E8 | the bounds of the tests in 2 to 5 |

## Risks the plan carries

- K-A. K3 depends on which points of a result the Boolean makes one
  vertex. The scratch test confirmed it on twelve results (section 15).
  It has not been tried on a result whose new face borders a face that
  the Boolean divided (Boolean M6) on a treatment plane. Phase 2's Q4
  and the accepted counterpart row are the first tests of it; a failure
  there is a defect of K3's wording, not of the Boolean, and is reported
  with a proposed wording before it is changed.
- K-B. B2 gives a face of a united tool the role "support" when a working
  face and a support face of two tools are merged into one. That happens
  only when a chamfer plane of one edge is coplanar with a flank of
  another. K2 then rejects a treatment that is geometrically sound. No
  row of 11.1 has that arrangement; if one appears, it is reported.
- K-C. In a result of the Boolean, an edge chain of more than one edge
  was found only where something touches the edge at a point, and K2 or
  K3 then rejects the treatment. Phase 2 therefore tests a successful
  treatment of a chain on a solid built by hand, which the Boolean's
  intake has not been seen to accept.
- K-D. Time. A fillet of n edges makes up to 2n + 1 calls of
  `boolean.Combine`. The experiment's calls took under 0.1 s each on a
  body of 11 to 13 faces. A request of 32 fillets on a large body is not
  measured; Phase 4 measures a request of eight fillets on the body of
  Q5 and reports the time.
- K-E. On the test constructor's general bridge every analytic body
  without a recipe counts as a general result, which is wider than on
  the default bridge. The tests of this plan use a default bridge and
  obtain general results the way PlainCAD does, from a Boolean that the
  legacy path declines, or from `putGeneral` below the bridge.
- K-F. The opening angle near 0 or π was not tried. Phase 4 adds four
  rows below the bridge: `chamfer` 1 and `fillet` 0.5 of the axial edge
  at (0,0) of the prism with the profile (0,0), (100,0),
  (100, 100·tan 5°) and height 10 (ψ = 5°), and of the axial edge at
  (10,0) of the prism with the profile (0,0), (10,0),
  (20, 10·tan 5°), (20,10), (0,10) and height 10 (ψ = 175°). Each
  tool fits its prism with room to spare and the angles are in scope
  (R7), so a row passes only with a result that meets the closed form of
  11.1 within 1e-8. Any other answer, an error of `boolean.Combine`
  included, is ST-1: the session stops and reports the row.

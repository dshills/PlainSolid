# Plan: crossing cylinders in the general Boolean

This plan implements [the specification](SPEC.md) in this directory. A
reference such as "E1" or "A-C2" names an entry of that specification,
and "the base specification" is
[the general Boolean specification](../general-boolean/SPEC.md).

## State

On 2026-11-19: the owner decided OD-C1 to OD-C7, each as the
specification recommended, so no phase below changes (Phase 0). The
phases are implemented on the branch `codex/general-boolean-crossing`,
one commit each.

Phase 1 is committed as work in progress and stopped under ST-C6. The
row of the equal pair is in, with every test of A-C2 and A-C6 passing
but one: two crossing bores at a sine of 0.8 in a hexagonal prism, in a
pose that puts no axis along a coordinate axis. Its second cut returns
`ErrValidation`, "V1: trimmed: curved trim refinement folds or
degenerates in UV". The same prism in the pose of the coordinate axes
passes, as do a box in the turned pose and the turned prism at a right
angle. The test is committed skipped, with that reason.

What refuses it: `trimmed.NewSolid` meshes each face once as a check. On
the wall of the upright bore, between its bottom rim and the crossing,
the corners of the outline along the rim are computed from the arcs of
the rim and differ in the last bits of the axial coordinate, where in
the pose of the coordinate axes they are equal exactly; the check then
meets a triangle of no area. No arrangement of the Boolean's arcs that
was tried (its six halvings) passes. `brep/trimmed` is outside this work
(N2), and how the Boolean places the corners of an outline is section
7.10 of the base specification (U2), so both ways on are the owner's to
choose.

What else Phase 1 found and did:

- Sines of 1/8 and 1/64 pass their closed forms (risk K-1).
- A defect of `brep/boolean` that the turned tests met first and that is
  fixed in this phase: a closed edge was told from an open one by taking
  its interval back from its end and comparing with the period exactly,
  which fails by rounding for a cylinder whose rim does not start at
  parameter zero. `edge.closed` now allows that rounding. It changes no
  rule of the base specification.
- T2-006, 008, 009 and 016 pass and left `pending.txt`; T2-007 and
  T2-013 are listed there for time; the reasons of the other rows say
  which phase they wait for.
- Default bridge (R-5): 3,116 of 3,118 responses are byte for byte those
  of `d4a157a6`. The other two are the requests of T2-014 that A-C4
  names, a legacy refusal before and an answer now; they are in
  `legacy_refusals.txt`. Their value is not yet checked by the oracle
  (Phase 4).
- Size: `brep/boolean` has 6,049 lines outside test files, 37 more than
  before, of the 350 of B1.

The owner had the plan go on with that case skipped. Phase 2 is done:
`overlapVolume` answers with the volume a cut removes where the common
is not a manifold. T2-001, 002, 003, 010, 011 and 017 pass and left
`pending.txt`; so did T2-005 and T2-014, whose values all agree and
which take over two seconds only beside other rows. OP-6 is closed.

Phase 3 is committed as work in progress and stopped, as its own stop
rule says. The fold of `cutMany` sets a refused tool aside and tries it
again after the others (`inAnyOrder`), and that is tested. It does not
make T2-004 pass. Of the six orders of a small bore, a larger bore
across it and a bore that covers the small one, three are cut to one
volume and three are refused: in those the small tool comes after the
bore across it, and once the covering bore is cut the small tool lies in
its hole, clear of the body, but the general Boolean cannot prove it
clear of what is left of the wall of the bore across. The two cylinders
have no closed form together (a quartic pair), so the faces must be
proved separate, and the proof of SP2 bounds a wall face by a whole ring
of its cylinder, which here includes the part the covering bore removed.

What would take it further is a sharper proof under SP2, which SP5
allows as an addition: that a face of one cylinder lies wholly outside,
or wholly inside, the other cylinder. That needs the least and greatest
distance from the face to the other axis, on its edges and at the
turning points inside it, and those are zeros of a trigonometric
polynomial of degree two, the same computation as the conic method of
Phase 5. T2-015 fails first at Phase 5's entry and is a concealed cutter
as well, so it is likely to need the same proof after it. The plan has
no phase for that proof, and with it B1 would be exceeded, so it is the
owner's to decide.

Tier 2 stands at 13 rows passing (T2-001, 002, 003, 005, 006, 008, 009,
010, 011, 012, 014, 016, 017), two listed for time with every value
right (T2-007, 013), and two failing (T2-004, 015), both concealed
cutters. The eight Tier 2 rows of the probe are among the 13.

Size after Phase 3: `brep/boolean` 6,049 lines outside tests (37 of the
350 of B1); the adapter file 557, which is 89 more than before this work
and 9 over the 80 of B2 (ST-C4).

Asked whether to finish without the two concealed-cutter rows, the owner
chose to have them worked on (OD-C8). That is done, as Phase 5 below
with one addition, and both rows pass:

- The curve table meets a circle or an ellipse with a cylinder where it
  had no entry (section 6.4), by the zeros, decided at the tolerance, of
  a trigonometric polynomial of degree two (`brep/boolean/trig.go`). A
  first version that looked for changes of sign reported four crossings
  at an exact tangency, from rounding; Q6 records why the tolerance is
  where the decision is made. The row for two coplanar conics that are
  not homothetic was not needed and is unchanged.
- SP2 has a further proof (section 6.7, `separation.go`): a face of one
  cylinder clear of the whole of another. With it a cutter that a later
  bore covers is proved clear of the wall of a bore across it.
- The three bores of Phase 3 are now cut in all six orders to one
  volume. T2-004 and T2-015 pass and left `pending.txt`.
- T2-014 has two more requests that the record refuses and the general
  Boolean now answers, a blind bore whose floor lies inside the crossing
  (A-C4). They are in `legacy_refusals.txt`. The values of the requests
  of A-C4 that the default bridge now answers were checked against an
  integration by slices written for the purpose, which agrees to 6e-6,
  its own error; the oracle of Phase 4 has not run.

Tier 2 now: 15 rows pass and two are listed for time with every value
right (T2-007, 013). None fails.

Default bridge: 3,115 of 3,118 responses are those of `d4a157a6`; the
other three are requests of T2-014, a legacy refusal before and an
answer now.

Size: `brep/boolean` 6,383 lines outside tests, 371 more than before
this work (B1, raised to 450 under OD-C8); the adapter 557, 89 more
(B2, raised to 100).

Outside the two packages of the plan, `go test ./...` showed one
consequence: `brep/blend` had a test, and a sentence of its package
comment, saying that two fillets that meet at a corner are refused.
`blend.Treat` now cuts them, and its test holds the result to the closed
form: each fillet removes (1 − π/4)r²l and both remove (5/3 − π/2)r³ at
the corner. `brep/blend` itself is unchanged, and the bridge still
refuses a fillet round a cap of a general result.

Phase 4 is committed as work in progress and stopped on A-C9.

- D1 and D2 are a test: crossed bores at a right angle, at a slant, and
  three at one point mesh to valid solids within the volume budget, and
  their STEP files read back on either bridge. D3 is the trace replay of
  R-5.
- The oracle (A-C5): eight cases added, with the helper built against
  the pinned reference. Seven agree at every step, face counts included.
  The eighth, a blind bore whose floor lies inside the crossing, returns
  `ErrInconsistent` from F3 at its second step, on the bridge as well;
  its image from the other side agrees with the oracle. (This entry
  first said the bridge answers it and blamed a seam; both were wrong,
  and the third record of `ACCEPTANCE.md` has the correction and a
  survey of such floors.)
- A-C9 does not hold. With the first bore cut by the general Boolean,
  one of the eight tests passes as written. The other seven are cut
  correctly and fail on what they assert of the legacy representation:
  results of two orders compared with `==` (four tests), the bore recipe
  read from the result (two), and a STEP file byte for byte the same
  after an import (one). `specs/general-boolean/ACCEPTANCE.md` has the
  list.

What A-C9 means for the owner's plan: the general Boolean now cuts what
those tests cut, and the tests would have to be rewritten against
behaviour before a bore family is moved, as the probe already found for
the tests of plain bores.

The owner had the branch merged to `main` as it stands on 2026-11-19.
Of Phase 6, the capability summary of the README and the status and
verification pages were done with the merge. The status line, section
15.1, OD-1 and a ruling RL-27 that supersedes RL-22 were written into
the base specification afterwards, on `codex/crossing-robustness`.

After the merge, on the branch `codex/crossing-robustness`:

- The turned case of Phase 1 passes and is no longer skipped. The cause
  was in `brep/boolean` and not in `brep/trimmed`: the chart image of a
  section circle carried the rounding of its axis vectors against the
  cylinder's axis, about 1e-16 in a pose off the coordinate axes, as a
  height that varies, so the corners of an outline along one rim were
  not at one height. `cylinderChart.image` now takes such terms as
  zero. No rule of the base specification changes, and the default
  bridge's 3,118 trace responses are byte for byte the same.
- Blind floors inside a crossing were surveyed and the record corrected
  (`ACCEPTANCE.md`, third record). `TestCrossingFloorInTheCrossing`
  holds floors outside the band of the smaller ellipse, and at its
  middle, to one volume in both orders and for the image from the other
  side.

The complete Go suite, run on 2026-11-20 at `f95276ec` under Go 1.23.12
at the owner's request, failed on two tests, and both are corrected on
the branch `codex/suite-failures`:

- `TestCrossingCylinders`, the union of two crossing cylinders off the
  coordinate axes, failed V1 under Go 1.23.12 and passed under Go
  1.27.4, which the default tier uses. It was the turned case again, one
  step on: the corners of an outline along a rim, and the two ends of a
  seam, took their place in the chart from the computed position of
  each vertex, which differs in its last bits from one vertex to the
  next. They now take the height of the section circle, and the place
  of the ruling, from the carrier (B3 of the base specification). U2
  left section 7.10 as it stood; this adds to B3.
- `TestBlindCrossedObliqueBoreSTEPAndFurtherCut`, a slow test of the
  legacy path, required two requests to be refused that the fallback
  now answers (A-C4). It was not run when Phase 1 changed them, because
  only the default tier and the general replays were run then. It now
  requires the legacy path to decline and holds the answers to a closed
  form and to the oracle's volume.

A check this plan lacked: R-3 names the general replays only. A phase
that changes what the default bridge answers must also run the slow
tier of `internal/plaincadbridge`, under the release of Go the complete
suite uses.

Still open: a blind floor strictly inside the band of the smaller
crossing ellipse is refused in some arrangements (M6, V1 or F3); and
A-C9 does not hold.

## Rules for every phase

- R-1. One phase is one commit on a `codex/` branch in its own worktree.
  Nothing is merged to `main` and nothing is pushed without the owner's
  word.
- R-2. Checks after each phase, all of which must pass before the commit
  of a phase that completes (a phase that stops is committed under R-8
  instead):
  `gofmt -l` on the changed packages prints nothing; `go build ./...` and
  `go vet ./...`, each also with `-tags research`; `go test` on the
  packages changed; `golangci-lint run ./...` with no finding in a
  changed file.
- R-2a. After each phase the lines outside test files that the work has
  added so far are counted against B1 and B2 and given in the commit
  message. Over either budget is stop condition ST-C4.
- R-2b. Each phase makes, in its own commit, the amendments of section 7
  of the specification that concern what it implements, the closing of
  an open item of the base specification included: Phase 2 closes OP-6,
  Phase 5 closes OP-7, and Phase 4 closes OP-5 on the evidence of the
  replay. Phase 6 makes only what is left.
- R-2c. Every new test finishes in under two seconds or starts with
  `slowtest.Skip(t)` (B3); the times of the new tests are read from
  `go test -v` before the commit. No phase changes `brep/boolean`'s
  validation V1 to V8, and every result in every test is one that
  `Combine` returned after it (U3).
- R-3a. A phase that changes what the default bridge answers also runs
  the whole slow tier of `internal/plaincadbridge`, with
  `PLAINSOLID_SLOW=1`, under the release of Go the complete suite uses
  (added on 2026-11-20; see State).
- R-3. Slow tests after each phase, by name, with `PLAINSOLID_SLOW=1`:
  `TestGeneralBooleanAcceptance`, `TestGeneralBooleanTraces` and
  `TestGeneralStepExportTraces` in `internal/plaincadbridge`. Every
  Tier 1 row must pass as it does at `d4a157a6`.
- R-4. The acceptance test fails when a row listed in
  `internal/plaincadbridge/testdata/general_boolean/pending.txt` passes
  (K7 of the base specification). The phase that makes a Tier 2 row pass
  removes that row from the list in the same commit.
- R-5. The default bridge. Before Phase 1 and after each phase the 113
  traces are replayed on a default bridge and every response compared
  with the one at `d4a157a6` (D3). A response may differ only where the
  earlier one is a legacy refusal and the later one an answer; each such
  response is listed in the commit message with its legacy message. Any
  other difference is stop condition ST-C5.
- R-6. No file of `brep/trimmed`, `geom`, `exchange/step` or
  `brep/blend` changes (N2). No file with the `research` build tag
  changes. No OCCT source is opened.
- R-7. Each commit message says what changed, what was run and what was
  not. The complete Go suite (the slow tier and the research tests, run
  by `tools/ci/run-go-qualification.py`), and the PlainCAD gates are not
  run by this plan; the owner asks for them. `go test ./...` is the
  default tier, which is a different thing and which Phase 6 runs.
- R-8. A stop condition of section 10 of the specification ends the
  phase with a commit labelled work in progress, whose message names
  each check of R-2 to R-5 that does not pass, and a report. The plan
  does not go on to the next phase by itself after a stop; it resumes
  when the owner says how.

## Phases

| Phase | Content | Specification | Size outside tests |
|---|---|---|---|
| 0 | Record the owner's decisions | 11 | documents only |
| 1 | The equal pair | 6.1, 6.5, A-C2, A-C6 | about 45 lines in `brep/boolean` |
| 2 | Overlap volume of a pinched common | 6.2 | about 20 lines in the adapter |
| 3 | The order of tools in `cutMany` | 6.3 | about 50 lines in the adapter |
| 4 | Acceptance records, the oracle, what a result must still do | 6.6, 8 | none; tests and data |
| 5 | Conics the curve table cannot meet | 6.4, A-C7 | about 250 lines in `brep/boolean` |
| 6 | Records | 7 | documents only |

Phases 2 and 3 do not depend on each other and both depend on Phase 1.
Phase 4 depends on Phases 1 to 3. Phase 5 depends on Phase 1 only and is
left out if the owner decides OD-C4 against it; Phase 3 is left out if
the owner decides OD-C3 against it. Phase 6 is last.

### Phase 0: the decisions

- Write the owner's decision on each of OD-C1 to OD-C7 into section 11
  of the specification and its status line. Where a decision differs
  from the recommendation, revise the requirements it touches and this
  plan before any code, and run `speccritic check` and `plancritic check`
  again.
- Done when: the specification states each decision and both critics
  have been run on the revised text.

### Phase 1: the equal pair

- Files: `brep/boolean/surface_table.go`, `closed_forms.go`, their test
  files, `matrix_test.go`; `pending.txt`; the base specification's
  section 7.4 row and SP3.
- Work: the row of E1 with the condition |r₁ − r₂| + |PQ| ≤ ε, the bound
  of E4, and the messages of R1 and R2. The row of the earlier attempt
  (`cc7a679e`) is the starting point; its condition is the wider one and
  it has no angle bound.
- Tests: A-C2, each closed form below the bound of A3, for sines of 1,
  0.8, 0.6, 5/13, 1/8 and 1/64, three concurrent bores, and a stock that
  is not a box with no axis along a coordinate axis; the rows of A-C6.
- Expected, from the experiment (section 3.4 of the specification): every
  Tier 2 response with a value agrees with its record; T2-006, 009, 012
  and 016 pass and leave `pending.txt`.
- Stop: ST-C1, ST-C2, ST-C4, ST-C6. If a sine of 1/8 or 1/64 fails
  (ST-C6), the report gives the smallest sine that passes, for the owner
  to decide OD-C6 again.
- Done when: R-2 to R-5 pass and the tests above pass.

### Phase 2: the overlap volume of a pinched common

- Files: `internal/plaincadbridge/general_boolean.go` and a test file;
  `pending.txt`; CO2 of the base specification.
- Work: P1 to P3 in `generalOverlapVolume`. The fallback is chosen on
  `errors.Is(err, boolean.ErrNonManifold)`, which needs the error of
  `Combine` before `generalFailure` maps it; a context error from the
  cut is returned as it is (P2).
- Tests: the overlap of a bored box with a crossing tool, at a right
  angle and at a slant, against the closed forms 18·9π − 144 and
  100π − 160/3 of the bridge's own tests; a common refused for another
  reason still fails; a cut that fails returns the error of the common;
  a context cancelled before the cut returns the context error (P2).
- Expected: T2-001, 002, 003, 010, 011 and 017 pass and leave
  `pending.txt`; T2-005 and T2-008 pass run alone.
- Stop: ST-C1, ST-C5.
- Done when: R-2 to R-5 pass and the tests above pass.

### Phase 3: the order of tools in `cutMany`

- Files: `internal/plaincadbridge/general_boolean.go` and a test file;
  `pending.txt`; CO1 of the base specification.
- Work: F1 to F7 in `generalCutMany`. The measures of the body are
  carried from step to step as today (RL-10).
- Tests: a small bore, a larger bore across it and a bore that covers the
  small one, in every order of the three, give one volume; a request
  with a quartic pair in every order fails with the error of F3 and
  publishes nothing; the number of calls of `Combine` on a request built
  to defer is at most the bound of F7; an error other than
  `ErrUnsupported` fails at once (F5).
- Expected: T2-004 passes and leaves `pending.txt`.
- Stop: ST-C1, ST-C5. If T2-004 does not pass with F1 to F7 as written,
  the phase stops and reports why; it adds no rule that recognises
  covered tools.
- Done when: R-2 to R-5 pass and the tests above pass.

### Phase 4: acceptance, the oracle, and use of a result

- Files: tests in `internal/plaincadbridge`; `pending.txt`,
  `legacy_refusals.txt`; `internal/occtcheck/testdata/general_boolean_cases.json`;
  `specs/general-boolean/ACCEPTANCE.md`.
- Work:
  - A-C3: T2-007 and T2-013 stay in `pending.txt` with the reason "time
    over two seconds" if OD-C5 is decided as recommended.
  - A-C4: the two requests of T2-014 go into `legacy_refusals.txt` with
    their legacy messages.
  - A-C5: one oracle case for each of the three Tier 2 families and one
    for each request of A-C4, run with `TestGeneralBooleanOracle` against
    the pinned reference build the trial used. The pinned build is at
    `.worktrees/qualification/reference` in the main checkout and is
    used from there by its path. If it is not there, the other items of
    the phase are done, the phase stops under R-8 with A-C5 named as
    outstanding, and it resumes when the owner supplies the build; the
    phase does not rebuild it.
  - A-C8: tests of D1, D2 and D3.
  - A-C9: on a scratch change that is not kept, make the bridge cut the
    first bore of the eight tests of section 3.3 with the general
    Boolean. All eight must pass; one that does not is reported with its
    failure and the phase stops under R-8.
- Stop: ST-C1 on any oracle disagreement; ST-C5.
- Done when: every Tier 2 row passes or is listed under A-C3 with its
  reason, the oracle cases agree within the bounds of A6, and R-2 to R-5
  pass. Phase 5 was done before this phase, at the owner's choice
  (OD-C8), so the oracle runs once, after it, with a case for a cut that
  needs the entry of section 6.4.

### Phase 5: a conic the curve table cannot meet, and a face clear of a cylinder

- Files: `brep/boolean/curve_table.go`, `closed_forms.go` and their test
  files; `pending.txt`; section 7.5 of the base specification.
- Work: Q1 to Q6, and G1 to G6 in `brep/boolean/pass_face.go` and a new
  file. Both rest on one polynomial type that takes its coefficients as
  data.
- Tests: the table of A-C7, each hit in closed form; the Tier 1 rows are
  unchanged (R-3), since the new result replaces `ErrUnsupported` only.
- Expected: T2-015 passes and leaves `pending.txt`. The experiment did
  not test this, so it is a hypothesis: T2-015 failed at this entry and
  may meet another limit after it.
- Phase 4 follows this phase (OD-C8), and its oracle run has a case for
  a cut that needs the new entry.
- Stop: ST-C1, ST-C3, ST-C4. If T2-015 still fails after the entry
  exists, that is a stop under R-8: the phase commits the entry and its
  tests as work in progress, leaves the row in `pending.txt` with the
  new reason, and reports. A-C3 does not allow T2-015 as an exception
  while section 6.4 is in, so the owner then decides whether to amend
  A-C3 or to have the new limit worked on.
- Done when: R-2 to R-5 pass, the table of A-C7 passes and T2-015
  passes.

### Phase 6: records

- Files: the base specification (sections 15.1 and 15.5, OD-1, OP-5,
  OP-6, OP-7, a ruling that supersedes RL-22), its `PLAN.md` state,
  `ACCEPTANCE.md`, `README.md`, `docs/STATUS.md`, this plan's State.
- Work: section 7 of the specification, as far as earlier phases did not
  already make each amendment; one line in the capability summary and
  the status matrix; the sizes measured against B1 and B2.
- Before a merge is proposed: `go test ./...`, and `go build -tags
  research ./... && go vet -tags research ./...`, as `AGENTS.md`
  requires.
- Done when: the documents agree with the code and the checks pass.

## Open before the work can be called complete

- A blind floor strictly inside the band of the smaller crossing ellipse
  (the third record of `specs/general-boolean/ACCEPTANCE.md`). The
  refusals come from M6, V1 and F3 of the base specification, which U2
  leaves as they stand, so what to do about them is the owner's: change
  those rules, or record the band as outside what is supported. One of
  them was looked into, the floor at 20.5 that fails V1. Such a floor
  leaves thin faces of the bore wall between its rim and the smaller
  ellipse. With the arcs as first cut one of them is a loop of two uses,
  which `trimmed.NewSolid` does not take; with the arcs halved, as the
  assembly then tries six times, its check of each face by a mesh folds
  or meets a triangle of no area on the thin face, or runs out of its
  budget. So this one is a limit of that check on thin faces, in
  `brep/trimmed`, and no small change in `brep/boolean` was found for
  it, unlike the turned case.
- A-C9: the bridge tests of crossing bores assert the legacy
  representation and would have to be rewritten before a bore family is
  moved to the general Boolean.

## Risks

- K-1. Small angles. No pair with a sine below 5/13 has been run. E4
  admits down to 1/64. Phase 1 tests it first and stops if it fails.
- K-2. Tier 1. The new row answers pairs that returned `ErrUnsupported`
  before; it must not change a pair that SP2 proves separate. R-3 checks
  every Tier 1 row after each phase.
- K-3. The default bridge. Results of the fallback gain crossing bores,
  so some legacy refusals in the traces become answers. R-5 lists each;
  A4 of the base specification requires each to be classified.
- K-4. Time. T2-013 takes 4.42 s. Nothing in this plan makes `Combine`
  faster.
- K-5. The conic method is the largest piece and the least tested by the
  experiment. It is last and can be left out.
- K-6. The fallback of P1 loses accuracy for an overlap much smaller
  than the body. The specification states that as a limit.

## What this plan does not do

- Quartic pairs (N1).
- Making the general Boolean the bridge's first path for any family, or
  deleting any legacy recipe (N4). Section 12 of the specification says
  what would follow.

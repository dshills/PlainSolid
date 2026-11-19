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
- Done when: every Tier 2 row other than T2-015 passes or is listed
  under A-C3 with its reason, the oracle cases agree within the bounds
  of A6, and R-2 to R-5 pass. T2-015 stays in `pending.txt` with its
  present reason until Phase 5, or under A-C3 if Phase 5 is left out.

### Phase 5: conics the curve table cannot meet

- Files: `brep/boolean/curve_table.go`, `closed_forms.go` and their test
  files; `pending.txt`; section 7.5 of the base specification.
- Work: Q1 to Q7. The method for two coplanar conics is one function
  that takes two conics as data; the row for a conic with a cylinder
  projects and calls it (Q2).
- Tests: the table of A-C7, each hit in closed form; the Tier 1 rows are
  unchanged (R-3), since the new result replaces `ErrUnsupported` only.
- Expected: T2-015 passes and leaves `pending.txt`. The experiment did
  not test this, so it is a hypothesis: T2-015 failed at this entry and
  may meet another limit after it.
- The oracle run of Phase 4 is repeated after this phase, with one case
  added for a cut that needs the new entry, since this phase changes
  which hits the Boolean finds.
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

# Plan: general Boolean trial

This plan implements [the specification](SPEC.md) as it stands at commit
`c96f79a5`. It is a plan only. It
contains no implementation, and nothing in it starts until the owner says so.
Identifiers such as PC2, A3 or ST4 are requirement identifiers of the
specification.

## Terms

- **Session**: one agent session that ends with a commit and a report to the
  owner. `AGENTS.md` ends every task that way, so the owner starts each
  session.
- **Part**: one unit of this plan, sized for one session. Phases 1, 2, 3 and
  5 have several parts; a phase's gate is checked at the end of its last
  part.
- **Row**: one row of `specs/general-boolean/ACCEPTANCE.md`, which is one
  existing bridge test function.
- **Leaf**: one subtest of the section 11 matrix, named
  `TestMatrix/<row>/<variant>/<operation>`, for example
  `TestMatrix/C4/planar/union`. The variant is `planar` or `cyl`. For C15
  and C21 the cylinder variant is run once for each stored form of section
  11 of the specification: `sf1`, `sf2`, `sf3`, and the same three under a
  placement, `sf1p`, `sf2p`, `sf3p` (SF1 to SF4).
- **General bridge**: a `Bridge` with the general path selected (L3).
  **Default bridge**: `plaincadbridge.New()`, unchanged by the trial.
- **Default tier**: `go test ./...` with no build tag and without
  `PLAINSOLID_SLOW=1`.
- **Trace**: the recorded requests and responses of one row, a file under
  `internal/plaincadbridge/testdata/general_boolean/`.

## Phases

| Phase | Parts | Content | Gate at its end |
|---|---|---|---|
| 0 | 1 | Freeze the acceptance rows (A1); probe the foundation (E1) | G0: traces replay on the default bridge |
| 1 | 4 | Planar operands through the whole pipeline (PL1) | G1: the planar gate of PL2 |
| 2 | 3 | Cylinders: carriers, charts, tables, seams | G2, a checkpoint that does not block: every leaf of section 11 (A5); leaves that fail are due at G3 |
| 3 | 2 | Bridge adapter and Tier 1 rows | G3: PC1 without the oracle run, PC2, PC3, PC4 |
| 4 | 1 | Tier 2, optional (OD-1) | none; kept or reverted |
| 5 | 2 | Oracle helper, the one oracle run, the A8 record, close | G4: the oracle run (A6), PC1 complete |

There are six phases and thirteen parts. One corrective phase exists
(section "Corrective phase"). No part of Phase 2 starts before G1 passes
(PL3). Phase 4 starts only after G3 passes. The oracle run is the last step
of the plan.

## Rules for every part

These come from `AGENTS.md` as it reads on `main` and from the
specification. They apply to every part and are not repeated in the parts.

- R1. Work is on one branch, `codex/general-boolean`, in its own worktree.
  `BASE` is the commit it is created from: the commit of `main` that holds
  the merged specification and this plan (D1). Nothing is merged to `main`
  or pushed unless the owner asks.
- R2. A part does its steps, commits, runs its checks against that commit,
  and adds fixing commits until the checks pass. The session may then go on
  to the next part of the same phase. It stops at the end of the phase, at a
  part whose checks it cannot make pass, and at any trigger in the table of
  "When a check or a stop condition trips". It never starts the next phase.
  When it stops it reports, for each part it attempted: what changed, what
  was run, what was not verified, and what comes next.
- R3. The record of a part is its commit message. No dated narrative is
  added to `docs/STATUS.md` or `docs/VERIFICATION.md`; those pages and
  `README.md` change only if the trial passes and the owner merges it (RC1).
- R4. No file in `brep/trimmed` or `geom` is edited (G4, L5). No file that
  starts with `//go:build research` is created, extended or untagged. No
  `t.Parallel()` is added to a test in `brep/trimmed`; the plan adds no test
  there at all. No dependency is added.
- R5. Verification uses the default tier. The complete suite, race runs,
  `tools/ci/run-go-qualification.py`, cross-platform builds and the PlainCAD
  browser gates are not run. There are two exceptions. The oracle run of
  Part 5.2 was approved by the owner (OD-5). Phase 0 runs individual
  legacy test functions by name with `PLAINSOLID_SLOW=1` when they are
  skipped in the default tier, to record their traces (A1); it never runs
  the slow tier as a whole.
- R6. Every new test finishes in under two seconds or starts with
  `slowtest.Skip(t)`. A matrix leaf or an acceptance row may not use
  `slowtest.Skip` (A7).
- R7. New kernel code goes in `brep/boolean` only. Every exported identifier
  of `brep/boolean` is in the list of P2 and has a caller in
  `internal/plaincadbridge` in the same commit.
- R8. Implementation sessions do not open the OCCT source (OD-6). They work
  from the specification and the literature cited in G3. Part 5.1 writes the
  oracle helper against the public interface of the built library, following
  `tools/occtref/README.md`.
- R9. A session that cannot finish its part leaves a commit labeled work in
  progress whose message says what remains. The next session continues the
  same part. This counts against the session budget of the phase (section
  "Session budgets").

## Standing checks

Each part ends by running the checks below that its "Checks" line names.
They run after the part's work is committed (R2), so `"$BASE"...HEAD` and the
working tree both show the whole part. The commands read the base commit
of R1 from the shell variable `BASE`, which the session sets first. Wherever a command below or in a
gate pipes `go test` into `rg` or `jq`, the same `go test` command is first
run alone and must exit with status 0; the pipe only counts or filters its
output. Lines of a check that name `brep/boolean` apply from Part 1.1, when
that package first exists.

- K1. Build, vet, lint and tests of what changed. The lint line fails only
  on findings introduced since `BASE`: `internal/plaincadbridge` had 14
  findings at `BASE`, and those are not the trial's to fix. A finding in
  code the trial adds or changes fails the check.

  ```sh
  go build ./... && go vet ./...
  golangci-lint run --new-from-rev "$BASE" ./brep/boolean/... ./internal/plaincadbridge/...
  go test ./brep/boolean -count=1
  go test ./internal/plaincadbridge -count=1   # when the part changed it
  ```

- K2. Size (PC4, ST4). Both commands print a number at or under the cap:

  ```sh
  find brep/boolean -name '*.go' ! -name '*_test.go' -print0 | xargs -0 cat | wc -l   # cap 15000
  wc -l < internal/plaincadbridge/general_boolean.go                                   # cap 400
  ```

  The commit message states both numbers and the part's budget from the
  table in "Size budget".
- K3. One path (PC2, AR8). The session reads the complete diff
  `git diff "$BASE"...HEAD -- brep/boolean internal/plaincadbridge` and
  confirms in the commit message that no function tests the number of
  faces, edges or vertices of an operand, the primitive it came from, or
  its placement, other than through sections 7.3 to 7.5 and the work limits
  of section 10, and that the constraints of AR8 hold, including traversal
  in index order with no randomness (no iteration over a map decides an
  output). Two searches support
  the review and do not replace it. The first must print nothing; every
  line the second prints must be a section 10 limit check or a loop bound:

  ```sh
  rg -n -i '^func .*(bore|pocket|notch|slot|boss|crossed|counterbore|box|cube)' brep/boolean -g '!*_test.go'
  rg -n 'len\(' brep/boolean -g '!*_test.go' | rg -i 'face|edge|vert|shell'
  ```

  A third search covers S3: the Go type of an edge's stored curve is never
  inspected. Every line it prints must act on a surface value (S2) and none
  on an edge curve:

  ```sh
  rg -n '\.\(type\)|\.\(\*?geom\.|CurveMapping' brep/boolean -g '!*_test.go'
  ```

- K4. Two seconds (A7). Only leaves and rows are timed, not the groups
  above them. The first command exits with status 0 and the second prints
  nothing:

  ```sh
  go test -json -count=1 ./brep/boolean ./internal/plaincadbridge \
    -run '^(TestMatrix|TestGeneralBooleanAcceptance)$' > "${TMPDIR:-/tmp}/general-boolean-k4.json"
  jq -r 'select(.Action=="pass" and .Test!=null and .Elapsed>=2
      and (.Test|test("^TestMatrix/[^/]+/[^/]+/[^/]+$|^TestGeneralBooleanAcceptance/[^/]+/[^/]+$")))
    | "\(.Test) \(.Elapsed)"' "${TMPDIR:-/tmp}/general-boolean-k4.json"
  ```

- K5. Boundaries (R4, R7). The first three print nothing; the fourth prints
  only the identifiers P2 allows and no other: `Combine`, the operation type
  and its three values, the options type, the result type, and the seven
  error values:

  ```sh
  git diff --name-only "$BASE"...HEAD -- brep/trimmed geom go.mod go.sum
  rg -l '^//go:build research' brep/boolean internal/plaincadbridge/general_boolean*.go
  git status --porcelain
  go doc -short plainsolid/brep/boolean
  ```

- K6. Stop conditions. The session checks ST1 to ST6 against the table in
  "When a check or a stop condition trips" and states the outcome in the
  commit message.
- K7. Pending lists. `brep/boolean/testdata/pending.txt` and
  `internal/plaincadbridge/testdata/general_boolean/pending.txt` name the
  leaves and rows that are expected to fail at this point. A listed case
  that fails is logged and not counted as a test failure; a listed case
  that passes is a test failure, so the list cannot hide a passing case. A
  case not listed must pass. The commit message gives the number of entries
  in each list. Each gate says which entries it allows.

## When a check or a stop condition trips

The implementer stops and reports. The implementer does not work around the
condition, raise a cap, add a case to a pending list to get past a gate, or
narrow a test.

| Trigger | Condition | Action |
|---|---|---|
| A result passes section 8 and differs from the analytic, legacy or oracle value beyond A3 or A6, and no general strengthening of section 8 catches it | ST1 | Stop. |
| The only known way to make a Tier 1 row or a leaf pass is code that K3 would reject | ST2 | Stop. Do not write that code. |
| A gate still fails when its phase's session budget and the corrective phase are both used | ST3, PL4 | Stop. |
| K2 exceeds a cap before G3 passes | ST4 | Stop. Do not move code to another package or into test files to get under the cap. |
| `trimmed.NewSolidContext` rejects a result that meets B2 and B3, `Classify` gives a wrong answer on a valid operand, or a row can pass only by changing `brep/trimmed` or `geom` or adding a dependency | ST5, E1 | Stop. Do not edit those packages. |
| A Tier 1 row needs a change to sections 7.2, 7.3 or 7.6 to 7.11 that the specification does not allow | ST6 | Stop. |
| K4 prints a leaf or a row | A7 | The case fails. Fix the general mechanism; if that is not possible within the phase, the gate fails and ST3 applies. Do not mark it slow. |
| K5 prints a forbidden path or an identifier outside P2 | R4, R7 | Revert that change before committing. |
| A bridge change is needed outside the files Part 3.1 lists | L2, L3 | Stop and ask the owner; this is not a trial failure. |
| The pinned OCCT build is not available for Part 5.2 | OD-5 | Stop and ask the owner; this is not a trial failure. |
| Phase 0 finds a Boolean response it cannot check for a reason that is not in section 15.2 | A1 | Stop and ask the owner. Do not drop or shorten the row. |
| `csg/poly.Combine` returns an error on a leaf that PL2 requires to be compared with it | PL2 | Stop and ask the owner whether that leaf may be compared with its analytic volume only. |
| The oracle run differs on a case | A6, ST1 | Triage as Part 5.2 step 4 describes. A second oracle execution needs the owner's authorization. |

On a stop under ST1 to ST6 the session follows section 16 of the
specification: it leaves the branch unmerged with a commit labeled work in
progress, leaves the legacy constructors and the default bridge untouched,
fills the A8 columns of `ACCEPTANCE.md` for the rows that produce a result
(A8), and reports which condition was met, the failing rows or leaves, and
the smallest input that shows the failure.

## Session budgets

ST3 speaks of phases. A part is sized for one session, and R9 lets a part
run over. So that ST3 has a definite meaning, each phase has a session
budget (owner decision D2). The unit is one attempt at one part: a session
that runs several parts of a phase (R2) uses one unit for each part it
attempts, and a session that resumes an unfinished part uses one more.

| Phase | Parts | Session budget |
|---|---|---|
| 0 | 1 | 2 |
| 1 | 4 | 6 |
| 2 | 3 | 5 |
| 3 | 2 | 4 |
| 4 | 1 | 1 |
| 5 | 2 | 3 |
| Corrective | 1 | 2 |

A phase whose gate has not passed when its budget is used has failed its
gate. Two phases differ. G2 is a checkpoint: when the Phase 2 budget is
used, leaves that still fail are carried into Part 3.2 and are due at G3.
Phase 4 has no gate: when its one session ends without Tier 2 supported,
the change is reverted.

## Size budget

Cumulative non-test lines in `brep/boolean` expected at the end of each
part. These are warnings, not caps. A part that ends more than 15 percent
above its line says so in its report, with the reason. The figures rose
by 300 lines for the required work when edge carriers came to be derived
by evaluation (I5), and Part 2.1 took over some of Part 2.2. The cap is 15,000
(PC4) and only the cap trips ST4.

| Part | 1.1 | 1.2 | 1.3 | 1.4 | 2.1 | 2.2 | 2.3 | 3.2 | 4 |
|---|---|---|---|---|---|---|---|---|---|
| Lines | 1,600 | 3,100 | 4,700 | 6,200 | 8,500 | 9,900 | 11,100 | 11,600 | 12,200 |

## Phase 0: freeze the acceptance rows

One part. No kernel code. Checks: K1 (the bridge lines), K5 (the lines
that do not name `brep/boolean`), K6.

Files created: `specs/general-boolean/ACCEPTANCE.md`,
`internal/plaincadbridge/general_boolean_trace_test.go`,
`internal/plaincadbridge/general_boolean_foundation_test.go` (the foundation
and publication probes),
`internal/plaincadbridge/testdata/general_boolean/*.jsonl`. No other file
is changed by the commit.

Steps:

1. List every test function in the files of section 15.1 of the
   specification, Tier 1 and Tier 2 (`rg -n '^func Test'` on each file).
2. Capture a raw trace of each function. Apply a temporary patch, never
   committed, to `internal/plaincadbridge/bridge.go` (`New`, `Call`) and
   `bridge_test.go` (`near`) that appends one JSON line per event to the
   file named by an environment variable: a new bridge, a request with its
   response, and each `near(got, want)` pair. Run each function alone with
   `-run '^Name$' -count=1 -parallel 1`. A function that is skipped in the
   default tier is run once with `PLAINSOLID_SLOW=1` and marked
   `slow legacy` in its row.
3. Revert the temporary patch (`git checkout -- internal/plaincadbridge/bridge.go internal/plaincadbridge/bridge_test.go`)
   and confirm `git status --porcelain` lists only new files.
4. Reduce each raw trace to its committed form, one JSON object per line.
   The schema is fixed before any trace is captured:
   - `{"new":true}` at each new bridge.
   - `{"req":…, "ok":…, "code":…, "handle":…, "result":…, "analytic":…}`
     for each request. `req` is the request exactly as issued, with the
     handles it refers to. `ok` says whether it succeeded and `code` is the
     failure code when it did not. `handle` is the handle the response
     published, when it published one; the replay maps it to the handle the
     same request publishes, and rewrites later `req` references through
     that map. `result` holds, for a response that is a shape, `valid`,
     `volume`, `surfaceArea`, `solidCount` and `bounds`; for
     `overlapVolume`, the value; for `pocketFloor`, the whole result
     object; for `mesh` and `stl`, only a marker that a mesh was returned;
     for every other request, nothing. STEP and proof payloads are dropped.
   - `analytic` is present when a `near(got, want)` pair of the source test
     has a `got` that is bit-for-bit the volume, area or overlap value of
     this response; it holds the `want` values. Other pairs are dropped. A
     `got` that equals a value of more than one response or property of
     its bridge is ambiguous: it is not attached by the rule, and the row
     lists it for a reading of the source test, which decides where it
     belongs.

   The reduction is done with a throwaway script kept outside the
   repository; the committed test of step 6 defines the format by reading
   it. Before all traces are frozen, steps 2 to 6 are carried out for two
   functions first, one that chains Boolean requests on earlier results
   (from `planar_boolean_chain_test.go`) and the one that issues
   `pocketFloor`, to confirm that handle mapping and the `pocketFloor`
   comparison replay correctly.
5. Decide which responses each trace checks. Every request of a trace is
   replayed, in order; none is removed and no trace is shortened. A
   response is *checked* (A3, A4) unless an exclusion of section 15.2
   applies to it, and each unchecked response is recorded in the row with
   that exclusion:
   - the request is a STEP, IGES, proof, face-role or edge-treatment
     request (15.2, last item): it is still issued, and its response is not
     compared;
   - the request is a Boolean request with an operand that is edge-treated
     stock or is outside the surface class (15.2, third and fourth items).

   Dependence on an unchecked result. The general bridge may fail an
   unchecked request that the default bridge answered, and then publishes
   no handle for it. Phase 0 therefore follows handles through each trace:
   a request that uses a handle published by an unchecked request, directly
   or through later results, is itself unchecked, for the same item of
   15.2, when it is a request of the first kind above or a Boolean request
   whose operand is thereby edge-treated stock or outside the surface
   class. In the replay an unchecked request is issued when every handle
   it uses exists on the bridge being replayed and is skipped when one
   does not; a legacy result is never substituted into the general path.
   If a Boolean response that 15.2 does not exclude depends on an
   unchecked result, Phase 0 stops and asks the owner before freezing that
   trace.

   A function is dropped only when every Boolean response in its trace is
   unchecked for those reasons, and its row in the second table states the
   reason (A1). If a Boolean response would go unchecked for any other
   reason, Phase 0 stops and asks the owner; it does not drop or shorten
   the row.
6. Write `general_boolean_trace_test.go` with
   `TestGeneralBooleanTraces/<function>`: it replays each trace on a
   default bridge, mapping each recorded handle to the handle the same
   request publishes in the replay, and requires each response to equal
   the recorded one (failure code equal; numbers
   within 1e-12 relative). A `slow legacy` row starts with
   `slowtest.Skip(t)` in this test only.
7. Foundation probe (E1), `TestGeneralBooleanFoundation`: for one stored
   shape of each kind a Tier 1 row uses as an operand (planar solid,
   cylinder, bored body, analytic body), obtain the `trimmed.Solid` the
   adapter will use (L2) and check that `Classify` at tolerance 1e-7
   answers inside, outside and boundary correctly at analytic points, and
   that `MassProperties` at integration tolerance 1e-9 gives the volume and
   area the bridge reports within 1e-7 × max(1, |value|). The achieved
   errors are logged. This is test code over existing packages; it is not
   cylinder code of the trial (PL3).

   Publication probe (D4), `TestGeneralBooleanPublication`: take the
   `trimmed.Solid` of an existing result that has a void (a box less an
   inner box, lifted with `trimmed.FromPlanar`), store it as an analytic
   body through `publish` with `solidCount` 1, and issue `inspect`, `mesh`
   and `stl` on the handle. Each must succeed, the reported volume and area
   must equal the legacy values within 1e-7 × max(1, |value|), and the mesh
   must pass the checks named in step 8. If an existing constructor yields
   a solid of two separate components, the same is done for it with
   `solidCount` 2; if none does, the report says so. The probe is a
   diagnostic and is not part of G0. If it passes it is committed as a
   test. If it fails it is not committed: its output goes into the commit
   message and the report with D4, the trial does not stop, and the owner
   decides which bridge changes are permitted before Part 3.1.
8. Write `ACCEPTANCE.md`: one row per kept function with an identifier
   (`T1-001`…, `T2-001`…), function, file, tier, family, the Boolean
   requests it issues with their counts, the number of checked responses,
   of unchecked responses with their 15.2 reasons and of analytic values,
   the trace file, the flag `slow legacy`, and three empty columns for A8 (largest volume error,
   largest area error, outcome). A second table lists every dropped
   function with its reason. A third gives counts by tier and family beside
   the survey counts of 15.1. The header names the mesh checks the source
   tests apply to `mesh` responses (file and helper), which the replay of
   Part 3.1 reuses (A3), and the row that issues `pocketFloor` (CO4); if no
   kept row issues it, Phase 0 stops and asks the owner. The third table also
   gives, for each of the requests `box`, `prism`, `circle` and
   `mixedExtrude`, the number of checked Boolean responses that have an
   operand made by it; each must be at least one (specification section
   11, stored forms). It gives as well the number of Boolean responses left
   unchecked because an operand has faces stored as spline patches (S2,
   the arc-loop branch of `mixedExtrude`).

Gate G0, by command:

```sh
go test ./internal/plaincadbridge -run '^TestGeneralBoolean(Traces|Foundation)$' -count=1
test "$(ls internal/plaincadbridge/testdata/general_boolean/*.jsonl | wc -l)" -eq "$(rg -c '^\| T[12]-' specs/general-boolean/ACCEPTANCE.md)"
git diff --name-only "$BASE"...HEAD   # only the files listed above
```

The publication probe is run separately and is not part of G0:
`go test ./internal/plaincadbridge -run '^TestGeneralBooleanPublication$' -count=1`.
If the foundation probe fails on a valid operand,
ST5 applies and the trial stops before any kernel code is written. The report states the row counts,
the number of unchecked responses by reason, and the total size of the
traces.

## Phase 1: planar operands through the whole pipeline

Four parts. No cylinder chart, no circle or ellipse edge and no cylinder
row of 7.4 or 7.5 is written in this phase (PL3). The code is nevertheless
built on a carrier interface, a chart interface and the tables of 7.4 and
7.5, with the plane and the line as their only entries, so that Phase 2
adds entries and not a second path.

### Part 1.1: contract, intake, assembly and validation

Operands that do not interact: every face becomes one piece.

Files created in `brep/boolean`: `doc.go`, `options.go`, `errors.go`,
`result.go`, `combine.go`, `tolerance.go`, `carrier.go`, `chart.go`,
`intake.go`, `edge_carrier.go`, `surface_table.go`, `piece.go`, `selection.go`, `assembly.go`, `validation.go`,
tests beside them, `matrix_test.go`, `testdata/pending.txt`. Files created
in `internal/plaincadbridge`: `general_boolean.go`,
`general_boolean_test.go`. File changed: `internal/plaincadbridge/bridge.go`
(one unexported field on `Bridge`; one branch in the `cut`, `fuse`,
`common` case of `execute`).

Steps:

1. Options, their validation and `ErrInvalid` (P3, P4); the operation
   enumeration; the seven error values (section 9); the result type,
   one exported struct that holds the output solid, the shell grouping, the
   face source lists and the report (P5). Its fields use only built-in
   types, slices and arrays of them, and `trimmed.Solid`, so that no
   further exported type is needed (P2).
2. `Combine`: context and option checks, ε (T1), empty-operand and
   disjoint-box short cuts (I2), no partial result (P6), no shared state
   (P7).
3. Intake for planar operands (I1, I3, S1 to S6): the carrier and chart
   interfaces with the plane as the only carrier; `ErrUnsupported` naming
   the first face outside the class, which at this point includes every
   cylinder face. Edge carriers are derived by I5 and never from the Go
   type of the stored curve: the 17 samples; the check of each sample
   against both face carriers; the candidate line from the row of 7.4 for
   two planes that are not parallel, which starts the surface-pair table
   as data in `surface_table.go`; the line through the samples for an edge
   between two faces on one plane; the choice among candidates; the
   interval and direction, where on a line only the direction rule of I5
   step 6 applies and a step that goes backwards is refined up to six
   times; and the errors I5 names. A planar operand whose edge is not a
   line is `ErrUnsupported` until Part 2.1.
4. Pieces without splitting, the selection table (7.8), shell grouping and
   ordering (B1), output through `trimmed.NewSolidContext` (B3 for lines,
   B4), the empty result (B5), output edge curves as `geom.Line` values (B6),
   τ (T6).
5. Validation V1 to V6 and V8 with the error mapping for mass properties;
   the report fields.
6. The adapter, minimal: the test-only constructor in
   `general_boolean_test.go` (L3), conversion of a planar stored shape with
   `trimmed.FromPlanar`, the call to `Combine` with the tolerances of L4,
   publication, and the mapping of the section 9 errors to bridge failures.
7. `TestMatrix` with the leaves of C1 and C27, planar variant, including
   the face source checks of section 11. `TestEdgeCarrier`: a box whose
   edges are stored as evaluators of a type the package cannot name gives
   the same carriers as the same box with `geom.Line` edges; an edge
   between two faces on one plane; and the first failure test of section
   11, an edge that leaves one of its faces by 10ε at its middle
   (`ErrInvalid` naming the edge).
8. `TestContract`, table-driven, one behavior per case: each invalid and
   each non-finite option returns `ErrInvalid` (P4), `Angular` above 0.5
   among them; a placement with a non-uniform scaling returns
   `ErrUnsupported` (S4); a nil context and an
   operation value outside the enumeration return `ErrInvalid`; ε ≥ L / 100
   returns `ErrInvalid` (T1); a cancelled context returns the context error
   unwrapped, also when the operands are invalid or unsupported, so that
   it takes precedence (section 9); every error is matched with
   `errors.Is` and comes with the zero result (P6); the operands are
   unchanged after a call, compared by topology, bounds and tolerance
   (P7); and two calls with equal arguments return results that are deeply
   equal apart from the surface and curve evaluators (P7).

Acceptance, by command:

```sh
go test ./brep/boolean -run 'TestMatrix/^(C1|C27)$/^planar$' -count=1 -v | rg -c '^\s+--- PASS: TestMatrix/C(1|27)/planar/'   # prints 6
go test ./brep/boolean -run '^(TestContract|TestEdgeCarrier)$' -count=1
go test ./internal/plaincadbridge -run '^TestGeneralBoolean' -count=1
```

`TestGeneralBooleanAdapter` in the bridge asserts that a `cut` of a box by
a disjoint box gives, on the general bridge, the same outcome as on the
default bridge. Checks: K1 to K7.

### Part 1.2: the arrangement and the passes below face with face

Files created in `brep/boolean`: `arrangement.go`, `node.go`, `span.go`,
`pass_vertex.go`, `pass_edge.go`, `curve_table.go`, `region.go`, and tests.

Steps:

1. Nodes with radii, merging and the growth bound (T2 to T4, AR1); spans
   and node lists per edge (AR2); bundles (AR3); the per-face lists (AR5);
   the span and bundle records allow a span whose two ends are one node,
   although no planar operand has a closed edge and Part 2.1 is the first
   to create one;
   touching-contact records (AR7); the work limits of section 10 with
   `ErrBudget` and the context checks.
2. The curve table of 7.5 as data, with the rows line with line and line
   with plane, and edge overlap for lines (CC4): one candidate interval at
   most, degenerate when its mid point is within ε of both ends.
3. Passes X1 to X5 in order, and the tolerant point-in-loops test in the
   plane chart (X9). Pairs are filtered by boxes taken from the current
   node positions and inflated by the filter margin μ = `MaxGrowth` × ε
   (T4), in the short cut of I2 as well.
4. Unit tests on pairs of boxes: node, span and bundle counts for the
   configurations of C4 to C8, a growth-bound case returning
   `ErrTolerance`, a filtering case in which two vertices 1.5ε apart, each
   already in a node of radius 2ε, must still be found coincident, a case for each work limit of section 10 returning
   `ErrBudget`, and a context cancelled during the passes returning the
   context error in preference to a budget error that the same input would
   raise. `TestContract` gains the determinism and operand-immutability
   cases on interacting operands in Part 1.4, when they first produce a
   result.

Acceptance: `go test ./brep/boolean -run 'TestArrangement|TestPass|TestCurveTable|TestRegion' -count=1`
passes, and the two commands of Part 1.1 still pass. Checks: K1 to K7.

### Part 1.3: face with face, and splitting

Files created in `brep/boolean`: `pass_face.go`, `trace.go`, `split.go`,
and tests. File changed: `surface_table.go`.

Steps:

1. The surface-pair table of 7.4, begun in Part 1.1, with the two rows
   for parallel planes and the node-by-node same-surface test (SP4, AR6).
2. Pass X6 for planes, with traces cut at existing nodes and reuse of an
   existing span or bundle (AR4); X8.
3. Splitting in the plane chart: F1 to F6, with the degree rule of F2 and
   the turning rule of F3 written against the chart interface.
4. Unit tests: piece counts and areas for the faces of C4, C5, C6 and for
   two boxes that overlap in a corner; an inconsistent arrangement returning
   `ErrInconsistent`; a sliver returning `ErrTolerance`; and a divergent
   pair (T8, SP4), two faces parallel within `Angular` that are coincident
   at one end and more than 4ε apart at the other, returning
   `ErrTolerance`. The same test for two edges (CC4) goes with Part 1.2.

Acceptance: `go test ./brep/boolean -run 'TestSurfaceTable|TestFacePass|TestSplit' -count=1`
passes, and the earlier commands still pass. Checks: K1 to K7.

### Part 1.4: classification to merging, and the planar gate

Files created in `brep/boolean`: `classify.go`, `manifold.go`, `merge.go`,
`sources.go`, `matrix_poly_test.go`, and tests. Files changed:
`combine.go`, `selection.go`, `assembly.go`, `matrix_test.go`.

Steps:

1. Classification K1 to K3 with `trimmed.Solid.Classify`.
2. The full selection table including shared pieces (7.8).
3. The manifold test N1 to N3.
4. Merging M1, M3, M4 and M5 for planes, with the union of source lists
   (P5).
5. `TestMatrix`, planar variant, for C1 to C10 and C24 to C27 and the three
   operations, with the outcomes, volumes, face counts and face source
   lists that section 11 states.
6. `TestMatrixAgainstPoly`: for each of those leaves whose stated outcome
   is a solid or the empty solid, except those of C26, the volume equals
   the volume of `csg/poly.Combine` on the same operands within β of V5.
   The comparison is mandatory (PL2). If `poly.Combine` returns an error on
   such a leaf, the session stops and asks the owner (trip table).

Gate G1 (PL2), by command. The first prints 42 and the second prints
nothing:

```sh
go test ./brep/boolean -run 'TestMatrix/^(C1|C2|C3|C4|C5|C6|C7|C8|C9|C10|C24|C25|C26|C27)$/^planar$' -count=1 -v | rg -c '^\s+--- PASS: TestMatrix/C[0-9]+/planar/(union|common|cut)'
rg -n 'planar' brep/boolean/testdata/pending.txt
go test ./brep/boolean -run '^TestMatrixAgainstPoly$' -count=1
rg -n -i 'cylind|ellipse|circle|seam' brep/boolean -g '!*_test.go'   # only comments or the ErrUnsupported message of Part 1.1
```

Checks: K1 to K7. If G1 fails when the Phase 1 budget is used, no part of
Phase 2 starts (PL4). The corrective phase may be spent here.

## Phase 2: cylinders

Three parts. Starts only after G1.

### Part 2.1: cylinder carrier, conic edges and closed forms

Files created in `brep/boolean`: `cylinder.go`, `conic.go`,
`closed_forms.go`, and tests. Files changed: `carrier.go`, `chart.go`,
`intake.go`, `edge_carrier.go`, `surface_table.go`, `curve_table.go`,
`matrix_test.go`.

Steps:

1. The cylinder carrier and its chart with one seam (I4); intake of
   cylinder faces by their surface values (S2), rigid placements (S4), and
   the polar plane chart with its collapsed pole edge and its radial edge
   used twice, which is treated as a seam (I4).
2. The rows of 7.4 for a plane with a cylinder (SP1) and for parallel and
   coaxial cylinders, as far as they give carrier curves. Part 2.2 adds
   what X6 needs on top of them.
3. Edge carriers on curved faces (I5): candidates from those rows for an
   edge between a plane and a cylinder or between two cylinders; for a
   seam or an edge between two faces on one cylinder, the line through the
   samples or the curve of the plane through three samples with the
   cylinder; for a curved edge between two faces on one plane, the circle
   through three samples or the conic through five; the interval on a
   periodic carrier, with the refinement of I5 step 6 (added samples where
   an advance exceeds one eighth of a period or goes against the edge, at
   most six halvings of a step), and a closed edge with the whole period
   as its interval; output edge curves as `geom.Ellipse` values (B6).
4. The closed forms of CC1: circle with circle, the homothetic reduction
   (CC3), the projection of a conic along a cylinder axis and the proof
   that a conic is clear of a cylinder face (CC5), point-to-parameter
   inversion for a conic, segment-to-segment distance.
5. The remaining rows of the curve table of 7.5: a conic with a plane; a
   conic whose plane is parallel to a cylinder's axis; a conic that
   projects to a circle along the axis; conics in different planes;
   homothetic coplanar conics; and the two rows that return
   `ErrUnsupported`. No row refers to a cylinder that the conic's own
   faces lie on.
6. Edge overlap on a periodic carrier (CC4): intervals taken in the
   periodic parameter, so that one may run through the parameter origin or
   through a closed edge's vertex; two intervals for arcs that overlap at
   both ends; the whole overlap of two closed edges, tested at three points
   a third of the period apart; degeneracy by the mid point.
7. Closed spans and bundles of closed spans (AR2, AR3), and X3 on closed
   edges: inside an overlap the two edges have the same nodes and their
   spans correspond one to one; a whole overlap adds no node.
8. Unit tests of every closed form against analytic cases, including
   tangent and coincident ones. Overlap tests, each on two circle edges of
   one carrier: two full circles with the same vertex (one bundle of two
   closed spans); two full circles with different vertices (two bundles);
   two arcs that overlap at both ends (two intervals); an overlap that runs
   through the parameter origin; an overlap that covers all but a gap
   shorter than ε between its ends (not degenerate); a degenerate overlap
   (isolated point); and a pair that passes at some test points only
   (`ErrTolerance`). `TestMatrix` gains the cylinder variants of C1, C2, C3
   and C27 that need no surface pair; C2 in its cylinder variant exercises
   the whole overlap on both cap circles. `TestEdgeCarrier` gains the
   stored forms SF2 and SF3 and their placed versions (SF4), each of which
   must give the carriers and intervals of SF1 within ε, and the second
   failure test of section 11, a cylinder wall split along a helical edge
   (`ErrUnsupported` naming the edge). Two tests fix the sampling bound: a
   full circle whose stored parameterization puts three quarters of the
   turn in the first sixteenth of its range is accepted with the same
   carrier and interval as an even one, and a curve that runs forward and
   then back along its carrier returns `ErrUnsupported`.

Acceptance: `go test ./brep/boolean -run 'TestClosedForms|TestCurveTable|TestOverlap|TestIntake|TestEdgeCarrier' -count=1`
passes, and G1's commands still pass except the last, which no longer
applies. Checks: K1 to K7.

### Part 2.2: surface pairs and splitting on a cylinder

Files changed in `brep/boolean`: `surface_table.go`, `pass_face.go`,
`trace.go`, `split.go`, `region.go`, and tests. No new non-test file is
expected.

Steps:

1. On the plane with cylinder rows and the coaxial and parallel cylinder
   rows that Part 2.1 gave carrier curves: crossing and touching marks,
   candidate pairs and the node-by-node same-surface test (SP4).
2. The proof of separation (SP2) and `ErrUnsupported` for every other
   non-parallel pair (SP3). The Tier 2 row is not added.
3. X6 and X7 for these pairs: closed traces with their one node, traces
   marked touching, and a closed carrier curve with exactly one node,
   which is one interval that an existing closed span or bundle of closed
   spans occupies instead of a new trace (the cap circle of C19 and C21).
4. The point-in-loops test (X9) and splitting (F3, F4) in the cylinder
   chart, including ordering by curvature where tangents agree.
5. Unit tests: traces and pieces for a plane cutting a cylinder
   perpendicular, oblique and parallel to its axis, for two overlapping
   parallel cylinders, for the tangent configurations of C11 to C14, and
   for a cylinder standing on a plane face, where the base circle must be
   recorded for both faces and no trace created.

Acceptance: `go test ./brep/boolean -run 'TestSurfaceTable|TestFacePass|TestSplit' -count=1`
passes. Checks: K1 to K7. This is where SP5 is first tested: if a row of
7.4 could be added only by changing the code of sections 7.2, 7.3 or 7.6 to
7.11, the report says so, and ST6 applies if a Tier 1 row depends on it.

### Part 2.3: seams, conditioning, and the whole matrix

Files changed in `brep/boolean`: `merge.go`, `assembly.go`,
`validation.go`, `matrix_test.go`, and tests.

Steps:

1. Seam handling in merging (M2) and merging across cylinder pieces (M1,
   M4).
2. Output conditioning B2: arcs of at most a quarter turn, then up to six
   halvings, `ErrValidation` after that; p-curves of B3 with
   `trimmed.EllipseUV`.
3. Edge and vertex residuals on curved edges (V3, V4) and sampling (V6) on
   cylinder operands.
4. `TestMatrix` complete: every leaf of C1 to C27 in both variants where
   section 11 gives one, with outcomes, analytic volumes, stated face
   counts and face source lists (the four fixed lists of C4, C6, C15 and
   C21 included). C15 and C21 run in the six stored-form variants `sf1` to
   `sf3p`, with the same outcome, volume, face count and face source lists
   in all six; in the SF2 forms the radial edge of each polar cap is gone
   from the result (I4). C15, C16, C19 and C21 are the leaves that depend on
   coincident closed edges; C18 is run with 4ε ≤ d ≤ 2r − 4ε.

Checkpoint G2 (A5), by command. The first passes; the second and third
print nothing:

```sh
go test ./brep/boolean -run '^TestMatrix$' -count=1
grep -c . brep/boolean/testdata/pending.txt | rg -v '^0$'
go test ./brep/boolean -run '^TestMatrix$' -count=1 -v | rg 'ErrInconsistent|ErrValidation'
```

Checks: K1 to K7. G2 does not block Phase 3 and has no corrective phase of
its own. Leaves that still fail when the Phase 2 budget is used are named
in `brep/boolean/testdata/pending.txt` and in the report, are carried into
Part 3.2, and are due at G3; the specification lets section 11 rows be
closed up to the end of the last required phase (ST3).

## Phase 3: the bridge adapter and Tier 1

Two parts.

### Part 3.1: adapter and the acceptance replay

Files changed in `internal/plaincadbridge`: `general_boolean.go`,
`general_boolean_test.go`, and one branch each in `bridge.go` (`cut`,
`fuse`, `common`, already present), `cut_many.go`, `join_all.go`,
`overlap_volume.go` and `to_face.go`, each placed after the existing
argument validation (CO1). Files created:
`general_boolean_acceptance_test.go`,
`testdata/general_boolean/pending.txt`. No other bridge file changes; a
need for one is a stop-and-ask (table above).

Steps:

1. Conversion of every stored shape that holds or lifts to a
   `trimmed.Solid` (L2): planar solids, cylinders and patch solids, bored
   and analytic bodies. A shape with none, which includes edge-treated
   stock (OD-8), returns `UNSUPPORTED`. On the general bridge there is no
   fall back to a legacy path.
2. The compositions: `cutMany` and `joinAll` as atomic left folds (CO1),
   `overlapVolume` as the volume of the common (CO2), `prismToFace` and
   `circleToFace` as the common of the extended sweep with a block on the
   target plane (CO3).
3. Publication. A result is stored as an analytic body holding the result
   solid with its volume and area from `MassProperties`, which is the
   analytic-body path of L2, with `solidCount` taken from the shell
   grouping of the result. `putAnalyticBody` accepts only one shell, so for
   a result with voids or several solids the adapter stores the same
   analytic body through `publish` directly, from inside
   `general_boolean.go`. This is the plan's reading of L2. It is owner
   decision D4, informed by the publication probe of Phase 0, and Part 3.1
   does not start until D4 is settled.
4. `TestGeneralBooleanAcceptance/<tier>/<function>`: replays each trace on
   a general bridge and compares every checked response as A3 and A4
   require: success where the trace succeeded and failure where it failed;
   `valid` true; `solidCount` equal; volume and area within
   1e-7 × max(1, |expected|) of the analytic value where the trace has one
   and of the legacy value always; bounds within 1e-7; each replayed `mesh`
   response passing the mesh checks that `ACCEPTANCE.md` names from the
   source tests (Phase 0, step 8). Responses that Phase 0 recorded as
   unchecked are issued and not compared.
5. A8: the test logs, for each row, `A8 <row id> vol=<largest volume error> area=<largest area error> none=<count>`
   using the formula of A8, where the largest is taken over the row's
   checked responses that produced a value and `none` is the number that
   produced none.
6. `pocketFloor` (CO4): the replay compares the `pocketFloor` response of
   the general bridge with the recorded legacy response of the row that
   `ACCEPTANCE.md` names for it. They must be equal.
7. Run the replay, put every row that fails into the bridge pending list,
   and report the count by family with the first error of each.

Acceptance, by command:

```sh
go test ./internal/plaincadbridge -run '^TestGeneralBoolean' -count=1
go test ./internal/plaincadbridge -count=1          # the default bridge is unchanged
go test ./internal/plaincadbridge -run '^TestGeneralBooleanAcceptance$' -count=1 -v | rg -c 'A8 T1-'   # one line per Tier 1 row that produces a result
```

Checks: K1 to K7.

### Part 3.2: close Tier 1

Files changed: whatever general mechanism in `brep/boolean` a failing row
shows to be wrong, with a unit test for each defect; the two pending
lists. The adapter changes only to fix a conversion or composition defect.

Steps:

1. Take the pending rows family by family. For each failure find the
   smallest input that shows it, add it as a unit test or a matrix-style
   test in `brep/boolean`, and fix the stage of section 7 that is wrong.
2. Before each fix, apply K3 to the intended change. A fix that would
   depend on the family is not written (ST2).
3. Close any section 11 leaf carried over from Phase 2.
4. Run PC3 in full:

   ```sh
   go build ./... && go vet ./... && golangci-lint run --new-from-rev "$BASE" ./... && go test ./...
   go build -tags research ./... && go vet -tags research ./...
   go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples -check research
   ```

Gate G3, by command. The first two pass, the third prints nothing, and
step 4 passes:

```sh
go test ./brep/boolean -count=1
go test ./internal/plaincadbridge -run '^TestGeneralBoolean' -count=1
rg -n '^(T1-|TestMatrix)' brep/boolean/testdata/pending.txt internal/plaincadbridge/testdata/general_boolean/pending.txt
```

Checks: K1 to K7, and step 4. The bridge pending list may still hold Tier 2
rows; nothing else. If G3 fails when the Phase 3 budget is used, the
corrective phase follows if it has not been spent; otherwise ST3.

## Phase 4: Tier 2, optional

One part, one session. Starts only after G3. It is not required for a pass
(OD-1).

Files changed in `brep/boolean`: `surface_table.go`, `curve_table.go`,
`closed_forms.go` and their tests only (SP5). File changed in the bridge:
the pending list.

Steps:

1. Tag the commit that passed G3 locally as the point to return to.
2. Add the row of 7.4 for non-parallel cylinders of equal radius with
   meeting axes (two ellipses that meet at two nodes, X7), and any row of
   7.5 it needs.
3. Remove the Tier 2 rows from the bridge pending list and run the replay.

Acceptance, by command:

```sh
git diff --name-only <G3 commit>..HEAD -- brep/boolean | rg -v 'surface_table|curve_table|closed_forms'   # prints nothing
go test ./brep/boolean -count=1 && go test ./internal/plaincadbridge -run '^TestGeneralBoolean' -count=1
```

Tier 2 is *supported* when both commands hold with no Tier 2 row pending
and K2 to K5 still pass. Otherwise the session reverts to the G3 commit
(`git revert` of the Phase 4 commits, so history shows the attempt), leaves
the Tier 2 rows pending, and reports Tier 2 as unsupported with the reason;
if the reason is that a file outside the three named had to change, the
report says so as evidence under SP5. Neither outcome fails the trial.
Checks: K1 to K7.

## Phase 5: oracle and close

Two parts. The oracle run is the last verification step of the trial:
every build, test and standing check comes before it, and after it only
results are recorded.

### Part 5.1: oracle helper and cases

Files created: `tools/occtref/general_boolean.cxx`,
`internal/occtcheck/testdata/general_boolean_cases.json`,
`internal/occtcheck/general_boolean_oracle_test.go`.

Steps:

1. Write the case file. A case is a list of primitives (box, prism,
   cylinder) with placements and a list of operations applied left to
   right. Cases: every section 11 leaf whose stated outcome is a solid,
   except those of C24 to C26; and for each Tier 1 family the operands and
   the Boolean requests of the first row of that family in
   `ACCEPTANCE.md`.
2. Write the helper: it reads the case file, builds each case with the
   pinned OCCT and prints validity, volume, area, centroid, solid count and
   face count per case. It has no per-case code.
3. Write the Go test, `TestGeneralBooleanOracle`: enabled by an
   environment variable naming the built helper, named by the convention
   of `tools/occtref/README.md`; otherwise it skips. It
   builds each case with `brep` and `brep/parametric` primitives, calls
   `boolean.Combine`, and compares with the helper's output at the bound of
   `internal/occtcheck/oracle_test.go:68`, and face counts for the section
   11 cases that state one.

Acceptance: `go build ./... && go vet ./... && go test ./internal/occtcheck -count=1`
passes with the new test skipped. Checks: K1, K5, K6.

### Part 5.2: the oracle run, the A8 record and the close

The pass is asserted on one commit (specification 15.5). Call it the
*final code commit*: the last commit that changes any Go file. Steps 1 to
3 all run on it. The commit made in step 6 changes only `ACCEPTANCE.md`
and names the final code commit in its message.

Steps:

1. On the final code commit, run PC3 in full, as in step 4 of Part 3.2,
   the commands of G3, and K1 to K7. Nothing in this list is run again
   after step 3.
2. Take the A8 numbers from one run on that commit:

   ```sh
   go test ./internal/plaincadbridge -run '^TestGeneralBooleanAcceptance$' -count=1 -v | rg -o 'A8 T[12]-[0-9]+ vol=\S+ area=\S+'
   ```

   Each row of `ACCEPTANCE.md` gets its largest volume error, largest area
   error, the number of its checked responses that produced no value, and
   its outcome; a row that produced no result at all is marked as such.
   Below the table: the largest volume error, the largest area error, and
   the number of rows above 1e-8 in either (OD-7). `ACCEPTANCE.md` is where
   this record lives.
3. Build the helper with the pinned OCCT as `tools/occtref/README.md`
   describes, and run the oracle test once (OD-5), with the environment
   variable that Part 5.1 defined naming the built helper:
   `go test ./internal/occtcheck -run '^TestGeneralBooleanOracle$' -count=1 -v`.
4. If a case differs beyond the bound, triage it before anything else,
   without running the oracle again. Each differing property (validity,
   volume, area, centroid, solid count, face count) is judged on its own,
   against evidence that is independent of both the kernel and the oracle:
   the analytic value or stated count of section 11, or the legacy value in
   `ACCEPTANCE.md`.
   - The independent evidence covers the property and agrees with the
     kernel: the case data or the helper is the likely fault for that
     property. The session reports it and asks the owner whether to
     authorize a second oracle execution after a fix.
   - The independent evidence covers the property and agrees with the
     oracle: the kernel returned a wrong result that passed section 8. If a
     general strengthening of section 8 would have caught it, the session
     reports that strengthening and asks the owner whether the corrective
     phase, if unspent, is to be used for it. If none would, ST1 is met and
     the trial stops.
   - No independent evidence covers the property, or it agrees with
     neither: the outcome is inconclusive. The session reports both values
     and the case, assigns no fault, and asks the owner for direction.
5. Add an "Oracle run" section to `ACCEPTANCE.md` with one line per case:
   the two volumes, the two areas, the difference, pass or fail. No file is
   added under `docs/evidence` unless the owner merges the trial and the
   verification record is to cite it (RC2). Delete any build or stage
   directory the run created.
6. Commit `ACCEPTANCE.md`, and report to the owner: pass or fail against
   PC1 to PC4 on the final code commit; the Tier 2 outcome; the A8 summary;
   the bridge paths and kernel constructors that no acceptance row needs
   any longer (specification 15.5); what the pass does not cover, namely
   edge-treated stock and the follow-up of OD-8; and what was not run.

Gate G4, by command: the oracle test of step 3 exits with status 0 on the
final code commit, and `git diff --name-only <final code commit>..HEAD`
prints only `specs/general-boolean/ACCEPTANCE.md`. No code changes after
the oracle run. A code change, or a second oracle execution, needs the
owner's authorization.

The trial does not merge itself, does not make the general path the bridge
default, and does not delete a legacy path. Each of those needs a separate
request from the owner.

## Corrective phase

There is one, of at most two sessions (ST3, PL4). It is entered when G1 or
G3 (which includes section 11 leaves carried from Phase 2) has failed with
its phase's budget used. Its scope is the failing leaves or rows of that
gate and nothing else, under the same rules and checks as Part 3.2.

Acceptance: the commands of the gate that failed, as written in that gate,
and K1 to K7. If they pass, the plan continues from the next phase and no
corrective phase remains. If they do not, the trial stops under ST3.

An oracle difference at G4 is handled by the triage of Part 5.2, step 4,
not by this phase unless the owner directs it there.

## Where each requirement is implemented

| Specification | Part |
|---|---|
| P1 to P7, T1, T6, S1 to S6 (planar), I1 to I3, I5 (lines), 7.8 (unshared), B1, B3 to B6, V1 to V8, section 9, L1, L3, L4 | 1.1 |
| T2 to T5, T7, AR1 to AR3, AR5, AR7, X1 to X5, X9 (plane), CC2, CC4 (lines), section 10 | 1.2 |
| AR4, AR6, SP4 (planes), X6, X8, F1 to F6 (plane) | 1.3 |
| K1 to K3, 7.8 (shared), N1 to N3, M1, M3 to M5, P5 sources, T8, PL1 to PL4, C1 to C10 and C24 to C27 (planar) | 1.4 |
| S2 to S4 (cylinders), I4, I5 (curved edges), SF1 to SF4 (carriers), the carrier curves of the 7.4 rows, CC1, CC3, CC4 (periodic carriers), CC5, closed spans of AR2, AR3 and X3, 7.5 complete | 2.1 |
| SP1 to SP4, 7.4 Tier 1 rows, X6 and X7 (curved, including a closed carrier with one node), X9 and F3 (cylinder chart) | 2.2 |
| M2, B2, B3 (curved), SF1 to SF4 (matrix), section 11 complete (A5) | 2.3 |
| L2, CO1 to CO4, A2 to A4, A8 logging | 3.1 |
| PC1 (without A6), PC2 to PC4, A7 | 3.2 |
| SP5, the Tier 2 row of 7.4, Tier 2 rows | 4 |
| L6, A6, A8 record, PC1 complete, RC1, RC2 | 5.1, 5.2 |
| A1, E1 probe | 0 |
| G1 to G4, AR8, ST1 to ST6, OD-1 to OD-8 | Rules, standing checks and the trip table, in every part |

## Risks the plan carries

- The foundation (E1). Whether `trimmed.NewSolidContext` accepts every
  conditioned result and whether `Classify` is right on every operand at
  tolerance ε is not known. Phase 0 probes the operands; results are first
  exercised in Part 1.1 (planes) and Part 2.3 (cylinders). A failure is ST5.
- Accuracy (OD-7). Numerical integration may not reach 1e-7 on every row.
  Phase 0 logs what `MassProperties` achieves on the operands, before any
  kernel code exists.
- Classification shares `Classify` with V6 (specification section 8). A
  wrong `Classify` answer is exposed only by analytic and legacy values and
  by the oracle run, which is the last step. A late ST1 is possible.
- Later requests on a general-path result. The general bridge stores every
  result as an analytic body, also when all its faces are planar. A
  replayed `mesh`, `inspect` or `pocketFloor` on such a result goes through
  the existing analytic-body code, which was written for one shell and for
  specific recipes. A row that fails for that reason may need a bridge
  change outside the listed files, which is a stop-and-ask.
- The adapter cap. `prismToFace` on an oblique target needs a block built
  on an arbitrary plane; with the conversions and folds, 400 lines is
  tight. Exceeding it is ST4.
- Unchecked responses. A row whose trace contains requests outside the
  trial checks fewer responses than its source test asserts. Phase 0
  reports how many responses this affects, by reason.
- Size of Parts 1.3, 2.1 and 2.2. These hold the hardest pieces
  (splitting, overlap on a periodic carrier, and splitting on a periodic
  chart) and are the parts most likely to need a second session.

## Owner decisions

The owner decided these on 2026-11-18. None is open except D4, which waits
for a probe.

- D1. Branch. The specification and this plan are merged to `main` first,
  and the implementation branch `codex/general-boolean` is created from that
  commit of `main` (R1).
- D2. Session budgets. The table in "Session budgets" is accepted: 21 units
  at most for the planned phases and 2 for the corrective phase. One session
  may run all the parts of a phase one after another and stops at the phase
  gate (R2); each part it attempts uses one unit.
- D3. Trace capture. Phase 0 records the existing tests by a temporary,
  uncommitted patch to `bridge.go` and `bridge_test.go`, and commits the
  reduced traces as test data. Rewriting each acceptance function by hand
  was not chosen.
- D4. Publication of results with voids or several solids. Open until Phase
  0 has probed it. L2 says results are published through the existing
  analytic-body path, and `putAnalyticBody` rejects more than one shell. The
  plan has the adapter call `publish` directly for those results, without
  changing `analytic.go`. Phase 0 probes whether `inspect`, `mesh` and `stl`
  work on such a body and reports; the owner then decides this, and which
  bridge changes are permitted if they do not, before Part 3.1.

  Phase 0 has run the probe. Its results are recorded here as input for
  that decision, which is still the owner's:
  - A body with a void, and a body of two solids, each stored by calling
    `publish` directly, work with `inspect`, `mesh`, `stl` and `clone`.
    Volume and area match the legacy values to about 3e-16.
  - `transform` fails on such a body with "expected one connected analytic
    shell", because `bridge.go` republishes the transformed body through
    `putAnalyticBody`. One Tier 1 row transforms a result with a sealed
    void, so that row cannot pass with the adapter file alone.
  - Seven frozen `common` responses are empty solids. `putAnalyticBody`
    rejects an empty solid, and the legacy path publishes one with `put`.
    How the adapter publishes an empty result is therefore part of this
    decision.
  - `trimmed.Solid.Bounds` pads the box by the solid's tolerance. A result
    carries τ ≥ 1e-7 (L4), so its published bounds miss the 1e-7 check of
    A3 unless the adapter removes the pad or A3's bound allows for it.
- D5. Face source lists. They are a field of the result with no reader
  outside tests during the trial. `AGENTS.md` asks every exported identifier
  to have an outside caller; the owner asked for this field, and that
  request covers it.
- D6. Line cap. 15,000 non-test lines in `brep/boolean` (PC4), raised from
  12,000 because this plan estimates 11,300 for the required work. The
  per-part figures in "Size budget" stay as warnings.

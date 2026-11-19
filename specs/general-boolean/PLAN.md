# Plan: general Boolean trial

This plan implements [the specification](SPEC.md) as it stands at commit
`17f95af0`. It is a plan only and contains no implementation.
Identifiers such as PC2, A3 or ST4 are requirement identifiers of the
specification; RL-1 to RL-9 are the rulings of its section 19.

State after Phase 2. Phase 0 is complete: gate G0 passed on `b37228e6`.
Parts 1.1 to 1.4 are complete: gate G1 passed on `dde63dee` for the rows it
then named. Phase 2 is complete to its checkpoint: G2 was taken on
`4fb6a3d4`, with 118 of 123 matrix leaves giving their stated outcome. All
of it is on the trial's branch (R1). Phase 2 was built from this plan as
it stood at `ecaae04d`, before Part 1.5 was added, so the rule for a face
that touches itself (M6) is not in the code yet. Part 1.5 is the next
part and Phase 3 follows it. The sections on finished parts describe what
was done, so that a later session can rely on them.

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
| 1 | 5 | Planar operands through the whole pipeline (PL1) | G1: the planar gate of PL2; passed for C1 to C10 and C24 to C27, and extended to C28 by Part 1.5 |
| 2 | 3 | Cylinders: carriers, charts, tables, seams | G2, a checkpoint that does not block: every leaf of section 11 (A5); leaves that fail are due at G3. Taken on `4fb6a3d4` |
| 3 | 2 | Bridge adapter and Tier 1 rows | G3: PC1 without the oracle run, PC2, PC3, PC4 |
| 4 | 1 | Tier 2, optional (OD-1) | none; kept or reverted |
| 5 | 2 | Oracle helper, the one oracle run, the A8 record, close | G4: the oracle run (A6), PC1 complete |

There are six phases and fourteen parts. One corrective phase exists
(section "Corrective phase"). No part of Phase 2 starts before G1 passes
(PL3). G1 passed before row C28 existed. Part 1.5 adds that row and its
rule (RL-8). Phase 2 ran before Part 1.5 reached its branch, so Part 1.5
now comes after Part 2.3 and before Phase 3, and builds on the merging
code as Part 2.3 left it. Phase 4 starts only after G3 passes. The oracle run is the last step
of the plan.

## Rules for every part

These come from `AGENTS.md` as it reads on `main` and from the
specification. They apply to every part and are not repeated in the parts.

- R1. Work is on one branch, `codex/general-boolean-kernel`, in its own
  worktree (RL-6). Phase 1 ran on it beside Phase 0, which ran on
  `codex/general-boolean`; that branch was then merged into it and is not
  used again. `BASE` is `ecaae04d`, the commit of `main` that both were
  cut from or had merged (D1). A later amendment of the specification or
  of this plan reaches the branch by a merge of `main` and does not move
  `BASE`. Nothing is merged to `main` or pushed unless the owner asks.
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
- R4. No file in `geom` is edited, and no commit of the trial edits a file
  in `brep/trimmed` (G4, L5). A limit of `brep/trimmed` that a valid
  result exposes is fixed by separate work on the foundation branch,
  `codex/trimmed-collinear-corners`, and reaches the trial branch only by
  a merge of that branch (RL-9). A session of this plan does not write
  those fixes. When the foundation branch has been merged, the next
  session runs K1 and `go test ./brep/trimmed -count=1` once, removes from
  the pending lists the cases that now pass, and names them in its
  report. No file that
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
  `slowtest.Skip` (A7). The two rules time different things: A7 times a
  row or a leaf, and the repository rule times a test function. A test
  function that holds many rows, such as `TestGeneralBooleanTraces` and
  `TestGeneralBooleanAcceptance`, meets the repository rule by running its
  rows as parallel subtests (RL-7). Neither is in `brep/trimmed`, where
  `t.Parallel()` is not allowed.
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
  its placement, other than through the recognition of surface values and
  rigid placements at intake (S2, S4), sections 7.3 to 7.5, the work limits
  of section 10 and the one test that an operand has no face (I2), and
  that the constraints of AR8 hold, including traversal in index order
  with no randomness (no iteration over a map decides an output). Three
  searches are aids to that review. They do not replace it, and passing
  them proves nothing by itself.

  The first looks for a function or method whose name contains the name
  of an arrangement, and must print nothing. It matches the name only,
  not the parameter or result types, so a signature that mentions
  `geom.Box` is not printed. A false match is a name in which the word
  means something else, such as a helper called `box` that computes a
  bounding box; rename it or say so in the commit message.

  ```sh
  rg -n -i '^func (\([^)]*\) )?\w*(bore|pocket|notch|slot|boss|crossed|counterbore|box|cube)\w*[\[(]' brep/boolean -g '!*_test.go'
  ```

  The second looks for a count of faces, edges, vertices or shells: a
  `len` whose argument names one, or a read of `Topology()`. It matches
  the expression and not the file path, so a `len` in `edge_carrier.go`
  or `surface_table.go` is not printed for its file name. Every line it
  prints must be a section 10 limit check, the empty-operand test of I2,
  the adapter's reading of the shell grouping for `solidCount` (L2), a
  loop bound, or the size of a table that is being allocated. A line
  that branches on a count for any other purpose is what K3 forbids. The
  search does not see a count held in a variable, which is why the diff
  is read.

  ```sh
  rg -n -i 'len\([^)]*(face|edge|vert|shell)[^)]*\)|Topology\(\)' brep/boolean internal/plaincadbridge/general_boolean.go -g '!*_test.go'
  ```

  The third covers S3: the Go type of an edge's stored curve is never
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

- K5. Boundaries (R4, R7). The first four print nothing; the fifth prints
  only the identifiers P2 allows and no other: `Combine`, the operation type
  and its three values, the options type, the result type, and the seven
  error values:

  ```sh
  git log --oneline "$BASE"..HEAD --not codex/trimmed-collinear-corners -- brep/trimmed
  git diff --name-only "$BASE"...HEAD -- geom go.mod go.sum
  rg -l '^//go:build research' brep/boolean internal/plaincadbridge/general_boolean*.go
  git status --porcelain
  go doc -short plainsolid/brep/boolean
  ```

  The first command lists the commits that touch `brep/trimmed` and are
  not commits of the foundation branch. A merge of that branch brings
  its changes to `brep/trimmed` without being listed; a commit of the
  trial that edits the package is listed, and R4 forbids it.
- K6. Stop conditions. The session checks ST1 to ST6 against the table in
  "When a check or a stop condition trips" and states the outcome in the
  commit message.
- K7. Pending lists. `brep/boolean/testdata/pending.txt` and
  `internal/plaincadbridge/testdata/general_boolean/pending.txt` name the
  leaves and rows that are expected to fail at this point. A listed case
  that fails is logged and not counted as a test failure; a listed case
  that passes is a test failure, so the list cannot hide a passing case. A
  case not listed must pass. The commit message gives the number of entries
  in each list. Each gate says which entries it allows. An entry may carry
  the reason `RL-9` and the limit after its name, for a case that fails
  only through a limit of `brep/trimmed` (trip table): `collinear
  corners`, `two arcs` or `mesh edge`, the three that E1 lists. Such an
  entry is allowed at every gate except the pass (PC1), is named in every
  report, and is not counted as passing.

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
| `Classify` gives a wrong answer on a valid operand, or a row can pass only by changing `geom` or adding a dependency | ST5 | Stop. Do not edit those packages. |
| `trimmed.NewSolid` or `NewSolidContext` rejects, or cannot represent, a result that meets B2 and B3: one of the three limits E1 lists, or a new one | E1, RL-9 | Do not stop, do not work around it, and do not edit `brep/trimmed` on the trial branch. Enter the leaf or row in its pending list with the reason `RL-9` and the limit, and report its smallest input for the work on the foundation branch. It is not a pass and is not counted toward ST3 or ST5 until the owner rules. A limit that E1 does not list yet is reported so that it can be added. |
| `Combine` returns `ErrUnsupported` from M6 on a leaf or a Tier 1 row: a face touches itself and has no divider | M6 | The case fails. Report the input. Do not add a vertex beyond the quarter-turn cut, or a case for it; extending M6 is the owner's decision. |
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
| 1 | 5 | 6 |
| 2 | 3 | 5 |
| 3 | 2 | 4 |
| 4 | 1 | 1 |
| 5 | 2 | 3 |
| Corrective | 1 | 2 |

Phase 1 gained Part 1.5 after its first four parts had used four units. Its
budget stays at six, which leaves two for that part; raising it is the
owner's decision (D2). Phase 2 used three of its five units.

A phase whose gate has not passed when its budget is used has failed its
gate. Two phases differ. G2 is a checkpoint: when the Phase 2 budget is
used, leaves that still fail are carried into Part 3.2 and are due at G3.
Phase 4 has no gate: when its one session ends without Tier 2 supported,
the change is reverted.

## Size budget

Cumulative non-test lines in `brep/boolean` at the end of each part, in
the order the parts are done: measured for the finished parts, expected
for the others. The expected figures are warnings, not caps. A part that
ends more than 15 percent above its line says so in its report, with the
reason. The cap is 15,000 (PC4) and only the cap trips ST4.

| Part | 1.1 | 1.2 | 1.3 | 1.4 | 2.1 | 2.2 | 2.3 | 1.5 | 3.2 | 4 |
|---|---|---|---|---|---|---|---|---|---|---|
| First estimate | 1,600 | 3,100 | 4,700 | 6,200 | 8,500 | 9,900 | 11,100 | none | 11,600 | 12,200 |
| Measured | 1,479 | 2,550 | 3,113 | 3,605 | 4,811 | 4,825 | 5,160 | | | |
| Expected now | | | | | | | | 5,450 | 5,700 | 6,000 |

Phase 2 ended at 5,160 lines against a first estimate of 11,100, which is
46 percent. Its three parts added 1,555 lines where the first estimates
added 4,900, because both charts and both tables turned out to be one
piece of code over the terms of a carrier. What remains is estimated from
that: 300 lines for Part 1.5, which now also covers curved loops and the
cylinder chart, and half of the first increments for Part 3.2 and Phase 4
(250 and 300). The required work is then expected near 5,700 lines, under
40 percent of the cap. The cap is the owner's (D6) and is not changed
here. The adapter file is 92 lines of its 400; Part 3.1 is where it grows.

## Phase 0: freeze the acceptance rows

One part. No kernel code. Checks: K1 (the bridge lines), K5 (the lines
that do not name `brep/boolean`), K6.

Done: G0 passed on `b37228e6`, with 109 rows (92 in Tier 1, 17 in Tier
2), 3 functions dropped under 15.2 and 14 that issue no Boolean request.
The steps below are kept as the record of how the rows were made. Where
the work departed from the steps as first written, the step now says what
was done, and the departures are listed after the gate.

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
   committed, to `internal/plaincadbridge` that appends one JSON line per
   event to the file named by an environment variable: a new bridge, a
   request with its response, and each `near(got, want)` pair, each with
   the source position of the call. The patch touched 15 bridge files,
   not the two first planned. Many source tests do not send every
   operation through `Bridge.Call`: they call the bridge method that a
   request dispatches to. The patch therefore also wrapped ten bridge
   methods, and a direct call of `cutMany`, `overlapVolume`,
   `mixedExtrude`, `importSTEP` or `exportSTEP` is recorded as the
   equivalent request (owner decision, 2026-11-18). Calls of constructors
   that have no request form are in no trace. Run each function alone
   with `-run '^Name$' -count=1 -parallel 1`. A function that is skipped
   in the default tier is run once with `PLAINSOLID_SLOW=1` and marked
   `slow legacy` in its row.
3. Revert the temporary patch (`git checkout -- internal/plaincadbridge`
   for the patched files) and confirm `git status --porcelain` lists only
   new files.
4. Reduce each raw trace to its committed form, one JSON object per line.
   The schema is fixed before any trace is captured:
   - `{"new":true}` at each new bridge.
   - `{"req":…, "reqText":…, "ok":…, "code":…, "handle":…, "result":…, "analytic":…, "unchecked":…, "direct":…}`
     for each request. `req` is the request exactly as issued, with the
     handles it refers to; `reqText` replaces it when the input the test
     sent is not one JSON object. `unchecked` names the exclusion of
     section 15.2 that keeps this response out of the comparison on a
     general bridge (step 5). `direct` marks a request that the source
     test made by calling the bridge method (step 2). These three fields
     were added during the freeze; the comment at the top of
     `general_boolean_trace_test.go` is the definition of the format. `ok`
     says whether it succeeded and `code` is the
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
     belongs. That reading used the recorded source positions and the
     parsed `got` expression of each call: 603 pairs attached by the rule,
     406 matched nothing and were dropped, and of 765 ambiguous pairs 737
     were placed and 28 left out because the source reads a mesh, face or
     bounds value there.

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
     compared. A shape that `importStep` publishes is an ordinary operand
     afterwards, and a Boolean response on it is checked (owner decision,
     2026-11-18);
   - the request is a Boolean request with an operand that is edge-treated
     stock or is outside the surface class (15.2). Two responses are
     unchecked for the second reason, in `TestFullTubeSTEPAndCoaxialCut`
     and `TestQuarterAnnularSTEPAndFurtherCut` (RL-4).

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
   `slowtest.Skip(t)` in this test only; 13 rows are. Run serially the
   test takes 8 to 10 seconds in the default tier, with no row above 0.7
   seconds, so its rows run as parallel subtests (R6, RL-7). Each subtest
   has its own bridge, and nothing is shared between rows.
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

The third command is run with `BASE=ecaae04d`. With the older base
`9d7417dc` it also lists `SPEC.md` and `PLAN.md`, which reached the branch
by a merge of `main`.

The publication probe is run separately and is not part of G0:
`go test ./internal/plaincadbridge -run '^TestGeneralBooleanPublication$' -count=1`.
If the foundation probe fails on a valid operand,
ST5 applies and the trial stops before any kernel code is written. The report states the row counts,
the number of unchecked responses by reason, and the total size of the
traces.

What the freeze found, beyond the departures written into the steps:

- The traces hold 3,118 requests, 653 of them recorded from direct calls,
  with 2,685 checked responses and 433 unchecked (424 STEP requests, 7
  face-role requests, 2 Boolean responses).
- Checked Boolean responses by the request that made an operand: `box`
  528, `prism` 44, `circle` 1,114, `mixedExtrude` 31. No Boolean response
  is unchecked for spline-patch faces (RL-3).
- Size. The traces are 7.9 MB, 91 percent of it STEP text in `importStep`
  requests. This is accepted on the trial branch. Before any merge of the
  trial to `main`, the distinct STEP sources are stored compressed or are
  otherwise reduced (RL-5); that is work for the merge request, not for a
  part of this plan.
- Two Tier 2 rows take about 2.1 seconds on the legacy path, most of it
  STEP import and export that a general bridge runs too. A7 applies to
  them in Phase 4 as to any row.

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
   step 6 applies and a step that does not advance is `ErrUnsupported` at
   once, with no added sample; and the errors I5 names. A planar operand whose edge is not a
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
   11, an edge that leaves one of its faces by 10ε at nine sixteenths of
   its range (`ErrInvalid` naming the edge). The deviation is not at the
   middle because `trimmed.NewSolid` samples edges at the eighths of
   their range and would not build the operand; the exact middle is
   tested on the edge alone, below `Combine`.
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
and tests. Files changed: `surface_table.go`, and `arrangement.go`, which
the first version of this list left out: the arrangement gains the trace
records and the order of the passes.

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
`combine.go`, `selection.go`, `assembly.go`, `matrix_test.go`, and two
that the first version of this list left out: `piece.go`, where the
stand-in builder of Part 1.1 was removed, and `split.go`, where an
interior point is now sought at the quarter points of a chord as well as
its middle (K2 needs a point off the other operand's boundary in C9 and
C10).

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

G1 passed on `dde63dee`: 42 leaves, 41 of them compared with `csg/poly`,
and C26 returning `ErrTolerance` for all three operations, which T8
allows. Part 1.5 extends the gate to C28.

### Part 1.5: a face that touches itself

Added after G1 (RL-8) and done after Phase 2. A stress test outside the
plan found that splitting can make a piece whose loop passes twice through
one node, and that `trimmed.NewSolid` rejects it. The specification has
M6 for it. This part also brings the code in line with two other changes
the specification made after Phase 2 was planned.

Before it starts, `main` with the amended specification and plan is
merged into the trial branch.

Files created in `brep/boolean`: `divide.go` and its test. Files changed:
`merge.go`, `assembly.go`, `edge_carrier.go`, `matrix_test.go`,
`matrix_poly_test.go`, `testdata/pending.txt`, and `chart.go` if the
chart interface needs a method that says whether a chord between two
chart points is a carrier in the edge class.

Steps:

1. Merging keeps the loops of an unmerged piece as F3 traced them. As
   Part 2.3 left it, `merge.go` lays a face out by the same walk as
   splitting and returns `ErrValidation`, with its own message, for a
   single piece that visits a point twice. That refusal goes: such a
   face reaches M6 with its loop intact.
2. M3 as the specification states it: a merge is refused when the merged
   face would have a repeated corner and performed when it removes one.
   Unit tests for both.
3. The quarter-turn cut of B2 is made before M6 and not inside the
   building of the solid, so that its vertices are corners M6 can use.
   The halving rounds of B2 stay where they are.
4. M6 in `divide.go`, after M4 and the quarter-turn cut: corners and
   their numbering; candidates by (a) to (d), with (a) asked of the chart
   (any chord in a plane chart; a chord along a ruling or around a circle
   in the cylinder chart) and the crossing test of (b) made with the rows
   of the curve table; the choice; the division with the loops reassigned
   as in F4; the repetition; `ErrUnsupported` when no candidate exists
   and `ErrInconsistent` when two loops of one face share a chart point.
   M7 after it.
5. `TestDivide`, table-driven on faces built directly: a rectangle less a
   triangle that touches its edge (one divider, the shortest admissible);
   a pair excluded by (c) although it is shortest; a loop that passes
   three times through one point (two dividers); a hole loop that
   touches itself, where the division cuts a new face off and leaves the
   old one with one hole; two chords of equal length, decided by the
   order of step 3 of M6; a circle that touches a straight edge, with the
   circle's own vertex at the point of contact, divided at a vertex of
   the quarter-turn cut; a chord in the cylinder chart; and a face with
   no candidate.
6. `TestMatrix` gains the three planar leaves of C28 with the volumes,
   the face counts, the end points of the two dividers and the source
   lists that section 11 states. `TestMatrixAgainstPoly` gains C28 union
   and common. The union of C20 now states a solid of volume V(A) + V(B)
   whose top face is two faces, and its entry leaves the pending list.
7. The direction rule of I5 step 6 as the specification now states it:
   the direction of an edge on a circle or ellipse is the one that more
   of the sixteen first advances have, and on a line a step that does not
   advance is refused without added samples. `TestEdgeCarrierFailures`
   gains a full circle that puts nine tenths of the turn in the first
   sixteenth of its range, which must be accepted. Like the other two
   sampling tests it calls the carrier routine directly.
8. The two stress tests of the Phase 2 note are run once more,
   uncommitted, and the commit message says how many of the 148 planar
   cases of this kind now return a solid and what the others return. It
   gates nothing.

Acceptance, by command. The first prints 3 and the second prints 1. In
G1's first command C28 is added to the list of rows, and it then prints
45:

```sh
go test ./brep/boolean -run 'TestMatrix/^C28$/^planar$' -count=1 -v | rg -c '^\s+--- PASS: TestMatrix/C28/planar/(union|common|cut)'
go test ./brep/boolean -run 'TestMatrix/^C20$/^cyl$/^union$' -count=1 -v | rg -c '^\s+--- PASS: TestMatrix/C20/cyl/union'
go test ./brep/boolean -run '^(TestDivide|TestMerge|TestMergeSeam|TestEdgeCarrierFailures|TestMatrixAgainstPoly)$' -count=1
go test ./brep/boolean -run '^TestMatrix$' -count=1
```

Checks: K1 to K7. The pending list of `brep/boolean` then holds the three
leaves of C18 and nothing else, each with the reason `RL-9 two arcs`. If
`csg/poly.Combine` returns an error on C28, the trip table applies.

## Phase 2: cylinders

Three parts. Starts only after G1.

Done, to checkpoint G2 on `4fb6a3d4`. The parts below read as they were
planned; each has a note of what was done differently, and the results
of G2 follow Part 2.3. Phase 2 was built from this plan at `ecaae04d`
and the specification at `c96f79a5`, so it does not hold what later
amendments added: M6, and the direction rule of I5 step 6 by the
majority of the first advances. Part 1.5 adds both.

### Part 2.1: cylinder carrier, conic edges and closed forms

As done (`115ed341`): this part also took most of the code of Part 2.2,
because the pieces could not be tested apart: closed and touching traces
with SP2 and SP3 in `pass_face.go` and `trace.go`, crossing counts on
either chart in `region.go`, and the walk on a cylinder in `split.go`.
Files changed outside the list below: `arrangement.go`, `assembly.go`,
`pass_face.go`, `pass_vertex.go`, `region.go`, `split.go`, `trace.go`,
`span.go`, `pass_edge.go`. A carrier states its period and its terms, and
both charts are the same code over those terms. The two tests of the
sampling bound in step 8 call the carrier routine directly, because
`trimmed.NewSolid` samples an edge at the eighths of its range and would
reject such a curve in a solid. The four changes to the planar code that
step 1 lists were made; the box of a curved face is the box of its
boundary edges padded by the sagitta of their sampled arcs.

Files created in `brep/boolean`: `cylinder.go`, `conic.go`,
`closed_forms.go`, and tests. Files changed: `carrier.go`, `chart.go`,
`intake.go`, `edge_carrier.go`, `surface_table.go`, `curve_table.go`,
`matrix_test.go`.

Steps:

1. The cylinder carrier and its chart with one seam (I4); intake of
   cylinder faces by their surface values (S2), rigid placements (S4), and
   the polar plane chart with its collapsed pole edge and its radial edge
   used twice, which is treated as a seam (I4). Four things in the planar
   code of Phase 1 are right for lines and planes only and change here:
   - The box of an edge or a face is built from the edge samples. A
     curved edge and a curved face reach outside that box, and T4 lets a
     box discard a pair only if it contains its entity. Boxes of curved
     edges and faces are widened to contain them, with a unit test in
     which two faces meet only where one bulges past its edges.
   - The curve interface has no period, and the nearest parameter on a
     carrier is clamped to the interval. Both are wrong on a circle or
     ellipse.
   - Making spans (X10) returns `ErrTolerance` for an edge whose two ends
     are one node. A closed edge is that by construction and makes a
     closed span instead.
   - Interior points of a piece (F6, K2) are sought on chart chords at
     their middle and quarter points. This is kept unless a leaf shows it
     to miss a piece that has an interior point.
2. The rows of 7.4 for a plane with a cylinder (SP1) and for parallel and
   coaxial cylinders, as far as they give carrier curves. Part 2.2 adds
   what X6 needs on top of them.
3. Edge carriers on curved faces (I5): candidates from those rows for an
   edge between a plane and a cylinder or between two cylinders; for a
   seam or an edge between two faces on one cylinder, the line through the
   samples or the curve of the plane through three samples with the
   cylinder; for a curved edge between two faces on one plane, the circle
   through three samples or the conic through five; the interval on a
   periodic carrier, with the direction and refinement of I5 step 6 (the
   direction is the one that more of the sixteen first advances have;
   samples are added where an advance exceeds one eighth of a period or
   goes against that direction, with at most six halvings of a step), and
   a closed edge with the whole period
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
   carrier and interval as an even one; and a curve that runs forward and
   then back along its carrier returns `ErrUnsupported`. A third case, a
   circle that puts nine tenths of the turn in the first sixteenth, came
   into the specification after this part and is made in Part 1.5.

Acceptance: `go test ./brep/boolean -run 'TestClosedForms|TestCurveTable|TestOverlap|TestIntake|TestEdgeCarrier' -count=1`
passes, and G1's commands still pass except the last, which no longer
applies. Checks: K1 to K7.

### Part 2.2: surface pairs and splitting on a cylinder

As done (`d7376002`): the code of this part went in with Part 2.1, so
this part added its tests, `TestFacePassCurved`, and one change they
showed, that a span found by both X5 and X6 is listed once in a face.

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
print nothing, apart from entries and log lines of cases marked `RL-9`
(K7):

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

G2 as taken on `4fb6a3d4`. 123 leaves ran; 118 gave their stated outcome
and none returned `ErrInconsistent`. The planar gate still held.

| Leaf | Returned | Why, and where it goes |
|---|---|---|
| C18 union, common, cut | `ErrValidation` | Each cap is bounded by arcs of two circles, which `brep/trimmed` cannot hold in one face (E1, RL-9 `two arcs`). Pending until the foundation branch is merged. |
| C20 union | `ErrValidation` | The top face touches itself. Section 11 then stated `ErrNonManifold`, which was wrong; it now states a solid, and Part 1.5 makes it one. |
| C23 union | `ErrValidation` | Section 11 states no outcome for it, so the leaf checks nothing. The display mesh of `brep/trimmed` does not close (E1, RL-9 `mesh edge`). |

Departures from the steps above, accepted by the reviewing session and
now written into the specification where it states the rule:

- B2. When the solid is refused, every arc of the solid is halved, not
  only the arcs of the loop that failed, because the error from
  `trimmed.NewSolid` does not reliably name the loop. The refusal
  reported after six rounds is the first one.
- K1. An interior point that falls on a seam or on the radial edge of a
  polar cap of the other face is passed over and the next is tried. The
  first cylinder stress run failed here.
- Source lists across stored forms are compared by operand. In the form
  SF3 the wall of a cylinder is four stored faces, so a merged wall names
  four faces where another form names one.
- Integration. The matrix integrates at relative tolerance 1e-9 with
  `MaxSamples` 64,000,000. With the default of 4,000,000 the tube of
  C17, the common of C23 and one more result with an oblique section run
  out of samples and return `ErrBudget`. L4 now states the larger budget
  for the bridge, and Part 3.1 measures what it costs.
- Seams. There is no fixed seam angle: a face that winds around its axis
  is cut along a chain of removed elements that merging puts back, so a
  seam is kept only while its face winds (M2).
- The matrix leaves and the rows of the trace replay call `t.Parallel()`
  (R6).

## Phase 3: the bridge adapter and Tier 1

Two parts.

### Part 3.1: adapter and the acceptance replay

Files changed in `internal/plaincadbridge`: `general_boolean.go`,
`general_boolean_test.go`, and one branch each in `bridge.go` (`cut`,
`fuse`, `common`, already present), `cut_many.go`, `join_all.go`,
`overlap_volume.go` and `to_face.go`, each placed after the existing
argument validation (CO1). One more change to `bridge.go` is allowed, in
the `transform` request (step 3, RL-2). Files created:
`general_boolean_acceptance_test.go`,
`testdata/general_boolean/pending.txt`. No other bridge file changes; a
need for one is a stop-and-ask (table above).

Steps:

1. The options of L4, in one place in the adapter: `Linear` 1e-7,
   `Angular` 1e-9, `Volume` 1e-7, and integration at relative tolerance
   1e-9 with `MaxSamples` 64,000,000. The adapter of Part 1.1 passes no
   sample budget, and the default runs out on results with curved caps.
   Conversion of every stored shape that holds or lifts to a
   `trimmed.Solid` (L2): planar solids, cylinders and patch solids, bored
   and analytic bodies. A shape with none, which includes edge-treated
   stock (OD-8), returns `UNSUPPORTED`. On the general bridge there is no
   fall back to a legacy path.
2. The compositions: `cutMany` and `joinAll` as atomic left folds (CO1),
   `overlapVolume` as the volume of the common (CO2), `prismToFace` and
   `circleToFace` as the common of the extended sweep with a block on the
   target plane (CO3).
3. Publication (L2, L3, D4 as decided by RL-2). A result is stored as an
   analytic body holding the result solid with its volume and area from
   `MassProperties`, with `solidCount` taken from the shell grouping of
   the result.
   - A result that is not empty: the adapter stores the analytic body by
     calling `publish` directly, from inside `general_boolean.go`.
     `putAnalyticBody` accepts one shell only, so it cannot take a result
     with a void or with several solids, and it computes the bounds with
     the pad described below. The probe of Phase 0 showed that `inspect`,
     `mesh`, `stl` and `clone` work on a body of several shells stored
     this way. The adapter of Part 1.1 publishes a result of one shell
     through `putAnalytic`; that changes here, because of the bounds.
   - An empty result: through `put` with an empty solid, as the legacy
     paths publish one. Seven frozen `common` responses are empty.
   - Bounds: `trimmed.Solid.Bounds` widens the box by the solid's
     tolerance, and a result carries τ ≥ 1e-7. The adapter publishes the
     box with that pad removed on every side, so that the 1e-7 check of
     A3 compares the geometry.
   - `transform`. In `bridge.go` the `transform` request republishes an
     analytic body through `putAnalyticBody`, which fails for more than
     one shell, and one Tier 1 row transforms a result with a sealed
     void. One minimal change there is allowed: a transformed analytic
     body of more than one shell is republished as the adapter publishes
     one. A body of one shell takes the path it takes today. The change
     is verified by `go test ./internal/plaincadbridge -count=1`, which
     holds the existing tests of the default bridge and the trace replay
     on it, and by a test in `general_boolean_test.go` that transforms a
     result with a void on a general bridge.
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
   and report the count by family with the first error of each. A row
   that fails only for a limit of `brep/trimmed` carries the reason
   `RL-9` and the limit (K7). The families Bore union and Side dual disk
   are expected to meet the limit `two arcs` until the foundation branch
   is merged.
8. The cost of integration (L4, A7). Run K4 on the replay and report
   every row at or above one second with its time, and for each row at or
   above two seconds how much of that time is spent in `MassProperties`,
   measured with a CPU profile of that row that is not committed. Such a
   row fails A7 and goes into the pending list with that reason. The
   sample budget and the tolerance are not lowered to make a row pass;
   changing L4 is the owner's decision.

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

Gate G3, by command. The first two pass, the third prints nothing but
entries marked `RL-9` (K7), and step 4 passes:

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
   PC1 to PC4 on the final code commit; every leaf or row still pending
   under `RL-9`, with its limit and its input: while one is pending PC1
   does not hold and the trial has neither passed nor stopped, so the
   report says what the pass waits for; the Tier 2 outcome;
   the A8 summary;
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
request from the owner. A merge to `main` has one condition already set
(RL-5): the distinct STEP sources inside the frozen traces, which make up
most of their 7.9 MB, are first stored compressed or otherwise reduced.

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
| T2 to T5, T7, AR1 to AR3, AR5, AR7, X1 to X5, X9 (plane), X10, CC2, CC4 (lines), section 10 | 1.2 |
| AR4, AR6, SP4 (planes), X6, X8, F1 to F6 (plane) | 1.3 |
| K1 to K3, 7.8 (shared), N1 to N3, M1, M3 to M5, P5 sources, T8, PL1 to PL4, C1 to C10 and C24 to C27 (planar) | 1.4 |
| M3 as restated, M6, M7, C28, the union of C20, the direction rule of I5 step 6 | 1.5 |
| S2 to S4 (cylinders), I4, I5 (curved edges), SF1 to SF4 (carriers), the carrier curves of the 7.4 rows, CC1, CC3, CC4 (periodic carriers), CC5, closed spans of AR2, AR3 and X3, 7.5 complete | 2.1 |
| SP1 to SP4, 7.4 Tier 1 rows, X6 and X7 (curved, including a closed carrier with one node), X9 and F3 (cylinder chart) | 2.2 |
| M2, B2, B3 (curved), SF1 to SF4 (matrix), section 11 complete (A5) | 2.3 |
| L2, CO1 to CO4, A2 to A4, A8 logging | 3.1 |
| PC1 (without A6), PC2 to PC4, A7 | 3.2 |
| SP5, the Tier 2 row of 7.4, Tier 2 rows | 4 |
| L6, A6, A8 record, PC1 complete, RC1, RC2 | 5.1, 5.2 |
| A1, E1 probe | 0 |
| G1 to G4, AR8, ST1 to ST6, OD-1 to OD-8, RL-1 to RL-9 | Rules, standing checks and the trip table, in every part |

## Risks the plan carries

- The foundation (E1, RL-9). Three limits of `brep/trimmed` are known:
  collinear corners in a tilted face, arcs of two curves at one vertex of
  a face, and a display mesh that does not close on the union of C23. The
  second blocks C18 and probably the Tier 1 families Bore union and Side
  dual disk, so a pass depends on the work on the foundation branch,
  which is outside this plan. Until that branch is merged those leaves
  and rows are pending and PC1 cannot hold. Whether `Classify` is right
  on every operand at tolerance ε is still known only from the probes and
  the matrix; a wrong answer is ST5.
- Faces that touch themselves (M6). The rule joins two existing corners,
  those of the quarter-turn cut included. A face can still lack a
  divider, and then returns `ErrUnsupported`. No leaf or Tier 1 row is
  known to need more; if one does, it fails and the owner decides whether
  M6 is extended (trip table).
- The cost of integration (L4). The sample budget that makes results
  with curved caps converge at 1e-9 is sixteen times the default. Whether
  every Tier 1 row then stays under two seconds is not known until Part
  3.1 measures it.
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

The owner decided D1, D2, D3, D5 and D6 on 2026-11-18. D4 was decided on
the same day by the reviewing session under the owner's delegation, as were
the other rulings of section 19 of the specification; each of those stands
until the owner confirms or reverses it. Where this plan applies them:
RL-1 and RL-9 in R4, K5, K7 and the trip table; RL-2 in D4 and Part 3.1; RL-3 and RL-4 in
Phase 0, step 5; RL-5 after gate G0 and in the close of Part 5.2; RL-6 in
R1; RL-7 in R6 and Phase 0, step 6; RL-8 in Part 1.5.

- D1. Branch. The specification and this plan are merged to `main` first,
  and the implementation branch is created from that commit of `main`.
  The branch is now `codex/general-boolean-kernel` (R1, RL-6).
- D2. Session budgets. The table in "Session budgets" is accepted: 21 units
  at most for the planned phases and 2 for the corrective phase. One session
  may run all the parts of a phase one after another and stops at the phase
  gate (R2); each part it attempts uses one unit.
- D3. Trace capture. Phase 0 records the existing tests by a temporary,
  uncommitted patch to `bridge.go` and `bridge_test.go`, and commits the
  reduced traces as test data. Rewriting each acceptance function by hand
  was not chosen.
- D4. Publication of results. Decided by RL-2 (2026-11-18, by the
  reviewing session under delegation, for the owner to confirm or
  reverse). The adapter stores a result with a void or several solids by
  calling `publish` directly; an empty result goes through `put`, as on
  the legacy path; the adapter removes the tolerance pad of
  `trimmed.Solid.Bounds` before it publishes bounds; and one minimal
  change to the `transform` path of `bridge.go` is allowed, so that it
  republishes an analytic body of more than one shell, with the behavior
  for one shell unchanged and verified by the existing tests of the
  default bridge and the trace replay. `analytic.go` is not changed. Part
  3.1, step 3 carries this out.

  The decision rests on the probe that Phase 0 ran:
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

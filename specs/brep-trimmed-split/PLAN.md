# Plan: separating `brep/trimmed`

Measured on `main` at `58c27fa4`, 2026-11-18. Numbers come from
`tools/refgraph` (a type-checked cross-file reference graph) and from the
symbols linked into a native build of `internal/plaincadbridge`.

## Terms

- **Default tier**: `go test ./...` with no build tag and without
  `PLAINSOLID_SLOW=1`. It is what runs for every change.
- **Complete suite**: the same with `PLAINSOLID_SLOW=1`, and after Phase 2
  also `-tags research`.
- **Core**: production files of `brep/trimmed` reachable from a root.
  **Research**: all other production files of the package.
- **Root**: anything a non-test Go file outside `brep/trimmed` uses from it,
  excluding everything under `examples/`: a function, type, method or field,
  and every method needed when a package type is used as an interface. Roots
  are found by type-checking the calling packages, because a method call or
  an interface conversion does not name the package.

## Goal

1. In a default local build, editing kernel code that PlainCAD uses should
   not rebuild and retest code that PlainCAD does not use, and editing
   research code should not invalidate default test results. CI is exempt: it
   compiles research code on every push so that it cannot rot.
2. The default tier for `brep/trimmed` should take under 60 s of wall time on
   the 14-core development machine.
3. No change to behavior, and no change to the code linked into the PlainCAD
   kernel.

## Compatibility boundary

Compatibility is promised to the packages in this repository and to PlainCAD.
The module path `plainsolid` is provisional and the README calls the kernel
experimental, so there are no outside importers to protect. After Phase 2 the
exported names declared in research files are absent from the default build
on purpose; a caller that wants them builds with `-tags research`. The owner
approved this as decision D1.

## Non-goals

- No new capability, no API redesign, no deletion of code from history.
- Not a split of every family into its own package. The measurements below
  show that is not worth its cost.

## Findings

`brep/trimmed` has 1,101 production files (219,406 lines) and 928 test files.

| Fact | Value |
|---|---|
| Largest group of mutually dependent files | 400 files, 96,153 lines, including `solid.go`, `persistence.go`, and 98 `spline_graph` and `algebraic_shell` files |
| Files that depend on that group and that nothing in it depends on | 665 files, 113,606 lines |
| Files with no dependency either way | 28 files (`native_prism`) |
| Files reachable from any non-example package outside `brep/trimmed` | 561 files, 124,961 lines ("core") |
| Files reachable from no outside package | 540 files, 94,416 lines ("research") |
| References from core files to research files | 0 |
| Unexported names research files use from core | 873 (438 fields, 298 functions, 72 methods, 55 types, 10 other) |
| Methods research files declare on core types | 326 (125 on `Solid`) |
| File pairs to cut before the 400-file group falls below 20 files | about 350 (about 1,000 references) |
| Functions linked into the PlainCAD bridge | 784 of 4,807, in 246 of 1,101 files |

Test time in `brep/trimmed`, split by whether a test file touches research
files:

| Tier | Core tests | Research tests |
|---|---|---|
| Default tier | 567 tests, 63 s | 568 tests, 62 s |
| Complete run | 4,225 s | 9,779 s |

The package's default tier takes 261 s of wall time because its tests run
one at a time; only 10 `t.Parallel` calls exist in `brep/trimmed` and
`exchange/step` together.

## Decision

A package boundary between core and research would require exporting about
870 internal names and rewriting 326 methods as functions. That makes the
public API worse to buy a compile-time boundary. A build tag gives the same
separation with no API change, because core never references research.

So the plan is: parallelize tests in place, quarantine research files behind a
build tag, then decide what to do with the quarantined code. Breaking up the
400-file group is deferred and has explicit entry criteria.

## Phase 0: baseline and tooling

1. Record the baseline with one pinned toolchain (`go version` recorded, and
   `GOFLAGS`, `GOEXPERIMENT` and `CGO_ENABLED` unset or recorded):
   - `sha256` of the PlainCAD WASM built with
     `GOOS=js GOARCH=wasm go build -trimpath -buildvcs=false ./cmd/plaincadwasm`.
     Build it twice from a clean cache directory and require identical hashes.
   - The linked-code fingerprint of the bridge: a native build of a one-line
     `main` that calls `plaincadbridge.New().Call`, then the sorted list of
     `go tool nm -size` text symbols with their sizes, and the size of the
     code section.
   - The `go test -list '^(Test|Fuzz|Example)'` inventory for every package.
   - Default-tier wall time, measured three times.
2. Extend `tools/refgraph` with a classifier, written in Go so that
   `go test ./tools/refgraph` covers it. Inputs: the reference graph of
   `brep/trimmed` and the roots as defined above. Rules:
   - Core is the transitive closure of the root-defining files over every
     reference kind `refgraph` reports, in non-test files. Research is the
     rest.
   - Test code is classified per top-level declaration, to a fixed point: a
     test, helper, type or variable declared in a test file is research if
     its signature, initializer or body references a research production
     file or another research test declaration, through any number of files.
   - A test file is research if any declaration in it is research. A file
     that mixes core and research declarations is split first, so that no
     core declaration ends up behind the tag.
   - An external-package test file, in the directory or in another package,
     is research if it names an exported symbol declared in a research file.
   - `refgraph` runs on the default configuration for `darwin/arm64`,
     `linux/amd64` and `js/wasm`; the three results must agree.
3. Write the resulting lists to `specs/brep-trimmed-split/`:
   `core-files.txt`, `research-files.txt`, `research-tests.txt`,
   `mixed-tests.txt` (research test files that also hold core declarations)
   and `outside-files.txt`, so the membership is reviewed, not just counted.
4. Give the classifier a test with a small fixture package covering a
   core-looking test that reaches research code only through helpers in two
   other files, a helper file shared by core and research tests, an
   external-package test, and an excluded `examples/` root.
5. Dry-run the quarantine without editing any source: use `go build -overlay`
   to delete the research production files and the outside non-test files;
   it must succeed for every package. Then build the tests of `brep/trimmed`
   with every research test file deleted except those in `mixed-tests.txt`;
   every error must be in a mixed file.

`refgraph` models static references only: calls, type and field uses, and
method selections on a concrete type. It does not model a method reached only
through an interface value, a type assertion, or reflection. The package has
no `init` function, so there is no registration at start-up. Three checks
cover what the graph cannot see: a core type that loses a method it needs to
satisfy an interface fails the default build; the code linked into the
PlainCAD bridge is compared function by function; and every
`.(interface{...})` assertion and `reflect` use in the module is reviewed
against the 326 methods that research files declare on core types.

Timing is always measured the same way: `go test -count=1 ./brep/trimmed`
with a warm build cache and the default `-parallel`, three runs, median
reported. `-count=1` disables test-result caching.

Acceptance: the classifier test passes; the tool reports 561 core and 540
research files with 0 core-to-research references on `darwin/arm64`,
`linux/amd64` and `js/wasm`; the lists are committed; the dry run in step 5 succeeds; the review of assertions and
reflection is recorded in `specs/brep-trimmed-split/BASELINE.md` with the
baseline figures.

## Phase 1: run default-tier tests in parallel

1. Add `t.Parallel()` as the first statement of every default-tier top-level
   test in `brep/trimmed` and `exchange/step` that does not call `t.Setenv`,
   `os.Setenv` or `os.Chdir` (none do today) and does not write a
   package-level variable. Tests that start with `slowtest.Skip` stay serial
   in this plan, so the complete suite's behavior is unchanged and does not
   need a new qualification run for this phase.
2. Before adding the calls, audit shared state: list every package-level
   `var` in the two packages' test files and every production cache they
   reach, and list tests that write files outside `t.TempDir()`. A test that
   writes shared state or a shared path stays serial, or gets its own fixture.
3. Run `go test -race -shuffle=on -count=3 ./brep/trimmed ./exchange/step`
   and the same without `-race` at `-parallel 14` and `-parallel 2`. Any test
   that fails only in parallel stays serial with a comment naming the state.

Acceptance: the three runs in step 3 pass; the test inventory is unchanged;
the median default-tier time for `brep/trimmed`, measured as in Phase 0, is
under 60 s on the 14-core machine. The complete suite also runs these tests in
parallel from now on, so Phase 1 is confirmed for the complete suite by the
next complete run the owner requests, at the latest the one in Phase 2.

This phase is independent of the others.

### Outcome (2026-11-18)

- `exchange/step`: 1,296 default-tier tests now call `t.Parallel()`. Its
  default tier went from 135 s to 51 s. Three shuffled race-detector passes
  found no data race, and six runs at the default parallelism and one at
  `-parallel 2` passed.
- `brep/trimmed`: **not parallelized.** Parallel tests made an existing fault
  frequent enough to block: the test binary aborts with "found pointer to free
  object" or "found bad pointer in Go heap". The fault is not a data race in
  the tests: the race detector finds none under Go 1.23.12, and the abort
  reproduces with one goroutine and `GOMAXPROCS=1`. It needs no parallelism
  at all; one serial run in three aborted during this work. Reproducer:
  `go test -count=40 -run ProjectiveSplineRoofAdjacentStep ./brep/trimmed`
  aborts within seconds on macOS ARM64 with Go 1.25.14, 1.26.8, 1.27.1 and
  1.27.4, and passes with Go 1.23.12 and with `GOGC=off`. The module has no
  `unsafe`, cgo, assembly or compiler pragma, so this points at the Go
  toolchain; it has not been reported upstream.
- The same feature's test `TestBuildProjectiveSplineRoofAdjacentStepFuse`
  fails outright with Go 1.26.0 on Linux ARM64 and Go 1.26.4 on macOS.
- Six tests in `brep/trimmed` were in the default tier by mistake. They run
  their subtests in parallel, so `go test` reported 0.01 s for tests that take
  3 to 50 s. `tools/ci/mark-slow-tests.py` now measures wall time, and those
  six are in the slow tier. The package's default tier went from 261 s to
  about 116 s.
- Goal 2 (under 60 s for `brep/trimmed`) is therefore not met by this phase.
  Phase 2 is expected to meet it without parallelism, because research tests
  are about half of the remaining time, and the adjacent-step feature that
  reproduces the abort is research code.
- The test inventory is unchanged apart from the new `TestClassifyFixture`.

## Phase 2: quarantine research files behind a build tag

Prerequisites: decisions D1 and D3, both approved on 2026-11-18.

1. Add `//go:build research` to the 540 research production files.
2. Split the 126 in-package test files in `mixed-tests.txt`. Each holds
   research declarations next to a core test, or next to a helper that a core
   test in another file uses. Move the research declarations to a new
   `*_research_test.go` file beside it and leave the core ones in place.
3. Add the tag to every test file that then holds a research declaration:
   498 in-package files before the split, and 24 of the 29 external-package
   test files in the directory. Rerun the classifier; `mixed-tests.txt` must
   be empty.
4. Tag the 33 files in other packages listed in `outside-files.txt`: test
   files in `exchange`, `exchange/step`, `exchange/iges`, `document` and
   `internal/occtcheck`, and the `examples/tubelens` program. Some of those
   test files also declare helpers that core tests of the same package use;
   the Phase 0 dry run found `runOracle` in `internal/occtcheck`,
   `checkTwoCirclesExchangeReadback` in `exchange` and
   `rationalStationSTEPFixture` in `exchange/step`. Move each such helper to
   an untagged test file first.
5. Keep research code compiling automatically, and its tests under an
   explicit manual gate:
   - `go.yml` (every push and pull request): keep the default build, vet and
     smoke steps, and add `go build -tags research ./...` and
     `go vet -tags research ./...`. These compile research code without
     running its tests, so drift is caught at the push that causes it.
   - `go-qualification.yml` and `tools/ci/run-go-qualification.py` (on
     demand): pass `-tags research` to every `go test`, `go vet` and
     inventory command. Research tests run only here. Qualification is
     started by the owner and is required before a release; nothing runs
     research tests automatically, and the documentation must say so.
   - Document the tag and these triggers in `tools/ci/README.md` and
     `AGENTS.md`.

Acceptance:

- `go build ./...` and `go vet ./...` pass with and without `-tags research`.
- Without the tag, `brep/trimmed` compiles 561 production files.
- The linked-code fingerprint of the bridge equals the Phase 0 baseline: the
  same text symbols, each with the same size, and the same code-section size.
  The WASM file itself is expected to shrink by about 14 KB, because type
  descriptors no longer list the research methods of core types and the line
  tables no longer name research files. Record its new size and hash.
- `go test ./internal/plaincadbridge` passes with `PLAINSOLID_SLOW=1`.
- With `-tags research`, the test inventory equals the Phase 0 inventory.
- The default tier passes, and `go test -tags research ./...` passes.
- A push to a scratch branch shows the two new `go.yml` steps running.
- Cache experiment: after a full default-tier run, touch one research file
  and rerun; `go test ./...` reports every package as cached. Then touch one
  core file and rerun; only `brep/trimmed` and its importers rerun, and they
  run core tests only.
- One complete suite run with `-tags research`, requested from the owner,
  passes with the Phase 0 inventory before this phase is called done.
- `tools/ci/smoke.sh` passes without the tag; any smoke case that lands in
  research is replaced by a core case.

Expected effect: default builds compile 44% less of the package, the default
tier runs about half the `brep/trimmed` tests, and an edit to a research
file no longer invalidates the default test cache for `brep/trimmed` or its
dependents.

### Outcome (2026-11-18)

Done, except for the complete-suite run, which is the owner's to request.

- 540 production files and 524 test files of `brep/trimmed` carry
  `//go:build research` (500 in-package, 24 external-package). 98 mixed test
  files were split first; their research declarations are in new
  `*_research_test.go` files.
- In other packages, 35 test files and `examples/tubelens` are tagged
  (`csg/offset` 2, `exchange` 13, `exchange/iges` 1, `exchange/step` 10,
  `internal/occtcheck` 8, `scene` 1). 15 of those are the research halves of
  mixed test files that were split the same way.
- Without the tag `brep/trimmed` compiles 561 production files.
  `go build ./...` and `go vet ./...` pass with and without the tag.
- Linked-code fingerprint of the bridge: identical to the baseline, 7,138
  text symbols with the same sizes, code section 6,554,116 bytes. The WASM is
  now 22,083,545 bytes, `sha256 8f296054…95f65`.
- `PLAINSOLID_SLOW=1 go test ./internal/plaincadbridge` passes.
- Test inventory with the tag: the baseline's 6,908 plus `TestClassifyFixture`.
  Without the tag: 5,998.
- Default tier: `brep/trimmed` 63 s, 64 s, 64 s (baseline 261 s); the whole
  module 78 s of wall time (was 281 s). Goal 2 asked for under 60 s for
  `brep/trimmed`; it misses by about 4 s.
- `go test -tags research ./...` (default tier plus research tests) passed
  once, in 138 s.
- `tools/ci/smoke.sh` passes without the tag; none of its cases is research.
- Cache experiment: after editing a research file, all 55 packages report
  cached and the run takes 1 s. After editing a core file, 17 packages rerun
  (`brep/trimmed` and its importers) in 90 s, and 38 stay cached.
- `go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples
  -check research` passes and runs in the new `research` job of `go.yml`,
  which also builds and vets with the tag. That job has not run on GitHub yet.
- The review lists from Phase 0 are removed from this directory. The tags are
  now the record: `rg -l '^//go:build research'` lists the quarantine.

Three things differ from the plan as written. The classifier had to learn
that a method call or an interface conversion from another package is a root.
A core test file also loses the helpers that only research tests use, so more
files were split than first counted. And one `scene` test needed the tag
because it passes a `trimmed` value as an interface whose method is research;
the classifier does not see interface use inside test files, and the default
`go vet` caught it.

## Phase 3: decide the fate of the quarantined code

This is a decision for the repository owner, made after Phase 2 has run for a
while. The options, cheapest first:

- Keep it tagged. It stays compiled and vetted on every push, and its tests
  run only in qualification.
- Move it to a long-lived branch and delete it from `main`. History and the
  `refs/archive` refs keep it.
- Promote it to a real package. Cost: export about 870 names and convert 326
  methods. Not recommended.

Acceptance: the owner's choice is recorded in `docs/STATUS.md` with its date.
If the choice is the branch: the branch exists on the remote; the tagged
files, tests and example in other packages are deleted with it; the
`research` tag is removed from `go.yml`, the qualification workflow, the
launcher and the documentation; and `go build ./... && go vet ./...` and the
default tier pass on `main`.

## Phase 4 (deferred): break up the 400-file group

Entry criteria, all required: Phases 1 and 2 are done; a core edit still costs
more than two minutes of default-tier wall time; and a specific PlainCAD task is
blocked or slowed by it.

Method: replace the direct calls from hub files into family files with
registration. `solid.go` reaches 11 files and `persistence.go` 19, mostly
through tessellation, validation and format dispatch. Cut one hub at a time
and rerun `refgraph`; the metric is the size of the largest strongly connected
component. Only when it is below about 50 files is a package split of core
worth planning.

Acceptance for each hub cut: the largest component is smaller than before,
the default tier passes, and the linked-code fingerprint from Phase 0 still
holds.

## Decisions for the owner

- **D1.** Is it acceptable that research exported names leave the default
  build? **Approved by the owner on 2026-11-18.**
- **D2.** The fate of the quarantined code (Phase 3).
- **D3.** The original gate for Phase 2 was a byte-identical PlainCAD WASM.
  The Phase 0 dry run showed that cannot hold: removing research files leaves
  every linked function and its size unchanged but shrinks type descriptors
  and line tables, so the file differs. This plan now uses the linked-code
  fingerprint plus the bridge tests instead. **Approved by the owner on
  2026-11-18.**

## Risks

- **Hidden shared state in tests.** Parallel tests may race on cached
  fixtures. Mitigation: the race detector gate in Phase 1.
- **Two build configurations drift.** Research code can stop compiling
  unnoticed. Mitigation: the qualification workflow and the vet step build
  with the tag.
- **Tooling that ignores tags.** Editors and linters see only the default
  configuration. This is intended for agents working on PlainCAD; anyone
  working on research code sets `GOFLAGS=-tags=research`.
- **The research set is defined by today's callers.** A consumer that starts
  using a research symbol fails to compile without the tag. That failure is
  the signal to untag the file deliberately.
- **Symbol-level figures are approximate.** Inlined functions do not appear as
  linked symbols, so the 784 figure undercounts. No phase depends on it.

## Adjacent finding, out of scope

The PlainCAD bridge links no function from `exchange/iges`, `document`,
`csg/offset` or `csg`, and 2,685 of 13,256 functions across the module. The
same quarantine could apply to whole packages. That is a separate decision.

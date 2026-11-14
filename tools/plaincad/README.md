# PlainCAD acceptance adapter

This opt-in harness runs PlainCAD's existing browser acceptance fixtures against
PlainSolid compiled to Go WebAssembly. It makes a fresh temporary app copy and
never changes the source PlainCAD checkout. There is no OCCT or faceted fallback
in the replacement adapter.

Run from the PlainSolid checkout (including an integration worktree):

```sh
node tools/plaincad/run.mjs \
  --source /absolute/path/to/PlainCAD \
  --work /tmp/plaincad-plainsolid-run
```

The source app must already have its npm dependencies and Playwright Chromium
installed. The default selection is the three existing XY/XZ/YZ signed and
symmetric extrusion cases in `e2e/extrusion-hole.spec.ts`. To select a different
existing gate:

```sh
node tools/plaincad/run.mjs \
  --source /absolute/path/to/PlainCAD \
  --work /tmp/plaincad-plainsolid-shell \
  --port 5294 --test e2e/shell.spec.ts --grep 'native Shell'
```

`--work` must be empty or nonexistent. The default is a fresh timestamped
temporary directory. Development uses port 5279 by default. The harness builds its own WASM
binary, copies the matching Go JavaScript runtime, typechecks the app, runs a
backend preflight, and then runs the selected existing fixtures. It requires
local server and browser process permissions. Fixtures that pin their storage
origin use the original port. The harness keeps fixture source unchanged.

Pass `--suite production --test production-e2e --grep '.*'` to build the
staged app and run its original production configuration on port 5280.
Use `--suite cross-browser --test cross-browser-e2e --grep '.*'` for the
original Chromium, Firefox and WebKit configuration on port 5281.

Use `--suite unit` to run the original Vitest suite with real PlainSolid WASM
initialized in each test file. It defaults to one worker to reduce contention
with solver deadlines. `--test modelingGeometry.test.ts` selects a file and
`--grep` filters test names. `--workers` allows an explicit concurrency check.
The unit run preserves the original test assertions and reports every failure,
including fixtures that assume OCCT-private bindings or an uninitialized
kernel. This mode uses the local WASM file and does not start a browser;
original middleware fixtures still require loopback access.

The opt-in side-opening overlap fixture runs through the actual application
commands, viewer and exchange workers, and is recorded separately from
original PlainCAD fixtures. Enable it with `--side-dual-disk-fixture --test
e2e/plainsolid-side-dual-disk.spec.ts --grep 'Go WASM overlap'`. The harness
adds the fixture only to the disposable copy and records its source hash.

## What changes in the isolated app

- The `OpenCascadeKernel` import boundary is replaced by `PlainSolidKernel.ts`.
  The compatibility class name remains, but geometry is dispatched to Go WASM.
- Exact provider string literals `"opencascade"` / `'opencascade'` in app and test
  TypeScript become `"plainsolid"` / `'plainsolid'`. Existing geometry, volume,
  topology, UI, persistence, and export assertions are otherwise preserved.
  Each changed file, replacement count, and before/after hash is recorded.
- Vite receives an isolated cache and explicit serving permission for the shared
  dependency directory. An optional port changes the copied test configuration.
- The staged viewer advances missed camera damping updates without drawing
  extra WebGL frames. The project-open command retains the drawable model
  and marks readiness pending during its asynchronous open. The harness
  records and verifies the exact runtime adaptations from the pinned source.
- A separate generated backend test proves real WASM execution and rejects OCCT
  asset requests. It checks nonempty material, no-op cut, connected-join behavior,
  and export union semantics. This probe is not counted as an existing fixture.

The initial adapter supports exact straight polygon extrusion (including supported
polygon holes), full-circle extrusion, boxes, planar
cut/join/common, clone/rigid placement, tessellation, STL, and unchanged rectangular
extrusion shelling. Circle geometry retains analytic cylinder/cap surfaces; only
display and STL are tessellated using the requested deflection settings. The
bridge conservatively maps angular deflection to half the kernel facet-normal
bound so sampled normal turning across cylindrical facets stays within the
requested angle. This is a sampled check, not a surface-wide certificate.

Mixed extrusion recognizes bounded analytic families from the actual connected
segments: one circular arc and its chord (including exact semicircles), a capsule with two semicircles and
tangent lines, or a capsule ring with matching centers. Disk and supported convex
polygon profiles can contain circular holes matched to their authored loop IDs.
Unsupported arc arrangements and mixed inner-loop families reject; curves are
never converted to polygon samples. Revolution forwards exact straight polygon
or full-circle profiles and the resolved world axis to the native bridge, retaining
`revolve` provenance. Profiles with inner loops remain unsupported for revolution.
General curved Boolean combinations, general fillets/chamfers, and unproved
topology references fail explicitly.

Modeled-thread and lead screw/nut methods forward their resolved dimensions and
handedness to bounded native helical constructors. Both original focused fixtures
pass: modeled Thread covers external editing, reopening and STL plus an internal
threaded nut; the lead pair covers linked edits and cold reopening. The
original production galleries also pass the left-hand Acme pair and posed
quarter-inch UNC screw and rectangular tapped block. Helical support is
bounded to qualified stock, dimensions and chart ranges.

Native STEP import/export runs in isolated workers and retains analytic BRep
geometry. Export returns independent before/after native read-back proofs and
millimeter units; the adapter decodes the native bytes without a mesh fallback.
The bridge bounds input/output size and body count, and rejects unsupported
STEP structures. Existing STEP fixtures keep their independent reader oracle,
placed bounds, cancellation, stale-result, persistence, and STL assertions.

Finite to-face extrusion resolves one actual retained planar face and proves that
it covers the projected profile, including the target's boundaries and holes.
Straight polygon profiles support planar termination; a full circle requires a
parallel target plane and stays an exact cylinder. Zero-depth, behind-direction,
missing, ambiguous, or uncovered faces reject without publishing a result.
Only parallel caps retain constant-distance extrusion metadata. Existing circular
and polygonal to-face cases measure upstream edits and reference recovery;
face-pocket and component-placement cases measure inward cuts, previews,
Undo/save/open, and oriented STL using the same native adapter.

Complete circular cap rims on an unchanged cylinder support bounded analytic
fillets. Authored cap roles identify the actual native rim centers; the bridge
retains toroidal fillet surfaces and rejects unsupported radius or rim selections.
The original XY/XZ/YZ circle-perimeter fixtures measure exact removed volume.

A bounded rectangular-stock history supports atomic fillet and equal-setback
chamfer batches on compatible parallel cap edges along the same stock length.
Every explicit reference is validated against the original input before one
result is published; repeated references resolve once. Authored straight outer-edge
IDs resolve to original endpoints and the physical sweep axis; native queries
prove complete sharp edges before advertising them. Treated or split edges are
not retargeted. A complete rectangular cap perimeter supports a bounded native
fillet on plain stock or an equal-setback chamfer on qualified plain or bored
stock. All four original edges must still exist; a missing original member is
diagnosed as lost instead of silently applying the survivors. Other nonparallel
perimeters remain unsupported.
Untouched native extrusions also expose their complete cap boundaries for linked
projection after actual planar-face validation; modified shapes do not inherit
that proof merely because source metadata remains.
The original multi-edge edit and edge-draft cases cover retained references,
preview cancellation, invalid-size recovery, repeated treatments, and persistence.
The history can
then retain one cylindrical through-bore, one aligned rectangular through-pocket,
and one upper rectangular boss, separated from each other. The pocket may
share one exact top boundary with a fillet tangent. The cylindrical patch
splits at the two contact endpoints and the top cap wire incorporates the
pocket notch. Bounds and geometry use signed cardinal frames with
translations and proper cardinal rotations; arbitrary rotations, reflections,
scales, interacting tools, and general modified-stock operations remain outside
this domain. The original long feature-chain and adjacent edge-treatment cases
measure parameter edits, invalid-size/downstream recovery, save/reopen, and STL.

Finite cylindrical through-cuts and blind bores are supported on qualified
convex planar and circular stock with parallel perpendicular caps. Through
tools must cover both stock caps, including exact endpoint equality. Blind
tools must open on one cap and end within the stock. One qualified rectangular
side-opening notch can be blind, retaining its circular-segment floor, or
through. Either can share a batch with disjoint contained blind bores from
either cap and through bores. Qualified cardinal placements round-trip
through STEP. A quarter-circle cut centered on a stock corner supports
the same contained bores and round-trips STEP in its blind and through forms.
A single exact side tangent on rectangular stock retains its four-use
contact for modeling and display. Fabrication and STEP reject that contact.

Scoped Hole checks each center against the original selected bodies and
requires every selected body to lose volume. A failed center or target
preserves the inputs and disposes intermediate outputs. The native batch
unions duplicate, contained and intersecting tools where the boundary is
proved. Unequal-depth and opposed entry tools retain analytic exposed
walls and floors across axial layers on qualified planar and circular
stock. The bridge accepts at most 32 tools per target batch. The measure
query computes common material volume without allocating an intersection
shape. The circular layered STEP codec verifies the regenerated canonical
graph and rejects unproved boundaries. The implementation remains bounded
to these qualified stock and tool families.
The backend probe
checks indexed scope errors and fills the remaining native handle capacity after
repeated late-target failures to verify cleanup. It also measures circular-lens
union volume and area, tool-order independence, duplicates, contained tools, and
the actual mesh and STL volume. It also rebuilds unequal-depth sequential Hole
features in both orders, saves/reopens them with the original app functions, and
checks their exported STL. Existing guided-hole UI overlap restrictions are
preserved; no existing browser fixture specifically measures overlapping scoped
holes, so that new coverage is reported separately from the existing regression
selection. Other scoped operations remain unsupported.
This harness is an acceptance integration, not a published production backend.

## Evidence

`readiness.json` records source paths, git revisions and dirty states, source
hashes, provider adaptations, the WASM hash, selected cases and their actual
results. Detailed logs, Playwright JSON, and failure traces remain beside it.
Skipped/interrupted tests cannot count as passes. A successful report requires a
passing backend probe and actual passes for every selected existing fixture.
Unselected fixtures remain **untested**; these results do not establish full
PlainCAD compatibility or full OCCT replacement. The original PlainCAD unit
suite contains OCCT-specific mocks and is not silently treated as portable.

### Optional production stage

After `run.mjs` has staged an isolated workspace with passing typecheck and real
Go WASM backend probe, verify it without launching a build or browser:

```sh
node tools/plaincad/production.mjs --work /tmp/staged-plaincad --check
```

The verifier checks recorded source hashes (including provider-only adaptations),
original production configuration against the pinned app commit, adapter and WASM
hashes. It rejects extra unrecorded production inputs. The original configuration
stays untouched; the derived configuration changes only the isolation port.

When other expensive qualification jobs are paused, run all original production
cases, or select an original file/title explicitly:

```sh
node tools/plaincad/production.mjs --work /tmp/staged-plaincad --run
node tools/plaincad/production.mjs --work /tmp/staged-plaincad --port 5415 --test production-e2e/step-import.spec.ts --run
```

This builds production assets, checks the built WASM identity, and records
`production-readiness.json`, `production-results.json`, build/test logs, original
source provenance and selected test outcomes. Original timeouts/assertions are
retained. Skips, zero selected tests and interrupted results cannot count as a
pass. Only the selected cases are qualified. The staging `readiness.json` is never
rewritten. Do not edit or concurrently run another harness in the staged workspace.

The [development](DEVELOPMENT.md) and [cross-browser](CROSS_BROWSER.md)
stages record independent evidence for their original suites. Direct suite
selection uses the same [verified delegation](SUITE_DELEGATION.md) for
production and cross-browser qualification.

Run the inexpensive verifier tests with `node --test tools/plaincad/production.test.mjs`.

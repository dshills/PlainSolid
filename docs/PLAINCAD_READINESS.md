# PlainCAD readiness

## Scope and evidence rules

The immediate target is the current PlainCAD application's actual kernel contract,
not all OCCT APIs. The source audit used the sibling PlainCAD checkout at revision
`fd9f7fadea2214b45946b9dcff748642acb5ae9d` on 2026-11-13. Record any uncommitted source
changes and source hashes with each run; this revision alone does not pin a dirty
working tree. The full PlainSolid specification remains required separately.

Source-hashed PlainSolid-backed browser acceptance runs are recorded below. Existing
OCCT test results, PlainSolid Go tests, a successful WASM build, and
extracted/reimplemented fixtures are not substitutes for running these scenarios
against the selected PlainSolid kernel.

## Overlapping Hole cutter checkpoint — 2026-11-13

**22 existing PlainCAD acceptance scenarios pass unchanged**, with zero skipped,
unexpected or flaky results. The regression stage took 133.393 seconds. Both
original TypeScript checks pass. A separate real-WASM supplemental probe passes
in 8.042 seconds. The [source-hashed report](evidence/plaincad-overlapping-holes-2026-11-13.json)
records `/tmp/plainsolid-overlap-acceptance-03`, every case, all selected local Go
runtime sources, the adapter/harness, and the tested WASM artifact.

There is **no existing overlap-specific browser fixture**. The existing 22 cases
are regressions; new overlap evidence is explicitly supplemental. It verifies
analytic two-circle lens volume and area, reversed tool order, duplicates and
contained cutters, mesh/STL volume and normals, late-failure rollback, input
ownership, handle cleanup, and an actual overlapping-Hole PlainCAD document
rebuilt after save/open. Guided-hole placement still intentionally rejects
overlapping circles; that application behavior is unchanged.

Scoped cutters are applied collectively, after every center is checked against
the original scope. A duplicate/contained cutter may add no extra removal within
a valid batch; standalone cuts still must remove meaningful material. Each target
must lose volume, and no outputs escape if any target fails. Analytic circular
arcs, cylinder walls and planar caps/floors are retained, not replaced by polygonal
solid geometry. The constructor is committed as `845cf3d`.

The qualified domain is convex planar stock, strictly interior parallel circular
cutters, and common effective entry/stop planes, with at most 32 retained tools.
Proper multi-circle overlaps, exact duplicates, containment and separated
components are supported for through or common-depth blind cuts. Unequal-depth
or opposed blind overlaps, tangent/ambiguous contacts, material-island footprints,
side-crossing cuts and arbitrary curved targets remain unsupported. Once a recipe
requires union construction, its whole recipe must share the effective depth;
independent separated mixed-depth recipes retain their prior support. Scope
intersection queries against already-bored targets still require separation from
existing cutter bounds. Generic native BRep serialization of the new anchored
trim type is not qualified; PlainCAD feature-recipe persistence is verified.

Native tests, race, vet, WASM build and independent review pass. Unselected
application tests and full production/release qualification remain open.

## Earlier scoped Hole checkpoint — 2026-11-13

**22 existing PlainCAD acceptance scenarios pass**, with zero skipped, unexpected
or flaky results. The consolidated acceptance stage took 151.620 seconds. Both
original TypeScript checks pass; a separate real-WASM backend probe passes in
8.450 seconds. The [source-hashed report](evidence/plaincad-scoped-holes-2026-11-13.json)
records `/tmp/plainsolid-hole-acceptance-11`, source identities, every selected
case and provider-only literal adaptations. A labeled post-run supplement pins
all selected kernel sources; rebuilding them exactly matches the accepted WASM
hash. Geometry/workflow assertions and
original test timeouts are unchanged. Interrupted and failed exploratory runs
are not counted as successful qualification.

| Existing selection | Passing cases |
|---|---:|
| Previous circle, extrusion, XYZ modeling, shell and stale-worker regressions | 11 |
| Guided face Hole: blind and through-all in XY/XZ/YZ; edits, Undo, save/open, STL | 6 |
| Explicit multi-body Hole: all centers/targets, rollback, raised-stock blind depth, persistence and STL | 3 |
| Native Hole dialog: explicit selection, blind/through edits, parameters and empty-center recovery | 1 |
| Automatic target capture across different centers and stale in-flight capture rejection | 1 |

The scope adapter validates every center against the original selected bodies,
requires every selected body to lose volume, and publishes owned outputs together.
Failure preserves original shapes and disposes intermediate outputs. The probe
checks exact center/target error indices and fills all 249 remaining handle slots
after repeated late failures with seven inputs live, proving no handle leakage.

Blind and through cylindrical cuts retain analytic walls and planar floors on
qualified convex planar stock, with strictly interior radial footprints and
perpendicular parallel caps. Either cap may be the opening. Repeated separated
bores and proven individual target misses are supported. Overlapping tools,
closed internal cavities, side-crossing/grazing cuts, unprovable finite reversal,
and arbitrary curved targets remain unsupported. Blind depth and remaining wall
must exceed the supported 1e-9 geometric threshold.

Display and STL share the requested tolerance mapping and mesh. Compact sampling
preserves circular edge detail without subdividing straight lengths or flat face
interiors unnecessarily: a representative bored part uses 144 triangles instead
of 2,048. Existing guided STL removal-volume accuracy and full modeling vertex
checks pass. The numerical prerequisite fixes are commits `0037fb1` and `14e5760`.
Native/race/vet/WASM checks and independent review pass. A supplementary full
trimmed-kernel run timed out at 600 seconds; it is not a full-suite pass (see
[verification](VERIFICATION.md)). Unselected tests and full production/release
qualification remain open. No full OCCT or PlainCAD replacement claim is made.

## Earlier circular extrusion and through-cut checkpoint — 2026-11-13

**Eleven existing PlainCAD scenarios pass** in the consolidated run, with zero
skipped, unexpected or flaky results. The acceptance stage took 78.999 seconds;
a separate real-WASM backend probe took 4.327 seconds. Both original TypeScript
checks passed. The [source-hashed report](evidence/plaincad-through-cuts-2026-11-13.json)
records `/tmp/plainsolid-circle-acceptance-06`, every case, provider-only literal
adaptations, source identities and WASM SHA-256.

The selection includes the six foundation scenarios below, circle-dialog profile
choice/negative-XZ preview, all three XYZ non-template modeling/edit/save/open/STL
cases, and negative/symmetric through-all cutting with invalid-direction rejection.
Existing geometry and workflow assertions are unchanged. The independent probe
also checks analytic circle and bored-stock volume/area, without OCCT requests.

Through-cuts retain analytic cylinder walls on convex planar stock with strictly
interior radial footprints and provably perpendicular parallel caps. Finite tools
must cover both caps; separated repeated bores are supported. Blind, partial,
grazing, side-crossing and unsupported oblique-cap cuts are rejected. General
curved Booleans, arbitrary curved placement and native scoped Hole remain open.
This is a selected PC1 subset, not full application or OCCT replacement.

## Exact circle extrusion milestone — 2026-11-13

The existing `extrude-dialog.spec.ts` case `profile choice and negative XZ native
preview cancel cleanly and apply only current settings` now passes with the
PlainSolid backend. It verifies profile choice between two full circles, negative
XZ extrusion, analytic volume to seven decimal places, bounds, invalid depth,
preview/cancel, rapid edits, Apply and Undo/Redo. Both original TypeScript checks
and the independent real-WASM probe passed. The probe checks analytic circle mass
and conservative bounds separately from the display mesh.

The [source-hashed circle report](evidence/plaincad-circle-extrusion-2026-11-13.json)
comes from `/tmp/plainsolid-circle-acceptance-03`. This is one additional existing
application scenario; it does not yet establish the cylindrical through-cut path.
The bridge keeps an analytic cylinder B-rep, supports signed height, rigid
placement and actual cap-plane queries, and rejects mixed-arc/holed-circle input.
Native tests cover all three axes and both signs, independent surface integration,
mesh convergence/closure, matching STL vertices, strict input rejection and limits.
Focused race, vet and WASM build checks pass; no external dependency was added.

```sh
node tools/plaincad/run.mjs --source /absolute/path/to/PlainCAD --test e2e/extrude-dialog.spec.ts --grep 'profile choice and negative XZ native preview cancel cleanly and apply only current settings'
```

## Historical foundation browser runs — 2026-11-13

Six existing PlainCAD acceptance cases pass against the real Go WASM backend:

| Existing case | Result |
|---|---|
| XY negative/symmetric extrusion, parameter edits, save/open and STL winding | Passed |
| XZ variant of that same scenario | Passed |
| YZ variant of that same scenario | Passed |
| Rectangular Shell: either cap, invalid walls, edits, Undo, command plan, save/open and STL | Passed |
| Real-worker stale result cannot overwrite a newer edit | Passed |
| Real-worker failure cannot overwrite a newer edit | Passed |
| XY non-template modeling with circular-profile extrusion and through-cut | Failed at circular-profile extrusion preview |

The [foundation report](evidence/plaincad-foundation-2026-11-13.json) preserves
source identities/hashes, provider adaptations and all six passing cases from
`/tmp/plainsolid-cad-acceptance-12`. The existing cases took 29.58 seconds;
a separate backend probe took 3.06 seconds. Neither stage skipped any tests.
The probe verifies actual Go WASM dispatch, `geometrySource: plainsolid`, no OCCT
asset requests, Boolean no-op/empty/disconnected handling, symmetric overlap
classification and negligible-shell-cavity rejection. It is not counted as an
existing application acceptance case.

Both TypeScript checks passed (`tsc -b --noEmit`, and separately in that copied
app `tsc -p tsconfig.e2e.json`). The runner now executes both checks itself.
Runtime: Go 1.27.4, Node 26.13.1, Playwright 1.63.0 with its Chromium on macOS arm64.
WASM SHA-256 for the foundation run:
`25dfddde41c2854e762230e53a9d7e76ee1d230564ca743cbff014b54447a93f`.
The original PlainCAD checkout was not modified; its pre-existing dirty validation
log is recorded in the report. Source hashes, not only Git HEAD, identify the run.

The [next-gate report](evidence/plaincad-circular-profile-2026-11-13.json), from
`/tmp/plainsolid-cad-acceptance-13`, records both TypeScript checks and the backend
probe passing, but the existing XY modeling fixture failing at its second
extrusion (`modeling.spec.ts:153`). It expects `Native preview ready` and receives
`Preview failed` after adding a circle. At that snapshot the adapter rejected circular
and arc profiles without substituting polygonal geometry. The circular extrusion
and through-cut failure is now resolved in all three planes. General mixed line/arc profiles
remain a separate requirement.

That historical run had **six passing scenarios and one failing next gate**,
not a full-application percentage. All unselected cases remain not run.
Full production/CSP, bundle/performance and release checks remain open.
No full OCCT or PlainCAD replacement claim is made.

Reproduce the passing selection from the integration worktree:

```sh
node tools/plaincad/run.mjs --source /absolute/path/to/PlainCAD --test 'e2e/(extrusion-hole|shell|stale-results).spec.ts' --grep 'native negative/symmetric|native Shell|real-worker'
```

Reproduce the formerly failing requirement:

```sh
node tools/plaincad/run.mjs --source /absolute/path/to/PlainCAD --test e2e/modeling.spec.ts --grep 'XY: non-template modeling, edit, save/open, and real-kernel STL'
```

Readiness has two separate dimensions:

- **Geometry support:** the PlainSolid APIs can represent and perform the required
  operation on the actual input family, including operation composition.
- **Application acceptance:** the existing browser scenario passes with a verified
  PlainSolid provider, unchanged geometric assertions, and no OCCT fallback.

For each run retain the PlainCAD revision and source hashes, PlainSolid source
identity, selected provider, exact expanded test titles, command, browser/runtime,
result counts, duration, and report/trace locations. Mark each case `passed`,
`failed`, `blocked`, or `not run`. A blocked launch is not a geometry failure; a
skipped or unsupported case is not a pass. Compute a percentage only against a
fixed, enumerated selected manifest, and label it **selected acceptance coverage**,
not full-spec completion or estimated remaining effort. Do not exclude failures
from the denominator or count repeated successful runs as additional coverage.

PlainCAD's current tests often expect `geometrySource === "opencascade"`.
Provider-only string-literal adaptations to expect `plainsolid` are allowed in an
isolated integration checkout and must be recorded with changed-file hashes and
replacement counts. Preserve all volume, solid-count, validity, bounds, topology,
STL, lifecycle, persistence and UI assertions. PlainSolid output must identify
itself as `plainsolid`; claiming OCCT identity to satisfy an assertion is forbidden.

## Kernel contract mapping

| PlainCAD requirement and source | Current PlainSolid correspondence | Remaining integration or geometry gate |
|---|---|---|
| [Kernel adapter](../../PlainCAD/src/cad/kernel/KernelAdapter.ts), browser worker and runtime handles | Go WASM adapter with six existing application scenarios passing | Broader geometry, production worker/CSP and runtime performance qualification |
| Exact line/arc/circle profiles with holes, signed extrusion: [OpenCascadeKernel](../../PlainCAD/src/cad/kernel/OpenCascadeKernel.ts), `createOcctFace` / `createOcctExtrusion` | Planar polygon extrusion and specialized curved extrusions | General supported sketch-face conversion retaining analytic arcs and holes, arbitrary sketch planes |
| Cut/fuse/common/scoped atomic cuts: same file, `cutScope` / `joinAll` | Exact planar BSP, selected cylindrical bores and curved Boolean families | Compose on revolved/filleted/modified bodies; preserve atomic failure, connected joins and no-op rejection |
| Cap/vertical-edge treatments: same file, `treatEdges` / `resolveNativeEdges` | Convex vertical-prism corner fillets, selected cylinder rims, planar chamfers | Cap perimeter blends and subsequent curved cuts; retained original-edge proof |
| Rectangular shell: [nativeShell](../../PlainCAD/src/cad/kernel/nativeShell.ts) | Convex planar hollowing with the existing rectangular Shell scenario passing | Wider input families remain out of scope; this does not require general curved shelling |
| Partial/full revolve around resolved axis: `createOcctRevolve` | Full meridian polygon revolution and selected analytic primitives | Partial-angle and circular-arc profile solids plus later cut/join |
| Real modeled threads: [nativeThreads](../../PlainCAD/src/cad/kernel/nativeThreads.ts) | No qualified helical tool path found in audit | Helical p-curve/model curve, solid sweep, thread cuts and material classification |
| Lead screws/nuts: [nativeLeadScrew](../../PlainCAD/src/cad/kernel/nativeLeadScrew.ts) | No qualified matching operation found in audit | Multi-start tools, end reliefs, engagement and nominal clearance checks |
| Exact shape measurements and retained geometry: [occtGeometry](../../PlainCAD/src/cad/kernel/occtGeometry.ts), [nativeDrawing](../../PlainCAD/src/cad/kernel/nativeDrawing.ts) | Planar/selected curved mass, queries, ancestry and roles | Current finite face, pocket-floor, rim and retained-edge proofs across operation histories |
| Deflection/angular mesh, bounds and STL | Existing mesh, tessellation, exchange packages | Adapt output buffers; cover all supported feature results; positive orientation and byte limits |
| STEP import/export: [nativeStep](../../PlainCAD/src/cad/kernel/nativeStep.ts) | Selected planar/analytic/spline STEP families | Import accepted one-solid files and roundtrip actual app-generated curved bodies, not only primitive fixtures |

PlainCAD retains its TypeScript sketch solver, parametric document/history,
commands, Three.js renderer and application worker protocol. Replacing those with
PlainSolid's corresponding optional systems is not a prerequisite. Its authored
face roles and rectangular shell restrictions are narrower than full OCCT parity.

## Ordered acceptance inventory

The selectors below come from existing PlainCAD tests. Parameterized titles expand
to separate cases; preserve every expansion selected for a gate. All cases below
start as **not run with PlainSolid**; subsequent run records must state results.

| Gate | Existing source and exact test title | What it proves |
|---|---|---|
| PC0 | [extrusion-hole.spec.ts](../../PlainCAD/e2e/extrusion-hole.spec.ts): `XY: native negative/symmetric coordinate extents, parameter edits, save/open and STL winding`; repeat `XZ:` and `YZ:` | Smallest initial geometry gate: rectangle only, signed coordinates, volume 2000 then 2800, persistence, STL orientation |
| PC0 | [stale-results.spec.ts](../../PlainCAD/e2e/stale-results.spec.ts): `real-worker result cannot overwrite a newer edit during debounce or after its rebuild`; `real-worker worker failure cannot overwrite a newer edit during debounce or after its rebuild` | Worker epoch/request semantics remain correct with actual WASM geometry |
| PC1 | [modeling.spec.ts](../../PlainCAD/e2e/modeling.spec.ts): `{XY,XZ,YZ}: non-template modeling, edit, save/open, and real-kernel STL` | Rectangle plus circular through-cut; actual GUI, edits, saved project, mesh and STL |
| PC1 | [extrusion-hole.spec.ts](../../PlainCAD/e2e/extrusion-hole.spec.ts): `negative and symmetric through-all tools cut the real target and reject a direction with no target extent`; `native hole dialog requires explicit centers/target; blind/through edits, parameter changes and empty-center recovery` | Through/blind holes, signed scope and failure diagnostics |
| PC1 | [modeling-operations.spec.ts](../../PlainCAD/e2e/modeling-operations.spec.ts): `finite to-face termination follows upstream edits and diagnoses lost references` | Finite target face coverage, termination and reference survival |
| PC1 | [face-pocket.spec.ts](../../PlainCAD/e2e/face-pocket.spec.ts): `endCap Draw here → mouse region → inward native pocket → Undo/save/open/STL`; repeat `side` | Face-derived sketches, pockets, role proof and persistence |
| PC1 | [shell.spec.ts](../../PlainCAD/e2e/shell.spec.ts): `native Shell preserves its body, opens either cap, cancels collapsed walls and saves editable thickness` | Restricted rectangular shell, cavity volume and invalid thickness |
| PC1 | [component-placement.spec.ts](../../PlainCAD/e2e/component-placement.spec.ts): `moves and rotates native components with mouse cancellation, exact fields, one Undo, save/open and correctly oriented STL` | Rigid placement, ownership and exported world coordinates |
| PC2 | [modeling-operations.spec.ts](../../PlainCAD/e2e/modeling-operations.spec.ts): `native edge treatments: edit, parameter change, save/open, STL, and invalid-size recovery` | Selected edge treatments and dimensional changes |
| PC2 | Same source: `long native feature chain preserves exact geometry through edits, blocked downstream recovery, save/open and STL` | Extrusion → fillet → chamfer → hole → pocket → boss on one retained body |
| PC2 | [edge-dialog.spec.ts](../../PlainCAD/e2e/edge-dialog.spec.ts): `native edge drafts cancel cleanly, reject invalid/changed edges, and apply three exact XZ treatments with stable owners and save/open` | Current references and repeated treatments on a non-XY plane |
| PC3 | [revolve-dialog.spec.ts](../../PlainCAD/e2e/revolve-dialog.spec.ts): `Revolve previews a construction-axis torus, diagnoses axis/angle errors, cancels, and saves only the latest native quarter sweep` | Circular profile, nontrivial axis and partial angle |
| PC3 | [modeling-operations.spec.ts](../../PlainCAD/e2e/modeling-operations.spec.ts): `native revolve axes and angle edits, plus real boolean cut and join` | Revolved solids compose with Boolean operations |
| PC4 | [fastener-presets.spec.ts](../../PlainCAD/e2e/fastener-presets.spec.ts): `modeled Thread previews and persists native external and internal grooves` | Actual helical solid cuts and both thread sides |
| PC4 | Same source: `lead screw and matching nut are linked, editable and survive a cold open` | Linked multi-start lead parts, geometry checks and rebuild from persisted intent |
| PC5 | [step-export.spec.ts](../../PlainCAD/e2e/step-export.spec.ts): `public STEP body selection exports analytic posed solids with actual native reader volume and world bounds through edits and save/open` | Actual STEP writer/reader, solid count, mass and world bounds |
| PC5 | Same source: `STEP cancellation and running/prepared same-ID stale results never download or retain a usable proof`; `a real STEP worker load failure is actionable, cannot download, and can be retried successfully` | Export cancellation, stale proof and worker recovery |
| PC5 | [step-import.spec.ts](../../PlainCAD/e2e/step-import.spec.ts): `STEP UI and JSON import preserve placed native geometry, cancel, undo, save/open and STL`; `same-ID project replacement aborts the pending native STEP preview and forbids Apply` | Actual imported solid, placement and safe preview lifecycle |

This inventory is an initial critical-path selection, not the complete PlainCAD
suite. Before claiming current-feature replacement, enumerate and execute the
remaining native application tests, including retained-edge picking, scoped cuts,
patterns, assembly common-volume collision, production workers/CSP, and the
repository's required `check:item` / `release:check` gates. Run them on an explicitly
selected PlainSolid build and record unsupported cases rather than removing them.

## First run and resumption

Scoped Hole now implements atomic output publication and indexed center/target
errors, with analytic blind/through cuts in its qualified convex-stock domain.
Retain the consolidated regression selection recorded above. Remaining work
includes unequal-depth overlaps and mixed profiles, broader curved target bodies,
and Hole editing that depends on downstream chamfers/PC2 edge treatments. Use the
first failing geometric assertion or explicit unsupported operation as the next
implementation task; do not relax the test to match the implementation.

Retain an OCCT baseline on the same selected source as a diagnostic control when
possible. Report control passes separately from PlainSolid passes. Any temporary
integration checkout, provider adaptation, test selection and build artifacts must
be reproducible from a checked-in harness, with generated artifacts kept out of
source control.

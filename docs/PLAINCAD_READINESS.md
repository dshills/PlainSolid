# PlainCAD readiness

The immediate target is the current PlainCAD application's actual kernel
contract, not all OCCT APIs. This page defines that contract and the
acceptance cases for each gate. Current results are in
[the status page](STATUS.md) and [the verification record](VERIFICATION.md).
The dated run log that this page replaced is in git:
`git show c35d3358:docs/PLAINCAD_READINESS.md`.

## Scope and evidence rules

The source audit used the sibling PlainCAD checkout at revision
`fd9f7fadea2214b45946b9dcff748642acb5ae9d` on 2026-11-13. Qualification runs
pin the app at `efae4d2` on its `codex/plainsolid-backend` branch. Record any
uncommitted source changes and source hashes with each run; a revision alone
does not pin a dirty working tree.

Only a PlainSolid-backed browser run of the original application scenarios
counts as acceptance. Existing OCCT test results, PlainSolid Go tests, a
successful WASM build, and extracted or reimplemented fixtures are supporting
evidence, not substitutes.

For every gate, keep the geometry, lifecycle and persistence assertions.
Provider-name assertions may expect `plainsolid`; never label its output
`opencascade` to make a test pass. Report unsupported, failed, blocked and
not-run cases separately. Use the first failing geometric assertion or
explicit unsupported operation as the next implementation task; do not relax
the test to match the implementation.

Retain an OCCT baseline on the same selected source as a diagnostic control
when possible, and report control passes separately from PlainSolid passes.
Any temporary integration checkout, provider adaptation, test selection and
build artifacts must be reproducible from a checked-in harness, with generated
artifacts kept out of source control. The harness is described in
[its README](../tools/plaincad/README.md).

## Kernel contract

These are the PlainCAD kernel requirements and where PlainCAD implements them
against OCCT. PlainSolid serves them through `internal/plaincadbridge`.

| PlainCAD requirement and source |
|---|
| [Kernel adapter](../../PlainCAD/src/cad/kernel/KernelAdapter.ts), browser worker and runtime handles |
| Exact line/arc/circle profiles with holes, signed extrusion: [OpenCascadeKernel](../../PlainCAD/src/cad/kernel/OpenCascadeKernel.ts), `createOcctFace` / `createOcctExtrusion` |
| Cut/fuse/common/scoped atomic cuts: same file, `cutScope` / `joinAll` |
| Cap/vertical-edge treatments: same file, `treatEdges` / `resolveNativeEdges` |
| Rectangular shell: [nativeShell](../../PlainCAD/src/cad/kernel/nativeShell.ts) |
| Partial/full revolve around resolved axis: `createOcctRevolve` |
| Real modeled threads: [nativeThreads](../../PlainCAD/src/cad/kernel/nativeThreads.ts) |
| Lead screws/nuts: [nativeLeadScrew](../../PlainCAD/src/cad/kernel/nativeLeadScrew.ts) |
| Exact shape measurements and retained geometry: [occtGeometry](../../PlainCAD/src/cad/kernel/occtGeometry.ts), [nativeDrawing](../../PlainCAD/src/cad/kernel/nativeDrawing.ts) |
| Deflection/angular mesh, bounds and STL |
| STEP import/export: [nativeStep](../../PlainCAD/src/cad/kernel/nativeStep.ts) |

PlainCAD retains its TypeScript sketch solver, parametric document/history,
commands, Three.js renderer and application worker protocol. Replacing those with
PlainSolid's corresponding optional systems is not a prerequisite. Its authored
face roles and rectangular shell restrictions are narrower than full OCCT parity.

## Ordered acceptance inventory

The selectors below come from existing PlainCAD tests. Parameterized titles expand
to separate cases; preserve every expansion selected for a gate. Current results
are in [the status page](STATUS.md).

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
| PC2 | [edge-edit.spec.ts](../../PlainCAD/e2e/edge-edit.spec.ts): `Edit Fillet/Chamfer preserves multiple authored refs` | Multiple authored edges, mixed caps, downstream validation, cancellation, save/open and STL |
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

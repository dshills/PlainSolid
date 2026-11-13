# Implementation plan

## Execution policy

Build usable vertical slices and keep the full target visible. Independent packages can be developed concurrently after agreeing shared types. Integrate each stage with tests and examples. Dependencies remain standard-library-only unless a measured requirement justifies an optional package. Completed foundational work must not be reported as completion of the entire plan.

The current execution status and unimplemented requirements are in [STATUS.md](STATUS.md).

## Immediate priority — PlainCAD compatibility

As of 2026-11-13, implement the capabilities consumed by PlainCAD before
expanding unrelated OCCT coverage. This changes execution order, not the full
replacement definition in [SPECIFICATION.md](SPECIFICATION.md). A working
PlainCAD subset is a distinct milestone and must not be called full OCCT parity.
The acceptance inventory and results live in
[PLAINCAD_READINESS.md](PLAINCAD_READINESS.md).

PlainCAD already owns its sketches, expressions, document history, commands,
Three.js renderer and worker lifecycle. Reuse those systems. Implement a
PlainSolid-backed `KernelAdapter` and browser worker bridge with explicit provider
identity, opaque runtime handles, bounded requests, disposal, and unsupported
operation diagnostics. Do not silently fall back to OCCT or faceted geometry.

Execute these gates in order; independent tests and later feature implementation
may proceed in parallel:

| Gate | Work to prioritize | Existing PlainCAD acceptance requirement |
|---|---|---|
| PC0: Browser foundation | Compile a Go JS/WASM kernel; implement handle ownership, box/polygon extrusion, exact mass/bounds, tessellation and STL; integrate provider selection without changing document persistence | Rectangle negative/symmetric extrusion in XY, then XZ/YZ; edit, save/open, positive STL winding; stale worker result rejection |
| PC1: Common part modeling | Exact mixed line/arc/circle profiles and holes; signed extrusion; planar and supported curved cut/fuse/common; scoped atomic cuts; blind/through holes; finite to-face termination; restricted rectangular shell; rigid placement and face/rim proofs | Non-template part modeling; extrusion/hole fixtures; planar pocket and shell fixtures; placed-body workflows |
| PC2: Composable edge treatments | Match selected authored cap-perimeter and vertical-edge fillet/chamfer semantics; preserve surviving references through earlier operations; cut and join the resulting curved solids | Existing six-operation chain: extrusion → cap fillet → cap chamfer → hole → pocket → connected boss; size edits and failed-middle-operation recovery |
| PC3: Revolved parts | Arbitrary resolved axis and partial/full angles for supported line/arc/circle profiles, plus subsequent cut/join and reference checks | Construction-axis torus and quarter sweep; revolve axis/angle edits and Boolean fixtures |
| PC4: Helical parts | Helical swept solid cutters, internal/external threads, handedness, lead-in/runout, multi-start lead screws and matching nuts; point/common-volume proofs | Existing modeled Thread and lead-screw/nut cold-open fixtures |
| PC5: Exchange and complete app qualification | STEP for actual app outputs and accepted imported solids; export/reimport volume, topology and bounds; worker cancellation, stale previews, production CSP and performance | Existing STEP import/export and failure tests, full native development and production suites |

Current checkpoint (2026-11-13): PC0's selected browser regressions pass. PC1
implements scoped Hole validation, atomic cutter batches, and analytic overlapping
circular footprints with a common effective entry/stop depth. Duplicates and
contained tools are accepted collectively; standalone no-op cuts remain rejected.
All 22 existing browser regressions and the separate real-WASM overlap probe
pass, along with both TypeScript checks. There is no existing overlap-specific
browser fixture; supplemental overlap evidence is labeled separately. Guided-hole placement restrictions remain unchanged.
See readiness for final source-hashed evidence and bounded support. Next PC1
work includes mixed line/arc profiles, differing-depth overlapping cuts, and
broader modified/curved target bodies. Downstream Hole editing with chamfers also
depends on PC2 edge-treatment support.

For every gate, run the original application scenarios with PlainSolid selected.
Retain geometry, lifecycle and persistence assertions. Provider-name assertions
may be parameterized to expect `plainsolid`; never label its output `opencascade`
to make tests pass. A Go unit test or extracted arithmetic fixture is supporting
evidence, not an application acceptance pass. Preserve the same selected inputs
and report unsupported, failed, blocked and not-run cases separately.

Use the first failed PlainCAD acceptance case to choose the next kernel task.
General curved intersection work remains necessary where these cases require
it, particularly PC2 and PC4. Prioritize those concrete inputs over unrelated
spline/exchange families. General OCAF-like documents, IGES breadth, unrestricted
rendering and unrelated feature families follow these gates unless needed by a
selected PlainCAD case. Keep the phase exits below as the remaining full-target
backlog, and continue them after PlainCAD compatibility.

## Phase 1 — numeric and mesh foundation

1. Establish Go module, documentation and CI; require formatting, vet, unit/race tests.
2. Implement vector/bounds/affine types with finite/singularity checks.
3. Implement rational B-spline curves and surfaces with validated knots, weights and copied storage. Test endpoint/rational behavior and derivatives where present.
4. Implement indexed meshes, incidence checks, area and volume; build box/cylinder/sphere and simple polygon extrusion.
5. Implement bounded OBJ/STL and versioned native mesh exchange. Test malformed data and roundtrips.
6. Implement exact regularized operations for unions of axis-aligned boxes, cell limits, mass properties and boundary meshing.
7. Integrate a modeling-to-exchange example and publish precise limitations.

Exit: public operations have passing tests, independent analytic examples work, no third-party dependencies, no claim of general CAD parity. These tasks run in parallel by package; solid integration depends on geometry and mesh interfaces.

## Phase 2 — robust geometry algorithms

Add adaptive/exact-sign predicates (filtered float64 with exact fallback), scale-aware tolerances, intervals, root isolation and stable linear solves. Extend curve/surface families and derivatives; implement knot insertion, subdivision, degree elevation, trimming, periodicity, adaptive quadrature, closest-point projection and fitting. Define result diagnostics for nonunique and unresolved cases. Build interval bounds and BVHs. Exit: analytic and high-precision comparison corpus passes over documented scale ranges; uncertain answers are reported rather than guessed.

## Phase 3 — exact boundary representation

Design immutable entity IDs and oriented uses separately from shared geometric data. Implement vertex, edge with 3D and per-face 2D parameter curves, wire, face with outer/inner loops, shell, solid and compound. Validate seams, degenerate edges, topology consistency, closure and tolerance budgets. Implement exact analytic primitives and planar extrusion, then revolution/loft/sweep. Add shape history. Exit: exact primitives and holed profiles pass topology, area/volume and persistent identity tests; mesh conversion is not used as the exact representation.

## Phase 4 — intersections and general Booleans

Implement analytic intersections first, then subdivision/interval-assisted spline intersections with continuation and certified residuals. Handle tangent, coincident and periodic cases explicitly. Build broad-phase candidates, split edges/faces, classify fragments, assemble shells and unify compatible domains. Preserve topology history and diagnose invalid inputs. Add cancellation and memory/work limits. Exit: arbitrary supported B-rep union/intersection/difference/section pass invariant, adversarial and pinned OCCT fixtures. Axis-aligned algorithms remain a specialized fast path.

## Phase 5 — advanced modeling and healing

Implement sewing and repair reports before fillets and offsets. Develop rolling-ball fillets/chamfers, surface offsets, thick solids, drafts, pipe/loft continuity and local features. Detect self-intersections and topology transitions. Repair operations return changed entities and maximum displacement. Exit: independent radius/distance checks and difficult topology fixtures pass; unsupported combinations fail explicitly.

## Phase 6 — tessellation and queries

Constrained trim-aware triangulation with holes, conforming shared edges, adaptive deflection and angular control, periodic seams/poles and optional parallel face processing. Add point classification, minimum distance, collision, centroid and inertia for exact shapes, with numerical error estimates. Exit: mesh approximation bounds and watertightness pass on all exact model fixtures; query invariants hold under rigid transforms.

## Phase 7 — CAD interoperability

Implement streaming STEP Part 21 lexer/parser, typed entity graph, schema coverage and diagnostics; begin with analytic/NURBS geometry and manifold B-reps, then assemblies/units/styles and AP203/AP214/AP242 conformance cases. Implement IGES records, directory/parameter sections and supported entity mappings. Add glTF/PLY and exact native persistence with schema migrations. Enforce file/entity/depth/allocation limits. Exit: independent CAD reader/writer roundtrips preserve supported geometry/topology/metadata, and unsupported entities produce explicit reports. Merely parsing syntax is not exchange support.

## Phase 8 — documents and presentation

Implement assembly instances, labels/attributes, transactional edits, undo/redo, stable naming and persistence. Add scene graph, cameras, picking, clipping and hidden-line output. Keep optional interactive rendering adapters separate. Provide migration examples corresponding to common OCCT workflows. Exit: document history and persistence tests, selection correctness and deterministic presentation fixtures pass.

## Phase 9 — replacement qualification

Pin an OCCT revision and curate licensed, reproducible fixtures for each capability matrix row. Build an optional reference runner, normalize units/tolerances and compare residuals/topology/mass properties. Run long fuzz campaigns, race tests, supported-platform builds, allocation benchmarks and regression suites. Resolve all advertised feature gaps, select project licensing and publish compatibility guarantees. Exit: the full specification acceptance matrix is satisfied and externally inspectable evidence exists. Until this gate passes the library is incomplete as an OCCT replacement.

## Risk register

- General intersections and topology reconstruction dominate correctness risk: invest in diagnostics and adversarial fixtures before API expansion.
- Tolerance accumulation can change topology: track per-entity tolerance and repair budgets.
- Coordinate-grid Booleans have cubic growth: reject excessive work and later add spatial decomposition.
- Triangle manifold checks do not prove absence of self-intersections: keep this limitation visible.
- STEP/IGES breadth requires schema-specific fixtures: publish an entity support matrix.
- Exact robust predicates alone do not make all numerical constructions exact: distinguish predicates, constructions and approximation error.
- OCCT parity is much larger than a short implementation cycle: use measurable stage gates, not a nominal completion label.

## Remaining full-target critical path

The PlainCAD gates above determine immediate implementation order. The following
work remains required for the phase exits above. Each result should
record what was proved and what remained unresolved; a successful example
does not widen the declared input domain by itself.

| Work item | Inputs and expected output | Verification before advancing |
|---|---|---|
| Global curved contact discovery | Start from conservative knot-aligned pair cells; refine unexplained cells; grow verified tubes from section seeds; reconcile overlap across charts, seams and knots. Return a graph plus a covering certificate, and retain unresolved cells. | Independently prove empty, isolated, one-loop and multi-loop cases. Compare accepted branches and residuals with pinned OCCT. Budget exhaustion must retain unexplained regions. |
| Singular and coincident contacts | Distinguish point contacts, ordinary branch crossings, seam events and coincident domains; prove equality of chart mappings where possible. Represent overlap areas explicitly instead of reducing them to sampled curves. | Exact constructed tangencies, crossed and cusp curves, coincident faces with different charts, nearly coincident disjoint faces, and all three Boolean operations on touching solids. |
| Trim arrangements | Split faces by certified branch pieces; split each shared model edge at each distinct proved parameter; transfer the same normalized p-curves to both incident supports. Reconcile endpoint identity independently of spatial tolerances. | Open, closed and nested wires, coincident ranges, periodic seam jumps, edge/face vertex fans, and all fragment classifications reproduced under rigid placement. |
| Curved Boolean assembly | Classify fragments against the other solid, select by the regularized operation, orient, reassemble, and track deleted/generated/modified entity ancestry. | Closed manifold result, interval or analytic geometry residuals, mass and volume identities, no missed independent crossings, deterministic behavior under operand permutation where appropriate. |
| Advanced operations | Build rolling ball transitions and offsets on the same trim graph, then draft, general local feature edits and healing on the resulting supports. | Compare radii, offsets, ancestry and topology on multi-face corners and transitions; reject self-intersection or produce a proved repair report. |
| Exchange and document parity | Extend the shared graph to general STEP and IGES supports with continuous model/UV curve agreement; carry placements, units, names, styles and stable feature references. | Independent inbound/outbound files with mixed analytic and NURBS faces, loops, cavities, repeated instances and regenerate-after-load fixtures. |
| Qualification | Expand the pinned reference corpus, run long fuzz campaigns and platform CI, review the API and license, and publish the exact compatibility matrix. | All required specification rows pass on the supported platforms. List residual deviations and do not claim replacement with required gaps. |

Independent work in analytic STEP/IGES, query bounds, modeling features,
documentation and corpus construction can run alongside the curved-contact
critical path. Merge small verified changes, keep their fixtures, and continue
from the first unclosed requirement in the status record.

For the curved Boolean path, qualify a closed, transverse contact strictly
inside one face of each operand before generalizing to sections that cross
edges. Both input shells need an embedding certificate, and all unselected
face pairs need strict contact exclusion. Prove coverage and branch junctions
separately, classify the connected fragments, then sew paired p-curves from
both original supports. The current arrangement propagates multiple split
events to both incident faces, reconciles common model edge parameters and
combines closed and open sections on rectangular spline charts. Extend these
proofs beyond the qualified simultaneous crossings to general edge events,
periodic seams, arbitrary certified input trims, and singular contacts.
For simultaneous edge events, retain the exact source parameters, prove
which incident face pairs enter both charts, verify every paired boundary
tube, and account separately for pairs whose only contact is the shared
point. The determinant-one warped fixture now qualifies this sequence
against pinned OCCT. The next release gate requires equivalent proof for
general curved inputs without a shared affine or quadratic-warp chart,
including tangential returns and algebraic boundary splits.

The current exact-trim path separates topology from the parameters needed to
evaluate it. Version 4 stores bounded homogeneous rational coefficients at
represented UV endpoints. Version 5 reconstructs exact rational UV and model
endpoint identity from the homogeneous controls and the owning support,
verifies the rounded display point, and revalidates loop and shared-edge
identity. Rational seam splits and conic Boolean junctions qualify that path.
Continuous edge agreement handles distinct algebraic knot events using ordered
root identities and polynomial composition. A dedicated native algebraic
subdivision format persists an irrational split as a bounded root recipe and
an independent plane, graph support or affine-edge constraint. Reload
reproves exact model identity, order and incidence, and
both coedges retain their algebraic domains. It distinguishes two roots
less than 2^-100 apart even if their display segments collapse. Verified
tubes can be clipped against selected quadratic trim loops, and a proper
open curved run can split an injective chart into two exact fragments
with shared algebraic endpoints. Closed cuts prove their winding, and
disjoint holes are assigned to the resulting fragment. A closed single
regular loop now seals an exact two-face graph lens. Continue from that
material proof to multi-loop containment and cross-shell sewing, including
original edge-to-edge and tangential-return events.
Independent edge-to-face isolation already provides regular terminal hints
to the tube verifier. Integrate exact algebraic terminal identities and
prove complete continuation before assembling the resulting material.

In parallel, extend automatic curved contact discovery and trim arrangements
beyond common-height graphs. Qualify each newly supported input family with
independent geometric fixtures, then add matching exchange and stable feature
references. Passing the newer graph, mesh or exchange fixtures does not close
the general curved phase exits above.

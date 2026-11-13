# Verification record

## Overlapping scoped Hole integration — 2026-11-13

The bridge adds bounded `cutMany` batches: validate all inputs first, construct
one retained result, require collective material removal, then publish one owned
handle. Planar batches also support redundant members. The adapter keeps every
center's original-target probe, requires each target to lose material, and holds
all outputs until the full scope succeeds. Standalone and batch cuts share the
application's material-removal tolerance, preventing rounding from making an
idempotent cutter appear to remove volume. Unsupported batches publish nothing.

Circular batches use the analytic union constructor when separated-bore handling
cannot prove the combined shape. They require common effective entry/stop planes;
through-tool reversal must preserve represented geometry. Union mass, cap area,
blind floor normals, clone/transform ownership and signed XYZ behavior pass.
Opposed-axis through tools, duplicates/containment, three-circle overlap, partial
batch failure and shifted no-op rounding regressions pass. Native bridge tests
pass (1.397 seconds), race passes (10.842 seconds), and vet/WASM build pass.
Independent geometry, bridge, adapter and probe reviews approved this domain.

There is no existing PlainCAD browser fixture specifically for overlapping Hole
cutters. Existing guided-hole staging intentionally rejects overlapping placement,
which remains unchanged. The supplemental real-WASM probe separately checks
closed-form lens volume/area, order independence, duplicates/contained tools,
actual mesh/STL volume and normals, original input preservation, late rollback,
handle capacity, and an actual overlapping-Hole document rebuilt after save/open.
The unchanged existing 22 scenarios remain the application regression selection.
The final consolidated run passes 22/22 existing cases in 133.393 seconds and
one separate supplemental probe in 8.042 seconds, with zero skipped, unexpected
or flaky results. Both original TypeScript checks pass. Evidence:
`docs/evidence/plaincad-overlapping-holes-2026-11-13.json`. All recorded local
runtime source hashes match the final tree, including the analytic union files.
Geometry/workflow assertions and original timeouts remain unchanged. The source
PlainCAD checkout and the other agent's main checkout were not modified.

## Analytic cylindrical-union construction — 2026-11-13

`trimmed.BoreCylinderUnion` retains the exposed circular arcs of up to 32 disks
and their inward cylindrical walls, with shared entry/exit rims or planar blind
floors. Inputs require convex stock, strictly interior footprints, perpendicular
parallel caps, and a common effective depth. Exact represented squared-distance
predicates classify duplicates, containment and tangency before constructing
transverse intersections. Geometry-based ordering makes the union independent
of input order. Unsupported contacts, ambiguous triple intersections and inner
footprint loops that could isolate material are rejected.

Adjacent arc supports share endpoint representatives only after agreement within
the existing BRep tolerance is checked. Model rim curves and interior p-curves
remain analytic; no mesh substitutes for the retained boundary. The new anchored
trim type is not yet supported by generic native BRep persistence. PlainCAD
persists its feature recipes, which are qualified separately.

Independent checks compare retained-boundary mass with closed-form two-circle
lens and three-circle formulas, plus a separate parametric cylinder-union
construction. Permutations, genuine triple common regions, disconnected loops,
duplicates/containment, exact tangent predicates, unresolved crescents, material
islands, mesh closure/budgets, and scaled/translated inputs pass. The final union
suite passes in 0.533 seconds; parent review and independent oracle tests pass.
Application adapter and browser qualification follow as a separate milestone.

## Scoped Hole implementation checks — 2026-11-13

The adapter's `cutScope` validates tools against original targets, requires every
selected target to lose material, returns the real indexed `HoleScopeError`, and
publishes independently owned outputs only after the whole operation succeeds.
Temporary probes and partial results are disposed on failure. The WASM preflight
checks crossed target/tool hits, missed-center and missed-target indices, preserved
inputs, repeated late rollback and the exact remaining capacity of the 256-handle
store. Overlapping cylindrical tools remain explicitly unsupported.

The bridge retains analytic cylinder walls and shared-edge planar floors for
blind pockets from either stock cap. Canonical reversal requires exact frame and
endpoint preservation. Tests cover all axes/signs, raised stock with 5 mm removed,
mixed and multiple blind/through recipes, original cap areas, floor normals,
cloning/placement, independent surface-integrated mass, closed meshes and rejected
closed cavities or ambiguous references. Proven disjoint bounds permit individual
scope misses; a new tool separated from previous bore bounds can query remaining
stock. Unproved overlap is never treated as a miss.

The PlainCAD angular setting maps conservatively to half the kernel facet-angle
bound. Tiny settings cannot silently disable checking. Actual guided two-hole
blind/through inputs satisfy the original 2% removed-volume mesh tolerance and
sampled normal-turn bound. Mesh and STL use the same helper; this does not certify
arbitrary surfaces between samples. A compact mesher for exact plane/rectangular-
cylinder domains keeps straight edges and axial strips unsubdivided while sharing
all circular boundary samples. The representative default one-bore mesh drops
from 2,048 to 144 triangles, preserving accuracy and matching display/STL vertices.
Independent tests verify linear mesh growth, exact triangle/subdivision budgets,
cancellation, unsupported-domain fallback and analytic-certificate bypass.
Selected trimmed tests, including cylinder, sphere, torus and cone poles, pass
(2.489 seconds); native bridge tests pass (0.861 seconds). The compact-specific
boundary limit delegates fine meshes to the general path;
explicit caller budgets remain enforced. The fine-deflection fallback regression
passes (0.623 seconds). Final bridge race (6.575 seconds), vet and WASM build pass.
Independent core, adapter and probe review approved the bounded implementation.

The final consolidated run passes all 22 existing PlainCAD scenarios in 151.620
seconds, plus one separate backend probe in 8.450 seconds, with zero skipped,
unexpected or flaky results. Both original TypeScript checks pass. Evidence:
`docs/evidence/plaincad-scoped-holes-2026-11-13.json`. The source-hashed snapshot
includes final compact meshing and angular mapping; geometry assertions and
original timeouts are unchanged. A post-run provenance supplement pins all 658
selected local Go source files and `go.mod`. Rebuilding those unchanged runtime
sources exactly reproduced the accepted WASM SHA-256. The runner then received
a metadata-only improvement to collect those dependency hashes in future runs;
its original run-time hash is preserved alongside the updated hash. All runtime work is isolated in the integration
worktree; the source PlainCAD and active main checkouts are untouched.

## Curved mesh angular-tolerance fix — 2026-11-13

PlainCAD's existing multi-body Hole STL check exposed coarse cylindrical facets:
centroid-only normal checks allowed eight-sided radius-3 holes at angle 0.2.
Refinement now checks vertices, edge midpoints and centroid, with explicit
singular-boundary/pole handling. Rectangular cylindrical walls use a structured
UV grid in the sampled fallback, preserving shared edge samples and avoiding
thin fan triangles. Cancellation and existing mesh budgets remain enforced.
Display and STL use the same tessellator and requested options.

Independent review and focused trimmed tests pass (1.568 seconds):
`TestSampledCylinderFallbackAngularBound`, `TestCylinderAngularCheckIncludesFacetEnds`,
`TestBoreMeshAngularSamples`, `TestBoreCylinder*`, `TestMultipleThroughBores`,
`TestPromotedSeamsAndPoles`, `TestTrimmedTorusAndConeQueries`, `TestSphereCutBox`
and `TestCertifiedWrappedConeTruePole`. Direct regressions check analytic cylinder
normals at every wall vertex, closure, and the previous fallback failure. Bridge
native tests pass, including previous transformed-bore cases and the application's
0.5% retained-volume mesh requirement. Browser qualification follows separately.

## Holed-face predicate projection fix — 2026-11-13

PlainCAD's guided Hole diameter edit exposed a valid radius-3 circular trim whose
seed triangulation contained a degenerate triangle. Arbitrary coordinate division
had changed exact collinearity before the triangulator's predicates. Projection
now uses reversible power-of-two scaling of the retained axes, keeps relative
resolution checks, and rejects exponent ranges that cannot preserve coordinates.
Area accumulation is translation-relative and winding compares signs directly.
Mesh validity checks remain unchanged.

The physical-collinearity regression, closed-prism check, scale tests from
2^-1000 through 2^1000, relative-resolution rejection and huge dropped-axis case
pass with `go test ./brep`. The bridge's exact guided radius-3 input also passes.
Focused trimmed-bore checks pass. A supplementary full `brep/trimmed` run hit
the 600-second Go timeout in `TestCombineSplineGraphNestedContours`, without a
preceding assertion failure reported. It used the pre-angular-fix snapshot and
is not a full-suite pass. Application scoped-Hole qualification is recorded
separately below when its existing scenarios complete.

## PlainCAD finite cylindrical through-cut milestone — 2026-11-13

Retained trimmed cylinder walls now support finite through-cuts on supported
convex planar stock. Exact endpoint predicates reject tools one ULP too short
and starts just inside a cap. Strict radial containment and perpendicular parallel
caps are required; blind, grazing, partial and side-crossing cuts are rejected.
Repeated separated bores rebuild from retained source recipes. Planar/cylinder
common clips the finite axial interval; general curved overlap remains unsupported.

Tests cover all axes and signs, independent curved-boundary mass integration,
current cap areas, finite/tangent/disjoint common, repeated-bore separation,
clone and supported placement ownership, closed meshes, and identical mesh/STL
sampling. Focused native tests, race tests (15.727 seconds), vet and WASM build
passed. Independent review approved the bounded core. Runner syntax and diff
whitespace checks passed. No dependency was added.

The consolidated original PlainCAD selection passes 11/11 cases in 78.999 seconds,
with zero skipped, unexpected or flaky results. Both original TypeScript checks
and the separate strengthened real-WASM probe pass (4.327 seconds). The report
`docs/evidence/plaincad-through-cuts-2026-11-13.json` preserves source hashes and
all per-case results. Only provider literals change in the staged application;
geometry and workflow assertions remain intact. Other checkouts are untouched.
Unselected application cases and full replacement qualification remain open.

## PlainCAD exact-circle bridge milestone — 2026-11-13

The bridge now retains a closed analytic cylinder for a single full circular
profile, with signed extrusion height and an explicit origin/axis. It publishes
analytic mass and conservative bounds independently of its display mesh.
Cloning, rigid placement, current cap-plane queries, meshing and STL retain the
curved B-rep. Nonrigid curved placement and curved Boolean combinations remain
explicitly unsupported at this milestone.

Focused tests verify all three axes and both height signs, mass/bounds, independent
retained-surface integration and cylinder evaluation, correct outward cap normals,
clone ownership, rigid placement, closed display topology and mesh-volume
convergence. Mesh and STL at the same deflection settings retain identical
triangle vertices after float32 export. Explicit null mesh settings are rejected;
tessellation budget exhaustion returns `LIMIT`.

Focused Go tests, race checks, vet and WASM compilation pass. The existing
PlainCAD negative-XZ circle profile-choice/preview/cancel/apply/UndoRedo scenario
also passes, along with both TypeScript checks and the real-WASM analytic-mass
probe. Its source-hashed report is
`docs/evidence/plaincad-circle-extrusion-2026-11-13.json`. Cylindrical through-cut
acceptance remains a separate next milestone. No dependency was added; work
remains isolated on `codex/plaincad-integration`.

## PlainCAD integration checkpoint — 2026-11-13

The first Git snapshot is `7b11e83` (work in progress, not full qualification).
PlainCAD integration continues on `codex/plaincad-integration` in a separate
worktree so the ongoing kernel development checkout is not edited by this task.

The initial standard-library-only JSON/WASM bridge supports planar boxes,
signed polygon extrusion with holes, planar cut/fuse/common, convex planar
hollowing, clone/transform, mass/bounds, mesh/STL and explicit handle disposal.
It independently checks planar embedding and shell nesting before publishing
material assertions; inward cavity shells do not increase the solid count.
Requests, profile vertices and live handles are bounded. Calls are synchronous;
browser cancellation requires terminating the owning worker.

Checks passed for this increment:

- `go test ./brep ./csg/poly ./csg/draft ./mesh ./exchange`.
- `go test -race ./internal/plaincadbridge ./cmd/plaincadwasm`.
- `go vet ./internal/plaincadbridge ./cmd/plaincadwasm`; focused tests, race
  checks and vet passed after the numeric parser review fix.
- `GOOS=js GOARCH=wasm go build -o /tmp/plainsolid-plaincad-reviewed.wasm ./cmd/plaincadwasm`.
- Independent review found and then rechecked strict rejection of JSON null
  numeric values (including JavaScript NaN/Infinity serialization), with no
  handle allocation or mutation on failure. No remaining bridge review findings.

Tests include both extrusion signs, holed volume, transformed bounds, independent
clone ownership, STL volume, cavity versus disconnected component counts, empty
intersection, both rectangular shell openings, malformed input and handle limits.
This is bridge evidence only; application browser results are recorded separately
in [PlainCAD readiness](PLAINCAD_READINESS.md). The full library test/race suite
and the full PlainCAD release gates were not rerun for this focused increment.

The next increment adds `planarFace`, which measures the current B-rep rather
than trusting authored metadata. It requires one finite matching face and
rejects missing or split/ambiguous supports using PlainCAD's existing plane
tolerances. Original, transformed, opposite-normal, tolerance-boundary,
coplanar-ambiguity and invalid-input cases pass; focused race, vet and WASM build
checks also pass. Independent review approved the query and the adapter after
correcting Boolean invariants, symmetric overlap thresholds, measurable shell
removal and skipped-test scoring. The query follows the actual
`OpenCascadeKernel.validatePlanarFace` contract (face-normal distance).

Real browser evidence on 2026-11-13:

- Six existing PlainCAD scenarios passed in a single selected run: XY/XZ/YZ
  signed extrusion with edit/save/STL, rectangular Shell with both cap choices
  and its full edit/undo/save/STL flow, and both stale-worker delivery cases.
- A separate Go-WASM/backend probe passed; it also verifies rejection of no-op
  and empty cuts, disconnected/contained joins, asymmetric-scale common-volume
  consistency, and negligible shell cavities. Disconnected export union remains
  allowed as in the original application.
- Both original TypeScript checks passed. Only provider string literals were
  adapted in existing fixtures; geometry/workflow assertions were retained.
- The next existing XY non-template modeling case failed at circular-profile
  extrusion preview. Circular/arc profiles are explicitly unsupported in the
  adapter; this is the next integration gap, not a passed feature.

The two full readiness reports, including source hashes, revisions/dirty states,
WASM hashes, adaptations and per-test results, are committed under `docs/evidence`.
See [PlainCAD readiness](PLAINCAD_READINESS.md) for commands and current scope.
No original PlainCAD source was modified. Production, bundle-size/performance,
the original OCCT-specific unit suite, and full application release checks are
not claimed by these targeted development-browser runs.

Local verification on 2026-11-12 used Go 1.27.4 on macOS/arm64 (Apple M4 Pro).
No third-party Go modules are present (`go list -m all` lists only `plainsolid`).
This record validates the current supported subsets, not full OCCT replacement.

## Completed checks

- `go test ./...`: passed.
- `go test -race -count=1 ./...`: passed across all packages and the example.
- `go vet ./...`: passed.
- `gofmt -l .`: no unformatted files.
- `GOOS=linux GOARCH=amd64 go build ./...`: passed.
- `GOOS=windows GOARCH=amd64 go build ./...`: passed.
- `go run ./examples/bracket -out /tmp/plainsolid-bracket.stl`: produced a
  112-triangle mesh with volume 7520. The automated example roundtrip test also
  imports the output and validates closed topology and expected volume.

Linux/Windows checks were compilation only; tests were executed on macOS. Go
1.23 compatibility and the independent OCCT build were verified in subsequent
runs below. Remote CI has not been run.

## Initial statement coverage

Measured with `go test -coverprofile=... ./...` and `go tool cover -func=...`:

| Package | Coverage |
|---|---:|
| brep | 97.1% |
| csg | 90.4% |
| document | 93.2% |
| examples/bracket | 56.7% |
| exchange | 88.1% |
| geom | 85.9% |
| mesh | 93.3% |
| query | 92.6% |
| solid | 97.7% |
| Total | 89.9% |

Coverage measures exercised statements, not numerical correctness or feature
parity. Independent reference calculations and malformed-input tests supplement
coverage; see the test files and STATUS.md.

## Focused fuzzing

Short local campaigns passed for exact-sign orientation, mesh boxes/extrusions,
planar B-rep boxes, convex box intersection, OBJ/STL/JSON/PLY and planar-BRep JSON
readers, document JSON, and extreme-scale triangle queries. The final query
campaign performed 913,036 executions over approximately five seconds with two
workers. Document fuzzing performed 25,490 executions; the final CSG campaign
performed 144,001 executions after the exact-rational fallback was introduced.
These are smoke tests, not long-duration robustness qualification. Seed cases
remain in the source tests; automatically discovered nonfailing cases live in the
Go build/fuzz cache and are not claimed as a committed fixture corpus.

## Benchmark smoke results

`go test ./... -run='^$' -bench=. -benchtime=100ms -benchmem` completed. One short
local run reported:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| SplineEval | 63.36 | 192 | 2 |
| Sphere | 135485 | 488128 | 28 |
| Closest | 49111 | 192 | 2 |
| Boolean | 476.1 | 1696 | 13 |

These are single-run observations on the stated machine, not stable performance
promises or comparisons with OCCT. Inputs are defined in the benchmark sources.

## Numerical regressions fixed during review

- Repeated domain-end knots in unclamped splines selected the wrong evaluation
  span; evaluation/derivative regressions now cover the left limit.
- Ray triangle triple products overflowed/underflowed at extreme scales; scaled
  arithmetic and explicit range errors replace silent misses/wrong distances.
- Closest-triangle Voronoi products underflowed for very small triangles; local
  normalization fixes the tested cases and rejects ill-conditioned ranges.
- Composed assembly transforms could become singular through underflow despite
  individually invertible inputs; world transforms are now checked.
- Near-dependent convex intersection planes were silently skipped; exact
  rational determinant/solve fallback and bounded plane-merging checks replace
  that behavior. Unstable reconstructed boundaries return errors.
- Nil document streams now return errors rather than panicking.

The remaining limitations in STATUS.md are still open. No full replacement or
production numerical-envelope certification follows from this record.

## Expanded implementation and differential runs

Later runs on the same host also used **Go 1.23.12**, the module's minimum Go
version, and passed the Go suite. Go 1.27.4 unit tests, race tests and vet passed
across the expanded package set. A coverage snapshot of that suite measured
84.6% across the library and both examples. These observations supersede the
initial coverage snapshot above; adding packages changed the denominator.

OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9` was built separately in
Release mode with AppleClang and the TKPrim, TKBO, TKDESTEP and TKDEIGES toolkits
and their prerequisites. Tests generate inputs at runtime. OCCT validity checks
remain enabled. The following differential cases passed:

- Box, sphere, cylinder, cone and torus area, volume and centroid.
- Sphere, parallel-cylinder and rotated convex planar union, intersection and
  difference; sheared nonconvex planar union/intersection/difference; aligned
  coplanar union/difference; orthogonal through-hole; bored sphere; orthogonal,
  oblique and multiple cylinder subtractions through a box; a convex prism
  fillet checked against an independently authored circular-arc extrusion.
- PlainSolid faceted and planar advanced STEP export of a box and holed prism
  imported by OCCT; OCCT advanced box imported by PlainSolid and re-exported.
- STEP standalone rational quarter-circle in both directions; quarter-cylinder
  export accepted by OCCT, and the spline support of the OCCT-written face
  decoded independently from its topology.
- Bidirectional STEP full sphere, ring torus, capped cylinder, cone and frustum.
- Bidirectional stepped shaft with two cylindrical walls and one annular
  shoulder. OCCT builds the reference by fusing two cylinders.
- OCCT hemisphere read into exact spherical cap and disk geometry. OCCT
  reads PlainSolid exports for the hemisphere and both smaller and larger caps.
- Bidirectional spherical zones with two latitude disks, including an
  independently clipped sphere and a rotated asymmetric zone.
- Selected planar draft against independent OCCT halfspace intersections: one
  and four drafted sides, an oblique neutral plane, and pyramid collapse.
- Bidirectional STEP cylinder with one oblique cap; the native fixture also
  checks closed-form area, volume and centroid. The external STEP check uses
  the documented OCCT tolerance for reconstructing omitted p-curves.
- Bidirectional bounded STEP parabola/hyperbola arcs, planar body and face
  colors, and conversion-based inch/foot units.
- Bidirectional planar STEP cavity shells and concentric hollow-cylinder
  bodies with annular caps. The local test corpus also checks a spherical
  box pocket against OCCT.
- OCCT XCAF assembly with repeated translated and rotated instances decoded
  into the STEP product graph and a document with one shared asset; OCCT
  read a PlainSolid-written nested assembly and verified placement and mass.
- Bidirectional IGES line, circular arc, rational curve and rational
  quarter-cylinder. OCCT's arc fixture uses type-124 affine placement.
- IGES type 142 matched model/parameter spline curves and type 144 planar
  polygon trims with a hole, checked in both directions by OCCT.

Modeling comparisons use relative area and volume tolerance `3e-7` and absolute
centroid tolerance `3e-7` in model units. The exchange curve and surface
checks use the tolerances stated in their individual tests. See
[the reference runner](../tools/occtref/README.md) for commands and build inputs.

Independent analytic tests additionally cover direct inertia, rounded box
area/volume, drafted holed prisms, ellipsoid transforms, curved trim seam
sharing, circular sweeps, elliptical lofts/extrusions, feature regeneration,
camera projection/picking and hidden-line clipping. Exact surface–plane
contour tests include closed conics and distinct near roots that must not be
stitched together. Exact planar distance and point-location fuzzing covered
52,440 and 348,442 generated cases, respectively. Fuzz campaigns covered the
new exchange readers, exact predicates, holed topology, sewing, collision,
convex Boolean and split subsets. None of these checks closes the unimplemented
acceptance requirements in STATUS.md.

Additional analytic regressions check nested spline-plane rings (annular face
and closed-form volume), two independently curved spline faces sharing a
projected chart (closed-form mass and centroid after persistence), native
spline-pair trim reconstruction and rejection of a recipe without contact,
and certified spline distance bounds for parallel and curved surfaces.
Mixed-axis tube tests distinguish exact endpoint identity from overlapping
enclosures and close an analytic circle across four axis changes. Separate
coverage checks detect missing tube regions and coincident input. Open
two-graph tests check mass, centroid and closed meshes on both sides of
the cut, preserving original curved faces and side walls through JSON. The
accelerated planar distance search reproduces the former exhaustive witness
and tie ordering; its differential fuzz run exercised 49,501 cases.

Additional pinned OCCT comparisons cover unequal two-distance and simultaneous
edge chamfers, planar protrusions and blind pockets (including concave oblique
sketches), torus elbows, and multi-step cylindrical shafts. The latter include
collars and grooves, with import of an OCCT cut groove whose support origins
were shifted along the common axis. Spline-graph Boolean tests use independent
analytic mass and centroid calculations for regular open and closed sections.
The local coverage checkpoint reached 83.1% of statements across Go packages
before the latest query and Boolean additions.

The shared-base spline Boolean has live OCCT comparisons for union,
intersection and difference against a separately authored curved prism and box.
Additional analytic fixtures cover mixed open/closed sections, nested holes and
islands, a reversed/sheared chart, knot insertion and parameter permutations.
The mixed subtraction's seed mesh is closed; its tight adaptive deflection
request can still exhaust the default budget. Certified implicit section uses
agree continuously after JSON round trips.

Latest exchange fixtures include hollow conical frusta, mixed axial profiles,
cylindrical and conical sectors, and product-local face styles in nested rigid
STEP assemblies. OCCT/XCAF verifies style overrides and repeated placements.


Standalone rectangular STEP spline faces pass live import/export of an
OCCT rational quarter-cylinder face. Exact sewn spline rectangles accept
matching edges represented with different degree, knots and orientation,
and reject distinct curves sharing endpoints. Open and closed mixed-axis
tube face splits preserve their shared edge graph through native reload.

The tube replacement regression preserves both exact support recipes and
checks its seed mesh and enclosed volume against an analytic curved pocket.
Exact rational trim subdivision resolves a winding query inside the original
curve hull, and reports an unresolved query under a deliberately small budget.
Spline chart identity distinguishes a degree-elevated transposed/reversed chart
from a one-ULP change to its surface. Shell embedding rejects a crossing face
and an inward copy of a certified warped prism. Nested, separated and full-face
joined spline shell Booleans pass 15 independent OCCT comparisons for volume,
area, centroid and shell count, including inset and partial-overlap interfaces.
Independently authored OCCT holed-face prisms qualify protrusions, blind and
through-pockets. Exact planar shell checks preserve nested void and island
ancestry and reject crossing or touching shells.

Bounded STEP faces and selected closed rectangular spline shells pass live OCCT
import and export. The fixtures include an interior trimmed rational patch and
a six-face warped prism. Import verifies shared vertices, edges and per-face
parameter curves; the closed-shell API requires a global embedding proof.
OCCT also verifies document export of multiple placements in inch units and
bidirectional IGES exchange of a trimmed rational patch. Different rational
spline bases pass analytic graph Boolean volume checks. Certified subinterval
enclosures distinguish winding across a transverse spline-pair trim.
Rigid IGES placement and a single interior rectangular hole on a rational
spline support have independent live exchange fixtures. Mixed-degree graph
fixtures also exercise nonunit parameter domains, a sheared affine XY chart,
and transposed/reversed operands.

A box and an independently parameterized paraboloid prism exercise the
complete single-contour transverse path. Exact weak coordinate extrema
account for corner-only contact candidates; verified mixed-axis junctions
and tangent component signs exclude duplicate interior segments. The
intersection is sewn from both original supports and its closed mesh is
checked against the analytic cap volume. The optional transverse OCCT suite
checks union, intersection and subtraction against independently authored
OCCT shapes. It compares closed-mesh volume, area and centroid with explicit
tessellation tolerances; enable with `PLAINSOLID_OCCT_TRANSVERSE=1`.
The two-contour fixture intersects a box with a spline roof with two peaks.
It verifies two disconnected components after intersection, a handle after
union and two cut wires after subtraction. In all three cases the retained
fragments map back to the original supports.
Exact tube endpoint tests verify both ends of an open branch against their
original model edges, including reversed parameter direction, and propagate
both shared edge splits. The same straight-branch proof exports a lossless
STEP spline curve whose length is independently verified by OCCT. Separate
STEP and IGES fixtures exercise a rational rectangular face with a hole.
The edge-crossing Boolean fixture uses six face pairs and twelve certified
segments. Independent OCCT boxes match volume, area, centroid and topology
for all three operations. A polynomial warp preserves the analytic volumes
while curving the top and bottom faces; native reload verifies the stored
exact endpoint witnesses. Separate OCCT XCAF fixtures validate repeated
certified spline-shell assets and rigid local placements through STEP and
native documents.
Automatic affine-chart discovery reproduces the open-wire result without
caller-proposed tubes. A separate plane-arc fallback discovers a closed
circle from exact trace-only knot cuts and revalidates against the original
supports. XCAF qualifies product-local spline-shell colors in repeated
occurrences, and independent IGES fixtures exercise multiple rectangular
holes with strict containment and separation checks.

The two-open-wire regression uses an independently authored split-chart unit
cube and a square through-bar. All three operations pass through explicit
sections, reversed section order, and automatic affine discovery. Analytic
volume/area expectations are respectively 1.25/8, .25/2.5, and .75/7.5, with
centroid (.5,.5,.5). The subtraction has Euler characteristic zero. Tests require
24 distinct generated section edges, complete disjoint source-face ancestry,
and complete disjoint original-edge/generated-edge ancestry; retained original
edge fragments follow their source parameter order. Independent OCCT primitive
Booleans confirm the same mass targets. The initial three-operation fixture
passed the race detector in 77.466 seconds; the subsequently expanded nine-route
provenance test passed normally in 32.042 seconds.

The repeated-face-pair fixture uses a unit cube and an independent quadratic-roof
prism: -.5 <= x,y <= 1.5 and -45/16 <= z <= (x-.5)^2-1/16. Analytic tube proposals
produce eight open branches across five ordered face pairs, including two
branches in each of three pairs. Both resulting wires lie entirely inside the
roof chart and cross original cube edges. Certified assembly checks two common
shells (Euler four), a union handle (Euler zero), and one cut shell (Euler two).
Normal tests additionally require a closed sampled mesh with deflection .01,
at most 64 subdivisions and 250000 triangles, and absolute volume discrepancy
at most .005 against union 319/24, common 1/24, and cut 23/24. Short tests omit
this sampled check. These mesh comparisons are not exact curved integration.
Independent OCCT-authored quadratic surfaces confirm the analytic volume,
area and centroid integrals; common centroid Z is 19/320. Full native trim
quadrature is separately gated by `PLAINSOLID_SPLINE_REPEAT_MASS=1`; that gate subsequently passed after integration caching and exact zero-strip
elision were added: 24.84 seconds for all three operations, including bounded
mesh checks. Native numerical quadrature agrees with the analytic volume and
XY centroid targets within 1e-8; its convergence estimates are not rigorous
error bounds.

A subsequent Go 1.23.12 `go test -short ./...` snapshot passed with the two-wire
and repeated-pair regressions present (trimmed 173.075 seconds; geom 10.754
seconds). The freshly rebuilt optional OCCT suite with STEP enabled passed in
11.421 seconds, including both new reference modes; only the explicitly gated
expensive transverse comparison was skipped. The later exact unification
history addition passed separately on Go 1.23.12 and under the race detector
(2.542 seconds), with package vet clean. Its tests cover deleted seams,
nonbinary exact edge parameters, removed subdivision locations, deterministic
reports, hole preservation, cancellation, and no partial results on budget
failure.

The mixed transverse fixture uses a box cutting one whole and one half
paraboloid lobe. Automatic plane tracing, exact iso-curve clipping, and an
exact affine chart relation at irrational tube junctions certify the complete
contact graph. All three Go operations pass topology and bounded mesh-volume
checks. Independent OCCT shapes confirm union volume 1.14943382037 and Euler
zero, intersection volume 0.001404400497 and two shells, and subtraction
volume 0.71734559155 and Euler two. The overlap is exactly 3πr⁴/4 for r=5/32;
the oracle also checks its analytic area and centroid. This does not qualify
automatic discovery of arbitrary nonlinear surface pairs.

Analytic sewing tests retain canonical periodic seams, closed circular model
edges and collapsed poles for sphere, torus, cylinder and cone. These include
affine placements, continuous model/UV checks, analytic mass comparisons,
watertight tessellation, and rejection of shifted evaluators and wrong seams.
Exact edge contact tests recover a shared model point and split both original
shell edges, and distinguish a rational but unrepresentable split parameter.

Independent OCCT exchange fixtures validate concave orthogonal rational spline
faces with multiple holes in STEP and IGES and a connected open STEP spline
shell with an exact shared boundary. Malformed fixtures exercise notch,
touching/nested holes, mismatched shared edge interiors, and inconsistent
source weight ratios. Go 1.23, race, vet and focused parser fuzz smoke passed.

The affine spline fallback and section fixture exercise coincident and
simultaneous edge events, preserve original spline supports and rational edge
ancestry, and reject nonrepresentable conversion. The output can be supplied
to another affine Boolean. The independent mixed boundary-partition tests
verify exact paired model/UV curves and volume preservation before selection.
The sheared unit box section retains exact corner coordinates and source IDs.

Trimmed spline distance tests check a through-hole, a circular trim, a closest
point on a nonplanar face, affine placement, cancellation, and resource limits.
The distance result preserves certified bounds on interruption. Scalar tests
cover rational underflow, preventing a positive upper bound from rounding
inward to zero. Scattered spline fitting is tested on irregular parameter
samples with fixed nonseparable rational weights and singular input rejection.

Wrapped surface fixtures prove both seams of a rational toroidal support,
verify a watertight mesh and independent volume integral, and preserve seam
topology through exact edge splitting and native reload. Reload rejects a
modified seam recipe. The selective fillet fixture compares OCCT's native
unequal-radius edge fillets with Go mass and area to 1e-8 and centroid to 1e-9.
Document tests preserve selected radii through save, edit and undo.

Exact affine-parameter tube certificates now remove only proven zero-width
integration strips whose exact endpoints equal the original face chord.
Proofs are cached per segment within one mass call and bounded independently;
failed proofs preserve quadrature. A call-local bounded trim sample cache also
reduces repeated implicit-root evaluations on genuine curved strips. The
48-strip planar fixture improved from 2.743 seconds to 8.82 milliseconds per
mass call without changing volume, area, centroid, or topology. Regression
checks include a one-ULP mismatched chord (not elided), paired proof reuse,
cache saturation, and rotated/scaled placement.

Multispan rational UV fixtures retain an exact internal join at 7/3, prove
clipped and reversed path topology, and reject an approximate declared
endpoint and a path that intersects an existing curved hole. Nested and
disjoint face cuts keep the original hole edge IDs and the solid volume.
An oblique affine spline solid split and rejoin preserve exact support
identity, cap coordinates, retained edge intervals, topology and analytic
centroids. The exact splitter rejects a generated coordinate of 1/3.

Wrapped spline traces close across irrational seam roots and reconcile
strictly transverse corners using exact tangent signs. The canonical rational
toroid's quotient embedding, outward orientation and complete seam edge
agreement are proved independently of its mesh. STEP solid exchange requires
that proof and explicit source closure; the pinned OCCT oracle verifies both
directions and compares adaptive area and volume. Toroid point queries cover
axial meridians for the general certified rational profile and all represented
3D points for the exact circular-profile subset.

Certified full-patch meshes preserve wrapped seam indices. Independent
bilinear fixtures verify bounded interior error, zero error for exact affine
vertices, and quadratic refinement of two-direction correspondence bounds.
Explicit spline and implicit paired tube trims have independent exact
one-direction chord bounds, including rounded model vertices. Nonlinear
edge contact tests include independent reversed parameter ranges, exact
overlap and an irrational root with separate enclosing brackets.

Canonical capped cylinders and frusta match analytic circular volume, area
and centroid, and native cylinder reload preserves continuous cap/rim
agreement. Exact finite-line fixtures include noncircular profiles, a
nonbinary cap parameter, rim deduplication, coincident generators and quartic
torus tangencies. Cylinder Booleans check separated height intervals,
through and blind bores, enclosed cavities and original support ancestry.
Nested-component tests recover an island within a cavity, reject incorrect
cavity orientation and verify volume. The circular torus offset fixture
proves the displacement is normal to both partials with exactly the required
length. Wrapped STEP fixtures compare independently authored cylinder,
frustum, tube and blind-bore material against pinned OCCT. Wrapped feature
recipes retain stable cap roles across save, edit, regeneration and undo.

Cubic trim fixtures prove a projection that needs exact subdivision, detect
a crossing, and leave a folded projection unresolved. Graph fixtures
certify an isolated quartic contact and a strict height gap of 1/1024;
the graph Boolean dispatcher retains the appropriate supports even when
ordinary section isolation is limited to a single cell.

Selected algebraic face inverse tests locate rational world points with
nonbinary and irrational original UV, distinguish trim interiors and holes,
and recognize exact isoparametric contacts. A finite axis-directed line
fixture retains its original signed parameter and groups coincident rational
face candidates at a shared edge. Same-domain curved spline healing accepts
independently refined knot bases, restores rectangular embedding eligibility,
and can undo an exact wrapped-seam subdivision after native reload.

The two-sided mesher qualifies polynomial and rational biquadratic cells,
including nonseparable weight fields. Knot-aligned and adaptive grids retain
every original knot and weld only certified wrap seams. A local refinement
fixture reduces 64 uniform triangles to 40 while preserving its error bound.
The wrapped station loft retains two exact uses of each intermediate rim
edge and matches independently integrated analytic area, volume and centroid.
Native reload and reordered faces independently recover its stations and
original IDs; inward orientation is rejected. Exact axis-line queries
discard interior cap hits, and an oblique circle chord at a station reports
two algebraic contacts with both wall IDs. Scene tests cover nearest exact
cap ordering, ties, the toroid hole and a coplanar loft cap entry.
The wrapped-bore example round-trips exact STEP and native material geometry.

New pinned OCCT differentials cover stepped wrapped Booleans, blind bores,
wrapped cavities and shoulders, and doubly wrapped hollow toroids. The
dedicated wrapped cavity fuzz target passed a 30-second run with 124
executions; this is only a bounded smoke check.

A two-band wrapped station loft exchanges with pinned OCCT in both
directions and agrees with independently calculated area, volume and
centroid. STEP rejects a disconnected or incorrectly owned interior rim.
The conforming straight-trim shell mesher was tested against a warped
prism and a rational reparameterization of the same material, including
face ancestry, shared sample counts and convergence to volume 5. A
simultaneous edge incidence fixture independently identifies the two
entering face pairs where a warped bilinear roof meets a planar box edge.

Whole-shell simultaneous-edge fixtures prove the intersection volume
1/768, the union and difference volume identities, and native persistence.
The curved variant uses a determinant-one vertical quadratic warp, tests
all three operations, and tests automatic proposals from an independently
recovered warp coefficient. Pinned OCCT agrees on volume, area and
centroid for all three operations. Original-surface proof is repeated, and an omitted
simultaneous terminal is rejected. Selected curved adjacent supports use
an exact separating quadric; shell orientation can use exact signed
volume bounds after pairwise embedding is established.

The adaptive polygon-shell fixture uses 336 triangles versus 768 uniformly
with the same certified deflection target. A curved planar annulus
fixture proves exact chord polygon triangle coverage and a two-sided
filled-region distance, including a point in the gap between the original
hole and its chord polygon. Model rounding is bounded separately.

Mixed wrapped STEP is qualified with a live twelve-family area and volume
comparison. The mixed product adapter independently verifies repeated
XCAF product identity, occurrence placement and adaptive volume with
OCCT, including a rotated shared rim fillet asset. The IGES filled-sheet
fixture verifies independently charted disks matching multiple annulus
holes by reversed whole model curves. Pinned OCCT reads all three faces
and matches their combined area. Algebraic edge and raw-plane event tests retain exact brackets in
both original ranges, including independently projected events less than
2^-80 apart.

Coaxial circular loft union and intersection split at an exact radial
crossing; analytic volume and centroid, shared rim radius and retained
source walls are checked. Strict radial subtraction produces a closed
annular shell with two cap holes and independently checked mass.
Native reload independently revalidates that annular shell and tests
material point classification, exact cap and rim line hits, and picking
through the bore. The circular-frustum offset test independently proves
the same squared displacement and orthogonality to both chart tangents
at nonbinary parameters, plus the capped result's analytic volume.
The loft and offset feature recipes preserve checked face roles through
document persistence, regeneration and undo. Exact document picking selects
the nearest of two occurrences of one asset and rejects unrepresentable
parent composition or local ray translation.

Local Linux amd64 and Windows amd64 cross-builds with CGO disabled passed.
The native trimmed JSON fuzzer passed a 20-second smoke run with 182,873
executions. Neither check replaces executing the full platform CI matrix.

Quadratic shared-base saddle union and intersection preserve four exact
top sectors and one shared saddle vertex. Independent analytic volumes
qualify both axis rays and diagonal rays to the four chart corners. Native
reload rechecks all model/trim edges, including graph side walls. An
explicit pinch test rejects subtraction with touching material components.
The independent OCCT saddle fixture agrees on union and intersection
volume, area and centroid; an off-center saddle is recovered exactly.

A knot-aligned partial-coincidence fixture checks all three graph Booleans,
including doubled input ancestry, exact shared coedges and reversed empty
subtraction. A connected coincident region becomes one retained face with
a map from each original cell. Pinned OCCT agrees on union and intersection
area, volume and centroid. The raw OCCT cut agrees on volume and centroid
but reports additional area (3.60554013101 versus 2.42205294213);
an independently authored OCCT boundary for the regularized difference
agrees with the smaller area. Enclosed island and annular difference fixtures independently
check analytic volumes 1/16 and 7/16 and reject pinched material. Exact
knot clipping subdivides spline face integration seeds at derivative creases.
The non-knot axis-line fixture checks all three operations at exact U=1/4
and returns a proof at U=1/3 without rounding that native topology endpoint.
A higher-degree fixture also passes exact factor division and a strictly
signed quotient test.

An independent pinned OCCT rolling fillet agrees with the selected circular
cylinder rim blends on area, volume and centroid. Line fixtures compare
analytic blend roots, algebraic cap chord endpoints, tangency multiplicity
and exact nonbinary generator parameters; they verify source-face ownership
and scene picking. The nearest-boundary query encloses analytic distances
and returns an exact algebraic witness on its original face. Annular cap
queries distinguish disconnected chord intervals, inner tangencies and
isolated outer tangencies, even for nearly coincident radii. Complete circular-cylinder offset tests compare dilation
volume with Steiner's formula, erosion with exact dimensions, and the
offset of an existing blend with exact nonbinary surface evaluations.

A six-face curved shell stitches 12 seams and eight explicit corners. Its
mesh preserves analytic volume 1 and passes an independent exact BVH display
embedding check. A mutated crossing is rejected with source face ancestry;
limits and cancellation cannot certify an incomplete traversal. A separate
seam fixture matches exact degree-one and degree-two rational trims with
distinct reversed active ranges, and rejects a one-ULP perturbation.

Parallel contact-line and rectilinear contact-grid Boolean fixtures verify
analytic mass, disjoint difference shells and rejection of crossing
pinches. Repeated even contact at U=1/3 selects the ordered envelope and
rejects positive pinched subtraction. Exact oblique sections compare
original supports across different knot partitions, retain a nonbinary
terminal, and match the independent enclosed-volume integral. Exact
vertical queries retain both top owners at a coincident height.

Two certified curved annuli cross an off-center C0 knot, on polynomial
and rational supports. Exact triangle-to-knot clipping and a piecewise
Jacobian bound establish the requested distance; exhaustion of cumulative
cell and piece limits returns no certificate. Mixed STEP cap and blend
face colors are independently verified in XCAF and through native document
round-trip. The multi-band station bore passes independent OCCT STEP
import/export with analytic area and volume checks, and native recovery
retains independently partitioned inner and outer walls.

Parallel and crossing oblique line Booleans pass analytic volume checks,
including disconnected subtraction and rejection of a crossing pinch.
Axis-aligned and rotated ellipse fixtures check exact four-arc topology,
closed lenses and annular differences, analytic volumes and native
reload with full edge revalidation. The pinned OCCT ellipse Booleans agree
on volume and centroid. OCCT's trimmed-area result changed with quadrature
settings; independent integration of the graph metric agrees with
PlainSolid within 1e-7 for all three operations. Exact rational conic arcs
retain nonrepresentable controls without rounding them into native topology.

Multi-span seam fixtures match separate original parameter ranges and knot
partitions, preserve nonbinary cuts 2/5, 2/3 and 6/7, and recertify the mesh
after assigning shared display vertices. The three-face collection checks
declaration-order invariance, opposed seam incidence, vertex fans and exact
absence of undeclared display contacts. Curved UV rational-cut subdivision
also preserves exact mandatory samples before any mesh construction.

The full Go 1.23 short suite and Linux/Windows amd64 CGO-disabled builds
passed at a checkpoint in this implementation. A full race attempt exceeded
Go's default ten-minute package timeout on the large trimmed suite; the CI
race limit is now explicit at 35 minutes. Recent focused race runs pass for
the exact contact, offset, exchange and meshing work.

The rational graph-line fixture proves that two independent top equations
share an irrational root, then merges their original face ownership.
Rounded-cylinder section tests preserve an irrational radius as a rational
radical and verify both tangency owners at a blend seam. Horizontal splits
of station lofts, bores, and the straight portion of rounded cylinders
preserve the expected source faces and analytic piece volumes. The
annular-frustum STEP fixture passes both directions with pinned OCCT,
including a shared colored product through native document persistence.

Native version 4 reload proves loop topology and the complete shared
model edge for exact rational quadratic trims. A roundtrip retains the
nonbinary weight 1/3 and rejects a sub-tolerance control edit. Exact
restriction and inverse restriction recover all homogeneous coefficients
after split and merge. The represented-only edit rejects nonbinary vertices;
the native version 5 edit stores the rational model point
`(19/26, 57/104, 24)`, tests affine placement and inverse merge, and rejects
an edited display vertex one ULP away. Four nonbinary conic Boolean cases
agree with independent analytic volume and retain the conic junction
`(5/12, 2/3, 2)` on reload. Edge comparison independently proves equality
across an irrational knot event.
A separate native-control annulus mesh verifies exact UV triangle coverage,
the hole topology, and an independent oblique-chart area integral.

Graph line checks also cover roots separated by less than 2^-43,
coincident envelope intervals with changing owners, and side-wall or
interior material spans across a tangency. Exact root order and height
sign are established before classifying the intervals.

The nonplanar STEP oval fixtures use independently authored polynomial
and genuinely rational biquadratic supports in pinned OCCT. Exact
homogeneous UV composition checks complete quartic model curves, with
separate native and both-direction OCCT roundtrips. Independent OCCT fixtures
also cover a curved oval annulus and two separately charted faces sharing
four exact seams. Exact graph scene
picking tests translated occurrences, a shared top owner and wall intervals.
A station-bore feature roundtrip preserves inner/outer wall roles and caps
after native reload, parameter edit and undo.

The algebraic knot enclosure fixture preserves exact continuous identity
across different knots and verifies a common represented point with a
model-space error bound that includes rounding. Plane and finite-line
queries independently identify the shared irrational parameter and both
cell owners. Trimmed-face line tests exclude a contact inside a hole and
retain ambiguous boundary evidence. A separate closed six-face warped box
with native exact curved seams verifies Euler characteristic 2, positive
display volume close to the analytic source volume 1, and no undeclared
display contacts in both the exact BVH and an independent triangle sweep.
These are boundary and display proofs; continuous smooth-shell embedding
requires its own evidence.

Algebraic subdivision persists both coedge restrictions of an irrational
split and rejects a nearby but different source-plane recipe. Two cuts less
than `2^-100` apart survive save and reload as distinct topological vertices;
the polyline reports the collapsed display segment without welding them.
Curved edge/face isolation independently verifies two regular roots, excludes
one outside an irrational cut, retains a singular tangent as unresolved and
excludes a root in a target trim hole. Two differently shaped spline shells
yield three boundary crossings in each direction. Both terminals of the
nonplanar tube fixture also pass independent edge/face recovery and the
original tube proof.

The holed certified mesh fixture uses a ten-face annular prism with 24
shared seams and 48 exact model-edge uses. Its display is a closed genus-one
manifold with no undeclared contacts and signed volume 0.4114837646484375
against independent source volume 5/12. A separate native version 5 lens
mesh proves the shared nonbinary corner against each original owner.
Pinned OCCT qualifies the closed spline lens and the document roundtrip,
including a shared local lens with translated and 90-degree rotated
occurrences. The local analytic mass comparison records OCCT integration
residuals separately from exact STEP geometry proofs.

The affine partition display test independently checks polygon and triangle
coverage, positive projected fragment areas and opposite shared cut use.
The nonlinear partition test instead composes the cut on the original spline
with exact algebraic UV endpoint images; its certified display reuses both
original seam vertices. Partitioning both faces preserves Euler characteristic
and all original ancestry across reload. Exact common-projection edge queries
prove the upper and lower cuts meet only at their two algebraic terminals,
and distinguish full coincidence with a reloaded copy.

Selected algebraic fragments can be sewn from independent native inputs.
The fixture preserves Euler characteristic 2, checks all vertex links and
rejects an omitted edge pair, reversed recipe, and endpoint-only matches.
The capped lens split proves two oriented material halves and their exact
sections; rational volume intervals enclose independent analytic cap segment
integrals. Line tests retain original parameter units, the generated cap,
coincidence on the lower face, a shared conic rim and an isolated tangent.
The affine fill certifies both directed distances to its exact source region.
The curved fill retains common cut samples, exact fragment ancestry and
two-sided distance bounds; a genuine C0 knot crease is independently tested.
Autonomous tube terminal tests isolate two irrational contacts and verify
all four chart coordinates against the original boundary and tube.
The independently authored partitioned STEP lens uses six authoritative
shared pieces at rational lower-chart knot crossings and preserves both
original supports. A 17-significant-digit OCCT writer retains its represented
pole coefficients; shorter rounded source output is deliberately rejected.
The shared partitioned lens fixture also passes independent OCCT validation
with translated and signed-axis rotated occurrences of one product. Native
reload retains one asset and all six original seam records; stale lineage
and inconsistent placement are rejected.

The exact tube trim fixture verifies two different face limits, reversed
original spline knot units, hole exclusion, and same-point ownership in both
charts. Its independently constructed tangent uses
`u²+v²=1/2` and `41u+113v=85`: complete trimmed face-pair coverage retains
only the tangent at `(41/170,113/170)`. A second pair retains exactly one open
interval and a patch that excludes part of a face trim is rejected.

The closed split-lens mesh fixture verifies one shared sample network for the
rim and both cap owners. Independent triangle checks see Euler characteristic
2 and no undeclared contacts on either half. Represented volumes are
approximately 0.00323363018 and 0.00091374464; separate analytic source
volumes are 0.00339162902 and 0.00094712385. The source-image distance
certificate does not imply a volume error bound.

A closed regular spline-pair section splits both chart disks into an inner
disk and an outer fragment with the opposite cut use. The analytic circle
fixture checks the original UV winding and height signs, assigns disjoint
holes on opposite sides, and reconstructs the trimmed topology from source
geometry after native reload. An open section fixture also assigns two
original holes to opposite fragments and partitions an original coedge
twice without conflating its parameters.

The closed tube lens retains both curved supports. Its analytic volume
`π(5/16)^4/2` is enclosed by rational bounds at two refinement levels.
The volume and exact centroid `(1/2,1/2,-11/384)` are independently
enclosed; scaling the common model XY projection by determinant six scales
the volume interval accordingly while preserving the original chart units.
Vertical queries check interior, rim and exterior cases. Exact finite line
tests include irrational endpoint roots, an isolated tangent, a coincident
top interval and graph roots outside the material.

The native trimmed-solid conversion revalidates every paired edge after
save and reload. The bounded shared-base subtraction fixture proves both
inputs strictly above the common base, reconstructs the selected material
on load and rejects a changed minuend that would require side-wall assembly.
Disjoint loop fixtures recover both components and their rim ownership;
an independent concentric fixture verifies the containment and annulus
incidence logic on two separately certified original circular loops.

Independent OCCT IGES tests verify curved oval disk and annulus area and
an annulus/fill sheet with distinct original charts and one reversed seam.
Source and regenerated surfaces and complete homogeneous curve agreement
are checked independently from the reader's area residual.
The explicit IGES open shell independently qualifies one shell, two faces,
eight edges and eight vertices, with all four shared edges used oppositely.

The native tube graph envelope fixture checks union and intersection against
independent volume integrals, preserves two original top faces, the common
base and four domain walls, and verifies every shared section use.
Reverse subtraction retains an exterior shell of Euler characteristic zero
and four side walls. Its volume agrees with the independent integral.
The tube lens inertia bounds include the analytic axial inertia and XY product.
Its boundary-distance search retains exact lower and upper bounds when its
cell budget cannot close the requested gap.

The exact nested quartic graph fixture automatically recovers two
rational-radius circles and verifies original-support coverage with 116
tubes. It assembles a genus-one annulus and separate inner disk, certifies
the annular volume against its analytic integral and reconstructs both
regions within the existing 128-cell native recipe cap.
The annular footprint reuses 128 exact-source boundary samples, has 128
triangles and encloses the source fill within a certified distance of 0.04.
Its represented area 0.2302451043 is compared separately with the analytic
annular area 0.2300971182. The nested native envelope has eight faces and
Euler characteristic two.
The same annulus produces a closed genus-one display with 6708 vertices
and 13416 triangles. Exact represented triangle checks found no undeclared
contacts. Its signed display volume is 0.0004485565 against analytic source
volume 0.0004628764. Independent quartic ellipse tests reconstruct a
diagonal metric with Y coefficient four and verify both loops with 112
tubes, checking coverage and topology separately from factor recovery.

Two independently sewn warped spline boxes produce a six-branch section
wire across their original shell edges. The native shell section recipe
reproves coverage and exact vertex ownership, and rejects reassignment of
an original face pair. A separate closed branch qualifies the scanner on
two whole shells without relying on edge crossings.

The closed IGES lens qualifies independent source and regenerated files as
one solid with two faces, four shared edges and four vertices. Adaptive OCCT
area and volume comparisons remain separate from exact material and
homogeneous edge-composition proofs.
The raw-weight document fixture additionally retains all 12 original curve
bases through native reload and checks the regenerated file with OCCT.
Modified poles, weights, owner IDs or native material are rejected even when
the enclosing recipe hash is recomputed.

Independent projected trace tests cover closed loops in XZ and YZ with
scaled, swapped original charts. The XZ material test automatically
discovers the section, reconstructs it from native geometry without
discovery hints, verifies the physical Y orientation of the two faces, and
bounds the exact original XYZ centroid `(1/2,-11/384,1/2)`.
The same test locates two irrational lower-face line contacts, verifies
both original XYZ plane-section curves at their shared rim, and proves a
zero boundary-distance witness on the original upper support. Closed mesh
race tests for XZ, YZ and ZX each check 1696 triangles, exact shared seam
indices, positive physical orientation and no undeclared triangle contacts.

The automatic dyadic guidance fixture closes the nonfactorized quartic
`x²+y²+x²y²−409/4096` on two original graphs with a common bilinear height.
Neither radial nor concentric ellipse recognition accepts that numerator.
The independent fresh cover and topology proofs pass without caller cuts.
The isolated coupled quartic `9*(x⁴+y⁴−x²y²)` qualifies shared-base union
and intersection at volumes `173/80` and `2`, and distinguishes an empty
difference from a pinched difference.

The four-branch shell wire crosses two seams at physical `Y=√(1/2)`.
The original model roots match between adjacent chart branches; native
section reload and subsequent seam subdivision retain both coedges and
Euler characteristic two. The warped fixture independently rebinds its
curved graph seam constraints and rejects malformed projection axes.

Independent OCCT qualification of six- and seven-edge IGES lens records
preserves two faces and the full split topology, along with original raw
model/owner curve bases and both owners' knot-cell ancestry. Irrational
partition cuts outside the bounded exchange format are rejected.

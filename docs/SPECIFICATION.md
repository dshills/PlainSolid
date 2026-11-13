# PlainSolid specification

## Objective and completion definition

PlainSolid is intended to become a portable Go replacement for the public capabilities of Open CASCADE Technology (OCCT). This means behavioral coverage of geometry, topology, modeling, exchange, visualization data, and application documents; it does not mean source or ABI compatibility with C++. The initial module path is `plainsolid`; a public repository module path must be selected before publication. The implementation must remain honest about supported domains. A faceted approximation is never an exact curved boundary representation. Passing the present tests does not establish OCCT parity.

Full replacement is complete only when every capability in the matrix below is implemented, documented, and passes its acceptance corpus. Until then the project is an experimental kernel. Unsupported operations must return explicit errors or be absent from the API; they must never return a plausible but incorrect shape. No runtime OCCT, CGo, native libraries, or third-party Go modules are required for the initial implementation. Optional renderers and reference-test tools must stay outside the kernel dependency graph.

## Reference baseline

The following upstream material was inspected on 2026-11-12:

- https://github.com/Open-Cascade-SAS/OCCT — overall platform scope and source organization.
- https://github.com/Open-Cascade-SAS/OCCT/tree/master/src — FoundationClasses, ModelingData, ModelingAlgorithms, DataExchange, Visualization, ApplicationFramework, and Draw.

The observed upstream head was `4dbfdcc448c0b3e565488afcd626c84483172db9` (commit dated 2026-11-11). The pinned revision has been built and used for an initial differential corpus; see [the oracle](../tools/occtref/README.md) and [verification record](VERIFICATION.md). Current Go implementations are independently authored; no OCCT source or fixtures have been copied. Any future copied or translated material requires provenance and applicable notices. Reference execution is development tooling only.

## Capability and acceptance matrix

| Domain | Required full-target behavior | Acceptance evidence |
|---|---|---|
| Numeric foundation | 2D/3D vectors, frames, affine maps, robust orientation predicates, bounds, numerical integration/root finding and linear solves | Algebraic identities, degeneracy/scale tests, exact or high-precision oracle |
| Curves | Lines, circles, ellipses, conics, Bezier, rational/nonrational B-splines; evaluation, derivatives, continuity, trimming, periodicity, knot insertion/removal, interpolation, fitting, projection, arc length | Analytic curves, rational circles, knot identities, residual and error bounds |
| Surfaces | Plane, cone, sphere, cylinder, torus, ruled/revolved/extruded, Bezier and NURBS; derivatives/normals, trimming, periodic seams, fitting, projection | Analytic derivatives, seam/pole tests, bounded fitting errors |
| Intersection | Curve/curve, curve/surface, surface/surface; isolated/tangent/coincident solutions, parameter curves | Residual checks, complete branch detection, near-coincident and tangency fixtures |
| Topology | Vertex/edge/wire/face/shell/solid/compound graph; orientation, shared identity, tolerances, p-curves, seam/degenerated edges | Incidence checks, closure, orientation, geometric/topological consistency |
| Construction | Primitives, planar faces, holes, extrusion, revolution, loft, pipe, sweep, sewing | Valid topology, analytic mass properties, section checks |
| Boolean modeling | Union, intersection, subtraction, section, splitting, same-domain unification for arbitrary supported B-reps | Volume identities, empty/disjoint/touching/coincident cases, curved fixtures, history mapping |
| Modification | Fillet/chamfer, offsets, thick solids, draft, defeaturing, local edits | Radius/distance checks, self-intersection detection, failure diagnostics |
| Healing | Tolerance repair, orientation, wire closure, sewing, small-edge handling | Defect reports, bounded displacement, no silent destructive repair |
| Queries | Point classification, distance/extrema, collision, bounding volumes, area/volume/centroid/inertia | Analytic reference values and invariant tests |
| Tessellation | Trim-aware, adaptive surface triangulation with deflection/angular budgets, shared-edge conformity, BVH | Watertightness, error bounds, seam/pole/holes tests |
| Exchange | Native exact graph; STEP AP203/AP214/AP242 subsets with explicit coverage; IGES geometry/topology; STL, OBJ, PLY, glTF | Independent reader roundtrips, units, assemblies, names/colors, bounded malformed input |
| Application data | Documents, labels/attributes, assembly transforms, units, transactions, undo/redo, persistence, stable naming/history | Save/load identity, rollback, assembly instance correctness |
| Visualization | Headless scene/selection, camera, clipping, presentation data, hidden-line output; optional rendering adapter | Picking correctness, deterministic image/vector fixtures, no graphics dependency in kernel |
| Tooling | Go examples, reproducible benchmarks, fixture runner, migration guide, capability/version reporting | Clean checkout tests, race/fuzz/architecture checks, documented unsupported cases |

## Architecture

Use small packages with one-way dependencies. `geom` owns numeric values,
transforms, curves and surfaces. `mesh` owns indexed faceted geometry; `brep`
adds shared planar topology with holes. `brep/parametric` adds analytic patch
topology and `brep/trimmed` adds UV trim loops with curved p-curves. `solid`, `csg` and
`intersect` provide restricted Boolean and intersection operations. `heal`,
`collision` and `query` provide repair and inspection. `exchange` handles
file formats, `document` handles mixed assets and feature recipes, and `scene`
handles headless presentation.

Geometry constructors validate finite coordinates, dimensional consistency, parameter domains and nonzero denominators. Read-only geometry should be safe for concurrent evaluation: copy caller-owned spline data on construction and never mutate it during evaluation. Exported mesh slices remain caller-owned and require caller synchronization during mutation. Solid constructors copy their box inputs. Heavy algorithms shall eventually accept context cancellation and explicit work budgets.

Use float64 and right-handed Cartesian coordinates. Angles are radians. Core geometry is unitless; documents/exchange attach explicit length units. No package-global mutable tolerance. Each future approximate algorithm receives an options structure with absolute/relative/angular tolerance and iteration/work limits. Exact cell operations compare input coordinates directly; they do not snap near coordinates. Zero-volume contacts are removed by regularized Boolean operations.

Error semantics distinguish invalid input, singularity, unsupported domain, iteration exhaustion and resource limits. APIs must not panic on malformed external files or return NaN as an undocumented failure signal. Low-level vector algebra follows normal float64 arithmetic; callers must validate finite results when constructing geometry. Extreme finite values can overflow and must be included in numerical testing before production claims.

## Initial implemented-domain contracts

### Geometry

`Vec3` supports arithmetic, dot/cross products and normalization; `Box` stores ordered bounds. Affine maps use an explicit 3x4 matrix, compose predictably, and reject singular inversion. B-splines use expanded nondecreasing knot vectors and positive rational weights. Parameter evaluation is restricted to the active knot interval; endpoint handling is explicitly tested. Tensor-product surfaces use a documented control-net layout. Initial analytic query coverage is documented by the exported Go API, not inferred from the full target matrix.

### Faceted models

`mesh.Mesh` has vertices and oriented indexed triangles. Structural validation checks finite vertices, valid indices and nondegenerate triangles. Closed-solid validation checks incidence and orientation; self-intersection detection is provided separately by `collision`. Sphere and cylinder outputs are polygonal approximations controlled by sampling counts, not exact surfaces. Extrusion supports a simple planar polygon; holes and general sweeps require later work. Mass properties apply to consistently outward-oriented closed meshes. Signed volume preserves orientation.

### Axis-aligned solids

`solid.Solid` is a union of positive-volume axis-aligned boxes. Boolean operations split at all input box coordinates, classify open cells, and retain those satisfying the operation. The result is an exact set operation in the input-coordinate domain, subject to float64 arithmetic. Results contain disjoint interiors. Operations have configurable cell and occupancy-work budgets to prevent cubic coordinate-grid explosion and excessive repeated box classification. Volume and centroid sum disjoint cells; containment includes boundary. Mesh conversion removes internal faces and produces conforming grid-face triangulations. Nonmanifold contacts may be represented as a regularized set but are rejected by mesh solid validation when appropriate. Empty solids have zero volume and no centroid/bounds. There is no claim of arbitrary-polyhedron or curved-surface Boolean support.

### Exchange

OBJ and STL are mesh exchange, not CAD topology preservation. Readers validate indices and numeric values, reject malformed/truncated input, and impose input limits. STL coordinates lose precision when encoded as float32. Roundtrips compare within format precision. Normals are derived from geometry. Native versioned mesh JSON is distinct from future exact CAD persistence. Imported meshes may be open; callers must request solid validation before solid-only queries.

## Verification and release requirements

Every feature needs ordinary valid examples, boundary/degenerate/error tests, and invariance tests where applicable. Geometry must test translation/rotation/scaling; splines must test endpoint and rational cases. Boolean tests must cover disjoint, contained, overlapping, face-touching and coincident inputs, algebraic identities, empty operands, resource limits, and immutability. Mesh tests must independently compare analytic area/volume where possible and validate closed topology. Exchange tests must include short reads/writes, invalid indices and nonfinite values; fuzz targets must not panic or allocate without bounds.

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and focused fuzz campaigns. Report coverage as supporting evidence, never as proof of geometric correctness. Add benchmark baselines before algorithm optimization. Differential tests against pinned OCCT are required for parity. The initial corpus covers restricted modeling and exchange cases; broader coverage remains open. Compare geometry, topology and invariants rather than unstable entity ordering. Differences need reproducible fixtures and adjudication; OCCT output alone is not an infallible mathematical oracle.

A production release requires supported-platform CI, API compatibility policy, license selection, provenance inventory, numeric envelope documentation, adversarial corpus, exchange security review, and published capability matrix with no unmarked gaps. The full replacement additionally requires completion of every full-target row above. This repository must not advertise that milestone prematurely.

## Additional implemented subsets

The [status matrix](STATUS.md) records the implemented domains separately from
full-target requirements. Package comments define numerical and work budgets.
In particular:

- Planar B-rep faces support strictly contained, disjoint inner wires. Curved
  analytic patches preserve seams and poles and integrate mass properties
  independently of tessellation. Separate UV trims support straight, quadratic and rational spline p-curves
  with inner loops and conforming dyadic refinement in the supported domain.
- Convex planar Booleans have explicit geometric preconditions. Two-ball
  Booleans preserve spherical surfaces. Seeded surface tracing only follows a
  transverse branch in bounded parameter domains.
- Native persistence supports validated planar, parametric and trimmed
  geometry types. Native trimmed versions 4 and 5 store bounded exact homogeneous
  quadratic trim coefficients and independently revalidate affected loop
  topology and model-edge agreement. Version 5 also reconstructs nonbinary
  rational endpoint identity independently of the stored display vertices.
  STEP and IGES report unsupported graph/entity combinations; the
  [pinned reference harness](../tools/occtref/README.md) exercises selected
  cross-kernel exchange cases.
- Document snapshots preserve exact supported assets, stable asset IDs and
  feature recipes. Stable topology naming across regeneration remains open.

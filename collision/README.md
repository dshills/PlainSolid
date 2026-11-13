# Exact surface contact and distance queries

`Triangles`, `MeshIntersections`, and `SelfIntersections` classify intersections
of the represented finite triangle coordinates using rational arithmetic.

`TriangleDistance` and `BoundaryDistance` add global closest-point queries.
`BoundaryDistance` accepts planar `brep.Solid` values and returns both triangle
and original polygon-face ancestry. The BRep tessellation preserves concave
faces and holes. Distance is between **boundaries**, so nested nonintersecting
shells can have positive separation even when their occupied material overlaps.
The query does not certify shell embedding, shell orientation, or material
containment.

Disjoint-triangle candidates comprise vertex-to-face orthogonal projections and
all edge-to-edge interior and endpoint minima. Exact triangle intersection is
checked first. Every candidate's feature membership and squared distance are
rational; exact squared AABB lower bounds prune pairs without tolerance. The
minimum is deterministic in input tessellation order when several witnesses tie.

`SquaredDistance`, `ExactPointA`, and `ExactPointB` are caller-owned rational
values. `Distance` uses a 256-bit square-root evaluation before float64 rounding;
`PointA` and `PointB` are rounded conveniences. Rounded witnesses may depart from
the exact face or coincide at extreme scales. The exact values remain decisive.
A positive distance too small or large for float64 returns `ErrDistanceRange`;
empty boundaries return `ErrEmpty`. Failures return no partial result.

`BoundaryDistance` uses a deterministic balanced BVH on each operand and a
best-first node-pair queue. Splits sort exact doubled AABB centers along the
longest exact extent, with triangle indices resolving ties. Pair priorities and
all pruning bounds are rational. Equal-distance pruning also compares the
smallest possible original triangle pair, preserving the flat traversal's
lexicographically earliest pair and its exact witness selection.

`DistanceOptions` defaults to 100,000 **node-pair lower-bound evaluations** and
4,096 triangles per operand. `Pairs` counts those evaluations, including pruned
children; `NarrowPhasePairs` counts actual triangle-distance predicates. This
changes the former flat-product budget accounting. Tree preparation is bounded
by triangle count; it checks cancellation during bounds scans and between
bounded sorts. Search checks cancellation at every pair. The existing hard cap
of 100,000 triangles remains. Worst-case search is still quadratic; exact BVH
pruning can remove whole Cartesian groups. A test with 1,200 triangles per operand
uses 41 node bounds and one triangle predicate instead of a 1.44-million-pair
flat product. Differential tests preserve exact witnesses against flat search.
Rational expressions have fixed algebraic depth from float64 inputs; no iterative
geometric splitting is performed. These are planar-boundary queries, not
analytic curved-surface distances.

## Material point location

`NewPointLocator` creates an immutable snapshot for repeated `Locate` queries;
`LocatePoint` validates and queries once. Construction validates closed manifold
topology, exact geometric self-intersection/contact, shell containment, and
alternating material orientation (outer shells outward, cavities inward, islands
outward). Unvalidated or incorrectly oriented shell arrangements are rejected.
The zero-value locator and zero-value BRep represent empty material.

Queries distinguish `Inside`, `Outside`, and `OnBoundary` with no tolerance.
Boundary ancestry chooses the first containing tessellation triangle, mapping it
to the original polygon face. Otherwise an exact rational ray computes signed
winding and reports all transverse hits, sorted by parameter, with original
face/triangle ancestry and exact points. Hits on edges or vertices, and rays in
triangle planes, retry deterministic directions `(1,k,k*k)` instead of double
counting ambiguous crossings. Exhausted retries fail explicitly.

Location defaults are 4,096 triangles, 2,000,000 work units per preparation/query,
100,000 collision comparisons during validation, and 64 ray directions (maximum
configurable 1,024). Shell validation can be quadratic in shell count and ray
queries are linear in triangle count per attempt. Cancellation is checked in
preparation and ray loops; the separately bounded self-intersection pass cannot
be interrupted internally. Exact results describe the represented planar
triangles, not any idealized curved or tolerance-expanded boundary.

`NewPointProjector` prepares immutable planar-boundary projection queries;
`ProjectBoundary` is the one-shot form. Face-interior, edge and vertex candidates
use exact rational arithmetic, with exact triangle-AABB lower-bound pruning.
The nearest witness is confined to the triangulated face region, including
concave boundaries and holes. Equal distances choose the lowest triangle index.
Results include original face/triangle ancestry, caller-owned exact witness and
squared distance, and rounded convenience values. Default limits are 4096
triangles and 100000 exact BVH node-bound tests per query; preparation is
separately bounded. An immutable balanced BVH is built once. Best-first queries
use exact lower bounds and original triangle indices for deterministic ties.
`Work` counts evaluated node bounds, including immediately pruned nodes, while
`NarrowPhase` counts actual triangle projections. Equal-distance nodes are only
pruned when they cannot contain an earlier winning triangle.

Optional signed mode prepares `PointLocator`, validating closed embedding and
material orientation under its own limits. It returns location diagnostics and
negative distance inside material. A point inside a cavity is outside material.
Unsigned mode makes no material-location claim. Empty boundaries and exhausted
limits fail explicitly. A prepared projector supports concurrent calls.

`ValidateAssembly` validates several planar BReps as one material boundary,
without coordinate welding, overlap union or orientation repair. It reuses the
existing exact embedding and nesting validator. Disjoint bodies and explicitly
inward cavity shells are accepted; crossing, coincident or touching shells and
incorrect nesting orientation fail. Inputs share one coordinate system.

The report includes the shell nesting tree, exact signed shell/total volumes,
and original solid/face ancestry. `PointLocator.Shells()` exposes caller-owned
shell diagnostics already computed during preparation, with deterministic
triangle indices and nesting depth; it does not rerun predicates. A material
island inside a cavity has even depth and positive orientation. Assembly input
count defaults to 256, and aggregate triangle/geometry limits come from
`LocationOptions`. No partial report is returned on failure. Tests cover reversed
input order, nested voids/islands, disjoint roots, crossings, overlap, face/vertex
contact, wrong orientation, resource limits and report ownership.

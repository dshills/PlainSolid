# Exact planar material and boundary distance

`DistanceRationalHierarchyPlanarMaterials`,
`DistanceOCCTRationalHierarchyPlanarMaterials`, and
`AffinePCurveHierarchy.PlanarDistance` measure authenticated finite planar
materials in exact rational world coordinates. Nonconvex faces, finite holes,
and their original source ownership are included. Existing imports and point,
line, mass and area APIs are unchanged.

This is deliberately a **per-occurrence planar subset**. Cylinder, sphere and
torus leaves return `Supported=false`, an explicit reason and nil distances.
Their general distances can require algebraic values, so no rounded analytic
fallback is used. There is no whole-assembly minimum over the supported subset.
Occurrences are independent even if they overlap.

## Two distances and complete nearest-boundary ownership

`BoundarySquaredDistance` is the squared Euclidean distance to the closed
material boundary. `MaterialSquaredDistance` is the distance to the regularized
closed material: zero inside and on the boundary, equal to boundary distance
outside. `NearestBoundary` always identifies boundary points, including when
the query point is interior. The result does not mislabel those as the nearest
material point of an interior query (which is the query point itself).

The full original source, native embedded outward boundary, occurrence paths
and proper rational frames are re-proved. Every planar face is bound to its
original directed outer/hole loops and native vertices/edges. For each face,
its exact orthogonal projection is a candidate only when it lies in the finite
face, including its boundary but excluding holes. All closed original edge
segments are also projected onto, with exact rational clamping to endpoints.
These candidates are complete: a face-interior minimum must be the orthogonal
projection; otherwise a minimum is on an original segment or its endpoint.
No tessellation, sampled search or floating-point nearest-point decision occurs.

All squared distances are compared exactly. Every distinct tied nearest point
is retained, then independently classified on the proved boundary to recover
**all** incident original face/edge/vertex IDs. Multiple face/edge candidates at
one physical point are merged by exact coordinates, not by tolerance. A hole
center can therefore have four nearest wall points; a point above that hole
can have four nearest rim points, each with both incident faces and its edge.
Nearest points are returned in exact lexicographic world-coordinate order.

Proper rational source occurrence transforms preserve squared Euclidean
distance. Returned points are transformed exactly, never baked through rounded
binary64 coordinates. Distance and point getters return owned rational copies;
source ancestry slices are newly allocated per result. Distances are in the
common source length unit squared; multiply by `LengthUnit²` for m².

## Budgets and independent evidence

Limits bound source parsing/material reproof, rational coefficient sizes,
cumulative finite-face proof tests and ray attempts, candidate projections,
and tied nearest points. `MaxNearestPoints` bounds both temporary tied sets
and the cumulative emitted points. These are work limits, so a too-small limit
can reject before a later, closer candidate replaces a temporary tie. All
candidate and ownership loops check cancellation; failure is atomic.

The pinned standalone OCCT comparator transfers the original hierarchy and
uses `BRepExtrema_DistShapeShape` against each solid's face compound for boundary
distance, plus independent solid classification for material distance. It
reports all nearest points, merging only oracle duplicates under its explicit
comparison tolerance. The pure Go proof never uses that tolerance.

Both frame conventions are checked on the original reentrant hierarchy and
the genuine Boolean-cut affine-pcurve holed prism, the latter after native
save/reload. Tests cover face, edge and vertex minima; interior zero material
distance; reentrant-notch and hole ties; projections excluded by holes;
oblique world coordinates; every nearest source owner; nil analytic distances;
source tamper; cancellation; coefficient/proof/candidate/tie limits; and owned
rational outputs. Original OCCT boundary/material distances and all nearest
world points agree for every retained sample.

```
sh tools/occtref/build_step_rational_hierarchy_distance.sh \
  /path/to/occt/include /path/to/occt/lib /tmp/plainsolid-rational-hierarchy-distance
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE=/tmp/plainsolid-rational-hierarchy-distance \
  go test -race ./exchange/step -run TestHierarchyPlanarDistance
```

Qualified OCCT source pin: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

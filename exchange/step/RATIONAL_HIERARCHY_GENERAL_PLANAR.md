# Exact nonconvex planar hierarchy point mode

`LocateGeneralRationalHierarchyPoint` and
`LocateGeneralOCCTRationalHierarchyPoint` add nonconvex/holed planar material to
the separately proved rational occurrence conventions. Existing analytic-only
and convex-only APIs retain their previous meaning. Line clipping is unchanged.

The complete source hierarchy, product geometry, closed manifold and exact
embedded native boundary are reproved first. Every native directed outer/hole
loop is matched cyclically to exactly one original source loop, with exact source
decimal vertex equality, edge ownership and original plane support/sense. Signed
projected areas authenticate outer versus hole winding. No rounded normal,
triangle approximation or convex hull substitutes for the original face region.

A point on a supporting plane is on the material boundary only when exact finite
polygon winding (including opposite hole loops) admits it. Original incident
face IDs are filtered by this finite membership; exact finite segment and vertex
tests retain edge/vertex IDs. Infinite supporting-plane coincidence alone is not
a boundary claim. Holes are tested by winding cancellation, not bounding boxes.

Otherwise the query tries rational directions `(1,k,k²)`. A direction is accepted
only if it is nonparallel to every face plane and every forward intersection
strictly inside a finite face avoids all loop edges/vertices. Exact winding counts
these transverse crossings; odd parity means inside. A degenerate ray is discarded
and contributes no classification. The result never falls back to floating
jitter. Exhausting the explicit ray/work limits returns no partial result.

MaxRayDirections defaults to64 and is capped at4096, cumulatively across all
occurrences. MaxPlanarTests, source limits and coefficient-bit limits remain
independent gates. RayDirections reports the successful query's cumulative
attempt count. Each original local planar proof is built once and reused across
its occurrences, with all rational coordinates privately owned.

Independent source: pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 authors
an L-shaped reentrant prism with integer original planes/lines and full pcurves
via `step_reentrant_planar.cxx`. The authoring script inserts its complete owned
geometry closure into each retained AP214 hierarchy without altering other local
bodies, product IDs or occurrence relationships. Both original files pass normal
OCCT transfer, and native source persistence/reload followed by exact queries
matches OCCT at inside, notch-outside, concave edge, corner and exterior points.
The live test uses PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION.

A forced edge-crossing first ray proves retry/failure semantics. A separate exact
planar prism with a rectangular cap hole tests finite hole exclusion, cap/hole
wall identity and rejection of a reversed original inner loop. This latter
helper regression is not claimed as an independently authored OCCT fixture.

Broader nonconvex line clipping remains separate work: construct all finite
plane/edge event stations and coplanar intervals, then classify intervening open
intervals with this proof. No such line acceptance is added here.

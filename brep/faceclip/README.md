# Exact planar face-region clipping

`Clip` retains one selected BRep face in the closed halfspace
`Normal dot point <= Offset`. Finite binary64 coefficients are interpreted
exactly without normalizing the cutting plane. The selected represented face
must be exactly planar. Its conforming tessellation preserves holes and concave
regions; each triangle is clipped with rational arithmetic and retains original
face/triangle ancestry. Result fragments are not merged into trim loops and do
not form a closed solid. Adjacent fragments share boundaries without overlapping
area. Zero-area contacts are omitted; a coplanar cutting plane retains the face.

`ProjectedArea` is exact area after dropping the dominant normal axis, reported
as `Axis`. The area centroid is an exact rational 3D point, with nil coordinates
for an empty result. `Area` is a rounded Euclidean convenience computed from the
exact projected area and normal ratio; underflow/overflow fails explicitly.
All returned rational coordinates and metrics are caller-owned. No output float
vertex conversion, welding, or approximation of curved geometry occurs.

Default limits are 8192 total input tessellation triangles, 100000 predicate/work
steps, and 16384 rational bits. Context is checked throughout clipping. Tessellation
is the immutable BRep's precomputed validated mesh; exact planarity is checked
again because the BRep constructor allows a planarity tolerance. Tests cover
holes, rational clipping centroids, empty/tangent cases, sheared support planes,
complementary-area invariants, ancestry, cancellation and exhausted budgets.

`MergeLoops` optionally promotes an unmodified clipping result into exact open
face-region boundaries. It aligns every fragment edge at all shared rational
vertices, cancels opposing segments, and traverses the remaining directed graph.
Outer loops, holes and separate islands are returned with exact coordinates;
holes name their containing outer loop. Winding follows the original face and
collinear vertices remain to preserve segmentation. Internal tessellation edges
are removed without rounding. The merged signed projected area must equal the
clipped fragment area exactly.

Branching/point-contact boundaries and duplicate directed edges return
`ErrNonmanifold`. The input must be an unmodified `Clip` result; this is not an
arbitrary overlapping polygon arrangement solver. Fragment ancestry remains in
the original result. Defaults are 8192 unique vertices, 2000000 work steps and
16384 rational bits. Edge alignment compares all fragment edges with all unique
vertices; loop containment is also bounded. Tests include retained holes,
cut-open holes, multiple disconnected islands, duplicates and resource limits.

`ClipRegion` intersects a face with an ordered collection of halfspaces, forming
an arbitrary convex clipping window rather than only an axis-aligned box.
Rational fragment coordinates pass directly between stages without float
conversion. Every surviving fragment retains its source triangle; projected
area and spatial centroid are recomputed exactly. `MergeLoops` accepts the final
result and reconstructs holes or disconnected trim components. An empty
halfspace list returns the complete face; inconsistent halfspaces return an
empty region. All halfspaces are validated even if an earlier stage is empty.

`RegionOptions.MaxHalfspaces` defaults to 32 with hard maximum 128. Initial
clipping uses `Clip` limits; subsequent stages share a separate cumulative
`Clip.MaxWork` budget and the same rational bit cap. Tests verify an inset window
retains an enclosed hole, exact repeated nonbinary intersections, reversed cut
order metrics, full/empty regions and invalid/over-budget windows.

`ValidateBoundary` independently checks rational open-region loops before later
sheet conversion or export. It proves common 3D planarity, nonzero simple loops,
absence of segment crossings/overlaps and inter-loop touching, correct nesting
and parent indices, consistent winding, and exact projected-area consistency.
It detects intersections even when no shared graph vertex exists. Forward
collinear segmentation is allowed; backtracking is rejected. It uses the same
`LoopOptions` defaults with its own work budget and checks all segment pairs,
so worst-case cost is quadratic. The validator returns explicit errors, not a
best-effort certificate. It does not create rounded geometry or repair invalid
loops. Tests include valid merged holes and independently corrupted area,
planarity, nesting, winding, crossing and backtracking fixtures.

`TriangulateSheet` converts a validated rational boundary to an open indexed
triangle mesh. `Sheet.ExactVertices` aligns with mesh vertices; `LoopVertices`
preserves every original loop vertex, including collinear segmentation. No
geometric vertices are added. The implementation temporarily simplifies
collinear vertices for the existing holed-polygon triangulator, extracts its
planar cap, then exactly splits boundary triangle edges to restore every source
vertex. Disconnected outer loops are triangulated separately.

Conversion defaults to maximum Euclidean displacement `1e-9`. Vertices must
remain distinct and exactly on the original rational supporting plane; local
turn signs, collinearity, hole nesting and validated disjoint boundary topology
must survive conversion. A horizontal plane at exact height `1/3`, for example,
is rejected because no binary64 point lies exactly at that height. This is a
conservative incidence guarantee, not a promise of zero coordinate displacement.
Rational planar windows at representable heights may convert successfully while
retaining the authoritative exact coordinates alongside the rounded mesh.

Final triangle orientations, boundary/internal edge incidence and exact rounded
projected area are verified. The mesh is an open sheet, not a solid. Defaults
allow 16384 triangles; boundary validation and conversion each use separate
`LoopOptions` work budgets. Existing polygon-constructor numeric restrictions
can yield explicit `ErrConversion` on otherwise valid geometry. No best-effort
repair or hidden tolerance welding occurs.

`ExtrudeRegion` converts a validated region into a closed material prism using a
finite translation, including oblique offsets. It requires exact binary64
representability of every input rational vertex and every translated endpoint;
there is no permitted coordinate displacement, even though sheet conversion
alone permits a bounded displacement. Tangential offsets are invalid. Both
positive and negative normal directions receive outward material orientation.

The solid retains holes, disconnected outer components and all original
collinear boundary vertices. Every face is triangular. `FaceSources` identifies
start/end cap sheet triangles or original loop/segment wall ancestry; the
returned `Sheet` retains exact source coordinates and loop maps. `ExactVolume`
is computed rationally from projected area and normal translation. The output
passes exact collision/material-orientation validation through `PointLocator`.
Sheet limits also cap total extrusion triangles; `ExtrusionOptions.Validation`
provides separate solid-validation limits. Defaults in that validator can reject
a solid before the larger triangle-construction cap is reached. No rounding,
healing or merging is performed to force acceptance.

`ClipRegionWithEvents` adds original-wire split provenance to exact region
clipping. `Edges` records every outer/hole segment's surviving normalized
interval and cutting supports coincident with the entire segment. `Events`
records strictly interior support crossings, grouping coincident parameters and
listing their cutting-halfspace indices. `Retained` distinguishes crossings
inside the complete clipping window from those excluded by another constraint.
Endpoints at zero/one are not new split events; point-only interval contacts have
equal endpoints, while wholly excluded segments have nil interval endpoints.

Loop 0 is the outer wire and subsequent loop indices correspond to source holes.
Parameters always follow oriented loop traversal, independent of the global
edge's stored direction; `Edge` and `Reversed` preserve that mapping explicitly.
Only original face-wire edges are reported, never tessellation diagonals. Event
and clipping passes have separate cumulative work budgets; collection charges
edge/halfspace comparisons, event generation, sorting and retained-point checks.
All coordinates/parameters are exact caller-owned rationals. Tests cover holes,
reversed winding, coincident cuts, rational one-third witnesses, culled events,
complete/empty intervals and exhausted limits.

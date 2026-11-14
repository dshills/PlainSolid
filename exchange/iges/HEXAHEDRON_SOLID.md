# Convex affine hexahedron IGES solids

The separate `ReadHexahedronSolid` / `WriteHexahedronSolid` APIs extend explicit
IGES186 topology to six quadrilateral faces, eight vertices and twelve shared
edges. The earlier tetrahedron APIs and acceptance contract remain unchanged.

The reader retains source topology IDs, all 42 source geometry entities and
labels/subscripts. Each face owns one full affine bilinear type128 chart and
four independently owned polynomial degree-one type126 pcurves. Twelve separate
type126 model curves are shared through a type504 edge list; type508 loops,
type510 faces and one outward type514 shell complete the type186 material graph.
Every input entity must belong to these exact roles, without aliasing support or
owner curve records. There are no ignored detached entities or properties.

Every chart uses its four represented corner poles over [0,1]² and satisfies
P00+P11=P01+P10 as an exact rational identity. Each owner pcurve follows the
same directed shared edge from one exact UV corner to the next. Both curves are
linear with the same parameterization; affine composition and endpoint equality
therefore prove whole-edge agreement. The four UV edges must form the positive
unit-square boundary. Source weights, knots, coefficient decimals and metadata
pass the same strict gates as the tetrahedron dialect; no rounding is accepted.

All eight vertices must be distinct. Every model edge has a unique unordered
vertex pair; the edge graph must be connected with degree three at every vertex.
All six four-vertex face sets must be distinct. For every face, all four other
vertices must lie strictly in its inward half-space, proved with exact rational
orientation determinants. These supporting planes and complete face polygons
prove convex global boundary embedding. Native BRep validation additionally
requires the closed manifold's paired, opposite edge uses and vertex links.
Disconnected, pinched, coincident, inward, concave or nonaffine shells fail.

The canonical writer takes eight authored model positions, indexed by binary
U,V,W: 0=origin, 1=U, 2=V, 3=U+V, 4=W, 5=U+W, 6=V+W, 7=U+V+W. These labels
are incidence roles, not instructions to reconstruct or snap a vertex. All eight
positions and six affine face identities must independently pass the reader.
Signed determinant selects outward orientation, including reflections. Output
is staged and re-imported before I/O; malformed input produces no output.
The writer creates fresh canonical metadata, not a replay of original headers.

The hard envelope is 58 entities, 96 control points, 4096 values, 1024 records and
64KiB, further reduced by caller limits. Arbitrary non-parallelogram quad faces,
curved surfaces, holes, void shells, rational nonuniform curves, partial domains,
transforms, tolerance repair and unsupported directory metadata are excluded.
This is a bounded convex polyhedron dialect, not general IGES solid interchange.

## Independent reflected oblique fixture

`tools/occtref/author_iges_hexahedron.py` independently writes the retained source
without the Go writer. Its origin is (3,4,5), with U=(-2,-1,0), V=(0,3,1),
W=(1,0,2): a reflected, sheared model with signed determinant -13.
Both this source and Go output transfer through ordinary IGES TransferRoots and
BRepCheck using pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`:

- One solid, six faces.
- Volume 13.
- Area 2(√41+√46+√21) = 35.536059831027913.
- Centroid (2.5,5,6.5).

Set `PLAINSOLID_OCCT_IGES_SOLID` to the existing `iges_topology_solid.cxx` helper
for both-direction tests. Native tests also cover positive orientation, original
geometry counts, cross-dialect rejection, changed supports/vertices/model and UV
ownership, shell/coedge sense, malformed source type and small budgets. Warped or
pinched writer inputs fail atomically. A bounded direct-reader fuzz target and
focused race, Go1.23 and vet checks supplement the independent material reference.

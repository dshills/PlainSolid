# Exact conical stock with cylindrical through aperture

`ReadConical45CylinderThroughSolid` / `WriteConical45CylinderThroughSolid`
authenticate a separate canonical 28-entity IGES type186 graph. The stock is a
positive-Z expanding 45-degree type194 frustum, and its concentric type192 bore
has constant positive radius strictly below the stock's lower radius. Both
axial ends are open annular caps. This is one connected four-face type514 shell,
not a negative detached void shell. Existing cylinder, cone and cavity readers
are unchanged.

The directed graph contains four exact full type100 circles, two type110 seam
lines, one four-vertex type502 list and six model edges in type504. Each wall
has two opposite uses of its seam; each cap has an outer and oppositely directed
inner loop. Both walls use model-only boundaries: no rounded angle or slant
pcurves are invented. Every source scalar, reference, direction and directory
ownership is checked against the independently recovered dimensions. Source
numbers must survive the existing exact decimal-to-binary64 gate; endpoint
sums must be lossless. Alternate ordering, transforms, rounded vendor graphs,
non-45-degree slopes, touching radii and partial openings are unsupported.

The neutral native builder constructs exact rational quarter-circle controls
with linear outer radius interpolation and a constant inner radius. Strict
endpoint clearance proves clearance at every station. Its two annular caps,
four outer and four reversed inner wall patches are sewn with continuous exact
edge agreement. Source face IDs and native quarter-rim IDs remain separate.
The source-bound JSON envelope reconstructs the native solid and verifies the
persisted payload, rather than trusting serialized shape claims. Source export
also rejects altered descriptor, topology ancestry, metadata or native shape.

## Independent qualification

`tools/occtref/author_iges_conical45_cylinder_through.py` constructs the IGES
records independently of Go. Its fixture has lower stock radius3, upper7,
height4, bore radius1 and origin0. Analytic values are volume304π/3,
areaπ(64+40√2), centroid(0,0,48/19). The pinned OCCT commit
4dbfdcc448c0b3e565488afcd626c84483172db9 transfers both that original file and
Go output as one valid four-face material solid, matching those measures.
Use `PLAINSOLID_OCCT_IGES_SOLID=/tmp/plainsolid-iges-topology-solid-oracle` with
the comparator built from `tools/occtref/iges_topology_solid.cxx`.

Focused tests cover full native JSON reload, exact continuous edges, closed
mesh, source-bound reload/export, cap/wall/edge ownership mutations, incorrect
void-like face orientation, invalid clearance, stale ancestry, parser limits
and forged native payload. Focused race, Go1.23, vet, existing cone/annulus and
STEP ellipse regressions, and 100 bounded fuzz executions pass. This is a
qualified original analytic material family, not a generic IGES BRep parser.

## Proper signed-axis placement

The separate `ReadPlacedConical45CylinderThroughMaterial` and
`WritePlacedConical45CylinderThrough` APIs admit one shared type124 proper
signed-coordinate rigid transform. It attaches to both original surface types,
both planes, four circles and two seam lines. The type502 vertices are already
world coordinates; source point/direction records remain local support data.
Every attachment and exact inverse-mapped vertex is checked before the original
canonical proof runs. A missing attachment, changed bore axis, general affine
matrix, reflection or nonrepresentable coordinate sum is rejected. This does
not enable record reordering or change the unplaced reader.

World native quarter controls are rebuilt using exact rational placement
arithmetic, never rounded transformed floats. Both cap and all four rim roles
are retained: original circles are [outer lower23/upper19, inner lower29/upper25],
with separate native quarter edge IDs. Source/native JSON reproof authenticates
that ancestry and the original placement. All continuous edge discrepancies
remain zero after placement and native reload.

The independent author's `--placed` fixture uses (x,y,z)→(10+z,−3+y,5−x).
Pinned OCCT accepts its original and Go output as valid four-face one-solid
material with unchanged area/volume and centroid (10+48/19,−3,5). A second
proper signed frame reverses the axial direction. Focused tests also reject
wrong bore axis, detached bore/cap/rim placement, forged pose/ancestry and
source limits. Live OCCT, race, Go1.23, vet and 100 fuzz executions qualify
this separate placement scope.

## Reordered aperture topology

`ReadReindexedConical45CylinderThroughMaterial` accepts the independently
qualified placed/unplaced family with arbitrary DE order/addresses, vertex-list
permutations, six-edge list permutations, four-face shell permutations and
cyclic starts of each four-use wall loop. Exact local support dimensions locate
the four distinct source vertices; directed endpoint incidences identify the
four circular and two line edges. Support kind plus shell sense distinguishes
outward cone, inward cylinder and the two caps. A bounded bijection then
reconstructs and rechecks the complete original canonical graph. No face or
edge role comes from sorted record IDs.

Annular face outer-first loop ordering is retained: exchanging outer and hole
lists is not a harmless permutation in this dialect. Reversing one wall use,
flipping the inner shell sense, cross-owner rewiring, ambiguous vertices,
unsupported transforms and missing/detached records fail closed. A source-only
writer and versioned JSON reproof preserve original IDs and list/use ordinals,
including the native face mapping and all four source circle IDs. The positive
finite tolerance argument applies to the placed rebuild; the unplaced typed
codec retains its original fixed native tolerance of 1e−9.

The independent Python reordering tool reverses DE, vertex, edge and face
lists and rotates each wall loop of both retained source files. Pinned OCCT
transfers these original reordered files and unchanged source re-exports as
one valid four-face solid with the same analytic measures and posed centroid.
Exact native continuous joins, prior canonical rejection, reversed inner sense,
wrong cap order, reversed use, cross-ownership, stale ancestry and limits are
covered by tests. Focused live/race, Go1.23, vet and 100 fuzz executions pass.

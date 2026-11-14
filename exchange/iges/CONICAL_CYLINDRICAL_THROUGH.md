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

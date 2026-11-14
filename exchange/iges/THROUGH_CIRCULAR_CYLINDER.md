# Exact analytic IGES through-bore material

`ReadThroughCircularCylinderSolid` and `WriteThroughCircularCylinderSolid`
provide a distinct strict type186 family: a complete positive-Z circular stock
with a smaller coaxial circular through opening. The original outer and inner
walls share their full circle rims with two annular caps. There is no blind
floor or detached inner shell. Source face order is outer wall, inner wall,
upper annulus, lower annulus; both wall supports point outward radially, but the
shell uses the inner wall with reversed material orientation.

The canonical36-record graph contains two192 cylinder supports, two190 planes,
four100 full circles, two110 model seams, four502 vertices and six504 edges.
Each cap owns an outer and a reversed inner508 loop. The lower cap has reversed
shell orientation. Both wall loops include the two uses of their own seam and
retain exact0–360 degree pcurve domains over the physical height. Every source
value, integer reference, ownership and orientation is re-proved against the
recovered dimensions and origin. Source decimal and derived arithmetic must be
lossless; no model circles or seam vertices are snapped.

The independently constructed native `FiniteCylinderPairDifference` result
retains native stock/cutter ancestry, while the returned parameter/metadata maps
retain the original source graph. Native quarter arcs need not equal the source
complete circle edge decomposition. The fixed1e-9 native construction clearance
is an additional rejection gate, never an exchange repair tolerance. A returned
material has strict positive inner radius, height, and stock wall thickness.

Input is capped at64KiB,1024records,36entities,4096values. Malformed metadata,
placements, unowned graph data and alternate topology are rejected. Reader and
writer failures publish no partial material/output. The standalone cylinder,
blind-bore and mixed-collection scopes remain unchanged.

## Independent evidence

`author_iges_through_cylinder.py` independently specifies the exact source graph
for outer radius3,inner radius1,height4. `author_iges_vendor_through_cylinder.cxx`
independently performs an OCCT cylinder Boolean cut from z=-1 through z=5 and
exports its unmodified47-entity source. The latter contains rounded seam points
and is explicitly rejected by the exact reader, not repaired.

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 ordinary TransferRoots and
BRepCheck_Analyzer accept the independently authored exact source, Go output,
and native OCCT Boolean source: one solid,four faces,volume32*pi,area48*pi,
centroid(0,0,2). Enable `TestOCCTThroughCircularCylinder` via
`PLAINSOLID_OCCT_IGES_SOLID`. Native analytic measures independently agree.
Tests include translated origin/unit roundtrip, invalid cap/hole ownership,
inner-wall sense, support/radius and degree-domain mutations, strict blind versus
through rejection, bounds, atomic failures, cancellation and bounded fuzz.

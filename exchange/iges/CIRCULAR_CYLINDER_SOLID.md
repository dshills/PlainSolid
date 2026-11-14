# Exact circular IGES186 cylinder material

`ReadCircularCylinderSolid` and `WriteCircularCylinderSolid` implement one
explicit circular material family, independently of the spline collections:
a complete finite cylinder along positive global Z, with a positive radius,
positive height and exactly represented translation. The fixed meridian is
positive X. This is a real type186 solid, not a type154 primitive declaration
or a standalone cylindrical support.

The canonical 24-entity graph has a type192 cylindrical support, two type190
planes, two complete type100 circular rims, one type110 model seam, two
vertices and three indexed edges. The cylindrical face has four directed uses:
upper rim reversed, seam reversed, lower rim forward, seam forward. Its four
independent type110 pcurves bound `[0,360]×[0,height]`. The upper cap uses its
rim positively; the lower cap's shell orientation is reversed. Each cap's planar
loop omits redundant pcurves but retains its authoritative model circle.

## Complete chart and material proof

IGES192 chart U is in **degrees**. OCCT's
`IGESToBRep_TopoSurface::ParamSurface` cylindrical branch explicitly applies
`uscale = M_PI/180`. Thus the authored integer interval 0–360 is one full turn;
it does not introduce a rounded radian endpoint. Native geometry uses radians,
with the explicit mathematical relation `theta = pi*U/180`.

The two horizontal chart edges lift to the same complete circles as their
model edges; each vertical chart edge lifts to the same model generator.
Opposite U boundaries are the one physical seam. All source origins, directions,
radii, endpoints, references, senses, shell flags and full-domain pcurve
coordinates must match the recovered descriptor exactly. The finite angular
coverage, cap ownership and material orientation are therefore proved from the
whole graph, not inferred from incidence alone or samples. Positive radius and
height make the cylindrical side and two disks an embedded outward closed
boundary. The equivalent native `parametric.Cylinder` is constructed only after
this complete source check.

Raw source numbers must pass the bounded exact-decimal gate. Required coordinate
sums and height subtraction use exact rational arithmetic and fail if their
binary64 result is not representable. Integer topology/reference fields require
integer lexical tokens, not real-valued tokens such as `1.`. That distinction
matters to external IGES parsers even when numerical values compare equal.
No approximate spline, repair or angle snapping occurs. Original numeric
parameters, directory labels/subscripts and canonical source IDs are returned.

The current dialect requires canonical directory order and fixed +Z/+X frame.
Arbitrary frame rotations, type124 placement, alternate topology, partial arcs,
additional roots, voids and optional extra parameter properties remain outside
scope. Caps are 32 KiB, 512 records, 24 entities and 2048 values; caller limits
may tighten them. There are no B-spline control nets. Writes stage a complete
typed readback before output; errors return no partial decoded solid.

## Independent reference and limits

`author_iges_circular_cylinder.py` independently authors the exact graph for
radius 2, height 3. Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots/BRepCheck accepts
this source and Go output as one solid with three faces. Expected volume is
12π, area 20π and centroid (0,0,1.5). A second Go output translated by
(.25,-2,3) independently verifies support/circle/floor placement and centroid.
Numerical oracle allowances do not participate in source acceptance.

`author_iges_vendor_cylinder.cxx` separately constructs the same primitive with
OCCT's native IGES B-rep writer. Its retained 30-entity source uses a transformed
upper circle and seam vertex Y=-4.898587197E-16. It is valid in OCCT's tolerance
model but is not this exact graph; the Go reader rejects it unchanged. It is not
silently normalized or described as an accepted independent inbound fixture.
The independent exact source is `circular-cylinder-solid.igs`; negative vendor
source is `occt-rounded-circular-cylinder.igs`. Optional live tests use
`PLAINSOLID_OCCT_IGES_SOLID` and the existing topology material oracle.

Negative tests change circle centers/endpoints, radius, seam vertices/line,
angular period, edge ownership, cap senses and root orientation; integer lexical
and byte/entity/value limits, cancellation and writer representability failures
are checked. Existing IGES support and spline-solid APIs remain unchanged.

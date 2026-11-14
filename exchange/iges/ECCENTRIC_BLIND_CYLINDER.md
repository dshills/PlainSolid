# Exact canonical eccentric blind-bore material

`ReadEccentricBlindCylinderSolid` and `WriteEccentricBlindCylinderSolid` add a
separate 39-record type-186 normal form. It contains a positive-Z stock cylinder
with a strictly contained displaced parallel blind opening from its upper cap.
`BoreOffset` is the nonzero XY displacement from `Origin`; `Floor` is the
height above the original stock bottom, **not** the cutting depth.

Exact rational arithmetic proves the radial clearance
`dx²+dy² < (Radius-BoreRadius)²`. Independent axial gates require
`0 < Floor < Height`: the floor stays strictly above the stock bottom and the
cut has positive depth. All source numbers must be losslessly binary64
representable. Original-center subtraction, cap/floor height addition, radius
endpoint addition and `Height-Floor` must be lossless; no source tolerance repair
or rounded clearance comparison occurs.

The complete source graph retains two independently centered type-192 walls,
three type-190 planes, six type-100/type-110 model edges, original type-502
vertices and type-504 edge list, and ordered type-508/type-510/type-514 topology.
The opening cap has the outer loop and eccentric hole; the original bottom is a
full disk; the floor is its own disk at the bore center and declared height.
Every source scalar/reference/sense is checked against the complete descriptor
normal form. Cylindrical pcurves use the full exact 0–360 degree period and
separate stock-height/bore-depth intervals. Planar caps use their model circles,
without inventing owner pcurves.

The shell's inner wall is reversed relative to its support. The floor has +Z
material orientation: it is the reversal of the cutter's outward lower cap,
not a second stock bottom. The native finite-cylinder difference retains
stock/cutter wall and cap ancestry, with one nonempty component and an empty
second slot. `SourceFaceIDs` orders original outer wall, inner wall, opening,
bottom and floor. Original labels/subscripts and directory parameters remain
available. Native sewing can conservatively reject unresolved small clearances;
this codec does not change its modeling guarantees.

This is a bounded exact normal form, not a general vendor IGES importer. It does
not admit zero-offset coaxial sources, through cuts, closed cavities, placement
transforms, arbitrary entity reordering or approximate circles. Those existing
APIs retain their contracts. The writer stages and revalidates all geometry
before emitting bytes. Source limits are 65536 bytes, 39 entities, 1024 records
and 4096 values, intersected with stricter caller limits; canceled and invalid
inputs return no partial material.

## Independent source and material qualification

`tools/occtref/author_iges_eccentric_blind_cylinder.py` independently specifies
`testdata/eccentric-blind-cylinder.igs`, not Go writer output. Stock R3/H4,
bore R1, offset `(1,1/2)`, and floor 2 give volume `34π`, area `46π`, centroid
`(-1/17,-1/34,33/17)`. The volume is stock `36π` minus cut `2π`; its first
moments independently account for the eccentric and raised removed cylinder.

A separate OCCT Boolean author retains
`testdata/occt-rounded-eccentric-blind-cylinder.igs`. Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9` transfers and validates the hand-authored
normal form, Go output and original Boolean file as one solid/five faces, with
volume ≈106.814150222053, area ≈144.513262065131 and the stated centroid.
The 53-entity rounded vendor file remains a **negative** input to this exact
reader; its validity in OCCT is not promoted into source equivalence here.

Native save/reload is byte-stable for translated, signed-offset and three floor
height fixtures. Actual native floor surface evaluations and oriented normals
confirm the declared floor and +Z cavity orientation; independent mass formulas
agree after reload. Numerical integration checks do not replace the exact
source gates or claim exact quadrature.

Mutations cover reversed floor shell/loop orientation, wrong owner/radius/model
vertex, incomplete angular coverage and source decimal rounding. Separate
adversaries exercise radial tangency, the rounded-hypot trap `(2,2^-50)`, both
axial boundary contacts, out-of-stock floors, nonrepresentable sums and atomic
writer failure. Blind/through/coaxial APIs cross-reject. Normal, race, Go 1.23,
vet and fixed-work `FuzzEccentricBlindCylinder -fuzztime=100x` qualify the codec;
optional live tests use `PLAINSOLID_OCCT_IGES_SOLID`.

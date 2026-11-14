# Exact elliptical annular prism

`ReadEllipticalAnnularPrism` and `WriteEllipticalAnnularPrismSTEP` exchange one
closed concentric, homothetic elliptical through-hole material. Outer radii
`a>b>0`, inner radii `c>d>0`, `c<a`, `d<b` and exact `a*d=b*c` prove strict
nesting throughout the profile. Positive height extrudes this annulus along
its exact rational normal. The original frame can be tilted; rational dot and
cross products prove a proper orthonormal basis. No tolerance establishes
containment or matching support geometry.

The original model-only STEP graph has 118 entities and four faces: two exact
ellipse extrusion walls plus two planar annular caps. Six model edges are
owned explicitly (two seams and four closed ellipse rims). Each seam has
opposite uses on its wall; inner walls and cap-hole uses preserve material
orientation. Complete graph comparison proves all original points, directions,
radii, owners and incidence after numeric role discovery. Record IDs and order
are not geometry. Unsupported PCURVE wrappers, partial trims, alternate loop
orders and foreign roots fail rather than being ignored. The unit context is
the existing strict millimeter envelope.

Native geometry uses exact homogeneous rational supports: two bilinear caps
with one rational quadratic inner loop each, four outer and four inward wall
charts. Original full rims are partitioned into rational quarters with weights
(1,1,2). All 16 native rational vertices and original homogeneous supports
survive ordinary trimmed JSON reload. Complete shared-edge identities have
zero squared discrepancy. Exact homothety and the rigid straight sweep supply
the family-specific global embedding proof; sewing alone does not do that.
Native face/source ancestry and six original source-edge IDs remain available.

The writer copies rational origin values and interprets binary64 direction
ratios exactly. Their normalization must be rational; all required normalized
axes/model points must have bounded terminating STEP decimal encodings.
Dimensions remain lossless binary64. A failure produces no output. Existing
disk/prism readers remain strict and reject this new topology.

Independent OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 builds an annular face
and extrudes it: origin (0.1,2,3), axis ratio (0,3,4), reference (1,0,0), outer
radii 5/2, inner 2.5/1, height 5. Its standard geometry transfer is retained in
`testdata/occt-elliptical-annular-prism.step`; a typed OCCT model rewrite strips
pcurve wrappers without changing underlying analytic model geometry. Source
and generated output both transfer conventionally as one valid solid, four
faces, twelve edge uses: volume 117.8097245096166 (37.5π), area 219.722234271338,
center (0.1,3.5,5). These independent measurements qualify the selected family,
not arbitrary IGES/STEP solids or vendor-rounded trimming.

Optional live test uses `PLAINSOLID_OCCT_ELLIPTICAL_ANNULAR_PRISM` pointing to
the independently compiled `step_elliptical_annular_prism.cxx`. Focused normal,
race, Go 1.23, vet and a 100-case bounded decoder fuzz campaign qualify the
milestone. Fuzz bounds are 64 KiB, 118 entities, 4096 values and depth 16.

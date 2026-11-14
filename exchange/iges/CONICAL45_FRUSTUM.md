# Exact full 45 degree conical frustum material

`ReadConical45FrustumSolid` / `WriteConical45FrustumSolid` implement a separate
20-record IGES type-186 normal form. The full cone expands along positive Z,
with positive base `Radius` and positive axial `Height`; top radius is exactly
`Radius+Height`. `Origin` is the original bottom reference-circle center and
the meridian reference is positive X. All source scalars and constructed rim,
cap and meridian sums must be losslessly binary64-representable.

The source type-194 form-1 surface declares **exactly 45 degrees**. This is the
symbolic relation `radial increase = axial increase`, not a rounded radian
angle passed through `atan`/`tan`. Both circular rims and the connecting LINE
therefore lie on the original positive cone branch over the full axial interval.
The native `parametric.Revolve` uses the original linear meridian polygon, with
planar caps and the same slope-one conical wall. No generic cone approximation
or negative/apex branch is admitted.

Type-508 loops use model-only edge references, which IGES permits; no slant
pcurves are invented. The cone's natural generatrix distance and native axial
meridian parameter are different: at native meridian `t∈[0,1]`, source signed
slant distance is symbolically `sqrt(2)*Height*t`. No floating number is claimed
to encode that exact irrational endpoint. Source edges retain their exact model
coordinates. Complete type-100 circle rims, shared vertex/edge ownership,
ordered directed loop incidence, cap senses, shell orientation, type-190/194
supports and every canonical reference are validated together. Only this exact
normal form receives material reconstruction.

The result preserves original source solid/shell/vertex/edge IDs, the three face
and support IDs, metadata and scalar parameters. Native face ancestry is
explicit: lower cap, wall, upper cap. Native reload retains its original
revolution charts. The writer stages and validates the whole graph before output.
The private topology parser adds type194 only under this new material mode;
existing cylinder, standalone support and mixed collection gates are unchanged.
Limits are 32768 bytes, 20 entities, 512 records and 2048 values, intersected
with stricter caller limits; canceled or malformed input returns no partial solid.

## Independent qualification and limits

`author_iges_conical45_frustum.py` independently specifies the retained standard
IGES source with base R1, top R3 and H2. It is not Go output. A separate C++
author uses OCCT `BRepPrimAPI_MakeCone(1,3,2)` and retains its unmodified
30-entity vendor graph as a negative exact-input case. The new codec does not
silently normalize that different rounded source representation.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` independently transfers and
validates the hand-authored source, Go output, and OCCT Boolean-free primitive
export: one solid/three faces, volume `26π/3` ≈27.2271363311115, area
`π(10+8√2)` ≈66.9589900411649, centroid `(0,0,17/13)`. Native numerical mass
checks match independent analytic formulas before and after byte-stable reload,
including a translated noninteger-height fixture; they are not exact quadrature
certificates. The native wall's midpoint is checked against its exact linear
source meridian to guard against cone-angle drift.

Tests reject altered degree angle (including decimals rounding toward 45), radius,
reference direction, cap/rim/LINE coordinates, loop/face/shell senses, ownership,
limits and nonrepresentable sums. Prior solid and standalone APIs cross-reject.
Normal, race, Go 1.23, vet and fixed-work `FuzzConical45Frustum -fuzztime=100x`
qualify this narrow family. Optional live tests use `PLAINSOLID_OCCT_IGES_SOLID`.

A quarter-cylinder sector was investigated first, but the existing native sector
constructor derives its ending radial plane from floating `cos(pi/2)`. That
cannot presently preserve an exact cardinal 90-degree IGES plane. This milestone
avoids that mismatch rather than snapping the source or claiming partial-sector
support; a separate native cardinal/symbolic boundary constructor remains needed.

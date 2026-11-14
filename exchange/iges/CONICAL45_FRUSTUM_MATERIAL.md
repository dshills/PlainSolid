# Exact native material promotion of IGES194 frustum

`ReadConical45FrustumMaterial` promotes the independently authenticated
`ReadConical45FrustumSolid` source graph to a sewn `trimmed.Solid`. The existing
analytic20-record IGES186 family is unchanged: positive-Z expanding45° cone194,
two horizontal plane190 caps, full circular100 rims, a straight110 seam and
complete502/504/508/510/514 directed incidence. No rounded vendor graph is
accepted, no pcurve or curve approximation is invented, and the original
parametric API remains available.

The native builder uses four exact rational circular quarters at each height,
with positive quarter weights(1,1,2). Each side is an exact rational tensor chart
linear in height between corresponding circular controls. Its radius is
r(v)=r0+(r1−r0)v; positive end radii and height prove an embedded conical band.
Separately sized exact circular trims on planar caps complete the boundary.
Sew verifies every continuous model/pcurve edge identity and closed opposite
incidence. The result has six native faces, mapped back to the original three
analytic source faces, and retains original lower/upper full-rim IDs separately
from native quarter-edge IDs. This is a real closed material promotion, not a
collection of independently approximated faces.

`WriteConical45FrustumMaterialJSON` and its reader store original source,
metadata, dimensions, units, native geometry and ancestry. Reload regenerates
and compares the entire binding. `WriteConical45FrustumMaterialSource` re-proves
the binding before unchanged source export. Normal descriptor-based IGES output
continues through the existing strict analytic writer. Source is bounded to
32KiB plus caller entity/record/value limits; native envelope cap is512KiB.
All control coordinates and endpoint sums must be exactly representable.

## Qualification

Tests use the independently authored retained `conical45-frustum.igs` and an
additional translated native-authored graph, checking native/plain JSON reload,
exact zero shared-edge discrepancy, closed mesh, analytic area/volume/centroid,
forged native/descriptor rejection and resource limits. The genuine retained
rounded OCCT source remains an explicit rejection.

Live qualification uses the pinned general solid comparator via
`PLAINSOLID_OCCT_IGES_SOLID`, checking original and emitted analytic files as
valid three-face solids and comparing OCCT volume mass properties with closed
form results. For r0=1,r1=3,h=2: volume26π/3, areaπ(10+8√2), centroidZ17/13.
The extrusion-specific elliptical helper is not used: its reported center is
an area-weighted surface centroid, which coincides with these symmetric prisms
but not with a frustum's material centroid.

`FuzzConical45FrustumMaterial` retains the positive original source seed. Focused
normal/race, Go1.23, vet,100-execution fuzz and existing STEP/IGES ellipse/circle
native regressions qualify the shared exact-cap helper change.

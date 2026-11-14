# Analytic convex prism corner fillets

`Prism` rounds every corner of a strictly convex counterclockwise XY polygon by
the same positive radius. `PrismCorners` selects vertical corners and accepts
an independent radius at each one, leaving other corners sharp. Both retain
planar caps, straight planar walls, circular
model-space edges, and cylindrical corner faces. The extrusion is vertical
between the supplied Z planes. This is a profile-corner fillet, not an arbitrary
3D edge fillet or a rolling-ball blend between general surfaces.

For exterior turning angle `theta`, each adjacent edge is shortened by
`r*tan(theta/2)`. The center is the intersection of its two inward offset
supporting lines. The circular sweep is `theta`. With these ideal constructions,
ordered tangent points on a convex polygon define disjoint corner triangles;
each rounded arc stays within its own triangle, and retained original-edge
segments cannot cross. This establishes nonoverlap of the ideal filleted
profile when the retained edge lengths are positive.

The supported numerical regime is stricter. Exact orientation predicates prove
strict convexity. Exact rational comparisons on represented centers certify
support-line clearance, tangency residual bands, and separation of every pair
of corner disks. Each nonadjacent support clears its disk by four tolerances;
center separation exceeds the sum of the two radii plus eight tolerances.
Retained segments have positive
setback margins and their represented endpoints/tangencies pass rational
residual checks. The disk-separation condition deliberately rejects some valid
large radii before the ideal short-edge collapse limit. Concave profiles, holes,
clockwise ordering, nonpositive selected radii, collapsed edges, and overlapping disks
are unsupported.

Coordinates and angles require float square roots/normalization/trigonometry.
“Analytic” means circular/cylindrical geometry is preserved, not that irrational
coordinates are represented exactly. `trimmed.NewSolid` checks shared topology
and sampled model-edge/p-curve agreement within tolerance; it does not supply a
continuous roundoff or embedding certificate. Endpoint agreement and tangency
are numerical, within the stated tolerance. Conforming tessellations are tested
for closed topology and exact triangle self-intersections independently.

`Corners` provides original-corner ancestry, centers, tangent endpoints,
setbacks, sweeps, arc edge IDs, and cylindrical face IDs. Cap and straight-face
IDs are also returned. Nominal area, perimeter, and volume use the ideal formulas

`A = A_polygon - sum(r_i² * (tan(theta_i/2) - theta_i/2))`

`P = P_polygon - sum(r_i * (2*tan(theta_i/2) - theta_i))`.

Use `Solid.MassProperties` to integrate the actual returned trimmed geometry.
Tests compare that independent integration with rectangle and regular-polygon
analytic formulas, translation/rotation/scaling behavior, and mesh conformity.
The selective unequal-radius fixture also matches OCCT's native 3D edge fillets.

Defaults: 64 corners (maximum 128), 100,000 support/pair work checks, tolerance
1e-9 model units. Radius and height must exceed eight tolerances. Context is
checked during construction and downstream topology validation. Failures return
no partial solid.

`WrappedCylinderRims` rounds either circular end rim of a canonical wrapped
cylinder. The retained cylinder and caps meet exact rational quarter-toroid
blends along shared quadrant curves. Each selected radius must be smaller
than half the cylinder radius, and their sum must leave a positive straight
lateral band. The result retains explicit seam and fillet ancestry and can
be saved and revalidated through the native trimmed format.

### Through-bore after circular rim blends

`BoreWrappedRoundedCylinder` removes an axial, optionally eccentric circular
bore from a validated canonical rim-filleted cylinder. The complete bore disk
must lie strictly inside both original flat cap disks, checked by exact squared
clearance. Those cap radii bound every intervening stock section from below, so
the bore cannot reach the original toroidal blends or lateral wall.

The graft preserves all original vertex, edge and face IDs and original surface
objects. Only cap hole lists change; the new reversed cylindrical wall and rim
edges are appended. Exact donor/source cap-chart equality, cap trim certification
and model-edge agreement gate construction. History separates surviving original
faces from the generated bore wall. Tangency, unsupported source topology,
nonrepresentable native geometry and exhausted proofs fail atomically. This is a
local through-removal subset, not a general Boolean against filleted stock.

`BlindBoreWrappedRoundedCylinder` uses the same strict radial clearance and
source-preserving graft for a single cap entry. Its exact floor must lie strictly
inside the straight outer wall's axial band, leaving positive stock beyond the
floor and excluding shoulder/through transitions. Native donor face roles supply
the inward floor support and orientation. `FloorFace`/`FloorZ` identify the new
floor (no original face ancestor); `BoreRims` retain lower/upper axial ordering.
The opposite original cap is untouched. For the through API, `FloorFace` is -1
and `FloorZ` is nil. Both modes retain every original topology ID.

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

`BoreWrappedRoundedCylinderSet` composes 1..16 independently authored through or
blind removals against the same original stock. For every overlapping CLOSED
axial interval pair, exact squared center distance must strictly exceed the
squared sum of radii. Disk tangency, shared floors and overlapping tools fail;
blind cutters with strictly separated axial intervals may share a center.
Every cutter first passes the existing single-stock proof. Generated topology
is then remapped into one result, all cap holes receive exact trim validation,
and the combined closed topology/model edges are checked. Original IDs remain
fixed and each cutter receives its own final wall/floor/rim history. No partial
result is returned on a conflict. Through specifications require zero depth and
no upper-entry flag, so ignored parameters cannot silently change meaning.

### Independent sharp circular rim chamfers

`ChamferWrappedCylinderRims` replaces either or both sharp rims of a validated
canonical circular cylinder with exact conical bands. Each end has independently
authored radial cap setback and axial wall runout; both must be positive or both
zero. Setbacks remain below the radius and summed runouts below the height,
proving positive-radius bands and a surviving central cylindrical wall.

The original circular basis/weights and physical cap planes are retained by
exact support edits and native station joining. History maps the central wall
and caps to their original supports, and each conical chamfer to its original
cap plus lateral face. Original source rims and new cap/shoulder rims are kept
separate; rebuilt output IDs are not represented as retained original IDs.
Apex/runout contact, unrepresentable geometry, malformed selections and exhausted
proofs fail atomically. This does not chamfer an already blended or bored stock.

`ChamferWrappedFrustumRims` extends this treatment to an ascending or descending
canonical circular frustum. Shoulder radii are evaluated on the original affine
wall equation at the authored axial runouts; they are not copied from the cap
radii. Each chamfer's removed radial width varies linearly from its cap setback
to zero at the shoulder, proving it stays inside the original stock. The middle
wall equation, cap planes, circular basis and original support ancestry remain
intact. Each setback is checked against its own cap radius; strict runout and
representation gates remain. The cylinder entry point is unchanged.

`ChamferWrappedCylinderTubeRims` treats a validated coaxial circular tube with
four independently authored cap-rim bevels. Outer setbacks reduce the stock
radius; inner setbacks enlarge the cap opening. Both original circular bases
and cap planes are retained. Each wall's end runouts must remain disjoint, and
strict radial clearance is proved at the union of all cap/shoulder heights;
between these levels the radius difference is affine. Native station-loft
joining and exact nested difference then revalidate shared seams and annuli.
History distinguishes original wall fragments from bevel fragments attributed
to the original wall plus cap. Different wall runouts can subdivide one bevel
into several result faces without losing its original ownership. Tangent walls,
apex/runout collapse and nonrepresentable output fail atomically.

`ChamferWrappedFrustumTubeRims` accepts independently tapered coaxial original
outer and inner walls. Each bevel adds/removes a linearly vanishing radial width
from the original affine radius function; shoulders use the original wall
values at their actual heights. Strict nested clearance is proved on the union
of all runout heights, and the same exact native station-bore construction
retains cap/wall ancestry and circular bases. This accepts rational slopes
without requiring rational unit-normal lengths. Eccentric axes remain outside
this API; all original cylinder tube behavior is unchanged.

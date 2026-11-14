# Exact rational plane frames in mapped planar material

`ReadPythagoreanMappedPlanarMaterials` is separate from the cardinal mapped
reader. It accepts the same complete two-to-eight-map closed material topology,
but permits exactly perpendicular plane X/Z direction ratios whose Euclidean
lengths are rational. It derives their normalization and Y=Z×X with rational
arithmetic. Model LINE and each actual owner's lifted planar LINE must contain
both original exact model endpoints, proved by zero rational cross product.
Thus the complete straight segment lies on both supports; this is not a claim
that arbitrary curve speeds are identical. Existing directed edge/face topology,
material embedding, contexts, ownership and mapped-use checks still apply.

Original model vertices must remain exactly representable binary64 points.
Plane ratios and lifted owner calculations need not be binary64 unit vectors.
No rounded control reconstruction or source snapping is used. Native output is
an exact planar BRep from those model points, not a new exact floating plane
normal claim. Source ratios, owner curves and entity IDs remain in Source.
Mapped targets and mapping origins retain the older signed-cardinal/identity
contract; this extension concerns original local geometry only.

`WritePythagoreanMappedPlanarMaterials` takes the cardinal native model plus one
`PythagoreanLocalFrame` per local product. Raw X/Z integer ratios are bounded to
one million per component, must be orthogonal and have rational lengths. The
proper normalized rigid transform is applied exactly to model points and plane
origins. Nonrepresentable transformed points reject before publication. Raw
model/plane directions are emitted as exact integer ratios; owner UV geometry
is retained under the rigid change. Complete source readback and native oriented
loop comparison precede output. Existing cardinal and document APIs do not widen.

Rational source coordinates have a128-character token cap, exponents bounded to
±1024 before rational allocation, and numerator/denominator caps of4096bits.
The ordinary cumulative entity/vertex/face/coedge/depth limits also apply.
Irrational direction norms, nonorthogonal axes, incorrect owner curves, missing
ownership and unsupported native rounding all reject.

## Independent qualification

`author_pythagorean_mapped_planar.py` independently transforms the retained OCCT
box graph: scale its original2×3×4 local product by5, rotate its original model
and owner data using exact(3,−4;4,3)/5, then translate by(5,−10,2). Scaling adjusts
original planar pcurve origins as well as model points; rigid rotation preserves
UV geometry. It retains original topology, the second independently authored
OCCT product and all mapped uses. No Go writer generates this incoming fixture.

The pinned OCCT reference revision remains
`4dbfdcc448c0b3e565488afcd626c84483172db9`. Ordinary TransferRoots/BRepCheck accepts
both this independent source and native Go output:3solids,18faces,volume6006,
area2622,centroid(71943/6006,61506/6006,31509/6006). The old cardinal reader rejects
both noncardinal graphs. Enable `TestOCCTPythagoreanMapped` with
`PLAINSOLID_OCCT_DISK_BOUND_MASS`.

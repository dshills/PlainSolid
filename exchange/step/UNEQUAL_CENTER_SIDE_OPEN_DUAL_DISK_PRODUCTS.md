# Explicit unequal-center side-cut STEP products

`UnequalCenterSideOpenDualDiskDescriptor` embeds the standard stock/frame/cutter
metadata and adds `Join [2]*big.Rat` in original stock-local coordinates.
`WriteUnequalCenterSideOpenDualDiskProduct` registers a distinct AP203 product ID
`side-cut-unequal-centers`; all prior source categories remain separate.

The native constructor independently verifies both exact circle residuals,
strictly external unequal center ordinates, overlapping rational side footprints,
the unique above-side crossing and excluded lower crossing, and stock clearance.
The join is checked evidence, never a point to snap. Native original vertices
must remain exactly represented under the signed-coordinate frame. Rational
coefficient size and geometric work remain bounded by native validation.

The emitter uses the immutable continuously sewn result, preserving each original
wall owner, its own center/radius and actual face loop incidence. It checks model
arc endpoints exactly on their original circle, minor-arc direction and opposing
coedge ownership before emitting LINE/CIRCLE/PLANE/CYLINDRICAL_SURFACE topology.
It emits no rounded pcurves or spline controls. Failure before the output write
is atomic; names and supported SI units are retained in the complete product root.

The independent fixture in `UNEQUAL_CENTER_SIDE_REFERENCE.md` is qualified in
blind/through, both cutter input orders and identity/reflected signed frames.
Eight live OCCT TransferRoots/BRepCheck cases match the independently generated
box-minus-fused-cylinders area, volume and transformed centroid. Tests also reject
nil, wrong, nonrepresentable or over-budget joins, the excluded lower crossing,
common center ordinates, invalid depth/units, cancellation and partial writes.

This writer milestone does not accept vendor source graphs. The reference OCCT
files retain independently authored angular pcurves and are rejected by the
existing strict source readers. The old equal, between-center unequal, external
radical, exact-wall and common-ordinate offset routes are unchanged.

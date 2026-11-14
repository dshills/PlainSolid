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

This dialect does not accept vendor source graphs. The reference OCCT
files retain independently authored angular pcurves and are rejected by the
existing strict source readers. The old equal, between-center unequal, external
radical, exact-wall and common-ordinate offset routes are unchanged.

## Strict typed re-import

`ReadUnequalCenterSideOpenDualDiskProduct` recovers the explicit join from actual
source ownership: exactly two cylindrical ADVANCED_FACE loops must share exactly
one EDGE_CURVE with a direct LINE support. Its two source model vertices must
have identical stock-local XY and axial endpoints at 0 and Depth; that XY must
satisfy both original circle equations exactly and lie above the opening side.
The native explicit-join certificate then proves the selected upper crossing and
excludes the other crossing. No caller hint or product string supplies the join.

The entire regenerated rooted graph is compared, including the actual LINE
support, all endpoints, complete face/edge incidence, directions, names and units.
Thus the preliminary ownership recovery is not a partial-geometry acceptance
path. Every unsupported source returns an empty typed result. Optional encoding
normalization is limited to the prior individually proved line/circle/plane
variants and still requires the complete graph comparison.

Returned descriptors normalize stock origin and disk order while preserving the
original world-space cutter centers, including their distinct below-side
ordinates. Join coordinates are in that normalized local frame. Descriptor Join
and the returned RadicalJoin proof are separate owned rational copies. Native
wall/floor IDs and source wall/floor face IDs remain distinct and explicit.

Tests cover typed blind/through and signed-pose roundtrip, original caller order,
nonzero stock origin, meter/millimeter metadata, composed opt-in encodings,
modified join vertices, circle radii, model owners, forged class roots, limits,
cancellation and retained vendor rejection. Eight re-exported typed results also
pass independent OCCT validity and mass/centroid comparison.

# Qualified unequal-radius side cuts

`UnequalSideOpenDualDiskDescriptor` and
`WriteUnequalSideOpenDualDiskProduct` are a separate writer API for the native
`trimmed.CutUnequalSideOpenDualDisks` domain. They do not broaden the equal-radius
writer, reader or its optional encoding equivalences. Equal radii must use the
equal API. The separate reader described below re-proves this exact unequal domain.

The two unequal positive-radius disks open the same lower rectangular side and
start at local z=0, with a common depth not exceeding stock height. Their upper
transverse crossing must be rational, exactly representable and strictly between
the centers. Both cutters must clear the other stock sides, and all native chart
coefficients and signed-coordinate placements must be losslessly representable.
Tangent, contained, irrational-crossing and unsupported placements fail closed.

The native constructor proves the circular envelope and closed sewn boundary.
The shared private STEP emitter retains the actual vertices, edges, coedges,
planes, subdivisions, and original cutter-owned cylinders with their distinct
radii. Each exposed circular arc is shorter than π under the native crossing
constraint. Exact endpoint circle equations and directed cross products select
its model-only `CIRCLE` traversal. There are no rounded angular pcurves.
The AP203 product/name/SI-unit envelope, exact decimal scalar checks, bounded
staging and I/O error behavior match the equal writer.

Independent OCCT qualification uses stock 60×22×6, disk centers (20,0) and (34,0),
radii 13 and 15, and depths 3 or 6. The exact crossing is (25,12). Tests compare
the exported STEP's conventional root transfer, validity, area, volume and
centroid with an independently constructed OCCT Boolean and analytic formulas.
Both cutter orders and identity/reflected frames pass:

- Blind: volume 6422.3981978, area 3674.30129501.
- Through: volume 4924.79639561, area 2726.20138856.

Set `PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS` to the existing standalone helper
built with `unequal-side`, `unequal-through` and `read` modes for these optional
tests. Native exactness is established by the constructor and emitter checks;
OCCT's floating numerical measures are independent qualification evidence.

## Strict unequal re-import

`ReadUnequalSideOpenDualDiskProduct` recovers each exact source cylinder center
and its own radius, normalizes the stock-local origin, sorts the paired disk
records together by local X, rebuilds with the unequal native constructor, and
compares the complete rooted entity graph. It returns the unequal native result;
it never exposes equal-radius analytical methods on unequal geometry.
`SourceWallFaces` maps each normalized disk to its original source face ID, and
`SourceFloorFace` identifies the stopped cut floor (zero for through).
Original caller disk ordering cannot be inferred from this geometry dialect.

`ReadUnequalSideOpenDualDiskProductWithOptions` supports the same explicit
orientation options as the equal reader. Each exact line segment, directed
circle arc and reflected planar support is separately proved, then the entire
source graph—including radii, centers, native ownership, cap geometry, names,
units and dependency sharing—is compared. The options do not permit substituting
one circle/cylinder for its neighbor or importing rounded vendor pcurves.

Both readers retain disjoint domain gates: unequal import rejects equal radii,
and equal import rejects unequal radii with any option combination. Tests cover
reversed input order, signed placement, source lineage, unchanged coordinates
under source-unit declarations, malformed units, swapped radius owners and
wrong circular-edge ownership. Optional live tests compare the re-imported
native solid and its regenerated STEP in OCCT and retain rejection of independently
generated OCCT unequal-cut source files. Bounds, cancellation and all-or-none
reader behavior are inherited from the shared strict graph reconstruction path.

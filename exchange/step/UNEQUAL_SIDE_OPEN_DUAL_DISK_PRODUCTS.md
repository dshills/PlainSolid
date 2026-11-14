# Qualified unequal-radius side cuts

`UnequalSideOpenDualDiskDescriptor` and
`WriteUnequalSideOpenDualDiskProduct` are a separate writer API for the native
`trimmed.CutUnequalSideOpenDualDisks` domain. They do not broaden the equal-radius
writer, reader or its optional encoding equivalences. Equal radii must use the
equal API. No unequal reader is included in this milestone.

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

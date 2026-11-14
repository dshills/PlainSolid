# Offset two-disc side-opening products

`WriteOffsetSideOpenDualDiskProduct` is a separate model-only AP203 dialect,
registered by PRODUCT identifier `side-cut-offset-disks`. Its descriptor retains
name, SI length unit, signed-coordinate frame, stock rectangle, two original
unequal cutters, height and blind/through depth. Cutter centers have a common
strictly external offset from the stock side; this is not a relaxation of any
side-centered reader or writer.

The writer obtains `trimmed.CutOffsetSideOpenDualDisks`' immutable sewn exact
boundary. Original wall face ancestry selects each cutter's own analytic cylinder.
Every model arc endpoint is checked against that original circle with exact
rational arithmetic. Directed minor-arc orientation, opposing coedge ownership,
closed face loops and exact planar/axial model lines are checked before output.
The native exact homogeneous walls are retained as proof; their controls are not
rounded into STEP spline records. Output contains planes, cylinders, CIRCLE and
LINE edges, no angular pcurves. Failure before the output write is atomic.

There is no offset reader in this milestone. All four prior strict side-cut
readers retain their original domains. In particular the separately retained
OCCT source, which includes vendor-authored angular pcurves, is not accepted as
this canonical dialect. Ordinary OCCT TransferRoots accepts the emitted product.

## Independent reference

The standalone `tools/occtref/overlapping_side_pockets.cxx` helper adds
`offset-side` and `offset-through`, constructing an OCCT box minus a fuse of
independent cylinders, followed by same-domain unification. It uses the pinned
OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9` documented in the oracle README.
Stock is [0,40] × [0,10] × [0,6], cutter centers (12.5,-7.5), (22,-7.5),
radii 65/8 and 87/8. Blind source cuts from z=3 upward; the native descriptor cuts
from local z=0 to 3, so the comparison explicitly reflects source centroid Z.
Through cuts span the full stock height.

| Source | Volume | Area | Source centroid |
|---|---:|---:|---|
| blind | 2282.9027441680728 | 1405.4325696885198 | (19.92733579442936,5.189578779803342,2.923060286209477) |
| through | 2165.8054883361469 | 1332.8003021557549 | (19.84681420820717,5.399657235130166,3) |

Both retained independent fixtures have 582 entities and 9 unified faces, including
2 cylinders. The native blind writer keeps 16 certified faces; the through writer
has 9. Different planar subdivision is intentional. Live tests compare area,
volume and transformed centroid for both cutter orders and two signed poses,
eight cases total; OCCT BRepCheck and standard single-root transfer are required.

Run with `PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS=/path/to/helper go test
./exchange/step -run TestOCCTWriteOffsetSideOpenDualDiskProduct -v`.
Default tests exercise source category rejection, exact wall class, deterministic
output, individually retained radii, cancellation, invalid units/depth/section,
short writes, and parse/rejection of the retained external source.

Retained source SHA256 (trailing whitespace removed):
- blind: `6f760167a3480890b0714197e861153faa672f8a7f40f5829b9dda5c31784d77`
- through: `0a73d9c43a993400ecb3e023841ebce17f1e4c199ebfc6e0d039e5ec7116c707`

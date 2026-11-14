# Named exact-wall side-cut STEP writer

`ExactWallSideOpenDualDiskDescriptor` and
`WriteExactWallSideOpenDualDiskProduct` select a separate canonical dialect for
external-radical unequal cuts requiring exact homogeneous native walls. The
writer proves this class: the ordinary general float-spline construction must
fail specifically with `geom.ErrUnrepresentable`, then
`CutGeneralSideOpenDualDisksWithExactWalls` must construct the continuously sewn
native solid. Ordinary float-wall shapes, equal radii and between-center joins
are not reclassified. The exact constructor's immutable result is used directly;
its complete sewing proof is not redundantly rerun by the emitter.

The native walls must be `geom.ExactBezierSurface`. Their rational coefficients
are retained by the kernel; the writer does not round or export them as floating
B-spline controls. Instead the existing checked model-only emitter writes the
original analytic cylinders, planes, CIRCLE/LINE edges, exact model endpoints and
retained directed topology. The circular endpoint identities and native wall
ancestry are unchanged. Model endpoints still must be exactly represented by the
current vertex format; this is not a general rational-endpoint STEP codec.

The complete AP203 product envelope has the explicit identifier
`side-cut-exact-walls`, while the authored product/solid names and SI units remain
intact. This marker captures the named dialect; it does not substitute for the
native geometry proof. All three earlier canonical readers/writers remain
strict. The separate exact-wall reader below retains this registered class contract.

## Qualification

The fixture uses stock 50×22×6, radii 13/15, centers (20,0)/(24,0), exact join
(15,12), and a retained native coefficient 26/3. Blind and through cuts, both input
orders and identity/reflected poses pass conventional OCCT TransferRoots,
BRepCheck validity, independent Boolean and analytic measures:

- Blind: volume 5482.55729549, area 3113.50758768.
- Through: volume 4365.11459098, area 2418.05337235.

The standalone OCCT helper's `exact-wall-side`, `exact-wall-through` and `read`
modes are exercised with `PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS`. Tests verify
native exact surface kinds and the 26/3 coefficient, no emitted B-spline records,
registered product ID, unchanged older reader rejection, invalid-class failures
without output, and I/O/cancellation propagation.

The unchanged independent-model splitter lives on the PlainCAD candidate rather
than this main kernel snapshot. A separate temporary qualification used the two
splitter implementation files from candidate 84509980625fd448b5d0942b06204f700198e00b,
joined an ordinary equal product before an exact-wall product, split the file,
and compared the entire second partition's rooted graph after renumbering.
`TestExactWallUnchangedSplitQualification` passed; the ordinary first partition
also passed its strict typed reader. No splitter or bridge acceptance changes
were made for this dialect.


## Strict exact-wall reader

`ReadExactWallSideOpenDualDiskProduct` and its `WithOptions` variant recover the
represented stock and original disk support, re-establish failure of ordinary
float-wall storage, construct the exact homogeneous-wall native solid, and
compare the complete rooted analytic model graph. The named product identifier
is mandatory; removing it fails. Adding the marker to an ordinary binary-wall
source also fails, because metadata never replaces the geometry-class proof.

The result retains source body/shell/face/edge IDs, per-disk source wall ownership,
the blind floor source ID, exact radical join coordinates and explicit left/right
classification. Original input order is normalized by local X. Exact native wall
surface types are checked, and native version-7 save/reload preserves coefficients
such as 26/3. No exact control is exported as a rounded STEP spline pole.

Only explicitly selected existing line/circle/plane orientation identities are
normalized; exact support and complete shared topology are still compared after
those proofs. Malformed support radii, circle ownership, names, units and product
class reject without partial results. All three old reader categories remain
unchanged, including with their orientation options enabled.

Focused tests cover blind/through and signed/reordered inputs, both external join
sides, all supported orientation identities composed, class tampering, native
persistence and optional OCCT readback. The unchanged candidate splitter was also
checked in the same isolated qualification described above, now including a real
`ReadExactWallSideOpenDualDiskProduct` call on the renumbered exact-wall partition.
Both product partitions retained their strict readback contracts.

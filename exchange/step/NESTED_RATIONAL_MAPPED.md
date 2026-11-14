# One exact nested mapped representation level

`ReadNestedRationalMappedPlanarMaterials` adds one explicitly bounded hierarchy
above the independently proved rational mapped local material domain. One root
SHAPE_REPRESENTATION contains uses of one shared intermediate representation map;
that intermediate SHAPE_REPRESENTATION contains uses of two to eight distinct
original planar local maps. Both mapping origins and common unit/context rules
retain the existing exact contract. Up to sixteen outer and sixteen inner uses
are allowed, with at most64 expanded occurrences. Recursive, cyclic or deeper
mapping is unsupported.

Original records must all belong to the complete validated root dependency graph
before the group wrappers are removed from a private flat proof view. Exact
outer operators are checked separately. The existing rational local proof checks
all inner poses, original bodies, model/owner pcurves and material ownership.
The composed transform is rational matrix multiplication without conversion to
binary64. Each expanded occurrence records both original item/target/map IDs,
representation IDs and names, outermost first. Repeated outer uses retain shared
leaf map IDs; their reused leaf item IDs alone are not occurrence identity.
No general assembly hierarchy, Boolean union or inter-instance nonoverlap claim
is made. Original flat readers remain strict.

`WriteNestedRationalMappedPlanarSource` is validated source write-back; it does
not synthesize a fresh arbitrary nested assembly from native cached values.
The separate document entry points `ReadNestedRationalMappedPlanarSTEP` and
`ReadNestedRationalMappedPlanarDocument` preserve this source, shared local native
assets, exact composed poses and two-step ancestry in a separately tagged version1
envelope. Ordinary flat native envelopes are not silently upgraded. Source
regeneration validates the complete original hierarchy. `OccurrencePaths` aligns
with the expanded `Occurrences` order. `BakeOccurrence(itemID)` rejects nested
documents because a leaf ID is reused; `BakeOccurrenceIndex` selects explicitly
and still rejects any world vertex needing rounding.

## Independent source and checks

`author_nested_rational_mapped.py` independently adds one intermediate map over
the retained independently OCCT-authored two-box graph. Three inner uses are
reused twice: identity outer pose, and exact3:4:5 outer rotation with Xtranslation100.
An inner occurrence already has a3:4:5 rational pose, so its second composed
orientation contains exact fractions−7/25 and24/25 rather than a copied frame.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots and
BRepCheck accept the independently authored source:6solids,36faces,V108,A252,
world bounds(−10,−8,0) to(106,22,4). Source→native→reload→STEP passes the same
independent bound check (`PLAINSOLID_OCCT_MAPPED_BOUNDS`). Tests cover source path
preservation, shared local assets, exact composed coordinates, represented-only
baking, flat-format rejection, cycles, duplicate uses, context mismatch, ignored
orphans, operator scale and cumulative limits.

## Editing outer group poses

`WriteNestedRationalMappedOuterPlacements` separately authors exact poses for
all outer group uses. Inner uses and their complete referenced geometry/context
closure remain byte-for-byte equivalent at original entity IDs. The editor
requires each original outer item once, so a leaf item cannot be mistaken for a
group use. New operator/datum records are private to each edited outer item;
item/map/name identity stays stable, while ancestry correctly reports the new
target IDs in the newly authored source. The old source is unmodified and can
still be exported independently. Unreferenced old operator datums are removed;
any datum still owned by the inner closure remains.

`RationalMappedPlanarDocument.WithOuterPlacements` returns a new separately
validated document and verifies unchanged native local asset bytes. It does not
mutate the receiver or retain caller-owned rational values. The separately tagged
nested version1 envelope and flat API restrictions stay unchanged.

A retained-source edit raises the first group by5 in Z and moves the second
rational group to Xtranslation200. Exact native bounds are(−10,−8,0) to(206,22,9).
After native write/reload and STEP regeneration, pinned OCCT independently
confirms these bounds and valid material. Tests verify unchanged inner dependency
records and old source, stable leaf ancestry, new outer target ancestry, shared
local assets, copied input rationals, invalid leaf/missing/duplicate selection,
irrational frames, budgets and atomic rejection. Enable
`TestOCCTEditedNestedMappedNative` with `PLAINSOLID_OCCT_MAPPED_BOUNDS`.

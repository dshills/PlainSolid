# Two exact shared mapped group levels

`ReadTwoLevelRationalMappedPlanarMaterials` adds a separate bounded source domain:
exactly two shared intermediate group levels above the previously proved local
planar maps. Each group definition has one identity representation-map origin,
a common original unit context and at most sixteen uses. At most128 composed
occurrences are returned. The existing flat and one-level APIs remain strict.
This is not general recursive assembly parsing or arbitrary DAG acceptance.

Every original entity must be reachable before any private proof view is built.
The outer map, target frame and group references are inspected, then only a
private product-root reference is redirected to the middle representation. The
existing one-level proof validates the remaining complete local graph. Exact
matrix multiplication composes all poses without floating conversion. Three-step
source paths retain original item/target/map/representation IDs and names;
repeated leaf item IDs are explicitly not unique instance identifiers.

`WriteTwoLevelRationalMappedPlanarSource` preserves validated source topology.
Separate document entry points read this STEP domain and its distinctly tagged
native version1 envelope. Shared local assets, exact composed bounds and all
three ancestry steps survive reload. `TwoLevelOccurrencePaths` returns copied
paths aligned with `Occurrences`. `BakeOccurrenceIndex` is explicit and
lossless-only; ambiguous item-only baking and the one-level outer edit API reject
this deeper document. Existing native formats are not reinterpreted.

## Independent observation

`author_two_level_rational_mapped.py` wraps the independent one-level source in
a second shared group definition with two uses: identity, and the cyclic pose
(x,y,z)→(z+200,x,y). Original inner rational rotations remain, so all three steps
visibly affect the final instances. Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9` TransferRoots/BRepCheck observes12solids,
72faces,volume216,area504. Exact bounds are(−10,−10,−8) to(204,106,22).
Native save/reload→source STEP passes the same independent bound check through
`TestOCCTTwoLevelMappedNative` and `PLAINSOLID_OCCT_MAPPED_BOUNDS`.

Regressions cover exact three-step coordinates, copied ancestry, shared assets,
format separation, cycles, context mismatch, cumulative limits and a144-instance
source rejected by the128-instance expansion cap. Fuzzing is bounded by source
bytes/entity/geometry limits and a fixed100-execution campaign.

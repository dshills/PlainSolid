# Heterogeneous exact STEP unit normalization

`ReadNormalizedIndependentMaterials` combines three previously proved families:
strict cardinal planar products, exact model-only half/quarter-cell cylinders,
and natural one-pole spheres. It accepts one to sixteen independently owned
products with individual metre/millimetre contexts and an explicit
`IndependentMetre` or `IndependentMillimetre` target. It retains the complete
original source, exact normalized source, per-root original units, typed member
geometry and original product/body/face ancestry. It does not infer assemblies,
union overlapping products, or validate unsupported families.

The complete source undergoes one shared root/dependency ownership partition.
Only a leaf whose every ADVANCED_FACE support is a PLANE receives the existing
positive rational LINE-speed proof view. Original speeds remain in source;
points, directions, topology and IDs are unchanged in that proof. Cylinder and
sphere leaves use their original strict scalar/topology proof directly. Thus a
planar source's rational-speed allowance cannot leak into another product family.

The dimensional engine scales each root according to its proven family: planar
3D and 2D points plus exact rational line speeds; round 3D points, radii and
binary64-exact line magnitudes; and exact decimal uncertainty metadata. Unit axes
and all topology remain invariant. The complete normalized graph is re-proved,
and its common target unit is checked. Both source partitions, scalar exponent /
bit / decimal limits, cumulative native limits and cancellation are enforced.
The older planar-only and round-only APIs retain their family restrictions.

The independently assembled fixture contains the original millimetre split
cylinder, genuine OCCT metre sphere and genuine OCCT metre box; only entity IDs
are rebased. `author_step_independent_heterogeneous_units.py` reproduces it without
Go-generated geometry. Both the mixed-unit original and normalized STEP output
transfer as three valid OCCT solids. Physical volume is
`24e9 + 12*pi + 32e9*pi/3` mm³ and area is
`52e6 + (16e6+20)*pi` mm². Use the existing round-unit comparator binary via
`PLAINSOLID_OCCT_INDEPENDENT_HETEROGENEOUS_UNITS` for the live test.

Tests cover native reload, original IDs, exact normalized dimensions, cross-root
borrowing, wrong support types, shifted pcurves, a nonbinary cylinder line speed,
cumulative limits, cancellation, prior API rejection, race and bounded fuzz.
`WriteNormalizedIndependentMaterialsJSON` and
`ReadNormalizedIndependentMaterialsJSON` use the distinct
`plainsolid-step-normalized-independent-materials` version-one envelope. They
retain the original source/units, normalized source, every root/shape role and
canonical native bytes. Reload invokes heterogeneous normalization once from
original source, then compares all stored claims. Planar speed allowances remain
confined to planar-owned records; an authored nonbinary cylinder speed still
fails before normalized material is accepted. Older planar, round and
unnormalized persistence tags remain unchanged and reject this new tag.

Persistence tests cover byte-stable reload, original model/pcurve speed changes,
projected-edge and pcurve offsets, round-family speed leakage, unit/target/root/
source/native tampering, limits, cancellation and atomic writer failure. The
three-product native reload→normalized STEP chain is independently read by OCCT.

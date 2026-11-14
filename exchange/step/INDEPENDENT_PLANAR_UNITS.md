# Exact planar STEP product normalization

`ReadNormalizedIndependentPlanarMaterials` is a separate planar-only metre /
millimetre route. It retains original and normalized graphs, original units,
product IDs, and qualified native materials. Old collection and round-product
normalizers remain strict and unchanged in accepted shape families.

The original product ownership and closed planar topology use the existing
cardinal-plane proof. Every edge is a model LINE with exactly its two owned
planar LINE pcurves. Original point and direction coordinates must be lossless;
the same supporting line contains both exact edge vertices. No TRIMMED_CURVE,
curved edge, angular UV, or explicit trim-parameter role is admitted.

The genuine OCCT metre export exposes an important distinction: model VECTOR
magnitudes are decimal `0.001`, while planar pcurve VECTOR magnitudes are `1`.
They are positive exact rational speeds, but not necessarily binary64-exact or
equal across owners. The new route checks every original speed with bounded exact
arithmetic, then uses a speed-one **proof view** to validate line support, finite
endpoint incidence and all ownership. Positive speed changes no line locus or
selected finite segment. No source point, direction, topology, units or record ID
is altered in this proof view, and the original speeds remain in retained source.
This is not a claim that the original model and pcurves use identical parameters.

Actual normalization scales all 3D points, all 2D planar UV points, and all model /
pcurve VECTOR magnitudes by the same exact unit factor. Plane axes remain unit
vectors. Thus each individual source curve retains its own parameter t, and the
plane lift commutes with scaling for its entire line. Points must remain exactly
binary64-representable; positive speeds and uncertainty metadata remain exact
finite decimals. The normalized graph receives the complete proof again. Work
is bounded to 256-bit scalars and 64 decimal places, with exponent checks before
rational allocation, whole-file parser/ownership caps, and selected native limits.

The independently authored fixture combines an original OCCT millimetre box and
an original OCCT metre box. The latter is generated with `write.step.unit=M` from
an OCCT box at (-10000,-4000,-2000), size (2000,3000,4000) in millimetres. No Go
writer creates either source. Original and normalized files transfer as two valid
OCCT solids with volume `24+24e9` mm³ and area `52+52e6` mm². Tests independently
check exact point/speed scaling and unchanged directions, reverse conversion of
the metre box, native reload, pcurve offsets, curve types, zero/negative speeds,
huge exponents, units, budgets and cancellation. Enable live qualification with
`PLAINSOLID_OCCT_INDEPENDENT_PLANAR_UNITS` and the dedicated helper in tools/occtref.

## Native persistence

`WriteNormalizedIndependentPlanarMaterialsJSON` and
`ReadNormalizedIndependentPlanarMaterialsJSON` use the distinct
`plainsolid-step-normalized-independent-planar-materials` version-one tag. The
original source retains each authored 2D/3D line speed, and the normalized source
retains its exact scaled speed. Reload invokes the complete planar normalization
pipeline once from original source, then compares both graphs, units, all product
and shape ancestry, and canonical native geometry. Stored normalized geometry is
not trusted as input. Older round and unnormalized tags remain separate.

Envelope allocation and source proof limits are independent; cancellation and
atomic writer validation follow the existing persistence contract. Tests reject
changed original model/pcurve speeds, shifted pcurves, projected-edge endpoints,
unit/target/root/native claims, and old tags. Byte-stable native reload followed
by normalized STEP output is independently transferred and measured by OCCT.

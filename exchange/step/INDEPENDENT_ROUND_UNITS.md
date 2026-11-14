# Exact SI normalization of independent round products

`ReadNormalizedIndependentRoundMaterials` is an opt-in bridge for independently
rooted model-only split cylinders and natural spheres. Each source root may use
metres or millimetres. The target is explicitly `RoundMetre` or `RoundMillimetre`.
Existing same-unit collection readers and persistence formats remain unchanged.

The original complete graph and each material are proved first. The separate
normalized graph preserves original IDs and product ownership. Every 3D point,
CIRCLE/CYLINDRICAL_SURFACE/SPHERICAL_SURFACE radius, VECTOR magnitude, and length
uncertainty is scaled by the exact rational factor 1000 or 1/1000. Direction
ratios, topology, senses, names and dimensionless parameters are unchanged. Shared
identity-frame coordinates are exactly zero and remain invariant. Only the length
SI_UNIT prefix changes; angular and solid-angle units remain untouched.

All changed geometry must remain exactly binary64-representable, and decimal
serialization is exact. Scalar work is bounded to 256-bit rationals and 64 decimal
places. Uncertainty metadata stays exact decimal and need not be binary64.
Unsupported unit declarations, family types, or unrepresentable results fail
atomically. All conversions finish before constructing normalized native shapes;
a complete normalized ownership/material reproof then supplies the result.
The original source is retained separately, never relabeled as normalized.
Caller tolerance is interpreted in each proof's model coordinates; this operation
does not use tolerance to accept geometry or round any dimension.

The fixture combines the independently authored millimetre cylinder with a
**genuine OCCT metre sphere export**, emitted using `write.step.unit=M` from a
sphere of radius 2000 in the OCCT millimetre system. The retained file has radius
2 and center (1,2,3) in metres. It is normalized to radius 2000 and center
(1000,2000,3000) in millimetres. Independent OCCT transfers both original and
normalized files with identical physical volume `12*pi + 32e9*pi/3` in mm³.
The reverse standalone sphere conversion is exact. In contrast, the 2 mm
cylinder radius cannot be represented exactly as binary64 metres and is rejected.
The helper and deterministic fixture combiner live under `tools/occtref` with
`independent_round_units` in their filenames. Enable the live test with
`PLAINSOLID_OCCT_INDEPENDENT_ROUND_UNITS`.

## Source-bound native persistence

`WriteNormalizedIndependentRoundMaterialsJSON` and
`ReadNormalizedIndependentRoundMaterialsJSON` use the separate
`plainsolid-step-normalized-independent-round-materials` version-one envelope.
It stores the original source graph, original per-root units, target unit,
normalized graph, original root/shape roles and canonical native bytes. Reload
runs the complete normalization pipeline once from the original source; stored
normalized geometry is never trusted as reconstruction input. Both source graphs
and all metadata/native claims must match. Envelope and STEP limits are separate,
cancellation is retained, and writer validation is atomic. Older native formats
and tags remain unchanged. Tests cover byte-stable reload, unit/target/root/source/
native tampering, limits, cancellation, bounded fuzz, and live OCCT transfer after
native reload and normalized STEP re-export.

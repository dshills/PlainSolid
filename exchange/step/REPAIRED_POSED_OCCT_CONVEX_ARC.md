# Proper signed-axis placement of original fillet repair

`RepairPosedOCCTConvexArcExtrusion` is additive to the six-face, single-fillet AP214 repair. It admits genuine translated and proper signed-coordinate orientations while retaining original source entity IDs, exact decimal positions, unchanged local product and explicit caller tolerance. Existing unposed repair and exact readers remain unchanged.

The first original model CIRCLE supplies two perpendicular cardinal direction ratios. Their signs define an exact proper rotation; arbitrary oblique rotations are unsupported. Every 3D CARTESIAN_POINT and DIRECTION reachable from the original body is rotated into a temporary local proof view. Neutral representation metadata is unchanged, and any shared coordinate record must be invariant. All other records remain present for complete strict rejection/ownership proof. No pcurve chart is transformed or silently ignored.

The existing bounded tangent repair runs in that view. The corrected graph is rotated back by the exact inverse, then `ReadPosedOCCTExactCircularLoopExtrusion` proves its complete original world graph. Every reported site's original point is checked against the actual original world decimal, and exact squared displacement is independently recomputed after rotation. This is not geometry fitting or an inferred source assembly placement.

The opaque result exposes the same copied source/repair/provenance methods plus `NormalizationRotation` and `ReferenceCircleID`. `Solid` remains product-local; `Descriptor.Frame` maps it to original model coordinates. The normalization has no translation: lateral translations stay in profile coordinates and axial translation in the descriptor frame. Consumers must apply that frame when querying world geometry. `WriteExactCircularLoopExtrusion` accepts the descriptor for independently readable generated STEP. Source-bound posed repair JSON is not part of this milestone; native geometry JSON roundtrip is tested separately.

The original inconsistent file still fails exact import. Corrected finite trim ownership is fully proved, but point/line repair bounds are not a Hausdorff claim for the original malformed finite trims. The exact mathematical supports are only rotated for proof; cardinal reference-azimuth normalization remains support-identical in the absence of pcurves. The numerical contract and limits in `REPAIRED_OCCT_CONVEX_ARC.md` apply unchanged.

## Independent qualification

The pinned `author_convex_arc_step.cxx` authors the actual OCCT fillet, then applies `BRepBuilderAPI_Transform` with:

- `write-x`: (x,y,z) → (z+2,x−3,y).
- `write-minus-x`: (x,y,z) → (2−z,−x−3,y).
- `write-translated`: (x,y,z) → (x+2,y,z+3).

All are proper rigid motions. The retained `occt-model-only-posed-oblique-prism-fillet.step` is the first case, normalized only for trailing whitespace. Its three offending world sites still carry the original nonzero discrepancy; the test does not rely on a pose that rounds it away.

With `PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL` pointing to the rebuilt helper, live tests reauthor all three sources and read original/corrected/generated STEP through ordinary `TransferRoots`. Each is valid one-solid/six-face material, with native-matching volume, area and world centroid. Native reload retains zero continuous shared-edge discrepancy. Adversaries cover a noncardinal original circle axis, disagreement between circle and cylinder axes, nonneutral metadata placement, incorrect original coedge ownership, insufficient tolerance, cancellation and limits. A fixed 60-execution single-worker fuzz campaign covers the retained posed source and malformed input.

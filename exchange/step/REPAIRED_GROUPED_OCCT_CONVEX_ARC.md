# Explicit repair of two independently authored prism fillets

`RepairGroupedOCCTConvexArcExtrusion` and `RepairPosedGroupedOCCTConvexArcExtrusion` are separate exactly-two-fillet APIs. They require seven faces (five planes and two outward cylinders), fifteen model edges, ten vertices and five convex profile segments. The previously qualified single-fillet entry points retain their original strict counts and continue rejecting these files. This is not arbitrary multiple-cylinder or pcurve import.

Each cylinder-incident vertex must have exactly one owning cylinder, one horizontal cap and one axial plane. The exact circle/plane tangency identity is proved against that cylinder, with tolerance independently bounded by each radius. The caller authorizes only the resulting point displacement and matched axial model-line translation. The result now exposes copied `Tangencies` evidence for every tangent, including original vertex/point/cylinder-face/cap-face/plane-face IDs and the exact repaired point. Posed results transport that point to original world coordinates.

## Matched reversed-circle proof

The actual second OCCT fillet uses −Z circle axes and true EDGE_CURVE same-sense, while both cylindrical supports remain +Z. For each negative-axis circle, the implementation requires:

1. Exactly one owning model edge, exactly two original oriented uses, and exclusive circle-frame/axis ownership.
2. Exact repaired endpoint membership in the original circle, including cap plane and radius.
3. A strictly negative planar determinant between endpoint radius vectors, proving the original −Z same-sense direction selects the clockwise **minor** arc. Antipodal, zero-length or complementary arcs reject.
4. Simultaneous exchange of model-edge endpoints, reversal of both original coedge senses, and +Z circle axis normalization. The new model edge traces the reverse of exactly the same corrected arc; reversed coedge senses preserve every material boundary traversal.

The entire corrected graph then passes the unchanged exact circular-loop importer, including unique directed loops, shared incidence, all planar/cylindrical supports and exact native material proof. An isolated axis flip, endpoint swap, one-coedge reversal, false same-sense, shared axis ownership, wrong cylinder, or topology-preserving complementary arc is rejected. `Reversals` returns copied original edge/circle/axis/use IDs and original endpoint IDs so source direction changes cannot be mistaken for unchanged orientation. All IDs remain present.

The posed mode chooses the original cylinder's cardinal axis for normalization, because a reference circle may intentionally point oppositely. All original body 3D coordinates/directions still transform exactly; neutral metadata remains invariant. The corrected world graph is independently re-proved. The native solid remains local and `Descriptor.Frame` carries its world placement.

## Independent qualification and limits

The retained `occt-model-only-two-oblique-prism-fillets.step` is the actual 207-entity OCCT `BRepFilletAPI_MakeFillet` result for radius 5/16 at the acute triangular corner and radius 1/4 at its right-angle corner. The pinned helper modes `write-both` and `write-both-x` author unposed and translated +X originals. Nine original point records need bounded correction; two circle edges need paired reversal; eight tangent vertices carry independently selected owner evidence.

Live original/corrected/generated transfer yields one valid seven-face solid, V=29.077978807115535 and A=65.62295497008438, with matching world centroids. Native JSON reload and every continuous shared-edge squared bound are separately checked (exact zero). Generated output uses the existing exact analytic writer and strict canonical reader.

The source limits remain 1 MiB, 1,024 entities, 8,192 values and depth 16; at most twelve point records may move. Tolerance must be positive finite and no more than one millionth of either radius. The unchanged single-fillet modes still permit at most six moved records. No legacy native format is widened and no source-bound grouped JSON envelope is added here.

As before, the original inconsistent finite trims are not declared exact. Bounds apply to original source point sites and shifted axial line supports. Exact corrected material is reconstructed; this is not a Hausdorff equivalence certificate for the original malformed B-rep. Pcurves, arbitrary axes and generalized shells remain unsupported.

Run the focused live checks with the rebuilt `tools/occtref/build_author_convex_arc_step.sh` helper through `PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL`. A fixed 60-execution single-worker fuzz campaign covers the genuine grouped source and malformed payloads.

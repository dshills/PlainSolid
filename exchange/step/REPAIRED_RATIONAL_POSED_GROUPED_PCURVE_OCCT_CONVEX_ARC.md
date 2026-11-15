# Rationally posed default OCCT two-fillet STEP repair

`RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion` authenticates a
complete two-fillet STEP source under an exactly orthonormal rational
rotation. The model circle supplies the reference direction and the original
cylinder supplies the positive axial direction. The original root, model
topology, exclusive owner graph and UV records are authenticated. As in the
single-fillet mode, `Pose` directly places the product-local `Solid` in source
world coordinates, and `CorrectedModelSTEP` carries the repaired world graph.

The tilted grouped fixture requires an unequal repair on axial edge `#252`.
In source world space its two projected stations retain their order.
The corrected line is parallel and its origin retains the original axial
station. Both corrected endpoints are independently derived from their
original shared tangent plane and cylinder at separate cap stations.
`AxialTrims` identifies the exclusive line and origin, the original vertices,
both directed station pairs and the exact squared distances. Linear
correspondence between the finite intervals bounds the complete line segment
displacement by the larger endpoint displacement. Each squared endpoint
bound is the perpendicular support shift squared plus its axial station
shift squared. Original vertex-to-source-line projection residuals are
proved separately.

Eight of the 28 LINE or CIRCLE owners need the finite planar LINE bounds.
The remaining owners retain their continuous original correspondence proofs.
Both rational cap curves retain the original full-turn radial projection
and monotonicity certificate. The repaired exact model vertices give the
cap endpoint rays with their original vertex and coedge identities. The
existing root proof selects the unique first-quadrant finite arc. Each
endpoint bound includes the original vertex displacement, its radial
distance and the rational cap projection error. Eight repair sites and
two matched circle and coedge reversals are reported in original units.
Each distance bound applies to the indicated source feature.

`tools/occtref/build_author_convex_arc_step.sh` builds the reproduction
helper against pinned OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`. The
`write-both-default-tilted` mode tilts a real OCCT filleted B-rep before
writing default AP214. The retained source has 440 entities and 30
pcurve owners.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairRationalPosedGroupedPCurveOCCTConvexArc' -count=1
```

The live test reloads the source-only native proof before writing the
repaired world model. Ordinary OCCT TransferRoots reads the regenerated
original and repaired world model as one valid seven-face solid. Independent volume
`29.077978807115539`, area `65.622954970084379`, and centroid
`(2.7627619257740741,1.0271143655601034,0.48298409896790118)` agree with
posed native material within `1e-11`. Strict bridge integration and
arbitrary rounded rotations remain open.

`WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON` and
`ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON` use a distinct
version-one source schema. Alongside the shared [source-only claims](REPAIRED_RATIONAL_POSED_PCURVE_OCCT_CONVEX_ARC.md),
the grouped record binds both directed axial intervals, circle and coedge
reversals, rational cap owner evidence, and the exact finite root recipes.
The writers and readers independently re-prove the original source under
bounded STEP and JSON input limits.

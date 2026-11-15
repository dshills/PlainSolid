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
posed native material within `1e-11`. Arbitrary rounded rotations require
separate qualification.

`WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON` and
`ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON` use a distinct
version-one source schema. Alongside the shared [source-only claims](REPAIRED_RATIONAL_POSED_PCURVE_OCCT_CONVEX_ARC.md),
the grouped record binds both directed axial intervals, circle and coedge
reversals, rational cap owner evidence, and the exact finite root recipes.
The writers and readers independently re-prove the original source under
bounded STEP and JSON input limits.

## Grouped metre and inch qualification

The pinned author modes `write-both-default-tilted-metre` and
`write-both-default-tilted-inch` create two independently authored originals
with matching native modelling and STEP output units. The repair allowance
and ledgers are in source units. Both files keep the exact rational world pose,
eight original finite LINE owners, two finite rational cap trims and the
unequal axial trim. Native reproof retains the exact source bytes and rejects
substituting the other valid unit's original document into its proof record.

The strict rational-world STEP consumer retains the same source unit, exact
pose and native solid and re-exports identical canonical bytes. The inch
factor in the canonical STEP is exactly `0.0254` metres. Pinned OCCT
`TransferRoots` reads the original, corrected and canonical graphs as one
valid seven-face solid. For the metre canonical it measures volume
`29077978807.11554` mm³ and area `65622954.970084399` mm². The corresponding
inch measures are `476502.69970284583` mm³ and `42337.305628499627` mm².
World centroids agree with the source-unit native results scaled by `1000`
or `127/5` to millimetres. A B-rep transferred from a different native unit
can have different owner parameter charts and needs its own proof.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRationalPosedGroupedConvexArcNonMillimetreUnits' -count=1
```

## Millimetre model converted to metre STEP by OCCT

The pinned `write-both-default-tilted-converted-metre` mode transfers a
millimetre-native two-fillet B-rep after scaling it by 1000. The resulting
432-entity AP214 file keeps the exact tilted world pose, metre model
geometry, and both original quadratic rational cap owners. It has eight
cylindrical degree-one spline owners. The 18 planar LINE owners keep unit
speed and use the exact model-to-owner map `t → t/1000`. Two planar CIRCLE
owners retain their analytic proofs.

Four cylindrical axial spans have exact slope `dv/dt = 1/1000` and knots
`[0,5000]`. On edge `#248`, the source and repaired finite model intervals
have different start stations. The repaired start is
`-3/500000000000000000000000000`, just outside the original spline knots.
The source proof records an exact directed affine map from the repaired
interval into the original model interval, which lies within the spline
knots. The lift difference is affine at constant angle, so the two
corresponding endpoint bounds apply throughout. The planar owner of this
edge has an unbounded LINE chart. Its proof covers both finite model
intervals with the same `t/1000` map.

Two cylindrical circle spans have `u = t`. The two owned by the quarter-turn
rational caps have `u = -t`. Endpoint order follows the source circle
reversal ledger. Exact rational sine and cosine enclosures prove both
quarter turns fit inside the authored interval `[0,1.570796326795]`.
Both rational caps retain their original radial and angular proofs and
positive-endpoint root enclosures. The source-only receipt reconstructs the
18 planar maps, eight original cylindrical laws, unequal axial map and
both finite cap proofs. A `1e-12` metre repair budget passes and a
`1e-13` metre budget rejects this original.

The independent OCCT evaluator checks all 30 actual owners, using radial
projection for the two rational caps. Ordinary OCCT transfer verifies
millimetre volume, surface area and centroid on the original, corrected
world file, and strict canonical metre file. The canonical read retains the
exact source pose and native solid and re-exports identical bytes.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairGroupedConvertedMetreConvexArc' -count=1
```

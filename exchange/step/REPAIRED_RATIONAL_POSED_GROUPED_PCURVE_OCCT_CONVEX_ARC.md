# Rationally posed default OCCT grouped-fillet STEP repair

`RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion` authenticates a
complete supported grouped-fillet STEP source under an exactly orthonormal rational
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

## Millimetre model converted to inch STEP by OCCT

The pinned `write-both-default-tilted-converted-inch` mode scales the
millimetre-native B-rep by 127 before transfer. The resulting 435-entity
original keeps the exact two-fillet model with 25-inch height, the declared
`127/5` millimetre inch factor and the rational world rotation.

All 18 planar LINE owners use `t → 5t/127`. The exact original model LINE
speed `3.937007874016E-02` differs from `5/127` by
`1/396875000000000` inch per millimetre. The two finite lift proofs include
this difference. The four axial splines have knots `[0,635]` and
`dv/dt = 5/127`. Edge `#248` has a positive repaired start
`1/984251968504000000000000000` and an original start of zero. Its exact
affine map gives the original spline parameter at both repaired endpoints,
and the lift remains within the authored knots.

The reversed cylindrical spans retain `u = -t` and the two rational caps
retain their exact radial and finite root proofs. OCCT uses opposite owner
orders for the two cap edges. Original `SURFACE_CURVE #220` lists cap
`#226` before cylinder `#237`. On `#271`, cylinder `#277` precedes cap
`#283`. Both source associations are authenticated. A changed master
representation, borrowed owner, shortened authored span, altered cap
pole, inexact inch declaration or edited model speed is rejected.

Source-only reproof and strict canonical inch reopen retain the exact
owner laws, pose, and native material. The source passes at `1e-11` inch
and rejects `1e-12` inch. Pinned OCCT samples all 30 original owners and
measures a maximum discrepancy of `1.11e-12` inch with the planar map.
The two original cap endpoint checks agree with the exact station
enclosures, with maximum distance `2.78e-13` inch. Independent transfers
of the original, corrected, and canonical STEP agree on world volume,
area and centroid in millimetres.

The separate `write-both-default-tilted-converted-inch-unscaled` output
rounds both oblique cylinder-plane tangencies. Faces `#247` and `#401`
share edge `#252` and have an exact source distance-minus-radius gap of
`1/5000000000000000` inch. Faces `#301` and `#428` share edge `#306` and
have a gap of `147/625000000000000` inch. OCCT transfers this source within
its tolerance. PlainSolid's endpoint repair retains the original supports,
so both gaps prevent exact reconstruction.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairGroupedConvertedInchConvexArc' -count=1
```

## Three disjoint fillets in an original millimetre STEP

The pinned `write-three-default-tilted` mode independently fillets the
three separate vertical edges of the triangular oblique prism. The 520-entity
AP214 original has one solid, three straight and three circular profile
segments, and eight faces. Its radii are `5/32`, `1/4`, and `5/16`
millimetres. The same explicit grouped repair retains ten repair sites, two
unequal axial intervals, and the exact source-world rotation.

Two original cylindrical CIRCLE owners on the additional fillet use
`u = -2.498091544797 + t`. Their constant axial parameters are 0 and 5.
The original cylinder and circle charts have different azimuths. The proof
expands the lift in `cos(t)` and `sin(t)`, encloses the sine and cosine of
the authored phase with rationals, and bounds the discrepancy for the entire
model curve. The exact original cylinder chart is retained in the source
receipt. Its corrected model carrier has an equivalent canonical azimuth
on the same axis and radius. Native reproof checks these two bounds, the
other 32 analytic owners, and the two original rational cap owners.

Pinned OCCT samples both ends and the midpoint of each phase-shifted owner.
The maximum observed difference is about `7.71e-14` mm with the authored
phase and `0.2965` mm with zero phase. Ordinary OCCT reads the original,
corrected and strict canonical files as valid eight-face solids and agrees
on world volume `28.968988172080639` mm³, area `64.184278587623695` mm²,
and centroid. The strict canonical Go reader recovers the same native solid
and re-exports identical bytes. The original requires explicit repair at
`1e-12` mm and rejects the smaller `1e-13` mm budget.

The separate pinned `write-three-default-tilted-nondyadic` mode uses a
third radius of `3/16` mm. Its exact local tangent vertices require
`(-2.775, 0.3)`, whose coordinates cannot be represented exactly by the
current profile. The source is rejected before material is published.
Wrong phases, traversal, nonunit oblique frames, and borrowed owners
also reject.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairThreeConvexArc' -count=1
```

## Three-fillet millimetre model converted to metre STEP

The pinned `write-three-default-tilted-converted-metre` mode scales that
millimetre-native model by 1000 before writing. The 508-entity AP214 source
keeps the three fillets, exact tilted placement and original metre unit.
Its 36 owners include 18 planar LINE maps `t → t/1000`, four planar
CIRCLEs, 12 clamped cylindrical degree-one splines and two rational caps.
Both original axial edges `#276` and `#449` require directed finite maps
from their corrected intervals into the authored intervals. The two
original rational caps retain their finite root and reversed-cylinder
proofs, including both authored owner orders.

The two oblique cylindrical charts `#207` and `#416` have knots
`[2.498091544797,4.712388980385]` and UV law
`(u,v) = (t - 2.498091544797, h)` at `h = 0` and `h = 5`.
The model circle angles at the original and repaired vertices are each
enclosed within `1e-12` radian of the corresponding knot by exact rational
sine and cosine bounds. The independently ordered circle angles and spline
knots are paired by normalized travel. Each source evaluation stays inside
the original knots and the angular discrepancy from the same-parameter
phase proof is bounded by `1e-12` radian across the entire span. The
resulting lift-distance bound is below `2.7e-13` metre for each shifted
chart. The receipt records the original spline coefficients and both
model-angle enclosures and recomputes them on reproof.

The original requires explicit repair at `1e-12` metre and rejects
`1e-13` metre. Strict canonical reopen preserves the source unit, pose and
native solid and re-exports identical bytes. Pinned OCCT independently
checks the source owners and measures the shifted charts at six stations
with a maximum distance of `2.85e-14` metre. It reads the original,
corrected and canonical worlds as one eight-face solid, with canonical
volume `28968988172.080647` mm³, area `64184278.587623693` mm² and
centroid `(2765.4896548128527, 1020.1651571311468, 479.34712691619546)` mm.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairThreeConvertedMetreConvexArc' -count=1
```

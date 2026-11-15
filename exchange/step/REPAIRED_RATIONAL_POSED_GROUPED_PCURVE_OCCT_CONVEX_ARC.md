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

## Three-fillet millimetre model converted to inch STEP

The pinned `write-three-default-tilted-converted-inch` mode scales the
millimetre-native model by 127 before transfer. The 511-entity STEP keeps
the exact `127/5` millimetre inch factor, world rotation and three circular
supports, with a 25-inch height. All 18 original planar LINE owners retain
`t → 5t/127`. The six axial splines retain knots `[0,635]` and
`dv/dt = 5/127`, with the authored rounded model LINE speed
`3.937007874016E-02` proved separately. Edges `#276` and `#449` keep
their directed unequal-trim maps. The corrected start stations are
`1/984251968504000000000000000` and
`-1/196850393700800000000000000`, respectively, with original start
stations of zero.

The oblique cylindrical owners retain the same original knots and exact
model-angle brackets as the metre original, with constant axial positions
0 and 25 inches. The whole-span normalized lift bound is below
`1.35e-12` inch for each of those owners. Pinned OCCT samples six
stations on the shifted charts with a maximum distance of
`1.43e-13` inch. Both finite rational cap proofs retain their original
paired cylinder owners, and the independent endpoint witness measures a
maximum cap distance of `2.78e-13` inch. The source passes at `1e-11`
inch and rejects `1e-12` inch, an inexact inch factor, changed model LINE
speed, and altered shifted knots even when their UV slope is unchanged.

The source-only receipt re-proves the original bytes and exact owner
charts. The strict canonical inch reopen preserves the native solid and
produces the same export bytes. Pinned OCCT reads the original, corrected
and canonical models as one eight-face solid. It measures canonical
volume `59339582.898891054` mm³, area `1035228.2293397824` mm², and
centroid `(351.21718616123235, 129.56097495565567, 60.877085118356803)` mm.

In the separate 523-entity `write-three-default-tilted-converted-inch-unscaled`
original the unit conversion rounds three tangent support pairs. The
orthogonal source cylinder axes and side-plane normals already have exact
unit length. Their exact plane-distance-minus-radius gaps are
`1/5000000000000000`, `147/625000000000000` and
`471/1250000000000000` inch on edges `#281`, `#335` and `#488`,
respectively. OCCT transfers the file within its tolerance. The PlainSolid
repair requires exact tangency of the original supporting surfaces and
rejects this model before publishing material.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairThreeConvertedInchConvexArc' -count=1
```

## Four disjoint fillets in an original millimetre STEP

The pinned `write-four-default-tilted` mode extrudes a trapezoid whose
profile vertices are `(0,0)`, `(2,0)`, `(2,1.5)`, and `(0,3)`.
It authors its planar support explicitly at the base of the 5 mm prism
and rounds all four separated vertical edges with radii of `1/4`, `3/8`,
`5/16`, and `5/32` mm. The default AP214 original
has 678 entities, one solid and ten faces. The grouped reader admits this
24-edge topology with four distinct cylinder supports, four straight
profile spans and exactly one original body. Its existing entity and
repair bounds apply.

The original 48 pcurve owners include 24 planar lines, six planar circles,
eight cylindrical lines, eight cylindrical arcs and two rational caps.
Eight planar lines and two rational caps need finite trim proofs. The
original unequal axial stations on edges `#338` and `#563` retain their
separate directed intervals. The two rational cap owners `#315` and
`#375` retain the same-circle pairs `#326` and `#368` with both original
owner orders. Their finite root enclosures remain in the source receipt.

On both caps the same-direction circular owners have two distinct
nonzero phases. Owners `#210` and `#482` use `u=t-1.570796326795`.
Owners `#267` and `#529` use `u=t-2.498091544797`. The existing
rational sine and cosine enclosures bound all four analytic charts
throughout the model arcs. The largest proved lift distance is below
`1.09e-13` mm. Pinned OCCT independently checks all 48 owners and compares
the four shifted charts at twelve stations. The latter check measures a
maximum `7.71e-14` mm with the authored phases and approximately
`0.442` mm with zero phase.

The original requires explicit repair at `1e-12` mm and rejects
`1e-13` mm. The strict canonical reader retains the exact world pose and
solid and re-exports identical bytes. Pinned OCCT independently reads the
original, corrected and canonical worlds as one ten-face solid with
canonical volume `22.155304183748054` mm³, area `51.012178844414059` mm²
and centroid `(2.5325644799355218, 1.1687478248848513, 0.789914026752639)` mm.

The separate `write-four-default-tilted-automatic-plane` original uses
OCCT's automatic planar support. In STEP, both cap planes are displaced
axially by exactly `-1/5000000000000` mm from their original model circles.
The exact repair retains those supports and rejects the incompatible
circle trims. Pinned OCCT transfers that independent original within
its tolerance. The separate `write-four-default-tilted-nondyadic` original
changes the acute fillet to `3/16` mm and requires non-binary tangent
coordinates. Wrong source phases, borrowed owners and an added B-rep root
item also reject without publishing repaired material.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairFourConvexArc' -count=1
```

## Four-fillet millimetre model converted to metre STEP

The pinned `write-four-default-tilted-converted-metre` mode scales the
millimetre-native trapezoidal prism by 1000 before transfer. The 662-entity
source keeps the exact metre declaration, rational world pose and four
circular supports. The 48 original owners include 24 planar LINE maps
`t → t/1000`, six planar CIRCLEs, 16 clamped cylindrical splines and two
rational caps. The rational cap proofs retain both original paired owner
orders and their finite angular root intervals.

Two same-direction cylindrical quarter spans start at zero. Four shifted
spans use the original nonzero knots. Owners `#207` and `#472` run from
`1.570796326795` to `2.498091544797` with unit angular speed. Owners
`#263` and `#518` run from `2.498091544797` to `4.712388980385` with
original `u0 = 4.440892098501e-16` and exact slope
`7380991451959998519702633833/7380991451960000000000000000`.
Both angular laws preserve fixed axial position. A rational bracket
encloses each directed model endpoint angle within `1e-12` radian of its
authored knot. With normalized travel through the original knots, the
angular discrepancy is bounded by the angle bracket plus the larger
authored coefficient discrepancy at either end. The two pairs have full
lift bounds below `4.47e-13` and `2.70e-13` metre,
respectively.

The original unequal axial trim on edge `#551` projects to directed line
stations `[0, 5000 + 4e-26]`. Its corrected model stations are
`[4e-26, 5000 + 4e-26]`. Its cylindrical spline owner `#564` has knots
`[0,5000]` and exact `dv/dt = 1/1000`. The directed owner map is
`t → t - 4e-26` on the repaired interval. The receipt records the original
projection, corrected interval and actual finite owner stations separately.
The clipped end removes an excess of `4e-29` metre. With constant angle,
the squared lift error to each model line is bounded at the corresponding
endpoints. The other unequal axial owner keeps its complete original
projected interval. No source spline evaluation leaves its authored knots.

The original passes explicit repair at `1e-12` metre and rejects
`1e-13` metre at shifted owner `#207`. Pinned OCCT independently samples
the original owners and observes a maximum shifted lift difference of
`1.22e-13` metre. Strict canonical reopen preserves the exact world pose
and native solid and re-exports identical bytes. Pinned OCCT reads one
ten-face solid from the original, corrected and canonical worlds. Its
canonical measurements are volume `22155304183.748058` mm³, area
`51012178.844414063` mm² and centroid
`(2532.56447993552, 1168.7478248848513, 789.91402675263919)` mm.

The independently authored `write-four-default-tilted-converted-metre-automatic-plane`
control transfers through OCCT and retains exact axial cap-plane gaps of
`1/5000000000000` metre to both original model circles. Exact repair rejects
this source, shortened knots, angular drift outside the permitted bounds,
and a forged finite owner endpoint in the native certificate.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairFourConvertedMetreConvexArc' -count=1
```

## Four-fillet millimetre model converted to inch STEP

The pinned `write-four-default-tilted-converted-inch` mode scales the
millimetre-native trapezoidal prism by 127 before transfer. The 665-entity
source declares the exact `127/5` millimetre inch factor and retains the
rational world pose, four circular supports and 25-inch height. Its 24
original planar LINE owners use `t → 5t/127`. The model LINE token
`3.937007874016E-02` enters the proof as authored, including its exact
difference of `1/396875000000000` from the owner scale.

The sixteen cylindrical degree-one spline charts preserve their source
knots, poles and directions. The four shifted arc owners retain the same
rational angle brackets and original angular coefficients as the metre
charts. The eight axial owners have knots `[0,635]` and
`dv/dt = 5/127`. Both finite axial maps, on edges `#332` and `#551`,
land inside the original knots. Their repaired start stations are
`1/984251968504000000000000000` and
`3/984251968504000000000000000`, respectively, with original start
stations of zero. Both cap pairs retain their original owner order and
finite rational root proofs.

Explicit source repair passes at `1e-11` inch and rejects `1e-12` inch
at planar owner `#64`. The original rejects under strict import.
Native reproof rejects a forged axial map. The strict canonical reader
preserves the inch factor, exact pose and native shape and re-exports
identical bytes. Pinned OCCT samples all 48 owners and observes a maximum
shifted lift difference of `6.06e-13` inch. It transfers the original,
corrected, and canonical worlds as one ten-face solid. Its canonical
measurements are volume `45382548.449818395` mm³, area
`822775.4325815544` mm² and centroid
`(321.63568895181112, 148.43097376037611, 100.31908139758519)` mm.

The independently authored
`write-four-default-tilted-converted-inch-unscaled` control contains three
rounded cylinder-to-plane support pairs. Their exact plane distance
minus radius is `1/5000000000000000`, `1/250000000000000`, and
`351/2500000000000000` inch on edges `#338`, `#392`, and `#619`.
OCCT transfers the original within its tolerance. PlainSolid rejects the
nonzero tangency gaps while retaining the exact authored supports. Inexact
inch declarations, edited LINE speed and undersized spline knots also reject.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairFourConvertedInchConvexArc' -count=1
```

## Five disjoint fillets in an original millimetre STEP

The pinned `write-five-default-tilted` mode builds a five-sided profile
with vertices `(0,0)`, `(3,0)`, `(3,2)`, `(2,2.75)`, and `(0,2.75)`.
It authors the base plane explicitly and rounds the five vertical edges
with distinct radii of `1/4`, `3/8`, `5/16`, `15/64` and `7/32` mm.
The 836-entity AP214 original contains one twelve-face solid, ten profile
segments and 60 original pcurve owners. This qualification covers the
native millimetre chart family.

The source proof retains 30 planar LINE owners, eight planar CIRCLEs,
ten axial cylindrical owners, ten cylindrical arcs and two rational
caps. The receipt includes twelve finite planar intervals, two unequal
axial trims and both rational cap roots. The six shifted cylindrical
charts keep their original offsets. Owners `#324` and `#633` have
`u = t - 3.14159265359` with constant axial positions 0 and 5. Their
unit circle and cylinder azimuths have exact dot product `-1`. For
these antipodal frames the existing rational sine and cosine proof
bounds the original lift over the full arc by `6.4e-14` mm. The
associated OCCT witness compares the two half-turn owners at six
stations. The general witness independently checks all 60 owners.

Explicit repair at `1e-12` mm preserves the original source bytes,
exact pose and native solid. A `1e-13` mm request rejects at original
oblique owner `#267`. An edited offset, a borrowed owner, a
non-antipodal frame and an insufficient entity budget also reject.
Strict canonical reopen re-exports identical STEP bytes. Pinned OCCT
reads the original, corrected and canonical files as one twelve-face
solid with canonical volume `39.084768347549009` mm³, area
`68.682455982680239` mm² and centroid
`(2.8653432472612699, 1.3261479966457206, 0.34620900365163998)` mm.

The independently authored `write-five-default-tilted-nondyadic`
control changes the chamfer-to-top fillet from `15/64` to `3/16` mm.
The supports at original edge `#804` are tangent exactly, and their
product-local tangent point is `(-217/80, 41/20)` on the base plane.
Those coordinates are non-binary. PlainSolid's current exact model
representation rejects them with `geom.ErrUnrepresentable`. OCCT
transfers the same original within its tolerance.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairFiveConvexArc' -count=1
```

## Five disjoint fillets converted to metres and inches

The pinned `write-five-default-tilted-converted-metre` and
`write-five-default-tilted-converted-inch` modes build the same five-fillet
product in millimetres and let OCCT convert its original model and charts.
The metre author scales the millimetre model by 1000. The inch author
scales it by 127 mm per base profile unit, so the resulting five-inch
coordinates retain the original exact dyadic support relations. Both
files contain one twelve-face solid with 60 pcurve owners.

Every original planar LINE retains the exact chart map (`1/1000` for
metres, `5/127` for inches). The original 3D LINE speed is the raw token
`1.E-03` for metres and `3.937007874016E-02` for inches. The receipt
retains the 20 cylindrical spline intervals and affine coefficients
independently of the planar map. For example, the original axial spans
map `[0,5000]` to `[0,5]` metres and `[0,635]` to `[0,25]` inches.
In the metre file edge `#653` retains its exact projected LINE station
`1250000000000000000000000000001/250000000000000000000000000`.
Owner `#666` clips the paired endpoint to its original knot at `5000`.
The receipt records both stations and the affine traversal inside the
stored knots. The source-derived inch stations lie inside the knots.
The two rational cap trims preserve both original owner orders.

Original spline owners `#319` and `#620` have knots `3.14159265359`
and `4.712388980385`. Their model and cylinder reference azimuths are
exactly antipodal. Their constant-axial charts have unit angular slope
starting at `u=0`. The finite proof encloses the original and repaired
model endpoints on the negative cosine and negative sine axes and
bounds the two complete lifts against the authored knot intervals.
Owners `#263` and `#574` preserve a different interval, from an oblique
quadrant II ray to the negative cosine axis. An altered half-turn knot,
reference azimuth or coefficient rejects, as does a substituted owner
in the written receipt.

The opt-in repair budgets are `1e-12` m and `1e-11` in. A `1e-13` m
request rejects at original owner `#207`. A `1e-12` in request rejects
at planar owner `#64`. Strict import rejects both original files.
Their corrected and canonical STEP transfer as one twelve-face solid
with the original pose and file units. The ordinary strict canonical
reader reproduces the exact native body and byte-stable re-export.
Pinned OCCT observes maximum antipodal-owner distances of `2.27e-14` m
and `1.14e-13` in at sampled stations. Removing the phase in that
witness increases those distances to `0.4375` m and `2.1875` in.

The pinned `write-five-default-tilted-converted-inch-unscaled` mode
exports the original physical millimetre-sized product directly to
inches. OCCT transfers it as a twelve-face solid. Its stored analytic
supports break exact tangency at seven shared edges. For example, the
plane distance minus the cylinder radius is `-429/1000000000000000` in
at edge `#723` and `-217/500000000000000` in at edge `#775`. The opt-in
reader rejects these stored support gaps.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairFiveConvertedConvexArc' -count=1
```

## Original two-fillet product with two interior cap wires

The pinned `write-both-default-tilted-through-bore` mode cuts a radius
`1/4` mm circular bore from the two-fillet prism before applying its exact
`3/5`, `4/5` tilt. The resulting AP214 original has 524 entities, one
solid, eight faces and two planar caps. Each cap references its own
single-circle interior wire. `InspectRationalPosedConvexArcThroughBore`
proves the bounded source and the corrected STEP graph with both wires.

The exterior proof still accounts for all original outer walls and
28 regular and two rational cap owners. The bore proof retains cap faces
`#137` and `#382`, original rims `#250` and `#435`, inward wall `#492`
and twice-used seam `#497`. It checks the complete original cap and
cylindrical lifts for both circles, cap and wall winding, and exclusive
owner links. Axial stations `1` and `6` measure the rim positions from
the original cylinder origin. The seam uses branches `6.28318530718`
and `0`, with the same affine axial law on both. A rational enclosure
of `2π` bounds the radial mismatch along the complete finite seam to
less than `1.05e-13` mm. The exact swept exterior cap proof places the
bore's entire enclosing square strictly inside both exterior cap trims.

The `1e-12` mm caller budget covers the exterior and the original seam
owner bound. A `1e-13` mm request rejects in exterior owner proof. An
edited seam exceeding the `1e-12` mm budget, changed owner or winding,
and separately authored blind bore also reject. The corrected exchange
graph reattaches the original interior wires and bore owners to the
repaired exterior and preserves their exact shared support records.
The native reconstruction retains the same geometry in a product-local
solid with four exact quarter-cylinder patches for the bore and a
clockwise interior loop on each cap. It certifies both complete planar
trim domains and every shared coedge. Its native world measures use the
original rational pose and match the independently reopened OCCT solid.

`WriteCanonicalSTEP` writes the full native geometry as a distinct
rationally placed analytic STEP product.
`ReadRationalPosedExactCircularLoopThroughHoleExtrusion` recovers the
occurrence and exterior recipe, then matches the entire input B-rep to
the regenerated analytic boundary. Strict reopen preserves native
material and re-exports the same bytes. Edited cap winding, borrowed
interior loops and incorrect bore wall incidence reject. The source-only
native receipt binds the original STEP bytes, outer owner ledgers, both
cap wires, all six bore owners and the native material digest. Reload
reconstructs the native body and verifies the exact stored claims.

Pinned OCCT reopens both the original and corrected graphs as one
eight-face solid, and the canonical graph as one eleven-face solid.
All three transfers have volume `28.096231102869` mm³ and area
`73.084237522361` mm². The separate `write-both-default-tilted-blind-bore`
control reopens in OCCT as a nine-face solid with a finite floor.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test -race ./exchange/step -run 'Through(Bore|Hole)' -count=1
```

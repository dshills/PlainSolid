# Rationally posed default OCCT single-fillet STEP repair

`RepairRationalPosedPCurveOCCTConvexArcExtrusion` authenticates the default
single-fillet solid under an exact rational proper rotation derived from an
original model circle. The circle's original axial and reference direction
ratios must be exactly unit and orthogonal. The exact transform applies to the
exclusive model dependency graph. The neutral representation placement and
original UV owner charts keep their values.

The complete local tangent repair authenticates the model topology and gives
three bounded world repair displacements on the retained fixtures. Their
source point tokens are compared exactly after applying the inverse
normalization. The returned `Pose` combines the exact rational rotation with
the descriptor's axial origin and places the native product-local `Solid`
directly in source world coordinates. The lateral translation remains in the
local profile. `OriginalSTEP` retains the full source and `CorrectedModelSTEP`
emits the corrected model-only graph in world coordinates.

The existing complete owner proof runs first for all 24 original pcurves.
Independently authored tilted sources introduce nonzero decimal direction
components on eight planar LINE owners. Their proof uses the directed finite
interval of the source model LINE. Orthogonal projection of both original
vertices must recover the same parameters as the proved corrected edge, and
each original projection residual must fit the caller's distance bound.
The two finite endpoints bound every component of the affine discrepancy.
For a source direction ratio vector `r`, STEP normalization contributes at
most `abs(1 - ||r||²)` times the authored speed and the maximum absolute model
parameter. All of these operations use exact rationals. Other owner kinds
retain their full-parameter proofs. `FiniteLineOwners` records the original
pcurve IDs and directed model parameter endpoints for the finite claims.

The finite comparison concerns the original pcurve lift at the same parameter
on the selected model interval. Each distance claim applies to its individual
owner or repair site. Exact original UV trim closure, a combined material
distance bound, and general rounded rotations remain open. Independently authored tilted
two-fillet sources use the separate [grouped repair](REPAIRED_RATIONAL_POSED_GROUPED_PCURVE_OCCT_CONVEX_ARC.md).

## Native metre and inch sources

The Go exchange reader also qualifies a translated tilted single-fillet
authored directly in metres or inches. The length unit, repair allowance,
owner records, model coordinate tokens, and exact pose stay in source units.
OCCT's inch declaration must contain the exact `127/5` millimetre factor,
length dimensions, and a millimetre SI base. The reader rejects a changed
decimal that would round to the same binary64 factor.

`WriteRationalPosedExactCircularLoopExtrusion` writes the repaired shape in
its original unit, including the exact `0.0254` metre factor for an inch.
Assuming the product-local frame is set to identity as
required by the writer, `ReadRationalPosedExactCircularLoopExtrusion`
recovers identical native geometry and the exact source-world pose before
writing byte-stable canonical STEP. To express world measures in millimetres,
apply the source length factor to world points after placement, its square
to areas, and its cube to volumes. The independent OCCT reader verifies that
scale on the original, corrected model-only, and generated canonical files.

The pinned author uses `write-default-tilted-translated-metre` and
`write-default-tilted-translated-inch` with the same local modelling and
output unit. These two default exports retain the proved analytic owner
charts.

## Millimetre model converted to metre STEP by OCCT

The pinned `write-default-tilted-translated-converted-metre` author starts
with metre-sized geometry in a millimetre-native OCCT model. During transfer
OCCT emits the metre context, changes the 3D LINE speed to `1/1000` and leaves
the planar owner LINE speed at one. For all 18 original planar LINE owners,
the exact model-to-owner map is `t → t/1000`. The proof checks both authored
speeds and encloses the lift discrepancy on the original finite model
interval, including the correction for direction normalization. The two
planar CIRCLE owners retain the existing proof.

The four original cylindrical owners are clamped degree-one B-splines with
two controls and one knot interval. `CylindricalSpanOwners` records that
interval and the exact affine UV coefficients. Two axial spans have constant
`u` and `dv/dt = 1/1000`. Their knots cover the entire directed original and
repaired LINE intervals. Two circular spans have `u = t` and constant `v`.
An exact cross-product inequality with rational sine and cosine enclosures
puts both original and repaired end rays inside each authored knot interval.
Truncating that interval while keeping `u = t` is rejected. The source-only
receipt authenticates all maps and coefficients by reconstruction.

The pinned OCCT evaluator independently samples all 24 original owners at
model-edge midpoints. It measures a maximum discrepancy of approximately
`1.54e-13` metres with the planar parameter map, and `2965.78125` metres
when the planar owner uses the model parameters directly. The original,
corrected and strict canonical STEP transfers keep the metre declaration,
exact placement, and equivalent independently measured physical geometry.
This qualification covers the proved degree-one cylindrical spans and
binary-lossless local model coordinates of this converted single-fillet
family.

## Millimetre model converted to inch STEP by OCCT

The pinned `write-default-tilted-translated-converted-inch` author scales the
millimetre-native model by 127 before transferring it with OCCT's inch output
unit. This choice retains the dyadic model geometry as five times the
dimensionless base profile in inches. The original cylinder and both
adjacent planes satisfy the exact tangency equations. The repaired shape keeps
its 25-inch height and the exact `127/5` millimetre unit factor. Its
product-local-to-world pose translates by `(8, 0, 6)` inches, with the
lateral shift retained in the local profile.

The file uses `3.937007874016E-02` as the model LINE speed. This authored
decimal exceeds the declared conversion `5/127` by exactly
`1/396875000000000` inch per millimetre. All 18 planar LINE owners retain
unit speed and use the declared exact parameter map `t → 5t/127`. The
finite lift bounds include the difference between that map and the actual
model speed. The two axial cylindrical owners have knots `[0,635]` and
exact slope `5/127`. At constant angle the difference between the lifted
spline and the model line is affine, so the source and repaired interval
endpoints bound the entire discrepancy. The zero-angle axial owner's maximum
drift is `25/15625000000001` inch. The two degree-one circular spans and two
planar circles retain the same proofs used for the converted metre source.
The source-only receipt re-proves every coefficient and accepts the fixture
with a `1e-11` inch budget. A `1e-12` inch budget rejects it.

The independent OCCT midpoint check measures a maximum owner lift discrepancy
of about `1.11e-12` inch with the declared planar map and `362.1875` inches
with the model parameter used directly. Ordinary OCCT transfer measures the
same world volume, surface area, and centroid in millimetres across the
original, corrected model-only, and strict canonical inch STEP.

The pinned `write-default-tilted-translated-converted-inch-unscaled` author
converts the base millimetre geometry directly. The decimal placement of the
original oblique plane, face `#340`, puts it
`297/1250000000000000` inch beyond tangency with the original
cylinder, face `#214`. Both values come from the original STEP tokens and its
exact unit side-plane normal. OCCT transfers the source using its geometric
tolerance. PlainSolid's endpoint repair rejects the file because it retains
both supports, which have no exact shared tangent regardless of vertex
placement.

## Source-only native proof

`WriteRationalPosedPCurveRepairedOCCTConvexArcJSON` and
`ReadRationalPosedPCurveRepairedOCCTConvexArcJSON` use the version-one
single-fillet source schema. The record stores the exact original bytes as
base64, the caller's maximum distance, and canonical claims for the exact
rational placement, provenance, repairs and original owner ledgers. The
source, corrected model, local recipe and native geometry have separately
checked SHA-256 claims. Reload reconstructs all derived material and checks
the canonical claims without parsing supplied rational coefficients.
The grouped reader uses its own source schema and also checks the finite
axial, reversal, rational cap and algebraic station evidence.

## Independent reproduction

Build `tools/occtref/build_author_convex_arc_step.sh` against OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`. The retained default fixtures use
`write-default-tilted` and `write-default-tilted-translated`. OCCT applies
the rotation `(x,y,z) → (3x/5+4z/5,y,-4x/5+3z/5)` to the
actual filleted B-rep before STEP transfer. The second fixture adds world
translation `(1,0,2)`. Each source has 352 entities and 24 owner pcurves.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^(TestRepairRationalPosedPCurveOCCTConvexArc|TestRationalPosedConvexArc|TestRepairConverted(Metre|Inch)ConvexArc)' -count=1
```

The live test regenerates both sources and reloads their native proofs.
Ordinary OCCT TransferRoots reads each original and repaired world STEP as
one valid six-face solid. Native
material and those transfers agree independently on volume
`29.145041881053842`, area `66.186284791166074`, and each world centroid
within `1e-11`. The negative cases change source placement orthogonality,
ownership, finite speed, preferred representation, metadata, or budgets.

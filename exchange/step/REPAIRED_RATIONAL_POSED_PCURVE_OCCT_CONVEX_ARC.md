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
charts. Exporting an existing millimetre-native B-rep in a different unit can
instead produce degree-one spline owners with different parameter scales.
Those charts require a separate proof.

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
  go test ./exchange/step -run '^(TestRepairRationalPosedPCurveOCCTConvexArc|TestRationalPosedConvexArc)' -count=1
```

The live test regenerates both sources and reloads their native proofs.
Ordinary OCCT TransferRoots reads each original and repaired world STEP as
one valid six-face solid. Native
material and those transfers agree independently on volume
`29.145041881053842`, area `66.186284791166074`, and each world centroid
within `1e-11`. The negative cases change source placement orthogonality,
ownership, finite speed, preferred representation, metadata, or budgets.

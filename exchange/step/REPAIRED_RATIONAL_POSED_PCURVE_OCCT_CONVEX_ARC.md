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
distance bound, general rounded rotations, native proof persistence, and
strict bridge integration remain open. Independently authored tilted
two-fillet sources use the separate [grouped repair](REPAIRED_RATIONAL_POSED_GROUPED_PCURVE_OCCT_CONVEX_ARC.md).

## Independent reproduction

Build `tools/occtref/build_author_convex_arc_step.sh` against OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`. The retained default fixtures use
`write-default-tilted` and `write-default-tilted-translated`. OCCT applies
the rotation `(x,y,z) → (3x/5+4z/5,y,-4x/5+3z/5)` to the
actual filleted B-rep before STEP transfer. The second fixture adds world
translation `(1,0,2)`. Each source has 352 entities and 24 owner pcurves.

```sh
PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL=/path/to/author-convex-arc-step \
  go test ./exchange/step -run '^TestRepairRationalPosedPCurveOCCTConvexArc' -count=1
```

The live test regenerates both sources. Ordinary OCCT TransferRoots reads
each original and repaired world STEP as one valid six-face solid. Native
material and those transfers agree independently on volume
`29.145041881053842`, area `66.186284791166074`, and each world centroid
within `1e-11`. The negative cases change source placement orthogonality,
ownership, finite speed, preferred representation, metadata, or budgets.

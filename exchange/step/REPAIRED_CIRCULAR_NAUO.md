# Explicit bounded circular-product repair

`RepairCircularNAUOProduct` is a separate, caller-authorized reconstruction. It
returns `RepairedCircularNAUO`, containing a newly owned canonical `CylinderBody`
and `parametric.Solid`, original inspection evidence, exact displacement bounds,
and source face/primary-edge/rim-vertex role IDs. It does **not** turn the original
file into an exact source-equivalent body, replace its failed conditions, or
change any exact importer. Only the selected source-local cylinder is rebuilt;
assembly poses and other products are not imported by this operation.

The strict `InspectCircularNAUOProduct` must first succeed. It proves the narrow
three-face support/incidence family and full rim model/pcurve support identities.
Malformed ownership, coedges, senses, support frames, pcurves, units or graph
layouts still fail. The repair chooses a complete canonical cylinder from the
original cylindrical surface and its two plane caps. Source center, axes, radius
and height are unchanged. They, the two new zero-angle rim vertices, and both cap
centers must all be losslessly representable by the existing native binary64
analytic model. A terminating decimal such as radius 2.1 is **not** silently
rounded into that model. Large origins that make center+radius unrepresentable
also fail before allocating the native shape.

The explicit `CircularNAUORepairOptions.MaxDistance` is in source model units.
It must be positive and finite, and at most `min(radius,height)/1,000,000`.
Original source uncertainty metadata is not an authorization or tolerance input.

## Exact bounded correspondences

For each original source vertex P, the chosen native vertex is
`Q = center + radius*X + height*Z` (height zero on the lower rim). The report
stores exact `|P-Q|²` from source decimal rationals. This also bounds point-to-rim
and point-to-cap distance; equation residuals with length-squared units are not
mistaken for distance tolerances.

Inspection establishes that the model seam is a unit axial LINE beginning at the
lower source vertex and reaching the upper source vertex after exactly the
support height. The repair maps that segment to the native zero-angle seam.
Its constant displacement squared is therefore the same `D²` as either endpoint.
This statement is about the explicit segment correspondence, not a new authored
trim parameter record.

For the two original cylinder seam pcurves, branch 1 is at decimal angle U and
branch 2 at zero. Reconstruction chooses mathematical angles 2π and zero.
Rigorous rational Machin bounds yield `ε >= |U-2π|`. Since unit-circle distance
is at most angular distance, the first owner lift has whole-line displacement
squared at most `r² ε²`; the second has zero displacement. For each owner,
model-to-owner disagreement is conservatively bounded by
`2*(D² + ownerBound²)`, using `(sqrt(a)+sqrt(b))² <= 2(a+b)` without rounded roots.
Every vertex, model-seam, owner-lift and combined model/owner squared bound must
fit `MaxDistance²`. A small radial residual alone cannot conceal an opposite-ray
vertex or a bad angular branch.

These are bounds on the mathematical analytic correspondences. They are not a
roundoff guarantee for arbitrary floating evaluator calls. The native constructor
retains its existing numeric evaluation/validation contract. This API does not
claim original finite circular trim selection, whole-face source equivalence,
a whole-source Hausdorff bound, or an inferred global assembly material.

## Source and native topology roles

The inspection report, including all original failed conditions and residuals,
is retained unchanged. Face role IDs map to native side/lower-cap/upper-cap.
The primary edge IDs map to native lower rim/seam/upper rim, and rim vertex IDs
map to the two newly constructed zero-angle vertices. Native polar caps introduce
additional radial/collapsed edges and center vertices; no one-to-one topology or
source parameterization identity is claimed for those added entities.

## Qualification

The genuine pinned OCCT fixture from `CIRCULAR_NAUO_INSPECTION.md` repairs at
`MaxDistance=1e-9` model units. Its original four exact failed conditions remain
visible. The returned local native cylinder has radius 2, height 5, area 28π,
volume 20π and centroid (0,0,5/2).

Independent pinned OCCT TransferRoots/BRepCheck accepts export of the newly
constructed descriptor through the existing `WriteCylinders` contract. The new
single solid has three faces, and its volume/area equal the original mixed
assembly totals minus the two independent box contributions (48 and 104).
This is an independent geometry comparison, not proof that a numeric STEP
export can encode symbolic 2π exactly.

Tests reject zero/negative/NaN/excessive/undersized tolerances, wrong-ray and
large-gap vertices, malformed owners, nonbinary support scalars, unrepresentable
constructed corners and parser limits. Valid inspection of such source does not
bypass the stricter repair gates. Normal, race, Go 1.23, vet and fixed-work
`FuzzRepairCircularNAUO -fuzztime=100x` cover atomic all-or-none reconstruction.

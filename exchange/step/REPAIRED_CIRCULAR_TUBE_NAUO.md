# Explicit repair of a circular through-tube product

`RepairCircularTubeNAUOProduct` is a separate opt-in local material reconstruction.
It selects one original NAUO product; it does not broaden the exact readers,
repair assembly placements, or add this family to the mixed DAG importer.

The original family has four faces: a positive outer cylindrical wall, a negative
inner cylindrical wall and two shared planar annular caps. Both supports must
have the same exact signed-cardinal frame, strictly positive ordered radii and
exactly the same positive height. Each wall has two one-vertex circle rims and a
twice-used axial LINE seam. Each cap has exactly one distinct rim from each wall.
Source EDGE_CURVE senses, oriented loop incidence, complete model circles,
planar circle pcurves and cylindrical line pcurves are checked. No additional
source face, shared loop, cross-product geometric ownership or unknown support
is admitted. The exact radial ordering proves concentric annular containment;
complete directed loop checks prove the outer/hole orientation of both caps.

This reuses the circular product proof by factoring the original shell into two
positive cylindrical support proofs. In a private parsed graph, FACE_BOUND
orientation is applied to its whole ordered loop; inner material loops are
complemented, and each annular cap contributes its corresponding rim. No source
point, direction, radius, model curve or pcurve coefficient is changed by this
factoring. Both full analytic pcurve lifts must equal the same model circle.
Association order can therefore be normalized for that support calculation,
while `RimMasterPCurveIDs` records the original selected master. In particular,
the genuine OCCT inner rims select the planar pcurve first. This calculation
does **not** certify the master's authored finite trim or parameter selection.

The result contains the new native `TubeBody` and `parametric.Solid`, original
body/shell/cap IDs, and separate outer/inner wall evidence. Each wall retains
original source failures, model/vertex IDs, pcurve branches and rational squared
displacement bounds from `RepairCircularNAUOProduct`. The original negative
inner face sense is retained explicitly. Construction chooses zero-angle native
seams and periodic seam branches; it does not pretend these equal the defective
source. The tolerance must bound every vertex, finite model seam, owner seam
and model/owner discrepancy, and be at most one millionth of each radius,
height **and wall thickness**. Center, frame, radii, height, thickness and the
primary constructed points must be exactly binary64-representable. No snapping
of underlying supports is permitted.

Bounds concern the mathematical analytic supports, not a new roundoff bound for
arbitrary floating evaluator calls. This is not a source-equivalence, finite
face-coverage or whole-source Hausdorff certificate. Newly authored output uses
`WriteTubes`; its source and topology are distinct from the retained input.
Native tube construction uses sixteen quarter patches and sixteen vertices,
rather than the source's four faces/four rim vertices. Limits require at least
16 faces, 16 vertices and 64 edge uses before allocation. Source limits also
include 8192 entities, 8 products and 32 direct occurrences plus normal parser
limits. Errors return no partial solid or evidence.

## Independent qualification

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` generates
`testdata/occt-circular-tube-nauo.step` through
`tools/occtref/author_circular_tube_nauo.cxx`. The tube is an independent
`BRepAlgoAPI_Cut` of radius-3 and radius-1 cylinders of height 5, placed in an XCAF
assembly alongside two planar boxes. Only trailing whitespace is stripped.

At `MaxDistance=1e-9` model units the two wall reports each retain four original
failed conditions: two rim vertices, one model seam and the decimal 2π branch.
The repaired tube has volume `40π`, area `56π` and centroid `(0,0,2.5)`.
Original OCCT TransferRoots/BRepCheck gives three solids, sixteen faces,
volume `48+40π` and area `104+56π`. Independently importing newly authored
`WriteTubes` output gives one valid solid/four faces with the tube's same mass
and centroid. Native save/reload is byte-stable; ordinary analytic reimport of
that newly authored file also passes. These numerical comparisons independently
qualify the result, rather than replacing its source arithmetic gates.

Focused mutations reject wrong wall/cap sense, reversed hole loop alone, shared
loop ownership, unexpected master selector, pcurve owner, radius, noncoaxial
center, height, excessive source gap, cross-product support sharing, extra
faces and undersized/oversized tolerance. Exact original cylinder and planar
family APIs remain unchanged. Normal/race/Go 1.23/vet and bounded
`FuzzRepairCircularTubeNAUO -fuzztime=100x` cover this isolated family.

# Exact IGES198 ring torus material

`ReadToroidalMaterial` and `WriteToroidalMaterial` authenticate a new canonical
13-record IGES186 ring-torus solid, distinct from standalone toroidal support.
The original graph has one198 face, one502 vertex and two complete100 circular
seams referenced twice with opposite508 uses. The major seam lies in XY at the
outer equator. The minor seam has a uniquely owned exact124 placement into XZ
at the major radius. The510/514/186 ownership and outward orientation are fully
checked. Extra geometry, alternate graphs and pcurves are unsupported.

The current descriptor is centered at the origin with axis+Z and reference+X.
MajorRadius>MinorRadius>0 is required, and the represented sums R±r must be
lossless. Every original source scalar, integer field, transform, reference,
metadata record and seam endpoint is rechecked against the descriptor-derived
analytic graph. Horn/spindle tori, approximate coordinates, wrong minor-circle
placement, changed coedge direction and wrong shell orientation fail closed.
The existing standalone198 support reader is unchanged.

## Native material proof

Promotion constructs sixteen original-geometry rational tensor patches: four
major-circle quarters × four minor-circle quarters. Each exact circle quarter
uses rational quadratic weights(1,1,2); tensor weights are their exact products.
Controls represent
((R+r*cos(v))*cos(u),(R+r*cos(v))*sin(u),r*sin(v))
without trigonometric rounding. Every pole must remain exactly representable.

For R>r, radial distance is strictly positive. Circle quarters are injective in
their interiors; their product charts cover the torus with intersections only
on declared shared seams. Their parameter orientation is outward. Exact sewing
checks continuous model/UV agreement and opposite closed incidence across all
64 uses. These constructive support/embedding facts establish material; a
positive integrated volume or watertight display mesh is only an independent
verification and does not replace the geometry proof.

`SourceFaceID` names the one original face shared by all sixteen native patches.
`SourceSeamCurveIDs` retains the two full circles and `NativeSeamEdges` maps each
to its four native quarter edges. Native quarter parameterization is not claimed
to equal the original angular circle parameter. The source-bound version1
native envelope regenerates all geometry and ancestry; source writing re-proves
before returning the unchanged file. Source caps are32KiB,512records,13entities,
2048values, native envelope1MiB, and caller limits remain effective.

## Independent qualification

`author_iges_toroidal_material.py` independently authors the analytic graph for
R=4,r=1 without Go geometry or writer helpers. The pinned general OCCT IGES solid
comparator imports original and Go output as valid one-face, one-solid material,
with volume8π² and area16π². Native mass, exact zero shared-edge discrepancy,
plain/native persistence and closed tessellation independently agree.

Tests reject horn/spindle/degenerate/nonfinite radii, wrong seam orientation,
wrong transform, invalid vertex index, changed circular endpoint, reversed shell,
forged native payload and input limits. `FuzzToroidalMaterial` retains the
positive independent fixture. Focused normal/race, Go1.23, vet, bounded100-run
fuzz and previous analytic support/conical/circular regressions qualify the
additive private parser mode.

## Proper signed-coordinate placement

`ReadPlacedToroidalMaterial` / `WritePlacedToroidal` add a separate 14-record
contract. A common type 124 is attached to the original type 198 support and
major seam. The minor seam's own type 124 must equal the exact composition of
that common placement with its original XZ frame. The type 502 vertex is stored
in world coordinates and independently inverted and checked against both seams.
No transform is inherited from a solid or shell, and nested type 124 records
are not accepted. Datum entities remain local to their owning support.

Only proper signed-coordinate rotations and exactly represented translations
are supported. Every transformed native control is calculated with rational
arithmetic and must convert losslessly to binary64. Reflections, general
rotations, disagreements between the two transforms, detached transforms,
rounded vertices, and translations that lose a control coordinate are rejected.
The unplaced reader remains strict. Source-bound placed JSON retains the common
placement, original source vertex, source metadata and native seam ancestry;
reload repeats the complete source and native proof.

The independent author script's `--placed` fixture rotates the torus axis to
+X and moves its center to (10,-3,5). Pinned OCCT imports both that source and
Go-authored output as one valid face/solid with volume 8π², area 16π² and the
correct volume centroid. Native exact continuous edge checks and source/native
reload agree. Focused race, Go 1.23, vet and 100-execution placed fuzz checks
cover the separate entry point. Entity reordering remains unsupported here.

## Semantic record and incidence reordering

`ReadReindexedToroidalMaterial` is a separate entry point for the same exact
unplaced or placed family. It accepts directory reindexing, the two type 504
edge-list entries in either order, and a cyclic start change of the four type
508 uses. Reversal remains unsupported. Major/minor seam roles are identified
by the shared/distinct transform ancestry of their original type 100 circles
and type 198 support, then independently checked by the unchanged canonical
geometry and placed-composition proofs. Numeric entity ordering is not used as
geometry evidence.

A bounded root traversal establishes a bijection for every original record,
including both distinct transforms in placed sources. Detached records,
conflicting/shared owner roles, recursive transforms and altered edge ordinals
fail closed. `SourceForCanonical`, `SourceEdgeOrdinals`,
`SourceVertexOrdinals`, and `SourceUseOrdinals` retain the original identities;
the sixteen native faces retain their original single source face and both
seam-curve IDs. Source-bound JSON repeats the entire proof and regenerates those
maps; source writing rejects changed maps before emitting any bytes.

The independent pointer-aware Python fixture generator reverses all directory
records, reverses the edge list, updates every owning ordinal, and rotates the
loop start. Both placed and unplaced originals transfer through pinned OCCT as
valid one-face/one-solid tori and retain native volume, area and centroid.
Focused race, Go 1.23, previous family regression and bounded fuzz qualify this
additive path under the same source limits and a 512-step graph traversal cap.

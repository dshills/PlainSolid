# Genuine planar STEP face with two independent elliptical holes

`ReadTwoEllipticalHoleFace` is a distinct exact source reader for one planar
outer ellipse with two eccentric, cardinally oriented ellipse holes. It returns
one native trimmed face, both original model curves/vertices and owner IDs,
individual exact containment certificates, and an exact pairwise separation
bound. This is not a material solid, two independent faces, or a relaxed
single-hole reader.

## Source ownership and complete domain

The genuine OCCT graph has 85 entities, one advanced face with three ordered
FACE_BOUND records, one positive outer bound, and two reversed inner bounds.
Each hole owns 19 records: its bound/loop/coedge, closed edge and vertex, analytic
model ellipse/frame, authoritative pcurve, definitional representation, original
2D ellipse/placement and parameter context. The shared analytic plane is explicit.

The reader traverses each hole dependency graph with cycle/depth limits,
stopping only at that shared plane. The two 19-record sets must be disjoint.
Removing either complete hole yields a 66-record source view that must pass the
existing strict eccentric-ellipse owner/model/pcurve proof. Their source union
is the entire original 85-record graph; their common 47 records prove the same
outer boundary, plane, product, representation and units. Detached records,
foreign sharing, references into another hole, wrong source dimensionality or
missing metadata cannot be silently discarded. The temporary single-hole
views are proof partitions, never returned as additional native faces.

Both holes must pass the exact enclosing-rectangle containment criterion from
ECCENTRIC_ELLIPSE_FACE.md. For pairwise separation the complete UV rectangles
must have a strictly positive interval gap along U or V. The largest positive
axis gap is returned as SeparationLower with its axis. Because the source
plane frame is exactly orthonormal, that gap is also a lower bound on physical
3D distance between the complete holes. Touching rectangle intervals, overlapping
boxes without another separating axis, or an inconclusive containment bound
are rejected. These are conservative sufficient certificates, not a general
ellipse-intersection solver or sampled acceptance.

The three simple disjoint closed loops and their declared positive/negative
windings prove the connected planar domain with two holes. Native outer edge
role is 0; hole roles 1,2 follow original bound-list order, not numeric source IDs.
Original IDs remain separately attached to each model/pcurve/vertex/bound.

## Source and native roundtrip

`WriteTwoEllipticalHoleFaceSource` re-proves and emits the complete original
source, including both holes. It does not bake arbitrary edits to decoded data.
The dedicated version 1 `WriteTwoEllipticalHoleFaceJSON` /
`ReadTwoEllipticalHoleFaceJSON` persist the original source and native chart,
all three curve/UV owner bindings, exact containment and separation evidence.
The redundant single-hole manifests are binding records, not separate faces.
Reload re-derives every proof and compares every native field. Cross-owned source
IDs, colliding native edge roles, swapped model/UV curves and forged separation
bounds fail atomically. Valid reload/rewrite is canonical-byte stable.

The aggregate parser is bounded at 64 KiB, 85 entities, 4096 values and depth 16;
proof subviews do not bypass the original cumulative limits. Native JSON is
capped at 256 KiB. Smaller caller limits apply. Writer source traversal is bounded
before serialization. Millimeter/radian/steradian units remain explicit, with
no geometry snapping or implicit conversion. Existing single-hole APIs remain
unchanged and reject this source family.

## Independent source and oracle

`tools/occtref/step_two_ellipse_holes.cxx` constructs a genuine OCCT face using
outer ellipse (10,6), center (1,2,3), normal +Y/reference −Z. Its holes have radii (2,1)
and source UV centers (−3,0), (3,1), the second with a 90 degree reference rotation.
Both reversed inner wires are added to the outer planar face, then exported by
STEPControl_AsIs. The retained `occt-two-ellipse-holes.step` is the original
export; only line-end whitespace is removed.

Containment enclosure values are 5/18 and 41/100; conservative physical outer
clearances are 13/6 and 177/100. Hole U intervals [−5,−1] and [2,4] have exact gap 3.
Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 conventionally transfers
original and STEP→native reload→STEP output as a valid one-face/three-edge shape.
Independent area is 56π, centroid (29/28,2,3), normal +Y, and total adaptive boundary
length 70.430894167891609. Native model points/tangents and both reversed pcurve
lifts agree; independent Simpson integration checks perimeter.

Tests also exercise reversed hole encounter order, positive U/V separation,
touching/overlapping enclosure rejection, duplicated bounds, foreign pcurve and
vertex ownership, flipped orientation, nonexact coefficients, native tampering,
resource limits and preserved single-hole strictness. Focused fuzz ran 100 inputs.
Optional live oracle: `PLAINSOLID_OCCT_TWO_ELLIPSE_HOLES`.

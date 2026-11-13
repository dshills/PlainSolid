# Convex planar affine draft

`Apply` replaces selected supporting halfspaces of one closed, embedded, outward-oriented convex planar BRep with their affine images. Unselected supports stay fixed. `Shear` constructs the exact map

    x -> x + displacement * dot(neutral.Normal, x-neutral.Origin)

for each selected face. This fixes the neutral plane pointwise and supports oblique neutral planes and different pulls on different faces. The displacement coefficient is not an angle. An arbitrary nonsingular affine map, including a reflection, is accepted by `Apply`.

All binary64 input coordinates and coefficients are interpreted as exact rationals. Original planarity and convexity, transformed planes, recession-direction tests, plane intersections, face hulls, and returned volume use rational arithmetic. No convex decomposition or tolerance-based welding is used. Faces may disappear, merge, or change their number of edges. Coplanar input faces must all receive the same resulting support; selecting only part of one supporting plane is rejected.

`Result` contains the rounded `Solid`, caller-owned exact vertices and volume, original-face ancestry for every output face, exact halfspaces, and an old-to-new face map (`-1` means the support disappeared). Exact vertices align with the solid's vertex array. A bounded empty or lower-dimensional intersection produces an empty regularized solid.

The recession test intersects the homogeneous support cone with a cube and checks its vertices exactly. Any nonzero vertex yields `ErrUnbounded`. This test intentionally rejects those normals even when inconsistent offsets would separately make the feasible region empty. Reconstruction enumerates plane triples and checks all halfspaces, so worst-case work grows as the fourth power of the support count. Defaults allow 32 unique supports; the hard maximum is 128. Work, source/output sizes, rational bit lengths, collision comparisons, and context cancellation bound the operation. Input and output embedding/orientation validation each have separate work budgets.

Float conversion must keep vertices distinct and meet `MaxCoordinateError` (default `1e-9`); the maximum actual squared displacement is returned exactly. The rounded output additionally passes BRep construction and exact mesh collision/orientation checks. Rounded vertices may not lie exactly on the rational output planes; exact metadata describes the mathematical result, while the returned BRep is the checked rounded representation. Small features or nonplanarity introduced by conversion can cause an explicit error. This is a convex planar support-editing operation, not general curved-surface draft or rolling-ball filleting.

Tests include analytic one/four-sided taper volumes, oblique neutral planes, top-face collapse, reflection, empty and unbounded intersections, nonconvex rejection, exact ancestry, conversion and work limits, and an analytic taper fuzz oracle.

## Selected two-distance chamfers

`Chamfer` accepts original edge IDs, one incident face ID, and separate strictly
positive setbacks into that face and its neighbor. It constructs all bevel
planes against the original solid and intersects them simultaneously with the
unchanged original supports. `ChamferResult.FaceEdges` attributes each bevel
face to its selected original edge; `EdgeFaces` maps selections to output faces
or `-1` if the bevel disappears. Original-face ancestry remains separate.

With `coordinateSetbacks=false`, distances are Euclidean, measured perpendicular
to the edge within each incident face. Exact square-root checks require rational
perpendicular lengths. Orthogonal and suitable Pythagorean geometry is supported;
irrational directions return `ErrUnsupported`, without approximation. With
`coordinateSetbacks=true`, distances instead measure the opposite face support's
linear coordinate, whose normal is normalized by its largest absolute component.
This explicitly different metric supports arbitrary rational planar geometry.
Neither mode creates blends, rounded corners, or curved fillets. Large setbacks
may remove original faces or empty the solid. Duplicate edge selections are
invalid. All draft validation, conversion and reconstruction limits apply.

## Convex hollowing

`Hollow` subtracts an interior reconstructed from independently inward-shifted
supports. Thickness values align with original face indices. Set
`HollowOptions.OpenFace=-1` for a closed cavity; otherwise the named face must
have zero thickness and all other thicknesses must be positive. The opening
must be a complete unique supporting face. The output remains a closed material
boundary, including rim surfaces, even when the cavity is accessible as a tray.
An empty/collapsed interior is an error rather than an unchanged-solid success.

Euclidean mode requires exact rational support-normal lengths; unsupported
irrational lengths fail explicitly. Coordinate mode measures thickness in the
same dominant-component normalized plane coordinate used by affine draft and
coordinate chamfers. The exact interior vertices/volume and original-support
ancestry are retained in `HollowResult.Interior`. Final Boolean history maps
operand A to the original base and operand B to that returned interior.
Reconstruction, original-support validation and final Boolean/history each use
separate configured bounded passes. Converted interior vertices must stay
strictly inside every unopened outer support, and an opening must remain exactly
coincident after conversion. This prevents rounding from changing the intended
cavity connectivity. Arbitrary curved shells and general face removal are not
supported.

## Convex support removal

`RemoveSupport` removes one complete unique supporting face and reconstructs
the remaining inequalities. It can undo a planar chamfer or corner cut. The
exact result contains the original convex solid because only one inequality is
removed. The added region lies in the removed support's exterior intersected
with the resulting convex body, and is therefore convex and connected when
nonempty. Remaining supporting normals must still bound a solid; otherwise the
operation fails with `ErrUnbounded`. Partial selection of a coplanar face group
is unsupported. Standard exact reconstruction, rounded output validation and
ancestry rules apply, with the removed face mapped to `-1`. This is not general
nonconvex feature recognition or curved defeaturing. Tests recover a unit cube
from an unequal two-distance chamfer and reject unbounded cube-face removal.

`RemoveSupports` removes several original support facets simultaneously and
retains `RemoveSupport` as its single-face wrapper. Empty selection performs
validated identity reconstruction; duplicate indices are invalid. A coplanar
support split into several faces must be selected in full. All original input
checks, remaining-normal boundedness proof, conversion and embedding checks
remain mandatory. The exact result contains the original body, but unlike
single-support removal, the aggregate added regions may be disconnected.
Analytic tests remove two or four independent chamfers. Live OCCT differential
tests independently reconstruct the remaining support intersections and compare
area, volume and centroid for both cases.

## Two-ended planar hollowing

`HollowThrough` extends the hollowing family to a through duct bounded by two
complete, unique, separated support faces. Both selected thicknesses
are zero; every other support moves strictly inward. Exact support signs at every original face vertex prove that each whole
opening lies strictly inside the other opening’s halfspace. This includes
nonparallel end planes whose finite faces are disjoint, while rejecting even
a single shared original corner or edge. Both reconstructed opening faces must
survive and remain exactly coincident with their original planes after vertex
conversion. Every converted cavity vertex must be strictly inside all unopened
supports. Touching or partial coplanar openings, collapsed cavities and
unproved conversion fail without returning a partial solid.

`ThroughHollowResult` embeds the existing hollow result and additionally reports
caller-owned ideal exact material volume and the two opening-to-interior face
IDs. Final material faces and edges retain the Boolean ancestry. Each support
reconstruction, containment check and Boolean pass retains its separate budget.
The output includes closed rim and cavity walls, rather than an open display
mesh. Euclidean and dominant-support-coordinate thickness keep their existing
explicit meanings; arbitrary curved shells remain outside this operation.

Independent box and sheared-box tests verify volume, centroid, exact interior,
opening ancestry, inward cavity orientation, genus-one topology and absence of
mesh contacts. Invalid end pairs, partial coplanar selection, exhausted work,
cancellation and collapsed cavities are rejected. An oblique cap regression
rejects a rounded cavity seam displaced from its original plane by less than
one ULP, even though that error is below the configured conversion tolerance.

Nonparallel wedge regression: `0≤y≤2`, `0≤z≤1`, `0≤x≤4−z`, with
openings at `x=0` and `x+z=4` and quarter-unit offsets of the other walls.
The exact material volume is `35/8`; independently integrated centroid and
surface area, closed genus-one topology and contact-free tessellation are
checked. A tiny but strictly positive original end-face gap remains accepted;
the coincident-edge limit is rejected before subtraction.

## Tapered planar chamfers

`ChamferTapered` accepts four strictly positive endpoint setbacks for every
selected original edge: one distance into each incident face at its stored
Start vertex and another pair at its stored End vertex. The distances vary
linearly along that original direction. Exact in-face offset construction and
four-point coplanarity prove a single planar bevel; incompatible width ratios
are rejected rather than approximated with a twisted or sampled surface.
The original edge endpoints must both lie strictly on the removed side, which
proves the same sign along the complete original edge.

Constant-width selections preserve `Chamfer` results. Mixed constant/tapered
selections are applied simultaneously to unchanged original supports. The same
Euclidean rational-length and dominant-support-coordinate metric rules apply.
Large/interacting bevels can remove faces or empty the convex result, with the
existing bounded reconstruction, conversion, and embedding checks. Selections
are atomic: any invalid edge, width, coplanarity or budget returns no result.

`TaperedChamferResult` retains the usual face/edge ancestry plus original edge
vertex direction, both incident face IDs, input widths and four caller-owned
exact offset points. Their linear interpolation describes the entire accepted
width field. Tests integrate tapered triangular cross-sections independently,
cover multiple separated bevels and mixed widths, reverse the original stored
edge direction, and check exact plane incidence and contact-free display.

`HollowPorts` extends through hollowing to 2–32 distinct complete supporting
faces. Every pair of original opening polygons must be strictly separated by
its original supports; adjacent or touching ports are rejected exactly. The
cavity must retain every port and conversion must preserve each opening plane
exactly. It returns original port IDs, cavity-face mappings, exact ideal volume,
and full Boolean topology history. Pairwise separation shares one reconstruction
work budget; reconstruction and Boolean validation retain independent budgets.
`HollowThrough` remains the compatible two-port wrapper.

## Independent original-edge hinge draft

`HingeDraft` rotates selected supporting faces about individually named incident
original edges. Each signed `HalfAngleTangent` means tan(angle/2), with the axis
oriented from the edge's stored Start to End. Rational Rodrigues coefficients
are constructed without rounding a transform matrix. Axis lengths must have
rational square roots; other axes fail explicitly. Reversing an edge's stored
orientation and negating its coefficient gives the identical rotation.

Returned hinge records retain original face/edge/endpoint IDs, exact fixed
endpoints and exact rotation matrices. Subsequent intersection with other moved
supports can trim away the hinge or remove a selected face; `OutputFace` is -1
for a vanished support. These records certify the original transformation axis,
not survival of an output edge. Preparation and reconstruction have independent
bounded work passes. This adds rigid per-face hinge rotation to the existing
common-neutral-plane shear; it does not create curved draft surfaces.

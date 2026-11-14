# Properly placed exact IGES194 frustum material

`ReadPlacedConical45FrustumMaterial` and `WritePlacedConical45Frustum` add one
proper signed-coordinate type124 placement to the independently authenticated
45° conical frustum graph. This is a separate canonical21-record dialect; old
unplaced, standalone-support and other placed codecs keep their entity gates.
No additional cone angle is admitted. IGES194 carries an angle in degrees;
a decimal approximation to atan(3/4) is not a proof of exact slope3/4.

The unique transform is attached to the100 circular rims,110 seam,190 cap planes
and194 wall. Original116/123 datums remain local to their analytic parents.
The502 vertex list carries world-space points and no transform. Every leaf
attachment and world vertex is checked; inverse mapping uses exact rational
arithmetic and must be losslessly representable. The existing local cone
validator then re-proves all original scalar equations, ordered bounds, senses
and shared incidence. Missing/foreign leaf transforms, reflections, scale/shear,
rounded rotations, lossy coordinates and changed half-angle are rejected.

The world `trimmed.Solid` is constructed directly from exact framed circular
quarter controls, separately sized cap trims and linearly varying conical wall
charts. No rounded transform is applied to a display mesh or trusted native
boundary. Sewing proves zero continuous edge discrepancy and closed incidence.
`Local` retains original analytic source roles, parameters and units; placement,
transform ID/metadata and original world vertices remain explicit. Native rim
IDs refer to world quarters and are kept separately from original full rims.

The separate version1 native envelope stores original source, the validated
local material recipe, world native geometry and placement/ancestry. Reload
reconstructs and checks every binding. `WritePlacedConical45FrustumSource`
re-proves before returning unchanged bytes. Source caps are32KiB,512records,
21entities,2048values; native envelope cap is1MiB, with caller limits honored.
Record/list reordering is not included in this milestone.

## Independent qualification

`author_iges_placed_elliptical_prisms.py --cone` independently places the retained
analytic194 source using (x,y,z)→(z+7.5,x−4.25,y+2.125), preserving local analytic
supports and explicitly transforming502 points. Pinned OCCT's general solid
comparator (`PLAINSOLID_OCCT_IGES_SOLID`) accepts original and emitted files as
valid three-face material. Areaπ(10+8√2), volume26π/3 and world centroid
(7.5+17/13,−4.25,2.125) agree with native integration and closed form.

Tests include exact native edge agreement, source/native reload, a proper pose
reversing the source axis, altered cone/rim transform ownership, bad matrix,
wrong world vertex, changed angle, forged persisted placement and bounded input
failures. `FuzzPlacedConical45Frustum` uses the original independent positive.

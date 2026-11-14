# Mixed baked and placed IGES roof geometry

`ReadMixedPlacedParabolicRoofSolid` and `WriteMixedPlacedParabolicRoofSolid`
handle a practical variation of the exact parabolic-roof material graph:
some model curves and surfaces are authored directly in world coordinates,
while the remaining leaves use a common proper signed-coordinate type124 pose.
The type502 vertices remain world coordinates. All owner UV curves remain in
local parameter coordinates with zero transform pointers.

This is an additive dialect, not a change to either existing unplaced or uniform
placed readers. At least one of the 18 model/support leaves must be baked and
one must use the shared matrix. There is exactly one terminal matrix, ID117,
and it cannot be referenced by topology or UV geometry. `RoofLeafBake` selects
canonical surface and model-curve roles for export; independent import retains
source entity ordering and mapped topology roles.

The reader decodes and retains `OriginalGeometry` and `SourceLeafTransforms`.
For proof only, baked control coefficients are mapped back by exact rational
inverse arithmetic. Any nonrepresentable result fails. Raw source weights must
already satisfy the lossless source gate. The full local ownership, continuous
model/UV equality, directed incidence, global embedding and outward material
checks then run through the uniform placed reader. Its world material is rebuilt
from exact transformed coefficients and independently certified. No sampled
comparison, nearest-point association or floating tolerance joins are used.
Original directory fields remain visible through staging: source style, form,
status and ownership violations cannot be silently removed by re-emission.

The returned `Local` describes the explicitly normalized local proof geometry;
`OriginalGeometry` is the actual authored mixture. `Solid` is the world material.
This is not a byte-preserving opaque IGES document editor. Labels, subscripts,
source IDs, explicit units and source leaf placement choices remain available.
Writers validate before output. Both directions retain the existing caps of
64 KiB, 1024 records, 59 entities, 4096 values and 104 controls, and honor smaller
caller limits. General affine matrices, reflections, independently chosen leaf
poses, rounded coefficients and arbitrary surface reparameterization are rejected.

## Independent qualification

`tools/occtref/author_iges_mixed_roof.py` authors the source independently from
polynomial equations, selecting baked leaves by source record position. Its
fixture uses the nontrivial world map `(x,y,z) -> (z+7,x-4,y+2)`, translated local
origin (.25,-2,3), and unit width, depth, height and rise. Both independently
created input and Go output pass ordinary TransferRoots/BRepCheck with pinned
OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`:
one solid, six faces, volume 4/3, area `20/3+sqrt(5)/2+asinh(2)/4`, and centroid
(10.7,-3.1875,.5). The independent numerical area allowance is 1e-8; it is not an
input acceptance tolerance. Optional live tests use `PLAINSOLID_OCCT_IGES_SOLID`.

Negative tests change placement pointers without changing the original poles,
place UV curves, inject ignored-looking directory metadata, exceed budgets, and
cancel validation. They require atomic failure and retain the old uniform API's
rejection of mixed files. The decoder has valid and malformed fuzz seeds.

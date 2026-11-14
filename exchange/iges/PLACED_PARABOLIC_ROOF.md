# Exact leaf placement of the parabolic material shell

`ReadPlacedParabolicRoofSolid` and `WritePlacedParabolicRoofSolid` add one
explicit IGES type124 placement to the previously verified parabolic-roof
family. The matrix is a proper signed-coordinate permutation (determinant +1)
with finite translation. Every required transformed coefficient and inverse
vertex coordinate must be exactly representable in binary64; no snapping,
normalization, or tolerance-based placement matching occurs.

The 58 original entities remain in local coordinates except the eight type502
vertices, which are authored in world coordinates. A single terminal type124
entity, ID117, is referenced by all six type128 supports and all twelve
model-space type126 curves. The 24 owner UV curves and every topology entity
have zero transform pointers. The reader rejects missing, independently placed,
foreign, or UV transforms and all unclaimed geometry. It retains original local
source IDs, index permutations, labels/subscripts and the explicit placement.
The `Local` field is explicitly local geometry; `Solid` is the world material.

The local graph is validated by the role-independent exact roof reader. Each
world support is then rebuilt from exact transformed controls, sewn with the
original rectangular parameter charts, and subjected to the complete global
spline-rectangle embedding proof. This checks world topology and material,
not merely pointwise samples. Original curve-to-owner identities transfer under
the same injective affine map. Reflections, general rotations/scales/shears,
multiple transforms, and arbitrary chart reparameterizations remain unsupported.
Limits are 64 KiB, 1024 records, 59 entities, 4096 parameter values and 104 controls;
caller limits may tighten these. Writers validate before emitting bytes.

## Independent interoperability

`tools/occtref/author_iges_placed_roof.py` independently authors a mapped roof
with world map `(x,y,z) -> (z+7,x-4,y+2)`. Its local origin is (.25,-2,3),
width/depth/height/rise are 1. Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9`.
Ordinary IGES TransferRoots and BRepCheck accept the independent source and Go
output as one solid with six faces. Independent expectations are volume 4/3,
area `20/3 + sqrt(5)/2 + asinh(2)/4`, centroid (10.7,-3.1875,.5).
Numerical OCCT quadrature is compared within 1e-8 for area; acceptance of the
source graph uses exact arithmetic, not that tolerance.

The same author's `--root-only` mode retains a negative fixture:
`placed-parabolic-roof.igs`. OCCT silently ignores a type124 matrix attached
only to the 186 solid root and reports the unplaced center (.8125,-1.5,3.7).
This API deliberately rejects root-only placement rather than claiming portable
semantics for it. The positive fixture is `leaf-placed-parabolic-roof.igs`.
Run optional live tests with `PLAINSOLID_OCCT_IGES_SOLID` pointing to the
`iges_topology_solid.cxx` oracle; no production OCCT dependency is introduced.

# Independent cavity mass tensors

Pinned OCCT source 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build cylinder_torus_cavity_moments.cxx with the non-STEP flags in
cylinder_torus_cavity_distance.md / torus_cylinder_separated.md. Set
PLAINSOLID_OCCT_CAVITY_MOMENTS and run TestCylinderTorusCavityMoments.

Original OCCT radius8 cylinder caps[-3,4] or[-4,3] minus original torus5/2,
placed with X=(3/5,0,-4/5), Z=(4/5,0,3/5), translation=(1/10,-2/5,3/8).
Compare volume, all world centroid coordinates and every central inertia tensor
entry. The1e-6 absolute comparator tolerance belongs only to OCCT floating
integration; native output uses rigorous rational intervals and exact pi/pi²
coefficients for raw local moments. Exact coefficient tests independently check
V=448pi-40pi², Fz=±224pi, Qxx=Qyy=7168pi-560pi² and torus Qzz=40pi².
Source and world arithmetic limits are independently checked and fail atomically.

The oracle also checks BRepBndLib::AddOptimal without triangulation or shape
 tolerance against the exact cylinder support extrema for the oblique pose,
for both signed cap placements (1e-6 numeric comparator threshold).
TestCylinderTorusCavityAffineRange independently checks those exact rational
extrema and a sheared row with irrational radial support, plus budget/singular
matrix rejection. The source witness family is retained symbolically; it is
not a rounded extremum vertex or a transformed native topology claim.

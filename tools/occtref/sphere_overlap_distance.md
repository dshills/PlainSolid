# Independent overlapping spherical Boolean nearest reference

Pinned OCCT revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Original source archive `/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

The standalone reference independently constructs original radius-5 spheres at
(-3,0,0) and (3,0,0), then OCCT Fuse/Common/Cut. Each must be valid and have
one solid. It receives no PlainSolid topology or approximations. Boundary-face
compounds avoid zero solid distance for interior queries. The intersection
circle is independently x=0, y²+z²=16.

| Operation | Query | Exact squared distance | Nearest foot |
|---|---|---|---|
| Union | (0,9/4,0) | 49/16 | (0,4,0) |
| Common | (0,36/5,0) | 256/25 | (0,4,0) |
| Difference | (3,9/2,0) | 37/4 | (0,4,0) |
| Union | (0,36/5,0) | 196/25 | two retained spherical feet |
| Common | (0,9/4,0) | 25/16 | two retained spherical feet |
| Difference | (0,9/4,0) | 25/16 | retained cutter spherical foot |

The first three reject both full-sphere radial minima because those points are
buried by the Boolean. The common circle foot is isolated and belongs to both
original faces. OCCT extrema search uses 1e-10; comparisons allow
3e-7·max(1,squared distance), and compare the isolated foot within 1e-6.
Solution counts establish existence only, not exhaustive multiplicity.

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_overlap_distance.cxx -o /tmp/plainsolid-sphere-overlap-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_OVERLAP=/tmp/plainsolid-sphere-overlap-oracle \
go test ./internal/occtcheck -run '^TestOCCTSphereOverlapNearest$' -count=1
```

The exact query is deliberately limited to coordinate-axis center displacement,
rational positive radial norms and rational intersection radius. It rejects
unresolved radicals and axis/center families. Native sewing tolerances remain
independent from the ideal original-sphere query proof.

# Independent separated sphere union distance reference

Source is pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9,
archive `/tmp/plainsolid-occt-4dbfdcc.tar.gz` SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The standalone C++ builds two original OCCT spheres and Fuse, without importing
PlainSolid geometry, samples, or topology. Radii 2 and 1 at (0,0,0)/(6,0,0)
give two solids, total volume 12π and area 20π. Nearest queries target a face
compound so points inside material do not get solid-distance zero.
A second equal-radius 2 pair at distance 4 exercises external contact and a
nonzero irrational-distance tie. This is a material-boundary comparison, not
permission to construct a manifold native tangent union.

Build with the pinned installation:

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_union_distance.cxx -o /tmp/plainsolid-sphere-union-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_UNION=/tmp/plainsolid-sphere-union-oracle \
go test ./internal/occtcheck -run '^TestOCCTSphereSeparatedUnionNearest$' -count=1
```

OCCT extrema uses 1e-10 search precision; Go comparison allows
3e-7·max(1,squared distance) for finite precision. OCCT solution count is only
checked for existence; it is not a claim of exhaustive minimizers. Exact Go
center queries retain the complete original spherical family. Contact at a
shared foot retains both original owners. Other exact tests independently
cover sub-float ordering and rejection one ULP inside the overlap threshold.

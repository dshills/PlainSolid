# Independent finite spherical Boolean line reference

Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`, source archive
`/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The standalone program independently constructs R5 spheres centered(0,0,0)
and(2,2,1), performs Fuse/Common/Cut and checks each body with BRepCheck.
It intersects their actual retained faces using IntCurvesFace_ShapeIntersector,
and independently classifies an interval midpoint as inside the result.

The authored line is origin(1/3,1/3,1/6), direction(2/9,2/9,1/9), raw parameter
range[-30,30]. OCCT uses a normalized direction then converts physical distance
back to original t by multiplying by3. Exact expected intervals are:

| Operation | Forward t interval | Endpoint original sphere owners |
|---|---|---|
| Union | [-33/2,45/2] | A,B |
| Common | [-15/2,27/2] | B,A |
| Cut | [-33/2,-15/2] | A,B |

The reversed-direction fixtures negate/reorder the parameters and swap endpoint
owners. OCCT numerical input uses double representations of the independently
authored rational line; search tolerance is1e-10 and endpoint comparison allows
1e-7 in raw t. Duplicate reference face crossings within1e-8 are consolidated
only in these well-separated oracle fixtures. This is not the exact Go root
identity rule. No PlainSolid geometry/mesh is supplied to OCCT.

The exact Go regressions additionally cover strict and tangent cavities,
isolated shared contact, disjoint components, containment, coincident supports,
empty regularized material, interior singleton ranges, irrational endpoints,
and sub-float range/tangency distinctions. Those cases are not claimed to have
native manifold OCCT Boolean validation by this oracle.

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_pair_rational_line.cxx -o /tmp/plainsolid-sphere-line-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_LINE=/tmp/plainsolid-sphere-line-oracle \
go test ./internal/occtcheck -run '^TestOCCTSpherePairRationalLine$' -count=1
```

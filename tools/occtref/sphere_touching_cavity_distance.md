# Internally tangent cavity analytical boundary reference

Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`, archive
`/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

This oracle builds independent complete OCCT sphere surfaces and a compound of
**faces**, not a cavity solid. For R5 centered0 and r1 centered(4,0,0), both full
surfaces are exactly the analytical boundary of the regularized difference.
They share the singular point(5,0,0). No valid manifold BRep claim is made;
PlainSolid's native tangent-cavity constructor still rejects this geometry.

Queries(4.5,0,0), (5,0,0), (6,0,0) have one physical minimum at the contact and
both original owners. At(4,0,0), the whole inner sphere is a minimum family;
the outer sphere also attains the same distance at the contact. At
(20/7,15/7,0), the exact minimum squared is100/49 with distinct feet(4,3,0)
and(60/17,15/17,0). The contact is not nearest. Further queries cover a strict
inner winner and the outer center. A second original pair, R6 at(2,-1,3), r1
at(5,3,3), has exact contact(28/5,19/5,3); query(53/10,17/5,3) has squared
minimum1/4 at that same two-owner point.

OCCT extrema uses1e-10 search precision. Comparisons allow3e-7·max(1,d²) for
finite precision and1e-6 in isolated foot coordinates. No solution count proves
infinite family completeness or exact owner multiplicity; those follow from
exact full-support comparison and independent contact membership in Go.

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_touching_cavity_distance.cxx -o /tmp/plainsolid-sphere-touching-cavity-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_TOUCHING_CAVITY=/tmp/plainsolid-sphere-touching-cavity-oracle \
go test ./internal/occtcheck -run '^TestOCCTSphereTouchingCavityNearest$' -count=1
```

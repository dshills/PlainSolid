# Independent reduced-sphere nearest reference

Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`; source archive
`/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The standalone C++ independently makes two original spheres and applies
Fuse/Common/Cut. Seven nonempty reductions must yield one valid solid; external
contact Common and identical Cut must have no solid. No PlainSolid geometry is
read. Face compounds provide boundary distance even at a sphere center.

Positive cases include strict containment Union, internally tangent Union and
Common, coincident Union and Common, externally tangent Cut and disjoint Cut.
Exact distances are checked at the contact, sphere center and an irrational
radial-norm point. OCCT search precision is 1e-10 and comparison allowance is
3e-7·max(1,squared distance). Representatives do not establish completeness of a
center family; the exact spherical proof does.

Original-owner policy follows the existing regularized material classifier:
Union keeps the containing sphere; Common keeps the contained sphere. A removed
redundant support does not own a point merely because it touches the retained
boundary. An externally tangent subtraction keeps only the stock owner.
Coincident Union/Common retains both original owners of the same full support,
which maps to one native face. Transverse shared rims retain both owners through
the separate transverse proof. Internally tangent cavities are not single-sphere
reductions and remain rejected by this nearest path. Empty regularized results
return no nearest witness.

```sh
c++ -std=c++17 -O2 -I/tmp/plainsolid-occt-build/include/opencascade \
-L/tmp/plainsolid-occt-build/mac64/clang/lib \
-Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
tools/occtref/sphere_single_distance.cxx -o /tmp/plainsolid-sphere-single-oracle \
-lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_SPHERE_SINGLE=/tmp/plainsolid-sphere-single-oracle \
go test ./internal/occtcheck -run '^TestOCCTSphereSingleNearest$' -count=1
```

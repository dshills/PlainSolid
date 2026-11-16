# Strict IGES import of conical rolling rims and an eccentric bore

`ReadConicalRimFilletBore` imports one model-only type 186 solid with
an ascending rational type 128 cone, two original analytic type 198
toroids, two type 190 cap planes and an eccentric type 192 through-bore.
The strict proof authenticates all original circular rims, the
acute and obtuse rolling meridians, two straight seam generators,
and their original directed loop uses. The native construction proves
cone–toroid tangency and separation from the bore. The reader retains
the original unit, source bytes, directory IDs, and one-based edge and
vertex list ownership. Added cardinal vertices subdivide complete
source circles into exact native quadrants.

The pinned OCCT author makes the conical stock, both outer fillets,
and the diagonal bore independently. Its default IGES transfer
provides the full group, topology and analytic source graph. The
exact carrier selects a rational cone control net in the OCCT writer
model and supplies the signed-coordinate frames of the rim and
meridian circles and toroid reference directions. The analytic
torus and bore axes, all six shared vertex-list points, finite model
edge endpoints, face ownership and original loop senses are preserved.
Both variants are serialized by OCCT and reproduce their directory
and parameter sections byte for byte.

The default source has a type 194 semi-angle of `53.130102354`
degrees. Its tangent differs from the exact original cone slope
`4/3`. The original lower type 124 meridian placement also has a
nonzero `-5.551115123E-17` component that takes the original type 100
arc endpoint off the shared seam plane. The exact-net control with
that original coefficient restored is rejected independently.

Pinned OCCT reads the exact source with positive signed volume. All
six original vertices, six full circles, two meridian arcs and two
seams, both cap holes and both stock and bore axes survive transfer.
The native solid serializes identically to the separately authored
exact STEP case.

The returned material exposes `TessellateCertified`. It reuses
the authenticated stock, fillet radii and bore from the original graph.
The mesh certificate covers both deflection directions and records the
original native owner of every triangle.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/author_iges_conical_rim_fillet_bore.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDEIGES -lTKXSBase -lTKFillet -lTKBool \
  -lTKPrim -lTKBO -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/author-iges-conical-rim-fillet-bore"
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/read_step_conical_rim_fillet_bore.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKDEIGES -lTKXSBase -lTKBRep -lTKTopAlgo \
  -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/read-conical-rim-fillet-bore"
PLAINSOLID_OCCT_CONICAL_RIM_BORE_IGES_AUTHOR="$PWD/author-iges-conical-rim-fillet-bore" \
PLAINSOLID_OCCT_CONICAL_RIM_BORE_IGES_READER="$PWD/read-conical-rim-fillet-bore" \
  go test -race ./exchange/iges -run '^(TestReadConicalRimFilletBoreIGES.*|TestConicalRimFilletBorePinnedIGESAuthor)$' -count=1 -v
```

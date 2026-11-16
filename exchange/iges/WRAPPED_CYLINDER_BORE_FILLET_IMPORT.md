# Strict model-only IGES import for circular bore rim fillets

`ReadWrappedCylinderBoreRimFillet` accepts one exact model-only type 186
solid, optionally owned by one type 402 group. It verifies both analytic
cylinders and annular caps, then matches the type 198 inside-quarter
toroids with the original model edge and loop graph. The type 100 circular
curves may use exact signed-coordinate type 124 placements. Every source
coedge must have an opposite effective owner after the type 514 face
orientation is applied.

The result keeps the original file units, directory and list identities,
and source bytes. A full source circle becomes four exact native quadrant
edges whose receipts all point to the original type 100 curve.

The pinned independent positives come from OCCT's Boolean Cut and rim
fillet builders at stock Y=64 and bore Y=66. Its IGES model-only setting
preserves their exact shifted seams in all three fillet selections.
The unshifted upper-only control contains vertices and a generator at
Y=-1.530808499E-15 while the outer cylinder is centered at Y=0.
The strict reader rejects that original and the negative tests that
modify the lower shoulder owner, the meridian frame, or the material group.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/author_iges_eccentric_bore_rim_fillets.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDEIGES -lTKXSBase -lTKFillet -lTKBool -lTKBO \
  -lTKPrim -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/author-iges-eccentric-bore-rim-fillets"
"$PWD/author-iges-eccentric-bore-rim-fillets" both "$PWD/occt-paired.igs" shifted
go test ./exchange/iges -run '^TestReadWrappedCylinderBoreRimFillet' -count=1 -v
```

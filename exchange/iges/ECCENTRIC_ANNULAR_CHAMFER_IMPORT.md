# Strict IGES import of four eccentric annular rim chamfers

`ReadEccentricAnnularChamfer` imports a single model-only type 186 body
with two annular caps, six rational type 128 walls, eight full type 100
rim or shoulder circles, and six finite type 110 meridians. It accepts
the original OCCT one-solid group, preserves file coordinates and units,
and returns the original IGES records. The result maps all face owners,
curve directory IDs, and one-based edge and vertex list indices to the
native boundary. A full original circle supplies four exact native
quadrants. An added cardinal vertex has original list index zero.

The input wall proof checks every knot, homogeneous weight, pole, and
finite bound. Exact quarter-circle weights and the original pole rows
establish circular sections of an affine-radius meridian. Each chart
retains the sign of its original meridian. Both cap holes, the complete
directed rim and seam incidences, and all stations of both eccentric
wall chains must agree with the reconstructed native boundary.

The pinned reference author cuts two eccentric cones and creates four
rim chamfers with `BRepFilletAPI_MakeChamfer`. The IGES writer transfers
that original body. The author selects exact rational nets and endpoint
segments inside OCCT's IGES model while preserving the original axes,
analytic circles, vertex list, and directed ownership. A second file
keeps the default type 194 angles and correspondingly rounded author
seams. The default central outer cone angle is `14.036243468` degrees.
Its tangent differs from the exact stock slope `1/4`. Its original
type 110 seam also stores nonzero Y coordinates while both of its
vertices have Y zero. The strict reader rejects that file.

Pinned OCCT reopens both the independent exact original and the
quadrant-subdivided native re-export with positive signed volume. That
check compares all six individual bands, both cap wires, and every
represented vertex.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/eccentric_annular_chamfer_exact_iges.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDEIGES -lTKXSBase -lTKFillet -lTKBool -lTKPrim -lTKBO \
  -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/occt-eccentric-annular-exact-iges-author"
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/read_eccentric_annular_chamfer.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKDEIGES -lTKXSBase \
  -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/read-eccentric-annular-chamfer"
PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IGES_EXACT_AUTHOR="$PWD/occt-eccentric-annular-exact-iges-author" \
PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IGES_IMPORT_READER="$PWD/read-eccentric-annular-chamfer" \
  go test -race ./exchange/iges -run '^(TestReadEccentricAnnularChamferIGES.*|TestEccentricAnnularChamferPinnedIGESAuthor)$' -count=1 -v
```

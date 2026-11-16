# Exact neutral transfer of an eccentric annular frustum with four chamfers

The STEP and IGES `WriteEccentricAnnularChamfer` functions accept a native
four-rim-chamfered annular frustum. They verify both independently placed
circular wall chains, both original annular cap wires and strict nesting at
every station. The three exact slope laws per axis recover the central
tapered stock and both material-removing end chamfers. The writers preserve
every original band, seam, cardinal vertex and circular quadrant.

STEP uses `SURFACE_OF_REVOLUTION` with exact model-line generators. IGES
retains the exact rational type 128 supports, with their original knots
and weights. In both formats the boundary curves are analytic circles and
lines with the original model vertices. This represents slopes such as
`1/4` without needing a finite decimal for their cone semi-angle.

The pinned OCCT author independently cuts and chamfers aligned and
diagonally offset bores. The comparator checks the matching eight cap
and shoulder circles against the two reopened bodies and all ten original
native wall partitions. It also checks positive signed volume, area, two
wires per cap, all 48 model vertices, and all 58 original edge incidences.
The oracle accepts OCCT's mathematically equivalent periodic conversion
of the IGES supports and checks the resulting poles, weights and knots.

The OCCT author's model-only cone literals for the central outer slope
are `0.244978663127` radians in STEP and `14.036243468` degrees in IGES.
Exact sine and cosine enclosures show that neither has tangent `1/4`,
which the original shoulder circles require. The strict reader requires source supports that encode the slope exactly.

`step.ReadEccentricAnnularChamfer` accepts an independently authored
model-only AP214 body with six bounded revolution supports. The pinned
`eccentric_annular_chamfer_exact_step.cxx` author runs
`BRepFilletAPI_MakeChamfer` and transfers the result through
`STEPControl_Writer`. It selects exact two-pole degree-one generator and
seam representations in the writer's STEP model before serialization.
The original axes, face senses, vertices, circle curves, paired seam uses,
and annular wires come from the OCCT-translated body. The same author
also writes its default cone-angle sample as a strict rejection control.
Both fixtures can be reproduced byte for byte from the DATA section onward.

The strict reader derives each source band from its original generator,
proves exact rim and seam incidence, and verifies continuous wall chains
and strict inner/outer nesting. It preserves the original unit and maps
each native face, edge and original vertex to its source entity. New
cardinal vertices subdivide complete original circles into exact native
quadrants. Pinned readback checks both the independent source and the
canonical STEP re-export from the native solid. This independent import
covers the full-rim two-pole generator dialect.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/read_eccentric_annular_chamfer.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKDEIGES -lTKXSBase \
  -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/read-eccentric-annular-chamfer"
PLAINSOLID_OCCT_ECCENTRIC_RIM_CHAMFER_AUTHOR="$OCCT_CHAMFER_AUTHOR" \
PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_TRANSFER="$PWD/read-eccentric-annular-chamfer" \
  go test ./exchange -run '^TestEccentricAnnularChamferNeutralTransfer$' -count=1 -v
```

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" tools/occtref/eccentric_annular_chamfer_exact_step.cxx \
  -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" -lTKDESTEP -lTKXSBase -lTKFillet -lTKBool -lTKPrim -lTKBO \
  -lTKBRep -lTKTopAlgo -lTKGeomBase -lTKGeomAlgo -lTKG3d -lTKG2d -lTKMath -lTKernel \
  -o "$PWD/occt-eccentric-annular-exact-author"
PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_EXACT_AUTHOR="$PWD/occt-eccentric-annular-exact-author" \
PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IMPORT_READER="$PWD/read-eccentric-annular-chamfer" \
  go test ./exchange/step -run '^(TestReadEccentricAnnularChamferSTEP.*|TestEccentricAnnularChamferPinnedSTEPAuthor)$' -count=1 -v
```

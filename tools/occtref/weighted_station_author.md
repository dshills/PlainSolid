# Independently authored weighted station STEP

`weighted_station_author.cxx` builds original rational B-spline supports directly
in pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`, sews the bands and plane
caps, and optionally performs a real `BRepAlgoAPI_Cut`. It does not read a Go
mesh, native recipe, STEP file or generated topology. The emitted STEP is fed
unchanged into the strict weighted station readers.

The profile has center (1/4,−1/2), the nine standard square-quadrant poles,
weights `[1,1/2,1,3/4,1,3/4,1,3/4,1]` and distinct knots `[-2,-1,1,4,8]`.
Outer stations have heights `[0,1,3]`, scales `[3,1,2]`; inner stations have
heights `[0,2,3]`, scales `[1/2,3/4,1/2]`. All original support coefficients are
dyadic. OCCT authors both wall and planar cap pcurves, whose complete exact
represented correspondence is independently checked by the Go reader. No
special payload, test-side geometry rewrite or tolerance widening is used.

Build against the pinned source-build include and library directories:

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" -L"$OCCT_LIB" \
  -Wl,-rpath,"$OCCT_LIB" tools/occtref/weighted_station_author.cxx \
  -o /tmp/weighted-station-author \
  -lTKDESTEP -lTKXSBase -lTKXCAF -lTKCAF -lTKLCAF -lTKBO -lTKPrim \
  -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_WEIGHTED_STATION_AUTHOR=/tmp/weighted-station-author \
  go test ./exchange/step -run '^TestIndependentWeightedStationSTEP$' -count=1
```

The executable arguments are `output.step output.json filled|bore`. It verifies
BRep validity, one solid and one shell before emitting original mass properties.
The test checks exact original support data and strict native reconstruction,
canonical deterministic reexport, and independent Green-area/station-integral
volume and centroid (1e-6 / 1e-7 numeric comparison tolerances). With the standard
`PLAINSOLID_OCCT_ORACLE` and `PLAINSOLID_OCCT_STEP=1` environment additionally set,
it transfers the canonical Go reexport back to OCCT and compares original vs
readback volume and all centroid coordinates.

Qualified original volumes are approximately 28.0579957962 (filled) and
24.3558991065 (bore); axial centroids are 71/54 and 383/300 respectively.
The exact current readers already accept this independently authored dialect;
no production importer change is needed. This does not assert arbitrary
weighted NURBS shell or arbitrary OCCT exporter-convention acceptance.

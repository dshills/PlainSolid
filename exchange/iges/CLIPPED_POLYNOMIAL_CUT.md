# Source-backed clipped Cut through IGES

`WriteClippedPolynomialCut` writes one type 402 form 1 group whose type 186
solids follow the source receipt's component order. Each component owns its
type 502 vertex list, type 504 edge list, type 128 surface supports, and both
type 126 parameter-space owners of each edge. The source contact coordinates
are exact decimals in their original, separate vertex records.
`VerifyClippedPolynomialCutIGES` authenticates this graph and its global and
directory records against the original native receipt. Start-section
annotations may vary.

The tests use the same three- and four-component source receipts as the STEP
transfer. Their explicit pole tolerance is `1e-14` in the stored model and UV
coordinates. It bounds rounding of derived interior Bézier poles. The
serialized vertices, endpoints, weights and original support data are exact.

## Independent transfer

Build the test reader against the repository's pinned OpenCascade source
revision `4dbfdcc448c0b3e565488afcd626c84483172db9`. Set the include and
library directories to the corresponding OCCT build.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" \
  exchange/iges/testdata/occt_clipped_polynomial_cut.cxx \
  -o /tmp/plainsolid-occt-clipped-cut-iges \
  -lTKDEIGES -lTKXSBase -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKMath -lTKernel

PLAINSOLID_OCCT_CLIPPED_CUT_IGES=/tmp/plainsolid-occt-clipped-cut-iges \
  go test -race ./exchange/iges -run '^TestWriteClippedPolynomialCut$' -count=1 -v
```

The reader validates the transferred BRep, reports each solid's face and edge
counts, and extracts its original exact vertex coordinates. The aggregate
indexed vertex count equals the sum of the component counts, including the
separate vertex records at the two contact points.

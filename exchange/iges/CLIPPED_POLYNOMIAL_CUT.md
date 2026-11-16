# Source-backed clipped Cut through IGES

`WriteClippedPolynomialCut` writes one type 402 form 1 group whose type 186
solids follow the source receipt's component order. Each component owns its
type 502 vertex list, type 504 edge list, type 128 surface supports, and both
type 126 parameter-space owners of each edge. The source contact coordinates
are exact decimals in their original, separate vertex records.
`VerifyClippedPolynomialCutIGES` authenticates this graph and its global and
directory records against the original native receipt. Start-section
annotations may vary.

`WriteClippedPolynomialThreeTransverseCut` and its corresponding verifier
use the same group for the source-proved three-crossing family. Each of the
three point contacts keeps its two component-local vertex references.

The tests use the same source receipts as the STEP transfer. Their explicit
pole tolerance is `1e-14` in the stored model and UV coordinates. It bounds
rounding of derived interior Bézier poles. The serialized vertices,
endpoints, weights and original support data are exact.

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
  go test -race ./exchange/iges -run '^TestWriteClippedPolynomial(Cut|ThreeTransverseCut)$' -count=1 -v
```

The reader validates the transferred BRep, reports each solid's face and edge
counts, and extracts its original exact vertex coordinates. The aggregate
indexed vertex count equals the sum of the component counts, including the
separate vertex records at the original contacts. The two-contact fixtures
reopen with 3 and 4 solids, respectively. The three-contact fixtures reopen
with 4 solids and 29 vertices, and 5 solids and 26 vertices. The verifier
rejects a third-contact edge redirected to the other component's vertex
record at the same exact coordinate.

The variable-count Cut writer also accepts one transverse crossing. Its
fixture retains three separate solids, 14 faces, 18 indexed vertices
and positive volume in each original component. To qualify both neutral
formats, compile the same component reader with
`-DPLAINSOLID_OCCT_STEP_COMPONENT_READER` and `-lTKDESTEP` instead of
`-lTKDEIGES`, then run:

```sh
PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS=/tmp/plainsolid-occt-clipped-cut-step \
PLAINSOLID_OCCT_CLIPPED_CUT_IGES=/tmp/plainsolid-occt-clipped-cut-iges \
  go test ./exchange -run '^TestVariableTransverseClippedCutExchange$' -count=1 -v
```

The exact native implementation accepts up to five transverse crossings.
STEP and IGES reject counts above three before writing records. The
[readback evidence](../../docs/evidence/variable-transverse-cut-transfer-2026-11-15.json)
shows why both topology and signed per-body properties matter: the default
pinned readers considered the four- and five-contact probe shapes valid,
but reoriented some of their thinnest components and returned negative
volume. The original native components have strictly positive volume.

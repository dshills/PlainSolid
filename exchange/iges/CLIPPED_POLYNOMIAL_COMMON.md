# Source-backed clipped Common through IGES

`WriteClippedPolynomialCommon` transfers the proved Common of two
unequal-footprint polynomial stocks with two transverse crossings or a
cubic crossing and quadratic tangency. Its single type 186 solid retains
the original roof and base spline data,
the bounded planar wall charts with their original parameter clocks,
and the two signed type 126 pcurve owners for each shared model edge.
`VerifyClippedPolynomialCommonIGES` authenticates the topology, supports,
owners, exact vertices, physical units and transfer settings against the
native receipt. Start-section annotations may vary.

`WriteClippedPolynomialThreeTransverseCommon` and its verifier use the same
single-solid transfer for the three-crossing family. Each junction keeps one
exact vertex record and all four original incident model edges, with the
signed owner pcurves in their original charts.

The transverse fixture uses millimetres and an explicit `1e-14` maximum
coordinate error for derived interior model and pcurve poles. Its type
502 vertices, curve endpoints, weights and original support data keep
exact decimal values.

## Independent transfer

The [source-backed Cut instructions](CLIPPED_POLYNOMIAL_CUT.md) give the
build command for the shared test reader against pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9`. Run the Common fixture with
that same executable.

```sh
PLAINSOLID_OCCT_CLIPPED_COMMON_IGES=/tmp/plainsolid-occt-clipped-cut-iges \
  go test -race ./exchange/iges -run '^TestWriteClippedPolynomial(ThreeTransverseCommon|Common)$' -count=1 -v
```

The transverse readback has 14 faces, 32 edges and all 20 original
vertices. It measures volume 11.970411420799792 mm³ and area
32.983255727388539 mm². The mixed Common splits one original cusp lens
where its two branches already have proved midpoint vertices. The seam
is a constant-station curve on the unchanged roof chart. The independent
reader accepts the resulting 15-face solid with 33 edges and the same
20 vertices, with volume 11.970703124180975 mm³ and area
31.655122029933921 mm². Both tip orders use the same split. The old
unsplit wire fails independent validation and canonical authentication;
the transverse stream is byte-identical.

Both three-crossing fixtures reopen as one valid solid with 15 faces,
36 edges and 23 exact vertices. The independent reader matches every edge
to its original endpoints and checks volume and area for both layouts.

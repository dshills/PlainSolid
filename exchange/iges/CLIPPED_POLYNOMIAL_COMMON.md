# Source-backed transverse clipped Common through IGES

`WriteClippedPolynomialCommon` transfers the proved Common of two
unequal-footprint polynomial stocks with two transverse crossings. Its
single type 186 solid retains the original roof and base spline data,
the bounded planar wall charts with their original parameter clocks,
and the two signed type 126 pcurve owners for each shared model edge.
`VerifyClippedPolynomialCommonIGES` authenticates the topology, supports,
owners, exact vertices, physical units and transfer settings against the
native receipt. Start-section annotations may vary.

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
  go test -race ./exchange/iges -run '^TestWriteClippedPolynomialCommon$' -count=1 -v
```

The reader qualifies one valid solid with 14 faces, 32 edges and all 20
original vertices. It measures volume 11.970411420799792 mm³ and area
32.983255727388539 mm². The mixed higher-order native fixture returns
`geom.ErrDomain` before export or verification. A raw analytic transfer
of that comparator gives an unorientable face and a self-intersecting
wire in the same pinned OCCT reader.

# Original rational affine owner pcurves

`ReadRationalUVNURBSPlanarAssembly` is an additive source-authenticated route for
genuine quadratic rational UV lines on the qualified two-axis rational affine
surface family. It preserves all earlier polynomial-UV and rational-support
readers. `WriteRationalUVNURBSPlanarAssemblySource` re-proves and exports complete
original STEP. Source-bound native JSON uses distinct version-one tag
`plainsolid-step-rational-uv-nurbs-planar-assembly`.

The actual original complex curve record must contain exactly the expected
BOUNDED_CURVE, B_SPLINE_CURVE, B_SPLINE_CURVE_WITH_KNOTS, CURVE,
GEOMETRIC_REPRESENTATION_ITEM, RATIONAL_B_SPLINE_CURVE and REPRESENTATION_ITEM
components. It has degree two, three exclusively owned two-dimensional poles,
positive nonconstant weights and one clamped knot interval. Every raw rational
coordinate and weight is retained in copied evidence.

For normalized original parameter t, the reader proves the complete homogeneous
vector identity `N(t) = W(t)*(P0+t*(P2-P0))` by comparing every cubic Bernstein
coefficient. Strictly positive weights guarantee W>0 on the closed interval.
This proves exact same-parameter affine evaluation, not just a coincident line
or its endpoints. The original interval is unchanged. A private polynomial
midpoint representation is used only after proof; original rational poles and
weights remain in source and `PCurves()` evidence.

The entire original unique ownership chain is required:
curve → DEFINITIONAL_REPRESENTATION → PCURVE → SURFACE_CURVE → EDGE_CURVE.
The prior strict material proof then authenticates both original owner pcurves,
model-edge equality over their domains, sense, face-support association,
directed topology, exact native material, product reuse, poses and appearance.
A coherent affine pcurve on the wrong line, a different parameter speed, a
reversed parameterization or a wrong support still fails. The polynomial proof
view is not exposed as original rational geometry.

Native reload re-proves original source once and compares every stored support,
UV basis, model-edge owner, occurrence, appearance and native claim. Limits are
1 MiB original source, 4096 entities, 65536 values, depth 16, at most 128 original
rational supports and 512 rational pcurves. Curves have fixed degree and three
poles; source rational numerators/denominators are bounded to 256 bits and
homogeneous coefficient results to 4096 bits. Exact private midpoint decimal
serialization remains bounded. Native envelopes are capped at 16 MiB. Caller
limits, cancellation, short writes and atomic rejection remain enforced.

## Independent source

The standalone OCCT helper directly replaces each original affine UV line with
a rational quadratic curve: weights `[1,2,3]`, middle Euclidean pole
`(3*P0+P2)/4`, same original knot range. It independently constructs the original
rational 2×2 tensor supports as well. XCAF emits all 48 actual complex rational
pcurve records with exact raw weights and terminating original poles. No Go
geometry tokens are used to author these positives.

Original and native-source re-export transfer independently as three solids /
eighteen faces, V10800, A4860 and centroid `(9.3125,14.5,12.0625)`, with original
shared products and colors. Retained and live original cases use normal and
shifted unequal UV domains. This rational UV milestone has single-span support
and curve bases; its author does not claim a rational internal-knot extension.
Pinned OCCT revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_rational_uv_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-rational-uv-nurbs-planar
PLAINSOLID_OCCT_RATIONAL_UV_NURBS_PLANAR=/tmp/plainsolid-ap242-rational-uv-nurbs-planar \
  go test ./exchange/step -run '^TestRationalUVNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzRationalUVNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Adversaries include changed, zero, negative or almost-equal weights; altered UV
middle poles; a coherently shifted line; wrong knot speed; reversed poles and
weights; duplicate complex components; shared poles; wrong owner support; and
forged native UV coefficients or edge IDs. Original rational-support adversaries
and prior strict-route regressions remain covered. Rational 3D model curves,
genuinely curved UV maps and tolerance-based repairs remain unsupported.

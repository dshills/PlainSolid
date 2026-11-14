# Exact wrapped spline pcurve coefficients

The wrapped tube, cavity, station and fillet readers can normalize a polynomial
B-spline pcurve to a unit isoparametric line only after proving its full-domain
Greville coefficient identity. This is an internal proof view; it does not make
arbitrary spline pcurves or rounded vendor geometry exact.

Original decimal pole coordinates and knots must equal the binary64 coefficients
used by that identity proof. Checking only their converted floating-point values
would admit distinct authored decimals such as `0.50000000000000000001` as `0.5`.
The bounded exact source parser now precedes conversion for both coordinates and
knots, including exponent bounds before rational allocation. Nonrepresentable
source scalars fail closed. No tolerance or snapping is introduced.

`TestWrappedIsoSourceDecimals` and `TestWrappedIsoSourceKnots` mutate an original
OCCT tube fixture below one binary64 ULP. The unchanged fixture remains accepted.
The live original/output transfer check uses the adaptive standalone oracle:

```sh
sh tools/occtref/build_step_frustum_hierarchy.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-frustum-hierarchy-oracle
PLAINSOLID_OCCT_FRUSTUM_HIERARCHY=/tmp/plainsolid-frustum-hierarchy-oracle \
  go test ./exchange/step -run TestWrappedIsoSource -count=1
```

The pinned OCCT source is `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Live transfer independently checks one solid and matching positive volume and
surface area before/after the strict native round trip. Numeric oracle measures
are interoperability evidence; the coefficient identity is exact rational proof.

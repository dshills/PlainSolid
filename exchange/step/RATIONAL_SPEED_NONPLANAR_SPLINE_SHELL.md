# Shared rational boundary speed on original nonplanar material

`ReadRationalSpeedNonplanarSplineShellSource` is a separate exact original-source
route. It authenticates one six-face quadratic polynomial spline material whose
12 model edges and 24 owner pcurves are original rational B-splines. Every curve
is a clamped single-span Bezier on `[0,1]`, model degree 1–2, UV degree one.
Carrier geometry remains genuinely nonplanar and unchanged. Arbitrary rational
trims, internal curve knots, negative weights and mismatched owner speeds are
not admitted.

## Exact parameter proof

For each original curve, raw exact positive weights must satisfy
`w_i = w_0*r^i`. All three curves belonging to a model edge must have the same
`r>0`. At least one edge must have `r != 1`. Common weight scales may differ
and are retained as exact rationals, not normalized through float64.

For polynomial control curve `P` and Bernstein basis of degree `n`,

```
phi(t) = r*t/(1+(r-1)*t)
B_i^n(phi(t)) = r^i*B_i^n(t)/(1+(r-1)*t)^n
sum_i w_i*B_i^n(t) = w_0*(1+(r-1)*t)^n > 0
```

Thus the original rational curve is exactly `P(phi(t))` on its entire original
domain. `phi(0)=0`, `phi(1)=1`, and `phi'(t)=r/(1+(r-1)*t)^2>0`. No endpoint,
interval, orientation or inverse branch is guessed. A common reparameterization
preserves both directed coedge uses and physical material.

The private proof view retains every control point, knot, original ID, carrier
and topology, but uses the polynomial control curves. The existing nonplanar
reader proves continuous model-to-carrier agreement for that view, plus exact
sewing and complete global outward embedding. If `P(s)=S(U(s))` for every `s`,
then the original source satisfies `P(phi(t))=S(U(phi(t)))` for every `t`.
This is the full original model/coedge identity, not a sampled approximation.
Uniquely owned original complex records and model/owner chains are checked
before normalization; both owner ratios are independently bound to the same
model curve and edge.

## Native semantics and limits

The returned native material has **polynomial boundary speed**. Its parameter
values are not claimed identical to original rational edge parameters.
`ParameterMaps()` returns copied original weights, poles, degree, dimensions,
ratio and curve/edge/surface/owner IDs. `PCurves()` retains the normalized
endpoint line data and original IDs; the rational basis is in `ParameterMaps`.
Original STEP is immutable and source re-export preserves it byte for byte.
Generated `WriteSplineShells` output is an explicitly normalized representation.

The separate `plainsolid-step-rational-speed-nonplanar-spline-shell-source`
version 1 native envelope stores source and both geometry/map claims. Reload
re-proves the complete source once and compares native, canonical STEP and
ancestry/weight/ratio claims. It never constructs accepted geometry from stored
native claims alone. Old readers and tags are unchanged.

Limits are 1 MiB source, 2048 entities, 32768 parser values, depth 16, exactly
36 rational curves, raw scalar size 256 bits, and intermediate weight checks
1024 bits. Controls/knots must be losslessly represented; raw rational weight
scales need not be binary64-representable because their exact cancellation and
parameter ratios are proved symbolically. Context, native byte limits and atomic
failure follow the existing source-bound conventions.

## Independent qualification

`tools/occtref/author_ap242_rational_speed_roof.cxx` independently constructs and
sews the actual polynomial roof solid `0<=x<=4, 0<=y<=3,
0<=z<=2+(x/4)^2`, then assigns original model and UV rational weights through
OCCT APIs before export. It authors `r=2` (`write`) and `r=1/2` (`write-half`).
No Go STEP tokens are used. The retained AP242 source has 322 entities; pinned
OCCT revision is `4dbfdcc448c0b3e565488afcd626c84483172db9`.

Both live sources, source-bound native reload/re-export, and generated STEP
transfer as one valid solid with six faces. Analytic volume is 28, centroid
`(15/7,3/2,83/70)`, area `12+56/3+15+3*sqrt(5)+12*asinh(1/2)`.
OCCT measure quadrature uses explicit `1e-12` accuracy; it is an independent
numerical comparison, not the exact acceptance proof. Native edge verification
reports exact zero squared discrepancy in normalized parameterization.
Tests independently evaluate original homogeneous model controls by rational
de Casteljau and compare each owner lift on the original support. Mutations
cover unequal model/owner speed, non-geometric weights, zero/negative weights,
domain, wrong carrier, changed geometry, near decimals, exponent attacks,
orphans and forged native maps. Common raw weight scale `1/10` is preserved
through native reload without float normalization.

```sh
sh tools/occtref/build_author_ap242_rational_speed_roof.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-rational-speed-roof
PLAINSOLID_OCCT_RATIONAL_SPEED_ROOF=/tmp/plainsolid-ap242-rational-speed-roof \
 go test ./exchange/step -run '^TestRationalSpeedNonplanarSplineShell' -count=1
go test -race ./exchange/step -run '^TestRationalSpeedNonplanarSplineShell' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzRationalSpeedNonplanarSplineShellSource$' -fuzztime=40x -parallel=1
```

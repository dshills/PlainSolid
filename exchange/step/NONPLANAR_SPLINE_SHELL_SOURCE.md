# Genuine nonplanar spline material with original spline pcurves

`ReadNonplanarSplineShellSource` adds a bounded real-source representation to the
existing exact rectangular spline material proof: one closed six-face polynomial
spline shell whose 24 original owner pcurves are two-pole degree-one B-splines.
The prior `ReadSplineShells` LINE-pcurve gate is unchanged. This is not a planar
conversion: at least one original support must have exactly noncoplanar control
poles, and the original nonaffine carriers and curved model edges are retained.
The existing generic spline-shell writer emits independently readable generated
STEP from the resulting `Model`.

Original pcurve poles/knots must be exactly representable, and each entire
bounded curve must be `(t,c)` or `(c,t)` on its owning model curve's full original
parameter interval. Exclusive pole and curve ownership, the unique definition /
PCURVE / SURFACE_CURVE / EDGE_CURVE chain, and source support IDs are retained.
Only a private proof view replaces the proven UV spline with its identical
analytic line; model curves and support geometry are not flattened or fitted.
Generated private line records are rebased explicitly before proof parsing.

The existing exact rectangular-face decoder compares complete original model
curves with support isocurves, verifies endpoints and directed uses, and sews
all original spline rectangles. Complete global spline-rectangle embedding and
outward material proof are mandatory. The qualified fixture additionally has
zero continuous discrepancy on every native edge use. Native source face IDs
follow the original CLOSED_SHELL order, independently of record encounter order.
Original edge IDs and pcurve basis/owner evidence are copied on access.

The source reader bounds one representation, one material body, six original
supports/faces and twelve shared edges. Source geometry must be reachable from
the representation; detached geometry and unknown record families reject.
Polynomial support nets have at most three poles per axis and degrees at most
two; original scalar geometry is checked for exact decimal/binary equality,
including knot values and integer degrees/multiplicities. Rational support,
assembly placement, arbitrary trim loops and tolerance repair are outside this
entry point. Product metadata is preserved in original source, not interpreted
as a new assembly contract. Length units use the existing strict source context.

`WriteNonplanarSplineShellSource` re-proves and emits original bytes. Native JSON
uses distinct version-one tag `plainsolid-step-nonplanar-spline-shell-source`.
Reload re-proves original STEP and compares original role IDs, canonical generated
STEP and actual trimmed native JSON. It never constructs trusted geometry from
serialized claims. Limits include 1 MiB source, 2048 entities, 32768 values,
parse depth 16 and 16 MiB native envelopes, plus existing finite geometry/embedding
limits. Caller limits and cancellation checks apply around bounded proof stages.
All prior APIs and native formats remain unchanged.

## Independently authored curved roof

The standalone OCCT helper constructs six original B-spline patches bounding
`0≤x≤4`, `0≤y≤3`, `0≤z≤2+(x/4)^2`. It sews them in OCCT, independently converts
each actual owner pcurve to a bounded B-spline, and exports AP242. This source is
rejected by the prior LINE-only reader and accepted by the additive route.
The roof and two curved rim model edges are genuinely quadratic; no planar
substitute or Go-generated source tokens author the fixture.

Analytic values are:

* Volume 28.
* Centroid `(15/7,3/2,83/70)`.
* Area `12 + 56/3 + 15 + 3*sqrt(5) + 12*asinh(1/2)`
  = 58.149412499881279.

Original source, source-native-source, and native-generated STEP independently
transfer as one valid solid with six faces and these measures. Direct trimmed
native reload re-proves global embedding. Original-source evidence uses pinned
OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_parabolic_spline_roof.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-parabolic-spline-roof
PLAINSOLID_OCCT_PARABOLIC_SPLINE_ROOF=/tmp/plainsolid-ap242-parabolic-spline-roof \
  go test ./exchange/step -run '^TestNonplanarSplineShell' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzNonplanarSplineShellSource$' \
  -fuzztime=40x -parallel=1
```

Mutations include changed curved roof poles, wrong UV speed/support, near-exact
source decimals, underflow exponents, shared poles, reversed face sense, detached
geometry and forged native/source/ancestry claims. Original record reordering,
source limits, cancellation and bounded decoder fuzzing are covered.

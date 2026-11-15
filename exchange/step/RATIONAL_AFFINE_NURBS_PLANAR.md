# Exact rational affine STEP support

`ReadRationalAffineNURBSPlanarAssembly` admits a separate original-source class:
positive, nonconstant-weight rational tensor supports of degree 2 in U and 1 in
V, with single-span clamped knot intervals. The source has the actual complex
STEP `B_SPLINE_SURFACE` / `B_SPLINE_SURFACE_WITH_KNOTS` /
`RATIONAL_B_SPLINE_SURFACE` records. All seven expected complex components and
their arities are checked; unsupported components or duplicate records reject.
This route requires the existing qualified affine multi-span UV pcurve family
and degree-one original 3D model curves. It does not widen prior readers.

Let the original Euclidean poles be `Pij`, weights `wij`, homogeneous numerator
`N = sum Bij wij Pij`, and denominator `W = sum Bij wij`. With normalized source
parameters `(u,v)`, the reader proves the complete polynomial identity

`N = W * (P00 + u*(P20-P00) + v*(P01-P00))`.

Both sides are expanded in the tensor Bernstein basis of degree (3,2), using
exact rational degree elevation and multiplication by u/v. Every coefficient
in each coordinate is compared. Positive original weights ensure W is strictly
positive throughout the closed parameter rectangle, so division proves the
whole source rational map is exactly affine with the same parameterization.
Corner planarity, endpoint agreement or geometric sampling are not substitutes.
Nonconstant weights are required; a common positive rescaling preserves the
quotient but retains its distinct raw source weights in evidence.

Each pole record must have exclusive support ownership. The surface has one
original face owner; original pcurves, shared model edges, face orientation,
product/occurrence/appearance ownership and globally embedded native material
are authenticated through the existing exact planar proof. Only after the
homogeneous identity succeeds does a private polynomial view use exact midpoint
poles under the original IDs. The original source and rational poles/weights
remain unchanged in the returned assembly and copied `RationalSupports()`
evidence. The private view is not a claim that original rational poles were
polynomial midpoint poles.

`WriteRationalAffineNURBSPlanarAssemblySource` re-proves and emits original bytes.
Native JSON uses distinct version-one tag
`plainsolid-step-rational-affine-nurbs-planar-assembly`; reload re-proves original
STEP once and compares the full native, source-owner and homogeneous evidence.
The original weights, pole IDs, raw rational coordinates, knot intervals and
face IDs are preserved. Existing native tags remain unchanged.

Limits: 1 MiB source, 4096 entities, 65536 values, depth 16, at most 128 rational
supports of six poles each, and the unchanged bounded UV proof. Original scalar
numerators/denominators have at most 256 bits; homogeneous coefficient results
have at most 4096 bits, with fixed-size bounded intermediate products/sums. The
private exact decimal midpoint must also satisfy the existing strict decimal
serialization bound; failure is unsupported, never rounding. Native envelopes
are capped at 16 MiB. Caller limits, cancellation and atomic rejection apply.

## Genuine OCCT positive

The independent helper constructs the original OCCT rational support directly:
weights along U are `[1,2,3]`, repeated in V, and the middle Euclidean pole is
`(3*P0+P2)/4`. The numerator is degree elevated relative to the affine map;
`W=1+2u` and `N=W*P`. Original poles such as 2.25 on a span of 9 are terminating
and survive OCCT STEP writing unchanged. No Go-generated token substitution is
used. OCCT emits the complex rational entities with the original weights;
normalization drift was not observed in this controlled fixture.

Original and native-source re-export independently transfer as three solids /
eighteen faces, V10800, A4860 and centroid `(9.3125,14.5,12.0625)`, preserving
shared products and colors. Live cases include shifted UV domains, mixed-degree
pcurves and nonuniform dyadic pcurve knots. Retained fixtures cover the standard
and mixed pcurve cases. OCCT source revision:
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_rational_affine_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-rational-affine-nurbs-planar
PLAINSOLID_OCCT_RATIONAL_AFFINE_NURBS_PLANAR=/tmp/plainsolid-ap242-rational-affine-nurbs-planar \
  go test ./exchange/step -run '^TestRationalAffineNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzRationalAffineNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Tests reject changed/zero/negative/nearby weights, exponent attacks, altered
interior poles with unchanged corners, almost-affine decimals, incorrect degree
or knot multiplicity, pole sharing, wrong face ownership, stale/native forged
homogeneous evidence, old tags, source limits and cancellation. A positive
common weight scaling checks that raw basis identity is retained separately
from geometric quotient identity. General rational surfaces, rational model
curves and toleranced near-affine repairs remain outside this scope.

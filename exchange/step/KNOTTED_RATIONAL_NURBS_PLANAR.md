# Original rational affine supports with internal knots

`ReadKnottedRationalNURBSPlanarAssembly` admits a distinct bounded original
rational support domain: quadratic in both parameters, four-by-four positive
weighted poles, and exactly one simple internal knot in each axis. Each original
knot vector therefore has multiplicities `(3,1,3)`. Both weight directions must
vary. The surrounding assembly includes the already authenticated rational
quadratic UV curves and continuous straight multi-span polynomial model edges.
All prior single-span APIs and native tags remain strict.

The proof does not replace original knots with a guessed Bernstein net. It
expands the original quadratic Cox–de Boor basis exactly on each of the four
nonempty knot rectangles, using rational arithmetic and the standard zero-term
rule for repeated-knot denominators. On each rectangle, local parameter powers
represent the complete original homogeneous numerator N and denominator W.
Every coefficient of

`N = W * (P00 + normalizedU*(P30-P00) + normalizedV*(P03-P00))`

is compared, through degree three in each local parameter. Original knot
locations determine both the span restriction and the affine parameter change.
Positive original weights and the validated clamped nonnegative spline basis
prove W>0 on the entire closed domain. The polynomial identities include span
endpoints, and simple internal knots give continuity; no samples or rounded
interior poles are used to establish geometry.

Only after all four identities succeed is a private unweighted affine
B-spline net constructed at exact original Greville coordinates, preserving
original knot vectors, support IDs and control-record identities. Existing
model/UV homogeneous and Greville proofs then feed the complete exact directed
material proof. Original rational source, weights, all sixteen poles, both knot
vectors, multiplicities and face-owner IDs remain in copied evidence. The
polynomial proof view is never presented as original source geometry.

Source export re-proves and emits original bytes. Native JSON has distinct
version-one tag `plainsolid-step-knotted-rational-nurbs-planar-assembly`; reload
re-proves original source and checks all original and derived claims. Limits
are 1 MiB source, 4096 entities, 65536 values, depth 16, at most 128 rational
supports, 512 rational UV curves and the bounded model-curve proof. Each support
has sixteen poles and four fixed cells. Original scalars have at most 256
numerator/denominator bits; homogeneous result coefficients at most 4096 bits.
Fixed-degree restriction bounds intermediate work. Private exact decimal
serialization and existing model/UV arithmetic limits remain enforced. Native
envelopes are capped at 16 MiB; caller limits, cancellation and atomic rejection
remain intact.

## Independent inserted-knot source

The standalone author constructs the previously qualified rational tensor in
OCCT and invokes `InsertUKnot` and `InsertVKnot` on that actual rational surface.
XCAF exports the resulting original 4×4 rational control net, not a Go-generated
replacement. Weights include `3/2`, `9/4`, `25/4`; original Euclidean poles such
as `6.3` remain exact terminating source values. No normalization drift occurs
in the retained controlled source. Original rational UV and multi-span 3D model
curves are authored independently in the same body.

Normal and shifted parameter-domain originals are retained. Source and native
source re-export independently transfer as three solids / eighteen faces,
V10800, A4860, centroid `(9.3125,14.5,12.0625)`, preserving shared products and
colors. OCCT source revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_knotted_rational_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-knotted-rational-nurbs-planar
PLAINSOLID_OCCT_KNOTTED_RATIONAL_NURBS_PLANAR=/tmp/plainsolid-ap242-knotted-rational-nurbs-planar \
  go test ./exchange/step -run '^Test(KnottedRationalNURBS|QuadraticOriginalSpanRestriction)' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzKnottedRationalNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Tests independently check the exact restricted basis coefficients on both
uniform spans and partition-of-unity/linear precision on nonuniform spans.
Source adversaries change an interior pole or weight with unchanged corners,
move an internal knot, introduce discontinuity or a false knot declaration,
rewire owners, change model/UV speeds, and forge native knot/basis provenance.
Raw common positive weight scaling is preserved exactly. Existing single-span
routes reject this source. Curved rational geometry, other degree/pole counts,
additional internal knots and toleranced repair remain unsupported.

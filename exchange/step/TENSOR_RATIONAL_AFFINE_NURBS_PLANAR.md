# Two-axis rational affine STEP support

`ReadTensorRationalAffineNURBSPlanarAssembly` is a separate authenticated
original-source route for degree-2×2 rational tensor supports whose strictly
positive weights vary along both U and V. It preserves the degree-2×1 API,
all polynomial readers, and their native format meanings. The new source
writer re-proves and emits original bytes; native JSON has distinct version-one
tag `plainsolid-step-tensor-rational-affine-nurbs-planar-assembly`.

The reader validates the complete seven-component complex rational surface
record, nine exclusively owned Euclidean pole records, positive weights,
single-span clamped knot intervals and original face ownership. It then proves

`N(u,v) = W(u,v) * (P00 + u*(P20-P00) + v*(P02-P00))`

coefficientwise in the degree-3×3 Bernstein basis, where N and W are constructed
from all original weighted poles. Exact degree elevation and multiplication
by each parameter include the mixed interior coefficients. No separability
assumption, sampling, weight normalization or corner-only inference is used.
Strictly positive weights prove W>0 over the whole closed rectangle, making
the rational identity a continuous same-parameter affine-map proof.

Only a private equivalent polynomial tensor is used for existing owner-pcurve,
model-edge, directed topology, occurrence, appearance and embedded native
material authentication. Its polynomial poles are computed exactly from the
proved affine map. Original rational poles, weights, knots and face/support IDs
are retained by `RationalSupports()`, copied evidence and complete original
STEP. The native envelope re-proves original source and compares the complete
homogeneous and native claims; it does not trust cached coefficients.

The surrounding material scope remains the qualified multi-span polynomial UV
family with degree-one original model edges. Both raw weight-axis variations
are required. General curved rational supports, internal rational support knots,
other degrees and rational model curves are outside this route. It does not
accept the older degree-2×1 source, and that reader still rejects degree-2×2.

Limits are unchanged in scale: source 1 MiB / 4096 entities / 65536 values /
depth 16; at most 128 rational supports of nine poles; 256-bit original scalar
numerators/denominators; 4096-bit homogeneous coefficient results and fixed-size
bounded intermediate sums. Private polynomial coordinates must pass exact
finite-decimal serialization limits. Native envelopes are capped at 16 MiB.
Caller limits, cancellation and atomic failure are enforced.

## Independent original OCCT evidence

The standalone author directly constructs the rational OCCT support with
weights `[1,2,3] ⊗ [1,2,3]` and Euclidean quarter-grid poles. Here
`W=(1+2u)(1+2v)`, but acceptance verifies the actual full source identity rather
than relying on this fixture formula. Original STEP retains the full weight
matrix `[[1,2,3],[2,4,6],[3,6,9]]` and terminating pole coordinates unchanged.
No normalization drift occurred in this controlled source.

Original/native-source transfer is independently valid with three solids /
eighteen faces, V10800, A4860, centroid `(9.3125,14.5,12.0625)`, original shared
products and colors. Live variants include shifted unequal UV intervals,
mixed-degree pcurves and nonuniform dyadic pcurve insertion. Two original
fixtures are retained. The helper uses pinned OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_tensor_rational_affine_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-tensor-rational-affine-nurbs-planar
PLAINSOLID_OCCT_TENSOR_RATIONAL_AFFINE_NURBS_PLANAR=/tmp/plainsolid-ap242-tensor-rational-affine-nurbs-planar \
  go test ./exchange/step -run '^TestTensorRationalAffineNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzTensorRationalAffineNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Tests include mixed central weight/pole changes with unchanged corners,
inconsistent far corner, zero/negative/nearby weights, near-affine poles,
incorrect complex degree/knot multiplicity, ownership rewiring, source reorder,
raw positive common weight rescaling, forged native evidence, strict prior-route
rejection, limits, cancellation and bounded fuzzing. The shared source export
retains original rational geometry rather than emitting the private proof view.

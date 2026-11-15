# Exact multi-span affine planar STEP assemblies

`ReadMultispanNURBSPlanarAssembly` is a separate source-authenticated route for
polynomial clamped B-spline supports and UV pcurves of independent degrees 1–8.
At least one internal knot must be present. The existing quadratic and elevated
single-span readers, writers and native tags remain unchanged. This API does
not admit rational weights, curved supports, discontinuous internal knots or
elevated/multi-span three-dimensional model curves.

The full expanded knot vector is validated: distinct knots strictly increase,
end multiplicities equal degree plus one, internal multiplicities lie between
one and degree, and total multiplicity equals pole count plus degree plus one.
For degree `d`, pole `i` has Greville coordinate
`g_i = (t_(i+1) + ... + t_(i+d))/d`. After normalization to the original knot
interval `[a,b]`, `h_i = (g_i-a)/(b-a)`, every original tensor pole must satisfy
`Pij = P00 + h_i*(Pn0-P00) + h_j*(P0m-P00)`; every UV pole must satisfy
`Pi = P0 + h_i*(Pn-P0)`. These are exact rational identities from original
source tokens. Partition of unity and linear precision of the validated
nonnegative polynomial B-spline basis prove the complete affine map on every
span, not just sampled points or knot stations. The two surface directions use
their own degrees, knots and Greville coordinates.

`QUASI_UNIFORM_KNOTS` additionally requires uniform distinct-knot spacing and
simple interior knots; `PIECEWISE_BEZIER_KNOTS` requires degree-fold interior
knots. `UNSPECIFIED` imposes neither extra declaration. Other knot types are
outside this scope. Changed control poles, contradictory knot declarations and
full-multiplicity internal breaks fail closed.

A private equivalent degree-one graph preserves endpoint poles, original entity
IDs and original parameter ranges. Only exclusively owned redundant control
records are removed from that private proof view. The unchanged oblique planar
reader then proves continuous model/pcurve agreement, directed closed material,
product frames, appearance and native embedded geometry. The complete original
STEP remains the returned assembly's source. Evidence copies contain original
face/model-edge/owner IDs, all original poles, degrees, knots and multiplicities.
Pole counts identify active entries of fixed-size arrays; nonzero multiplicity
identifies active knot entries. No original source is rewritten by acceptance.

`WriteMultispanNURBSPlanarAssemblySource` re-proves and exports original bytes.
The distinct native version-one tag
`plainsolid-step-multispan-nurbs-planar-assembly` re-proves original STEP once and
compares all native, topology, appearance and basis claims. Existing tags remain
strict. Limits are 1 MiB original source, 4096 entities, 65536 values, depth 16,
17 poles per direction/curve, 17 distinct knots, and 4096 total processed control
poles. Original rationals have at most 256 numerator/denominator bits; normalized
Greville and affine identity results at most 1024. These fixed degree/count caps
also bound intermediate arithmetic. Caller limits, cancellation and atomic
failure apply.

## Independent qualification

The standalone OCCT author constructs boxes, converts their original surfaces,
elevates bases and inserts knots through OCCT itself before XCAF STEP export.
Retained fixtures cover cubic 5×5 control grids with one internal knot in each
direction; degree-2×3 supports with degree-four pcurves; and nonuniform dyadic
quarter/three-quarter insertion. Live qualification also shifts UV domains.
Unequal original spans are 3 and 6. Shared local products, colors and repeated
occurrences remain original. Source and native-source re-export independently
transfer as three solids / eighteen faces with volume 10800, area 4860 and
centroid `(9.3125,14.5,12.0625)`.

```sh
sh tools/occtref/build_author_ap242_multispan_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-multispan-nurbs-planar
PLAINSOLID_OCCT_MULTISPAN_NURBS_PLANAR=/tmp/plainsolid-ap242-multispan-nurbs-planar \
  go test ./exchange/step -run '^TestMultispanNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzMultispanNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

OCCT source revision: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Tests include changed interior controls with unchanged corners, sub-binary
near-affine changes, invalid knot counts/clamping/discontinuities, false knot
specification, owner rewiring, source reorder, native corruption, old route
rejection, limits, cancellation and bounded decoder fuzzing. This proof does not
extend acceptance to vendor-rounded near-affine sources.

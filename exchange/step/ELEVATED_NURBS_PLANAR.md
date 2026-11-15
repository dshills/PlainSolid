# Exact degree-elevated planar NURBS STEP

`ReadElevatedNURBSPlanarAssembly` is an additive source-authenticated route for
unweighted, single-span clamped polynomial tensor supports and UV pcurves of
independent degrees 1 through 8. It requires at least one elevated support and
one elevated pcurve. The existing quadratic and lower-degree APIs and native
formats are unchanged. This route also admits their qualified quadratic basis;
it does not admit arbitrary curved NURBS, rational weights, or internal knots.

For every support pole, exact original decimal arithmetic proves
`Pij = P00 + (i/du)(Pdu,0−P00) + (j/dv)(P0,dv−P00)`.
Every pcurve pole satisfies `Pi = P0 + (i/d)(Pd−P0)`. These all-pole identities
prove the complete affine polynomial and its original parameterization on the
authored knot intervals. Corner-only planarity is insufficient. Control records
must have exclusive ownership, and each elevated 2D curve must have a unique
PCURVE / SURFACE_CURVE / EDGE_CURVE owner chain. Elevated 3D model curves are
outside this scope.

Only after this proof does a private equivalent degree-one view feed the
existing oblique planar material proof, including continuous owner-pcurve/model
agreement, directed topology, product frames and native embedded boundaries.
The returned assembly retains complete original source. Evidence copies retain
original support, face, pole, curve and owner IDs, knot intervals and degrees.
Fixed-size evidence arrays use indices zero through the corresponding degree;
unused entries are empty. No source coordinates are snapped.

The source writer re-proves original source and rejects stale supplied claims.
Native JSON uses distinct version-one tag
`plainsolid-step-elevated-nurbs-planar-assembly`; reload re-proves original STEP
and compares geometry, appearance, occurrence and basis evidence. Legacy tags
remain separate. The route caps source at 1 MiB, 4096 entities, 65536 values and
depth 16, with at most 4096 elevated control poles. Original rationals are bounded
to 256 numerator/denominator bits and expected identity results to 1024 bits.
Caller limits and cancellation remain enforced; malformed input is atomic.

## Independent evidence

`tools/occtref/author_ap242_elevated_nurbs_planar.cxx` independently constructs
OCCT boxes, converts their original surfaces, reparameterizes and elevates their
bases, and exports a shared colored XCAF assembly. The retained positive cases
use cubic 4×4 supports with unequal UV spans 3 and 6, and mixed degree 2×3
supports with degree-four pcurves. Live qualification also uses shifted domains
`[-3,0] × [6,12]`. Proper cyclic coordinate placement preserves exact authored
poles. Original and native-source re-export transfer as three solids / eighteen
faces, volume 10800, area 4860, centroid `(9.3125,14.5,12.0625)`.

A genuine OCCT 3:4:5 oblique export is retained as an explicit rejection: one
interior pole has authored X coordinate about −4.44e−16 where the exact affine
identity requires zero. OCCT transfers that file, but this reader rejects its
non-affine original decimal evidence. This is a representability limit, not an
approximate import mode.

Build the pinned comparator with:

```sh
sh tools/occtref/build_author_ap242_elevated_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-elevated-nurbs-planar
PLAINSOLID_OCCT_ELEVATED_NURBS_PLANAR=/tmp/plainsolid-ap242-elevated-nurbs-planar \
  go test ./exchange/step -run '^TestElevatedNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzElevatedNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

The helper uses OCCT source revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`. Tests additionally cover degree-eight
and mixed degree-one/eight exact basis boundaries, reordered records, changed
interior poles with unchanged corners, almost-affine decimals, forged ownership,
invalid domains/degrees, exponent attacks, native claim corruption, limits and
cancellation. Degree-eight boundary fixtures are synthetic exact proof tests;
the independent OCCT positives are cubic and mixed quadratic/cubic/quartic.

# Original multispan UV curves on nonplanar spline material

`ReadMultispanNonplanarSplineShellSource` is additive to the two-pole
`ReadNonplanarSplineShellSource` contract. It authenticates the same bounded
six-face, twelve-edge polynomial nonplanar material, with 24 original polynomial
UV splines of degree 1–8 and at most 17 control points each. At least one UV
curve must have an internal knot. It does not admit rational curves, arbitrary
trim loops, approximate affine curves, or planar substitutes.

Every original clamped, continuous knot vector is checked, including the
advertised knot specification and multiplicities. Exact normalized Greville
abscissae prove all original UV poles equal the corresponding affine endpoint
map. Polynomial B-spline partition of unity and linear precision therefore
prove the complete parameterized curve, across every nonempty span. Positive
span widths and interior multiplicity no greater than degree exclude jumps.
Original scalar text must also equal its binary representation; near decimals
are not silently substituted. Source arithmetic is bounded to 256 bits,
intermediate comparison expressions to 1024 bits, and the source to 1 MiB,
2048 entities, 32768 values and depth 16.

The uniquely owned curve → definitional representation → pcurve → surface
curve → model edge chain is retained. Only the private UV proof view is reduced
to degree one; original 3D model curves, six spline supports, face senses and
all topology are unchanged. The existing nonplanar reader then proves the
model-domain match, continuous model/support agreement, closed native sewing
and complete outward embedding. Independent owner uses may have different
interior knot partitions; this is not permission to change their physical
parameter speed or the model-edge interval.

`PCurves` returns copies of original knots, multiplicities, degree, poles and
all owner IDs. Source re-export preserves the original file. The separate
`plainsolid-step-multispan-nonplanar-spline-shell-source` version 1 JSON envelope
reproves source on reload, compares original ancestry and canonical STEP/native
claims, and rejects forged stored native geometry. It does not trust stored
native bytes for reconstruction. Old envelope tags and readers are unchanged.

## Independent qualification

`tools/occtref/author_ap242_multispan_spline_roof.cxx` independently builds and
sews the six OCCT spline faces of
`0 ≤ x ≤ 4, 0 ≤ y ≤ 3, 0 ≤ z ≤ 2+(x/4)^2`. It converts the actual owner pcurves
to quadratic B-splines, then inserts alternating quarter/half-domain knots.
The pinned OCCT revision is `4dbfdcc448c0b3e565488afcd626c84483172db9`.
The retained AP242 source has 370 entities, two genuine curved quadratic model
rims, nonplanar roof/side supports, and 24 original four-pole UV curves.
No Go output tokens are used to author the input.

Exact expected volume is 28, centroid `(15/7, 3/2, 83/70)`; area is
`12 + 56/3 + 15 + 3*sqrt(5) + 12*asinh(1/2)`.
Independent OCCT original/source-re-export/generated STEP transfers report one
valid solid, six faces, volume 28 and area 58.14941249988127. Native continuous
edge checks report zero squared discrepancy; native JSON reload revalidates
embedding. The live test compares both source and generated STEP after native
source-bound reload.

```sh
sh tools/occtref/build_author_ap242_multispan_spline_roof.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-multispan-spline-roof
PLAINSOLID_OCCT_MULTISPAN_SPLINE_ROOF=/tmp/plainsolid-ap242-multispan-spline-roof \
 go test ./exchange/step -run '^TestMultispanNonplanarSplineShell' -count=1
go test -race ./exchange/step -run '^TestMultispanNonplanarSplineShell' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzMultispanNonplanarSplineShellSource$' -fuzztime=40x -parallel=1
```

Mutations cover changed interior knot/pole, invalid continuity and degree,
wrong owner/model parameter speed, near-decimal and exponent attacks, detached
geometry, original roof deformation, and forged native/source/basis claims.
Record reorder and copied evidence are verified. Failure is atomic; byte,
entity, parser, face/use limits and canceled context are exercised.

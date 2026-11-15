# Original multi-span straight model splines

`ReadModelSplineNURBSPlanarAssembly` authenticates a further independent original
STEP representation: straight polynomial model edges with degree elevation and
internal knots, in the already qualified multi-span affine planar assembly.
It is a separate API; the previous multi-span reader still rejects these model
curves. `WriteModelSplineNURBSPlanarAssemblySource` preserves original source.
The source-bound native version-one tag is
`plainsolid-step-model-spline-nurbs-planar-assembly`.

Every admitted 3D curve has degree 1–8, at most 17 exclusively owned poles,
clamped strictly ordered knots and continuous internal multiplicities. The
shared exact Greville basis checker is identical to the multi-span support/UV
proof. Every original pole must equal the original endpoint line evaluated at
its normalized Greville coordinate. This proves full same-parameter identity,
not merely collinearity or endpoint agreement. The original knot interval and
orientation are retained. At least one elevated or multi-span model curve is
required, together with the prior multi-span support/UV family.

Each model curve must have exactly one original SURFACE_CURVE owner, itself
owned by exactly one EDGE_CURVE. The private proof view reduces only that model
basis to the identical endpoint line; original flags, edge vertices, directed
uses, both PCURVE owners and topology remain subject to the unchanged full
material proof. Source edge orientation inconsistent with the proved line is
rejected. No approximate curve fitting, endpoint snapping or rational-weight
normalization occurs. All original model pole/knot/multiplicity IDs and values,
model-edge ownership, supports, pcurves and occurrence identities remain in
copied evidence and original source.

Native reload re-proves the original source once and compares every stored
geometry, appearance, occurrence and model-basis claim. It does not trust the
stored straight-line assertion. Native output has a 16 MiB cap; source has a
1 MiB / 4096 entity / 65536 value / depth-16 cap. Each model proof processes at
most 4096 poles in addition to the existing support/UV budget; total work is
also bounded by the shared source entity limit. The common rational limits
are 256 source numerator/denominator bits and 1024 result bits. Cancellation,
source limits and atomic rejection remain enforced.

## Independent evidence

The standalone helper independently NurbsConverts OCCT boxes, elevates original
3D curves to degree three, inserts internal knots with the OCCT curve API, and
updates the original B-rep before XCAF STEP export. It also independently
inserts/elevates the support and pcurve bases. No Go-generated geometry tokens
are used to author the positive source.

Retained original fixtures have 24 model curves, 12 surface supports and 48
owner pcurves. Live cases cover midpoint and nonuniform dyadic insertion,
shifted support domains and mixed support/UV degrees. Original and native-source
re-export transfer as three shared-product solids, eighteen faces, volume
10800, area 4860, centroid `(9.3125,14.5,12.0625)`, with original colors.
The comparator is pinned to OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_model_spline_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-model-spline-nurbs-planar
PLAINSOLID_OCCT_MODEL_SPLINE_NURBS_PLANAR=/tmp/plainsolid-ap242-model-spline-nurbs-planar \
  go test ./exchange/step -run '^TestModelSplineNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzModelSplineNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Adversaries change an interior model pole while keeping endpoints, introduce
sub-binary near-line displacement, share poles/curves across owners, reverse
only the basis or edge sense, corrupt knot multiplicities and forge native
model ownership. Prior quadratic, elevated and multi-span regressions remain
qualified. Variable rational weights and genuinely curved model edges are not
part of this milestone.

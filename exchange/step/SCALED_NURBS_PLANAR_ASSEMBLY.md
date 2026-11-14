# Exact non-unit cardinal NURBS chart parameters

`ReadScaledNURBSPlanarAssembly` is a separate extension of
`ReadNURBSPlanarAssembly`. It authenticates the same bounded polynomial planar
material topology and assembly ownership, but permits non-unit cardinal affine
surface derivatives. The older unit-derivative entry point is unchanged and
rejects the new retained source fixture.

For each original chart

```
S(u,v) = O + DU*u + DV*v
DU = a*X, DV = b*Y; a>0, b>0
```

the adapter derives exact rational positive factors `a,b`. It changes only
surface knot values and original owner pcurve poles in a private view:

```
u' = a*u, v' = b*v
S'(u',v') = O + X*u' + Y*v'
```

The unit-derivative importer then proves the complete affine surface identity,
perpendicular cardinal axes, all finite curve endpoints, original face/edge
ownership, full continuous lifted pcurve identity and closed native material.
Positive scaling preserves interval order and maps the original chart rectangle
bijectively, so the original finite-domain and incidence claims follow exactly.
Model spline geometry and its knot parameters are never changed. No fitted or
sampled comparison substitutes for these identities.

Every original scalar consumed by scaling must be exactly representable as
binary64, and every resulting STEP knot or pole must also be exactly
representable. Factors are retained as rational strings; they need not be
binary64 if every actual transformed scalar is lossless. Nonfinite or oversized
coefficients, underflow exponents, unrepresentable products, zero/reversed knot
intervals, diagonal/nonaffine supports, shared UV poles and contradictory owner
records fail closed. The original unit-mode derivative and model-curve speed
constraints, common-unit policy, neutral child frame, exact signed occurrence
frames, metadata and appearance restrictions remain in force.

## Data and source contracts

- `Geometry` is the derived native planar assembly; its source is the equivalent
  PLANE/LINE proof view.
- `OriginalSTEP()` retains the original complete source bytes.
- `NormalizedSTEP()` is the exactly equivalent NURBS source after only the
  proven chart knot / UV pole changes. Its original model curves, all IDs,
  topology, uses, metadata and style records are unchanged.
- `ChartScalings()` returns original chart coefficients, original face/pole IDs,
  exact factors and derived knot ranges.
- `PCurveScalings()` returns original and derived UV poles and owner IDs.
- `ModelCurves()` retains original model-curve evidence.
- `NormalizedPCurves()` describes the derived plane-coordinate equations, with
  original curve parameters and IDs. These are explicitly not claimed to be the
  original UV coefficients.

Evidence getters return copies. `WriteScaledNURBSPlanarAssemblySource` re-proves
and writes original source only, ignoring edits to derived native fields.
Failure returns no partial result; source writes validate before output. No
new native JSON format is added. Existing `WriteColoredAssembly` can separately
encode the derived native planar bodies as newly authored STEP.

Original and proof views each obey the same bounded limits: at most 1 MiB,
4096 entities, 65536 values, depth 16, 4096 edge uses and 16 bodies (or smaller
caller limits). Derived rational numerator and denominator sizes are bounded
before decimal conversion. Cancellation is checked before and after proof and
throughout the chart/pcurve traversal. No cache or cross-call mutable proof is
introduced.

## Independently authored OCCT qualification

Pinned OCCT: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

`author_ap242_scaled_nurbs_planar.cxx` constructs two boxes (2×4×8 and 1×2×4),
converts them through `BRepBuilderAPI_NurbsConvert`, then independently changes
the OCCT surface knots to `[0,1]²` and updates each OCCT spline pcurve before
STEP export. The unchanged 3D model curves retain their original parameters.
It requires the original OCCT B-rep to pass `BRepCheck_Analyzer`.

The retained 681-record AP242 source has 12 charts, 24 model splines and 48
pcurves. Only trailing line whitespace is stripped. No STEP geometry tokens are
rewritten by Go to make the positive original fixture.

A second live source is freshly authored with chart knots `[-2,-1]×[3,4]` using
`write-shifted`; this independently exercises shifted as well as non-unit chart
parameters. Source, original re-export, normalized NURBS and regenerated planar
STEP all transfer through XCAF and pass BRepCheck: three solids, 18 face uses,
volume 136, area 252, centroid `(13/2,180/17,57/17)`, and retained product colors.
Native local volumes are 64 and 8, with one product used twice.

```
sh tools/occtref/build_author_ap242_scaled_nurbs_planar.sh OCCT_INCLUDE OCCT_LIB OUTPUT
OUTPUT write fixture.step
OUTPUT write-shifted shifted.step
PLAINSOLID_OCCT_AP242_SCALED_NURBS_PLANAR=OUTPUT go test ./exchange/step -run '^TestScaledNURBSPlanarAssembly' -count=1
go test -race ./exchange/step -run '^Test(Scaled)?NURBSPlanarAssembly' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzScaledNURBSPlanarAssembly$' -fuzztime=40x -parallel=2
```

Focused tests check exact scaling equations, unchanged unrelated records,
source/evidence roundtrip, original record reordering, copied observation data,
old-mode rejection, malformed ownership and domains, sub-ULP perturbations,
coefficient representability, limits/cancellation and atomic invalid writes.

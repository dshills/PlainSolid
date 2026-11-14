# Exact ordered STEP composite spline geometry

`ReadCompositeCurves` / `WriteCompositeCurves` add a separate bounded
`COMPOSITE_CURVE` / `COMPOSITE_CURVE_SEGMENT` dialect. Existing `ReadCurves`
continues to reject composites. This is ordered curve geometry, not a B-rep
edge/wire ownership model, face boundary, simple-loop proof or material solid.

`CompositeCurveModel` retains product, representation and curve-set names and
explicit SI length units. Each `CompositeSplineCurve` retains its name, segment
order and proved closure; each `CompositeSplineSegment` retains its named full
spline basis and `SameSense` direction. A reversed use does not rewrite the
original poles, weights, knots or parameter domain. Polynomial and rational
explicit-knot source bases are supported; output uses canonical rational records.

## Exact supported domain

- One fixed, fully validated AP203-style product envelope, one bounded wireframe
  representation and one geometric curve set. No assembly or implicit placement.
- At most64 composite roots, 64 direct segments per root, 4096 total poles,
  degree≤ 16,8192 total entities and 1MiB source; tighter caller limits apply.
- Full clamped, continuous, nonconstant B-spline leaves with positive weights.
  Leaf curve/knot form declarations must be UNSPECIFIED. Geometry scalars must
  preserve exact source decimals in binary64, with finite decimal re-emission
  bounded to 64 characters and 256 rational coefficient bits. Weight normalization
  must preserve source ratios exactly. No coefficient or endpoint rounding.
- Every segment and leaf/control-point entity is exclusively owned. Sharing,
  nested composites, cycles, TRIMMED_CURVE and detached geometry reject.
- Intermediate transitions must be CONTINUOUS and adjacent directed endpoints
  must agree exactly. Closed composites require final CONTINUOUS and exact
  final-to-first equality. Open composites require final DISCONTINUOUS and
  distinct terminal endpoints. Stronger transition declarations reject: no
  tangent/curvature continuity is inferred from position.
- Composite self-intersection declaration must be unknown. Leaf logical flags
  are retained informational metadata, not certified self-intersection evidence.
  Closed geometry may self-intersect; no Jordan or material claim is made.

Positional joins follow directly from exact clamped endpoint poles, after the
source coefficient and normalization checks. Internal full-multiplicity breaks
are rejected by the spline decoder. Source record IDs and FILE_HEADER provenance
are not retained. Control point names and context labels must be empty; other
unsupported metadata rejects rather than being silently omitted. The fixed root
metadata graph is checked in full, and every source entity must be owned.
The writer stages and strictly reimports output before caller I/O.

## Independent OCCT qualification

`tools/occtref/step_composite_curve.cxx` independently authors a STEP fixture,
then normalizes it through the pinned OCCT reader/writer. It does not use Go
writer output to construct the reference. The first component is a line from
(0,0,0) to(1,0,0). The second is a reversed rational quadratic with poles
(2,1,0),(2,0,0),(1,0,0), weights 1,3/4,1. A separate closed fixture appends a
line back to origin. The retained OCCT outputs contain 25 and 29 entities.

OCCT's `StepToTopoDS_TranslateCompositeCurve` processes SameSense separately and
uses the final segment transition to select closed-wire processing; this is
explicitly exercised in both fixtures. Ordinary TransferRoots and BRepCheck
accept both source and PlainSolid re-emission: one wire, 2 or 3 edges, open/closed
state and directed terminal endpoints retained. Lengths are approximately
2.5792428865957646 and 4.8153108640955544. Tests compare independent transfer
lengths and require them inside the native exact rational curve-length bounds.

Build with `tools/occtref/build_step_composite_curve.sh` using the pinned OCCT
include/library paths. Enable the live test through
`PLAINSOLID_OCCT_STEP_COMPOSITE=/path/to/helper go test ./exchange/step -run
TestOCCTCompositeCurves -v`.

Default tests cover exact continuous basis comparison after re-emission,
multiple independently owned definitions, source reversal, closure/gap mistakes,
stronger continuity claims, source rounding, nonlossless weight normalization,
cycle/sharing/orphan graphs, metadata/limits and short-write behavior.
`FuzzCompositeCurves` has open/closed/malformed seeds,128 poles and 256 entities
per input; CI runs a 100-execution smoke, not an exhaustive fuzz campaign.
Preflight only adds structural candidate hints, never geometry acceptance.

Qualification run: focused live/ordinary/preflight checks passed in 1.583 seconds,
race in 2.169 seconds, Go 1.23 in 0.274 seconds, and STEP vet passed. A 100-execution
fuzz smoke passed in 0.279 seconds with one new interesting input and no failure;
this is a small bounded smoke, not exhaustive robustness evidence.

An additive derivative-qualified API is documented in [TANGENT_COMPOSITE_CURVES.md](TANGENT_COMPOSITE_CURVES.md); the positional codec behavior above is unchanged.

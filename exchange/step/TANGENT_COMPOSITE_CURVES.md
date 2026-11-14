# Exact derivative-qualified STEP composite transitions

`ReadTangentCompositeCurves` / `WriteTangentCompositeCurves` add a separate API for
selected `CONT_SAME_GRADIENT` transitions. `TangentCompositeCurveModel` wraps the
unchanged `CompositeCurveModel` and adds a per-curve, per-segment boolean matrix:
a true entry describes the join after that directed segment. All names, units,
original knot parameters, basis coefficients and use directions remain retained.
The positional-only codec still rejects stronger declarations.

STEP geometric tangent continuity allows different derivative magnitudes (see
[the corresponding geometry transition definition](https://standards.buildingsmart.org/IFC/RELEASE/IFC2x2/FINAL/HTML/ifcgeometryresource/lexical/ifctransitioncode.html)
and [STEP's transition enumeration](https://downloads.steptools.com/docs/stp_aim/html/t_transition_code.html)).
This implementation deliberately accepts the narrower sufficient condition:
**exact equality of nonzero directed endpoint derivatives in the original spline
parameters**. It does not claim complete G1 interoperability. Positive parallel
but unequal-speed derivatives reject, as do stationary endpoints, opposed
tangents and curvature declarations. No declaration is inferred or downgraded.

## Proof

For a clamped degree-p rational spline, the start derivative is
`p/(U[p+1]-a) * (w[1]/w[0]) * (P[1]-P[0])`. The end derivative is
`p/(b-U[n]) * (w[n-1]/w[n]) * (P[n]-P[n-1])`, where n is the final pole index.
All differences, weight ratios, span factors and vector components are computed
as exact rationals from represented source coefficients. Reversed use selects
the opposite basis endpoint and negates its derivative. Every asserted join
requires componentwise exact equality and nonzero vectors. A closed final join
compares the final directed endpoint with the first; an open terminal segment
cannot declare a tangent join.

The existing exact decimal, weight normalization, clamping, positional join and
ownership gates run unchanged. The derivative result is bounded to 4096 numerator
and denominator bits per component. Parent model limits still bound degrees,
control counts, entities, bytes and syntax depth. The writer checks the explicit
matrix dimensions and stages strict source re-import before I/O; failures return
no partial typed model. Matrix rows returned by the reader are caller-owned.

## Independent reference

The standalone `step_composite_curve.cxx` helper adds `write-tangent` and
`write-tangent-closed` modes. Its independently authored source has a line from
(0,0,0) to (1.5,0,0), followed by a reversed rational quadratic with poles
(2.5,1,0),(2.5,0,0),(1.5,0,0), weights 1,3/4,1 and domain [0,1]. OCCT normalizes
the STEP entity graph, transfers it through ordinary TransferRoots and checks
BRep validity. Independent StepToGeom evaluation reports both declared join
derivatives as (1.5,0,0), including the reversed basis use.

Open/closed transfer preserves one wire with 2/3 edges, lengths approximately
3.0792428865957646 and 5.7718252901630169. The closed fixture has a positional
closing line, not a claimed tangent closure. Source and PlainSolid re-emission
both retain the declaration and evaluated derivatives. Pure-Go tests additionally
prove all four tangent joins of a closed rational quarter-profile loop, including
its final-to-first join, and an endpoint adjacent to an interior knot.

Mutations cover weights, knot domains, source senses, zero/opposed derivatives,
unsupported curvature assertions, matrix shape, open terminal assertions and
limits. `FuzzTangentCompositeCurves` adds bounded open, closed, positional and
malformed seeds; its fixed 100-execution CI smoke is not exhaustive coverage.
This remains ordered curve geometry, not a simple wire or material certificate.

Qualified against OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Focused old/new codec, preflight and live transfer tests pass; the focused race
suite, Go 1.23 compatibility run, STEP package vet and fixed 100-execution fuzz
campaign also pass. These checks do not extend the explicitly bounded C1 subset.

A separate [geometric tangent mode](GEOMETRIC_TANGENT_COMPOSITE_CURVES.md)
accepts exact positive proportional derivatives with unequal parameter speeds.
It does not change this API's equal-derivative requirement.

# Exact rational UV topology certification

`ValidateLoops(ctx, outer Loop, holes []Loop, Options)` works with representation-
independent `Segment{Start, Control, End, Kind}` values. `Line` is the zero-value
kind; `Quadratic` uses a quadratic Bezier control point. `RationalQuadratic` additionally
requires three finite strictly positive `Weights`, applied to Start/Control/End. The optional
`brep/trimvalidate/adapter` package converts `trimmed.Face` data without making
the core validator depend on trimmed solids.

**Always inspect `Report.Status`, even when the returned error is nil.**

- `Certified`: every loop is simple, the outer orientation is positive, holes
  are negatively oriented, strictly contained, pairwise disjoint and nonnested.
- `Invalid`: an exact degeneracy, winding error, crossing, contact, or containment
  violation was proved. Loop/edge IDs and a reason locate the finding.
- `Unresolved` (also the zero value): a curve type is unsupported or proof limits
  exhausted. This never asserts either validity or invalidity.

Invalid options and context cancellation return an error. Ellipse evaluators,
splines above degree eight, and custom p-curves remain unresolved.

All geometry predicates use rational arithmetic on represented coordinates.
Positive-weight rational quadratic control hulls conservatively enclose their
traces. Subdivision uses exact homogeneous de Casteljau operations and retains
positive weights. The adapter decomposes continuous clamped degree-one and
degree-two `SplineUV` trims into homogeneous Bézier spans. Higher degrees
through eight additionally require a strictly monotone chord projection,
certified from exact homogeneous subdivision if needed. Their oriented area
is bounded by the chord and perpendicular control-hull extrema. It supports clipped
and reversed intervals, retains nonbinary rational internal join points,
requires exact represented declared topological endpoints, and maps findings
back to the original uses. `MaxEdges` counts the resulting spans. Exact separating
axes prove disjoint hulls or that adjacent hulls can meet only at their prescribed
common endpoint. Otherwise de Casteljau subdivision refines the proof. Rational
endpoint-on-curve and line intersection tests prove contact, including rational
line/quadratic roots and double roots at nondyadic tangency parameters. A
monotone-graph/intermediate-value argument proves transverse crossings where
control projections meet its conditions. Remaining ambiguous tangencies can
exhaust the budget and return unresolved.

A noncollinear positive-weight rational quadratic is injective (its barycentric
endpoint-weight ratio is strictly monotone); collinear quadratics are accepted only
when their middle pole lies between the endpoints, giving a monotone segment
trace. Retracing is invalid. The certificate concerns geometric traces, not
nonzero parameter derivatives: a monotone segment with zero endpoint speed can
be certified. Polynomial edges use exact line integrals for orientation. For rational edges,
the oriented area lies between the endpoint chord and the control polygon area.
Exact subdivision refines this enclosure until the total loop area has a proven
sign; uncertain signs return unresolved. No numerical quadrature establishes
orientation.

Containment uses a separate proof: curves are subdivided until each control hull
excludes the query point. Replacing those pieces by chords is then a homotopy
that cannot pass through the point, so exact chord winding equals true curved-
loop winding. No sampled polygon is accepted without that exclusion proof.

Defaults: 256 edges, 200,000 work units, 100,000 subdivisions, maximum binary depth
48 (configurable through 256), and 16,384 bits per stored control coordinate.
Temporary expressions have bounded algebraic depth but can exceed that coordinate
bit count. Work and subdivision budgets are shared across separation and
containment. All subdivision/pair loops check context.

This certifies **UV-loop topology only**. It does not certify surface embedding,
model-space edge/p-curve agreement, surface Jacobian regularity, numerical
floating-point evaluation, or a curved trim's model-space approximation error.

True circle quadrants can be represented exactly with poles `(1,0)`, `(1,1)`,
`(0,1)` and weights `(1,1,2)`. This differs in parameter speed from the familiar
symmetric square-root-weight form but traces the same exact circle. Tests prove
`x²+y²=1` at rational parameters and exact subdivision identities, certify an
annulus of such quarters and a near-tangent gap, and retain an unresolved result
for a nondyadic conic/conic tangency when the proof cannot decide it.

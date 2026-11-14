# Rational spline generatrices in a split cylinder product

`ReadSplineGeneratrixCylinderProduct` is a separate, bounded solid importer.
It accepts the existing strict rooted AP203 half-split cylinder with its two
axial LINE edges replaced by full clamped degree-one B-spline curves. Positive
rational weights and original finite knot domains are supported. The output
retains each original `SplineCurve`, keyed by authored `EDGE_CURVE` ID, and its
`same_sense`, separately from the canonical analytic cylinder and source roles.

Each spline must have exactly two control points equal to the original directed
edge vertices (reversed when `same_sense` is false). A positive-weight rational
linear segment is a strictly monotone reparameterization of that entire segment:
its normalized position coefficient is t*w1/((1-t)*w0+t*w1), with derivative
w0*w1/((1-t)*w0+t*w1)^2 > 0. Thus neither endpoint snapping nor a sampled
straightness test is involved. The two endpoint positions must differ in exactly
one model coordinate. All spline scalars must pass the exact decimal/binary gate;
weight normalization must preserve their exact ratios.

After these proofs, private temporary LINE records express the same directed
supports. The unchanged strict cylinder product reader proves the original
circle rims, planar caps, complementary lateral cells, signed frame, four-face
closed shell, coedge senses, source ownership, product/name binding and SI unit.
Only the two validated axial edges may have been splines. Original curve IDs and
edge/face ancestry are preserved. Removed dedicated control points are retained
in immutable source geometry; shared references remain subject to whole-graph
ownership checks. No other input entities are silently dropped. Rejection returns
no partial result. Input has at most 240 entities; normalization adds four bounded
local direction/vector records. Caller limits and the existing stricter topology
limits still apply, including to normalized staged input.

This does not widen the old LINE-only readers. It does not accept pcurves,
partial spline trims, higher-degree straight curves, non-cardinal placements,
unnamed metadata conventions outside the registered product envelope, or rounded
vendor support coordinates. Curve form/knot form must be UNSPECIFIED, closed F,
self-intersection U. Source curve names and finite parameter domains are retained;
STEP header provenance is not an asset persistence contract.

## Independent fixture and checks

`testdata/spline-generatrix-cylinder.step` is independently authored from the
retained model-space split-cylinder graph, not produced by the Go writer. Its
radius is 2 and height 3; rational axial curves have domain [3,7], weights [1/2,1],
and opposite basis directions. Conventional OCCT `TransferRoots` and BRepCheck
accept it as one valid four-face solid. The independent
`read_split_cylinder.cxx` helper built against OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9` reports volume 37.699113210939316,
area 62.831855123588653 and centroid (0,0,1.5), up to floating numerical error.
Analytic values are 12π and 20π; the test's 3e-6 absolute allowance covers OCCT's
rational-edge quadrature, not Go geometric acceptance.

Set `PLAINSOLID_OCCT_SPLIT_CYLINDER` to that helper to repeat the root-transfer
check. Native tests retain original knot/weight/sense data and check atomic
rejection for endpoint, weight, domain, sense, shared curve, cylinder radius,
product/unit, orphan geometry, rounded coordinates and resource mutations.
The bounded fuzz target exercises this typed reader directly.

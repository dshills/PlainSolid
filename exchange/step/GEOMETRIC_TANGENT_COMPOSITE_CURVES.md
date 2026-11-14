# Directed geometric tangent composite curves

`ReadGeometricTangentCompositeCurves` and
`WriteGeometricTangentCompositeCurves` add an explicit geometric G1 mode to the
bounded direct spline composite dialect. The wrapper carries the same names,
units, original parameters, segment uses and transition matrix as the C1 codec.
Neither existing positional nor exact equal-speed C1 APIs change acceptance.

For each `.CONT_SAME_GRADIENT.` join, exact nonzero directed endpoint derivatives
A and B must be positively proportional. A nonzero pivot of A must have the same
sign in B; all three exact identities A[i]B[pivot] = B[i]A[pivot] must hold.
This is invariant under positive parameter speed changes; opposite tangents,
zero derivatives and almost-parallel rounded directions fail. Existing exact
source decimal/weight, clamping, continuous source basis, ownership and closure
checks still run. Endpoint derivative components remain limited to 4096 bits;
three bounded rational product comparisons add no data-dependent iteration.
No normalization or snapping changes the source spline or its parameter domain.

The standard transition is geometric, as specified by the
[STEP transition code](https://downloads.steptools.com/docs/stp_aim/html/t_transition_code.html)
and [transition semantics](https://standards.buildingsmart.org/IFC/RELEASE/IFC2x2/FINAL/HTML/ifcgeometryresource/lexical/ifctransitioncode.html).
This mode does not support curvature assertions, nested composites, trimmed
leaves, a zero-speed tangent limit, wire simplicity or material topology.

## Independent reference

The standalone OCCT helper's `write-geometric-tangent` mode independently authors
and normalizes a two-segment source. Its line derivative is (1.5,0,0); its reversed
rational quadratic derivative at the join is (1,0,0), with weights (1,1/2,1).
The retained 25-entity file therefore fails the original C1 reader but passes
this mode. OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9` independently
loads both the source and Go re-emission with ordinary TransferRoots and valid
BRep, one wire/two edges, length 3.0259717630258898, and the original unequal
derivatives. `PLAINSOLID_OCCT_STEP_COMPOSITE` selects the rebuilt helper.

Tests cover cross-API rejection, exact and near proportionality, opposite and
zero derivatives, different pivot coordinates, bounded input and atomic failure.
A bounded fuzz target checks accepted write/read closure. Native, race, Go 1.23,
STEP vet and live OCCT checks qualify this slice only.

# Exact same-domain planar face unification

`Unify` merges edge-connected faces whose **represented coordinates** lie on the
same exact oriented plane. Rational plane coefficients determine equality;
there is no distance/angular tolerance, coordinate welding, or vertex movement.
Merely nearby faces and coplanar but disconnected faces remain separate.

Input and output are checked as embedded closed material boundaries using
`collision.NewPointLocator`, including self-intersections and shell orientation
relative to cavity/island nesting. Faces accepted by the general BRep planarity
tolerance but not exactly planar are rejected by this operation.

Internal edges cancel within each connected plane group. Directed remaining
edges form an outer loop and inner loops, preserving holes. Globally degree-two
collinear feature vertices are removed consistently from both incident faces.
A necessary higher-valence collinear vertex cannot be represented by the current
BRep face contract; such a junction fails explicitly. Branched loops, multiple
outer loops in one connected group, and unsupported holed-face sizes likewise
fail rather than guessing a boundary or introducing T-junctions.

`OldToNew` and `NewToOld` describe polygon-face ancestry. Output groups are
ordered by their lowest source face index, and source lists are ascending.
`VertexMap` identifies preserved original coordinates or `-1` for removed
vertices. No partial solid or maps are returned on failure. Results and history
slices are caller-owned. Determinism is relative to the supplied graph ordering;
this is not persistent naming invariant under arbitrary source renumbering.

Default limits are 4,096 input faces, 8,192 validation triangles, 2,000,000 work
units per unification/validation phase, and 100,000 collision comparisons per
validation. Exact loops check context; the bounded collision validation pass is
not internally cancellable. Hole tessellation retains the underlying BRep limit.
This package handles planar faces only; it does not unify curved analytic or
spline supports.

`UnifyWithHistory` additionally reports exact normalized source/output edge
intervals and the locations of removed boundary subdivision vertices on output
edges. Source parameters follow `Start` to `End`; target parameters may decrease
when orientations differ. Deleted intervals explicitly describe lost edge
identity, including internal coplanar seams, rather than inferring a mapping to
nearby geometry. Every output edge must have full source coverage. The exact
squared displacement is zero because coordinates and boundary geometry do not
move. A removed internal seam vertex has output vertex -1 and no edge locations.

History has a separate default budget of two million edge/edge and vertex/edge
comparisons, with cancellation checks, in addition to the existing unification
limits. It uses exact rational arithmetic on the finite input coordinates, no
welding tolerance, and returns no partial result on failure. All intervals and
slices are caller-owned; ordering is deterministic.

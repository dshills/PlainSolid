# Exact sphere/torus occurrence line clipping

`ClipRationalHierarchyAnalyticLine` and
`ClipOCCTRationalHierarchyAnalyticLine` authenticate the corresponding complete
source and caller claims, then clip `origin+t*direction` to a caller-selected
finite closed rational interval. Direction need not be unit length. Returned
algebraic parameters retain the original t, including negative values; no
normalization, numerical root fitting, or world geometry baking occurs.

This slice supports sphere, ring-torus and finite-cylinder and proved convex hole-free planar leaves. Other planar leaves
remain present with `Supported=false`; no assembly-wide first-hit or union claim
is made. Within the supported subset, hits are globally ordered using exact
algebraic comparison. Identical parameters retain all original occurrence owners
and paths, even for separately parameterized coincident spheres. Source face IDs
refer to the complete original material shell. No source seam branch is guessed.

The exact sphere quadratic and ring-torus quartic use rational inverse-transported
coordinates. R>r>0 is already proved, so the torus polynomial has no extraneous
squared branch. Complete Sturm isolation retains multiplicities and both endpoint
roots. Endpoint signs and the exact right-hand Taylor sign at the lower clip end
establish every strictly interior span; odd multiplicity flips the sign. Tangent
contacts remain hits even when no interior span exists. Span endpoints are closed
by material continuity, including clip ends that lie inside the material.

The additive `geom.IsolateExactPolynomialRoots` wraps existing exact isolation
and immutable algebraic identities for bounded polynomial degree (the query uses at most four). It excludes a
previous endpoint root from any non-singleton closed root certificate, validates
unique roots and retains original multiplicities. Source associations are proved
by the caller, not by this general numeric wrapper.

Defaults: 2 million cumulative root operations, 4096 coefficient bits, and 4096
cumulative root subdivisions. Maxima: 20 million, 8192 bits, and 65536 subdivisions.
All isolation, root construction and global comparison work consumes the same
query counters. Exact fixed-degree coefficient/pose arithmetic is separately
bit-bounded; the existing source limits bound products, depth and 64 expanded
occurrences. Errors return no partial hits/spans. Inputs/accessor intervals are
copied; callers must not mutate input models concurrently.

Ordinary tests prove exact nonunit parameter transport, four-hit torus crossings,
tangencies, endpoint clipping, irrational roots, simultaneous ownership under
different sphere frames, sorted global events and exhausted work rejection.
Independent pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 reads the original
standard assemblies and intersects actual transferred sphere/torus solids.
Set `PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE` to the standalone helper to compare
all multi-hit source parameters under both occurrence conventions. The oracle's
numerical tolerance does not enter the Go proof.

## Finite cylinders

Finite cylinders are now supported by exact axial slab clipping followed by
radial quadratic isolation. Endpoint signs are material signs: negative inside,
zero boundary, positive outside. All cap/wall root comparisons share the query's
cumulative root budgets. A transverse rim hit retains both support roles once
for that occurrence. `Supports` records each support multiplicity; zero denotes
a line coincident with that support. `Multiplicity` remains the single support's
multiplicity, or zero when several supports meet (no fictitious product root).

`Spans` still means strictly material-interior open intervals with included
endpoints. `BoundarySpans` separately represents nonzero closed intervals lying
on a cap or the cylinder wall. Their Roles identify those supports. Arbitrary
clip ends of a coincident span are not invented discrete hits; real cap/rim
intersections remain hits. Tangent singleton contacts have no nonzero span.
Other planar leaves remain explicitly unsupported. Analytic source face IDs name the
complete shell, not a guessed split-cylinder periodic cell.

Pinned OCCT independently checks transverse, axial and simultaneous rim stations
for both exact rational hierarchy conventions. Coincident wall/cap intervals,
clipped boundary-only lines, reversed axial direction, tangent points and singleton
cap clips are tested analytically with exact endpoint comparison. The query uses
main's existing degree-eight polynomial wrapper; no replacement geometry API is
required.

## Exact convex planar leaves

Planar leaves require one hole-free strictly convex polygon per face and a
complete outward convex supporting-halfspace proof over every original vertex.
Before clipping, each directed native edge is uniquely rebound to the source
EDGE_CURVE and both original VERTEX_POINT records using exact decimal coordinate
equality; source face winding and original PLANE support/sense must agree exactly.
The already authenticated source remains responsible for closed manifoldness and
embedding. No approximate plane fit or triangle intersection is used.

The common rational occurrence inverse retains original t. Exact intersection
of all oriented halfspaces yields a closed material interval, a singleton corner
contact, or emptiness. Constant-zero constraints identify complete coplanar
boundary spans. Endpoint contacts retain all incident original face, edge and
vertex IDs; exact cross-products and segment dot bounds distinguish a shared
edge from its infinite supporting line. An edge-coincident span retains both
source faces and its original edge. Arbitrary clip ends are not discrete hits.

Nonconvex/holed/unqualified planar leaves return Supported=false with an explicit
UnsupportedReason. Malformed or forged source/claims still fail the whole query.
The added MaxPlanarTests defaults to65536 (maximum1048576) and cumulatively bounds
source coordinate matching, halfspace tests and segment ownership across all
occurrences. Exact scalar arithmetic retains the existing coefficient-bit gate.

Tests cover face, edge, triple-face corner, tangent corner, clipped coplanar
intervals, outside parallel lines, exact source ancestry and work exhaustion.
A source-authored concave prism is explicitly rejected by the convex query gate.
Pinned OCCT independently agrees on transferred box transverse and corner hits
in both hierarchy conventions. Earlier cylinder/sphere/torus checks still run.

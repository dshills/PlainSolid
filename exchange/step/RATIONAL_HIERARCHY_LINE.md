# Exact sphere/torus occurrence line clipping

`ClipRationalHierarchyAnalyticLine` and
`ClipOCCTRationalHierarchyAnalyticLine` authenticate the corresponding complete
source and caller claims, then clip `origin+t*direction` to a caller-selected
finite closed rational interval. Direction need not be unit length. Returned
algebraic parameters retain the original t, including negative values; no
normalization, numerical root fitting, or world geometry baking occurs.

This slice supports sphere and ring-torus leaves. Planar and cylinder leaves
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
and immutable algebraic identities for degree at most four. It excludes a
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

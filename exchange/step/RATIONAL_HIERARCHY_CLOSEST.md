# Exact retained-boundary minimum

`ClosestRationalHierarchyBoundary`, `ClosestOCCTRationalHierarchyBoundary`, and
`AffinePCurveHierarchy.ClosestBoundary` compare complete per-occurrence planar,
sphere, finite cylinder, and ring-torus boundary minima. They retain every
exactly equal occurrence and its complete nearest point/circle/sphere set.
These are distances to retained boundaries, including internal boundaries of
overlapping solids. They are not distances to a regularized material union.

Each existing family query authenticates the entire source and occurrence table;
this deliberately performs four bounded source proofs rather than introducing a
new cache. Every occurrence must have exactly one supported family. Root
isolation, rational-root creation, and exact comparison share cumulative work
and subdivision budgets. Finite planar candidate/projection work has separate
explicit quotas. No unsupported or exhausted partial minimum is returned.

The minimum root is immutable. Returned typed leaf payloads are caller-owned
observations, not certificates accepted by subsequent geometry APIs. Their
existing exact witness accessors/enclosures retain each family's semantics.

Focused tests compare original seven-solid source minima against the pinned
OCCT distance comparator in both frame conventions, check an exact source
boundary zero and original owner, cancellation and insufficient work. A separate
exact candidate test compares differently represented equal algebraic/rational
roots across all four payload kinds and repeated occurrence paths. It does not
claim an independently transferred all-four-family coincident fixture.

Live check:

    PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE=/tmp/plainsolid-rational-hierarchy-distance go test -race ./exchange/step -run '^TestHierarchyClosest' -count=1

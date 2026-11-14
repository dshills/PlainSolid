# Exact analytic occurrence point classification

`LocateRationalHierarchyAnalyticPoint` and
`LocateOCCTRationalHierarchyAnalyticPoint` authenticate the corresponding complete
source and caller claims, then inverse-transport a caller-owned rational world
point through each proper occurrence and local support frame. No pose or point
is rounded. Coordinates remain in the common source LengthUnit.

The result retains every original occurrence, product/body IDs, full source face
list and NAUO path. Cylinder, sphere and ring-torus occurrences return inside,
outside or boundary; planar occurrences explicitly return unsupported. There is
no assembly-union classification, first-hit ordering, algebraic ray solver, or
claim about periodic trim parameter selection. Equal/coincident occurrences are
not deduplicated. Boundary roles name analytic supports, not a selected source
split-cell face/coedge. Cylinder rim points name both adjacent support roles.

Cylinder signs test x²+y²−r² and both exact axial bounds. Sphere signs test
x²+y²+z²−r². Ring torus signs test
`(x²+y²+z²+R²−r²)²−4R²(x²+y²)`; the previously proved R>r>0
makes the unsquared left expression positive, so this polynomial introduces no
spurious interior or boundary branch. Existing analytic/angular native chart
qualification is unchanged; this is exact support/material classification, not
an exact rational trigonometric patch claim.

Default coefficient bound is 4096 bits, allowed range 256–8192. Input size and
intermediate rational operations are checked. Existing bounded source parsing,
8-level/64-occurrence graph restrictions and per-occurrence cancellation remain.
Failure returns no partial classifications. Input rational values are copied;
coordinate accessors return copies. Callers must not concurrently mutate inputs
while invoking the query.

Tests cover exact oblique transport, cylinder rim/cap ownership, both torus
radial boundaries and excluded central hole, sphere poles, forged ancestry,
coefficient limits and coordinate ownership. Optional independent pinned OCCT
4dbfdcc448c0b3e565488afcd626c84483172db9 source transfer/classification uses
`PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION` and the standalone helper in
tools/occtref. Its 1e-8 oracle tolerance is not used by the Go predicates.

## Additive convex planar point mode

`LocateRationalHierarchyPoint` and `LocateOCCTRationalHierarchyPoint` additionally
classify strict convex, hole-free planar leaves. The earlier `...AnalyticPoint`
functions keep returning unsupported for every planar leaf.

The point and line paths now share one private immutable source-bound proof:
exact owned vertices, outward halfspaces, original directed edge/vertex IDs and
source plane senses. Point queries prepare each local planar product once and
reuse it for every occurrence. No source or caller-native geometry is mutated.
Exact inverse placement yields `ProductCoordinates` (owned copies); analytic
support-frame coordinates retain their prior distinct meaning.

A point is outside if any proved halfspace is positive, inside if all are
negative, and on the boundary otherwise. Boundary results return all original
incident face, edge and vertex IDs using exact coordinate equality, collinearity
and finite segment tests. No tolerance merges nearby identities. Nonconvex or
holed material remains explicitly unsupported with a reason. Malformed source
or forged claims still fail transactionally.

MaxPlanarTests defaults to65536, maximum1048576, cumulatively covering proof and
classification across occurrences. This is separate from source parsing and
coefficient-bit limits. Tests preserve source bytes and demonstrate accessor and
result mutation isolation, exact near-face rational distinctions, old API
behavior, and shared corner/edge IDs. Pinned independent OCCT classifies original
box interior, face, edge, corner and outside points in both hierarchy modes.

A broader planar crossing mode would require replacing convex halfspace signs
with exact finite polygon/holes membership and an oriented crossing ledger.
The shared source-binding proof can be factored further, but merely deleting its
convexity gate would not establish correct nonconvex material intervals.

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

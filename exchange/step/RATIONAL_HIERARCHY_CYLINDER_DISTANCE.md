# Exact finite closed-cylinder hierarchy distance

`DistanceRationalHierarchyCylinders`,
`DistanceOCCTRationalHierarchyCylinders`, and
`AffinePCurveHierarchy.CylinderDistance` add a separate finite-cylinder query.
Original source/native geometry, material ownership and proper rational poses
are re-proved. Other families return explicit unsupported results and nil
distances; no partial assembly-global minimum is inferred. Existing planar and
sphere contracts remain unchanged.

## Complete boundary candidates

In the authenticated local cylinder frame let the query be (x,y,z), radius r>0,
height h>0, and s=x²+y². The boundary is the closed side plus two closed disks.
The side's nearest axial station is exactly clamp(z,0,h). Its squared distance
is `(sqrt(s)-r)² + (z-station)²`. A cap projects to (x,y,capZ) when s<=r², or to
its rim when s>r². These three candidate distances cover the whole boundary.
They are compared with exact algebraic root comparison, retaining every tie.
Duplicate rim points reached through both side and cap candidates are merged
by their proved identical radial station, not a numeric proximity test.

A radial distance plus rational axial square A is the smaller root of

```
t² - 2(A+s+r²)t + A² + 2A(s+r²) + (s-r²)² = 0.
```

For s>0 and s!=r² it is the unique root in [0,A+s+r²]; the other root is larger
than the upper endpoint. At s=0 and s=r² the distance is rational and is
constructed directly. Material distance is zero inside/on the closed cylinder,
and equals boundary distance outside. Boundary distance remains positive for
interior points.

## Nearest sets and original source owners

A unique radial point uses the positive algebraic scale lambda determined by
`s*lambda²-r²=0`. Its world coordinates are represented as
`Center + lambda*Direction`; disk-interior projections use a rational center
and zero direction. Point enclosures reuse the exact bounded affine interval
method already qualified for spheres. No rounded coordinate is promoted to
exact geometry.

On the axis, a side minimum is an **entire circle**, represented by its exact
world center, proper unit axis and radius. `Kind="circle"` explicitly differs
from `Kind="point"`. Unique-point enclosure of a circle returns ErrDomain;
no arbitrary direction is selected. Circle and cap-center ties remain separate
sets, as do two cap points or side-plus-two-cap point ties.

For a unique point, `SourceFaceIDs` retains every incident original cap and
half/quarter wall face, including both owners at an axial seam. For a circle,
it names the wall faces that collectively cover the whole nearest set, not an
assertion that each point belongs to every face. This API does not infer
original edge/vertex roles from an algebraic point. Source product/body IDs
and full occurrence paths are retained; exact rational observation getters
return copies. Radius/axis describe the supporting cylinder/circle; they are
not a distance estimate.

## Qualification and bounded work

All root isolation, construction and comparison work is cumulative across
occurrences. A fixed 512-operation reservation per supported use bounds the
small affine/candidate coefficient calculations. Coefficient sizes and root
subdivisions are bounded; source reproof retains its separate limits. Point
enclosure is separately bounded. Failures and cancellation return no partial
query results.

The original radius-2/height-3 retained hierarchies exercise side/cap/rim
projections, source seam multiplicity, nonrational radial points, exact zero,
two-cap ties and three distinct point ties. A separately authored standard
radius-1/height-3 variant is produced by
`tools/occtref/author_step_thin_cylinder_hierarchy.py`: only the cylinder's owned
Cartesian radial coordinates and analytic radii are scaled exactly, retaining
all source products, poses and other bodies. This variant is independently
transferred by pinned OCCT and exercises whole-circle and circle-plus-cap ties.
It is not described as a new vendor-authored graph.

Both rational frame conventions pass native reload and original STEP OCCT
boundary/material distance and finite nearest-point checks. Infinite circle
sets are proved analytically rather than inferred from the oracle's finite
witness list. Exact quadratic sign checks cover irrational distances; rational
distances/ties are compared algebraically. Existing sphere and planar hole-tie
regressions run with the same independent comparator.

```
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE=/tmp/plainsolid-rational-hierarchy-distance \
  go test -race ./exchange/step -run 'TestHierarchyCylinderDistance|TestHierarchySphereDistance|TestHierarchyPlanarDistance'
```

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

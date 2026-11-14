# Exact ring-torus distance and nearest sets

`DistanceRationalHierarchyTori`, `DistanceOCCTRationalHierarchyTori`, and
`AffinePCurveHierarchy.TorusDistance` add per-occurrence queries for the already
proved natural ring-torus family R>r>0. Source/native material, exact proper
rational pose and original product/body/face/path ownership are re-proved.
Other families are explicitly unsupported here. Existing planar, sphere and
cylinder queries remain separate; no partial global minimum is implied.

## Quartic distance and root selection

In the source torus frame write P=(x,y,z), s=x²+y², A=s+z²+R², and
`d_minus=sqrt(A-2R*sqrt(s))`, `d_plus=sqrt(A+2R*sqrt(s))`.
The exact boundary squared distance is `(d_minus-r)²`.
This is a global boundary minimum: every ring-torus surface point has positive
radial coordinate at least R-r. For s>0, minimizing its azimuthal squared-distance
term therefore forces the query azimuth; the remaining problem is the ordinary
meridian circle. For s=0 the azimuthal term is constant and all azimuths tie.
For a regular query this is a root of the rational quartic

```
K(t)² - 16 R² s (t-A+r²)² = 0
K(t) = (t-A-r²)² + 4 R² s - 4 r² A.
```

The four real roots, counted with multiplicity, are `(d_minus ± r)²` and
`(d_plus ± r)²`. The desired root is the least nonnegative root. Indeed,
d_plus>=d_minus and d_plus+d_minus>=2R>2r by the triangle inequality between
the two meridian centers. These inequalities also prove the selection when
d_minus<r; it is not a numerical choice between conjugates.
All roots lie below the rational bound
`U+r²+2r(U+1)`, with `U=A+2R(s+1)`. Complete bounded isolation and exact root
comparison choose the least root. On the boundary the result is exactly zero.
The meridian-center case is exactly r²; the axis case needs only a quadratic.

Material inside/outside is exactly determined by
`F=(A-r²)²-4R²s`: its second meridian factor is strictly positive for R>r.
Material squared distance is zero inside/on the closed torus and equals
boundary squared distance outside.

## Unique and nonunique nearest geometry

`NearestBoundary.Kind()` is immutable and distinguishes three cases:

- `point`: for s>0 and d_minus>0, put beta=1/sqrt(s) and lambda=r/d_minus.
  The local nearest point is
  `(x*(R*beta+lambda-R*beta*lambda),
    y*(R*beta+lambda-R*beta*lambda), z*lambda)`.
  Beta is the positive quadratic root of `s*beta²-1=0`.
  Lambda is the largest positive root of
  `(A²-4R²s)*lambda⁴ - 2Ar²*lambda² + r⁴=0`.
  These selection rules retain the original mathematical branch exactly.
- `meridian-circle`: z=0 and s=R². Every point on the tube's meridian circle
  is nearest. Its center is P, radius r, and axis is the transported rational
  azimuth direction `(-y/R,x/R,0)`. No arbitrary point is selected.
- `axis-circle`: s=0. Every azimuth is nearest. The circle's local center is
  `(0,0,z*lambda)`, radius `R*(1-lambda)`, axis the torus axis, with
  `lambda=r/sqrt(R²+z²)`. Center/radius can be algebraic. R>r proves its radius
  strictly positive.

`EnclosePoint` and `EncloseCircle` return certified outward rational intervals
for the appropriate geometry and reject the other kind with ErrDomain. Proper
rational occurrence matrices transport all intervals without rounded baking.
The independent beta/lambda intervals may conservatively overestimate their
correlated expression; bounded refinement reduces the enclosure to the requested
coordinate/radius width. Exact boundary points and rational meridian circles
return exact singleton intervals. Root equations and owned rational source
parameters remain inspectable. Only the original owning torus face is claimed;
no periodic seam/pole edge identity is invented.

## Limits and independent qualification

Root isolation/construction/comparison operations and subdivisions are cumulative
across occurrences, with a 512-operation reservation per supported use for its
small rational coefficient/pose calculation. Coefficient bounds apply to every
intermediate expression. Source reproof has its existing separate bounds.
Enclosure is a separate bounded operation, charging all root refinements and
512 interval-arithmetic operations per attempt, at most 256 attempts. Ill-
conditioned or over-budget requests return ErrConvergence with no partial
claimed geometry. This is an explicit unresolved result, not approximate
acceptance. Cancellation is checked throughout.

Tests cover outer/inner/top points, interior and exact boundary queries, nested
irrational coordinates, raised and central axis circles, rational meridian
circles (including nonbinary local coordinates), both frame conventions and
native reload. Exact polynomial sign and rational-root comparisons verify the
retained equations. Original pinned OCCT distances and unique nearest points
agree with certified enclosures; nonunique sets are proved by the formulas,
not inferred from the oracle's finite witness list.

A focused four-family integration test applies the planar/sphere/cylinder/torus
queries to one authenticated mixed hierarchy and verifies that exactly one
explicit family result supports each original occurrence. It does not silently
combine them into an assembly-global minimum.

```
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE=/tmp/plainsolid-rational-hierarchy-distance \
  go test -race ./exchange/step -run 'TestHierarchyTorusDistance|TestHierarchyDistanceFourFamily'
```

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`.

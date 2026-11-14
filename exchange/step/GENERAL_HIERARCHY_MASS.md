# Source-oriented exact mass for general planar hierarchy products

`MeasureGeneralRationalSphericalHierarchy` and
`MeasureGeneralOCCTRationalSphericalHierarchy` add an explicit source-loop
integration route for authenticated nonconvex/holed planar products. The
`AffinePCurveHierarchy.Measure` method applies it to the independently authored
OCCT Boolean-cut product without losing its original face IDs. Previous mass
APIs and native tags remain unchanged. Their native triangulation also supports
holes; this new route is an independent oriented-chain proof, not a claim that
all such geometry was previously rejected.

The complete STEP/native source binding and global embedded outward material
are re-proved first. Each original face loop is matched to the native directed
edges and vertices. Exact support-plane equality is required, the outer loop
has positive projected area relative to the outward normal, and each hole has
negative projected area. Only then is every loop written as a signed fan from
its first original vertex. This is a chain decomposition, not a triangulation:
fan triangles may overlap or extend outside a concave polygon, but signed
contributions cancel. Inner loop orientation subtracts each hole. No absolute
volumes or floating-point geometric choices are used.

For a signed tetrahedron with vertices 0,a,b,c, its volume is
`det(a,b,c)/6`, first moments are `V*(a+b+c)/4`, and second moment ij is
`V*((a+b+c)_i*(a+b+c)_j + a_i*a_j+b_i*b_j+c_i*c_j)/20`.
All operations are exact rationals; the final total local volume must be
strictly positive. The existing exact proper rational occurrence transform
then transports first and second moments, and derives origin/centroid inertia.
Products are counted once locally; occurrences are summed independently,
including overlaps. This is not a Boolean assembly-union mass computation.
Analytic cylinder, sphere and ring-torus formulas remain symbolic in pi.

`MaxTriangles` counts signed fan terms cumulatively over local products.
`MaxPlanarTests` bounds the additional source-loop proof work cumulatively.
Coefficient limits apply to intermediate determinants, moment sums and
transforms, and cancellation is checked during source proof and integration.
Failures return no partial products or aggregate. `PlanarTests` is zero for
the previous integration route. Source face/product/body/path identifiers are
retained in the unchanged mass result shape.

## Independent qualification

Both exact rational frame conventions are tested on the original reentrant
OCCT product, wire-authored holed prism and Boolean-cut affine-pcurve prism.
Local moments through degree two are independently calculated as separable
Cartesian box integrals, by union of two disjoint boxes for the reentrant solid
and subtraction for the through-hole solid. Their volumes are 28 and 48,
respectively. The Boolean-cut source has independently measured OCCT area 120
and volume 48; surface area is not returned by this mass API.

The independent pinned OCCT mass oracle compares aggregate volume, centroid,
all nine origin-inertia entries and all nine central-inertia entries across
seven placed material occurrences, including oblique rotations and nonneutral
child frames. Native reload is exercised for the affine-pcurve source-bound
wrapper. Adversaries cover changed original hole orientation, cumulative
triangle/proof quotas, cancellation and coefficient growth.

```
PLAINSOLID_OCCT_RATIONAL_HIERARCHY_MASS=/path/to/pinned/oracle \
  go test -race ./exchange/step -run 'TestGeneralHierarchy.*Mass|TestRationalHierarchyMass'
```

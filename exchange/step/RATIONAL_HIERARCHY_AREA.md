# Exact source-aware rational hierarchy boundary area

`MeasureRationalHierarchyBoundaryArea` and
`MeasureOCCTRationalHierarchyBoundaryArea` independently re-prove the original
source and native local products before producing exact per-face, per-product
and per-occurrence areas. `AffinePCurveHierarchy.BoundaryArea` exposes the same
query for the source-bound Boolean-cut spline-pcurve family. Existing mass and
query APIs and native formats are unchanged.

## Exact supported geometry

For planar products, the existing general finite-face proof authenticates every
original face, outward support plane, directed outer loop and hole, including
its native vertex/edge correspondence and embedded closed material. The area
is the signed projected shoelace sum, multiplied by the exact normal sign and
one half. Outer loops contribute positively and holes negatively. A complete
face must have strictly positive area. There is no polygon tessellation,
absolute-value hole repair, quadrature or sampled acceptance.

The current source domain has signed-cardinal local plane unit normals.
World occurrence transforms may be genuinely oblique proper rational rotations;
their exact orthogonality preserves area. Consequently this domain needs no
irrational normalization or approximated radical. A general nonunit or arbitrary
non-cardinal source chart is not added by this API. Such input must first obtain
its own complete source proof; unsupported normals are rejected.

The independently authenticated analytic families use their natural exact
areas: finite cylinder caps `pi*r^2`, equal half/quarter lateral cells
`2*pi*r*h/cells`, sphere `4*pi*r^2`, and ring torus `4*pi^2*R*r`. Each source
face is classified from the already validated owning support and whole material
proof. This does not promote approximate curves or claim zero-discrepancy
rational evaluation of the analytic sphere/torus native angular patches.
All areas are immutable rational polynomials in pi in the common source unit
squared. `LengthUnit` is metres per source unit; multiply by its square for m².

## Ownership and aggregation

Every result retains original product/body/face/support/bound IDs and occurrence
paths. Planar `SourceBoundIDs` align with `SignedLoopAreas` in proved native loop
order (outer first); they are actual source IDs, not guessed record ordinals.
The private source proof now retains that exact loop-to-bound association.
Mutable evidence slices are independently copied between local products,
occurrences and successive calls. `PiPolynomial.Coefficients` returns owned
rational copies.

`Sum` is the sum of occurrence boundary areas. Repeated local assets are counted
once per use. Overlapping or contacting uses are not Boolean-unioned, and this
query does not establish pairwise disjointness or remove internal interfaces.
The same local exact area applies to each proper rigid occurrence.

Limits bound cumulative planar proof tests, original loop-edge integration and
intermediate rational coefficient sizes. The inherited source graph limits
also apply. Cancellation or failed source/ownership/positivity/budget checks
return no partial result.

## Independent checks

Both rational frame conventions are tested for the original box, reentrant
prism, wire-authored holed prism, and genuine OCCT Boolean-cut holed prism.
Their planar areas are respectively 52, 78, 120 and 120. The seven-occurrence
mixed hierarchies have exact area `A_planar + 72*pi + 24*pi^2`, including
independently owned cylinder, sphere and torus faces.

The pinned OCCT comparator verifies original STEP transferred aggregate area
and solid count for all eight cases. The affine-pcurve case is queried after
source-bound native save/reload. Tests cover signed hole contributions, exact
face totals, source/path ownership, evidence isolation, low budgets,
cancellation, forged ancestry, and unchanged point/line and mass regressions.

```
PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY=/path/to/pinned/oracle \
  go test -race ./exchange/step -run 'TestRationalHierarchyBoundaryArea|TestHierarchyArea'
```

The qualifying local comparator is `/tmp/plainsolid-independent-torus-assembly-oracle`,
using OCCT source pin `4dbfdcc448c0b3e565488afcd626c84483172db9`.

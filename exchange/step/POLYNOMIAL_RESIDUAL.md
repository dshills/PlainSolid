# Continuous polynomial spline support residual

`BoundPolynomialSplineSphereResidual` is an opt-in evidence API. It reads one simple nonrational STEP B-spline curve and bounds its sphere equation residual on every nonempty knot span. It does not construct or accept a BRep.

Poles and knots use exact source decimal rationals; the supplied center and radius use their exact represented float64 values. Exact de Boor evaluation at `2p+1` rational parameters reconstructs the degree-at-most-`2p` residual in the Bernstein basis by exact rational linear algebra. This is polynomial identity reconstruction, not sampled geometric acceptance. The convex hull of the Bernstein coefficients encloses the residual over the whole closed span. Dividing its absolute bound by the positive radius bounds the point-to-sphere radial distance because `abs(norm(C-center)-R) = abs(norm(C-center)^2-R^2)/(norm(C-center)+R)`.

For the retained pinned OCCT off-axis fixture, model curve #25 has 28 spans. The exact rational result proves a radial-distance upper bound below `3e-8` model units, while the exactness regression separately proves a nonzero residual. These statements are compatible: the source is close to the sphere but not identical to it.

Limits are degree 1–8, 512 poles, 64 spans, clamped knots, source decimals at most 64 characters with exponent magnitude at most 256 and numerator/denominator at most 1024 bits, and computed coefficients at most 16384 bits. General rational splines, periodic conventions and discontinuous interior knots are unsupported. Normal STEP parser limits also apply.

This result does **not** prove proximity to the cylinder, paired pcurve agreement, coverage of the true intersection, source edge ownership, closed-shell embedding or material equivalence. It does not authorize exact or approximate import. The output retains exact rational bounds; decimal rendering must use outward rounding if displayed as a certified upper bound.

## Cylinder evidence

`BoundPolynomialSplineCylinderResidual` uses the same bounded source decoder and Bernstein enclosure for `|d|²-(d·a)²/(a·a)-R²`, with `d=C(t)-origin`. The axis `a` is any finite nonzero represented vector. All projection arithmetic is rational; normalization and square roots are unnecessary. Exact scaling or sign reversal of the axis leaves the result invariant, including very small or large axis magnitudes.

On the same retained OCCT model curve #25 and the same 28 parameter spans, the cylinder radius 2, source origin `(1,0,-0.75)`, axis `(0,0,1)` certificate proves distance to the infinite cylindrical support below `8e-8` model units. Together with the sphere certificate below `3e-8`, this establishes continuous proximity to **each** support for every represented model-curve parameter.

It still does not bound distance to their intersection without a separate conditioning/transversality proof. It does not establish inverse parameter matching to either source pcurve, matching source parameter intervals, unique traversal or endpoint/branch correspondence. No ownership, closed-face coverage or material inference follows. The STEP body remains rejected by the exact analytic importer.

## Strict angular monotonicity

`BoundPolynomialSplineAzimuth` bounds `(x-origin.X)*y'-(y-origin.Y)*x'` with exact degree-at-most-`2p-1` Bernstein polynomials on every source span. The derivative is the exact spline derivative; source knots, poles and the origin remain rational. The initial API uses model-space X/Y, hence azimuth about positive Z. Strict positivity (or negativity) of all Bernstein bounds proves strictly increasing (or decreasing) continuously unwrapped azimuth. A zero-containing bound is unresolved, not a proof of reversal. The positive proof also excludes contact with the projected axis.

For the retained OCCT curve #25, all 28 spans have strictly positive angular-velocity numerators. Source start and end coordinates are exactly identical, with exact endpoint squared distance zero. Both facts are independently returned; the API does not infer a winding count. A positively traversed closed curve can wind several times, so one-turn coverage still requires a certified ray crossing count or equivalent global argument. The evidence also leaves pcurve inverse matching, source interval restrictions and topological ownership unresolved.

## Certified signed winding

`BoundPolynomialSplineWinding` adds a bounded global proof after requiring strict angular monotonicity and exact 3D source closure. It counts roots of `y-origin.Y=0` on the positive-X radial ray. Exact Bernstein sign variations prove absence of interior roots or exactly one interior root; rational midpoint subdivision separates ambiguous roots and proves the sign of `x-origin.X`. Strict angular velocity ensures every retained ray crossing is transverse. Shared knot and subdivision-boundary roots are deduplicated by exact rational parameter; the terminal source endpoint is excluded.

A crossing with equal bounds is an exact parameter. Unequal bounds denote an **open** isolating interval with exactly one interior root; endpoint roots are separate events. Results are ordered by parameter. Exhausted depth/cell/bit budgets return an error rather than a guessed count. Defaults are 64 subdivision levels and 65536 cells, with hard maxima of 128 and one million. The earlier source and coefficient budgets also apply.

The retained OCCT curve #25 has exactly one positive-ray crossing and therefore signed winding +1, certified over 28 processed source spans. Independent fixtures with two turns report +2, and reversed fixtures report −1 or −2. A curved fixture exercises genuine subdivision and rejects an insufficient depth budget. This closes the angular one-turn coverage question only; inverse pcurve matching, finite source interval ownership, support-intersection distance and BRep material validation remain separate obligations.

## Cylinder pcurve ancestry and angular range

`InspectCylinderPCurve` verifies a selected EDGE_CURVE → SURFACE_CURVE → two distinct PCURVEs graph. It requires exactly two face-owner uses with opposite normalized loop directions, one pcurve for each owning support, exactly one cylindrical owner, one containing closed shell and one manifold body owner. Detached extra oriented uses, duplicated owner support links and sharing the selected pcurves or surface curve with another source curve fail. This is selected-edge ancestry, not validation of the whole shell.

The cylinder pcurve must be a bounded simple polynomial 2D spline in a dimension-2 definitional context. Its exact source parameter interval must equal the 3D polynomial curve's domain. Exact derivative Bernstein coefficients certify strictly increasing or decreasing U. The reader retains source IDs, edge sense, domain, endpoint U values and per-span bounds. It does not assume equality of the interior 3D and 2D parameterizations merely because the domains match.

For the pinned off-axis fixture, edge #21 uses model #25 and cylinder pcurve #375 → parameter curve #382 on cylinder #376. The SURFACE_CURVE master is `PCURVE_S1`, sphere pcurve #196; this authority is retained explicitly rather than assigning it to the 3D curve or cylinder pcurve. All 28 U-derivative spans are strictly positive on `[0,1]`; U runs from 0 to the exact decimal `6.28318530718`. An exact rational Machin-formula enclosure of π proves `0 < (UEnd-UStart)-2π < 5e-13` radians. Thus the stored U interval is approximately one turn, not exactly one turn. The independent 3D curve winding proof remains exactly +1.

This evidence does not establish pointwise phase correspondence, pcurve-lift distance to the model curve, inverse interval matching, endpoint tolerance compatibility or material ownership. No source reconstruction or import has been enabled.

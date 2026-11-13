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

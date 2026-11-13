# Continuous polynomial spline support residual

`BoundPolynomialSplineSphereResidual` is an opt-in evidence API. It reads one simple nonrational STEP B-spline curve and bounds its sphere equation residual on every nonempty knot span. It does not construct or accept a BRep.

Poles and knots use exact source decimal rationals; the supplied center and radius use their exact represented float64 values. Exact de Boor evaluation at `2p+1` rational parameters reconstructs the degree-at-most-`2p` residual in the Bernstein basis by exact rational linear algebra. This is polynomial identity reconstruction, not sampled geometric acceptance. The convex hull of the Bernstein coefficients encloses the residual over the whole closed span. Dividing its absolute bound by the positive radius bounds the point-to-sphere radial distance because `abs(norm(C-center)-R) = abs(norm(C-center)^2-R^2)/(norm(C-center)+R)`.

For the retained pinned OCCT off-axis fixture, model curve #25 has 28 spans. The exact rational result proves a radial-distance upper bound below `3e-8` model units, while the exactness regression separately proves a nonzero residual. These statements are compatible: the source is close to the sphere but not identical to it.

Limits are degree 1–8, 512 poles, 64 spans, clamped knots, source decimals at most 64 characters with exponent magnitude at most 256 and numerator/denominator at most 1024 bits, and computed coefficients at most 16384 bits. General rational splines, periodic conventions and discontinuous interior knots are unsupported. Normal STEP parser limits also apply.

This result does **not** prove proximity to the cylinder, paired pcurve agreement, coverage of the true intersection, source edge ownership, closed-shell embedding or material equivalence. It does not authorize exact or approximate import. The output retains exact rational bounds; decimal rendering must use outward rounding if displayed as a certified upper bound.

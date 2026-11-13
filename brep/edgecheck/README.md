# Continuous rational edge agreement

`Check` compares rational spatial Bezier curves at identical normalized
parameters. `CheckPlanar` maps a UV curve through an affine planar chart and
compares it to a spatial model curve. `CheckPlanarPair` compares two UV curves
under separate affine charts. `CheckPlanarLine` compares against an analytic line
over an exact affine parameter interval, including reversed intervals.

The affine map `Origin + u*U + v*V` and line interval endpoints are formed in
exact homogeneous arithmetic. Intermediate model-space coordinates are never
rounded to float64. Positive weights are required; nil weights mean one.
Constant curves, differing degrees, common weight rescaling, and explicit
parameter reversal are supported. This proves same-parameter agreement, not
Hausdorff distance or agreement after arbitrary reparameterization.

Let `A=Xa/Wa`, `B=Xb/Wb`, `N=Xa*Wb-Xb*Wa`, and `D=Wa*Wb`. Positive weight control
hulls prove `D>0`. Bernstein products are computed with exact binomial factors.
The polynomial

`Q(t) = tolerance²*D(t)² - (Nx(t)²+Ny(t)²+Nz(t)²)`

is nonnegative exactly when the requested spatial tolerance holds. Nonnegative
Bernstein coefficients prove it continuously. Exact de Casteljau subdivision
refines ambiguous intervals. A negative evaluation gives an exact discrepancy
witness; no finite sample set establishes a certificate. An initial homogeneous
control-hull bound also provides a conservative global squared discrepancy bound.

Always check `Report.Status` even when error is nil:

- `Certified`: discrepancy is at most the supplied tolerance everywhere.
- `Exceeded`: an exact normalized parameter, rational points, and squared
  discrepancy prove a violation.
- `Unresolved`: work/degree/bit/subdivision limits prevent a decision. Equality
  at a nondyadic tangency may remain unresolved despite true agreement.

Invalid inputs/options and cancellation return errors. No partial certificate is
returned on failure. Rounded witness points are conveniences: they can coincide
or overflow when the authoritative exact witnesses do not. Returned rationals
are caller-owned. These guarantees concern real arithmetic on represented input
coordinates, not roundoff in a floating-point geometry evaluator.

Defaults: maximum degree 32 (configurable through 128), two million work units,
100,000 visited intervals, subdivision depth 48 (configurable through 256), and
16,384 bits per checked rational numerator/denominator. Temporary expressions can
exceed that stored-value bound. Product and subdivision work grows quadratically
with degree; cancellation is checked in their loops.

Tests cover exact identities across degrees and weight scales, parameter
reversal, affine chart pairs and analytic line ranges, equality and violation
inside intervals, exact chart mapping at float-collapse scales, positive-weight
rational curves, and independent 256-bit evaluation. A cubic fixture agrees at
both endpoints and the midpoint but violates tolerance between those samples.

# Certified rational spline derivative bounds

`Bound`, `Whole`, and `FromData` bound first and second partial derivatives of
clamped tensor-product rational B-spline surfaces. `Result.DU`, `DV`, `DUU`,
`DUV`, and `DVV` are conservative **Euclidean norm** upper bounds. Exact component
bounds are also returned as caller-owned rationals.

The certificate applies to real-arithmetic evaluation of the represented poles,
weights, and knots. It does not certify roundoff in `geom`'s float evaluator.
Weights must be strictly positive; the minimum active weight is an explicit
positive denominator certificate. Nil data weights mean all weights equal one.

For homogeneous controls `H=(X,w)`, derivative controls are computed exactly as
`p*(H[i+1]-H[i])/(k[i+p+1]-k[i+1])` and repeated for second/mixed derivatives.
Zero-support derivative controls are irrelevant to the represented functions.
Only controls whose support intersects the requested rectangle's interior enter
each convex-hull bound. This is conservative for trimmed rectangles without
requiring knot insertion, but it can be loose. Pole positions are translated by
one pole before homogeneous arithmetic; derivative bounds are consequently
insensitive to exact common translations.

Let `m>0` bound `w` below, `P` bound one translated position component, and
`X_u`, `w_u`, etc. denote homogeneous component absolute bounds. The certificate
uses the exact quotient identities and triangle inequalities:

- `|S_u| <= (X_u + P*w_u)/m`, similarly for `S_v`.
- `|S_uu| <= (X_uu + 2*|S_u|*w_u + P*w_uu)/m`.
- `|S_uv| <= (X_uv + |S_u|*w_v + |S_v|*w_u + P*w_uv)/m`.
- `|S_vv| <= (X_vv + 2*|S_v|*w_v + P*w_vv)/m`.

These formulas apply componentwise. The sum of squared component bounds is
exact. A candidate float norm is advanced upward until its exact squared value
is at least that sum, preventing a rounded-down certificate. Unrepresentable
finite float upper bounds return `ErrRange`.

Knots must be clamped; discontinuous interior knot multiplicities are rejected.
Every knot strictly inside the requested rectangle must provide at least C2
continuity. On the rectangle edges, derivatives mean one-sided limits from the
interior; lower continuity there is allowed. A float evaluator's branch choice
at an internal knot need not equal that one-sided derivative. Partition at such
knots before using these bounds for interpolation/tessellation estimates.

Defaults: 16,384 controls, degree at most 32 (configurable through 64), two million
work units, and 16,384 bits per checked stored rational numerator/denominator.
Temporary exact expressions may use more bits. Exhaustion/cancellation returns
no partial certificate. Curved trimmed regions can use an enclosing parameter
rectangle; arbitrary signed/zero weights and nonclamped supports are unsupported.

Tests include affine planes, bilinear mixed derivatives, exact quadratic second
derivatives, nonunit parameter intervals, rational patches, constant rational
geometry, active weight/support restriction, continuity rejection, outward norm
rounding at subnormal/overflow ranges, and independent sampled differentiation.

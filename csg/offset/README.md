# Orthogonal prism offsets

`Compute` offsets the **union** of orthogonal XY profiles with a shared extrusion
interval. It accepts concave outer loops, holes, overlapping profiles, and
multiple components. `FromSolid` promotes outward axis-aligned BReps with common
base/top Z planes through the orthogonal cell representation.

The metric is explicit: distance `r > 0` forms the Minkowski sum with the square
`[-r,r]²`; distance `-r` retains centers whose whole square lies in the original
footprint. Results are regularized (zero-area remnants disappear). The extrusion
height does not change. This is neither a circular Euclidean offset nor an
arbitrary-plane offset. Square-metric corners stay orthogonal. Notches and holes
can close, components can merge, and erosion can split or erase components.

The implementation builds an exact rational occupancy arrangement from the
represented input coordinates. Dilation unions expanded occupied rectangles;
erosion excludes expanded complementary rectangles and the outside of the
inset bounding box. Two-dimensional range accumulation avoids testing every
output cell against every expanded rectangle. Exact surviving cells are merged
into disjoint rectangles.

`Result.Cells` and `ExactArea` describe the authoritative exact footprint.
`Result.Solid` rounds grid coordinates to float64. Conversion must preserve
strict coordinate order and each coordinate's error bound (default 1e-9 model
units); otherwise it fails explicitly. Because the conversion is a strictly
monotone map of each axis, it preserves the arrangement's incidence and topology.
Underlying mesh/BRep validation can still reject extremely small features.
Point-only connections fail closed-manifold construction. No partial result is
returned on any failure. Rational result values are caller-owned.

Default budgets are 256 input segments (maximum 2048), 262,144 grid entries,
and 20 million work units. Work applies separately to the exact arrangement and
each downstream solid conversion pass. Input validation and conversion are
bounded but are not internally cancellable; exact grid loops check context.
`FromSolid` counts four segments per promoted occupied XY column, so highly
subdivided inputs may reach its segment limit earlier than direct profiles.
`FromSolid` rejects variable extrusion height, rotated prisms, and diagonal
edges in triangulated caps; use explicit profiles for those supported footprints.

Tests include analytic notch/hole closure, erosion splitting a connecting neck,
complete disappearance, overlapping-cross morphology, exact decimal-offset
area, topology-preserving float conversion, rejection of collapsed coordinates
and point contacts, promotion, and budget/cancellation failures.

### Exact rounded-cylinder blend extinction

`WrappedRoundedCylinder` also accepts inward offsets that exactly reduce the
smaller of two positive original rim-fillet radii to zero. The surviving radius,
cap planes and cylindrical radius remain exact represented values. The extinct
blend has no result face (`ResultBlends[end] == -1`); surviving faces retain their
original source-face IDs. No negative-radius continuation is accepted.

This is a convex erosion identity, not tolerance-based removal of a small face.
For inward distance `t`, the core has radius `R-t`, cap interval
`[zLower+t,zUpper-t]`, and rim radii `rLower-t,rUpper-t`. Its straight cylindrical
band and rolling-circle centers remain unchanged. Adding a radius-`t` ball to
this convex core reproduces the original two-rounded-rim body, including a sharp
core rim when its new radius is zero. Subtracting the ball in the convex support
inequalities therefore gives exactly the core. Existing geometric limits remain
in force; extinction with an originally sharp opposite rim is still rejected.

`ErodeWrappedRoundedCylinderPastBlend` is an explicit continuation for two
unequal positive original rim radii, with inward depth strictly between them.
It composes exact extinction with a sharp/rounded-core erosion; the erosion
semigroup proves the result equals erosion of the original body. All stage
coordinates must remain exactly representable. The surviving opposite blend
keeps its original face ancestor. `SharpRim` reports the original lateral, cap
and extinct blend IDs, exact resulting radius/height, and the shared result rim
edges in cap traversal order. The extinct blend is contact history only, not a
surviving face ancestor. The second extinction and deeper offsets remain outside
this entry point. Work limits apply per source proof and per each of two fixed
stages; failures expose no intermediate body or history.

`ErodeWrappedRoundedCylinderToSharp` handles the next transition separately:
with two originally positive blends and depth at least their maximum, both
rounded support inequalities become redundant. It returns the exact sharp
cylinder at radius `R-depth` and cap interval `[lower+depth,upper-depth]`, provided
radius and height remain strictly positive and the native construction is exact.
Equality at the larger blend radius is accepted; zero-size bodies are rejected.
Both original blends occur only in the two rim contact records, while the three
retained faces map only to the original lateral wall and caps. Neither prior
entry point changes its boundary policy. A nonrepresentable resulting coordinate
is rejected rather than rounded into a different erosion.

### Independent circular-frustum support setbacks

`InsetWrappedCircularFrustumSupports` edits a validated native circular frustum
with independent inward lateral-normal, lower-cap and upper-cap distances.
For source radius `r0+s*(z-z0)`, the result is exactly the support intersection
`r <= r0+s*(z-z0)-lateral*sqrt(1+s*s)` between the independently shifted cap
planes. Equal distances recover whole-body ball erosion; unequal distances are
selective support edits, not a claim of constant Euclidean boundary distance.

The normal scale must be rational, retained endpoint radii and height strictly
positive, and native output coordinates exactly representable. Original circular
weights and circumferential basis survive exact draft/reheight operations.
History identifies the three original faces, exact resulting intersection
radii/heights and both shared rim edge lists. Limits apply separately to the
fixed proof/edit/construction calls; all failure paths return no partial body.

`InsetWrappedCircularFrustumTube` applies independent material setbacks to a
validated coaxial circular through-bore: the outer wall shrinks, the inner wall
expands, and both annular cap planes move inward. Each wall uses its own exact
normal scale. The original native wall and annular-cap identities are recovered
by full bore validation, rather than inferred from two isolated surfaces.
Strict positive radial clearance at both retained endpoints proves separation
throughout because the two radii and their difference are affine in height.
History retains the original face IDs and exact endpoint clearance witnesses.
Blind/cavity/eccentric inputs, contacting walls, collapsed intervals and
nonrepresentable native results are rejected without a partial result.

`InsetWrappedEccentricCircularFrustumTube` additionally permits distinct parallel
wall axes. It retains both authored centers and applies the same exact normal
setbacks. For each endpoint, a positive radius difference `g` must satisfy
`g*g > dx*dx+dy*dy`; this avoids rounding even when the axis distance is
irrational. The affine radius difference then exceeds the fixed axis distance
throughout the interval. Exact tangency and crossings fail. The existing coaxial
entry point remains restricted to coincident axes. In eccentric history,
`RadialClearance` is the radius difference, not the shortest physical wall gap.

### Source-preserving axial tube resizing

`ResizeWrappedCircularFrustumTube` trims or extends a validated circular
through-bore to two authored model-Z cap heights. Both original affine radius
functions, centers and circular parameterizations are retained, including for
eccentric parallel axes. This is cap relocation, not wall offset or a claim that
an extended result remains inside the original body. It does not require a
rational normal length: the wall equations are evaluated exactly at the new
ends. Positive end radii and exact squared endpoint clearance prove the complete
new annular interval. Original cap/wall ancestry survives; apex/contact,
nonrepresentable geometry and unsupported source topology fail atomically.

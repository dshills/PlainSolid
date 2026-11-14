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

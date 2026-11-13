# Standalone exact plane supports

`ReadPlanarSupports` and `WritePlanarSupports` exchange IGES190 form1 supports
with uniquely owned type116 locations and two type123 directions. The explicit
normal and reference must be perpendicular signed coordinate unit vectors.
`Geometry` returns `geom.PlaneSurface` with U along the reference direction and
V along normal×reference, both in raw coordinate distance. No normalization,
projection, unit rescaling or tolerance-based frame repair occurs.

These are unbounded supports, not trimmed faces or solids. Form0 implicit
frames, type108 plane equations, transforms, topology, shared dependencies and
unused entities are rejected. Original geometry decimals must convert exactly
to float64; staged writer read-back enforces the same rule. All four labels and
subscripts and the explicit length unit survive. Presentation styling is not
part of the descriptor; directory geometry usage status is emitted canonically.

The separate private parser mode accepts at most 64 roots and 256 entities plus
ordinary byte, record and value limits. Existing cylinder and topology modes
retain their previous type/form gates. Malformed inputs return an empty model;
writers validate the whole model before caller I/O and propagate short writes.

The retained fixture is independently authored with OCCT's type190 constructor.
Optional `PLAINSOLID_OCCT_IGES_PLANE` tests compare source and Go-authored
supports through OCCT analytic plane transfer for both normal signs, nonzero
location and a nondefault reference direction. Position and both derivatives
are checked; no bounded-face transfer or material validity is implied.

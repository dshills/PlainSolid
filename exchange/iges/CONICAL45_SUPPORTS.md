# Exact 45-degree IGES conical support

`ReadConical45Supports` / `WriteConical45Supports` accept IGES194 form1 with
exactly 45 degrees semi-angle, positive reference-circle radius, and uniquely
owned location116 / axis123 / reference123 entities. `Center` is the center of
the reference circle, not the apex. Frames are perpendicular signed coordinate
unit directions, source scalars are decimal-to-float64 exact, and names,
subscripts and explicit length units are retained without coordinate rescaling.

`Geometry` returns an immutable `geom.Surface` evaluator preserving source U
(azimuth in radians) and V (signed generatrix distance from that circle). With
q=1/√2, its defining expression in the explicit frame is
`((R+qV) cos(U), (R+qV) sin(U), qV)`. This retains the opposite nappe when
R+qV is negative and the singular apex at V=-√2 R. Evaluation and derivatives
use ordinary floating-point elementary functions; source acceptance does not
use sampled geometric equality. No finite trim or material closure is inferred.

The existing `geom.ConeSurface` instead uses nonnegative axial height from its
apex. Returning it as the complete IGES support would lose the opposite nappe.
It is deliberately not substituted. On the positive nappe only, the exact
parameter relation would be H=R+V/√2 with apex Center-R Axis. This codec does
not silently convert or round that relation.

There is no lossless STEP cone bridge in this milestone: STEP's radians would
require π/4, which cannot be encoded as an exact finite decimal. Other IGES
angles, radius zero, form0, transforms, topology, sharing and unused entities
are rejected. Limits are 64 supports / 256 entities plus parser limits. Output
is staged and strictly re-read before caller I/O.

An independently authored OCCT194 fixture and optional
`PLAINSOLID_OCCT_IGES_CONE` helper verify source and Go-authored files on both
nappes, both axis signs, and a nondefault reference ray, comparing positions and
both partials. This is analytic support qualification, not face/solid transfer.

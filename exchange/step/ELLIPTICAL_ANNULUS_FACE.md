# Original STEP elliptical face with an owned inner boundary

`ReadEllipticalAnnulusFace` is a separate exact reader for the genuine OCCT
66-entity single planar face with two concentric, aligned elliptical boundaries.
It returns one native trimmed face, both original model ellipses and vertices,
source graph, units, name and separate inner/outer edge and pcurve ancestry.
The unholed disk reader and its persistence format remain strict.

The source outer FACE_BOUND has orientation true; the inner FACE_BOUND has
orientation false. Both underlying ORIENTED_EDGE and EDGE_CURVE senses are
true. Each closed edge has its own original vertex, 3D ellipse, SURFACE_CURVE,
PCURVE, definitional representation, 2D ellipse and parameter context. Only the
original analytic PLANE is shared. The reader checks these complete distinct
owner paths, not just the two ellipse radii or a sampled boundary.

Every source frame is a matching exact cardinal frame. Model and UV centers,
axes and radii prove continuous plane-lift equality for each natural ellipse.
All geometric decimals are required to be exactly represented. The vertex of
each closed edge is proved to be its positive major-axis point. There is no
rounded angular endpoint, trim repair, fitting, or tolerance-based sewing.

Let outer radii be (a,b), inner radii (c,d). Each major radius must exceed its
positive minor radius, and c<a, d<b. Concentric aligned containment follows
exactly because every inner point satisfies

`(x/a)^2 + (y/b)^2 = (c/a)^2 cos²(t) + (d/b)^2 sin²(t) < 1`.

Both loops are simple; their strict separation and opposite effective winding
prove the planar annulus domain. Touching, crossing, coincident, offset, rotated
or unsupported source loop encodings fail. The accepted subset includes
nonhomothetic inner ellipses when both strict inequalities hold.

Native local edge roles are outer 0 and inner 1. The native inner UV use runs
from full turn to zero and marks the model use reversed, reflecting the source
FACE_BOUND reversal without changing its forward original model ellipse.
This is one bounded face, not a closed material solid. The mathematical natural
curve proof is separate from numerical trigonometric evaluation roundoff.

`WriteEllipticalAnnulusFaceSource` re-proves and emits the complete original
source; it does not propagate edits to decoded fields. The dedicated version 1
`WriteEllipticalAnnulusFaceJSON` / `ReadEllipticalAnnulusFaceJSON` persist the
native plane, two model curves, both UV loops, original STEP, vertices and owner
IDs. Reload re-proves the source and compares all stored native fields. Wrong
hole orientation, substituted outer model curves, stale source IDs and forged
radii fail atomically. Reload and rewrite are canonical-byte stable. The nested
outer geometry manifest is not an independently valid unholed disk envelope.

STEP is bounded at 64 KiB, 66 entities, 4096 values and depth 16; native JSON at 256 KiB.
Caller-specified smaller limits apply. Source tree traversal before writing is
bounded, including cycles and strings. Millimeter/radian/steradian context and
original uncertainty metadata are validated; uncertainty is never used to alter
geometry. Names and original graph IDs are retained.

## Independent evidence

`tools/occtref/step_ellipse_annulus.cxx` uses OCCT MakeEdge/MakeWire/MakeFace with
an outer ellipse (4,2), a reversed inner ellipse (2,1), center (1,2,3), normal +Y,
reference −Z. The retained `occt-elliptical-annulus-face.step` is the original
OCCT export, with only trailing line whitespace removed.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 conventionally transfers both
source and STEP→native JSON→reload→STEP output as a valid one-face/two-edge
shape. Area is 6π, centroid (1,2,3), normal +Y, and total adaptive boundary length
29.065344661643032. Native model point/tangent and the inner reversed lift are
checked; independent Simpson integration checks boundary length.

Negative tests include cross-owned pcurves/representations, swapped bounds,
incorrect bound or coedge sense, mismatched original vertices and frames,
nonexact coefficients, consistently authored touching/crossing/coincident
ellipses, stale native geometry/ancestry and resource limits. The focused fuzz
campaign ran 100 inputs. Optional live test variable:
`PLAINSOLID_OCCT_ELLIPSE_ANNULUS`.

# Exact quadratic planar NURBS STEP

`ReadQuadraticNURBSPlanarAssembly` adds an explicit original-source representation
class to the existing exact oblique polynomial planar material workflow. It
accepts original unweighted quadratic 3×3 tensor Bézier supports and three-pole
quadratic UV Bézier pcurves, with two strictly increasing knots and clamped
multiplicities three. Existing degree-one model curves may remain. Other degrees,
rational weights, internal knots and genuinely curved supports are not admitted
by this additive route. The previous unit/scaled/oblique readers remain strict.

For each support, every original rational-decimal pole must equal
`P00 + (i/2)(P20−P00) + (j/2)(P02−P00)`. This proves the entire tensor polynomial
is affine, including the fourth corner; checking corners alone is insufficient.
Every UV curve's middle pole must equal the exact endpoint midpoint, proving
identical parameterization over its original knot interval. Arithmetic starts
from bounded exact original numeric tokens, not rounded binary64 observations.
Each control point has exclusive source ownership. Only after those identities
are proved are the original IDs privately reduced to the equivalent bilinear /
degree-one basis, and the now-unused exclusively owned interior pole records
removed from that private view.

The existing oblique material reader then proves original continuous model and
owner-pcurve agreement, full directed closed topology, exact product-coordinate
recovery and globally embedded native planar boundaries. Original colors,
product identities and occurrence geometry remain authenticated by that reader.
The returned `Geometry.Assembly.Source` is the complete original graph, not the
private reduced graph. Evidence retains original support/face IDs, all nine
support pole IDs/coordinates, both knot domains, and each quadratic curve's
original poles/knots and PCURVE/surface/model-edge owner IDs. Evidence getters
return copies. Native planar geometry is the proved original material; no
curved-geometry tessellation or tolerance fitting is involved.

`WriteQuadraticNURBSPlanarAssemblySource` re-proves and exports original STEP
bytes, rejecting stale supplied geometry/evidence. Native `Write/Read...JSON`
uses distinct version-one tag `plainsolid-step-quadratic-nurbs-planar-assembly`.
It stores original source, exact basis evidence, original product/appearance
claims and native planar boundaries. Reload reconstructs from original source
once and compares all stored claims; cached geometry and alleged identities are
never authorities. Input ceilings are 1 MiB source, 4096 STEP entities, 65536
values, depth 16 and the underlying bounded material limits; native envelopes
are at most 16 MiB. Raw pole/degree/knot rationals are bounded to 256 bits before
the constant-size identity arithmetic. Failure returns no partial model.

## Independent qualification

`tools/occtref/author_ap242_quadratic_nurbs_planar.cxx` independently converts two
OCCT boxes to NURBS, changes their chart domains, raises both surface degrees
and all original pcurve degrees to two, applies a non-cardinal exact-compatible
3:4:5 product transformation, and writes genuine XCAF AP242 with shared uses
and two product colors. It does not read Go-generated geometry. The retained
789-record source contains 12 original quadratic supports and 48 quadratic
pcurves; its ordinary and shifted UV-domain variants both pass live
source → strict Go → native reload → original STEP → XCAF qualification.
XCAF independently observes volume 1750, area 1350, three solids, eighteen faces,
centroid `(69/14,66/7,99/14)` and both colors. Native local volumes/areas are
750/550 and 250/250 exactly.

Build with:

```
sh tools/occtref/build_author_ap242_quadratic_nurbs_planar.sh OCCT_INCLUDE OCCT_LIB OUTPUT
```

Set `PLAINSOLID_OCCT_QUADRATIC_NURBS_PLANAR=OUTPUT` for the optional live test.
The comparator is qualified against OCCT source commit
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

Adversaries change an interior support pole while leaving all corners unchanged,
add a sub-binary decimal perturbation, alter pcurve midpoint/knot interval or
owner, share a pole illegally, use an almost-integer source degree, or supply
underflowing numeric text. All reject. Native tests additionally mutate original
pole/edge identities, exact evidence, placement and cached native coordinates;
check old tags, limits, cancellation, short writes and trailing payloads; and
verify stable bytes and original source preservation. Reordered source records
are also accepted without deriving geometry roles from record order.

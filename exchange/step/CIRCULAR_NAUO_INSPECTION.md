# Genuine XCAF circular-product inspection

`InspectCircularNAUOProduct` is a bounded source-evidence API, **not** a mixed
exact material importer. It selects an original product definition ID in a
structurally validated NAUO assembly and inspects its single three-face cylinder
body. The existing exact planar importers remain strict.

Supported inspection requires a signed-coordinate cylindrical support, two
coaxial circle rim edges, matching planar caps, and a twice-used straight seam.
Original EDGE_CURVE, SURFACE_CURVE / SEAM_CURVE, both per-owner PCURVEs, plane and
cylinder chart frames, face/coedge senses and connected directed endpoint IDs
are checked. The body has two opposite uses for each edge (including the side's
two seam uses). Original product/body/shell/face/edge/vertex/pcurve IDs and direct
NAUO uses are retained. Geometric dependency sharing with another body product
is rejected. The owning representation must declare unprefixed radians; the
normal assembly unit and parser limits apply. Inspection also caps the source at
8192 entities, eight products and 32 direct assembly uses. Only this selected local body's
sites are inspected: other product geometry and global occurrence material are
not certified by this API.

For each rim, the full model CIRCLE agrees exactly with both the cylindrical
horizontal U-line lift and planar CIRCLE lift under the supported canonical
frames. This is a full-support functional identity, not an inferred finite trim
interval. The topological vertex's signed plane residual and squared-radius
residual are then computed from original decimal numbers using bounded rational
arithmetic. The straight model seam is parallel to the axis and meets both source
vertices; its constant squared-radius residual is reported independently.

Both source seam pcurves are retained in original association order. The admitted
layout has the first positive U branch and the second U=0 branch, with V-directed
unit lines and V-origin zero. Their exact decimal angle difference is compared
with a rigorous rational enclosure of 2π using the existing Machin alternating
series bounds. `TurnDefectLower/Upper` enclose that **one-turn** discrepancy.
If the rational enclosure contains zero, that comparison is inconclusive; it
is not proof of exact periodic equality. No branch is silently swapped or
periodically rephased.

`FailedConditions` identifies nonzero vertex/support residuals, the nonzero seam
support residual, and a provably nonzero one-turn seam discrepancy. Malformed
ownership, unsupported frames or graph layouts return zero evidence and an
error; valid-but-inexact source data produces a diagnostic report. An empty
failure list still would not certify finite trim selection, lifted closure,
global embedding or material. Source uncertainty metadata is not treated as an
acceptance tolerance. There is no repair, snapping or output solid.

## Independent source and the exactness boundary

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` authored the 503-entity
`testdata/occt-circular-nauo.step` using
`tools/occtref/author_circular_nauo.cxx`. It contains two occurrences of an original
2×3×4 box and one placed radius-2, height-5 cylinder, written by real XCAF. Only
trailing line whitespace was removed.

The cylinder is product #389 / body #395. Its rim vertices #402 and #436 are
(2,-4.898587196589E-16,5) and (2,-4.898587196589E-16,0), respectively. Their exact
squared-radius residual is `(4.898587196589E-16)^2`, strictly positive. The model
seam #435 has the same whole-line residual. Its two original pcurve U values are
`6.28318530718` and `0`; the report proves their positive difference from 2π.
Thus this actual file cannot be imported as an exact source-equivalent native
cylinder merely by assuming its named analytic supports determine the vertices.

Independent ordinary OCCT TransferRoots/BRepCheck accepts that source under its
own numerical semantics: three solids, 15 faces, volume `48+20π` and area
`104+28π`. The optional test uses `PLAINSOLID_OCCT_DISK_BOUND_MASS`. That acceptance
is recorded separately from exact native source validity.

Tests retain all four failed conditions, reject incorrect owner/radius/coedge/
seam/unit/product linkage and budgets, and show that making model vertices exact
in an explicit test mutation does not hide the remaining periodic discrepancy.
The original source is unchanged. Normal/race/Go 1.23/vet and fixed-work
`FuzzInspectCircularNAUO -fuzztime=100x` qualify bounded inspection; no widened
mixed material acceptance is claimed.

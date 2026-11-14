# Circular conic annular prism material

`ReadConicCircularAnnularPrismMaterial` and
`WriteConicCircularAnnularPrism` authenticate a separate canonical28-record
IGES186 solid with two circular type104 directrices, two type122 lateral walls,
two annular type190 caps and explicit502/504/508/510/514 topology. Each cap owns
a positive outer loop and negative inner loop; the shell reverses the inner
wall. No type192 inference or tolerance repair occurs.

Both circles must be centered, aligned, exactly representable and strictly
nested:0<innerRadius<outerRadius. The heights have an exact positive difference.
Source conic coefficients have A=C>0, B=D=E=0, F<0. All original four rim curves,
two seams, cap ownership, source roles and numeric records are retained.

Proof uses the same explicit positive-determinant normalization y→y/2 as the
filled conic prism. Each104 coefficient C changes exactly to4C; every other
coordinate-bearing record is first proved invariant, including all four source
vertices and both wall terminals. The unchanged elliptical annular reader then
proves full support/edge/topology agreement and strict continuous containment.
The inverse affine map preserves the original nested closed material. Native
promotion uses actual circular rational quarter controls and exact annular cap
trims via additive `ellipticprism.CircularAnnulus`; no elliptical native support
is retained as a substitute. Existing ellipse and circular codecs stay strict.

Native topology has ten faces: two annular caps, four outer quarter walls and
four reversed inner quarter walls. `NativeRimEdges` and `SourceRimCurveIDs`
separate the partitioned native rims from original full source rims. Version1
source-bound native JSON regenerates geometry and ancestry and rejects forged
records. Source caps are32KiB,512records,28entities,2048values; native envelope
cap is512KiB, additionally constrained by caller limits. Current API requires
canonical record and incidence order and unplaced geometry.

## Qualification

`author_iges_conic_circular_prism.py --annular` independently specializes the
retained hand-authored hollow source to exact radii5 and2 at Z3 and7. The pinned
OCCT comparator transfers both original source and Go output as valid four-face,
one-solid material. Independent expected area is98π, volume84π and centroid
(0,0,5). Native mass agrees; all continuous edge discrepancies are exactly zero,
native JSON reload succeeds, and a closed mesh is verified at deflection0.1
with bounded256subdivisions/1million triangles. A smaller tessellation budget
fails explicitly rather than returning incomplete geometry.

Tests reject touching/reversed/degenerate radial nesting, incorrect inner cap
use, noncircular conics, noninvariant coordinates, forged native binding,
nonfinite authoring and exhausted limits. Existing elliptical input is rejected
by this API, and the existing ellipse API rejects its circular source.
`FuzzConicCircularAnnularPrism` retains the independent positive source seed.

## Original reordered graph

`ReadReindexedConicCircularAnnularPrismMaterial` adds the previously authenticated
record/shell-face/edge/vertex permutations and cyclic lateral-loop starts to this
same circular two-wall material. It shares source normalization and whole-graph
mapping with the filled circular variant; each route invokes its own canonical
material validator. It cannot admit a filled, placed or elliptical source.

The nested `Material` retains canonical circular native geometry and ancestry;
the wrapper exposes original `SourceForCanonical`, `SourceVertexOrdinals`,
`SourceUseOrdinals`, `NativeFaceSourceIDs` and all four `SourceRimCurveIDs`.
`WriteReindexedConicCircularAnnularPrismSource` revalidates these bindings and
writes the original bytes. Its separate version1 native envelope contains the
original source and canonical material recipe, bounded to1MiB; reload rebuilds
all mappings and rejects stale geometry or ancestry.

`reorder_iges_elliptical_prisms.py --loops --circle-annular` independently
permutes all supported topology levels in the retained exact circular source.
The pinned OCCT comparator accepts original/re-exported bytes with four faces,
one solid, area98π and volume84π. Tests additionally reverse a genuine inner
cap coedge sense and require rejection; source and native round trips retain
original wall/rim/use identity. The bounded seeded target is
`FuzzReindexedConicCircularAnnularPrism`.

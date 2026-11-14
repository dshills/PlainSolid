# Exact analytic elliptical prism IGES topology prerequisite

`ReadEllipticalPrismTopology` / `WriteEllipticalPrismTopology` authenticate and
emit a bounded model-only20-record186/514/510/508/504/502 graph. Original model
supports are two full104 elliptical rims, one110 axial seam, one122 extrusion
of the lower104 and two190 planes. Each original vertex, edge index, ordered
coedge, surface reference, shell/face sense and record is compared against a
canonical graph recovered from the original raw conic coefficients and heights.
Original parameters/metadata and source role IDs remain inspectable. The writer
authors a fresh canonical graph from its descriptor; it does not claim to
reproduce arbitrary source labels or record ordering.

The exact bounded domain is a centered ellipse with major axis+X, normal+Z and
positive height. It uses the existing exact104 coefficient/radius gate. Height
subtraction must be lossless. The two complete rims each share a single exact
seam vertex; the seam joins those vertices. Side loop uses upper−, seam−,
lower+, seam+. Top cap uses upper+ and bottom cap lower+, with bottom face
reversed by the shell. Thus every physical edge has two opposite material uses,
including the same-side seam. The122 terminal point is the upper seam vertex,
so its full support restricts exactly to both104 rims. Plane restriction and
full positive elliptical disk selection are exact. No source pcurves are
present: the codec does not invent their parameter values or accept arbitrary
trim coverage. Unsupported alternate representations fail closed.

`ExactLateralPatches` constructs the descriptor's side support as four immutable
rational degree(2,1) patches. For an unchanged accepted descriptor, the complete
C(t)+vD identity proves agreement with the source122, with exact joined cardinal
endpoints. This is a topology/geometry transfer prerequisite: it deliberately
returns no native `trimmed.Solid`, does not assert native sewing/persistence of
the20-record B-rep, and does not route into generic material APIs.

Independent positive: `author_iges_elliptical_prism_topology.py` constructs the
standard analytic graph without Go helpers. Pinned OCCT
4dbfdcc448c0b3e565488afcd626c84483172db9 ordinary TransferRoots/BRepCheck accepts
source and generated output as three faces and one solid. The standalone
`iges_elliptical_prism_topology.cxx` integrates original surface derivatives
independently (32768-step Simpson integration on the elliptical lateral chart):
area154.88430345445522, volume125.66370614355492, centerZ5, agreeing with
20π+4*ellipsePerimeter(5,2) and40π. Default OCCT surface quadrature differs by
about0.012 here; adaptive GProp also showed a larger error, so neither is used
as the stringent area oracle. Its reported default volume is125.66370614359175.

Independent negative: `author_iges_vendor_elliptical_prism.cxx` builds the same
solid via native OCCT MakePrism and IGESControl mode1. The retained genuine
export has27 entities and degree6 polynomial126 approximations with42 poles,
not exact104 model rims. The first/last source pole Y values are
−1.110223025E−16 and−4.440892099E−16, while502 seam vertex Y is0. Rounded knots
include6.283185307. These are actual source mismatches, not exact conic equality;
no rounding repair or native solid promotion is attempted. This fixture remains
explicitly rejected.

Limits:32KiB,512records,20entities,2048values; context cancellation before/after
bounded parsing and exact geometry's own budgets. Staged writer re-import,
mutation tests for rim ownership/vertex incidence/coedge and shell senses,
normal/race/Go1.23/vet and100x fuzz. Optional live variable:
`PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY`.

## Native exact material promotion

`ReadEllipticalPrismMaterial` now authenticates that same strict source and
constructs a native `trimmed.Solid` directly through `internal/ellipticprism`.
The internal builder is shared with existing STEP codecs; no STEP text or
format dependency is introduced. It uses exact rational ellipse quarters with
weights(1,1,2), exact representable translated poles, four tensor side patches
and two analytic planar caps. Continuous model/pcurve edge identity and native
closure are proved by existing exact sewing. The positive-height extrusion of
one simple ellipse gives the constructive material orientation and embedding;
no topology-only inference or vendor approximation is promoted.

Native face order is lower cap,upper cap,four sides, mapped separately to source
face IDs35,33,31,31,31,31. Two arrays of four native rim edges retain the partition
of source full rim curves9,11. Original source edges/face/entity IDs stay in the
`Topology` record and are not overwritten by native partition IDs. A positive
caller tolerance controls native validation only; it cannot relax the strict
source predicate or change exact control geometry.

`Write/ReadEllipticalPrismMaterialJSON` persists a version1 source-bound recipe
and the complete native trimmed JSON. Read independently re-proves source,
rebuilds canonical native geometry and matches the supplied native JSON plus
all source parameters/metadata and partition ancestry. Changed controls,
ancestry, tolerance or format cannot bypass this reproof. Envelope512KiB maximum
(and caller MaxBytes), source retains the smaller topology limits. Invalid data
returns no partial material. Stable canonical bytes, plain trimmed reload,
zero continuous edge discrepancies, closed tessellation, analytic volume/area
and centroid are exercised. Independent OCCT also loads IGES generated after
material JSON reload. Original STEP exact/repaired/Pythagorean builder paths
remain regression tested after the shared code extraction.

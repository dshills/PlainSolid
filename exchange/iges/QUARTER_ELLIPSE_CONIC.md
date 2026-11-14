# Exact IGES 104 cardinal quarter ellipses

`ReadQuarterEllipseConic` and `WriteQuarterEllipseConic` are separate from the
full-conic APIs. They retain the exact coefficients, units, metadata and
optional owned proper signed-coordinate type 124 of the full ellipse atom. Source
endpoints must be exactly adjacent cardinal ellipse points in positive local
orientation: +X→+Y, +Y→−X, −X→−Y or −Y→+X. No arbitrary angular endpoints,
complementary 270° arcs, full loops or tolerance repair are accepted.

`StartQuarter` records the source role; `AngularRange` exposes its natural
radian interval numerically. Its multiples of π are descriptive values, not
source decimals used for exact endpoint acceptance. `ExactCurve` gives an
immutable rational quadratic on [0,1] with homogeneous weights (1,1,2). The exact
normalized coordinates are ((1−t²)/(1+t²),2t/(1+t²)), rotated by the cardinal
quarter and scaled by the original semiaxes. Consequently its angular mapping
is qπ/2+2atan(t), with positive derivative 2/(1+t²). This proves the entire
conic image and directed quarter coverage, without sample-based acceptance.
The spline parameter is explicitly not the native conic angle.

Proper spatial frame reversal can reverse apparent winding while retaining a
positive local arc. The independent reverse fixture uses a flipped normal,
local −Y→+X, and traverses the same world quarter as the forward +X→+Y fixture
with endpoints swapped. Their rational parameter correspondence is
s=(1−t)/(1+t), not 1−t. Swapping the original source endpoints alone instead
selects the complementary arc and is rejected. Negative-determinant type 124
reflection, shear and scale remain unsupported.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 independently authors both
variants. Forward world endpoints are (1,7,3)→(1,2,5); the reverse is precisely
(1,2,5)→(1,7,3). Both original files and Go output for all four quarter roles
agree with OCCT on angular range, endpoints, point and derivative. Native
quadratic controls also retain exact nonbinary intermediate points; an explicit
binary64 ExactPoint conversion fails rather than rounds these values.

The codec retains 16 KiB/128-record/2-entity/256-value limits. Native exact curve
construction honors the supplied coefficient/work budget. Focused normal,
race, Go 1.23, vet and 100 fuzz executions cover all quarter roles, the unchanged
full reader, complete/partial/complement endpoint adversaries, exact reversal
endpoints and caller-visible parameter semantics. Optional live qualification
uses `PLAINSOLID_OCCT_IGES_QUARTER_ELLIPSE_CONIC` and
`tools/occtref/iges_quarter_ellipse_conic.cxx`. No face or material is implied.

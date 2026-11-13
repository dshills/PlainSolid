# Continuous polynomial spline support residual

`BoundPolynomialSplineSphereResidual` is an opt-in evidence API. It reads one simple nonrational STEP B-spline curve and bounds its sphere equation residual on every nonempty knot span. It does not construct or accept a BRep.

Poles and knots use exact source decimal rationals; the supplied center and radius use their exact represented float64 values. Exact de Boor evaluation at `2p+1` rational parameters reconstructs the degree-at-most-`2p` residual in the Bernstein basis by exact rational linear algebra. This is polynomial identity reconstruction, not sampled geometric acceptance. The convex hull of the Bernstein coefficients encloses the residual over the whole closed span. Dividing its absolute bound by the positive radius bounds the point-to-sphere radial distance because `abs(norm(C-center)-R) = abs(norm(C-center)^2-R^2)/(norm(C-center)+R)`.

For the retained pinned OCCT off-axis fixture, model curve #25 has 28 spans. The exact rational result proves a radial-distance upper bound below `3e-8` model units, while the exactness regression separately proves a nonzero residual. These statements are compatible: the source is close to the sphere but not identical to it.

Limits are degree 1–8, 512 poles, 64 spans, clamped knots, source decimals at most 64 characters with exponent magnitude at most 256 and numerator/denominator at most 1024 bits, and computed coefficients at most 16384 bits. General rational splines, periodic conventions and discontinuous interior knots are unsupported. Normal STEP parser limits also apply.

This result does **not** prove proximity to the cylinder, paired pcurve agreement, coverage of the true intersection, source edge ownership, closed-shell embedding or material equivalence. It does not authorize exact or approximate import. The output retains exact rational bounds; decimal rendering must use outward rounding if displayed as a certified upper bound.

## Cylinder evidence

`BoundPolynomialSplineCylinderResidual` uses the same bounded source decoder and Bernstein enclosure for `|d|²-(d·a)²/(a·a)-R²`, with `d=C(t)-origin`. The axis `a` is any finite nonzero represented vector. All projection arithmetic is rational; normalization and square roots are unnecessary. Exact scaling or sign reversal of the axis leaves the result invariant, including very small or large axis magnitudes.

On the same retained OCCT model curve #25 and the same 28 parameter spans, the cylinder radius 2, source origin `(1,0,-0.75)`, axis `(0,0,1)` certificate proves distance to the infinite cylindrical support below `8e-8` model units. Together with the sphere certificate below `3e-8`, this establishes continuous proximity to **each** support for every represented model-curve parameter.

It still does not bound distance to their intersection without a separate conditioning/transversality proof. It does not establish inverse parameter matching to either source pcurve, matching source parameter intervals, unique traversal or endpoint/branch correspondence. No ownership, closed-face coverage or material inference follows. The STEP body remains rejected by the exact analytic importer.

## Strict angular monotonicity

`BoundPolynomialSplineAzimuth` bounds `(x-origin.X)*y'-(y-origin.Y)*x'` with exact degree-at-most-`2p-1` Bernstein polynomials on every source span. The derivative is the exact spline derivative; source knots, poles and the origin remain rational. The initial API uses model-space X/Y, hence azimuth about positive Z. Strict positivity (or negativity) of all Bernstein bounds proves strictly increasing (or decreasing) continuously unwrapped azimuth. A zero-containing bound is unresolved, not a proof of reversal. The positive proof also excludes contact with the projected axis.

For the retained OCCT curve #25, all 28 spans have strictly positive angular-velocity numerators. Source start and end coordinates are exactly identical, with exact endpoint squared distance zero. Both facts are independently returned; the API does not infer a winding count. A positively traversed closed curve can wind several times, so one-turn coverage still requires a certified ray crossing count or equivalent global argument. The evidence also leaves pcurve inverse matching, source interval restrictions and topological ownership unresolved.

## Certified signed winding

`BoundPolynomialSplineWinding` adds a bounded global proof after requiring strict angular monotonicity and exact 3D source closure. It counts roots of `y-origin.Y=0` on the positive-X radial ray. Exact Bernstein sign variations prove absence of interior roots or exactly one interior root; rational midpoint subdivision separates ambiguous roots and proves the sign of `x-origin.X`. Strict angular velocity ensures every retained ray crossing is transverse. Shared knot and subdivision-boundary roots are deduplicated by exact rational parameter; the terminal source endpoint is excluded.

A crossing with equal bounds is an exact parameter. Unequal bounds denote an **open** isolating interval with exactly one interior root; endpoint roots are separate events. Results are ordered by parameter. Exhausted depth/cell/bit budgets return an error rather than a guessed count. Defaults are 64 subdivision levels and 65536 cells, with hard maxima of 128 and one million. The earlier source and coefficient budgets also apply.

The retained OCCT curve #25 has exactly one positive-ray crossing and therefore signed winding +1, certified over 28 processed source spans. Independent fixtures with two turns report +2, and reversed fixtures report −1 or −2. A curved fixture exercises genuine subdivision and rejects an insufficient depth budget. This closes the angular one-turn coverage question only; inverse pcurve matching, finite source interval ownership, support-intersection distance and BRep material validation remain separate obligations.

## Cylinder pcurve ancestry and angular range

`InspectCylinderPCurve` verifies a selected EDGE_CURVE → SURFACE_CURVE → two distinct PCURVEs graph. It requires exactly two face-owner uses with opposite normalized loop directions, one pcurve for each owning support, exactly one cylindrical owner, one containing closed shell and one manifold body owner. Detached extra oriented uses, duplicated owner support links and sharing the selected pcurves or surface curve with another source curve fail. This is selected-edge ancestry, not validation of the whole shell.

The cylinder pcurve must be a bounded simple polynomial 2D spline in a dimension-2 definitional context. Its exact source parameter interval must equal the 3D polynomial curve's domain. Exact derivative Bernstein coefficients certify strictly increasing or decreasing U. The reader retains source IDs, edge sense, domain, endpoint U values and per-span bounds. It does not assume equality of the interior 3D and 2D parameterizations merely because the domains match.

For the pinned off-axis fixture, edge #21 uses model #25 and cylinder pcurve #375 → parameter curve #382 on cylinder #376. The SURFACE_CURVE master is `PCURVE_S1`, sphere pcurve #196; this authority is retained explicitly rather than assigning it to the 3D curve or cylinder pcurve. All 28 U-derivative spans are strictly positive on `[0,1]`; U runs from 0 to the exact decimal `6.28318530718`. An exact rational Machin-formula enclosure of π proves `0 < (UEnd-UStart)-2π < 5e-13` radians. Thus the stored U interval is approximately one turn, not exactly one turn. The independent 3D curve winding proof remains exactly +1.

This evidence does not establish pointwise phase correspondence, pcurve-lift distance to the model curve, inverse interval matching, endpoint tolerance compatibility or material ownership. No source reconstruction or import has been enabled.

## Same-parameter cylinder lift agreement

`BoundCylinderPCurveLift` verifies the selected-edge ancestry above and bounds the difference between the 3D model spline and the cylinder lift of its associated pcurve **at the same source parameter**. The initial subset requires equal degrees and identical exact knot sequences, positive source cylinder radius, and an explicitly represented signed-coordinate orthonormal cylinder frame, and one explicit unscaled-radian body representation context. Angular conversion is not inferred. It retains the source master-representation metadata, including the sphere-pcurve authority in the retained fixture.

Each source span is converted exactly to Bernstein coefficients. For U, the algorithm chooses a dyadic expansion center; this choice does not alter U. Degree-64 rational Taylor sums enclose sine/cosine at the center with explicit Lagrange remainder. Degree-10 Taylor composition in `U-center` uses exact Bernstein products and elevation. Its omitted terms and center-evaluation uncertainty receive rational bounds. The U-control hull must be within one radian of the center, whose magnitude is at most 16. These fixed orders and the existing rational bit budgets bound the work. The V component is compared directly as a polynomial.

The output gives exact rational coordinate bounds in the cylinder frame and their summed squared-distance bound, with the caller's positive tolerance in original model units. `WithinTolerance=true` proves the entire model/pcurve lift difference is no larger than that tolerance. `false` means the bound did not certify it; it does not by itself prove actual disagreement.

For the retained OCCT curve, all 28 spans certify a Euclidean tolerance of `4e-8` model units; `1e-8` is not certified. A changed cylinder-pcurve height is detected by the bound. This proves approximate continuous **same-parameter cylinder** agreement only. The spherical pcurve, endpoint-to-topological-vertex tolerance, source trimming interval ownership, inverse matching and material validation remain unproved. It does not turn the approximated STEP boundary into exact geometry and does not enable import.

## Same-parameter spherical lift agreement

`BoundSpherePCurveLift` adds the spherical owner of the same two-owner sphere/cylinder graph. In the exact signed-coordinate sphere frame it bounds `R*cos(V)*cos(U)`, `R*cos(V)*sin(U)` and `R*sin(V)` against the same-parameter model curve. It requires explicit unscaled radians, equal degrees and exact knot partitions, and a latitude control hull strictly inside `(-π/2,π/2)`, verified using rational π bounds.

To bound cost, the spherical lift uses degree-six Taylor composition per angle (the cylinder's public behavior remains degree ten). Exact Bernstein multiplication combines the trigonometric polynomials. If the individual approximations have errors `eA,eB`, their product error is bounded by `eA+eB+eA*eB`, since the true sine/cosine factors have absolute value at most one. Products have degree at most 96 and a 32768-bit coefficient/result budget; individual trigonometric approximations retain the earlier budgets.

For the retained OCCT sphere pcurve #196 → #203, all 28 spans certify a Euclidean tolerance of `4e-7` model units. The resulting bound does not certify `1e-7`. Altering the latitude pcurve by 0.001 radians fails a `1e-6` tolerance check. The source master-pcurve metadata remains explicit.

Both sphere and cylinder lifts now have independent continuous same-parameter bounds. This does not establish topological endpoint compatibility, trimmed interval selection, source tolerance compatibility, shell embedding or material equivalence. Exact import remains disabled; these are opt-in approximate evidence reports.

## Selected closed-shell topology and combined evidence

`ValidateOffAxisBoreTopology` validates the retained source's narrow three-face incidence pattern: one sphere face, one inward cylinder face and one plane floor; one bound per face; four edges and three vertices; one spherical seam and one cylindrical seam; and two closed rims joining the distinct supports. It preserves face/bound order, source coedge IDs and bound-normalized traversal. This initial subset requires the fixture's face-sense roles and forward EDGE_CURVE senses.

Every loop must close by exact vertex identity. Every edge has exactly two opposite normalized uses, including the two uses of each seam. Vertex links must each be a single degree-two cycle; the pole's one-node/self-link is handled explicitly. The face adjacency graph is connected, Euler characteristic is 2, and detached or reused topological records fail. All four edge geometries retain their model and two pcurve IDs; seam branches must both name the same owning support, and cross-face rim pcurves must name the two distinct owners. Parameter contexts must be two-dimensional. Assigning a seam branch to an individual coedge and proving its geometric continuity remain unproved. This proves the selected combinatorial oriented-manifold graph, not an embedded spatial shell or correct orientation relative to its supports.

`InspectOffAxisBoreEvidence` combines that topology with the existing pcurve ancestry, increasing cylinder U, positive one-turn model winding and both same-parameter lift bounds. Its initial combined frame is positive model Z with positive X reference; required axis-origin coordinates must be exactly representable. It computes exact squared gaps from both source topological rim vertices to the corresponding model-spline endpoints. On the fixture these gaps are nonzero and equal, while the combined numerical evidence meets a `1e-6` model-unit tolerance.

`WithinRequestedTolerance` checks only curve-lift and endpoint error bounds. Entire polynomial source domains are evaluated; selection of trimmed intervals, complete face coverage, planar floor geometry, spatial self-intersection, source uncertainty policy and material classification are **not** certified. No shape is reconstructed or imported from these reports.

### Floor circle and cylinder seam evidence

`InspectOffAxisSimpleGeometry` first validates the selected bore incidence graph,
then checks exact source-decimal primitive geometry. Its supported frames use
signed coordinate axes. A floor circle must be coaxial with the cylinder and
parallel to the floor plane. The returned constant squared-radius residual and
signed plane distance bound the entire circle, without sampled acceptance.
A cylindrical seam LINE must be parallel to the cylinder axis; its constant
squared-radius residual and each source vertex's squared perpendicular distance
to the infinite line are computed exactly. These checks do not establish line
trim direction/extent or pcurve lift agreement.

For the retained OCCT fixture, both floor-circle support residuals are zero.
The seam's source Y coordinate is `-4.898587196589E-16`: its cylinder residual
is that exact decimal squared, strictly positive. The upper source vertex has
a different Y coordinate, so its line-distance square is also positive. Neither
is snapped to zero. A radius or parallel-plane displacement is returned as a
nonzero residual; unsupported tilted or noncoaxial geometry is rejected.

`SourceUncertainties` inventories source `UNCERTAINTY_MEASURE_WITH_UNIT`
length values with entity and unit IDs in file order. The fixture declares
`1.E-07` against unit #639 (millimetres). Values are retained in their declared
units, not converted or interpreted as acceptance tolerances; this inventory
does not assign an uncertainty to a particular edge or claim its context applies
to every body. No reconstruction, face coverage or material certification is
provided by this API.

### Spherical seam supporting circle and endpoint evidence

`InspectOffAxisSphericalSeam` checks the selected sphere seam using exact source
numbers. It requires a concentric circle and sphere, nonzero exactly orthogonal
circle normal/reference directions, and a signed-coordinate sphere frame. The
circle directions may be nonunit. The full-circle sphere residual is the exact
difference of squared radii. The meridian flag reports whether the circle plane
contains the source sphere axis. This is not a claim about the selected arc.

For both topological endpoints, the report includes the sphere equation
residual, squared distance to the circle plane, radial equation residual of the
orthogonal projection into that plane, and squared distance to the sphere axis.
Plane distance divides by the raw normal's exact squared norm, so no rounded
normalization is needed. These are point/support claims, not an endpoint angle,
nearest-circle distance, pole acceptance, or edge-interval proof.

The retained fixture's full circle is exactly on the sphere and its plane
contains the sphere axis. Its normal and reference squared norms are equal but
strictly greater than one. Both source endpoints have small positive sphere
residuals and plane distances; the nominal pole also has positive axis distance.
Those represented discrepancies remain nonzero. Nonconcentric circles and
nonorthogonal directions are rejected; radius mismatches are reported exactly.
No tolerance-based repair, face coverage or material reconstruction is implied.

### Linear seam and floor owner representations

`InspectOffAxisLinearPCurves` exposes both cylindrical seam LINE pcurves
(constant U, linear V), the floor's cylindrical LINE pcurve (linear U, constant
V), and its planar CIRCLE pcurve with source center/reference/radius. The
supported lines have signed coordinate-axis directions and positive magnitude.
Original pcurve IDs and source order are retained; no seam branch is assigned
to a coedge. The floor cylinder height residual compares constant V with the
model circle center's exact cylinder-axis coordinate; the planar circle radius
residual compares source radii, without asserting the remaining planar lift.

The report includes the floor vertex's model-circle frame coordinates and
squared displacement from the model circle's zero-angle point. In the fixture
this displacement is strictly positive, not snapped. The two seam constant
angles differ by `6.28318530718`; exact rational bounds show its positive defect
from 2π. The radian context is verified. This is a source angle difference,
not an inferred parameter interval or vertex angle.

STEP LINE and CIRCLE representations here author no finite trim parameters.
`AuthoredTrimIntervalsKnown` therefore remains false; topological endpoints
are not silently converted into curve parameters. Arbitrary source offsets
are preserved and discrepancies reported. Full floor pcurve lift agreement,
seam endpoint parameter matching, circle arc selection and face coverage remain
outside this inspection API; it performs no repair or shape import.

### Spherical seam meridian pcurve branches

`InspectSphericalSeamPCurves` preserves the two source association branches and
requires each to be an untrimmed constant-U LINE with signed coordinate-axis V
direction and positive VECTOR magnitude. It reports each source U/V origin,
direction, magnitude and pcurve/model IDs. A verified radian context permits
exact rational enclosures of `(U0-U1)-2π`; the difference is not normalized to a
preferred angular representative. Separate fields report V-origin difference,
equal V direction and equal magnitude; disagreement is retained, not repaired.

The retained fixture has branches #564/#571, each starting at V=-6.28318530718
and moving in positive V with magnitude one. Their U difference is
6.28318530718, whose positive discrepancy from 2π is rigorously bounded.
This does not prove periodic lift equivalence: no finite LINE interval is
authored, and the unusual V origins cannot be treated as trim endpoints.
Branch-to-coedge mapping, selected meridian arc, pole matching, spherical face
coverage and material interpretation remain unknown. Nonmeridian parameter
lines are rejected rather than approximately classified.

### Exact planar floor circle lift

`InspectOffAxisFloorCircleLift` compares the authoritative 3D CIRCLE with its
planar owner's CIRCLE pcurve after exact lifting through the plane placement.
For signed-coordinate 3D frames and an exactly unit represented 2D reference,
it compares all coefficients of `C + A cos(t) + B sin(t)`. The returned center,
cosine and sine differences retain exact source decimals; the identity flag is
true only when every coefficient agrees. Thus a phase-shifted circle is not
mistaken for the same parameterization merely because its point set is equal.
Nonunit planar reference directions are explicitly unsupported rather than
rounded during normalization.

The retained floor circle passes whole-parameter identity. Mutated pcurve
center, radius and phase produce explicit nonzero coefficients. This closes
only the planar owner's full-circle lift gap. The source floor vertex still
has its previously reported zero-angle discrepancy; neither a finite edge
interval nor branch/coedge parameter mapping is inferred. Cylindrical floor
lift and seam interval correspondence, spherical coverage, global embedding
and material reconstruction remain independent proof obligations.

### Cylindrical floor circle lift

`InspectOffAxisFloorCylinderLift` retains the source LINE relation
`U(t)=AngleOrigin+AngleSlope*t, V(t)=Height`. For signed-coordinate cylinder and
model-circle frames it compares the lifted circle center, radius and plane
exactly. `SupportCircleEqual` concerns the entire untrimmed point set; any
nonzero angular slope covers that circle, regardless of direction or speed.
`SameParameterization` is a stronger sufficient proof: zero authored phase,
slope ±1, matching zero-angle axes and correctly signed sine axes. False does
not choose a replacement parameter mapping. The exact affine U relation is
always reported for supported input.

The fixture passes both flags. Reversed U, doubled speed and a nonzero phase
preserve support equality while failing same-parameter equality. A changed
constant V yields an explicit center discrepancy and fails support equality.
No arbitrary origin is reused to force a match. As with the planar owner, no
finite trim interval, coedge branch parameter assignment, oriented disk coverage
or material interpretation follows from this full-support identity.

### Evidence review: angular units

The standalone cylinder-pcurve inspection now requires the uniquely owning
body representation to declare unscaled radians before reporting its 2π
turn-defect enclosure. Previously only downstream lift checks enforced that
context; standalone inspection could label raw angles as radians despite a
prefixed or missing angular unit. Mutation regressions reject those cases.
This does not convert angles or extend the supported unit subset.

The endpoint-gap bundle still compares clamped model-domain endpoints with
source topological vertices; it does not establish an authored trim interval.
Source uncertainty inventory remains unassigned metadata, not an acceptance
threshold. Primitive support proofs use exact source decimals and bounded
arithmetic; they do not validate full EXPRESS semantics or material coverage.

### Exact upper-intersection correspondence

`BoundOffAxisRimCorrespondence` adds a bounded full-domain curve certificate,
not a solid importer. It validates the existing three-face ownership graph and
requires exact canonical +XYZ sphere/cylinder frames, a positive X center
offset `d`, equal center Y, and `R>d+r`. Support values must be losslessly
representable by the existing residual APIs; there is no support snapping.
The model polynomial spline must close exactly and have one strictly positive
azimuth turn. Its full control hull must lie above the sphere center's Z.

At each model point, radial XY projection onto the cylinder preserves azimuth;
choosing the sphere's positive Z root defines a unique continuous target.
One-turn strict azimuth makes this correspondence onto the entire upper
intersection. For sphere/cylinder equation bounds `Es,Ec`, let `delta=Ec/r`,
`H` bound the source horizontal norm, `z0>0` bound source height, and
`h0=R-d-r>0`. The vertical error is at most
`(Es+2*H*delta+delta²)/(z0+h0)`. Adding `delta` yields a rational Euclidean
Hausdorff upper bound in both directions. All source-polynomial bounds are
continuous Bernstein proofs, not samples. Tolerance is in source length units
and is supplied explicitly; source uncertainty metadata is not adopted.

The result concerns the entire clamped spline domain and the entire exact upper
intersection. It does not establish authored edge trim selection, source
vertex coincidence, seam/pole correspondence, pcurve lift agreement, other
boundary curves, face coverage, embedding of the complete boundary or material
equivalence. Existing strict shape importers continue rejecting this approximate
STEP B-rep. Small tolerance failure returns a completed certificate with
`WithinTolerance=false`; unsupported geometry or exhausted proof budgets errors.

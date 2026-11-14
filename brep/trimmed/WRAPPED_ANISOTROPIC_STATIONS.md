# Exact independently scaled weighted stations

`NewWrappedAnisotropicStationLoft` constructs a native closed solid from 1–16
original `WrappedSurfacePatch` bands. `ValidateWrappedAnisotropicStationLoft`
authenticates the actual native walls, caps and edge uses after reload/reindexing.
`SectionWrappedAnisotropicStationRational` returns an exact homogeneous section,
original incident face IDs and original station rim IDs without binary64 rounding
of the query height or curve controls.

This differs from `parametric.LoftEllipticStations`, which uses analytic angular
ellipse supports with cubic axial interpolation, and from homothetic weighted
station lofts. The new source is degree2×1, nine U poles in four weighted
quadrants and two V rows. Its X and Y control scales vary independently and
linearly with height. An arbitrary weighted profile is not called an ellipse.
The default circular/homothetic mesh/query/STEP paths remain unchanged.

The geometry certificate checks every original homogeneous control against
`(cx+sx(v)*f(u), cy+sy(v)*g(u), z(v))`: weights are positive and identical between
V rows; both endpoint scales are strictly positive; height strictly increases;
original U knots have four ordered clamped quadratic intervals. The normalized
weighted square profile has strictly increasing polar angle on each quadrant.
A positive diagonal map preserves that property. Since
`dot(Su×Sv,(x-cx,y-cy,0)) = sx*sy*z_v*(f*g'-g*f') > 0`, each open piece is regular
and outward. C0 knot joins are allowed without claiming C1 continuity. Height
separates all sections; ordered open band slabs therefore cannot intersect.

The constructor reuses exact native cap trim validation and complete edge/model
agreement. Neighboring corners and scales alone are insufficient: the shared
full rational rim must agree, so changing interior weights at a station is
rejected. Only two terminal disks become material caps. Every intermediate
station is a shared four-edge rim with both original wall owners. Native cap
curve export must be exactly representable; unsupported nonbinary native pole
export fails rather than rounding. Rational interior sections have no such
export requirement and retain the exact controls.

Limits are explicit: at most16 bands, fixed18 controls per source, supplied
surface coefficient/pole/work limits per geometric proof and section extraction,
and the existing bounded exact native edge agreement. Validation uses a fixed
number of passes over each band and reuses the exact original topology matcher;
its work limit is per proof pass, not a single aggregate scalar counter.
Cancellation or any failed pass returns no partial solid or section.

Qualification uses center(1/4,−1/2), weight3/4, U cuts−2,−1,1,4,8, V domain2..5,
heights0,1,3 and independent scales(3,1),(1,2),(2,1). The section at height1/3 has
scales(7/3,4/3), original V=3 and first-quadrant midpoint
(23/12,19/42,1/3). Independent normalized-profile Green integration gives
volume `profileArea*43/6` and axial centroid `60/43`, matching native integration.
Tests include reload/face reordering, original rim parameter ranges, a changed
interior profile with unchanged corners, source drift, collapsed scale, unequal
V weights, missing height interval, altered native wall, quotas and cancellation.
No certified display, general loft Boolean or arbitrary frame acceptance is
claimed by this milestone.

## Certified volume and centroid

`BoundWrappedAnisotropicStationMass` reuses the existing exact normalized-profile
area/first-moment enclosure after full anisotropic native authentication. One
integration work ledger covers all bands, adaptive profile subdivisions and
returned interval arithmetic. The existing `WrappedRationalStationMassOptions`
controls volume width and resource limits; source authentication has its own
per-pass limits. No separate centroid-width tolerance is promised.

For normalized profile area A and first moments Mx,My, the exact transfers are

- V = A ∫sx sy dz;
- firstX = cx V + Mx ∫sx² sy dz;
- firstY = cy V + My ∫sx sy² dz;
- firstZ = A ∫z sx sy dz.

Products of the original affine station factors are integrated as exact rational
polynomials. Signed interval arithmetic carries profile uncertainty; the shared
area factor is cancelled analytically in the exact Z centroid. Exact central
symmetry is used only after the original weight ratios prove it. The shared full
rim certificate ensures every band represents the same normalized profile,
even when the two physical station scales change independently.

For the qualification above, factors are V/A=43/6, firstX's profile factor=37/3,
firstY's profile factor=32/3 and firstZ/A=10. An asymmetric first-quadrant weight
1/2 (other weights3/4) gives genuinely distinct horizontal centroid offsets.
An independently translated case at (−2,3,−4) checks negative X/Z and positive Y
first moments, with exact Cz=−112/43. Independent Green quadrature checks the
nontrivial profile values, tighter refinement gives nested volume intervals,
and segment/work/bit/cancellation failures are atomic. No numerical quadrature
is used by the certified API itself.

## Exact point and original incidence

`LocateWrappedAnisotropicStationRational` authenticates the original solid and
classifies exact rational model points. It uses independent positive scale
inversion at the query height and the guarded original weighted-quadrant conic;
it does not use an ellipse or a rounded membership test. `Faces` reports all
original material boundary owners. `RimEdges` reports only actual station edges,
while `Edges` also includes the native periodic wall seam at positive X.
Cardinal station points own both adjacent rim edges and their shared original
vertex. Neither this incidence nor a representative point is claimed to prove
a unique native curve parameter.

The geometry primitive `LocateWrappedAnisotropicRational` reports local capped
band support masks. The native query explicitly removes artificial internal
band-cap attainment: an internal station disk is interior, whereas its rim owns
both original walls. Outside points on a cap plane are still outside. The query
arithmetic and one conic reproof share the exact remaining `MaxPointOperations`
allowance; native source authentication retains its separate source quotas.
Failure returns no partial evidence and must not be interpreted as Outside.

Tests qualify nonbinary rational sections, all four weighted quadrants, opposite
radial perturbations smaller than binary64 resolution, original periodic seam
and cardinal vertex incidence, rotated coedge loops, reversed face ordering and
native reload. A weighted profile interior point outside the corresponding
ellipse explicitly rejects circular substitution. The exact reported allowance
succeeds while one fewer query operation fails atomically.

## Exact horizontal isolated line clipping

`Solid.ClipWrappedAnisotropicStationHorizontalLineRational` reports original
wall-face owners, isolated algebraic parameter brackets and open material
intervals for finite rational horizontal lines strictly between station planes.
The independent exact X/Y inverse maps to the original weighted conic; the
caller’s unnormalized parameter is unchanged. This is not an ellipse substitute.
Terminal-cap planes, internal-station planes, nonhorizontal lines and coincident
boundary intervals fail explicitly. An empty result outside the height range is
proved empty. Tangencies do not create material intervals, and ambiguous touching
root enclosures fail rather than being merged numerically.

The original source is authenticated before selection. Geometric source proof,
normalization and all four conic stages share one operation allowance; root
subdivision is bounded per quadrant. `WrappedRationalStationLineOptions` bounds
candidate events and divides the point-work allowance over exact gap queries;
each gap reproof has its separately documented source allowance. Outputs are
owned and failures return no partial ledger. This adds no general oblique or
coincident-line claim.

## Exact isolated lines confined to one band

`ClipWrappedAnisotropicStationBandLineRational` additionally accepts nonhorizontal
finite lines when both exact endpoint heights are strictly inside the same
original station interval. Affine height keeps the complete finite line in that
band. With positive scales `sx(t),sy(t)`, it forms `U=(x-cx)sy`, `V=(y-cy)sx`
and `R=sx sy`. Each original weighted-quadrant equation is the quartic
`w0*w2*(U+V-R)^2 - 4*w1²*(R-U)*(R-V)`. Exact algebraic signs of `R-U`, `R-V`
and `U+V-R` retain only the original quadrant arc; roots of the other conic branch
are not contacts. The parameter is never normalized.

One geometric operation ledger includes source proof, polynomial construction,
root isolation and exact sign filtering. Work is structurally bounded by four
quartics, at most sixteen roots and three sign proofs per root. Subdivision caps
apply independently to each quartic and each sign proof. Native event and gap
quotas remain as above. Identically zero candidate polynomials (including real
wall generators), ambiguous nonexact root-bracket overlap, station crossings and
boundary-plane endpoints fail atomically. This does not claim a multi-band or
coincident-line solver. The earlier horizontal API is unchanged.

## Complete nonhorizontal multi-band lines

`ClipWrappedAnisotropicStationLineRational` clips a finite rational line to every
original band at exact station parameters and merges only proved equal rational
events. It retains both incident wall faces at a crease, actual rim edges and
cardinal vertices at station contacts, and both exposed terminal-cap events.
Internal station disks remain material interior. Nonexact overlapping brackets
are not guessed equal. The horizontal and strict single-band APIs remain unchanged;
this collection API rejects horizontal lines and coincident wall polynomials.

`WrappedAnisotropicMultiBandLineOptions.MaxOperations` is a cumulative query
ledger for parameter clipping, all closed-band geometric source/root/sign proofs,
station classifications and gap classifications. Prepared point classification
reuses the authenticated original source. Initial native topology authentication
retains the existing separate per-pass `Geometry.Surface` limits and sixteen-band
source cap. Root subdivisions remain bounded per quartic and per sign proof.
The exact charged operation count is returned only on success. Event limits are
charged before merging; all failures are atomic. Independent tests cover both
terminal caps, a waist crease touch with both owners, a cap/rim tie, opposite
parameter direction, two wall contacts in different bands, and exact N/N−1 work.
Cardinal contacts at non-dyadic rational parameters are authenticated by the
original affine equations `x-cx=±sx` or `y-cy=±sy` and exact quartic evaluation
inside the isolating interval. This proves shared quadrant identity (including
parameter8/5 in the regression); interval overlap alone is still insufficient.

## Coincident original terminal-cap lines

`ClipWrappedAnisotropicStationCapLineRational` explicitly accepts horizontal
lines on either exposed original terminal cap. It returns closed `Boundary`
spans, isolated rim/tangent hits or a singleton clip; it has no volume-interior
field. Exact root isolation partitions the line, and prepared exact cap
membership proves each root-free span. Span interior owners are the original cap;
endpoint owners additionally retain the adjacent original wall and rim incidence.
For nonexact roots, affine coordinate signs over the entire enclosure prove the
original rim quadrant. Unresolved quadrant identity fails rather than choosing
an approximate azimuth. Exact cardinal contacts retain both rim edges and their
original vertex. Lower-degree sector polynomials are normalized before exact GCD
sign proofs, including horizontal lines whose quadratic coefficients vanish.

The same cumulative query ledger and separately bounded initial source proof
apply. MaxEvents bounds hits plus spans, and all errors return an empty result.
Internal station planes and nonhorizontal lines remain outside this cap API;
existing isolated-line contracts are unchanged. Tests use independent weighted
midpoint roots ±5/7, scaled cardinal roots, upper/lower caps, reverse direction,
finite clipped and singleton intervals, isolated tangency and exact work limits.

# Verification record

## PlainCAD repeated and deepened crossed Holes — 2026-11-14

A crossed Hole batch can contain coaxial cutters proved covered by another
member. The exact cardinal line, radius and finite floor are checked before
discarding a covered tool. Deepening retains the extended floor, and
continuing through the other cap produces the same canonical STEP as an
authored through Hole. A wholly redundant batch reports zero material
removal. Other bore families keep their original tool sets.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-contained-crossed-browser-2026-11-14.json)
pass. Focused native tests verify grouped duplicates, shorter and narrower
cutters, incremental overlap, deepening and final opening. Race and Go vet pass.

## PlainCAD STEP for two intersecting blind Holes — 2026-11-14

Both equal-radius perpendicular cutters can stop beyond their complete
intersection. Each retains its own circular floor face and opening cap.
The analytic STEP reader recovers both finite spans before verifying the
generated closed boundary. Grouped and successive cuts preserve the same
measures and both floor references, including decimal depths.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-double-crossed-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-double-crossed-step-native-2026-11-14.json)
pass on the same WASM. The authored project reimports and reopens the
44,829-byte STEP and confirms both floor positions in watertight STL.
Twenty-four signed native entry combinations and twelve independent OCCT
STEP imports pass, including inch units. The authored direct Boolean and
STEP readback agree on volume and surface area within 0.000001 in model
units. Focused race tests and Go vet pass.

## PlainCAD STEP for a blind Hole crossing a through Hole — 2026-11-14

Equal-radius perpendicular Holes on rectangular stock preserve the
complete crossing and the circular blind floor beyond it. The retained
face index supports subsequent planar references. Shared angular samples
join the conic quarter-wall strips into a closed display and STL mesh.
The exact elliptical STEP trim graph passes canonical readback. The
existing both-through STEP graph is byte-identical.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-blind-crossed-step-browser-2026-11-14.json)
and a [supplemental authored case](evidence/plaincad-blind-crossed-step-native-2026-11-14.json)
pass on the same WASM. The authored project imports the 44,132-byte STEP
and finds the retained floor at Z=2 mm in its 400-triangle STL. Twelve
signed native placements and entry sides pass. Six independent OCCT
STEP imports pass, including inch units. OCCT's direct Boolean and STEP
readback agree on the authored case within 0.000001 in volume and area.
The focused race checks, related compact tessellation tests and Go vet pass.
Next geometry is two blind Holes with both floors beyond the crossing.

## PlainCAD STEP for partially overlapping perpendicular Holes — 2026-11-14

The single radical trim loop exchanges as paired cubic Hermite splines
with the two native cylinder supports. Double interior knots remove
redundant join controls after an explicit convex-hull displacement
check. Fourth-derivative bounds and a separate numerical allowance
govern the fit. The file declares 0.0000001 mm distance uncertainty,
and the canonical reader checks the complete graph before rebuilding
the analytic trim loop.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-partial-skew-step-browser-2026-11-14.json)
and a [supplemental authored case](evidence/plaincad-partial-skew-step-native-2026-11-14.json)
use the same WASM. The authored model exports in 367,338 bytes, reimports
within the 512 KiB app limit, reopens and exports watertight STL. Four
independent OCCT STEP imports cover both radius orders, equal radii,
signed placements and a decimal offset. Focused race and Go vet pass.

## PlainCAD partial transverse overlap on perpendicular axes — 2026-11-14

Two offset full-through bores can overlap across only part of their
transverse widths. Both cylinder charts retain the same closed intersection
loop. The factored analytic parameter removes singular derivatives at the
two turning points. The native topology uses six box faces and four
cylindrical faces, and the bounded adaptive mesh keeps the shared trims
conforming. This applies to equal and unequal radii.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-partial-skew-native-browser-2026-11-14.json)
and a [supplemental authored case](evidence/plaincad-partial-skew-native-2026-11-14.json)
pass on the same WASM. The authored model uses a 5 mm offset with radii
5 mm and 4 mm. Reopen retains the two Holes, and the STL has both passages.
Native tests measure two radius orders, signed and decimal offsets, and
float32 embedding. Independent OCCT Boolean fixtures and focused race
and vet checks pass. Very thin near-contact cases can exceed the bounded
integration or tessellation budget. STEP exchange for this trim family
is the next gate.

## Selectable PlainSolid app build — 2026-11-14

The separate PlainCAD app worktree has a selectable Go WASM provider and
the matching worker imports. A release build verifies the pinned source
revision and hashes, emits a fingerprinted WASM asset and checks that no
OCCT assets load while PlainSolid is selected. Its existing browser
assertions still require the selected native provider. The default unit
fixtures preserve their OCCT and fallback conditions.

The [app qualification evidence](evidence/plaincad-provider-integration-2026-11-14.json)
pins PlainCAD integration commit `9779044` and PlainSolid `f571a964`. It
records 345 development cases, including the new asynchronous-open race
regression, 66 production cases, and 15 cross-browser cases across Chromium,
Firefox and WebKit. All three browser runs complete with zero skips or
retries. The ordinary unit/component suite passes all 1,691 cases using
the app's earlier documented local worker and retry settings. Four
provider-specific contracts exercise actual PlainSolid WASM. Type checks
and the production bundle budget also pass. Later geometry commits have
separately pinned qualification reports.

## PlainCAD STEP for a single oblique through Hole — 2026-11-14

The retained cylinder and rectangular stock exchange as analytic surfaces.
Eight conic edges preserve the two planar opening ellipses. Canonical
readback recovers the axis ratios and entry face from the cylinder and
opening graph, then verifies the regenerated graph. Tests cover adjacent,
opposite, three-component and reversed/translated directions, non-millimeter
units, altered conics and grazing input.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-oblique-step-browser-2026-11-14.json)
and a [supplemental authored case](evidence/plaincad-oblique-step-native-2026-11-14.json)
pass on the same WASM. The authored case exports only the drilled body,
verifies the native worker's readback, reimports the STEP, reopens the
project and exports watertight STL. Independent OCCT Boolean and STEP
reader comparisons match volume and area for opposite and adjacent
apertures within 0.000000003 in model units. Focused race and Go vet pass.

## PlainCAD single oblique through Hole — 2026-11-14

A rectangular body accepts one oblique cylindrical through Hole when
the finite tool covers both complete opening ellipses on opposite or
adjacent faces. Exact rational halfspace checks qualify the finite tool
caps. The native boundary builder checks each aperture against every
other stock face and preserves the original planar IDs. The same proof
drives scoped overlap, additive history and Cut. Partial openings reject
without changing handles or material.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-oblique-hole-native-browser-2026-11-14.json)
and a [supplemental authored case](evidence/plaincad-oblique-hole-native-2026-11-14.json)
pass on the same WASM. The supplemental case uses a separate sloped
guide face and a signed offset to place the Hole start. The scoped Hole
rebuilds, preserves both bodies on project reopen, and exports watertight
STL. Independent OCCT Boolean checks cover the opposite and adjacent
apertures. Focused race and Go vet checks pass. This milestone leaves
oblique-bore STEP exchange for the next gate.

## PlainCAD skew perpendicular through Holes — 2026-11-14

Two perpendicular through bores may have different transverse centers.
This family requires the larger bore to enclose the full transverse width
of the smaller bore. Both intersection loops retain their analytic
radical trims. STEP applies the same bounded cubic-spline exchange
representation and recovers the distinct axes from the actual cylinder
supports. The writer normalizes signed placements and decimal offsets
before constructing the canonical STEP graph.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-skew-step-browser-2026-11-14.json)
and the [authored offset-axis probe](evidence/plaincad-skew-step-native-2026-11-14.json)
use the same WASM. The probe verifies a 1 mm axis offset through STEP
export, reimport, project reopen and watertight STL. The first export
exceeded the app's 512 KiB import limit. Bounded rounding of interior
spline controls reduces the same export to 503,220 bytes and leaves
shared endpoint coordinates intact. Four independent OCCT STEP imports cover signed placements, both radius orders and decimal
offsets. Native tests check 36 arrangements against independent numerical
cross-section volume, area and centroid, and check the float32 mesh
embedding separately. The final race and vet checks are recorded with
the browser evidence.

## PlainCAD STEP for unequal-radius crossing Holes — 2026-11-14

STEP preserves the analytic cylindrical and planar supports. Each
nonrational model edge and both owner trims use cubic Hermite splines
with shared parameters and knots. The interpolation chooses its span
count using a fourth-derivative bound and a separate numerical reserve.
The file declares 0.0000001 mm distance uncertainty. The importer checks
the complete emitted graph before regenerating the native analytic
intersection. Near-equal radii and large coordinates can exceed this
exchange budget and are rejected.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-unequal-step-browser-2026-11-14.json)
pass on `50432ed`. The [authored browser probe](evidence/plaincad-unequal-step-native-2026-11-14.json)
uses the same WASM to export, reimport, reopen and tessellate the verified
model. Independent OCCT imports match the native volume, area and centroid
in 12 signed and swapped configurations and on the browser's downloaded
STEP. Additional tests reject altered curve controls or uncertainty and
measure agreement between both parameter curves and the model spline.
Focused race and Go vet checks pass.

## PlainCAD unequal-radius intersecting perpendicular through Holes — 2026-11-14

Two unequal-radius cylinders with intersecting perpendicular axes now retain
the analytic intersection on both cylindrical walls. The two shared rings
have radical parameter curves. Their endpoints and both face uses agree on
the retained topology. Bounded surface integration computes material
measures independently of tessellation. Conforming adaptive refinement
preserves the shared boundary at the app's display and STL tolerances.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-unequal-crossed-browser-2026-11-14.json)
pass on `50432ed`. A [supplemental Chromium probe on the same WASM](evidence/plaincad-unequal-crossed-native-2026-11-14.json)
builds and reopens two unequal authored Holes, matches the pinned
OCCT Boolean's volume and area, and verifies both open passages in
watertight STL. Native tests compare volume, area and centroid with
independent cross-section quadrature across 18 signed, swapped and offset
configurations. Focused race, float32 embedding, existing helical meshing
and Go vet checks pass. STEP reports this trim family as unsupported pending
a qualified exchange representation.

## PlainCAD disjoint nonparallel through Holes — 2026-11-14

Canonical STEP preserves rectangular stock with up to 32 disjoint interior
through Holes on two or three cardinal axes. The native constructor requires
strict separation of padded tool bounds. Export sorts sites independently of
cut order, and import proves the complete graph before retaining the bore
recipes. Coaxial tools covered by a retained through Hole measure zero
remaining material on the same axis.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-disjoint-bore-browser-2026-11-14.json)
pass on `50432ed`. A [supplemental Chromium probe on the same WASM](evidence/plaincad-disjoint-bore-native-2026-11-14.json)
checks three authored through Holes of distinct diameters, STEP export and
reimport, project reopen, exact measures, open passages and watertight STL.
Thirteen independent OCCT comparisons include six signed placements with
two and three axes and a 32-site model. Native bridge tests cover grouped
and successive cuts, non-millimeter scaling, and material queries after
import. Focused race tests and Go vet pass.

## PlainCAD intersecting perpendicular through Holes — 2026-11-14

Two equal-radius through Holes on intersecting perpendicular axes in
rectangular stock retain analytic cylindrical walls and elliptical
intersection edges. Grouped and successive cuts share the same result.
The finite crossing must lie more than one radius from each stock face.

[Sixteen original PlainCAD Hole and STEP tests](evidence/plaincad-crossed-bore-browser-2026-11-14.json)
and [a supplemental Chromium probe on the same WASM](evidence/plaincad-crossed-bore-native-2026-11-14.json)
pass on `50432ed`. The supplemental case verifies both open passages,
exact measures, STEP reimport, project reopen and watertight STL.
Independent OCCT comparisons pass for 12 placed STEP models. Native
volume, surface area and centroid also match closed forms, and the STL
vertices pass watertightness and self-intersection checks after rounding
to float32.

## PlainCAD opposite Hole floors meeting at one depth — 2026-11-14

The section arrangement retains separate, oppositely oriented floor
regions at the shared level. The overlapping footprint leaves an open
passage between the two blind holes. It contributes no floor sheet.

[Sixteen original PlainCAD Hole and STEP cases](evidence/plaincad-meeting-floor-browser-2026-11-14.json)
and [a supplemental Chromium probe on the same WASM](evidence/plaincad-meeting-floor-native-2026-11-14.json)
pass on `50432ed`. The supplemental case verifies opposing 3 mm and 5 mm
cuts through STEP reimport, project reopen and watertight STL. Forty-eight
new independent OCCT checks cover same-side, adjacent-side, opposite-side
and corner openings across six signed placements. The full focused STEP
corpus passes all 402 comparisons with race detection, and Go vet passes.

## PlainCAD STEP for fully concealed Hole cutters — 2026-11-14

The rectangular-stock boundary builder checks which cutter disks cover
each section cell over each axial interval. A cutter whose entire removal
is covered by other cuts disappears from the boundary recipe. Rebuilding
the section without it removes the redundant wall and floor partitions.
STEP infers a through cut from the absence of its own blind floor, even
when later cuts hide the wall at one stock cap.

[Sixteen original PlainCAD Hole and STEP cases](evidence/plaincad-concealed-cutter-browser-2026-11-14.json)
pass on `50432ed`. A [supplemental probe on the same WASM](evidence/plaincad-concealed-cutter-native-2026-11-14.json)
verifies byte-identical STEP for the concealed and visible-only models,
reimported dimensions and measures, watertight STL, and preservation of the
authored Hole features after project reopen. The native bridge test drills
through the same site after importing the STEP and exports the new result.
Independent OCCT checks cover union concealment across depth layers,
concealment at a stock vertex, disjoint visible cuts, and a through cutter
with a concealed entry. Focused race checks and Go vet pass.

## PlainCAD Hole cutters through exact stock vertices — 2026-11-14

The section arrangement uses a single vertex when a circular cutter passes
exactly through a stock corner. The exact squared distance establishes the
contact before either side attaches to the shared vertex. The arrangement
also accepts two intersecting cutters that share the corner and endpoint
tangency when the cutter opens across the adjacent side. Point-only
external contact removes zero volume.

[Sixteen original PlainCAD Hole and STEP cases](evidence/plaincad-exact-vertex-browser-2026-11-14.json)
and [three Chromium probes on the same WASM](evidence/plaincad-exact-vertex-native-2026-11-14.json)
pass on `50432ed`. The probes cover blind, through-all and successive
overlapping Holes, with exact measurements, STEP reimport, project reopen,
and watertight STL. They check the affected stock vertex at the remaining
cap and floor heights. Independent OCCT checks cover all six signed axes,
including shared vertices and endpoint tangencies. Focused native race
checks and Go vet pass. A fully concealed cutter can leave face
partitions that canonical STEP cannot reconstruct from the visible
cylindrical supports. That export remains outside the qualified path.

## PlainCAD intersecting Hole cutters across stock corners — 2026-11-14

The rectangular-stock arrangement clips circular cutters against both
finite sides adjacent to a corner. Centered and offset corner footprints
can intersect other corner or side footprints. Blind floors and through
openings retain cylindrical walls. The qualified result keeps one solid
and retains all six stock extents. Exact contact at a stock vertex is
outside this boundary proof.

[Sixteen original PlainCAD Hole and STEP cases](evidence/plaincad-corner-opening-browser-2026-11-14.json)
and [three Chromium probes on the same WASM](evidence/plaincad-corner-opening-native-2026-11-14.json)
pass on `50432ed`. The probes check grouped and successive centered
corner cuts, plus successive offset corner cuts, with exact volume and
area, STEP reimport, project reopen and watertight STL. Native tests also
cover opposed cuts, further drilling after STEP import, and retention of
a connected blind corner island. Independent OCCT checks include signed
axes, blind and through openings, and stock with all four original
corners removed. Focused race checks and Go vet pass.

## PlainCAD intersecting groups of side-opening Hole cutters — 2026-11-14

The rectangular-stock arrangement accepts up to 16 circular cutters with
parallel axes and intersecting footprints. It retains analytic walls,
blind floors, through openings, and connected material islands. STEP
regenerates the complete boundary graph and recovers partially buried
walls from the orientation of their neighboring floors. A strictly
contained cutter already covered by one wider cut adds no boundary.

[Thirteen original PlainCAD Hole and STEP cases](evidence/plaincad-multi-side-opening-browser-2026-11-14.json)
and [two same-WASM Chromium probes](evidence/plaincad-multi-side-opening-native-2026-11-14.json)
pass on `50432ed`. The supplemental cases exercise a grouped three-center
Hole with shared overlap and four successive Holes at unequal depths,
including through-all. Both retain exact measures, STEP and watertight
STL through project reopen. Native checks include cutting again after
STEP import, input-order and redundancy checks, and atomic rejection.
Independent OCCT checks cover signed placements, a 16-site chain,
buried cylindrical walls, and an inner wire in a blind floor. Focused
race checks and Go vet pass.

## PlainCAD intersecting Hole cutters on different stock sides — 2026-11-14

The rectangular stock arrangement now supports two intersecting circular
Hole footprints opening on adjacent or opposite lateral sides. A cut may
split an entry-cap face into separate patches while the stock remains one
solid. Canonical STEP derives the full stock height from actual boundary
vertices and verifies the regenerated graph, including those cap patches.
Cuts that split the stock into separate solids are rejected atomically.

[Thirteen original PlainCAD Hole and STEP cases](evidence/plaincad-lateral-side-opening-pair-browser-2026-11-14.json)
and [two same-WASM Chromium probes](evidence/plaincad-lateral-side-opening-pair-native-2026-11-14.json)
pass on `50432ed`. The authored probes exercise adjacent sides with one
entry cap and opposite sides with opposed entry caps. Both retain the
expected measures, blind floor levels and bounds through STEP readback
and project reopen, and both export watertight STL. Native grouped and
successive cuts pass in both orders. Independent OCCT validates 90 STEP
round-trips by topology, volume, area and centroid, including 48 new
different-side cases. Focused race checks and Go vet pass.

## PlainCAD intersecting side openings from opposite caps — 2026-11-14

The same rectangular stock side can retain two intersecting circular Hole
footprints entering from opposite caps. The analytic result preserves
overlapping depth intervals or a material web between separated blind spans.
Grouped and successive native cuts pass in both orders. Exact contact between
opposed floors remains outside this milestone.

[Nine original PlainCAD Hole and STEP cases](evidence/plaincad-opposed-side-opening-pair-browser-2026-11-14.json)
pass on `50432ed`, along with the real WASM probe and both TypeScript checks.
[Two Chromium probes on the same WASM](evidence/plaincad-opposed-side-opening-pair-native-2026-11-14.json)
verify overlapping and separated spans, save/reopen, STEP reimport and watertight
STL. The focused native race checks and Go vet pass. Independent OCCT confirms
STEP volume, surface area and centroid for 42 signed-axis/depth cases, including
12 opposed-entry cases.

## PlainCAD intersecting same-side Hole cutters — 2026-11-14

Two transverse circular cutters that open on the same rectangular stock
side and enter the same cap now form a joined analytic notch. The
section arrangement retains exact arcs and cylindrical walls at each
depth, including unequal blind depths, through cuts and mixed depths.
The overlap query supports successive authored Holes as well as a
single scoped batch. STEP recovery regenerates and compares the complete
entity and edge-use graph.

All [13 original PlainCAD Hole and STEP cases](evidence/plaincad-side-opening-pair-browser-2026-11-14.json)
pass on `50432ed` alongside both TypeScript checks and the real WASM
probe. [Two same-WASM Chromium probes](evidence/plaincad-side-opening-pair-native-2026-11-14.json)
verify grouped and successive equal-depth cuts, separate 3 mm and 6 mm
floors, save/reopen, STEP readback and watertight STL. Independent OCCT
matches volume, area and centroid across 30 signed-axis/depth cases.
Focused race checks and Go vet pass.

## PlainCAD reflected side- and corner-notched stock — 2026-11-13

Signed-coordinate reflections now retain the side or corner notch recipe
in a right-handed frame and mirror the contained bore centers. Native
checks cover blind and through cuts, exact measures and bounds, further
drilling and STEP round-trip after reflection. The
[original mirror sketch and STEP cases](evidence/plaincad-reflected-notch-2026-11-13.json)
pass, along with focused race checks and Go vet.

## PlainCAD cross-browser and native short-suite coverage — 2026-11-13

The [latest cross-browser run](evidence/plaincad-cross-browser-cylinder-stock-2026-11-13.json)
passes all 15 originals on clean `19b63f4` across Chromium, Firefox and
WebKit. Both TypeScript checks, the bundle budget and separate real-WASM
probe pass.

The [native short-suite run and continuation](evidence/native-short-partition-2026-11-13.json)
record 38 passing packages and four packages without tests. The trimmed
package reaches Go's 30-minute package timeout after 353 passing and
three skipped top-level tests. A second short-mode selection completes its
remaining 44 tests and one explicit skip. The combined native evidence
therefore covers all 401 trimmed top-level tests as 397 passing and four
skipped across the two runs. The original all-package invocation remains
recorded as a timeout. The two earlier timeout cases also pass directly.

## PlainCAD equivalent STEP cylinder orientation encodings — 2026-11-13

Main's paired circle-frame and edge-sense reversals, plus axial and
face-bound reversals, now pass the strict half- and quarter-cylinder
readers in the integration worktree. The imported native tests include
positive and malformed encodings, and independent OCCT reads verify the
qualified fixtures. Both [original PlainCAD STEP cases](evidence/plaincad-cylinder-cell-orientation-2026-11-13.json),
focused race checks and Go vet pass.

## PlainCAD full production acceptance on `254c83b` — 2026-11-13

All [66 original production cases](evidence/plaincad-production-cylinder-stock-2026-11-13.json)
pass on PlainCAD `50432ed` in 308.528 seconds, with zero skips, retries
or flaky results. Both TypeScript checks, the production bundle budget
and independent browser WASM probe pass. The [original performance
case](evidence/plaincad-production-cylinder-stock-performance-2026-11-13.json)
finishes in 31.382 seconds with all 25 edits and 11 validated STL downloads.

## PlainCAD coaxial opposed blind bores on circular stock — 2026-11-13

The layered STEP reader merges connected wall spans without filling a
gap between opposite blind bores. Full graph regeneration still checks
the source trims and topology. Both signed placements match independent
OCCT measurements. The bridge native regression drills out the exact
web after STEP reimport.

The [three original Hole Edit and STEP cases](evidence/plaincad-circular-coaxial-web-browser-2026-11-13.json)
pass. A [same-WASM app probe](evidence/plaincad-circular-coaxial-web-native-2026-11-13.json)
reopens a circular part with an intact 3 mm web and verifies analytic
volume and area. Its first STL check used a 2 mm³ absolute tolerance
borrowed from the rectangular probe. The updated check verifies closed
incidence, all four axial rim levels and each regular 32-sided rim, then
matches signed volume to the analytic polygonal solid within 0.001 mm³.
The original PlainCAD cases and the supplemental exact B-rep checks keep
their assertions. Focused race checks and Go vet pass.

## PlainCAD preserves ordinary cylinder STEP as machinable stock — 2026-11-13

The native reader now retains the complete cylinder created by PlainCAD's
ordinary STEP writer. It checks the recovered cap and side topology
against the complete cylinder constructor. The focused regression drills
the reimported cylinder and exports it again. The
[original STEP import and export cases](evidence/plaincad-step-cylinder-stock-2026-11-13.json)
pass, as do the split/quarter cylinder, oblique cylinder, race and Go vet
checks.

## PlainCAD quarter-cylinder STEP import from main — 2026-11-13

Main's exact four-quarter cylinder model and rooted product dialects
are available through the PlainCAD bridge. Both retain the canonical
cylinder as native stock for a subsequent Hole and STEP export.
The [two original STEP cases](evidence/plaincad-quarter-cylinder-browser-2026-11-13.json)
pass. The [same-WASM Chromium case](evidence/plaincad-quarter-cylinder-native-2026-11-13.json)
imports the exact fixture, preserves it through reopen, and exports
analytic STEP and validated STL. Independent OCCT model and product
transfer, focused race checks and Go vet pass.

## PlainCAD split-cylinder STEP import from main — 2026-11-13

The isolated integration worktree takes main's exact two-half cylinder
coverage checks, signed placements, model and rooted product writers
and independent rejection fixtures. The PlainCAD bridge retains the
imported cylinder as native stock. The native check machines either
admitted input dialect and re-exports its analytic boundary.

Both [original STEP cases](evidence/plaincad-split-cylinder-browser-2026-11-13.json)
pass. The [same-WASM browser case](evidence/plaincad-split-cylinder-native-2026-11-13.json)
opens the signed posed fixture, preserves its exact source across
project reopen, and exports verified analytic STEP and validated STL.
Independent OCCT transfer checks, focused race checks and Go vet pass.

## PlainCAD full development acceptance on `50432ed` — 2026-11-13

All [344 original development cases](evidence/plaincad-development-50432ed-2026-11-13.json)
pass in 3,014.679 seconds on clean integration commit `92e22ab`.
There are zero skips, retries and flaky results. Both TypeScript
checks and the independent browser WASM probe pass. The full run
includes the original Parts Browser, YZ guarded drag, long modeling chain,
grouped edge treatments, helical parts, camera idle, ORBIT and 40-body
viewer cases. It includes the staged asynchronous project-open fix.

## PlainCAD reverse-endpoint blind corner cut — 2026-11-13

A finite corner cutter with its terminal endpoint on the stock cap now
shares the same native cut and overlap measures as the forward tool.
The native check covers scoped Cut, direct Cut and STEP readback.
Both [original guided Hole and scoped Revolve Cut tests](evidence/plaincad-corner-blind-endpoints-2026-11-13.json)
pass. Focused race checks and Go vet pass.

## PlainCAD same-site opposed blind bores on notched stock — 2026-11-13

Opposed blind cuts at one interior site preserve their intervening
material web. The native overlap query subtracts both previously cut
spans and reports the remaining material for a subsequent through tool.
STEP uses axial spans as well as circular supports to recover both bores.
The [three original grouped Hole and STEP cases](evidence/plaincad-opposed-coaxial-notch-browser-2026-11-13.json)
pass. The [same-WASM authored browser case](evidence/plaincad-opposed-coaxial-notch-native-2026-11-13.json)
preserves a 3 mm web and its analytic measures through STEP export,
reimport, project reopen and validated STL. Focused race checks and Go
vet pass.

## PlainCAD STEP units on notched stock — 2026-11-13

The bridge retains a qualified notched-stock boundary after converting
its recipe to millimetres. A native regression imports a centimetre STEP
file with a corner notch and an opposed blind bore, checks world bounds
and volume, adds another scoped bore and exports STEP again. The
[two original STEP browser cases](evidence/plaincad-notch-step-units-2026-11-13.json)
pass, along with focused race checks and Go vet.

## PlainCAD notched stock batch order — 2026-11-13

An interior through cutter joins opposed blind spans at the same site in
any input order, including after a blind span has already been committed.
Native checks cover the union's surface and volume, atomic rejection
of disconnected spans and canonical STEP readback. Both
[original grouped Hole and STEP cases](evidence/plaincad-notch-batch-order-2026-11-13.json)
pass on PlainCAD `50432ed`. Focused race checks and Go vet pass.

## Reproducible PlainCAD unit qualification — 2026-11-13

The staging harness now runs original Vitest cases with a locally served
Go WASM asset and one worker by default. It records the generated runtime
setup, selected assertions, source hashes and actual pass/fail counts.
The [unit and browser harness checks](evidence/plaincad-unit-harness-2026-11-13.json)
pass the original native cylinder rim unit test and Parts Browser
acceptance, including the independent browser backend probe.

The [full one-worker unit run](evidence/plaincad-unit-one-worker-full-2026-11-13.json)
passes 1,647 of 1,689 originals on PlainCAD `50432ed`, with zero pending
tests. Thirty-seven failures recur from the prior full run, 22 recover,
and five newly fail. The recorded harness source predates its final
formatting change, and the report includes the actual setup and WASM hashes.
The five new failures and the previously intermittent cold Unified thread
case all pass unchanged on the same staged WASM in a six-case retest.

## PlainCAD coaxial side and corner Hole extension — 2026-11-13

Connected depth spans at the same side or corner notch center and radius
extend its original floor or open the far cap. Native checks verify the
removed volume, surface area, grouped order and atomic rejection of an
isolated opposed span. [Four original guided, grouped Hole and STEP
cases](evidence/plaincad-coaxial-notch-browser-2026-11-13.json) pass.
The [same-WASM browser probe](evidence/plaincad-coaxial-notch-native-2026-11-13.json)
extends a blind corner Hole through the stock, verifies exact removed
volume and reopens the resulting STEP before validating STL. Focused
race checks and Go vet pass.

## PlainCAD mixed corner and contained Hole batch — 2026-11-13

A single blind or through corner Hole can share a native batch with
disjoint contained through bores and blind bores from either cap. Native
checks cover swapped input order, successive builds, exact measures,
atomic failure and canonical STEP. The independent OCCT readback matches
volume, area and centroid for both mixed corner cases. All
[ten original browser cases](evidence/plaincad-corner-batch-browser-2026-11-13.json)
pass on PlainCAD `50432ed`. The [same-WASM probe](evidence/plaincad-corner-batch-native-2026-11-13.json)
reopens an actual placed STEP export with three authored holes and
validates the resulting STL. Race checks and Go vet pass.

## PlainCAD STEP for quarter-circle corner Holes — 2026-11-13

The canonical notch codec now supports the existing rectangular stock
corner cuts, including the blind floor and through-all boundary. All
[ten original browser cases](evidence/plaincad-corner-step-browser-2026-11-13.json)
pass on PlainCAD `50432ed`, including Hole Edit, multi-body cuts and STEP.
Both [same-WASM authored corner cases](evidence/plaincad-corner-step-native-2026-11-13.json)
preserve placed bounds and analytic measures through STEP export and
import, project reopen and validated STL. The independent OCCT reader
matches volume, area and centroid in both corner cases. Focused race
checks and Go vet pass.

## PlainCAD STEP for a one-side through notch — 2026-11-13

A qualified through-all Hole opening one rectangular stock side now
round-trips canonical STEP, also when grouped with disjoint interior bores.
Native checks cover grouped and successive cuts, atomic rejection, exact
measures and placed bounds. Independent OCCT reads match volume, surface
area and centroid in five blind and through cases.

All [13 original browser regressions](evidence/plaincad-side-through-step-browser-2026-11-13.json)
pass on PlainCAD `50432ed`, including the export hub and STEP workers.
Both original Hole Edit cases pass separately on the same staged build.
The [same-WASM app probe](evidence/plaincad-side-through-step-native-2026-11-13.json)
exports a posed scoped through-all Hole, imports the downloaded STEP in a
fresh browser context, reopens the saved project and validates STL. Focused
race checks and Go vet pass.

## PlainCAD scoped through-all side notch — 2026-11-13

Single-tool scoped Hole now uses the same qualified rectangular side and
corner cylindrical notch constructors as direct Cut. Native tests compare
through-notch measures and bounds through both bridge operations. All
[five original browser regressions](evidence/plaincad-scoped-side-through-2026-11-13.json)
pass. The [same-WASM app probe](evidence/plaincad-scoped-side-through-native-2026-11-13.json)
rebuilds a through-all side-opening Hole, checks analytic area and volume,
saves and reloads it, and validates STL. Race checks and Go vet pass.

## PlainCAD unit run on `50432ed` — 2026-11-13

The [full unit report](evidence/plaincad-current-unit-50432ed-2026-11-13.json)
records 1,630 passes and 59 failures of 1,689 originals with real WASM
and loopback available, while browser and native suites also run. Of the
failures, 38 repeat the prior full run and 21 are new in this run. Eight
of the prior failures pass, including the inline sketch workspace test.
All 21 newly failed tests pass unchanged in the one-worker retest on the
same staged app and WASM, including all three helical cases. The 17-file
selection passes 213 of 217 originals. The four repeated failures in that
selection all exercise fallback behavior.

The [two remaining repeat diagnostics](evidence/plaincad-unit-remaining-controls-2026-11-13.json)
hold the test assertions fixed. The workbench layout case passes a
one-worker rerun on the original WASM. The laser screening fixture expects
three unverified profiles. It gets one with either initialized kernel,
and passes with OCCT left uninitialized, the fixture's original default.
The two authored rectangular extrusions have verified cap faces when the
native kernel is initialized, while the fitted body still receives a warning.

## PlainCAD asynchronous project-open readiness — 2026-11-13

The staged runtime keeps the previous drawable model while an asynchronous
project open is pending. Readiness belongs to the newest open. A rejected
or canceled open restores the previous readiness if no newer model state
has replaced it. Original fixture assertions and timeouts are unchanged.

All [seven original cases](evidence/plaincad-project-open-readiness-2026-11-13.json)
pass on `50432ed`, including Parts Browser, file-drop handling, recovery
and both stale-worker tests. The [supplemental browser control](evidence/plaincad-project-open-ownership-2026-11-13.json)
intercepts the actual import worker and verifies that a superseding invalid
open restores the old two-body model and that the obsolete open cannot
replace it.

## PlainCAD opposite-cap interior bores on notched stock — 2026-11-13

A disjoint contained blind hole can enter either cap of a side-notched
body. The same bounded constructor retains through bores, and canonical
STEP recovers which cap each blind cut opens from. Independent OCCT
readback matches the opposed, placed geometry's volume, area and centroid.

The [four original browser regressions](evidence/plaincad-opposed-side-batch-browser-2026-11-13.json)
pass on PlainCAD `50432ed`. The [native evidence](evidence/plaincad-opposed-side-batch-native-2026-11-13.json)
records two same-WASM checks for authored opposed Hole edits, reload,
placed STEP and validated STL, along with focused race checks and Go vet.

## Current PlainCAD cross-browser and focused acceptance — 2026-11-13

On committed PlainCAD `50432ed`, the
[original cross-browser suite](evidence/plaincad-current-cross-browser-2026-11-13.json)
passes all 15 cases across Chromium, Firefox and WebKit. Both TypeScript
checks, the production bundle budget and the actual Go WASM preflight pass.
The current app's committed test changes resolve the earlier stale control
locators. The integration harness preserves the current test assertions.

The [focused run](evidence/plaincad-current-focused-timing-2026-11-13.json)
passes six of seven originals. All four selected Focused modeling cases,
the original YZ guarded drag and the current ORBIT gesture pass. Parts
Browser again reads the previous successful empty rebuild while its
asynchronous file open is still pending. The staged readiness fix is
being verified against the same original case.

The combined native short run passes `internal/plaincadbridge`, `exchange/step`
and `mesh`, and builds `cmd/plaincadwasm`.

## PlainCAD mixed side-opening and contained bore batch — 2026-11-13

A single side-opening blind cut can share a native batch with disjoint
interior blind cuts from its entry cap and through bores. Grouped and
successive application preserve exact material volume and publish one
solid. Contact with the side-notch footprint and unsupported extra entry
configurations are rejected atomically. The retained batch round-trips
canonical STEP, and independent OCCT checks match volume, area and centroid.

All [seven original browser regressions](evidence/plaincad-mixed-side-batch-browser-2026-11-13.json)
pass for Hole, multi-body scope, and STEP. The [native evidence](evidence/plaincad-mixed-side-batch-native-2026-11-13.json)
records focused race checks, Go vet and two same-WASM app probes for authored
mixed Hole persistence and placed STEP-to-STL fabrication.

## PlainCAD STEP for a single side-opening blind bore — 2026-11-13

The canonical STEP path reconstructs the notched box from its actual plane
and cylinder supports, verifies all topology and pcurves, and preserves the
original box extrema for cardinal placements. Independent OCCT reads match
native volume, area and centroid for two posed cases with the tool center
on either side of the crossed stock plane.

All [eight original browser regressions](evidence/plaincad-side-notch-step-browser-2026-11-13.json)
pass for Hole Edit, machined references, STEP workers, import and export.
The [native evidence](evidence/plaincad-side-notch-step-native-2026-11-13.json)
records the focused race checks, Go vet and both supplemental same-WASM
checks for authored persistence and placed STEP-to-STL fabrication.

## PlainCAD full development rerun after viewer changes — 2026-11-13

The [344-case development rerun](evidence/plaincad-development-viewer-complete-2026-11-13.json)
on `7cd0396` passes 342 originals in 3,569.354 seconds. There are no
retries or skips. The original pan idle, source point markers, gesture PNG
and 40-body viewer cases all pass on that run.

Parts Browser reads the prior empty rebuild while its project is opening.
The YZ guarded drag reaches its 20-second startup assertion while the
kernel is loading. Both original cases pass unchanged on the identical
WASM snapshot in isolated retests. The Parts Browser original also passes
on pristine OCCT. The complete suite retains both failures in its record.

## PlainCAD full production rerun on `50432ed` — 2026-11-13

The [later corner-batch production run](evidence/plaincad-production-corner-batch-2026-11-13.json)
tests clean `d2db51d` and passes 65 of 66 originals, both TypeScript
checks, the bundle budget and the independent WASM probe. The benchmark
exceeds its 90-second case deadline while several suites run. Its
[recorded samples](evidence/plaincad-production-corner-batch-performance-timeout-2026-11-13.json)
include all 25 edits and 11 validated STL downloads. The
[unchanged retest](evidence/plaincad-production-corner-batch-performance-retest-2026-11-13.json)
passes in 56.048 seconds with the same staged production assets, WASM,
deadline and test assertions.

All [66 original production tests](evidence/plaincad-production-50432ed-2026-11-13.json)
pass in 357.132 seconds, with zero skips, retries or flaky results. Both
TypeScript checks, the production bundle budget, and the separate WASM
probe pass. The [benchmark](evidence/plaincad-production-50432ed-performance-2026-11-13.json)
passes in 42.966 seconds against its original 90-second deadline, with
25 verified edits and 11 validated STL exports.

The run started on `03d5740` with the staged project-open change in the
working tree. The report's source hash for `display-runtime.mjs` matches
the committed file in `92e22ab`. The report retains the working-tree
identity and hashes of the inputs that were actually run.

## PlainCAD full production qualification with dense mesh validation — 2026-11-13

All [66 original production tests](evidence/plaincad-production-dense-validation-2026-11-13.json)
pass on `11216c5`, with zero retries or skips, including both TypeScript
checks, the production bundle budget, the real-WASM probe and the original
performance test. The [benchmark report](evidence/plaincad-production-dense-performance-2026-11-13.json)
records 25 verified edits and 11 validated 144-triangle STL exports. The case
finishes in 60.790 seconds against its original 90-second deadline.

Main's dense-link manifold validation passes its 300 seeded comparisons
against the reference implementation and rejects the disconnected vertex fan.
A native benchmark on this host records 12.0–18.4 ms and 147 allocations per
mesh, compared with 34.4–57.0 ms and roughly 137,505 allocations for the
reference. The browser result measures the integrated build.

## PlainCAD side-opening blind cylindrical notch — 2026-11-13

One finite axial cylinder crossing one rectangular stock side now forms
a watertight notch with its true circular-segment floor. Direct Cut and
single-tool scoped Hole share the same analytic construction. Native
checks cover all axis signs, tool endpoint orders, centers inside and
outside the crossed side, volume, area, bounds and STL topology.

All [seven original regressions](evidence/plaincad-side-blind-notch-browser-2026-11-13.json)
pass for guided Hole, edited Hole, the long fillet/chamfer/machining chain,
and multi-body Extrude/Revolve Cut. A [supplemental app test](evidence/plaincad-side-blind-notch-native-2026-11-13.json)
uses the same WASM to rebuild and reload an authored Hole and validate STL.
Native race checks and Go vet pass. STEP for the side-opening notch and
mixed batches combining side-opening tools remain open.

## PlainCAD STEP for overlapping circular bores — 2026-11-13

The canonical STEP writer preserves the actual circular arcs, planar
crescent floors, cylindrical walls, face uses and paired pcurves. The
reader recovers candidate cutter intervals from those surfaces and accepts
the file only if native reconstruction regenerates the complete graph.
The focused suite verifies reader limits, graph corruption rejection,
placement and retained editing after reimport. Independent OCCT checks
match area, volume and centroid for both sweep directions.

All [eight original cases](evidence/plaincad-layered-round-stock-step-browser-2026-11-13.json)
pass for STEP import, export and cancellation, stale workers, machined
references and Hole Edit. The [native evidence](evidence/plaincad-layered-round-stock-step-native-2026-11-13.json)
records two same-WASM supplemental checks for authored rebuild, persistence,
STEP and validated STL, the native race checks and Go vet. The full
browser run's YZ guarded sketch drag reaches the readiness deadline with
`loadingKernel`; the unchanged original passes on the same WASM in
10.826 seconds in isolation.

## PlainCAD overlapping bores on circular stock — 2026-11-13

Circular stock now reuses the exact axial bore arrangement. Unequal-depth
opposed tools, subsequent cuts and measure-only overlap preserve material
volume, cylindrical walls and crescent floor topology. Native tests also
cover the original planar-stock callers and verify signed, placed stock,
independently integrated area and volume, and watertight tessellation.
STEP exchange remains open for the intersecting bore boundary.

All [eight original browser cases](evidence/plaincad-overlap-round-stock-browser-2026-11-13.json)
pass for Hole, grouped edge edits, multi-body scopes, and STEP import. A
[supplemental PlainCAD check](evidence/plaincad-overlap-round-stock-native-2026-11-13.json)
uses the same WASM for authored opposed cuts, project reload and validated
STL. Focused native race checks and Go vet pass. The same evidence records
both successful native full-mode spline tests and the unchanged Parts Browser
reruns. The browser full-run failure read the initial empty rebuild before the
asynchronous project open. The same WASM and pristine OCCT both pass isolated
reruns of that test.

## Additional full-mode trimmed spline checks — 2026-11-13

`TestNestedQuarticTubeMaterial` and
`TestCombineTransverseSplineShellsAdditionalOperations` both pass in the
integration worktree with full mode enabled. The tests take 524.10 and
29.69 seconds. The separate 398-case short run excludes those two cases
and two explicitly opt-in investigations.

## PlainCAD disjoint blind bores from opposite circular caps — 2026-11-13

Disjoint blind cuts from either circular cap retain analytic walls and floors,
including unequal depths whose axial spans overlap. Sequential and grouped
cuts work in both entry orders. Canonical STEP readback and the independently
validated analytic STEP path preserve which cap each pocket opens from. The
pinned OCCT reader independently matches area, volume and centroid for the
new opposed cases and the signed single-cap controls.

All [six original browser regressions](evidence/plaincad-opposed-round-stock-browser-2026-11-13.json)
pass. Two supplemental PlainCAD tests on the same WASM pass authored opposing
Hole features through rebuild and save/reopen, STEP reimport, and full STL
fabrication. The [native evidence](evidence/plaincad-opposed-round-stock-native-2026-11-13.json)
records race checks for the bridge, STEP and trimmed B-rep and Go vet.

## PlainCAD grouped and successive circular-stock bores — 2026-11-13

The bridge retains disjoint through bores and blind bores from one cap
on circular stock, including authored profiles with eccentric inner
wires and stock reimported from STEP. A group skips redundant tools
and publishes only after the complete exact boundary succeeds. The
measure-only query subtracts voids already present in the retained body.
The current constructor limits circular stock to 16 sites and rejects
intersecting bore footprints and blind entries from both caps.

All [eight original browser cases](evidence/plaincad-grouped-round-stock-browser-2026-11-13.json)
pass for Hole creation/edit, Revolve, compound profiles, and STEP
exchange. Two supplemental PlainCAD tests use the same WASM bytes to
qualify editable grouped and successive holes, project reload, STEP
readback, and STL fabrication. The [native evidence](evidence/plaincad-grouped-round-stock-native-2026-11-13.json)
includes the focused race checks and Go vet. The preceding full bridge
short run passes, and a separate trimmed B-rep short run passes 398
top-level cases with four skips. The two non-opt-in cases skipped by
short mode have been started separately.

## PlainCAD production on the staged viewer update — 2026-11-13

The [integrated production run](evidence/plaincad-integrated-production-2026-11-13.json)
passes 65 of 66 original cases. The 90-second performance case records all
25 current-model edits and 11 fully validated STL downloads, then reaches the
test deadline. An identical staged build passes the same original case in
89.225 seconds on the second isolated retest. The earlier production build
passes the parallel control in 86.294 seconds. The
[benchmark artifacts](evidence/plaincad-integrated-production-benchmark-controls-2026-11-13.json)
record both timeouts and the successful controls. All observed STL outputs
retain 144 triangles and their original geometry checks.

## PlainCAD eccentric bores in circular stock — 2026-11-13

Finite, strictly contained cylindrical tools now cut native circular
stock in the scoped Hole and direct Cut paths. Both entry signs and
through cuts retain exact circular boundary faces and analytic measures.
The bridge also reads and reexports independently authored round stock
with one blind and one through bore. Tests cover native measures,
manifold tessellation, and STEP readback.

The [five original browser regressions](evidence/plaincad-circular-stock-browser-2026-11-13.json)
pass unchanged. The [real-WASM supplement](evidence/plaincad-circular-stock-native-2026-11-13.json)
verifies scoped overlap and Cut, STEP reimport and PlainCAD STL
fabrication validation. The focused native bridge and STEP race checks
and Go vet pass.

## PlainCAD consolidated current development run — 2026-11-13

The [full current development report](evidence/plaincad-current-development-full-2026-11-13.json)
passes 342 of 344 original cases in 3,721.761 seconds. No cases skip or
retry. The source-hashed WASM snapshot includes the analytic arc profiles
and the separate backend preflight passes. Two staged storage-state origins
follow the isolated server port. Original assertions and deadlines were
preserved.

The two failures are viewport idle after a right-button pan and the final
Playwright screenshot of the ORBIT gesture case. The gesture run completes
the native geometry, full-resolution PNG, buffer reuse, and resolution
restoration assertions. Its final screenshot exceeds the original deadline.
The unchanged viewport idle test reproduces on the same PlainSolid WASM
and with pristine OCCT at the same pan step. The
[isolated viewer controls](evidence/plaincad-viewer-idle-controls-2026-11-13.json)
record both reproductions and the original gesture case passing an
unchanged rerun in 88.653 seconds. The staged viewer now advances missed
camera damping updates without issuing extra WebGL draws. The original
pan-idle, source-marker and gesture cases all pass with that change.
Their measured times are 38.779, 20.592 and 54.799 seconds. The
[final viewer harness run](evidence/plaincad-final-viewer-harness-2026-11-13.json)
passes those three originals plus studio effects, 40-body GPU buffer
reuse, and viewport fit. All six pass with zero skips or retries. Applying
the same camera change to a separate OCCT-backed staged copy also passes
the original pan-idle case.

## PlainCAD STEP for rounded stock and canonical long-edge chains — 2026-11-13

The bridge now reads and reexports independent OCCT-authored rounded
stock with separate circular blind and through bores. It also round-trips
the origin-anchored opposite fillet/chamfer stock and its complete circular
through bore, rectangular through pocket and upper boss chain using the
existing exact STEP graph readers. The rounded-pocket reader and fixture
come from main's `c35c793`; this worktree uses the committed independent
file for the comparison.

All [six original browser regressions](evidence/plaincad-machined-step-browser-2026-11-13.json)
pass, including exact STEP measurements and placement, worker cancellation,
retry, imported STL and machined references. The
[real-WASM supplement](evidence/plaincad-machined-step-native-2026-11-13.json)
imports the independent rounded stock, places it, reexports, reloads,
and runs PlainCAD's STL fabrication checks. Native round trips and focused
race checks pass in the bridge and STEP packages.

## PlainCAD opposed-entry blind bores — 2026-11-13

The layered cylinder construction now retains tools entering either stock
cap. Exact circular footprint changes orient each floor toward its
corresponding opening. Tests cover intersecting axial intervals and tools
that meet at one floor plane, including a coaxial through opening and two
oppositely oriented crescent floors. Independent volume and area formulas,
B-rep integration, ordered and reversed batches, manifold mesh validation,
and measure-only overlap queries pass on the qualified domain.

All [12 original browser regressions](evidence/plaincad-opposed-holes-browser-2026-11-13.json)
pass, covering guided blind and through holes, editing, multi-body scope,
rollback and STL. The [real-WASM supplement](evidence/plaincad-opposed-hole-native-2026-11-13.json)
builds two intersecting blind Hole features from opposite faces of one
PlainCAD document, verifies exact measures, reopens the saved document,
and validates its STL. The supplemental fixture is separate from the
original acceptance cases. The same final WASM passes the unchanged
side-tangent guided Hole test after adopting the shared exact contact
classifier from main.

## PlainCAD exact side-tangent blind Hole — 2026-11-13

The [focused retest](evidence/plaincad-tangent-contact-2026-11-13.json)
passes all seven portable failures from the full original unit baseline
and both neighboring outward through-all cases with real Go WASM. The
original guided Hole test accepts the exact one-millimetre and
one-tenth-millimetre contacts. Native checks cover both tool axis signs,
measure-only overlap, exact area and volume, planar floor reference,
edge display, rigid reflection, and rejection of through and off-contact
tools from this domain.

The [pristine OCCT contact probe](evidence/plaincad-occt-tangent-contact-2026-11-13.json)
finds four incident triangles on one welded contact edge. PlainCAD's
fabrication validation rejects that mesh. The PlainSolid model retains the
same exact contact as a regularized material boundary and displays it
with one balanced four-use edge. Every other display edge has two
opposite uses. Native STL and STEP export reject the boundary. An additional
real-WASM probe confirms that PlainCAD's STL fabrication API rejects it
with both topology-only and full validation. The native bridge short
suite passes in 152.081 seconds.

## PlainCAD full unit rerun and provider controls — 2026-11-13

The [current unit reports](evidence/plaincad-current-unit-controls-2026-11-13.json)
record 1,643 passes of 1,689 originals with initialized WASM and loopback
available. All 15 local middleware cases and all 39 modeling geometry
cases pass. The sandboxed concurrent run records 1,622 passes. All seven
portable failures from the original baseline pass both reruns.

Of the 46 failures with loopback, 36 repeat fallback-dependent or
OCCT-private fixture assumptions. Nine of the ten new failures pass
unchanged in focused retests. The remaining inline sketch workspace
fixture reproduces on pristine OCCT. The full original unit gate remains
open, including the provider-specific setup described below.

## PlainCAD current real-WASM unit baseline — 2026-11-13

The [classified unit report](evidence/plaincad-current-unit-baseline-2026-11-13.json)
records 1,646 passes and 43 failures across 219 original test files on
PlainCAD `e65f07b`. The isolated runner initializes real Go WASM before each
file. Twenty-six failed assertions depend on uninitialized fallback behavior
and ten inspect or mock private OCCT APIs or handle layouts. Seven portable
assertions remain for focused verification: two treatments after pockets,
one tangent hole, two outward-Hole diagnostics, one display budget diagnostic,
and one tangent-contour selection rule. The
[focused unit retest](evidence/plaincad-portable-unit-final-2026-11-13.json)
passes the six repaired assertions, both neighboring outward through-all
assertions, and repeats the exactly tangent blind Hole failure. The
pristine OCCT control passes that original case. The exact circular wall
and stock side touch along a generator, where the boundary has four local
surface branches. PlainSolid currently rejects this operation under its
closed manifold boundary contract. The staged test sources and authored
hole dimensions were preserved.

The [fixture controls](evidence/plaincad-unit-fixture-controls-2026-11-13.json)
run the ten OCCT-private cases against pristine OCCT, where all pass.
Among 26 fallback-dependent originals, pristine OCCT with the original
initialization mode passes 25 and fails one at missing extrusion draft
setup. PlainSolid with the global WASM initializer omitted passes five
and fails 21. The replacement adapter requires an initialized WASM
kernel for modeling. The fallback cases exercise the application
provider's pre-initialization behavior and still need provider-neutral
fixture setup for a replacement-only unit gate.

## PlainCAD cross-browser acceptance and OCCT control — 2026-11-13

The [cross-browser report](evidence/plaincad-current-cross-browser-2026-11-13.json)
passes the original keyboard face-card and dimension dialog case in Chromium,
Firefox, and WebKit. Four other cases on each engine stop on the original
locators for the sketch row, AI drawer, part name, or Start a part region.
The [pristine Chromium OCCT control](evidence/plaincad-cross-browser-occt-control-2026-11-13.json)
reproduces the same four failures on PlainCAD `e65f07b`. The
[Firefox and WebKit OCCT controls](evidence/plaincad-cross-browser-current-controls-2026-11-13.json)
reproduce the same failures on both engines. The same report records all
12 passes when the existing current production controls for the compact
Workbench, AI drawer, and native sketch are run across all three engines.
The current application opens the part-name controls through the closed-by-default
New part menu and exposes AI through the bottom dock. The original tests
and production application checkout were preserved.

The [three-engine production modeling selection](evidence/plaincad-native-three-engines-2026-11-13.json)
passes six cases in 43.3 seconds. It runs the original production CSP
modeling and STL case and the placed STEP import case in each of Chromium,
Firefox, and WebKit. The staged configuration changes only the preview port,
artifact directory, and browser projects.

## PlainCAD full current production acceptance — 2026-11-13

The [production rerun after exact side-tangent Hole support](evidence/plaincad-current-production-tangent-build-2026-11-13.json)
passes all 66 original cases in 611.585 seconds on committed PlainSolid
`0b17938`. The TypeScript checks, real-WASM backend preflight, and production
bundle budget pass. There were zero skips, flakes or retries. The original
performance case passes its 90-second deadline while other validation
runs are active on the same host.

The [production report](evidence/plaincad-current-production-2026-11-13.json)
passes 66 of 66 originals in 403.549 seconds on PlainCAD `e65f07b`. No
tests skip, flake or retry. The built app uses the same Go WASM as the
isolated performance comparison. The performance case also passes within
the full run in 56.2 seconds. The original production configuration,
assertions and deadlines are preserved. The build passes the JavaScript
bundle budget across 72 bundles, with a largest bundle of 496.82 kB.

The [production rerun after arc-region support](evidence/plaincad-current-production-arc-build-2026-11-13.json)
also passes all 66 originals in 417.857 seconds. It uses the same analytic arc
support as the full current development run.

## PlainCAD isolated production performance — 2026-11-13

The [production benchmark comparison](evidence/plaincad-production-performance-isolated-2026-11-13.json)
records the original test on two serial production builds of PlainCAD
`e65f07b`. PlainSolid passes in 29.5 seconds. The pristine OCCT control
exceeds the test's original 90-second deadline. Both complete 25 edits
and 11 exports with the original volume, bounds, winding and triangle
checks. No benchmark assertions or deadlines were changed.

Median edit-to-current-model time was 721 ms for PlainSolid and 1,100 ms
for the OCCT control. Median export-to-download time was 69 ms and 150 ms.
Cold load took 1.015 and 3.518 seconds. The production bundle budget passes
for both builds. These measurements describe one workload and one run per
provider.

## PlainCAD full development baseline — 2026-11-13

The [classified baseline](evidence/plaincad-development-baseline-2026-11-13.json)
records 264 passes and 76 failures on frozen PlainSolid `048dd26` and
PlainCAD `5942ff9`, with zero skips or flakes. The 340 originals ran in
105.3 minutes. Forty-four failures later passed focused original tests.
Another ten passed when rerun on the original port 5279. Eleven had a
later failing focused result and eleven lacked a focused retest at the
time of classification. These results span artifacts and require a current
consolidated run before release readiness can be claimed.

The [first arc-region retest](evidence/plaincad-current-arc-loop-baseline-2026-11-13.json)
passes all six analytic circle/arc divider originals on XY, XZ and YZ.
The remaining ten fail in the older frozen preview path. The
[current 17-case retest](evidence/plaincad-current-arc-loop-2026-11-13.json)
passes every original in 162.429 seconds, including major arc fragmentation,
lens and crescent selection, circular notches, arc-only intersections, and
arc Extend on YZ. Native race tests verify signed convex/concave corners,
analytic volume, mesh closure and JSON round-trips. The
[development retest](evidence/plaincad-current-development-retests-2026-11-13.json)
passes all 17 originals in the remaining focused selection. That includes
configuration packaging, placed circles, reusable parts, both PNG tests,
gallery, and 40-body GPU buffer reuse. The [revolve retest](evidence/plaincad-current-revolve-capture-2026-11-13.json)
passes original Cut and Join auto-target capture on PlainCAD `e65f07b`.

## PlainCAD current studio, native modeling and lineage — 2026-11-13

PlainCAD `e65f07b` was archived with the locked npm dependencies. Its
[studio baseline](evidence/plaincad-current-studio-baseline-2026-11-13.json)
passes six originals. The remaining studio case requires the independently
cancellable detail worker on the small template. The
[cancellation retest](evidence/plaincad-current-studio-cancellation-2026-11-13.json)
passes both the original studio cancellation and the ORBIT full-resolution
gesture export on the same artifact.

The [original native unit selection](evidence/plaincad-lineage-unit-2026-11-13.json)
records the eight lineage cases and six geometry cases on hashed Go WASM.
Twelve pass initially, including the convex/concave corner pair with no net
volume change, circular rims and sloped circular termination. Two originals
hit the sketch solver budget before the native geometry assertions and pass
unchanged on the same artifact in a separate run. A prior eight-case lineage
selection passes in one run. The retained geometry supports closed internal
cavities with a validated outer shell and inward cavity shells. The native
race run also passes the translated and rotated reopening case.

The [exact bounds and arc STL retest](evidence/plaincad-current-arc-bounds-2026-11-13.json)
passes all five originals, including mirror and linear sketch copies.
The [portable unit evidence](evidence/plaincad-current-portable-unit-repairs-2026-11-13.json)
records the machined STEP round-trip, thin-wall thread diagnostic, STEP
ownership validation, and compact circular treatment mesh. The fine
one-, two- and four-start lead-screw pairs pass the app mesh validator
and native fit checks. Provider-internal OCCT fixture tests are tracked
separately from these portable contracts.

## PlainCAD ORBIT display and signed profile edges — 2026-11-13

The [ORBIT display report](evidence/plaincad-current-orbit-display-2026-11-13.json)
passes all three original cases on PlainCAD `1c591283` in 129.156 seconds.
These cover fine close-up tessellation, Render visibility and PNG export, and
a full-resolution PNG during orbit or zoom. The harness records the runtime
changes applied to the isolated application copy. Fine display uses the
accepted geometry worker's native feature cache and retains the original
document, bounds, volume and cancellation checks. The original test bodies
and deadlines are unchanged. Both TypeScript checks and the separate real
WASM provider check pass.

The [signed edge report](evidence/plaincad-signed-edges-2026-11-13.json)
passes all three original XY, XZ and YZ negative/symmetric profile-edge
cases in 116.943 seconds. The [retained reference report](evidence/plaincad-retained-and-origin-2026-11-13.json)
passes 18 originals for retained edges, workspace and guided modeling, Parts
Browser, and AI To Face. The circular side and corner cut and moved projected
cut also pass their original selections.

## PlainCAD rotary fixture and downstream Hole edit — 2026-11-13

The [rotary acceptance](evidence/plaincad-rotary-accepted-2026-11-13.json)
passes the original 40-body cold load, rename and platter edit in 80.381
seconds. Compact axial cylinder sampling retains 68 native features in
6,039,938 bytes. Rename uses 39 edge-proof cache hits and zero modeling
operations. The platter edit uses 13 edge-proof hits and 47 modeling
operations. The [metrics](evidence/plaincad-rotary-cache-metrics-2026-11-13.json)
record cache behavior across all three rebuilds.

The original explicit Join/Cut test passes in 10.9 seconds on the
[earlier snapshot](evidence/plaincad-rotary-first-2026-11-13.json).
Exact axis-aligned material cells retain the 1,050 cubic millimeter join
and 750 cubic millimeter cut when the generic planar output cannot be
represented. The [Hole Edit retest](evidence/plaincad-hole-edit-chamfer-2026-11-13.json)
passes the downstream chamfer, cancellation, native volume, undo, save/open
and STL. The rotary snapshot passes Hole Edit again.

The [compact bore production report](evidence/plaincad-production-mesh-2026-11-13.json)
passes STEP import and still exceeds the overall 90-second performance
deadline under concurrent browser load. All 25 edit samples and 11 STL
downloads were validated. The STL mesh now uses 144 triangles and
retains the requested angular accuracy. An isolated performance retest
is still required.

## PlainCAD modeling and production repairs — 2026-11-13

The [development retest](evidence/plaincad-gap-repairs-2026-11-13.json)
passes all seven originals with zero skips or flakes. Exact analytic
boundaries preserve the four prior through bores through a blind pocket,
the island in a rectangular pocket, the circular boss, and the top fillet
through an opposite-cap chamfer. The remaining cases exercise compound
profiles, XY holes and pocket, and separated rotated collision. Both
TypeScript checks and the separate WASM check pass.

The [production retest](evidence/plaincad-production-repairs-2026-11-13.json)
passes the original fabrication package, cable-guide live preview and
left-hand Acme pair. The quarter-inch UNC pair reaches placement after
building the exact external screw and internally threaded rectangular
block, then fails the uniform display seed. Conforming helical tessellation
passes both the native placement and the [original gallery retest](evidence/plaincad-production-unc-2026-11-13.json).
The same production snapshot completes all performance samples and
exceeds the 90-second overall deadline while generating the report.
The [circle retest](evidence/plaincad-circle-measurement-2026-11-13.json)
passes original cylinder measurement and grouped circular fillets across
XY, XZ and YZ.

## PlainCAD original modeled Thread and lead pair — 2026-11-13

The [accepted helical report](evidence/plaincad-helical-accepted-2026-11-13.json)
passes both original cases and the independent WASM probe, with both
TypeScript checks passing and zero skips or flakes. Modeled Thread takes
79.325 seconds and the four-start lead pair takes 22.783 seconds. Exact
constant trim interpolation removes the WebAssembly-only internal
construction failure. Final-boundary validation in the composed external
thread constructor avoids repeated display validation in each cold plan
worker. The original geometry assertions and 90-second deadline pass.

## PlainCAD full production baseline — 2026-11-13

The first full production run passes 61 of 66 original cases in 445.821
seconds, with zero skips or flakes. Production compilation and the original
bundle budget pass with 70 JavaScript bundles and a maximum of 496,089
bytes. The [full report](evidence/plaincad-production-full-baseline-2026-11-13.json)
records the exact WASM and source hashes, every original result and the
original five failures.

The fabrication package remains in its STEP validation stage at the
original 20-second wait. The controlled performance test records all
25 edit samples and the measured STL exports, then exceeds its overall
deadline. The cable-guide gallery cannot display its native preview.
The Acme and UNC cases expose a nonrepresentable reciprocal lead in the
native wall chart, and UNC also requires an internal thread on bored
rectangular stock. The M6 and trapezoidal gallery originals pass.

## PlainCAD scoped Join, fitted parts and cached rebuild — 2026-11-13

The [expanded acceptance report](evidence/plaincad-expanded-first-2026-11-13.json)
passes nine of eleven originals, both TypeScript checks and the native WASM
probe. Scoped Extrude and Revolve Join connect selected targets, preserve
first-target identity, reject disconnected or contained selections, and
survive edits, save/open and STL. The native bridge allocates no intermediate
handles. Its cylindrical route proves exact containment of every planar target.
Go race checks for scoped Join, ownership and rectangular shell pass.

Centered box creation matches the PlainCAD adapter contract and all three
fitted styles pass the original edit, placement, save/open and STL workflow.
The incremental native cache test and rectangular Shell pass. Three original
Hole pattern cases pass. The two remaining failures expose a planar cut on
already-pocketed stock and a cylinder/cylinder overlap query. Focused native
repairs are underway.

The [focused follow-up](evidence/plaincad-pattern-performance-2026-11-13.json)
passes both previously failing original patterns and the original controlled
performance benchmark. Exact finite cylinder lens overlap diagnoses pattern
collisions. Direct planar cap sewing preserves the represented rotated
pocket profile and validates the resulting connected material boundary.
The benchmark passes its original rebuild, export, native ownership and
WASM capacity checks. Slider and hinge pass. A true empty planar
intersection in the rotated assembly case and missing curved features in
the 40-body rotary fixture account for the two failures in that report.

## PlainCAD thread mesh embedding and native rebuild — 2026-11-13

Skinny seam triangles in external threads could cross after the float32
conversion used by STL. The native tessellator now collapses eligible
interior vertices onto fixed exterior cylindrical trim vertices while
retaining the shared model edges and checking the replacement triangles.
Fresh WASM meshes for the 3 mm and edited 2 mm external M3 pass PlainCAD's
original full mesh validator with 14,268 and 11,414 triangles. Native
float32 embedding checks pass for both external lengths and the internal
3 mm groove. Both lead hands and both screw/nut parts pass native mesh
checks. The stepped head attachment validates the completed boundary once
after its three internal rim splits. Existing public split and attachment
tests pass.

The [second browser report](evidence/plaincad-helical-second-run-2026-11-13.json)
records external creation, edit, cold-open and STL passing before the
overall original Thread test times out during internal preview. That
intermediate snapshot also regressed the lead nut mesh budget. Restricting
boundary collapse to exterior twisted cylinders restores the native lead
checks. Both full originals are being rerun with that restriction and the
head attachment change.

## PlainCAD viewport operation drops — 2026-11-13

All eight original cases pass in 57.369 seconds. Complete rectangular cap
fillets and chamfers on XY, XZ and YZ pass preview, native volume, Undo/Redo,
parameter edits, save/open and STL. The other cases cover individual
chamfers on those planes, cancellation and a section overlay. The two
TypeScript checks and the separate Go WASM probe pass. The
[report](evidence/plaincad-operation-drops-2026-11-13.json) records the
snapshot with zero skips or flakes.

## PlainCAD retained references after Cut and Join — 2026-11-13

The first five-case run passes the original complete outer-perimeter
chamfer after a bore, single-edge fillet after a rectangular extension,
split-cap ambiguity and earlier-sketch cases. Its two failures are a
merged STL requiring a circular boss on bored stock and a rectangular join
whose unmerged coplanar fragments obscure the original side face.

The corrected two-case run passes retained cap edits, merged STL and a
sketch on the original side after Join, in 54.1 seconds. The union merges
qualified aligned boxes to a single native boundary. A circular boss on a
qualified bored cap sews the actual analytic cap and cylinder, retaining
the disjoint blind or through bore. Both TypeScript checks and the WASM
probe pass, with zero skips or flakes. The [first report](evidence/plaincad-retained-references-first-2026-11-13.json)
and [corrected report](evidence/plaincad-retained-references-2026-11-13.json)
preserve the exact snapshots.

## PlainCAD selected production acceptance — 2026-11-13

All seven selected original production cases pass in 25.572 seconds, with
zero skips or flakes. They exercise CSP, native modeling and STL,
scope capture, shop drawing, and STEP import and export with the original
independent reader. The production build passes its original JavaScript
bundle budget, with 70 bundles and a 496,089 byte maximum.

The earlier production snapshots exposed two gaps. The Go WASM was served
outside the asset path inspected by the CSP test, and the radius-10
cylinder mesh lost 0.641% of native volume. The final package serves the
kernel under assets and uses finer cylinder sampling, with unchanged
original assertions. The [production report](evidence/plaincad-production-2026-11-13.json)
pins the staged app, original test selection, build and WASM.

## PlainCAD grouped circular cap rims — 2026-11-13

The adapter resolves a complete original circular cap as one native rim.
The analytic cylinder retains selected quarter-torus blends and planar caps.
All three original XY, XZ and YZ grouped circular-perimeter cases pass their
native removed-volume assertions. Both TypeScript checks and the separate
real-WASM supplement pass. The [three-case report](evidence/plaincad-circle-rim-2026-11-13.json)
records the exact snapshot and zero skips or flaky results.

## PlainCAD machined references and multi-rim mesh — 2026-11-13

The qualified rectangular stock retains a blind rectangular pocket and
nested floor cut, with a disjoint analytic through bore. Floor validation
compares the complete current finite wire and its outward direction.
The trimmed floor becomes unavailable after the nested cut. Bore rim
measurements require a complete inward cylinder and a matching circular
wire on its outward planar cap. The original machined-reference acceptance
passes native measurements, repair, parameter edits, save/open and floor
loss after the nested cut.

The [four-case report](evidence/plaincad-machined-references-2026-11-13.json)
also passes the two original XY guided multi-hole cases and imported STEP
STL. The acceptance stage takes 50.0 seconds. The separate
[three-case regression report](evidence/plaincad-multi-rim-mesh-2026-11-13.json)
passes the same guided and STEP cases in 39.2 seconds. Both runs pass both
TypeScript checks and the real-WASM supplement with zero skips or flakes.

## PlainCAD curved profiles, revolutions, drawing and STEP — 2026-11-13

The bridge retains analytic semicircles, circular segments, capsules, nested
capsule rings and qualified circle holes. A circular segment reports its
actual arc/chord extent after rigid placement. This matters after the
original sketch solver edits the radius and returns an arc slightly shorter
than a semicircle. Native cap queries integrate the straight and elliptic
trims in the actual planar chart. The adapter uses those queries before
offering complete untouched caps for linked projection.

Native revolutions include full cylinders, quarter sectors, annular bores,
ring tori and torus elbows. A stepped shaft with a polygonal blind socket
supports the original metric and inch blank. Drawing measures complete
inward cylindrical bands and places their axes and endpoints in world
coordinates. STEP covers the selected planar and analytic imports and
exports, including the original placed bored plate and capsule ring.
The export reader verifies the retained body count, analytic measures and
world bounds before publication. Existing STEP tests independently read
the generated STEP files using their original reader.

The first 10-case mixed/revolve run passed eight originals. The next run
passed 11 of 13, including linked arc and revolve edit. Separate original
runs passed the socket-head blank, guided tap-drill and native drawing
workflows. Four of five STEP originals passed on the first run, including
all three export cases, independent readback, cancellation, stale results
and worker load recovery. The STEP import then missed its STL volume error
bound. The later three-case run passes the original imported STEP STL,
the placed drawing and the solved circular-segment arc edit. Both TypeScript
checks and the separate Go WASM proof pass. The [first STEP report](evidence/plaincad-step-first-run-2026-11-13.json)
and [affected-case report](evidence/plaincad-curved-profile-drawing-step-2026-11-13.json)
preserve their distinct snapshots. The first
[consolidated report](evidence/plaincad-consolidated-regressions-2026-11-13.json)
passes 47 of 53 original cases in 579.170 seconds, with zero skips or flakes.
All six failures are guided multi-hole previews exhausting the mesh budget
with the finer rim setting. The corrected setting retains fine single-bore
meshes and restores the original budget for multiple rims.

The [helical baseline](evidence/plaincad-helical-baseline-2026-11-13.json)
records zero of two original PC4 passes. The thread case was blocked by the
hex-socket blank, subsequently fixed. The lead case was blocked by the
adapter's unavailable helical capability. Neither reached a helical
geometry assertion in that snapshot.

## Tangent through pocket on filleted stock — 2026-11-13

A rectangular pocket may share an exact top edge with a longitudinal fillet.
The native constructor splits the cylindrical face at the two contact
endpoints and incorporates the three remaining pocket edges into the planar
cap boundary. The pocket wall shares the middle cylinder edge directly.
The opposite cap, other pocket edges, and adjacent features still require
their prior clearance.

The native fillet suite passes, including both cap-edge choices at depths 10
and 15, independent volume and area, a closed display mesh, native JSON
roundtrip, and rejection on both sides of unresolved near-contact. The
complete bore, tangent pocket and boss chain also passes a deterministic
native case. The original long-chain browser case passed in the first
combined profile and revolve run. A consolidated application rerun is pending.

## Native mixed profile and revolve constructors — 2026-11-13

The integration branch imports the coordinate-axis polygon and circular
revolution constructors from `5f42f97` and `1a4ea44` and the certified
display variants from `1801da5` and `2e16016`. The native revolve test
selection passes, including the conservative display certificates.
The rectangular annular sector retains both coaxial cylinders, two polar
caps and the exact radial faces. Bridge tests verify full and signed quarter
revolutions with through bores, closed meshes and independent volumes.
A box corner can also be cut or fused with a qualified finite cylinder
using shared analytic arcs and the true shoulder and floor.

Native semicircle, capsule and nested capsule-ring extrusions retain arc
trims and cylindrical walls. Their signed-axis tests verify analytic
volume, area and centroid, closed mesh and native JSON roundtrip.
The first browser run passes both original capsule offset cases and both
original circle offset cases. Projection and revolve edit were separate
failures in that snapshot. The next browser run includes their repairs.

## PlainCAD original helical workflows, first run — 2026-11-13

The original linked four-start lead screw and nut passes in 18.194 seconds.
It measures retained native stock, edits handedness, cold-opens the document
and exports both real STL meshes. The modeled Thread case passes external
native preview, both length checks and save/open, then fails during the
full STL intersection check. Replaying the exact WASM snapshot reproduces
a crossing between two nonadjacent triangles near the external lead-in.
Native tests independently verify the internal M3 groove and both hands of
the finite lead pair. A mesh embedding repair is in progress.

The [two-case report](evidence/plaincad-helical-first-run-2026-11-13.json)
records the one pass and one failure, both TypeScript checks and the
separate native WASM supplement.

## Phased lead screw and nut end reliefs — 2026-11-13

Native phased reliefs retain the true finite start windows, with a reduced
shaft entry and widened tube entrances at both ends. The helical bore wall
splits into independent analytic strips where the reliefs open both ends.
Both hands pass native topology, integration against independent material
coefficients, shoulder normals and closed linear-deflection mesh. The
bridge reproduces the original four-start shaft and nut volumes and bounds
and checks the shared clearance envelope. Browser angular display and the
original helical workflows are being qualified separately.

## Native modeled threads with overlapping lead-in — 2026-11-13

The stock wall, helical flanks and end faces are clipped in their analytic
charts. The lead-in shoulder uses polar trims on the exact retained flank
intersections. Its material proof subtracts the overlapping groove once.
The external thread attaches to the unchanged socket head. Native tests
verify both handednesses, internal and external M3 geometry, retained
B-rep volume and area, JSON and closed linear-deflection display.

The bridge reproduces the original external M3 blank, the 3 mm and 2 mm
thread edits, and the original internal bore within the acceptance volume
bounds. Browser angular tessellation and the original thread workflow still
require qualification. The bounded twisted chart now permits up to 32
unwrapped turns to construct the full 12 mm blank before its lead-in cut.

## Finite phased starts and grooved shaft head attachment — 2026-11-13

The four-start constructor keeps the authored angular offsets in a common
finite axial window. Both hands of the original lead screw and nut recipes
build as a single connected analytic shell. Integrated volumes agree with
independent meridian integration. Existing axial-offset constructors retain
their original contract.

The native shaft attachment preserves the grooved shaft's noncardinal seam
phase and replaces its upper disk with an analytic annular shoulder attached
to the retained head and polygon socket. Both hands pass exact volume,
closed coarse display, native JSON, cancellation and input ownership checks.
Fine helical display and intersecting end reliefs remain separate gates.

## Bounded chart coordinates for PlainCAD M3 grooves — 2026-11-13

The bounded entry point retains the actual authored groove surfaces and
permits a quantified axial and angular representation error on the stock
wall trims. The strict entry points retain their exact requirements.
The external and internal M3 profiles construct for both hands and their
retained B-rep mass matches independent radial integration. A closed
analytic-bound mesh passes the native display check at 0.1 mm deflection.
The browser angular setting still reaches the uniform tessellation budget.
The overlapping lead-in and the original browser thread cases remain open.

## Native helical groove dependencies — 2026-11-13

The integration branch imports the qualified single- and multi-start
helical constructors, finite relief families, material queries, and their
analytic bounds, display seeds and native codecs from main snapshot
`9be557f`. The live helical selection passes in geom and trimmed
(0.147 and 26.698 seconds). PlainCAD's actual thread dimensions require
bounded representation of certain axial chart endpoints and an entry
relief intersecting the groove. Both remain separate from this import.

## PlainCAD finite To Face integration — 2026-11-13

The native bridge qualifies a unique planar target against its finite boundary,
including trim holes and signed direction. Polygon sweeps support qualified
oblique targets. Circular sweeps use an exact perpendicular-target proof and
retain an analytic cylinder. Bridge tests cover coverage misses, zero depth,
oblique polygon volume, signed cardinal frames and shape ownership. The brep,
parametric and bridge suites and vet pass.

The focused acceptance passes five original browser cases in 48.624 seconds,
including the rectangle and circle To Face workflows, both face-pocket cases,
and component placement. Both TypeScript checks and the separate WASM probe
pass. The five bridge, adapter and harness source hashes match both reports.
The [focused report](evidence/plaincad-to-face-2026-11-13.json) records the
exact source snapshot and WASM hash.

A broader run on pinned PlainCAD revision
`5942ff960612ecf298ca4bf73bf6bd161622dc95` passes 30 of 31 original
cases in 306.543 seconds, with zero skips or flakes. Both TypeScript checks
and the separate WASM probe pass. The long-chain test selects the rear edge
on this run. Its radius edit makes the through pocket touch the fillet tangent
at a shared straight segment, and the existing strictly interior pocket proof
rejects it. The [consolidated report](evidence/plaincad-to-face-consolidated-2026-11-13.json)
preserves the failing run. Exact boundary splitting and sewing is in progress.

## PlainCAD grouped edge edit integration — 2026-11-13

The adapter sends individual authored edge references as one native operation.
The bridge resolves each against the retained source shape, deduplicates repeated
physical edges, and publishes a single result after the entire group succeeds.
Mixed orientations, partial or consumed references, overlapping treatments and
mixed-owner adapter inputs fail explicitly. The original extrusion direction
anchors the cap frame, so reversing a mixed-cap group preserves the location of
a later cap boss. Native tests also verify closed display meshes, independent
volume and centroid, face area and normal, unchanged inputs and handle counters.
The final bridge race checks pass in 16.153 seconds; fillet race checks pass in
21.661 seconds. Vet and an independent WASM build pass.

The baseline failed initial volume in the original multi-edge edit test because
the previous adapter rejected two references. The focused grouped build passes
both original edge edit/draft tests, both TypeScript checks and the separate
WASM probe. The probe adds grouped chamfer mass, mesh and STL, reordered and
repeated edge selections, and a failed second reference that preserves input.
The final frozen run passes all 26 selected existing acceptance cases in
172.906 seconds. The original multi-edge edit takes 9.926 seconds and the
repeated XZ drafts take 8.716 seconds. Both TypeScript checks and the separate
7.385-second WASM probe pass. All 27 recorded cases actually pass with zero
skips, failures or flakes. The original browser assertions and timeouts are
preserved, with only the provider label adapted. The 80-triangle multi-edge
STL has positive signed volume within 0.0026% of the analytic volume.

Evidence is in `docs/evidence/plaincad-grouped-edge-treatments-2026-11-13.json`
from `/tmp/plainsolid-multiedge-acceptance-03`. All 694 recorded PlainSolid source
hashes match, and the independent WASM SHA-256 matches the tested artifact
`a5056d13c374ff9cb6e664dba6e7255ed1a4467b482757db4d635c9ae139d5c1`.
The original PlainCAD checkout and the other agent's main checkout were untouched.

## Grouped analytic longitudinal edge treatments — 2026-11-13

The grouped constructor supports one independent sharp, rounded or chamfered
state at each of the four parallel corners of a rectangular section. It retains
analytic cylindrical rounds and planar bevels. Exact represented side-clearance
checks reject treatments that consume a supporting side. The original constructors
retain their interface and expose consistent corner and cap metadata.

Native tests cover all 81 corner assignments, analytic volume and area, closed
meshes, physical corner placement, invalid sizes, and retained bore, pocket and
boss composition. The full fillet suite passes in 2.352 seconds. An independent
bridge oracle verifies volume and centroid against closed-form removed-corner
moments for both XZ edit states and a grouped chamfer. Application qualification
is tracked separately.

## PlainCAD long feature chain integration — 2026-11-13

The WASM bridge resolves complete authored straight cap edges on retained stock.
Each operation publishes one owned shape after successful construction. It
replays fillet and chamfer with the disjoint through bore, rectangular through
pocket, and upper boss, preserving the actual curved and planar supports.
Cardinal frame and rectangle proofs guard tool placement. A measure-only query
qualifies Hole scope without creating an intersection handle. Native tests
verify the rotated and translated chain, exact bounds, retained volume and area,
closed mesh, unchanged prior shapes, and the suppressed-pocket state. Final race
checks pass for the bridge (16.883 seconds) and fillet package (9.291 seconds).
Vet and an independent WASM rebuild pass.

The final frozen-source browser run passes all 24 selected original cases in
161.868 seconds, including the original long chain (12.606 seconds) and adjacent
invalid-size recovery (6.586 seconds). Both TypeScript checks and the separate
WebAssembly probe (7.698 seconds) pass, with zero skips, unexpected results or
flakes. The original acceptance source differs in the staged copy only in the
provider label. The final chain's 216-triangle STL has positive volume and
0.0036% error against the analytic volume.

Evidence is in `docs/evidence/plaincad-long-feature-chain-2026-11-13.json` from
`/tmp/plainsolid-chain-acceptance-03`. All 691 recorded PlainSolid source hashes
match. The independently built WASM matches the browser artifact SHA-256
`c4c83804d6dc8e30c37a5dfd9ac915cdcfd4219b6aba8d9db8cdbd75d933b68b`.
The original PlainCAD checkout and the other agent's main checkout were untouched.

## Incremental analytic edge chain — 2026-11-13

The integration checkout imports the long-edge treatment and feature-chain
constructors from `main` (`35e9118`, `9537848`), together with the native
extrusion builders they use and the anchored circular trim codec from snapshot
`9be557f`. The original top/back chamfer constructor and its tests are retained.
The incremental constructor also supports the bottom/front chamfer selected by
PlainCAD's actual authored references, with optional through bore, rectangular
through pocket and upper boss. Native tests cover geometry at each intermediate
stage, including a suppressed pocket, the chamfer's actual plane, exact analytic
measures, native BRep reload, and closed meshes.

The compact mesher now recognizes straight surface trims and cylindrical loops
that start at an axial seam. Its related bore, union and persistence regressions
pass in 2.498 seconds. The native long-edge suite passes in 1.602 seconds. The
PlainCAD bridge and browser acceptance are a separate checkpoint.

## Mapped cylinder display for the long edge chain — 2026-11-13

The compact trimmed mesher recognizes mapped plane, cylinder and line supports
while sampling their actual placed geometry and retaining shared model edges.
A cyclic coordinate placement with translation passes mesh closure and volume
checks within a 200-triangle budget. The related bore, union and compact mesh
regressions pass.

The baseline run of PlainCAD's existing long native feature chain passed both
TypeScript checks and the backend probe, then failed at the first fillet preview
(`modeling-operations.spec.ts:559`). The adapter did not expose fillet at this
checkpoint. Evidence is in `/tmp/plainsolid-chain-acceptance-01/readiness.json`.

## Unequal-depth scoped Hole integration — 2026-11-13

The bridge selects layered construction for common-entry cutter recipes with
unequal stop depths, including blind/through mixtures. Through tools can be
reoriented exactly to the blind opening side. Signed XYZ tests verify retained
mass, entry/exit areas, individual floor areas/normals, mesh closure, transforms
and ownership. `overlapVolume` measures actual additional material removal without
allocating handles, including at full capacity. Symmetric operand queries and
no-op queries pass; the ordinary `common` operation does not invent geometry.
The adapter uses this measure to validate successive Hole features on already
bored stock while preserving indexed scope errors and atomic publication.

Final bridge race tests pass in 16.519 seconds; vet and WASM build pass. The
reviewed lower geometry is committed as `302184f`. The frozen-source application
run passes all 22 selected existing acceptance cases in 135.638 seconds, both
original TypeScript checks, and one supplemental probe in 7.852 seconds (9.329
seconds including runner startup). All 23 recorded cases actually pass, with
zero skipped, unexpected or flaky results. Evidence:
`docs/evidence/plaincad-unequal-depth-holes-2026-11-13.json`, from
`/tmp/plainsolid-depth-overlap-acceptance-02`. All 677 recorded local source hashes
match the final tree; independent WASM rebuild SHA-256 is
`9434e0ea7173091be4f41e2374a27bc49315b4d35c77f624516625e9eec658a8`,
identical to the tested artifact.

The supplement checks unequal-depth batches in reversed order, sequential actual
PlainCAD Hole features in both orders, save/reopen/rebuild, exact volume/area,
mesh and STL volume/normals, late-target rollback and retained inputs. Existing
acceptance assertions and timeouts are unchanged. No overlap-specific existing
browser case exists, and guided-hole placement restrictions remain unchanged.
Opposed-entry blind overlaps, tools beginning wholly inside original stock,
unresolved/tangent contacts or depth levels, material islands, side-crossing cuts
and arbitrary curved stock remain unsupported. Neither the original PlainCAD
checkout nor the other agent's main checkout was modified.

## Unequal-depth analytic bore construction — 2026-11-13

`trimmed.BoreCylinderDepthUnion` retains a global circular-arc arrangement across
axial layers. Exposed cylinder walls share edges across layers; differences
between successive footprints produce crescent or annular planar floors. The
history records removed volume, surface-area change, opening areas and individual
floor areas. Inputs remain bounded to 32 strictly interior parallel cutters on
convex stock with a common entry side. Zero depth denotes a through cutter.

Independent closed-form lens/counterbore checks compare retained-boundary volume,
area and centroid, not merely reported metadata or tessellated estimates. Tests
also cover three-depth chains and triple common regions, permutations, nested and
disconnected annular floors, duplicate depths, mesh closure, cancellation and
mesh budgets. Unsafe tangent contacts, unresolved depth/floor separation and
material-island footprints reject. Related bore/union/compact-mesh tests pass
in 2.100 seconds. Independent bridge-side geometry review found and resolved
area cancellation by anchoring each loop's Green integral locally. Floor nesting
rejects uncertain classifications; eighth-turn analytic subdivisions avoid a
collinear floor seed in the three-depth fixture without weakening validation.

The bridge and application checkpoint is qualified separately above. Generic
native BRep persistence of the anchored trim remains unqualified; application
feature-recipe persistence passes the separate browser check.

## Overlapping scoped Hole integration — 2026-11-13

The bridge adds bounded `cutMany` batches: validate all inputs first, construct
one retained result, require collective material removal, then publish one owned
handle. Planar batches also support redundant members. The adapter keeps every
center's original-target probe, requires each target to lose material, and holds
all outputs until the full scope succeeds. Standalone and batch cuts share the
application's material-removal tolerance, preventing rounding from making an
idempotent cutter appear to remove volume. Unsupported batches publish nothing.

Circular batches use the analytic union constructor when separated-bore handling
cannot prove the combined shape. They require common effective entry/stop planes;
through-tool reversal must preserve represented geometry. Union mass, cap area,
blind floor normals, clone/transform ownership and signed XYZ behavior pass.
Opposed-axis through tools, duplicates/containment, three-circle overlap, partial
batch failure and shifted no-op rounding regressions pass. Native bridge tests
pass (1.397 seconds), race passes (10.842 seconds), and vet/WASM build pass.
Independent geometry, bridge, adapter and probe reviews approved this domain.

There is no existing PlainCAD browser fixture specifically for overlapping Hole
cutters. Existing guided-hole staging intentionally rejects overlapping placement,
which remains unchanged. The supplemental real-WASM probe separately checks
closed-form lens volume/area, order independence, duplicates/contained tools,
actual mesh/STL volume and normals, original input preservation, late rollback,
handle capacity, and an actual overlapping-Hole document rebuilt after save/open.
The unchanged existing 22 scenarios remain the application regression selection.
The final consolidated run passes 22/22 existing cases in 133.393 seconds and
one separate supplemental probe in 8.042 seconds, with zero skipped, unexpected
or flaky results. Both original TypeScript checks pass. Evidence:
`docs/evidence/plaincad-overlapping-holes-2026-11-13.json`. All recorded local
runtime source hashes match the final tree, including the analytic union files.
Geometry/workflow assertions and original timeouts remain unchanged. The source
PlainCAD checkout and the other agent's main checkout were not modified.

## Analytic cylindrical-union construction — 2026-11-13

`trimmed.BoreCylinderUnion` retains the exposed circular arcs of up to 32 disks
and their inward cylindrical walls, with shared entry/exit rims or planar blind
floors. Inputs require convex stock, strictly interior footprints, perpendicular
parallel caps, and a common effective depth. Exact represented squared-distance
predicates classify duplicates, containment and tangency before constructing
transverse intersections. Geometry-based ordering makes the union independent
of input order. Unsupported contacts, ambiguous triple intersections and inner
footprint loops that could isolate material are rejected.

Adjacent arc supports share endpoint representatives only after agreement within
the existing BRep tolerance is checked. Model rim curves and interior p-curves
remain analytic; no mesh substitutes for the retained boundary. The new anchored
trim type is not yet supported by generic native BRep persistence. PlainCAD
persists its feature recipes, which are qualified separately.

Independent checks compare retained-boundary mass with closed-form two-circle
lens and three-circle formulas, plus a separate parametric cylinder-union
construction. Permutations, genuine triple common regions, disconnected loops,
duplicates/containment, exact tangent predicates, unresolved crescents, material
islands, mesh closure/budgets, and scaled/translated inputs pass. The final union
suite passes in 0.533 seconds; parent review and independent oracle tests pass.
Application adapter and browser qualification follow as a separate milestone.

## Scoped Hole implementation checks — 2026-11-13

The adapter's `cutScope` validates tools against original targets, requires every
selected target to lose material, returns the real indexed `HoleScopeError`, and
publishes independently owned outputs only after the whole operation succeeds.
Temporary probes and partial results are disposed on failure. The WASM preflight
checks crossed target/tool hits, missed-center and missed-target indices, preserved
inputs, repeated late rollback and the exact remaining capacity of the 256-handle
store. Overlapping cylindrical tools remain explicitly unsupported.

The bridge retains analytic cylinder walls and shared-edge planar floors for
blind pockets from either stock cap. Canonical reversal requires exact frame and
endpoint preservation. Tests cover all axes/signs, raised stock with 5 mm removed,
mixed and multiple blind/through recipes, original cap areas, floor normals,
cloning/placement, independent surface-integrated mass, closed meshes and rejected
closed cavities or ambiguous references. Proven disjoint bounds permit individual
scope misses; a new tool separated from previous bore bounds can query remaining
stock. Unproved overlap is never treated as a miss.

The PlainCAD angular setting maps conservatively to half the kernel facet-angle
bound. Tiny settings cannot silently disable checking. Actual guided two-hole
blind/through inputs satisfy the original 2% removed-volume mesh tolerance and
sampled normal-turn bound. Mesh and STL use the same helper; this does not certify
arbitrary surfaces between samples. A compact mesher for exact plane/rectangular-
cylinder domains keeps straight edges and axial strips unsubdivided while sharing
all circular boundary samples. The representative default one-bore mesh drops
from 2,048 to 144 triangles, preserving accuracy and matching display/STL vertices.
Independent tests verify linear mesh growth, exact triangle/subdivision budgets,
cancellation, unsupported-domain fallback and analytic-certificate bypass.
Selected trimmed tests, including cylinder, sphere, torus and cone poles, pass
(2.489 seconds); native bridge tests pass (0.861 seconds). The compact-specific
boundary limit delegates fine meshes to the general path;
explicit caller budgets remain enforced. The fine-deflection fallback regression
passes (0.623 seconds). Final bridge race (6.575 seconds), vet and WASM build pass.
Independent core, adapter and probe review approved the bounded implementation.

The final consolidated run passes all 22 existing PlainCAD scenarios in 151.620
seconds, plus one separate backend probe in 8.450 seconds, with zero skipped,
unexpected or flaky results. Both original TypeScript checks pass. Evidence:
`docs/evidence/plaincad-scoped-holes-2026-11-13.json`. The source-hashed snapshot
includes final compact meshing and angular mapping; geometry assertions and
original timeouts are unchanged. A post-run provenance supplement pins all 658
selected local Go source files and `go.mod`. Rebuilding those unchanged runtime
sources exactly reproduced the accepted WASM SHA-256. The runner then received
a metadata-only improvement to collect those dependency hashes in future runs;
its original run-time hash is preserved alongside the updated hash. All runtime work is isolated in the integration
worktree; the source PlainCAD and active main checkouts are untouched.

## Curved mesh angular-tolerance fix — 2026-11-13

PlainCAD's existing multi-body Hole STL check exposed coarse cylindrical facets:
centroid-only normal checks allowed eight-sided radius-3 holes at angle 0.2.
Refinement now checks vertices, edge midpoints and centroid, with explicit
singular-boundary/pole handling. Rectangular cylindrical walls use a structured
UV grid in the sampled fallback, preserving shared edge samples and avoiding
thin fan triangles. Cancellation and existing mesh budgets remain enforced.
Display and STL use the same tessellator and requested options.

Independent review and focused trimmed tests pass (1.568 seconds):
`TestSampledCylinderFallbackAngularBound`, `TestCylinderAngularCheckIncludesFacetEnds`,
`TestBoreMeshAngularSamples`, `TestBoreCylinder*`, `TestMultipleThroughBores`,
`TestPromotedSeamsAndPoles`, `TestTrimmedTorusAndConeQueries`, `TestSphereCutBox`
and `TestCertifiedWrappedConeTruePole`. Direct regressions check analytic cylinder
normals at every wall vertex, closure, and the previous fallback failure. Bridge
native tests pass, including previous transformed-bore cases and the application's
0.5% retained-volume mesh requirement. Browser qualification follows separately.

## Holed-face predicate projection fix — 2026-11-13

PlainCAD's guided Hole diameter edit exposed a valid radius-3 circular trim whose
seed triangulation contained a degenerate triangle. Arbitrary coordinate division
had changed exact collinearity before the triangulator's predicates. Projection
now uses reversible power-of-two scaling of the retained axes, keeps relative
resolution checks, and rejects exponent ranges that cannot preserve coordinates.
Area accumulation is translation-relative and winding compares signs directly.
Mesh validity checks remain unchanged.

The physical-collinearity regression, closed-prism check, scale tests from
2^-1000 through 2^1000, relative-resolution rejection and huge dropped-axis case
pass with `go test ./brep`. The bridge's exact guided radius-3 input also passes.
Focused trimmed-bore checks pass. A supplementary full `brep/trimmed` run hit
the 600-second Go timeout in `TestCombineSplineGraphNestedContours`, without a
preceding assertion failure reported. It used the pre-angular-fix snapshot and
is not a full-suite pass. Application scoped-Hole qualification is recorded
separately below when its existing scenarios complete.

## PlainCAD finite cylindrical through-cut milestone — 2026-11-13

Retained trimmed cylinder walls now support finite through-cuts on supported
convex planar stock. Exact endpoint predicates reject tools one ULP too short
and starts just inside a cap. Strict radial containment and perpendicular parallel
caps are required; blind, grazing, partial and side-crossing cuts are rejected.
Repeated separated bores rebuild from retained source recipes. Planar/cylinder
common clips the finite axial interval; general curved overlap remains unsupported.

Tests cover all axes and signs, independent curved-boundary mass integration,
current cap areas, finite/tangent/disjoint common, repeated-bore separation,
clone and supported placement ownership, closed meshes, and identical mesh/STL
sampling. Focused native tests, race tests (15.727 seconds), vet and WASM build
passed. Independent review approved the bounded core. Runner syntax and diff
whitespace checks passed. No dependency was added.

The consolidated original PlainCAD selection passes 11/11 cases in 78.999 seconds,
with zero skipped, unexpected or flaky results. Both original TypeScript checks
and the separate strengthened real-WASM probe pass (4.327 seconds). The report
`docs/evidence/plaincad-through-cuts-2026-11-13.json` preserves source hashes and
all per-case results. Only provider literals change in the staged application;
geometry and workflow assertions remain intact. Other checkouts are untouched.
Unselected application cases and full replacement qualification remain open.

## PlainCAD exact-circle bridge milestone — 2026-11-13

The bridge now retains a closed analytic cylinder for a single full circular
profile, with signed extrusion height and an explicit origin/axis. It publishes
analytic mass and conservative bounds independently of its display mesh.
Cloning, rigid placement, current cap-plane queries, meshing and STL retain the
curved B-rep. Nonrigid curved placement and curved Boolean combinations remain
explicitly unsupported at this milestone.

Focused tests verify all three axes and both height signs, mass/bounds, independent
retained-surface integration and cylinder evaluation, correct outward cap normals,
clone ownership, rigid placement, closed display topology and mesh-volume
convergence. Mesh and STL at the same deflection settings retain identical
triangle vertices after float32 export. Explicit null mesh settings are rejected;
tessellation budget exhaustion returns `LIMIT`.

Focused Go tests, race checks, vet and WASM compilation pass. The existing
PlainCAD negative-XZ circle profile-choice/preview/cancel/apply/UndoRedo scenario
also passes, along with both TypeScript checks and the real-WASM analytic-mass
probe. Its source-hashed report is
`docs/evidence/plaincad-circle-extrusion-2026-11-13.json`. Cylindrical through-cut
acceptance remains a separate next milestone. No dependency was added; work
remains isolated on `codex/plaincad-integration`.

## PlainCAD integration checkpoint — 2026-11-13

The first Git snapshot is `7b11e83` (work in progress, not full qualification).
PlainCAD integration continues on `codex/plaincad-integration` in a separate
worktree so the ongoing kernel development checkout is not edited by this task.

The initial standard-library-only JSON/WASM bridge supports planar boxes,
signed polygon extrusion with holes, planar cut/fuse/common, convex planar
hollowing, clone/transform, mass/bounds, mesh/STL and explicit handle disposal.
It independently checks planar embedding and shell nesting before publishing
material assertions; inward cavity shells do not increase the solid count.
Requests, profile vertices and live handles are bounded. Calls are synchronous;
browser cancellation requires terminating the owning worker.

Checks passed for this increment:

- `go test ./brep ./csg/poly ./csg/draft ./mesh ./exchange`.
- `go test -race ./internal/plaincadbridge ./cmd/plaincadwasm`.
- `go vet ./internal/plaincadbridge ./cmd/plaincadwasm`; focused tests, race
  checks and vet passed after the numeric parser review fix.
- `GOOS=js GOARCH=wasm go build -o /tmp/plainsolid-plaincad-reviewed.wasm ./cmd/plaincadwasm`.
- Independent review found and then rechecked strict rejection of JSON null
  numeric values (including JavaScript NaN/Infinity serialization), with no
  handle allocation or mutation on failure. No remaining bridge review findings.

Tests include both extrusion signs, holed volume, transformed bounds, independent
clone ownership, STL volume, cavity versus disconnected component counts, empty
intersection, both rectangular shell openings, malformed input and handle limits.
This is bridge evidence only; application browser results are recorded separately
in [PlainCAD readiness](PLAINCAD_READINESS.md). The full library test/race suite
and the full PlainCAD release gates were not rerun for this focused increment.

The next increment adds `planarFace`, which measures the current B-rep rather
than trusting authored metadata. It requires one finite matching face and
rejects missing or split/ambiguous supports using PlainCAD's existing plane
tolerances. Original, transformed, opposite-normal, tolerance-boundary,
coplanar-ambiguity and invalid-input cases pass; focused race, vet and WASM build
checks also pass. Independent review approved the query and the adapter after
correcting Boolean invariants, symmetric overlap thresholds, measurable shell
removal and skipped-test scoring. The query follows the actual
`OpenCascadeKernel.validatePlanarFace` contract (face-normal distance).

Real browser evidence on 2026-11-13:

- Six existing PlainCAD scenarios passed in a single selected run: XY/XZ/YZ
  signed extrusion with edit/save/STL, rectangular Shell with both cap choices
  and its full edit/undo/save/STL flow, and both stale-worker delivery cases.
- A separate Go-WASM/backend probe passed; it also verifies rejection of no-op
  and empty cuts, disconnected/contained joins, asymmetric-scale common-volume
  consistency, and negligible shell cavities. Disconnected export union remains
  allowed as in the original application.
- Both original TypeScript checks passed. Only provider string literals were
  adapted in existing fixtures; geometry/workflow assertions were retained.
- The next existing XY non-template modeling case failed at circular-profile
  extrusion preview. Circular/arc profiles are explicitly unsupported in the
  adapter; this is the next integration gap, not a passed feature.

The two full readiness reports, including source hashes, revisions/dirty states,
WASM hashes, adaptations and per-test results, are committed under `docs/evidence`.
See [PlainCAD readiness](PLAINCAD_READINESS.md) for commands and current scope.
No original PlainCAD source was modified. Production, bundle-size/performance,
the original OCCT-specific unit suite, and full application release checks are
not claimed by these targeted development-browser runs.

Local verification on 2026-11-12 used Go 1.27.4 on macOS/arm64 (Apple M4 Pro).
No third-party Go modules are present (`go list -m all` lists only `plainsolid`).
This record validates the current supported subsets, not full OCCT replacement.

## Completed checks

- `go test ./...`: passed.
- `go test -race -count=1 ./...`: passed across all packages and the example.
- `go vet ./...`: passed.
- `gofmt -l .`: no unformatted files.
- `GOOS=linux GOARCH=amd64 go build ./...`: passed.
- `GOOS=windows GOARCH=amd64 go build ./...`: passed.
- `go run ./examples/bracket -out /tmp/plainsolid-bracket.stl`: produced a
  112-triangle mesh with volume 7520. The automated example roundtrip test also
  imports the output and validates closed topology and expected volume.

Linux/Windows checks were compilation only; tests were executed on macOS. Go
1.23 compatibility and the independent OCCT build were verified in subsequent
runs below. Remote CI has not been run.

## Initial statement coverage

Measured with `go test -coverprofile=... ./...` and `go tool cover -func=...`:

| Package | Coverage |
|---|---:|
| brep | 97.1% |
| csg | 90.4% |
| document | 93.2% |
| examples/bracket | 56.7% |
| exchange | 88.1% |
| geom | 85.9% |
| mesh | 93.3% |
| query | 92.6% |
| solid | 97.7% |
| Total | 89.9% |

Coverage measures exercised statements, not numerical correctness or feature
parity. Independent reference calculations and malformed-input tests supplement
coverage; see the test files and STATUS.md.

## Focused fuzzing

Short local campaigns passed for exact-sign orientation, mesh boxes/extrusions,
planar B-rep boxes, convex box intersection, OBJ/STL/JSON/PLY and planar-BRep JSON
readers, document JSON, and extreme-scale triangle queries. The final query
campaign performed 913,036 executions over approximately five seconds with two
workers. Document fuzzing performed 25,490 executions; the final CSG campaign
performed 144,001 executions after the exact-rational fallback was introduced.
These are smoke tests, not long-duration robustness qualification. Seed cases
remain in the source tests; automatically discovered nonfailing cases live in the
Go build/fuzz cache and are not claimed as a committed fixture corpus.

## Benchmark smoke results

`go test ./... -run='^$' -bench=. -benchtime=100ms -benchmem` completed. One short
local run reported:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| SplineEval | 63.36 | 192 | 2 |
| Sphere | 135485 | 488128 | 28 |
| Closest | 49111 | 192 | 2 |
| Boolean | 476.1 | 1696 | 13 |

These are single-run observations on the stated machine, not stable performance
promises or comparisons with OCCT. Inputs are defined in the benchmark sources.

## Numerical regressions fixed during review

- Repeated domain-end knots in unclamped splines selected the wrong evaluation
  span; evaluation/derivative regressions now cover the left limit.
- Ray triangle triple products overflowed/underflowed at extreme scales; scaled
  arithmetic and explicit range errors replace silent misses/wrong distances.
- Closest-triangle Voronoi products underflowed for very small triangles; local
  normalization fixes the tested cases and rejects ill-conditioned ranges.
- Composed assembly transforms could become singular through underflow despite
  individually invertible inputs; world transforms are now checked.
- Near-dependent convex intersection planes were silently skipped; exact
  rational determinant/solve fallback and bounded plane-merging checks replace
  that behavior. Unstable reconstructed boundaries return errors.
- Nil document streams now return errors rather than panicking.

The remaining limitations in STATUS.md are still open. No full replacement or
production numerical-envelope certification follows from this record.

## Expanded implementation and differential runs

Later runs on the same host also used **Go 1.23.12**, the module's minimum Go
version, and passed the Go suite. Go 1.27.4 unit tests, race tests and vet passed
across the expanded package set. A coverage snapshot of that suite measured
84.6% across the library and both examples. These observations supersede the
initial coverage snapshot above; adding packages changed the denominator.

OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9` was built separately in
Release mode with AppleClang and the TKPrim, TKBO, TKDESTEP and TKDEIGES toolkits
and their prerequisites. Tests generate inputs at runtime. OCCT validity checks
remain enabled. The following differential cases passed:

- Box, sphere, cylinder, cone and torus area, volume and centroid.
- Sphere, parallel-cylinder and rotated convex planar union, intersection and
  difference; sheared nonconvex planar union/intersection/difference; aligned
  coplanar union/difference; orthogonal through-hole; bored sphere; orthogonal,
  oblique and multiple cylinder subtractions through a box; a convex prism
  fillet checked against an independently authored circular-arc extrusion.
- PlainSolid faceted and planar advanced STEP export of a box and holed prism
  imported by OCCT; OCCT advanced box imported by PlainSolid and re-exported.
- STEP standalone rational quarter-circle in both directions; quarter-cylinder
  export accepted by OCCT, and the spline support of the OCCT-written face
  decoded independently from its topology.
- Bidirectional STEP full sphere, ring torus, capped cylinder, cone and frustum.
- Bidirectional stepped shaft with two cylindrical walls and one annular
  shoulder. OCCT builds the reference by fusing two cylinders.
- OCCT hemisphere read into exact spherical cap and disk geometry. OCCT
  reads PlainSolid exports for the hemisphere and both smaller and larger caps.
- Bidirectional spherical zones with two latitude disks, including an
  independently clipped sphere and a rotated asymmetric zone.
- Selected planar draft against independent OCCT halfspace intersections: one
  and four drafted sides, an oblique neutral plane, and pyramid collapse.
- Bidirectional STEP cylinder with one oblique cap; the native fixture also
  checks closed-form area, volume and centroid. The external STEP check uses
  the documented OCCT tolerance for reconstructing omitted p-curves.
- Bidirectional bounded STEP parabola/hyperbola arcs, planar body and face
  colors, and conversion-based inch/foot units.
- Bidirectional planar STEP cavity shells and concentric hollow-cylinder
  bodies with annular caps. The local test corpus also checks a spherical
  box pocket against OCCT.
- OCCT XCAF assembly with repeated translated and rotated instances decoded
  into the STEP product graph and a document with one shared asset; OCCT
  read a PlainSolid-written nested assembly and verified placement and mass.
- Bidirectional IGES line, circular arc, rational curve and rational
  quarter-cylinder. OCCT's arc fixture uses type-124 affine placement.
- IGES type 142 matched model/parameter spline curves and type 144 planar
  polygon trims with a hole, checked in both directions by OCCT.

Modeling comparisons use relative area and volume tolerance `3e-7` and absolute
centroid tolerance `3e-7` in model units. The exchange curve and surface
checks use the tolerances stated in their individual tests. See
[the reference runner](../tools/occtref/README.md) for commands and build inputs.

Independent analytic tests additionally cover direct inertia, rounded box
area/volume, drafted holed prisms, ellipsoid transforms, curved trim seam
sharing, circular sweeps, elliptical lofts/extrusions, feature regeneration,
camera projection/picking and hidden-line clipping. Exact surface–plane
contour tests include closed conics and distinct near roots that must not be
stitched together. Exact planar distance and point-location fuzzing covered
52,440 and 348,442 generated cases, respectively. Fuzz campaigns covered the
new exchange readers, exact predicates, holed topology, sewing, collision,
convex Boolean and split subsets. None of these checks closes the unimplemented
acceptance requirements in STATUS.md.

Additional analytic regressions check nested spline-plane rings (annular face
and closed-form volume), two independently curved spline faces sharing a
projected chart (closed-form mass and centroid after persistence), native
spline-pair trim reconstruction and rejection of a recipe without contact,
and certified spline distance bounds for parallel and curved surfaces.
Mixed-axis tube tests distinguish exact endpoint identity from overlapping
enclosures and close an analytic circle across four axis changes. Separate
coverage checks detect missing tube regions and coincident input. Open
two-graph tests check mass, centroid and closed meshes on both sides of
the cut, preserving original curved faces and side walls through JSON. The
accelerated planar distance search reproduces the former exhaustive witness
and tie ordering; its differential fuzz run exercised 49,501 cases.

Additional pinned OCCT comparisons cover unequal two-distance and simultaneous
edge chamfers, planar protrusions and blind pockets (including concave oblique
sketches), torus elbows, and multi-step cylindrical shafts. The latter include
collars and grooves, with import of an OCCT cut groove whose support origins
were shifted along the common axis. Spline-graph Boolean tests use independent
analytic mass and centroid calculations for regular open and closed sections.
The local coverage checkpoint reached 83.1% of statements across Go packages
before the latest query and Boolean additions.

The shared-base spline Boolean has live OCCT comparisons for union,
intersection and difference against a separately authored curved prism and box.
Additional analytic fixtures cover mixed open/closed sections, nested holes and
islands, a reversed/sheared chart, knot insertion and parameter permutations.
The mixed subtraction's seed mesh is closed; its tight adaptive deflection
request can still exhaust the default budget. Certified implicit section uses
agree continuously after JSON round trips.

Latest exchange fixtures include hollow conical frusta, mixed axial profiles,
cylindrical and conical sectors, and product-local face styles in nested rigid
STEP assemblies. OCCT/XCAF verifies style overrides and repeated placements.


Standalone rectangular STEP spline faces pass live import/export of an
OCCT rational quarter-cylinder face. Exact sewn spline rectangles accept
matching edges represented with different degree, knots and orientation,
and reject distinct curves sharing endpoints. Open and closed mixed-axis
tube face splits preserve their shared edge graph through native reload.

The tube replacement regression preserves both exact support recipes and
checks its seed mesh and enclosed volume against an analytic curved pocket.
Exact rational trim subdivision resolves a winding query inside the original
curve hull, and reports an unresolved query under a deliberately small budget.
Spline chart identity distinguishes a degree-elevated transposed/reversed chart
from a one-ULP change to its surface. Shell embedding rejects a crossing face
and an inward copy of a certified warped prism. Nested, separated and full-face
joined spline shell Booleans pass 15 independent OCCT comparisons for volume,
area, centroid and shell count, including inset and partial-overlap interfaces.
Independently authored OCCT holed-face prisms qualify protrusions, blind and
through-pockets. Exact planar shell checks preserve nested void and island
ancestry and reject crossing or touching shells.

Bounded STEP faces and selected closed rectangular spline shells pass live OCCT
import and export. The fixtures include an interior trimmed rational patch and
a six-face warped prism. Import verifies shared vertices, edges and per-face
parameter curves; the closed-shell API requires a global embedding proof.
OCCT also verifies document export of multiple placements in inch units and
bidirectional IGES exchange of a trimmed rational patch. Different rational
spline bases pass analytic graph Boolean volume checks. Certified subinterval
enclosures distinguish winding across a transverse spline-pair trim.
Rigid IGES placement and a single interior rectangular hole on a rational
spline support have independent live exchange fixtures. Mixed-degree graph
fixtures also exercise nonunit parameter domains, a sheared affine XY chart,
and transposed/reversed operands.

A box and an independently parameterized paraboloid prism exercise the
complete single-contour transverse path. Exact weak coordinate extrema
account for corner-only contact candidates; verified mixed-axis junctions
and tangent component signs exclude duplicate interior segments. The
intersection is sewn from both original supports and its closed mesh is
checked against the analytic cap volume. The optional transverse OCCT suite
checks union, intersection and subtraction against independently authored
OCCT shapes. It compares closed-mesh volume, area and centroid with explicit
tessellation tolerances; enable with `PLAINSOLID_OCCT_TRANSVERSE=1`.
The two-contour fixture intersects a box with a spline roof with two peaks.
It verifies two disconnected components after intersection, a handle after
union and two cut wires after subtraction. In all three cases the retained
fragments map back to the original supports.
Exact tube endpoint tests verify both ends of an open branch against their
original model edges, including reversed parameter direction, and propagate
both shared edge splits. The same straight-branch proof exports a lossless
STEP spline curve whose length is independently verified by OCCT. Separate
STEP and IGES fixtures exercise a rational rectangular face with a hole.
The edge-crossing Boolean fixture uses six face pairs and twelve certified
segments. Independent OCCT boxes match volume, area, centroid and topology
for all three operations. A polynomial warp preserves the analytic volumes
while curving the top and bottom faces; native reload verifies the stored
exact endpoint witnesses. Separate OCCT XCAF fixtures validate repeated
certified spline-shell assets and rigid local placements through STEP and
native documents.
Automatic affine-chart discovery reproduces the open-wire result without
caller-proposed tubes. A separate plane-arc fallback discovers a closed
circle from exact trace-only knot cuts and revalidates against the original
supports. XCAF qualifies product-local spline-shell colors in repeated
occurrences, and independent IGES fixtures exercise multiple rectangular
holes with strict containment and separation checks.

The two-open-wire regression uses an independently authored split-chart unit
cube and a square through-bar. All three operations pass through explicit
sections, reversed section order, and automatic affine discovery. Analytic
volume/area expectations are respectively 1.25/8, .25/2.5, and .75/7.5, with
centroid (.5,.5,.5). The subtraction has Euler characteristic zero. Tests require
24 distinct generated section edges, complete disjoint source-face ancestry,
and complete disjoint original-edge/generated-edge ancestry; retained original
edge fragments follow their source parameter order. Independent OCCT primitive
Booleans confirm the same mass targets. The initial three-operation fixture
passed the race detector in 77.466 seconds; the subsequently expanded nine-route
provenance test passed normally in 32.042 seconds.

The repeated-face-pair fixture uses a unit cube and an independent quadratic-roof
prism: -.5 <= x,y <= 1.5 and -45/16 <= z <= (x-.5)^2-1/16. Analytic tube proposals
produce eight open branches across five ordered face pairs, including two
branches in each of three pairs. Both resulting wires lie entirely inside the
roof chart and cross original cube edges. Certified assembly checks two common
shells (Euler four), a union handle (Euler zero), and one cut shell (Euler two).
Normal tests additionally require a closed sampled mesh with deflection .01,
at most 64 subdivisions and 250000 triangles, and absolute volume discrepancy
at most .005 against union 319/24, common 1/24, and cut 23/24. Short tests omit
this sampled check. These mesh comparisons are not exact curved integration.
Independent OCCT-authored quadratic surfaces confirm the analytic volume,
area and centroid integrals; common centroid Z is 19/320. Full native trim
quadrature is separately gated by `PLAINSOLID_SPLINE_REPEAT_MASS=1`; that gate subsequently passed after integration caching and exact zero-strip
elision were added: 24.84 seconds for all three operations, including bounded
mesh checks. Native numerical quadrature agrees with the analytic volume and
XY centroid targets within 1e-8; its convergence estimates are not rigorous
error bounds.

A subsequent Go 1.23.12 `go test -short ./...` snapshot passed with the two-wire
and repeated-pair regressions present (trimmed 173.075 seconds; geom 10.754
seconds). The freshly rebuilt optional OCCT suite with STEP enabled passed in
11.421 seconds, including both new reference modes; only the explicitly gated
expensive transverse comparison was skipped. The later exact unification
history addition passed separately on Go 1.23.12 and under the race detector
(2.542 seconds), with package vet clean. Its tests cover deleted seams,
nonbinary exact edge parameters, removed subdivision locations, deterministic
reports, hole preservation, cancellation, and no partial results on budget
failure.

The mixed transverse fixture uses a box cutting one whole and one half
paraboloid lobe. Automatic plane tracing, exact iso-curve clipping, and an
exact affine chart relation at irrational tube junctions certify the complete
contact graph. All three Go operations pass topology and bounded mesh-volume
checks. Independent OCCT shapes confirm union volume 1.14943382037 and Euler
zero, intersection volume 0.001404400497 and two shells, and subtraction
volume 0.71734559155 and Euler two. The overlap is exactly 3πr⁴/4 for r=5/32;
the oracle also checks its analytic area and centroid. This does not qualify
automatic discovery of arbitrary nonlinear surface pairs.

Analytic sewing tests retain canonical periodic seams, closed circular model
edges and collapsed poles for sphere, torus, cylinder and cone. These include
affine placements, continuous model/UV checks, analytic mass comparisons,
watertight tessellation, and rejection of shifted evaluators and wrong seams.
Exact edge contact tests recover a shared model point and split both original
shell edges, and distinguish a rational but unrepresentable split parameter.

Independent OCCT exchange fixtures validate concave orthogonal rational spline
faces with multiple holes in STEP and IGES and a connected open STEP spline
shell with an exact shared boundary. Malformed fixtures exercise notch,
touching/nested holes, mismatched shared edge interiors, and inconsistent
source weight ratios. Go 1.23, race, vet and focused parser fuzz smoke passed.

The affine spline fallback and section fixture exercise coincident and
simultaneous edge events, preserve original spline supports and rational edge
ancestry, and reject nonrepresentable conversion. The output can be supplied
to another affine Boolean. The independent mixed boundary-partition tests
verify exact paired model/UV curves and volume preservation before selection.
The sheared unit box section retains exact corner coordinates and source IDs.

Trimmed spline distance tests check a through-hole, a circular trim, a closest
point on a nonplanar face, affine placement, cancellation, and resource limits.
The distance result preserves certified bounds on interruption. Scalar tests
cover rational underflow, preventing a positive upper bound from rounding
inward to zero. Scattered spline fitting is tested on irregular parameter
samples with fixed nonseparable rational weights and singular input rejection.

Wrapped surface fixtures prove both seams of a rational toroidal support,
verify a watertight mesh and independent volume integral, and preserve seam
topology through exact edge splitting and native reload. Reload rejects a
modified seam recipe. The selective fillet fixture compares OCCT's native
unequal-radius edge fillets with Go mass and area to 1e-8 and centroid to 1e-9.
Document tests preserve selected radii through save, edit and undo.

Exact affine-parameter tube certificates now remove only proven zero-width
integration strips whose exact endpoints equal the original face chord.
Proofs are cached per segment within one mass call and bounded independently;
failed proofs preserve quadrature. A call-local bounded trim sample cache also
reduces repeated implicit-root evaluations on genuine curved strips. The
48-strip planar fixture improved from 2.743 seconds to 8.82 milliseconds per
mass call without changing volume, area, centroid, or topology. Regression
checks include a one-ULP mismatched chord (not elided), paired proof reuse,
cache saturation, and rotated/scaled placement.

Multispan rational UV fixtures retain an exact internal join at 7/3, prove
clipped and reversed path topology, and reject an approximate declared
endpoint and a path that intersects an existing curved hole. Nested and
disjoint face cuts keep the original hole edge IDs and the solid volume.
An oblique affine spline solid split and rejoin preserve exact support
identity, cap coordinates, retained edge intervals, topology and analytic
centroids. The exact splitter rejects a generated coordinate of 1/3.

Wrapped spline traces close across irrational seam roots and reconcile
strictly transverse corners using exact tangent signs. The canonical rational
toroid's quotient embedding, outward orientation and complete seam edge
agreement are proved independently of its mesh. STEP solid exchange requires
that proof and explicit source closure; the pinned OCCT oracle verifies both
directions and compares adaptive area and volume. Toroid point queries cover
axial meridians for the general certified rational profile and all represented
3D points for the exact circular-profile subset.

Certified full-patch meshes preserve wrapped seam indices. Independent
bilinear fixtures verify bounded interior error, zero error for exact affine
vertices, and quadratic refinement of two-direction correspondence bounds.
Explicit spline and implicit paired tube trims have independent exact
one-direction chord bounds, including rounded model vertices. Nonlinear
edge contact tests include independent reversed parameter ranges, exact
overlap and an irrational root with separate enclosing brackets.

Canonical capped cylinders and frusta match analytic circular volume, area
and centroid, and native cylinder reload preserves continuous cap/rim
agreement. Exact finite-line fixtures include noncircular profiles, a
nonbinary cap parameter, rim deduplication, coincident generators and quartic
torus tangencies. Cylinder Booleans check separated height intervals,
through and blind bores, enclosed cavities and original support ancestry.
Nested-component tests recover an island within a cavity, reject incorrect
cavity orientation and verify volume. The circular torus offset fixture
proves the displacement is normal to both partials with exactly the required
length. Wrapped STEP fixtures compare independently authored cylinder,
frustum, tube and blind-bore material against pinned OCCT. Wrapped feature
recipes retain stable cap roles across save, edit, regeneration and undo.

Cubic trim fixtures prove a projection that needs exact subdivision, detect
a crossing, and leave a folded projection unresolved. Graph fixtures
certify an isolated quartic contact and a strict height gap of 1/1024;
the graph Boolean dispatcher retains the appropriate supports even when
ordinary section isolation is limited to a single cell.

Selected algebraic face inverse tests locate rational world points with
nonbinary and irrational original UV, distinguish trim interiors and holes,
and recognize exact isoparametric contacts. A finite axis-directed line
fixture retains its original signed parameter and groups coincident rational
face candidates at a shared edge. Same-domain curved spline healing accepts
independently refined knot bases, restores rectangular embedding eligibility,
and can undo an exact wrapped-seam subdivision after native reload.

The two-sided mesher qualifies polynomial and rational biquadratic cells,
including nonseparable weight fields. Knot-aligned and adaptive grids retain
every original knot and weld only certified wrap seams. A local refinement
fixture reduces 64 uniform triangles to 40 while preserving its error bound.
The wrapped station loft retains two exact uses of each intermediate rim
edge and matches independently integrated analytic area, volume and centroid.
Native reload and reordered faces independently recover its stations and
original IDs; inward orientation is rejected. Exact axis-line queries
discard interior cap hits, and an oblique circle chord at a station reports
two algebraic contacts with both wall IDs. Scene tests cover nearest exact
cap ordering, ties, the toroid hole and a coplanar loft cap entry.
The wrapped-bore example round-trips exact STEP and native material geometry.

New pinned OCCT differentials cover stepped wrapped Booleans, blind bores,
wrapped cavities and shoulders, and doubly wrapped hollow toroids. The
dedicated wrapped cavity fuzz target passed a 30-second run with 124
executions; this is only a bounded smoke check.

A two-band wrapped station loft exchanges with pinned OCCT in both
directions and agrees with independently calculated area, volume and
centroid. STEP rejects a disconnected or incorrectly owned interior rim.
The conforming straight-trim shell mesher was tested against a warped
prism and a rational reparameterization of the same material, including
face ancestry, shared sample counts and convergence to volume 5. A
simultaneous edge incidence fixture independently identifies the two
entering face pairs where a warped bilinear roof meets a planar box edge.

Whole-shell simultaneous-edge fixtures prove the intersection volume
1/768, the union and difference volume identities, and native persistence.
The curved variant uses a determinant-one vertical quadratic warp, tests
all three operations, and tests automatic proposals from an independently
recovered warp coefficient. Pinned OCCT agrees on volume, area and
centroid for all three operations. Original-surface proof is repeated, and an omitted
simultaneous terminal is rejected. Selected curved adjacent supports use
an exact separating quadric; shell orientation can use exact signed
volume bounds after pairwise embedding is established.

The adaptive polygon-shell fixture uses 336 triangles versus 768 uniformly
with the same certified deflection target. A curved planar annulus
fixture proves exact chord polygon triangle coverage and a two-sided
filled-region distance, including a point in the gap between the original
hole and its chord polygon. Model rounding is bounded separately.

Mixed wrapped STEP is qualified with a live twelve-family area and volume
comparison. The mixed product adapter independently verifies repeated
XCAF product identity, occurrence placement and adaptive volume with
OCCT, including a rotated shared rim fillet asset. The IGES filled-sheet
fixture verifies independently charted disks matching multiple annulus
holes by reversed whole model curves. Pinned OCCT reads all three faces
and matches their combined area. Algebraic edge and raw-plane event tests retain exact brackets in
both original ranges, including independently projected events less than
2^-80 apart.

Coaxial circular loft union and intersection split at an exact radial
crossing; analytic volume and centroid, shared rim radius and retained
source walls are checked. Strict radial subtraction produces a closed
annular shell with two cap holes and independently checked mass.
Native reload independently revalidates that annular shell and tests
material point classification, exact cap and rim line hits, and picking
through the bore. The circular-frustum offset test independently proves
the same squared displacement and orthogonality to both chart tangents
at nonbinary parameters, plus the capped result's analytic volume.
The loft and offset feature recipes preserve checked face roles through
document persistence, regeneration and undo. Exact document picking selects
the nearest of two occurrences of one asset and rejects unrepresentable
parent composition or local ray translation.

Local Linux amd64 and Windows amd64 cross-builds with CGO disabled passed.
The native trimmed JSON fuzzer passed a 20-second smoke run with 182,873
executions. Neither check replaces executing the full platform CI matrix.

Quadratic shared-base saddle union and intersection preserve four exact
top sectors and one shared saddle vertex. Independent analytic volumes
qualify both axis rays and diagonal rays to the four chart corners. Native
reload rechecks all model/trim edges, including graph side walls. An
explicit pinch test rejects subtraction with touching material components.
The independent OCCT saddle fixture agrees on union and intersection
volume, area and centroid; an off-center saddle is recovered exactly.

A knot-aligned partial-coincidence fixture checks all three graph Booleans,
including doubled input ancestry, exact shared coedges and reversed empty
subtraction. A connected coincident region becomes one retained face with
a map from each original cell. Pinned OCCT agrees on union and intersection
area, volume and centroid. The raw OCCT cut agrees on volume and centroid
but reports additional area (3.60554013101 versus 2.42205294213);
an independently authored OCCT boundary for the regularized difference
agrees with the smaller area. Enclosed island and annular difference fixtures independently
check analytic volumes 1/16 and 7/16 and reject pinched material. Exact
knot clipping subdivides spline face integration seeds at derivative creases.
The non-knot axis-line fixture checks all three operations at exact U=1/4
and returns a proof at U=1/3 without rounding that native topology endpoint.
A higher-degree fixture also passes exact factor division and a strictly
signed quotient test.

An independent pinned OCCT rolling fillet agrees with the selected circular
cylinder rim blends on area, volume and centroid. Line fixtures compare
analytic blend roots, algebraic cap chord endpoints, tangency multiplicity
and exact nonbinary generator parameters; they verify source-face ownership
and scene picking. The nearest-boundary query encloses analytic distances
and returns an exact algebraic witness on its original face. Annular cap
queries distinguish disconnected chord intervals, inner tangencies and
isolated outer tangencies, even for nearly coincident radii. Complete circular-cylinder offset tests compare dilation
volume with Steiner's formula, erosion with exact dimensions, and the
offset of an existing blend with exact nonbinary surface evaluations.

A six-face curved shell stitches 12 seams and eight explicit corners. Its
mesh preserves analytic volume 1 and passes an independent exact BVH display
embedding check. A mutated crossing is rejected with source face ancestry;
limits and cancellation cannot certify an incomplete traversal. A separate
seam fixture matches exact degree-one and degree-two rational trims with
distinct reversed active ranges, and rejects a one-ULP perturbation.

Parallel contact-line and rectilinear contact-grid Boolean fixtures verify
analytic mass, disjoint difference shells and rejection of crossing
pinches. Repeated even contact at U=1/3 selects the ordered envelope and
rejects positive pinched subtraction. Exact oblique sections compare
original supports across different knot partitions, retain a nonbinary
terminal, and match the independent enclosed-volume integral. Exact
vertical queries retain both top owners at a coincident height.

Two certified curved annuli cross an off-center C0 knot, on polynomial
and rational supports. Exact triangle-to-knot clipping and a piecewise
Jacobian bound establish the requested distance; exhaustion of cumulative
cell and piece limits returns no certificate. Mixed STEP cap and blend
face colors are independently verified in XCAF and through native document
round-trip. The multi-band station bore passes independent OCCT STEP
import/export with analytic area and volume checks, and native recovery
retains independently partitioned inner and outer walls.

Parallel and crossing oblique line Booleans pass analytic volume checks,
including disconnected subtraction and rejection of a crossing pinch.
Axis-aligned and rotated ellipse fixtures check exact four-arc topology,
closed lenses and annular differences, analytic volumes and native
reload with full edge revalidation. The pinned OCCT ellipse Booleans agree
on volume and centroid. OCCT's trimmed-area result changed with quadrature
settings; independent integration of the graph metric agrees with
PlainSolid within 1e-7 for all three operations. Exact rational conic arcs
retain nonrepresentable controls without rounding them into native topology.

Multi-span seam fixtures match separate original parameter ranges and knot
partitions, preserve nonbinary cuts 2/5, 2/3 and 6/7, and recertify the mesh
after assigning shared display vertices. The three-face collection checks
declaration-order invariance, opposed seam incidence, vertex fans and exact
absence of undeclared display contacts. Curved UV rational-cut subdivision
also preserves exact mandatory samples before any mesh construction.

The full Go 1.23 short suite and Linux/Windows amd64 CGO-disabled builds
passed at a checkpoint in this implementation. A full race attempt exceeded
Go's default ten-minute package timeout on the large trimmed suite; the CI
race limit is now explicit at 35 minutes. Recent focused race runs pass for
the exact contact, offset, exchange and meshing work.

The rational graph-line fixture proves that two independent top equations
share an irrational root, then merges their original face ownership.
Rounded-cylinder section tests preserve an irrational radius as a rational
radical and verify both tangency owners at a blend seam. Horizontal splits
of station lofts, bores, and the straight portion of rounded cylinders
preserve the expected source faces and analytic piece volumes. The
annular-frustum STEP fixture passes both directions with pinned OCCT,
including a shared colored product through native document persistence.

Native version 4 reload proves loop topology and the complete shared
model edge for exact rational quadratic trims. A roundtrip retains the
nonbinary weight 1/3 and rejects a sub-tolerance control edit. Exact
restriction and inverse restriction recover all homogeneous coefficients
after split and merge. The represented-only edit rejects nonbinary vertices;
the native version 5 edit stores the rational model point
`(19/26, 57/104, 24)`, tests affine placement and inverse merge, and rejects
an edited display vertex one ULP away. Four nonbinary conic Boolean cases
agree with independent analytic volume and retain the conic junction
`(5/12, 2/3, 2)` on reload. Edge comparison independently proves equality
across an irrational knot event.
A separate native-control annulus mesh verifies exact UV triangle coverage,
the hole topology, and an independent oblique-chart area integral.

Graph line checks also cover roots separated by less than 2^-43,
coincident envelope intervals with changing owners, and side-wall or
interior material spans across a tangency. Exact root order and height
sign are established before classifying the intervals.

The nonplanar STEP oval fixtures use independently authored polynomial
and genuinely rational biquadratic supports in pinned OCCT. Exact
homogeneous UV composition checks complete quartic model curves, with
separate native and both-direction OCCT roundtrips. Independent OCCT fixtures
also cover a curved oval annulus and two separately charted faces sharing
four exact seams. Exact graph scene
picking tests translated occurrences, a shared top owner and wall intervals.
A station-bore feature roundtrip preserves inner/outer wall roles and caps
after native reload, parameter edit and undo.

The algebraic knot enclosure fixture preserves exact continuous identity
across different knots and verifies a common represented point with a
model-space error bound that includes rounding. Plane and finite-line
queries independently identify the shared irrational parameter and both
cell owners. Trimmed-face line tests exclude a contact inside a hole and
retain ambiguous boundary evidence. A separate closed six-face warped box
with native exact curved seams verifies Euler characteristic 2, positive
display volume close to the analytic source volume 1, and no undeclared
display contacts in both the exact BVH and an independent triangle sweep.
These are boundary and display proofs; continuous smooth-shell embedding
requires its own evidence.

Algebraic subdivision persists both coedge restrictions of an irrational
split and rejects a nearby but different source-plane recipe. Two cuts less
than `2^-100` apart survive save and reload as distinct topological vertices;
the polyline reports the collapsed display segment without welding them.
Curved edge/face isolation independently verifies two regular roots, excludes
one outside an irrational cut, retains a singular tangent as unresolved and
excludes a root in a target trim hole. Two differently shaped spline shells
yield three boundary crossings in each direction. Both terminals of the
nonplanar tube fixture also pass independent edge/face recovery and the
original tube proof.

The holed certified mesh fixture uses a ten-face annular prism with 24
shared seams and 48 exact model-edge uses. Its display is a closed genus-one
manifold with no undeclared contacts and signed volume 0.4114837646484375
against independent source volume 5/12. A separate native version 5 lens
mesh proves the shared nonbinary corner against each original owner.
Pinned OCCT qualifies the closed spline lens and the document roundtrip,
including a shared local lens with translated and 90-degree rotated
occurrences. The local analytic mass comparison records OCCT integration
residuals separately from exact STEP geometry proofs.

The affine partition display test independently checks polygon and triangle
coverage, positive projected fragment areas and opposite shared cut use.
The nonlinear partition test instead composes the cut on the original spline
with exact algebraic UV endpoint images; its certified display reuses both
original seam vertices. Partitioning both faces preserves Euler characteristic
and all original ancestry across reload. Exact common-projection edge queries
prove the upper and lower cuts meet only at their two algebraic terminals,
and distinguish full coincidence with a reloaded copy.

Selected algebraic fragments can be sewn from independent native inputs.
The fixture preserves Euler characteristic 2, checks all vertex links and
rejects an omitted edge pair, reversed recipe, and endpoint-only matches.
The capped lens split proves two oriented material halves and their exact
sections; rational volume intervals enclose independent analytic cap segment
integrals. Line tests retain original parameter units, the generated cap,
coincidence on the lower face, a shared conic rim and an isolated tangent.
The affine fill certifies both directed distances to its exact source region.
The curved fill retains common cut samples, exact fragment ancestry and
two-sided distance bounds; a genuine C0 knot crease is independently tested.
Autonomous tube terminal tests isolate two irrational contacts and verify
all four chart coordinates against the original boundary and tube.
The independently authored partitioned STEP lens uses six authoritative
shared pieces at rational lower-chart knot crossings and preserves both
original supports. A 17-significant-digit OCCT writer retains its represented
pole coefficients; shorter rounded source output is deliberately rejected.
The shared partitioned lens fixture also passes independent OCCT validation
with translated and signed-axis rotated occurrences of one product. Native
reload retains one asset and all six original seam records; stale lineage
and inconsistent placement are rejected.

The exact tube trim fixture verifies two different face limits, reversed
original spline knot units, hole exclusion, and same-point ownership in both
charts. Its independently constructed tangent uses
`u²+v²=1/2` and `41u+113v=85`: complete trimmed face-pair coverage retains
only the tangent at `(41/170,113/170)`. A second pair retains exactly one open
interval and a patch that excludes part of a face trim is rejected.

The closed split-lens mesh fixture verifies one shared sample network for the
rim and both cap owners. Independent triangle checks see Euler characteristic
2 and no undeclared contacts on either half. Represented volumes are
approximately 0.00323363018 and 0.00091374464; separate analytic source
volumes are 0.00339162902 and 0.00094712385. The source-image distance
certificate does not imply a volume error bound.

A closed regular spline-pair section splits both chart disks into an inner
disk and an outer fragment with the opposite cut use. The analytic circle
fixture checks the original UV winding and height signs, assigns disjoint
holes on opposite sides, and reconstructs the trimmed topology from source
geometry after native reload. An open section fixture also assigns two
original holes to opposite fragments and partitions an original coedge
twice without conflating its parameters.

The closed tube lens retains both curved supports. Its analytic volume
`π(5/16)^4/2` is enclosed by rational bounds at two refinement levels.
The volume and exact centroid `(1/2,1/2,-11/384)` are independently
enclosed; scaling the common model XY projection by determinant six scales
the volume interval accordingly while preserving the original chart units.
Vertical queries check interior, rim and exterior cases. Exact finite line
tests include irrational endpoint roots, an isolated tangent, a coincident
top interval and graph roots outside the material.

The native trimmed-solid conversion revalidates every paired edge after
save and reload. The bounded shared-base subtraction fixture proves both
inputs strictly above the common base, reconstructs the selected material
on load and rejects a changed minuend that would require side-wall assembly.
Disjoint loop fixtures recover both components and their rim ownership;
an independent concentric fixture verifies the containment and annulus
incidence logic on two separately certified original circular loops.

Independent OCCT IGES tests verify curved oval disk and annulus area and
an annulus/fill sheet with distinct original charts and one reversed seam.
Source and regenerated surfaces and complete homogeneous curve agreement
are checked independently from the reader's area residual.
The explicit IGES open shell independently qualifies one shell, two faces,
eight edges and eight vertices, with all four shared edges used oppositely.

The native tube graph envelope fixture checks union and intersection against
independent volume integrals, preserves two original top faces, the common
base and four domain walls, and verifies every shared section use.
Reverse subtraction retains an exterior shell of Euler characteristic zero
and four side walls. Its volume agrees with the independent integral.
The tube lens inertia bounds include the analytic axial inertia and XY product.
Its boundary-distance search retains exact lower and upper bounds when its
cell budget cannot close the requested gap.

The exact nested quartic graph fixture automatically recovers two
rational-radius circles and verifies original-support coverage with 116
tubes. It assembles a genus-one annulus and separate inner disk, certifies
the annular volume against its analytic integral and reconstructs both
regions within the existing 128-cell native recipe cap.
The annular footprint reuses 128 exact-source boundary samples, has 128
triangles and encloses the source fill within a certified distance of 0.04.
Its represented area 0.2302451043 is compared separately with the analytic
annular area 0.2300971182. The nested native envelope has eight faces and
Euler characteristic two.
The same annulus produces a closed genus-one display with 6708 vertices
and 13416 triangles. Exact represented triangle checks found no undeclared
contacts. Its signed display volume is 0.0004485565 against analytic source
volume 0.0004628764. Independent quartic ellipse tests reconstruct a
diagonal metric with Y coefficient four and verify both loops with 112
tubes, checking coverage and topology separately from factor recovery.

Two independently sewn warped spline boxes produce a six-branch section
wire across their original shell edges. The native shell section recipe
reproves coverage and exact vertex ownership, and rejects reassignment of
an original face pair. A separate closed branch qualifies the scanner on
two whole shells without relying on edge crossings.

The closed IGES lens qualifies independent source and regenerated files as
one solid with two faces, four shared edges and four vertices. Adaptive OCCT
area and volume comparisons remain separate from exact material and
homogeneous edge-composition proofs.
The raw-weight document fixture additionally retains all 12 original curve
bases through native reload and checks the regenerated file with OCCT.
Modified poles, weights, owner IDs or native material are rejected even when
the enclosing recipe hash is recomputed.

Independent projected trace tests cover closed loops in XZ and YZ with
scaled, swapped original charts. The XZ material test automatically
discovers the section, reconstructs it from native geometry without
discovery hints, verifies the physical Y orientation of the two faces, and
bounds the exact original XYZ centroid `(1/2,-11/384,1/2)`.
The same test locates two irrational lower-face line contacts, verifies
both original XYZ plane-section curves at their shared rim, and proves a
zero boundary-distance witness on the original upper support. Closed mesh
race tests for XZ, YZ and ZX each check 1696 triangles, exact shared seam
indices, positive physical orientation and no undeclared triangle contacts.

The automatic dyadic guidance fixture closes the nonfactorized quartic
`x²+y²+x²y²−409/4096` on two original graphs with a common bilinear height.
Neither radial nor concentric ellipse recognition accepts that numerator.
The independent fresh cover and topology proofs pass without caller cuts.
The isolated coupled quartic `9*(x⁴+y⁴−x²y²)` qualifies shared-base union
and intersection at volumes `173/80` and `2`, and distinguishes an empty
difference from a pinched difference.

The four-branch shell wire crosses two seams at physical `Y=√(1/2)`.
The original model roots match between adjacent chart branches; native
section reload and subsequent seam subdivision retain both coedges and
Euler characteristic two. The warped fixture independently rebinds its
curved graph seam constraints and rejects malformed projection axes.

Independent OCCT qualification of six- and seven-edge IGES lens records
preserves two faces and the full split topology, along with original raw
model/owner curve bases and both owners' knot-cell ancestry. Irrational
partition cuts outside the bounded exchange format are rejected.

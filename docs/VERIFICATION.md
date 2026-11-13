# Verification record

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

On clean `5a73d31`, two 20-second campaigns with two workers passed:
`FuzzExactRationalCircleContainment` executed 6,795 cases;
`FuzzOrient3DAntisymmetry` executed 51,408. No failures or skips were reported.
The combined bridge's bounded JSON request fuzz campaign executed 926 cases
with two workers in 21.3 seconds, with no failures. It checks that rejected
requests preserve handle state and that clones and transforms own independent
handles. Mutated requests are limited to bounded registry operations.

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

A later full checkpoint passed with a 25-minute package budget (the trimmed
suite took 1,223 seconds). A concurrent run exceeded that budget while still
in the trimmed suite; the other packages finished. CI now allocates 45 minutes
per ordinary package and 60 per race package to preserve the full fixture
set. Those new CI budgets have not yet been verified remotely.

The subsequent local 45-minute run passed across all packages; the full
trimmed suite took 2,204 seconds. Later analytic cylinder and exchange
milestones also passed their focused race and differential checks.

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
The same irrational wire imprints both source shells and sews exact Boolean
boundaries for union, intersection and subtraction. Their face counts are
12, 5 and 10; each has Euler characteristic two. The native Boolean recipe
reconstructs topology from the source shells even when public diagnostics
have been modified. Its intersection flux bounds `[0.00963859, 0.06284603]`
enclose the independently integrated `(sqrt(2)−1)/12`; the interval is
conservative and does not imply that precision for other inputs.

Independent OCCT qualification of six- and seven-edge IGES lens records
preserves two faces and the full split topology, along with original raw
model/owner curve bases and both owners' knot-cell ancestry. Irrational
partition cuts outside the bounded exchange format are rejected.
The rotated affine-basis lens has six original seams and independent OCCT
area and volume checks in both directions. The partitioned document chains
recheck native source geometry, placements, raw bases and per-owner cells.

Partial coincidence tests compare differently weighted and knotted spline
charts on an exact overlap starting at `1/3`, and independently reject an
altered pole. Sheared single-cell graph tests retain a triangle with an
exact `2/3` corner. The line/parabola node test proves exactly one
transverse intersection, four original terminals, and the two branch pairs.

Canonical closed periodic fixtures cover a true cone pole, independently
tapered coaxial bore walls, and three wrapped loft bands with two genuine
station creases. They pass outward manifold and exact represented contact
checks. The loft test reloads the native solid before mesh qualification,
and matches the original station identities along all 64 shared samples.

Signed partial polygon revolutions qualify a translated, rotated, concave
meridian both with and without an axis side. The independent first radial
moments give volume `abs(angle)*2.5` and `abs(angle)*5.5` respectively. A
negative sweep retains topology and outward tessellation after native reload.
The helix tests compare both hands to independent turn, speed and derivative
formulas and verify conservative ideal-curve chord and parameter bounds.
The two-turn helical polygon solid qualifies both hands on a translated
frame, keeps five faces and Euler characteristic two after native reload,
and agrees with independent volume `7π/3` and axial centroid. The display
uses the analytic derivative deflection bound; a profile width equal to
the lead is rejected because adjacent turns can meet.
The coaxial common operation clips a convex meridian exactly and verifies
the retained helical solid against `5π/3` for both hands. It distinguishes
zero-width contact from unresolved caps cutting through the sweep.

Independent OCCT polygon revolve fixtures compare area, volume and centroid
for 15 signed partial sweeps, including concave meridians and both apex
directions on a rotated axis. A separate exact trig display proof validates
closed embedding for selected sweeps, including rounded tilted cap frames.
The circular-meridian tests cover both signed quarter turns and a full torus.
Signed rectangular extrusions with two exact circular bores preserve four
cap inner wires, Euler characteristic minus two, and independent volume
`abs(height)*(32-2π*0.75²)` through native reload.
Mixed signed through/blind extrusion tests verify volume and distinguish the
void, floor and retained material behind it after native reload. The cylinder
groove tests compare both handedness values with independently integrated
volume `(12-11/96)π`, recheck original cap/wall topology, and reject a cut
crossing the current cylinder seam.
The twisted chart case retains a two-turn groove without splitting its
opening at that seam, verifies volume `(12-11/24)π`, and checks a closed
analytic-bound tessellation after native reload for both hands.
Two start openings on the same twisted wall independently verify retained
volume `(16-11/12)π`, preserve both inner wires, and reject starts whose
profile widths reach their phase spacing. Exact axial helical queries
retain intervals and ancestry and distinguish retained from cut material
on their selected finite line.
An internally grooved tube retains Euler characteristic zero and its
analytic annular caps through reload. Both handedness values match the
independent two-start volume `(16-5/6)π`. The paired profile query reports
exact radial-slice clearance with original profile edges and finite turns.

Finite planar extrusion tests reproduce the selected-cap volume change from
500 to 1000 as the target is rebuilt. They check positive and negative sweeps
to a tilted cap, a diagonal sweep, contact with coincident boundaries, and
exact loss of coverage at a target hole, a concave notch, and a one-ULP
overhang. Parallel rays, nonplanarity, endpoints without an exact float64
representation, a lost face index, and cancellation return no partial solid.
These are Go kernel tests; the PlainCAD browser gate is separate.

Circular stock with one eccentric bore and one blind pocket was checked for
both signed extrusions: independent volume and centroid, native reload, bore
and floor point classification, closed collision-free display and ancestry for
every native face. A pinned OCCT STEP reader independently validated positive
and negative exports with and without the cavities and matched analytic
volume, area and centroid. The general trimmed tessellator hit its budget on
the mixed round-stock fixture; use the specialized certified tessellator.
Circular finite termination tests cover the six coordinate directions and
reject footprint overlap with target holes or one-ULP uncovered material.

Mixed line and quarter-circle rectangle extrusions with one sharp corner,
three unequal radii, a through bore, and a blind pocket pass independent
analytic mass and area checks on a rotated coordinate frame in both
directions. Native reload retains the rounded wall, bore and floor point
queries. The certified mesh checks closed topology, zero contacts and all
straight/curved strip ancestry. Pinned OCCT reads the STEP boundary with
three exterior curved walls and matches volume, area and centroid for both
signs. These cases qualify quarter circles on a rectangular footprint.

Mixed rounded stock tests use a circular bore, a separate rectangular
through-pocket, and a rectangular boss. They independently check the added
and removed volumes, the two through handles, point queries on the boss, and
the embedding and face ancestry of its certified mesh. The feature layout
uses the dimensions from the PlainCAD scenario; this particular fixture
rounds vertical corners. A separate long-cap treatment reproduces the
application's horizontal fillet and opposite chamfer and tests both analytic
surfaces. The integrated horizontal feature chain is tracked separately.

The actual long-cap edge treatment fixture retains a cylindrical horizontal
fillet and a planar chamfer on the opposite cap edge. The native chain grafts
a circular through-bore, rectangular through-pocket, and boss onto those
supports, preserves original model-edge and vertex IDs, and matches the
PlainCAD fixture's analytic mass formula. Native reload, genus-two closed
topology, and a complete mesh contact check pass. The generic display uses
an explicit 128-subdivision budget; its distance is not certified here.

The certified circular revolve display checks both signed elbow directions
and a periodic full torus, including shared rim indices and independent
faceted volume. A tilted-frame fixture checks the reported ideal-boundary
enclosure against the analytic parameter correspondence. Exact display
contact and positive orientation checks are independent of the error bound.
Signed elbow queries qualify both planar caps, side hits and material
classification before and after native reload for angles below and above π.

Document recipes regenerate a planar to-face tool after edits to its selected
box face, switch a signed polygon revolution between coordinate axes, and
rebuild the long cap chain after radius and depth edits. Missing references,
uncovered targets, excessive angles, and feature intrusion leave the prior
document revision intact. Tests also cover owned recipe snapshots, undo,
redo, and reload.

Paired two-start shaft and nut fixtures match independent exact removed volume
coefficients 21/4 and 4 (times π), and native retained volumes for both hands.
The shared radial band [7/4,2] has exact nominal lower and upper flank gaps
of 1/8. Tests distinguish shaft valleys, retained ridge and clearance probes,
reject zero clearance on either flank, and preserve both native parts on
reload. These tests do not qualify clearance at finite runouts.

Finite helical engagement tests clip a ridge spanning neighboring starts at
the last available turn, preserve both exact shaft and nut witnesses, and
distinguish nominal flank fit from actual runout overlap on a quarter ray.
A shifted nut tests cap-only contact. The independent stepped-entry relief
preserves all later groove supports and qualifies the analytic removed
volume and native reload; it does not intersect a groove. Helical document
recipes preserve both source meridians and handedness and reject edited
zero-clearance flanks on regeneration.

Exact helical runout tests cover full-turn phase seam reindexing, unequal
start counts, both hands, rational shifts, and an interference witness that
does not occur on either radial boundary. Finished-pair tests verify both
nut shoulders and the shaft tip through independent mass differences. A
full-turn comparison splits at relief radii and distinguishes a radial-wall
witness from an interior one; it preserves cap-only contact as zero positive
axial overlap.

Finite coaxial sphere/cylinder intersection tests distinguish two curved
rims, one curved rim plus an axial cap, caps at the exact transverse rim,
a spherical zone, and regularized pole contact. Analytic mass agrees with
independent axial integration; a mesh has positive volume and no detected
contacts. Off-axis support tests independently qualify all angular topology
regimes, including algebraic boundary rays and the distinct curve tangent
directions at a shared node.

Finite coaxial subtraction tests compare native mass with independent exact
coefficients for an enclosed cavity, blind and through bores, and two caps
separated by a cut. They verify an empty result for complete removal, no
change for a pole-only cut, and rejection of a nonmanifold tool-floor pinch
or a seam outside the exact representation contract.

The long fillet and chamfer STEP fixtures retain native vertex and edge
identities through export. The optional OCCT oracle builds an independent
arc-and-line prism, adds the bores and pocket by cutting, and adds the boss
by fusing; both resulting STEP bodies match its area, volume and centroid.

Finite union tests qualify annular shoulders, both transverse rim joins,
contained stock and two disconnected bodies. The shoulder mesh has positive
orientation and no detected self-contacts. Independent OCCT comparisons
match native area, volume and centroid for a shoulder union, blind bore,
finite common with one axial cap, and finite common with two algebraic
sphere-to-cylinder rim heights and algebraic cap radii. The off-axis
finite graph tests distinguish cap folds, a four-germ node, and isolated
points without inventing edges for sheets clipped to zero width.

The native off-axis blind bore matches the independent OCCT cut in area,
volume and centroid using adaptive oracle integration. An independent
tool-polar volume integral and native save/reload also agree. Rational
chart Hessian bounds qualify the real-arithmetic tessellation error,
excluding evaluation roundoff. The conforming mesh has positive signed
volume and passes a complete self-contact check. The exact query tests
cover both support junctions, hidden sphere filtering, tangencies, line
endpoints, and contained cutters that do not cover the sphere axis.

The tool-centered shell covers cutters centered on the sphere axis, tangent
to it in projection, and separated from it. The upper annulus and lower
hemisphere share an equatorial seam and analytic derivative bounds. The
separated case has a closed mesh without detected contacts, exact native
reload, and independent OCCT area, volume and centroid checks. Coincident
line tests also preserve floor and wall intervals, clipped and reversed
parameter ranges, exact endpoints, and both owners at a shared rim.

The native offset through bore matches an independent OCCT cut and the
volume identity from two half cuts. Both rim loops persist through native
reload. Its closed oriented mesh passes the self-contact check. Exact
line queries retain both shared sphere/cylinder owners on wall intervals.

The finite off-axis common tests qualify every choice of flat or spherical
end with an independent polar volume integral, native reload, and a closed
mesh with an analytic error bound. OCCT independently matches the case
with one flat end and one spherical end. Exact cap classification covers
the partial lens, tangencies, and coincident disks; exact query tests also
cover isolated points and intervals owned by either finite cap.

The upper partial lens fixture matches the independent OCCT intersection
and tool-polar volume integral. Both algebraic cap events remain available
alongside the native spherical strips, cylinder wall and planar lens.
Native reload and sampled watertight display with no detected contacts
pass. The rounded angular split is not covered by a deflection certificate.

The chord-centered counterpart covers lenses containing neither circle
center. It agrees with the tool-centered chart where both apply and with
OCCT for a narrower lens beyond the tool axis. Native reload and mesh
closure/self-contact tests also pass for the chord-centered shell.

The double-lens shell keeps separate algebraic cap points at shared angular
stations and handles unequal lens radii. Independent polar volume integration,
native reload and sampled mesh closure pass. OCCT compares area, volume and
centroid for an asymmetric pair of partial caps.

The off-axis boss union qualifies the spherical-to-cylindrical rim, exposed
flat end, native reload, and a closed display with an analytic deflection
bound. Its volume satisfies the Boolean volume identity and independently
matches OCCT area, volume, and centroid.

Same-side lens tests cover both hemispheres and different lens widths,
preserve the exact cap and wall incidence report, and verify volume with
independent polar integration. Native reload and sampled mesh closure pass.
The positive slab matches the independent OCCT intersection.

Finite offset dispatch tests cover both hemispheres, preserved cap ownership,
reflected lower lenses, mixed full spherical and partial lens ends, disjoint
and tangent regularization, and rejection of an unhandled sphere-only disk.

Covered-slab tests use exact containment at the largest sphere section,
including tangency, full sphere retention, and irrational planar radii
next to an uncut pole. The shell keeps no cylindrical wall. Native mass
matches exact axial integration and an independent OCCT cut.

Perpendicular cylinder fixtures cover smooth closed seams at unequal radii,
the equal-radius tangent nodes, exact finite cap clipping, and coincident
line intervals. The unequal native solid matches independent slice volume
and OCCT area, volume, and centroid. Native reload preserves its supports,
and the certified display passes closure and self-contact tests. Algebraic
sphere/cylinder tests now treat retained uncut poles as a positive case.

The skew orthogonal common matches independent rectangle slice integration
and OCCT mass properties. The fixture preserves placement, orientation,
and native chart data on reload. Its certified display has no detected
self-contacts. Equal-radius native equator-cap tests retain one half of the
Steinmetz solid and verify analytic area, volume, and centroid.

Nonnested skew cylinders use a regularized shared-coordinate chart at both
ends. Native reload and a certified closed mesh pass. Independent slice
integrals and OCCT mass properties agree. The oblique equal-radius fixture
retains both analytic cylinder supports and proves exact cap clearance and
volume; it also passes reload, certified display, and OCCT comparison.

The unequal-radius oblique common retains original cylinder ownership for
either radius ordering and handles different raw axis norms. Independent
slice integration and OCCT mass checks agree. The reloaded body has a
closed certified display with no detected contacts.

The oblique skew fixture checks original cylinder equations, affine volume
scaling, shifted axes, certified display and native reload. OCCT independently
qualifies the asymmetric overlapping case. A nonrepresentable placement is
rejected by this exact-map path.

The finite cylinder dispatcher has six independent OCCT differential cases:
one clipped cap, a two-cap slab, a two-axis quadrant, a translated signed and
scaled perpendicular slab, reversed and scaled parallel overlap, and disjoint
empty. Both the default and STEP+IGES pinned oracle builds pass the focused
check. Nonempty cases compare area, volume, and centroid. Native recipe tests
also verify round-trip raw cap parameters and face ancestry and reject altered
payloads. Further differential cases qualify both-axis clipped caps, opposite
clipped tails, and exact containment of a skew perpendicular cylinder. Four-cap
fixtures compare native volume with independent slice integration and verify
face ownership, native reload, mesh closure, and no detected self-intersections.

The open-plane spline shell fixtures qualify union, intersection and difference
of warped overlapping solids. OCCT independently extrudes the bilinear base
patches and compares the area, volume and centroid of all three results. Tests
also check simultaneous source-edge ownership, require proof before discarding
conservative extra pairs, and reject omitted contacts.

The corner-plane clip adds a contact through an existing stock vertex. It
proves all 36 original face pairs, the oriented section cycle, and strict
inactive halfspaces before sewing. Analytic volumes `7/32` and `25/32`,
original-face and edge ancestry, native reload, a closed mesh, and zero
detected display contacts verify the two retained bodies. OCCT independently
sweeps the bilinear stock and the planar cutter and verifies both shell counts
and mass properties.
Shifting the active cutter plane by `1/8` also qualifies a closed section with
four original edge-interior events and no vertex contact. Native intersection
and cut retain their original stock supports and the oriented tool cap. Their
volumes `11/32` and `21/32`, centroid, and area match independent checks.
An active plane through two opposite original stock vertices certifies both
oriented corner fans and all four exact side branches. Native results retain
both vertex identities and source-edge ancestry, have volumes `17/32` and
`15/32`, reload identically when the proof order is reversed, and produce
closed display meshes with no detected contacts. An independent OCCT sweep
and planar cutter verify one shell, area, volume, and centroid for each result.
Two opposite active planes qualify a six-face slab on the same warped stock.
Exact ancestry covers eight cap section edges and four retained stock edge
intervals. The independent OCCT cutter is an oblique planar face swept by
`1/4`; the result agrees on one shell, volume `1/4`, centroid and area.
Two nonparallel cutter prisms with disjoint caps produce one six-face shell
on the warped stock. Native reload, edge ancestry, mesh closure and exact
display contacts are checked in both cutter orders. Independent sequential
OCCT commons agree on area, centroid and volume `1/2`.
The crossing-cap fixture produces a single shell with one shared cap-cap
segment. Both original charts lift the complete exact segment, including an
independent interior rational sample, and the ancestry accounts for every
output edge. The closed mesh and native reload pass. Independent sequential
OCCT commons agree on area, centroid, and volume `25/192`.
The regular quadratic graph cut retains the parabola on the original warped
stock and cap as exact native pcurves. Source edge parameters, independent
volumes `29/640` and `611/640`, centroids, native reload, and closed display
meshes with zero detected contacts pass. The independently swept OCCT stock
and planar cutter agree on both shell counts, area, volume and centroid.
The piecewise extension proves full-range C0 continuity at an actual crease,
retains six exact section pieces and two original-chart knot junctions, and
rejects a source with agreeing section endpoints but a discontinuous seam
away from them. Native common and cut volumes are `631/15360` and
`14729/15360`. Independent OCCT shell, area, volume, and centroid checks pass.
OCCT's ordinary volume integrator differed on first moments at the crease;
its span-aware Gauss–Kronrod integrator agrees with the analytic moments.
The bounded polynomial graph clip retains a cubic model section as native
`SplineUV` on both original supports. Exact composition proves the full shared
curve, and the native reader rejects a changed interior with unchanged endpoints.
The independent OCCT swept stock and box agree on one shell, area, centroid,
and volumes `23/448` and `425/448`. A separate test proves a quartic section;
unrepresentable trim controls and degrees above four fail explicitly.
The piecewise polynomial fixture crosses a genuine C0 crease in
`x³/8+y/4+|x−1/2|/32`. The two exact knot junctions retain their original
UV and model coordinates and both neighboring edge owners. The native common
and cut agree with analytic integrals at volumes `4031/86016` and
`81985/86016`; independent span-aware OCCT calculations agree on shell count,
area, volume, and centroid for both operations.
An oblique cutter `Z=3/16+X/32` produces cubic and creased commons of
`3257/53760` and `23921/430080`. Both complementary cuts, original cap
support, native reload, closed meshes, and absence of undeclared mesh
contacts pass. Independent OCCT prisms agree on area, volume and centroid
for all four results. A cyclic physical-axis permutation checks the YZ
projection without changing the original UV coordinates.

A combined short suite at `5a3c612` reached the existing 45-minute package
deadline in `brep/trimmed` while checking a mixed-contour display mesh. The
other packages passed. Point-only B-spline surface evaluation now visits only
the active tensor span and skips derivatives. The degree-two rational
benchmark dropped from 1.25–1.85 microseconds, 3,872 bytes and 25 allocations
per call to 0.18–0.23 microseconds and zero allocations. Full-multiplicity
knot sidedness, the timed-out mixed-contour fixture in isolation, and the
corner/slab mesh paths pass focused normal and race checks. This isolated
result does not supersede the combined run until it is rerun.
First partial derivatives now use the same active controls and differentiate
the adjacent homogeneous controls in the original knot units. A separable
rational surface checks both derivatives analytically at interior points and
domain ends. The corresponding benchmark now uses zero allocations and took
0.40–0.49 microseconds per call.
Second partial derivatives use the active span as well. Existing repeated-knot
and rational surface tests pass normally, under race, and on Go 1.23. The
second-partial benchmark uses zero allocations; it took 1.46–2.06 microseconds
instead of 6.32–6.62 microseconds on the sampled host.

On combined `5580058`, the opt-in pinned OCCT run with STEP, IGES, and
transverse flags passed 112 test and subtest entries. The transverse union
hit its existing 120-second deadline while other work was active; its common
and cut passed. Isolated union runs retained the original deadline: baseline
73.97 seconds, tube coverage cache 86.50 seconds, and oriented trim memo
49.54 seconds. Wall times reflect different load; profiled tessellation CPU
dropped from 12.62 to 7.28 seconds with the trim memo. The first cache is
verified for equivalence and budgets; this fixture does not show its speedup.

The perpendicular union retains both walls and four exposed end disks.
Its volume matches the exact inclusion-exclusion formula and its area
matches the two cylinders minus the interior Steinmetz faces. Native reload,
certified display, no detected self-contacts, and OCCT comparison pass.

The oblique equal-radius union preserves both original cylinder equations
and four exposed ends. Native mass matches the inclusion-exclusion volume
and independent OCCT properties, with reload and certified display checks.

Oblique subtraction verifies the two component volumes, opposing centroids,
both original support types, exact pole-contact provenance, and a closed
display for each body after reload. OCCT independently returns and qualifies
the corresponding two solids.

The wider perpendicular cutter yields two strictly separated bodies. Tests
check the analytic gap, inclusion-exclusion volume, native reload, certified
display, and each component against an independent OCCT cut.
The oblique variant checks the same invariants with the gap measured along
the unit normal to the cutter axis in the two-axis plane.

The narrower perpendicular cutter uses signed, scaled raw axes and retains
one closed stock with a through-bore. Its stock and cutter faces match their
original support equations, the bore normals point into removed material,
and the display mesh has genus one and no detected self-contacts. Independent
slice integration and the pinned OCCT cut agree on area and volume; the OCCT
cut also verifies centroid and one closed shell. Native geometry and the raw
difference recipe both reload, and the unused result slot remains empty.

Coaxial through-bore fixtures cover both exactly coincident and extending
cutter caps on all three coordinate axes. The native annulus preserves the
original signed stock cap order and both radii. Its exact volume coefficient,
independent OCCT mass properties, canonical reload, and closed genus-one mesh
with no detected contacts verify the retained result.
The eccentric case independently verifies the centroid shift, original bore
radius and axis, canonical reload, and rejection of tangent or clipped disks.
The enclosed cavity uses one stock and two oriented closed shells. It
preserves both cutter floors in raw parameter order and compares area,
volume, and centroid against OCCT. Its canonical recipe and certified mesh
pass; radial tangency is rejected, and axial opening selects the blind-bore
path instead of emitting an interior shell.
Coaxial union fixtures with overlapping axial intervals and exactly touching
end disks verify both operand orders, exposed cap ancestry, exact common
volume, canonical reload, and a single stock against the pinned OCCT fuse.
Unequal coaxial union also retains the original annular step support for
overlapping and exactly touching ends; canonical reload, analytic-bound
mesh closure, and independent OCCT mass properties agree.
Strict radial containment also qualifies one eccentric parallel boss extending
through a stock end, retaining the exposed annulus on the original cap plane.
Both axial ends, operand orders, overlapping and exactly touching spans pass
mass and closed-mesh tests. The native recipe rechecks original cap ancestry,
and independent OCCT overlap and touching fuses agree on one shell and mass.
When the eccentric boss projects past both stock caps, both annular shoulders
retain their source cap ownership and the internal narrow wall is discarded.
The two-end fixture passes closed-mesh and independent OCCT mass checks. The
single-end native recipe keeps its existing face ordering.
Equal-cap parallel transverse sets compare signed cylinder placement, native
recipe reload, original cap ancestry, and area, volume, and centroid against
OCCT. Near tangencies and differing caps remain outside this path.
The same transverse union and difference now support selected oblique
parallel axes through an exactly represented similarity. Antiparallel
input order preserves original wall and cap ownership. Both native reload
and closed meshes pass, and independent OCCT mass properties agree.
The native two-radius stepped cylinder can also round-trip through STEP from
its proved complete revolution boundary. Both signed axial orientations retain
the analytic annular cap, units, name, and mass. OCCT independently reads the
export; unsupported boundaries publish no STEP bytes.
Eccentric blind-bore fixtures cover both axial entries, signed and scaled
axes, and cutter caps coincident with or beyond the entry. They verify exact
mass identities, native reload, floor ancestry, rejection of grazing cuts,
mesh closure and independent OCCT area, volume and centroid.
Coordinate-parallel cutters that completely cover a stock disk now remove
an axial interval. Signed/scaled and antiparallel axes preserve each stock
wall interval and the original cutter-cap identity, with exact squared gap
for two resulting solids. Internal radial tangency may still cover the entire
disk, but the first representable uncovered sliver fails this route. Native
reload rechecks cap ancestry, and independent OCCT cut results agree on both
shells and their area, volume, and centroids.

Asymmetric clear-cap fixtures verify each original raw cap in both union
orders and in the narrower-cutter difference. Independent extension volume
and first moments, the pinned OCCT result, canonical recipe reload, and a
closed mesh all agree while caps that enter the common are still rejected.

The original pinned PlainCAD production suite (`5942ff9`) against the clean
reconciliation snapshot `8aba3fb` passed 64 of 66 on port 5418, including the
controlled native rebuild and validated STL performance case in 39 seconds.
Both focused-workspace fixtures use saved browser storage for port 5280. An
isolated rerun at that original port, with the same WASM, original tests and
original production configuration, passed both in 2.4 and 4.0 seconds. The
[run record](evidence/plaincad-pinned-production-reconciliation-2026-11-13.json)
contains hashes and per-case results. These two runs cover all 66 cases; a
single full run on the original port and qualification of later revisions are
separate checks. The production runner now rejects changed ports for fixed-origin
fixtures before building.

The newer clean PlainCAD archive `1c59128` against reconciliation snapshot
`5a69c0b` independently passed the real Go-WASM backend probe, the original
zoomed native display test, and the original ORBIT orbit/zoom test including
full-resolution PNG export during a gesture. The [reconciled run record](evidence/plaincad-reconciled-orbit-2026-11-13.json)
records the exact app and kernel revisions. Production preflight verified all
841 recorded original app files against the pinned Git tree and replayed the
three display runtime edits from those originals before accepting the staged
source. No application source checkout was modified.

The same `1c59128`/`5a69c0b` archive and WASM passed all **66 original
production cases in one run** at port 5280, with no failures, retries, or
skips. The original production configuration hash was unchanged. The controlled
native model and validated STL performance fixture finished in 50.4 seconds
within its existing 90-second budget. See the
[complete production record](evidence/plaincad-reconciled-production-2026-11-13.json)
for fixture and source identities. Later integration changes require their own
qualification.

The independent [Firefox and WebKit OCCT control](evidence/plaincad-cross-browser-occt-firefox-webkit-2026-11-13.json)
ran the untouched current PlainCAD archive at the original port 5281. Both
engines reproduced the same four stale UI failures and passed the original
keyboard face-card and dimension case. With the Chromium control, the three
engines all reproduce the replacement run's three passes and twelve failures.
The unchanged suite remains failing; the control establishes that these
observed failures are shared by the original provider.

The clean reconciled candidate `73cb617` and current PlainCAD archive
`e65f07b` passed [all eleven original arc-region regressions](evidence/plaincad-reconciled-arc-regressions-2026-11-13.json),
including major arc fragmentation, all signed plane variants of circle/arc
intersections and arc-only regions, and the analytic YZ Extend workflow.
The real Go-WASM preflight and both original TypeScript checks also passed.
This focused pass qualifies the merged arc implementation independently of
the earlier full development run, which started before the arc merge.

The [reconciled divider retest](evidence/plaincad-reconciled-divider-regressions-2026-11-13.json)
passed the original XZ connected-group gesture and all six original analytic
circle and arc divider cases. It used `db9c484` with current app archive
`e65f07b`, after the analytic arc merge. The earlier frozen development suite
still reports its own failures; these selected retests identify the repaired
cases without altering that original report.

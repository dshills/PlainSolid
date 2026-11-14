# Implementation status

The full OCCT replacement is **not complete**. This document distinguishes the
implemented, tested kernel from the much larger specification. Full replacement
and general-purpose production use remain unqualified.

Recent native progress covers the original mixed curved intersection, union,
and difference (`407ea58d`), revalidated persistence and conservative volume
bounds (`10a59a3f`), rational weighted stock cut/line/first-moment queries
(`1ee34f66`, `cc94975e`), and seven exact ellipse-prism source forms in one
bounded STEP dispatch (`96ccfff0`). Proper rational placements preserve
fillet distance witnesses even for irrational azimuths (`4f1e9580`).
These additions do not qualify arbitrary curved Boolean inputs, arbitrary
STEP assemblies, or general offset/fillet topology.
The weighted stock case subsequently gained a closed certified fill
(`e594887`) and exact original-wall distance under placement (`952e9a4`).
Exact IGES exchange covers full and directed quarter ellipses, elliptical
disk and concentric annulus faces, and a strictly authenticated model-only
elliptical prism topology graph (`a33b4a4`). Pinned OCCT accepts the
authored prism graph; the independently emitted vendor spline file
remains outside the exact input domain. Filled and concentric hollow
prisms can be sewn into native solids and revalidated on reload, including
proper signed-axis placement with exactly stored translation. All five
planar mixed Boolean fragments have certified fills on the shared
boundary network (`7e3c670`). The original mixed intersection now has a
closed certified mesh for all eight fragments, with both strict signed
roof cells and separate collars bound to original trims. Its 2,004
triangles preserve the exact boundary source and have a source error
bound below 0.098 model units. The matching difference also has a closed
certified mesh for all 14 fragments and two independently oriented
components. Its 2,018 triangles retain the original charts and reversed
cutter faces. The matching union has a certified closed mesh with 2,076
triangles for all 17 original fragments. It includes the roof with an
enclosed hole, uses the shared original boundary samples, and has a
source error bound below 0.143 model units. Its complement proof clips
query cells to the opposing chart before testing height. All three mesh
operations now run end to end on the two original shells without caller
face assignments or rational wall seeds.
On proved original roof boxes, exact rational point and line queries
preserve original floor, side, and roof owners, including simultaneous
contacts and a retained difference span below binary64 resolution.
The arbitrary-direction line sweep isolates up to quartic restrictions,
distinguishes repeated roots from crossings, and reports exposed
coincident spans separately from the interior. It qualifies both the
mixed restricted-wall Boolean and the independently curved bubble roofs.
The native tube adapter also assembles the original triple radial
intersection as separate disk and annular solids with four original
faces, 228 retained edges, and two inner wires. Whole original ring
ownership is required; general algebraic fragment materialization remains open.
The original disk and annular regions now bind the exact radial band
integral to their complete wires and outward supports. The total volume
divided by π is exactly `63333935546875/864691128455135232`; the
outward volume interval is positive and narrower than `2^-80`.
For the mixed roof solids, bounded nearest queries search the original
exposed trims and provide exact rational witnesses, including shared
seam owners. The distance bounds, rotated centroid, first moments, and
inertia have separate original OCCT comparisons.
The same IGES exchange now qualifies exact filled and hollow circular
prisms and exact 45° conical frusta, with source-bound reload, proper
signed-axis placement, and authenticated record, vertex, edge, and
cyclic-use ordering. Exact stock line queries preserve original cap and
wall owners across isolated contacts, coincident runs, and common line
intervals between two placed stock bodies. Directional separation and
nearest owner tests distinguish exact ties from overlapping numerical
enclosures below binary64 precision.
Exact ring torus IGES material now covers canonical and signed-axis placed
sources, including authenticated reordered records. The analytic sphere
reader qualifies exact IGES pole and seam graphs, including proper signed-axis
placements; the promoted native sphere uses its established numerical angular
evaluation and does not claim rational patch sewing.

## Immediate priority: PlainCAD compatibility

The [selectable PlainSolid provider qualification](evidence/plaincad-provider-integration-verified-2026-11-14.json)
pins the PlainCAD app implementation at `9779044` and PlainSolid at
`f571a964`. Its selected build passed 345 development, 66 production and
15 cross-browser cases with no skips or retries. The ordinary app suite
passed all 1,691 cases with its documented worker and retry settings;
four separate contracts exercised actual PlainSolid WASM. The report,
WASM asset, and stamped Go runtime hashes were independently checked.
This opt-in app build has separate evidence from the later kernel worktree
reconciliations below.

Development now prioritizes the operations exercised by PlainCAD's existing
acceptance tests before expanding unrelated OCCT domains. The full specification
remains the eventual target. See [the prioritized plan](IMPLEMENTATION_PLAN.md)
and [PlainCAD readiness](PLAINCAD_READINESS.md) for application-specific gates.

A finite blind Hole crossing one rectangular stock side now retains a
watertight analytic notch and circular-segment floor. Seven original
browser regressions and an authored rebuild, reload and STL check pass.
Canonical STEP now round-trips the placed boundary and matches OCCT's
independent volume, area and centroid. Mixed batches with disjoint
blind bores from either cap and through bores now pass original
regressions, authored persistence, placed STEP and STL fabrication. Single-tool
scoped through-all Hole now accepts the same qualified rectangular side
and corner notches as direct Cut. A one-side through notch also supports
disjoint interior bores and round-trips canonical STEP. Corner-notch
blind and through cuts also round-trip canonical STEP, including batches
with disjoint interior bores. Same-center, equal-radius tools with
connected depth spans can deepen the original notch or open its far cap.
Contained coaxial cutters on the same body merge independently of batch
order, including through cutters that join two opposed blind spans.
Separated blind spans at the same interior site preserve the material web
and both cap openings through STEP.
Intersecting side- and corner-opening Hole groups preserve analytic
cylindrical walls and blind floors on rectangular stock. The arrangement
accepts up to 16 distinct footprints opening on one side or two adjacent
sides, including offset corners. Grouped and successive cuts, unequal
depths, through cuts that leave one solid, and canonical STEP round-trip
pass verification. Cutters can enter from either cap, including opposed
blind spans that overlap or leave a material web. STEP recovers partially
buried cylindrical walls from their neighboring faces. Exact stock
vertex contacts also pass for blind and through cutters, for two cutters
sharing the vertex, and for endpoint tangency when the cutter opens
across the adjacent side. Fully concealed cutters no longer leave extra
STEP face partitions. The exported boundary also round-trips a through
cutter whose entry is concealed by other Holes. Opposed blind floors
can now meet exactly and retain an open passage through the shared
footprint. Two equal-radius through cutters whose perpendicular axes
intersect inside a rectangular block retain exact cylindrical walls and
elliptical intersection edges. Grouped and successive cuts, overlap
queries, STEP and STL pass. An equal-radius crossed through pair can
retain up to 16 strictly separated cardinal companion bores in the same
analytic solid and STEP graph. Three concurrent equal-radius through
Holes along X, Y and Z retain eight exact triple junction vertices and
exchange their elliptical intersection arcs through STEP. One of those
three Holes can stop beyond the full intersection, retaining its planar
floor from either entry side of any cardinal axis. Disjoint through cutters on two or three
cardinal axes now retain canonical STEP, including unequal radii and
non-millimeter import. When the full projected footprints are separated,
disjoint mixed-axis blind bores can also exchange their finite floors and
entry faces through STEP. The finite-bore path also handles crossing
full-through projections when the actual cutter spans have separated
padded bounds. Exact rational comparisons of finite radial sections can
also prove disjointness when those bounds overlap. Intersecting unequal-radius through cutters on
perpendicular axes retain analytic radical trims and pass native rebuild,
project reopen and watertight STL. STEP preserves the analytic support
surfaces and exchanges the nonrational trim rings as cubic splines with
declared 0.0000001 mm uncertainty. The native importer checks the complete
canonical graph before recovering the analytic recipe. Perpendicular
skew axes also retain two radical trim rings and round-trip STEP when
the larger radius covers the smaller bore across the whole transverse
width. Partial transverse overlap now retains a single analytic trim
loop for equal or unequal radii and passes native STL. STEP exchanges
the single loop through compact Hermite splines, retaining the analytic
supports and recovering the native B-rep on readback. Tighter near-contact
convergence and oblique cutter intersections remain open. A blind Hole
can intersect an equal-radius perpendicular through Hole when the floor
lies beyond the complete intersection. Both entry caps, retained face
references, exact elliptical STEP trims and watertight STL pass. Two
intersecting equal-radius perpendicular blind Holes also retain both
complete planar floors and round-trip STEP when both floors lie beyond
the intersection. Repeated covered cutters can be grouped, and a
successive coaxial cutter can deepen or open the blind floor. Equal-radius
perpendicular bores also retain two circular-segment floor faces when
the blind stop passes the crossing axis and remains within the through
bore. STEP and STL preserve both segments on either side of the crossing
axis. A blind stop exactly at the crossing axis retains the wall
intersection and consumes the entire planar floor. The exact far-tangent
stop retains a full floor and passes manifold STEP and STL. The
near-tangent limit forms a nonmanifold contact diameter and rejects.
Unequal-radius perpendicular Holes with one or both floors beyond their
complete intersection now retain those floors through STEP and STL.
Compact finite STEP brings the verified closer-radius and covered-skew
cases below PlainCAD's import limit. The existing spline span guard still
limits some smaller clearances and near-contact geometry. Partial skew
intersections also retain finite floors beyond both tools' axial overlap,
with compact STEP and watertight STL. A single oblique through cutter in rectangular stock now
retains two complete elliptical apertures on opposite or adjacent faces.
The native bridge verifies the finite tool covers both openings before
scoped Cut, retains the stock face IDs, and generates watertight STL.
Its STEP exchange preserves both analytic ellipses and the cylinder support.

PlainCAD also imports the strict half- and quarter-cylinder STEP model
and rooted product dialects from main as native cylindrical stock, retaining its
signed coordinate pose and allowing a subsequent exact Hole and STEP
export.

Circular stock now supports grouped and successive disjoint axial bores,
including preexisting authored eccentric bores and STEP reimport. The
original eight-case Hole, Revolve, compound profile and STEP selection
passes. Supplemental PlainCAD rebuild, save/reopen and STL validation
pass on the same WASM. Disjoint blind bores from opposite circular caps
now preserve their entry faces through STEP and pass six original browser
regressions and independent OCCT exchange checks. Overlapping opposed
bores now also pass exact native volume and surface checks, editable
PlainCAD reload and STL fabrication. Canonical STEP now round-trips
qualified overlapping circular boundaries and matches independent OCCT
measures. Opposed blind bores with the same footprint can also preserve
the remaining material web through STEP and a later through cut.
Eight additional original STEP, Hole Edit and machined-reference
browser regressions pass. Both native trimmed spline cases excluded by
short mode pass with full mode enabled.

The latest completed production run on clean `254c83b` passes all 66
originals on PlainCAD `50432ed`, with zero skips or retries. Its original
performance case finishes in 31.382 seconds, with all 25 edits and 11 STL
downloads validated. The earlier full pass on the same app includes the staged project-open
readiness fix. The original performance case finishes in 42.966 seconds with
all 25 edits and 11 STL downloads validated. The preceding production build
`11216c5` also passes all 66 on `e65f07b`, including main's dense mesh
manifold validation and the 60.790-second performance case.
The earlier integrated build passed 65 cases in one run and the benchmark
in an unchanged isolated retest. The previous complete production pass
was `0b17938`.

The newer corner-batch build `d2db51d` passes 65 of 66 production originals
on `50432ed` while several suites run. The benchmark records all 25 edits
and 11 validated STL downloads but exceeds its 90-second case deadline.
The unchanged test passes on the same production assets in 56.048 seconds
when run separately.

The full development run on PlainCAD `50432ed` passes all 344 originals
on clean `92e22ab`, including Parts Browser and YZ guarded drag. It has
zero skips, retries or flaky results. Both TypeScript checks and the
independent WASM probe pass.

All 15 cross-browser originals pass on clean `19b63f4`. The native short
suite reaches the 30-minute package timeout in `brep/trimmed`; the
remaining 44 tests pass in a direct continuation. The combined trimmed
short-mode runs record 397 passes and four explicit skips.

The frozen development baseline ran 340 originals with 264 passes. Later
focused runs pass 54 of the baseline failures. The first consolidated current
run passes 342 of 344. Its two viewport failures pass in focused verification,
with the staged camera damping update for pan idle. The new full rerun
also passes 342 of 344, including the original pan, marker, gesture and
40-body viewer cases together. Parts Browser and YZ guarded drag fail on
startup timing in that run and both pass unchanged on the identical WASM
in isolation.

The focused development retest passes all seven originals for a pocket
after four bores, island pocket, circular face boss, compound profiles,
opposite cap treatments, XY feature addition and separated rotated
collision. Authored circle measurement and all three circular rim
regressions pass in a separate four-case run. The 40-body rotary fixture now passes cold load, rename and platter edit.
The original exact Join/Cut, Hole Edit with downstream chamfer, multi-body
corner cuts and moved projected cut pass. All 18 originals in the retained
reference and workspace retest pass, including Parts Browser and AI To Face.
The current ORBIT model passes the original close-up, Render and gesture PNG
tests after the display worker reuses accepted native features. The three
original signed profile-edge cases also pass on XY, XZ and YZ. Focused
real-WASM cases now pass for pocket
and Hole lineage, fine lead-screw meshes at one, two and four starts, and
STEP round-trip of machined stock, including independently authored rounded
stock with blind and through bores and a canonical long-edge treatment chain. The current studio cancellation and ORBIT
gesture originals pass together. All 17 original arc-region cases pass.
The full one-worker real-WASM run on PlainCAD `50432ed` passes 1,647 of
1,689 originals. Twenty-two earlier failures pass, 37 repeat and five
newly fail. All five new failures and the recurring cold Unified thread
case pass unchanged on the same WASM in a focused retest.
The earlier full real-WASM unit run on the same app passes 1,630 of 1,689
originals with loopback and concurrent verification. Thirty-eight failures
repeat the preceding full run. All 21 newly failed tests pass unchanged
on the same WASM in the one-worker retest. Eight earlier failures pass. The preceding unit run passes 1,643 of 1,689, including all 39
modeling geometry and 15 local middleware cases. The baseline passed 1,646. Thirty-six failures repeat fallback
expectations and OCCT-private fixture bindings. The two other repeated cases
are a workbench layout timeout that passes in isolation and a laser screening
fixture that disagrees with both initialized kernels. Nine of ten new failures
pass unchanged in focused reruns. The remaining inline sketch workspace
case also fails with pristine OCCT. All ten private tests pass on pristine OCCT. The 26 fallback
fixtures pass 25 on the OCCT control and five with PlainSolid deliberately
uninitialized. Focused original tests now pass all seven portable failures,
including an exact side-tangent blind Hole. This Hole is retained for
modeling and display as a regularized difference. Fabrication and STEP
validation reject its four-face contact. The current PlainCAD revision `50432ed` passes all 15 original cross-browser
cases across Chromium, Firefox and WebKit. It passes six of seven focused
originals. The Parts Browser asynchronous-open timing failure reproduces
and the staged readiness fix passes the original Parts Browser case,
file-drop, recovery and both stale-worker cases. A separate browser probe
verifies superseded and rejected opens. The native bridge, STEP and mesh
packages pass a combined short run.
See [PlainCAD readiness](PLAINCAD_READINESS.md).

Scoped Extrude and Revolve Join, fitted parts, cached rebuilds, rectangular
Shell and three Hole pattern cases pass the expanded original selection.
The same run exposes two remaining pattern failures, a circular pocket
pattern on previously cut stock and the overlapping Hole diagnostic. Both
now pass the focused original retest. Its controlled performance test passes
the original 25 rebuild and 11 export iterations and resource-growth bounds.
Slider, hinge and separated rotated collision originals pass.

Focused browser runs now pass the original mixed profile, revolve, socket-head
blank, shop drawing and STEP workflows, including the three repaired regressions
for the solved circular segment, placed bore axis and imported STEP STL.
The first consolidated run passes 47 of 53 originals. A finer bore mesh
exhausts the triangulation budget in six guided multi-hole previews. Single
bores retain the finer setting and multiple rims use their earlier setting.
The corrected focused runs pass both original XY guided cases and STEP STL.
Both original helical cases now pass on one immutable snapshot. Modeled
Thread completes external preview/edit/cold-open/STL and internal preview,
apply and exact volume in 79.325 seconds under its 90-second deadline. The
four-start lead screw and matching nut pass linked edits and cold open in
22.783 seconds. The original gallery extends qualification to Acme and UNC.

The original machined-reference acceptance passes nested rectangular blind
pockets, a disjoint cylindrical through bore, measured rim and floor,
parameter edits, save/open, and rejection after the original floor is cut.
All seven selected original production cases pass, including browser CSP,
native modeling and STL, STEP workers, scope capture and shop drawing.
The original three grouped circular rim fillet cases also pass.

The original retained-reference cases now pass across two focused runs,
including whole rectangular cap chamfer after a bore, an authored fillet
after Cut and Join, merged STL with a circular boss and a retained side
sketch. Complete rectangular cap fillets pass native volume, area, centroid
and mesh checks. All eight original viewport-operation cases pass, including
complete rectangular cap fillet and chamfer on XY, XZ and YZ, individual
chamfer on those planes, cancellation and the section overlay.

Finite polygon and circular To Face extrusion pass the five selected original
PlainCAD cases, including face pockets and component placement. The bridge
uses the actual bounded planar support, tracks it through owner edits, and
rejects lost, uncovered or zero-depth targets. The pinned 31-case run passes
30 and exposes an exact tangent pocket in the earlier edge-treatment chain.
That boundary is now split and sewn. The deterministic native cases and the
first two combined browser runs pass the long chain.

Grouped longitudinal fillet and chamfer geometry is committed as `4f51147`.
The final browser run passes PlainCAD's existing multi-edge edit and repeated
XZ edge-draft cases and the prior 24 regressions. All 26 existing cases, both
TypeScript checks and the supplemental grouped chamfer probe pass with zero
skips or flaky results. Groups resolve up to four parallel original edges on
qualified rectangular stock, with a single publication after validation.
Complete circular rims and sharp rectangular cap perimeters now have native
constructors. General nonparallel and intersecting treatments remain open.

The imported analytic edge chain is committed as `4916667`. Its browser
checkpoint passed PlainCAD's existing fillet, chamfer, Hole, pocket and boss workflow,
including edits, downstream failure recovery, save/reopen and STL. The adjacent
edge-treatment recovery case and the previous 22 cases pass as well. All 24
existing cases, both TypeScript checks, and the supplemental WASM probe pass with
zero skips or flaky results. The bridge supports individual cap edges and a
single disjoint through bore, rectangular through pocket and upper boss on the
qualified rectangular stock. Broader edge treatments and cuts on modified stock
remain open. See [PlainCAD readiness](PLAINCAD_READINESS.md) for the evidence.

The Go WebAssembly adapter implements scoped Hole operations with indexed
center/target errors and atomic publication. Analytic cylinder walls and planar
floors support blind and through cuts from qualified convex stock. Cutter sets
combine proper overlaps, duplicates and contained circles; unequal depths now
use exact axial layers with crescent and annular floors. Retained recipes are
bounded to 32 tools. A volume-only overlap query supports successive Hole
features on already-bored stock without fabricating intersection geometry.

The unequal-depth analytic constructor is committed as `302184f`. Independent
volume, area and centroid oracles, related bore/mesh regressions, bridge race
checks and geometry review pass. All 22 selected existing browser acceptance
cases pass unchanged, with zero skips or flaky results. Both TypeScript checks
and the separate real-WASM unequal-depth/document-roundtrip supplement pass.

PlainCAD has no existing overlap-specific browser case; its guided-hole UI still
intentionally rejects overlapping placement. New overlap checks are supplemental.
Opposed-entry blind overlaps now pass exact measures, manifold topology,
PlainCAD save/reopen, and STL validation. Coincident stop levels qualify
for the supported circle arrangements. Circular stock also supports one strictly contained eccentric blind or
through bore, with native STEP readback and STL validation. General internal
cutters, unresolved contacts, material-island footprints, closed cavities,
and arbitrary curved targets remain open. Some named interior pocket floors and one-side through
cuts already have qualified constructors. The full
replacement remains partial. See source-hashed reports in [PlainCAD readiness](PLAINCAD_READINESS.md).

Current integration evidence is kept on local `codex/reconcile-plaincad`.
The other worktree's revision stays separate and is merged into that review
branch after each coherent milestone. The current committed PlainCAD revision
`50432ed` passed all 66 original production tests on `0312d69`. The integration
worktree independently qualified all 15 current cross-browser cases and all
344 original development cases on its pinned build with staged project-open
handling. The full unit result still contains failures, with separately
recorded isolated retests and provider-specific fixture gaps. Review candidate
`bc7f9c3` combines the other worktree's committed `db05a77` revision with
the newer kernel. The original Go/WASM backend preflight and both supplemental
different-center side openings passed without retries in Chromium, including
rebuild, reopen, STL, and STEP, on all 841 pinned PlainCAD source files.
The [reconciled acceptance record](evidence/plaincad-reconciled-bc7f9c3-side-openings-2026-11-13.json)
identifies the actual browser build. These application checks do not establish
a general OCCT replacement.
Review candidate `754ae8e` combines that same worktree revision with the later
native query, exact-owner mesh, algebraic persistence and exchange milestones.
All packages compiled with CGO disabled. The
[second pinned Chromium run](evidence/plaincad-reconciled-754ae8e-side-openings-2026-11-13.json)
verified the same original backend and both unchanged supplemental opening
workflows with no retries or skips.
The reconciled production build `967e3c3` passed all 66 original PlainCAD
production cases on the same 841 pinned sources. The
[full original development run](evidence/plaincad-reconciled-development-178df69-2026-11-13.json)
passed all 344 cases on review revision `178df69`, with no skips or retries.
Its application source and original test config were independently checked
against the pinned revision. The
[production record](evidence/plaincad-reconciled-967e3c3-production-2026-11-13.json)
includes the verified no-OCCT backend preflight, production build and exact
test selection. The [latest reconciled production run](evidence/plaincad-reconciled-7b037e2d-production-2026-11-14.json)
also passed all 66 original cases on `7b037e2d`, which includes the separate
integration worktree through `cc1c41f`. It passed the original performance
fixture and preserved every original source assertion. The [reconciled original unit run](evidence/plaincad-reconciled-bf37aa5-original-units-2026-11-14.json)
passed 1,653 of 1,689 on `bf37aa5`. All 36 failures also occurred in the
earlier one-worker run. The unit and release gates remain open.
The next reconciled build `0bdb699` passed the five original development STEP
import/export cases with the no-OCCT Go/WASM preflight, no retries or skips,
and checks of all 841 original source files. The
[development STEP record](evidence/plaincad-reconciled-0bdb699-development-step-2026-11-13.json)
covers lifecycle and placed native geometry on that exact build.
The [reconciled modeling run](evidence/plaincad-reconciled-ff15d87-development-modeling-2026-11-14.json)
adds three original passing cases for edge treatment edits, revolve with cut
and join, and the long feature chain through recovery, save/open and STL.
It verified the same 841 original source files and no-OCCT browser preflight.
The [next reconciled run](evidence/plaincad-reconciled-89f244c-development-holes-step-2026-11-14.json)
passed ten original Hole and STEP export tests after merging the separate
worktree's same-side and opposite-cap opening commits. In the review branch,
the existing exact dual-disc recipe runs first; if it cannot represent a
crossing, the independent analytic opening-pair recipe may certify it.
The [different-side reconciliation](evidence/plaincad-reconciled-586dcd3-development-side-holes-2026-11-14.json)
also incorporates the independently developed stock-side dispatch at
`cc1c41f`. Seven supplemental cases passed through Go/WASM, including
rebuild, reopening, STL and STEP for overlapping side holes. These are
separate from the original application acceptance counts.
The next [reconciled Hole and STEP run](evidence/plaincad-reconciled-d52a3af3-development-holes-step-2026-11-14.json)
passed all ten original scenarios on `d52a3af3`, incorporating grouped side
openings from worktree commit `852f3e08`. Exact two-hole stock can feed the
grouped path for a third cut. The combined bridge, STEP, and native focused
tests passed, as did the no-OCCT browser preflight. Both the pinned application
sources and the full Go/WASM source inventory were independently verified.
On the same independent worktree ancestry, [the current cross-browser
reconciliation](evidence/plaincad-reconciled-56b4633-cross-browser-2026-11-14.json)
passed 15 unchanged cases across Chromium, Firefox, and WebKit on
`56b4633`. The exact original Playwright config and every source hash
were independently checked; there were no skipped cases or retries.
The [corner-opening reconciliation](evidence/plaincad-reconciled-1ca7a88-development-corner-holes-step-2026-11-14.json)
combines the worktree's across-corner cutters from `9770a5a` with
the native kernel on `1ca7a88`. All 16 original Hole and STEP
browser cases passed, with no skips or retries. The focused trimmed,
STEP, and bridge tests passed on that same review build.
The [exact-vertex reconciliation](evidence/plaincad-reconciled-d6bfd4e-development-vertex-holes-step-2026-11-14.json)
adds the worktree's `8df00f2` milestone to `d6bfd4e`. The same 16
original browser cases and the separate no-OCCT WASM preflight passed.
The native corner, side-opening, and STEP regression set passed on the
combined revision. On the later [concealed-cutter reconciliation](evidence/plaincad-reconciled-dc2ad4e-development-concealed-holes-step-2026-11-14.json),
all 16 originals and the Go/WASM preflight also passed. That revision
includes worktree commit `c51d2762` for canonical STEP export of the
qualified fully concealed cutters. Native bridge and STEP tests separately
check round-trip measures and removal of redundant wall partitions.
The [shared-floor reconciliation](evidence/plaincad-reconciled-9011e7b-development-shared-floor-holes-step-2026-11-14.json)
adds worktree commit `8e15addf`. All 16 original cases also passed on
`9011e7b`, with no skipped tests or retries and a successful Go/WASM
backend preflight. The verifier confirmed that changing the local port
was the only edit to the original Playwright configuration.
The [crossed-bore reconciliation](evidence/plaincad-reconciled-45218737-development-crossed-bore-holes-step-2026-11-14.json)
adds worktree commit `f571a964` to clean review revision `45218737`.
The trimmed, STEP, and bridge tests passed, including the live OCCT
comparison of the analytic crossed-bore STEP file. All 16 original
browser cases and the Go/WASM preflight passed with no skips or retries.
The verifier checked 841 application and 1,875 kernel source hashes and
confirmed that only the local port changed in the Playwright configuration.
The [disjoint-bore reconciliation](evidence/plaincad-reconciled-6d900753-development-disjoint-bore-holes-step-2026-11-14.json)
adds worktree commit `e5ee180a` to clean review revision `6d900753`.
All 17 original Hole and STEP browser tests and the Go/WASM preflight
passed with no skips or retries. Both crossed and disjoint native STEP
regressions passed against live OCCT. The verifier checked all 841
application and 1,885 kernel source hashes.
The [unequal crossing reconciliation](evidence/plaincad-reconciled-6c990c74-development-unequal-bore-holes-step-2026-11-14.json)
adds worktree commit `144afff1` to review revision `6c990c74`. The shared
adaptive mesher retains the existing helical allocation switch. All 17
original browser cases and the Go/WASM preflight passed without skips or
retries. Three unequal-radius solids matched live OCCT volume, area, and
centroid within `10⁻⁶`. All 841 application and 1,894 kernel source hashes
were verified. STEP for the unequal trim itself was outside this revision.
The [unequal STEP reconciliation](evidence/plaincad-reconciled-f8f37eee-development-unequal-step-holes-step-2026-11-14.json)
adds worktree commit `0b97ce13` to review revision `f8f37eee`.
The unequal boundary exports as bounded STEP splines with declared
distance uncertainty. Focused native agreement, bridge, and live OCCT
readback tests passed. All 17 original browser cases and the separate
Go/WASM preflight passed with no skips or retries. The verifier checked
all 841 application and 1,907 kernel source hashes.
The [separate authored unequal STEP workflow](evidence/plaincad-reconciled-f8f37eee-supplemental-unequal-step-2026-11-14.json)
also passed on the same reconciled WASM. It exports and imports the
unequal boundary, reopens the saved source, and verifies both open
passages in the STL. Pinned OCCT independently read the emitted STEP.
The [skew-bore STEP reconciliation](evidence/plaincad-reconciled-b56d3035-development-skew-step-holes-step-2026-11-14.json)
adds worktree commit `de3aa141` at review revision `b56d3035`.
Focused native geometry, STEP readback against live OCCT, and bridge tests
passed. All 17 original Hole and STEP browser cases and the separate Go/WASM
preflight passed without skips or retries. All 841 app and 1,912 kernel
source hashes were checked.
The [oblique-bore reconciliation](evidence/plaincad-reconciled-d02f38ee-development-oblique-holes-step-2026-11-14.json)
adds worktree commit `96d4a583` at review revision `d02f38ee`. Its adaptive
mesher retains the shared entry point and the existing helical allocation
switch. Native oblique, crossed and skew regressions passed, as did all 17
original Hole and STEP browser cases and the separate Go/WASM preflight.
That snapshot covers oblique modeling and display; oblique STEP is the
next independent review. All 841 app and 1,921 kernel source hashes matched.
The [oblique STEP reconciliation](evidence/plaincad-reconciled-dbab5434-development-oblique-step-holes-step-2026-11-14.json)
adds worktree commit `98c1f0d4` to review revision `dbab5434`. All 17
original browser cases and the Go/WASM preflight passed, and native
regressions passed with the pinned STEP oracle. The [authored oblique
STEP case](evidence/plaincad-reconciled-dbab5434-supplemental-oblique-step-2026-11-14.json)
exercised exact elliptical STEP exchange and STL on the same WASM;
pinned OCCT independently read the downloaded solid. All 841 original
app and 1,929 kernel source hashes matched.
The [partial transverse bore reconciliation](evidence/plaincad-reconciled-3d787ab9-development-partial-skew-holes-step-2026-11-14.json)
adds worktree commit `650934f5` at review revision `3d787ab9`. Native and
bridge tests passed, as did all 17 original browser cases and the Go/WASM
preflight without skips or retries. The scope was the native partially
overlapping geometry. The verifier checked 841 app and 1,932 kernel files.
The [partial overlap STEP reconciliation](evidence/plaincad-reconciled-4a9619a5-development-partial-step-holes-step-2026-11-14.json)
adds `a3259fb7` at review revision `4a9619a5`. All 17 original browser
cases and the Go/WASM preflight passed without skips or retries, and the
native STEP and bridge tests passed with the live oracle. The [separate
authored case](evidence/plaincad-reconciled-4a9619a5-supplemental-partial-step-2026-11-14.json)
verified the shared opening through STEP export, import, and STL. Pinned
OCCT independently read the emitted file. All 841 app and 1,940 kernel
source hashes matched.
The [blind crossed STEP reconciliation](evidence/plaincad-reconciled-0066dae8-development-blind-crossed-holes-step-2026-11-14.json)
adds worktree commit `c2456d0e` to review revision `0066dae8`. Native and
live STEP checks passed, along with 17 original browser cases and the
Go/WASM preflight. The [authored browser case](evidence/plaincad-reconciled-0066dae8-supplemental-blind-crossed-step-2026-11-14.json)
checks the retained blind floor through export, import and STL. Pinned
OCCT independently read the exported solid. All 841 app and 1,950 kernel
source hashes matched.
The [two-blind-bore reconciliation](evidence/plaincad-reconciled-3a4a37c8-development-double-blind-holes-step-2026-11-14.json)
adds worktree commit `7c54cc75` to review revision `3a4a37c8`. All 17 original
browser scenarios and the Go/WASM preflight passed without skips or retries,
matching 841 app and 1,959 kernel source hashes. The [unchanged authored
two-floor case](evidence/plaincad-reconciled-3a4a37c8-supplemental-double-blind-step-2026-11-14.json)
used that same WASM and retained both floor positions through STEP and
watertight STL. Live OCCT readback of the browser's STEP reproduced the
reference volume, area, and centroid.
The [covered and deepened Hole reconciliation](evidence/plaincad-reconciled-407cb101-development-contained-crossed-holes-step-2026-11-14.json)
adds worktree commit `b3899732` to clean review revision `407cb101`.
Focused bridge tests check exact coverage, overlap and the transition from
blind to through. The live native tests, Go/WASM preflight and all 17 original
browser scenarios passed; 841 app and 1,966 kernel source hashes matched.
The [before-axis and at-axis STEP reconciliation](evidence/plaincad-reconciled-1de8d6fa-development-before-crossed-holes-step-2026-11-14.json)
adds worktree commit `8ecb989b` to review revision `1de8d6fa`. It passed
the native and live STEP checks, the Go/WASM preflight and 17 original
browser scenarios, verifying 841 app and 1,977 kernel source hashes.
Both [authored browser cases](evidence/plaincad-reconciled-1de8d6fa-supplemental-before-crossed-step-2026-11-14.json)
passed on that same WASM, and the independent OCCT reads reproduced
each browser STEP's reference mass properties.
The [disjoint finite Hole reconciliation](evidence/plaincad-reconciled-e0dc2756-development-disjoint-finite-holes-step-2026-11-14.json)
adds worktree commit `37eee35d` to review revision `e0dc2756`. It passed
the native OCCT checks and all 17 original browser cases, verifying
841 app and 1,982 kernel source files. The [authored two-floor case](evidence/plaincad-reconciled-e0dc2756-supplemental-disjoint-finite-step-2026-11-14.json)
also passed on the same WASM, and live OCCT confirmed the exported STEP.
The [far-tangent blind floor reconciliation](evidence/plaincad-reconciled-eda273e2-development-far-crossed-holes-step-2026-11-14.json)
adds worktree commit `1963370e` to review revision `eda273e2`. The native
and live STEP tests and all 17 original browser cases passed, with
841 app and 1,986 kernel source hashes verified. The [authored far-floor
case](evidence/plaincad-reconciled-eda273e2-supplemental-far-crossed-step-2026-11-14.json)
passed on that WASM and produced matching STEP and STL bytes; OCCT
independently reproduced the expected mass properties.
The [unequal finite bore reconciliation](evidence/plaincad-reconciled-6f2d43e1-development-unequal-finite-holes-step-2026-11-14.json)
adds worktree commit `d3f6fbd7` to review revision `6f2d43e1`. The native
and live STEP tests and all 17 original browser cases passed, verifying
841 app and 1,994 kernel source blobs. The [authored unequal-floor case](evidence/plaincad-reconciled-6f2d43e1-supplemental-unequal-finite-step-2026-11-14.json)
passed on the same WASM, and live OCCT confirmed the generated STEP.
The [triple and companion bore reconciliation](evidence/plaincad-reconciled-f12fd14a-development-triple-crossed-holes-step-2026-11-14.json)
adds worktree commits `42d13a60` and `bcda2835` to review revision
`f12fd14a`. Native and live STEP tests and the original 17 browser cases
passed; the verifier checked 841 app and 2,002 kernel blobs. Both authored
projects ([triple crossing](evidence/plaincad-reconciled-f12fd14a-supplemental-triple-crossed-step-2026-11-14.json),
[disjoint companion](evidence/plaincad-reconciled-f12fd14a-supplemental-crossed-companion-step-2026-11-14.json))
preserved their STEP and STL outputs on that WASM and passed live OCCT readback.
The [compact and partially overlapping finite-bore review](evidence/plaincad-reconciled-329fe4a3-development-partial-finite-holes-step-2026-11-14.json)
adds `b18f3b75` and `9a0f757a` to review revision `329fe4a3`. The
original 17 browser tests and focused live STEP checks passed; 841 app
and 2,008 kernel source hashes matched. In the [authored floor case](evidence/plaincad-reconciled-329fe4a3-supplemental-partial-finite-step-2026-11-14.json),
the browser preserved the blind floor through STEP and STL, and OCCT
independently reproduced the STEP's mass properties.
The [blind triple-crossing reconciliation](evidence/plaincad-reconciled-c130b571-development-blind-triple-holes-step-2026-11-14.json)
adds worktree commit `65441a2d` to review revision `c130b571`. The
native and live STEP checks and all 17 original browser cases passed,
verifying 841 app and 2,015 kernel source hashes. The [authored blind floor](evidence/plaincad-reconciled-c130b571-supplemental-blind-triple-step-2026-11-14.json)
survived STEP reimport and watertight STL; live OCCT matched the exported
solid's reference volume, area, and centroid.
The [latest complete production reconciliation](evidence/plaincad-reconciled-c0e81c4-production-2026-11-14.json)
passed all 66 original tests on clean review revision `c0e81c4`. It includes
the separate worktree through `8df00f2` and the more recent native
directional, line, distance, and exchange proofs. All 841 original
application files and 1,840 kernel and harness source blobs were verified.

The first gate is a real Go WebAssembly kernel adapter exercised through
PlainCAD's signed polygon extrusion, edit, save/reload and STL browser fixtures.
The application is staged separately; its original checkout is not modified.
Provider labels may be adapted explicitly to `plainsolid`, but geometry and
workflow assertions must remain intact. No OCCT or faceted curved fallback can
count as a PlainSolid pass. Existing Go tests and OCCT differential tests remain
supporting evidence, not PlainCAD end-to-end passes. The separate
`codex/plaincad-integration` worktree records selected Go/WASM browser passes
through scoped blind/through holes in commit `3d9bf1f`; that branch owns its
browser evidence and is not included in this checkout's kernel test results.

The integration branch also records the long feature chain (`64152c9`),
grouped edge treatments (`c5206db`), and finite-face sweep proofs (`86a0a73`).
A read-only merge preview against `0c09b62` found six conflicts: repository
instructions, three rectangular extrusion builders, and two long-edge
treatment builders. Reconcile shared builder behavior and face/edge ownership
on a separate integration candidate before merging. Run the kernel suite and
the affected PlainCAD browser scenarios on that same candidate. The two
working directories are kept separate while either has uncommitted work.
The integration branch later qualified finite To Face extrusion (`cf07586`).
Its subsequent working changes also touch the parametric persistence,
deflection, and bounds registration files changed on main. Preserve both
sets of registered analytic charts when reconciling those files, then run
native reload and certified mesh tests alongside the browser acceptance.

A later read-only preview of `cecb0f3` and `df1fe09` found 11 conflicts.
Reconcile phased helical starts and integration rounding bounds with the
existing strict constructors; retain the shared certified angular sweep
helpers in the polygon revolution code. The two branches also define the
same `certifiedSweepPi` helper in separate files, which a textual merge
does not detect. Keep one definition. Retain the grouped/optional edge
implementation with the original chain ancestry and qualify translated
STEP output. The integration worktree was left untouched.

An isolated candidate then reconciled `589411f` with `cc55825`, preserving the
registered analytic charts and both branches’ treatment and sweep behavior.
It compiled all packages and passed the targeted helical, treatment, and
translated STEP checks. Chromium passed the Go/WASM provider preflight and
existing finite To Face, native edge treatment, revolve/Boolean, and long
machining chain fixtures on that combined source. The candidate also adds a
bounded native display edge operation for the current PlainCAD adapter. It
samples native curves and suppresses periodic seams and sampled tangent joins.
The later `53d8432` integration snapshot adds circular rim fillets and machined
references. Its overlapping helical relief implementation needed a distinct
result type to retain the existing main constructor and both public operations.
The combined candidate also passed the three existing grouped circular rim
browser cases on XY, XZ, and YZ. The local `codex/reconcile-plaincad` review
branch retains this candidate and the independent `45110e0` production evidence.
These are integration candidate results; neither active branch was advanced.
The seven-case modeling and STEP browser gate plus the machined reference
case also passed on `d06a739`. The next reconciliation, with integration
`391651f`, brings the committed lead and bored-stock reference operations
into the candidate. The linked lead screw and nut browser fixture passed.
The external modeled-thread fixture reached STL export and failed the
nonadjacent triangle intersection check. The merged `eff3658` repair passed
external creation, edit, reopen, and STL on the combined branch. The full
original case then timed out during internal preview; the lead pair and all
eight viewport operation cases passed. A separate Node/WASM reproduction
found that constant-radius UV interpolation could exceed the annular domain
by one ULP. The committed fix resolves that reproduction. The browser trace
also showed repeated rebuilding of the existing external thread during each
step of the internal nut preview. With the committed stepped construction
speedup `048dd26`, both original helical browser cases passed on `80fac9c`
with unchanged deadlines and assertions.
The three bridge failures from the earlier broad candidate run passed when
rerun on the reconciled snapshot. The full short-mode Go suite then passed on
candidate `274540f`; later changes have separate focused checks. Both original
target-capture cases also passed after exact planar containment was connected
to the overlap query. The independent production baseline `65f053d` passed
61 of 66 original cases; its five failures remain separate from candidate
browser evidence. The original production fabrication case later passed on
`0955719` after integrating `72a778a`, which routes retained fitted boundaries
through the planar STEP writer. The previous bridge error returned quickly;
the stale UI status had made it appear to be a validation timeout. The
[production rerun record](evidence/plaincad-fabrication-reconciliation-2026-11-13.json)
retains the tested build identity and result. The production Acme gallery
case passed on the same build. UNC progressed past fractional pitch but
still needed an internal thread on rectangular stock. The combined checkout
`6736193` passed the original Cable guide, Acme and UNC production gallery
cases after incorporating the other worktree’s machining and thread repairs.
The [gallery retest record](evidence/plaincad-gallery-reconciliation-2026-11-13.json)
pins that production build. The isolated
controlled performance case collected all 25 edits and 11 STL samples, then
exceeded its 90-second overall deadline; the original test remains failed.
The subsequent clean compact-mesh snapshot `1e78c8a` passed the unchanged
original production performance test in 64.7 seconds. The
[focused performance record](evidence/plaincad-compact-performance-reconciliation-2026-11-13.json)
preserves the original deadline, WASM identity, and all samples. Full
production qualification on one final combined revision remains open.
On `acce666`, stale fabrication ZIP validation/native union and cancellation
of a pending STEP preview on same-ID project replacement both passed.
On `e73ed26`, both real-worker stale-result/error ordering cases and the
real STEP worker load failure and retry case passed.
Three further original development cases failed before their target actions on
candidate `13fd01a`. Hole Edit could not apply a chamfer after layered bores;
the display test could not rebuild its orbit housing; the retained-edge
preview case could not find its authored joined-stock edge. The real WASM
backend check passed for all three. The
[reconciliation report](evidence/plaincad-reconciliation-2026-11-13.json)
records these as fixture failures, not regressions in zoom refinement or
stale-preview cancellation.
The original Hole Edit case then passed with the isolated repair applied
as `96ab139`: the selected bottom chamfer replays both through holes and the
counterbore on the qualified bevel, and the browser reaches undo, save, open,
and STL. Exact complete-edge chain recognition and chamfer also pass the
original retained joined-edge preview case. The orbit fixture remains open;
an isolated lower-perimeter chamfer advances it to the raised enclosure join.

Further gates cover planar features and rectangular shelling; exact mixed
line/arc profiles and partial revolutions; cap-edge treatments followed by
bores, pockets and joins; helical threads and lead parts; and STEP roundtrips of
the resulting solids. General current-face/edge proof and browser resource
behavior are part of those gates, not optional integration details.
Polygon profiles can terminate against a finite planar face, including a
concave outline and holes. The Go operation proves complete projected coverage
and positive sweep height; PlainCAD must still resolve references and run the
browser acceptance case.
Signed partial polygon-meridian revolutions and bounded constant-lead
helical polygon sweeps are now available in the Go kernel. They still need
application acceptance. Selected convex profiles cut blind-ended grooves
in a cylinder. A pitch-matched helical seam retains multiple separated,
multi-turn openings inside one analytic wall chart. Arbitrary curved
meridian profiles, lead-in and runout, and complete lead screw and nut
qualification remain open.
The same bounded construction also cuts grooves on the inside of a tube
while retaining annular caps and an unchanged outer wall. A paired shaft and
nut constructor now checks exact clearance at both nominal flanks over the
shared radial band. The finite ridge proof resolves neighboring starts across
turn wraps. An exact joint radius/phase arrangement can prove no positive
axial overlap or report a witness across a full turn. The bounded finished
pair supports a reduced shaft tip and stepped nut reliefs outside the grooves.
Overlapping lead-ins and application qualification remain open.
Selected signed off-axis circular meridians produce analytic torus elbows.
The STEP polygon revolve writer qualifies signed partial sweeps and axis
contact; the closed display proof is opt-in. Convex polygon extrusion with
strictly disjoint circular holes now retains analytic cap arcs, walls and
selected blind-hole floors. STEP independently qualifies signed through
and blind cylindrical features. The strict circular extrusion STEP reader
reconstructs and checks canonical producer graphs; alternate OCCT placements
and pcurve conventions require a separate import path. Circular stock now
supports eccentric through bores and blind pockets, with a certified display
and an analytic STEP writer. Axis-aligned circular profiles can also terminate
against a parallel or supported oblique polygon face, or a recognized
round cap with circular holes. The exact polygon sweep also supports recognized
round caps. Mixed line and quarter-circle rectangular profiles retain analytic
rounded walls, through bores and blind floors, and have a certified mesh and
STEP writer. The tolerant analytic STEP reader separately verifies selected
independently authored polygon and round stock with through and blind holes.
The native PlainCAD long feature layout now combines horizontal fillet,
opposite chamfer, through bore, rectangular through-pocket and top boss.
Its mesh passes exact contact checks with a larger subdivision budget;
browser acceptance of this chain remains separate.

Bounded spline plane cuts retain exact cubic and quartic rational UV trims
when the homogeneous cutter residual is linear in the dependent coordinate
with a nonvanishing coefficient of degree at most two. Selected rational
supports have exactly affine projection and weights depending only on the
free coordinate. The stock and cutter keep their original supports, including
selected cuts across an original free-axis knot. The
native version 6 codec verifies continuous edge agreement and UV topology
again on reload. This still requires a certified graph, exact endpoints and
one active cutter plane. Bounded spline face and ray queries classify whole
enclosures against the resulting cubic and quartic trims.
The same graph proof now discovers a directed plane section of an embedded
rectangular spline shell without a cutter. It retains original face pieces
and exact source edge parameters, and keeps nonbinary affine endpoints
separate from display coordinates. Coplanar faces and unresolved contacts
are rejected. An optional proof projects the exact wires into an oriented
plane and groups simple, disjoint rings into material regions and holes.
Those regions classify points exactly and reconstruct independent planar cap
faces on an authored affine spline support, retaining each original section
piece and hole. In the certified rectangular spline domain, direct plane
split now sews both complete halves with opposite cap orientation and
separate original-edge versus section-edge ancestry.
Two parallel cuts can also form a band directly on the same curved stock,
including square-tube caps with holes and faces crossed by both planes.

Two same-side overlapping circular cutters have native rectangular-stock
constructors for equal or distinct radii, qualified rational crossings,
common depth, and signed coordinate frames. Separate exact off-center and
nonbinary-wall gates retain the original circle charts. Native version 7
reproves continuous edges with the actual stored homogeneous supports;
the old float spline path retains its lossless storage gate. Exact point,
axial section, line and first-hit queries distinguish exposed and hidden
supports, including both normals at the circle seam. Native documents
regenerate the stock and cuts; headless picking follows exact boundary events.
Picking can also return the original outward seam normals through exact affine
occurrence placements. The bounded display path proves distance, smooth-wall
normal angles, and closed embedding for exact homogeneous walls too, including
native reload and shared document occurrences. The default PlainCAD viewer,
STL and STEP workflow passed its supplemental equal-radius fixture on the
local review branch. The supplemental close-up passed on candidate `57f40ad`
with the original 20-second worker deadline and identical pick geometry.
The standalone Go/WASM mesh took 6.7–7.3 seconds after the final change,
compared with 29.1 seconds before both collision and boundary improvements.
Canonical model-only AP203 codecs cover the three lossless float-support
classes and separately register nonbinary external-radical walls. The latter
round trip through their own reader, with exact wall and floor lineage.
Vendor-rounded partial-cylinder pcurves remain rejected. Unequal disks with a
common center Y below the opening side now have a separate exact section and
native blind/through construction, exact point and line boundary queries,
and an independently checked model-only STEP round trip. A separate
different-center construction verifies an explicit rational join and its
excluded lower intersection. It supports native queries, certified display,
authored document regeneration, and registered analytic STEP readback.
Arcs extending past a semicircle and general cutter arrangements remain open.
Both common-offset PlainCAD workflow fixtures passed on candidate `2851e23`.
The different-center blind workflow passed at `50dd15f`; the through workflow
passed at `1c1c70d`, retaining the same eight-second inactivity watchdog after
the certified mesher gained an intermediate rational rim sampling count.
Exact model spline and surface-composed curve queries order isolated plane
and finite line events, retain coincident plane spans, and classify matching
knot-side germs. Strictly affine line coincidences have exact finite clipping.
The display mesher certifies affine faces with a bounded chain of adjacent
cubic or quartic graph trims, including C0 joins and shared sample indices.
Nonbinary exact supports participate in complete certified transverse section
traces and standalone algebraic section persistence, including original
trim clipping. A closed two-face shell intersection preserves its exact Bézier
roof through imprint, original-face selection, volume bounds and native
reconstruction. Exact support queries cover face membership, bounded line
contacts, distance, regularity, and material location on rectangular shells.
Independently refined wrapped charts now sew constant
whole-period UV line shifts. A bounded partial coincidence and arrangement API
retains exact original lifted intervals, materializes original-support faces,
and proves immutable open union, intersection and directed difference with
original edge ancestry and oriented hole cycles. Lifted rectangle injectivity
now spans a true fundamental seam using one exact global projection proof.
The opt-in wrapped face arrangement and open region operators use that proof
and keep original edge evaluators across the seam. Multi-image periodic
assembly remains a separate gate.
The shell imprint now keeps disjoint closed sections on the same source face
when exact chart boxes prove that they do not touch or nest. A two-lobe roof
has two closed common components and can return two independent native solids
with continuously certified paired trims. General mixed and nested arrangements
remain open.
Qualified shell Booleans now classify represented material points and finite
line transitions and bound volume, area, centroid and inertia on the original
charts. The certified indexed display shares vertices between opposing
coedges and rejects unindexed contacts. Two-face graph lenses also have a
closed certified fill. A rotated shell section with unequal graph rectangles
proves coverage on the full source charts after clipping only the seed search.
The cylindrical quotient proof handles two physical intersections and a
connected annular union, with separate native open topology, original
parameter lifts, and continuous edge proofs for representable vertices.

Exact finite line and first-hit queries cover transverse finite cylinders and
sphere/cylinder union and subtraction. They report algebraic parameters and
all exposed outward support normals, including simultaneous contacts and
reversed cutter normals. Regularized two-sphere point, line and first-hit
queries distinguish empty,
contained, coincident and singular touching sets. Query support does not make
every touching set a manifold solid.

Independent standalone exchange codecs support exact signed-coordinate
cylindrical supports in IGES 192 form 1 and STEP, with explicit units and no
inferred trims or material. Standalone exact planes use STEP and IGES 190
form 1, and spherical supports use STEP and IGES 196 form 1 without adding
seam or pole topology. Converters require lossless frame, unit and metadata transfer.
The separate IGES/STEP extrusion converter
retains exact fraction-to-distance parameter scaling and rejects metadata or
normalization that cannot be preserved.
Ordered mixed support imports and direct transfers cover planes, cylinders,
spheres and ring tori; a strict native envelope preserves these leaf records.
A model-only STEP reader also accepts a bounded four-face split cylinder,
proves both semicylinders cover the original cylinder, and retains source
face and edge IDs. The canonical native cylinder does not retain the artificial
split. The rooted dialect supports exact import and a canonical native
cylinder export with one independently verified OCCT product root.
An independently authored OCCT STEP with off-circle binary seam
coordinates remains outside the exact reader.

| Plan phase | Status | Delivered | Still required for phase completion |
|---|---|---|---|
| 1: Foundation | Implemented initial domain | Go module/CI configuration; vectors/transforms; rational curves/surfaces; meshes/primitives/extrusion; bounded exchange; box Booleans; end-to-end example | Remote multi-platform CI execution and longer validation campaigns |
| 2: Geometry algorithms | Partial | Exact-sign predicates, analytic conics and selected intersections, exact raw-plane traces, spatial spline/segment contacts, bounded rational subpatches and rational evaluation, certified spline-pair coverage, curve/surface distance and length bounds, exact chart identity across spline bases, curve/tensor/scattered fixed-basis fitting, uniform periodic curves and wrapped spline curves/surfaces with exact seam proofs | Fitting with unknown parameters or weights, full periodic surface editing, more curve and surface pairs, production numerical envelope |
| 3: B-rep | Planar and curved subsets | Shared topology and shell/compound access; planar holes; exact analytic patches with seams and poles; rational quadratic UV topology certificates; continuous planar, spline-section and selected analytic edge agreement; anchored and closed certified spline-tube face splits; exact sewing of spline rectangles and selected analytic periodic faces; revolutions, circular sweeps, elliptic station lofts and affine maps | Certified arbitrary trim validation, general loft and sweep, richer topology history |
| 4: Booleans | Restricted subsets | Rational nonconvex planar BSP, Boolean face and edge ancestry, oblique splits and section loops; orthogonal cells and common-height prisms; sphere, parallel-cylinder and coaxial sphere/cylinder Booleans; oblique cylindrical bores and spherical box pockets; exact quadratic plane tangencies and subdivision-corner reconciliation; certified spline–plane sealing of single open, disjoint or nested closed contours; open and closed sealing of two spline graphs and shared-base graph Booleans across open, closed, nested and mixed contours; spline-pair tubes, mixed-axis open/closed branches, paired trim evaluation, automatic discovery and independent tube coverage checks | General curved operations, complete curved branch discovery, curved coincidence/tangency assembly and complete topology history |
| 5: Advanced modeling | Restricted subsets | Planar sewing with remaps/displacement; exact same-domain planar face unification with retained/deleted edge intervals and vertex locations; faceted shell promotion; convex mitered offsets and chamfers; rational square-metric offsets for orthogonal footprints; rounded rectangular bodies, selective unequal-radius convex prism fillets, draft prisms, affine support edits, two-distance edge chamfers, planar sketch protrusions/pockets, convex hollowing, support removal, and cylindrical, spherical and toroidal thickening | General fillets/offsets/thickening, draft, local edits, wider sewing and repair |
| 6: Tessellation/queries | Partial | Conforming planar and curved meshes, derivative bounds for supported analytic and clamped rational spline surfaces, curved UV-trim refinement and certified drawing chords on spline model edges, exact planar boundary distance and point projection with BVH pruning and exact point location, exact mesh contact classification, signed trim-strip mass/inertia integration, analytic trimmed point and ray queries and exact unit-chart spline graph classification, certified spline line contacts, single-shell embedding proof for selected spline rectangles, exact planar assembly embedding and nesting, mesh BVH | Guaranteed arbitrary curved trim bounds, general exact-shape queries and curved embedding beyond certified subsets |
| 7: Interoperability | Restricted CAD and mesh subsets | Faceted and planar advanced STEP, convex planar STEP cavities, body and face colors, conversion-based units, analytic primitives, tubes, stepped shafts and mixed cylinder/cone profiles, spherical caps and zones, cylindrical/conical sectors and torus elbows, cylinders with an oblique cap, rigid planar STEP assemblies with product-local face colors, STEP conic/spline geometry, bounded rectangular and orthogonal spline faces and selected certified closed spline shells, IGES analytic and spline geometry plus planar types 142/144, native topology including verified implicit spline–plane and spline-pair tube recipes and opt-in exact planar embedding checks, OBJ/STL/PLY, embedded glTF/GLB scenes | General curved STEP B-reps, broader IGES, wider assemblies/metadata, additional glTF and PLY features |
| 8: Documents/presentation | Partial | Mixed assets, typed labels/attributes, bounded feature graph regeneration, STEP occurrence import/export, shared assets and planar body colors, versioned snapshots, history, orthographic/perspective projection, picking and mesh-occluded edge spans/SVG, stable box face roles for bores and planar sketch features; colored STEP document adapters | General stable topological naming, broader feature dependencies, exact curved occlusion and silhouette extraction, optional interactive rendering |
| 9: Qualification | Initial differential cases pass | Locally built pinned OCCT, primitive and restricted Boolean comparisons; independent STEP and IGES qualification; Go unit/race/vet and focused fuzzing | Broad differential corpus, platform CI execution, adversarial numerics, API/packaging/license decisions |

## Test evidence and limitations

Certified rectangular spline shells support regularized Booleans for strict
nesting, strict separation, and planar interfaces that are full faces, inset
rectangles or partial overlaps of aligned affine rectangles. The join preserves
curved exteriors and propagates exact splits to adjacent edges. Graph Booleans
accept different degrees, knot partitions and positive rational weight fields,
with exact normalization of compatible affine charts. A closed transverse
contact strictly inside one face of each embedded spline shell can be split,
classified and sewn once all other face pairs are proved disjoint. Multiple
closed contours use exact winding to classify nesting separately in both
charts. Separate fixtures qualify disjoint multi-contour assembly and nested
contour classification. Multiple transverse wires crossing original shell
edges can be assembled from verified face-pair tubes and exact terminal hints.
The arrangement splits original edges at every proved crossing, follows open
paths around fragments, nests closed loops and records retained face and edge
ancestry. A fixture exercises one closed and one open branch on the same face
pair. The dispatcher can discover wires with affine charts and selected
sections against an affine plane, including exact raw oblique support planes.
Plane fallback can use exact trace-only knot cuts; exact iso-curve clipping
handles selected exits on the affine chart. Callers can retrieve the exact
section graph or partition both original boundaries without removing material.
An opt-in general tube fallback recovers selected rational free-coordinate
terminals from independently certified enclosures. A nonplanar face-pair
fixture proves a curved branch and both terminals without supplied seeds.
An exact planar arrangement handles simultaneous edge events and coincident
contacts for wholly affine spline boundaries while retaining original supports.
For a simultaneous represented spline edge event, exact jets in all four
incident charts distinguish the local face-pair continuations. Boundary
anchored tubes and an exact isolated-point exclusion support selected
regular simultaneous crossings on both shells. A common-XY graph
fallback can discover a closed branch between two genuinely curved graphs.
An opt-in exact vertical quadratic unwarp can infer a common warp coefficient
and propose simultaneous crossings on a restricted curved shell family;
proofs and results retain original supports. General curved edge-edge assembly, tangential touch-and-return
arrangements, singular contacts, arbitrary trim input embedding and general
branch discovery remain open.

Wrapped rational spline supports have exact seam position and first derivative
proofs, bounded knot insertion and reparameterization, canonical seam sewing,
and shared seam edge splitting. Plane contours can reconnect at algebraic seam
roots and at certified transverse doubly wrapped corners. Native reload
revalidates seam recipes. Canonical tensor rational toroids have an independent
quotient embedding and outward-orientation proof. STEP can exchange that
specific material solid and standalone wrapped spline surfaces and faces.
Disjoint rectangular and canonical wrapped spline components can be combined
and reused. Strictly nested component Booleans preserve cavity orientation
and material islands, with exact box-in-circular-frustum and coaxial-profile
containment in addition to rectangular spline pair proofs. Affine spline
shells can also be split by a represented oblique
plane without changing retained support charts. Nonlinear spline edges with
a proved affine model-space projection have certified contact brackets in
both original parameter units; general edge-edge Boolean assembly is open.
Clamped rational p-curves use exact homogeneous trim topology validation.
Degree one and two spans are supported directly, and degrees three through
eight require a certified monotone projection. Face splits preserve certified holes when
each hole's resulting fragment is proved. Opt-in certified meshing covers
selected curved spline polygon trims with holes and whole closed shells whose
straight trim boundaries admit common exact dyadic samples, including
adaptive refinement. Rational quadratic trim polyline proofs retain exact
UV and model joins, original trim ancestry and two-sided chord bounds.
Selected curved trims with holes have certified filled-region distance and
triangle coverage on affine and bounded multi-span rational biquadratic
supports. Exact knot clipping and a piecewise Jacobian bound preserve
the certificate across C0 knots.
The curved proof transports an exact UV fill bound and independently
bounds triangle interpolation. Selected quadratic shared seams have exact
model agreement and synchronized samples, including exact affine
correspondence between degree-one and degree-two trims over distinct
active ranges. A closed quadratic trim shell stitches explicit corners
and proves all vertex links. An opt-in exact BVH check independently proves
embedding of the represented display mesh, separate from smooth-shell
embedding. A multi-span pair and collection stitcher consumes exact
nonbinary knot cuts on selected rational linear UV seams; the curved
model edges retain shared display vertices and recertified distance.
Native exact rational quadratic controls support bounded region
tessellation with holes, and selected quadratic trims on different
supports share synchronized rational cuts before triangulation. A bounded
closed-shell stitcher synchronizes native curved seams across multiple faces
and proves display connectivity, opposite edge incidence, all vertex links,
positive signed volume and exact absence of non-incidence triangle contacts.
It does not infer continuous source embedding from the display. Arbitrary
support and trim meshing remains open.
Supported trimmed spline distances retain exact lower bounds and valid upper
witnesses even when trim ambiguity or a work limit prevents convergence.
Canonical wrapped rational cylinders and frusta have exact caps, seams,
material location and line queries. Axial and concentric cylinder Booleans
support through-bores, blind bores and closed cavities. STEP independently
qualifies capped solids, frusta, tubes, blind bores, shoulders, cylindrical
cavities, toroidal voids and circular station lofts. The station loft
preserves shared crease rims and supports exact material and finite-line
queries after independent native topology validation. Coaxial equal-height
station loft Booleans support radial profile crossings and strictly nested
through subtraction, with original wall and cap ancestry. Circular wrapped
toroids have exact quartic line queries and Euclidean normal offsets.
Circular frustum lateral normal offsets are exact when the normal scale is
rational; the trimmed variant caps the shifted rims. Selected circular
cylinder end rims have exact toroidal rolling blends. An ascending conical
lower rim also accepts a rolling ball when the cone normal and torus trim
parameter can be represented exactly. The original cone and cap support
ancestry, full-circle tangency and native reload are verified. Complete circular
cylinder offsets move both caps and the lateral wall in the Euclidean
metric; supported existing blends remain concentric, and outward offsets
round sharp rims. Scene picking uses exact capped frustum, circular
toroid, station loft, loft bore and rounded cylinder contacts on opt-in
inputs. Rounded cylinder distance returns an algebraic witness; station
bore line queries include annular cap intervals and their rim ancestry.
Station lofts and bores also have exact horizontal sections and bounded
nearest-face witnesses from the original outer and inner walls and caps.
Horizontal cuts split both closed station lofts and bores, preserving
uncut supports and independent inner and outer ancestry. Document picking retains occurrence IDs and
requires exact translations; ambiguous depth ordering is reported. Exact
scene selection also supports shared-base graph envelopes independently
of whether a native trimmed solid can be assembled.
Document recipes support wrapped stock cylinders, drafts and bores with
stable exterior face roles, along with circular station lofts and capped
frustum normal offsets. Station bore recipes retain inner/outer wall roles
and cap roles through regeneration and persistence. Shared-base graph Booleans can recover an
isolated higher-degree contact or a strict height margin from exact proofs
when regular section tracing cannot finish. A quadratic saddle over the
unit chart can be split into four exact sectors for union and intersection
when the saddle and terminal vertices are represented. Algebraic terminals
retain brackets but cannot yet be sewn into this Boolean result.
For knot-aligned partial coincidence, exact sign and perimeter certificates
support union, intersection and selected subtraction with closed islands or
holes, retaining original support and contact ancestry. Each maximal
coincident region becomes one face with a cell-to-face map. Subtraction rejects
zero-thickness internal seams, isolated contacts and pinched fans. Exact
non-knot axis-line contacts support all three operations when their rational
coordinate is representable; nonbinary coordinates are reported as evidence.
This includes higher-degree differences whose common axis factor has a
strictly signed quotient. Parallel contact lines and rectilinear grids
preserve signed bands or cells; oblique contact works across different
source knot partitions, including two parallel or crossing oblique lines.
Crossing subtractions that pinch are rejected. Axis-aligned and selected
rotated elliptical contacts support exact rational p-curves, disk lenses
and annular graph differences. Native version 4 preserves nonbinary
homogeneous trim controls for represented endpoints. Version 5 preserves
exact rational UV and model identity at nonbinary endpoints, derives the
display vertex by rounding, and rechecks exact loops, complete shared edges,
and model endpoint identity on reload. Exact rational splits and inverse
merges preserve the shared topology. A rational-point conic whose normal
quadrant stations are irrational can use an alternate exact rational seam
partition and produce native union, intersection or difference.
An additive algebraic seam subdivision format preserves ordered irrational
splits in both coedge loops without changing the original supports. Native
reload verifies the plane, graph support or paired affine-edge constraint
and reconstructs both chart domains.
It preserves different roots even if their represented coordinates agree.
A two-fragment straight UV arrangement uses these exact vertices without
rounding incidence. It retains the complete model curve on the original
spline, including separate knot cells, and saves/reloads several face cuts
as one closed arrangement. Exact contact between its generated edges keeps
both original algebraic parameter identities. Affine fragment fills prove
represented polygon coverage and source distance. Bounded curved fragment
fills transport exact UV boundary bounds through rational spline supports,
including a C0 knot crease. Selected fragments can be sewn across independent
arrangements with complete curve equality and verified vertex links.
A vertical-plane split of a certified conic lens proves material selection
on both sides, exact cap ownership, point and line queries, and rational
volume bounds. Both halves can share one certified rim and cap boundary
network. Selected independently curved graphs, including a common C0
knot crease, also produce closed display meshes with exact represented
coverage, positive oriented volume and independent collision checks.
General curved material classification and sewing remain open.
Continuous edge equality can be proved across independent algebraic knot
events even when the curve pieces cannot be exported as rational cuts.
Plane and finite-line contacts retain algebraic knot identity and original
cell owners. The certified polyline shares one rounded vertex at each knot
and includes its error in both incident segments. A spline face line query
tests the whole root enclosure against the trimmed region, including holes.
An exact projection path proves vertical shell-ray crossings on selected
identity graph faces with native rational trims, leaving shared seams explicit.
An independent curved-edge to spline-face isolator proves regular crossings
with original edge ownership and filters whole UV enclosures against trims.
The two-sided shell scan can propose rational terminal coordinates, which the
tube solver verifies independently. Selected graph contacts also retain exact
irrational roots, tangency multiplicity and one-sided signs. General algebraic
terminal sewing remains open.
Supported regular surface tubes can be clipped against two certified sets of
quadratic trim loops, including holes and distinct active knot spans. The
clip groups equal algebraic free parameters, retains original coedge and
knot lineage, and certifies each intervening interval by separating an exact
UV enclosure from the original loops. A complete supporting face-pair trace
can then report trimmed runs and isolated tangent contacts. It can persist
the source charts and tubes and reconstruct the section without stored
incidence. An open run with two transverse boundary crossings partitions
one or both injective faces into exact loops on their original supports.
Closed cuts prove their original UV winding. Existing holes strictly
separated from either kind of cut remain intact and retain their owners.
A single regular loop on two convex affine-XY graph charts can bound an
exact two-face lens with oriented shared edges, material line queries,
rational volume and centroid intervals and verified native reload.
The ordered-coordinate graph path also discovers closed sections in XZ and
YZ on independently oriented charts. A selected XZ lens seals and reloads
with original supports, outward orientation, exact finite-line and plane
sections, boundary-distance bounds and original XYZ mass bounds. Closed
display proofs preserve both owner charts and positive physical orientation
for XZ, YZ and ZX.
Disjoint closed loops assemble into separate exact shells. Independently
scaled and reversed chart tracing now passes full coverage, and a true C0
knot crossing has automatic one-sided tube discovery and exact native
anchor reconstruction. Closed display meshing synchronizes both face
owners and their interior refinement, including a crease across the cut.
The rational concentric quartic case produces an annulus and inner disk,
proves their separate material queries, bounds each volume and reconstructs
both on reload. A certified closed annular display shares both rim networks
and passes the represented mesh embedding check. Separate exact anisotropic
quartic discovery proves full coverage of two nested elliptical loops.
Exact dyadic cut guidance discovers a separate nonfactorized quartic loop
without caller-provided cuts. The lattice only proposes cuts; the exact
contour graph and independent tube coverage decide completeness.
Shared-base graph subtraction keeps only certified
positive-thickness components, including selected exterior wall regions.
The lens also has exact point, line and bounded boundary-distance queries.
Native graph union and intersection also retain both original top charts
around one closed interior section, with four original domain walls and
the common base.
Coincident face trim spans and unsupported projections remain unresolved.
The whole-shell section and subdivision path verifies a four-branch wire
with irrational intersections on two original edges. Each split is shared
by both coedges and persists through native reload. A separate warped-shell
fixture uses bounded curved graph constraints to reprove its split edges.
For this regular rectangular-shell subset, exact imprints preserve each
original support and split coedge, then classification and sewing assemble
all three Boolean operations as an algebraic boundary. A native recipe
reproves the result. The volume integrator accounts conservatively for cells
that overlap the section cover. General irrational Boolean topology and
conversion into the float-only `Solid` remain open.
The shared-base Boolean dispatcher now also proves the single isolated
contact of selected coupled quartics with zero Hessian. It preserves the
selected original top in union/intersection and diagnoses a pinched result.
An exact straight-section report retains nonbinary terminal coordinates
and the original curve pieces. Shared-base envelopes support exact point
and vertical line queries even when the result cannot be stored as a shell.
Nonvertical finite-line queries prove isolated original graph and selected
envelope contacts, including a shared irrational root across different
original splines. Exact side-wall queries and an interior line clipper return selected
material intervals. A separate coincidence query returns retained
top/base intervals with original owners; overlapping event bounds that
cannot be resolved within the proof budget remain errors.
Planar sketch features support holes,
including annular blind pockets and through cuts with disconnected material.
Document adapters retain named, unit-aware spline shell assets and shared
rigid STEP occurrences. STEP retains product-local body and face colors and
supports standalone concave orthogonal spline faces with exact iso-curves and
up to 16 orthogonal holes. The connected open shell adapter verifies shared
curves, incidence, and vertex links; global embedding is not established for
general open shells. Closed STEP affine spline shells also have a separate
reader and writer with exact planar embedding and orientation proofs.
IGES supports orthogonal rational spline faces with paired verified model
and parameter curves and up to 16 disjoint orthogonal holes, and selected
affine faces with curved rational oval boundaries and holes. IGES can verify
coincident model curves between a holed face and separately charted fill
disks; it reports those seams without asserting shared IGES topology. Both formats
retain strict rectangular APIs. STEP also exchanges these bounded oval
faces, with a separate reader for independently partitioned quarter arcs.
Mixed wrapped STEP supports twelve independent body families and explicit
rigid product occurrences, including repeated products and multiple bodies
per definition. An additive style API retains default body colors and
explicit uniquely named face colors on shared product definitions. The separate document adapter validates native asset
geometry and provenance before writing these families. Closed rim fillet
STEP exchange also verifies each complete quarter-toroid chart and its
trim ownership independently. The standalone station-bore STEP adapter
independently validates multi-band outer and inner walls with shared
annular caps, including different station partitions. A separate annular
frustum adapter handles two single-band walls when one or both taper.
Bounded nonplanar oval STEP faces verify exact UV-to-model composition on
a rational biquadratic support, including full quartic model curves and up to
16 disjoint oval holes. A connected annulus and fill disk with distinct
nonplanar charts share four authoritative model edges with both source
p-curves and opposite uses; this qualifies connected open seam topology.
Closed spline lens STEP exchange preserves both original graph supports and
proves exact material orientation. Its document adapter preserves the local
lens and its roles through native save and reload. Separate shared products
can use exact signed-axis rotations and translations without flattening charts.
Selected sheared conic lenses preserve their original supports through STEP
and document roundtrips, with exact representability gates.
Independent knot partitions are retained when every seam quarter fits in one
source knot cell, with original homogeneous composition rechecked on import.
The separate partitioned lens adapter also splits quarters at selected
representable rational knot crossings. It records every source piece's
parent interval and owner knot cell while retaining canonical native material.
The shared partitioned lens assembly retains a single native asset for a
product, all six source edge records, and separate rigid occurrences.
An independent refined-curve mode preserves different knot bases on each
authoritative model edge and its two owner p-curves. Native recipe reload
and the shared-product adapter reprove continuous agreement and retain all
original bases for supported exact weights.
A separate raw-weight mode also preserves independently refined support,
model and owner p-curve bases when binary normalization would change the
exact geometry. Its seven-edge lens fixture passes independent exchange
in both directions. The shared assembly and native document recipes also
retain product identity, names, units, body color, face colors and exact
rigid placement.

IGES can preserve selected nonplanar biquadratic oval disks and annuli,
including complete paired rational p-curves and model curves. The connected
sheet mode certifies opposite continuous use of the shared curve on distinct
supports. A strict topological reader preserves an explicit open type 514
shell with eight source edges and vertices and opposite shared edge uses.
Global embedding of arbitrary connected sheets remains separate.
A separate closed IGES spline lens proves the material and all four paired
model edges before accepting or writing its closed shell and solid records.
Its raw-weight document adapter preserves the original model and owner
curve bases, source knot cells and entity IDs through native reload and
checks the exact geometry and source provenance again before export.
The separate partitioned IGES lens preserves six- and seven-edge source
graphs with exact rational cut intervals and independent owner knot cells.
The affine partitioned IGES path also qualifies a rotated original trim
basis with unequal semiaxes and exact represented model coefficients.

The overlap classifier verifies selected finite-area coincident subdomains
in different rational graph bases. It compares nonbinary UV endpoints and
independent knot grids exactly. Ordered model projections retain exact XYZ
corners; a separate single-cell proof returns exact UV polygons under shear.
The ordinary-node proof also identifies the two paired germs of a line and
parabola, with no other branch on the qualified graph rectangle.

Closed certified mesh paths now cover canonical wrapped frustums, exact
cone poles, nested wrapped bore walls with independent taper, and selected
multi-station loft solids. They preserve shared rim/station topology, bound
both directions of source-image distance, and check triangle embedding.

Unit tests include analytic mass properties, rational circle and Bernstein
oracles, randomized spline edit/affine invariance, topology incidence and vertex
fans, malformed parsers, short I/O, resource limits, assembly rollback/conflicts,
and representation immutability. Independent voxel classification tests check
box Booleans, volumes, centroids and boundary mesh volume. Independent brute-force
geometry checks validate BVH queries. Regression tests cover extreme ray scales,
composed-transform underflow and repeated unclamped spline endpoints.

The bracket integration test constructs a union, subtracts a through-slot,
validates its mesh, exports and imports binary STL, and compares volume with the
independently calculated 7520 cubic units. Curved mesh sampling and sampled curve
chord refinement are approximations; they do not certify arbitrary surface error.
Topological manifold checks do not detect geometric self-intersections; the separate collision package checks represented triangle contacts.

Focused fuzz runs are smoke campaigns. Remote CI is configured but has not been
executed here. The pinned OCCT build independently validated primitive and
sphere Boolean mass properties and exposed a STEP import failure that was
corrected by writing explicit planar surface entities. Both-direction IGES
curve qualification also passed. See [verification](VERIFICATION.md) and
[oracle instructions](../tools/occtref/README.md).

Finite coaxial sphere/cylinder common now retains the applicable
cylindrical band, spherical caps or zones, and planar clipping caps.
Subtraction supports a contained cavity, a blind or through bore, and
cuts that separate the sphere into two native components. Union retains
annular shoulders at the ends of protruding cylindrical stock and preserves
disjoint components. The nonpolar algebraic common operation encloses the
ideal volume and supplies explicit seam bounds for irrational trims. A
separate finite blind bore supports exact and algebraically enclosed rim
heights. Exact off-axis angular contact classification distinguishes
smooth loops, isolated tangency, and a crossing node; general off-axis
trim assembly remains open. Fixed finite cap events have an exact zero,
one, two or coincident-circle classifier. Finite off-axis contact graphs
preserve cap ownership, isolated points, and four-germ tangent nodes.
The off-axis blind-bore shell covers contained round cutters with footprints
that include or miss the sphere axis. It retains native analytic faces and
has a bounded display and native reload. Exact point and line queries
support the same material, including coincident floor and wall intervals.
The off-axis through-bore shell retains both analytic rim loops and its
inward cylindrical wall, with corresponding exact point and line queries.
Finite off-axis intersection now assembles full planar or spherical ends
when the cap planes do not cut part of a curved rim. A separate native
partial-cap construction handles one transverse upper lens and a full lower
disk, even when the upper lens excludes both circle centers. Exact cap and material point/line classifiers
cover broader partial cuts. Two transverse partial caps on opposite sides
of the equator now have native shell assembly, including unequal lens radii.
The same-side case uses a common pole inside both lenses and preserves
the spherical zone with only the exposed cylinder wall sectors.
Finite common dispatch now selects these native cases from exact cap
material, including a full spherical end opposite a transverse lens.
The dispatcher also recognizes a cylinder covering an entire sphere slab,
including tangency and algebraic planar radii next to an uncut pole.
Finite difference dispatch similarly selects enclosed cavity, blind bore,
through bore, unchanged or empty outcomes where the finite limits prove
those constructions apply.
Finite off-axis union also retains a protruding cylindrical boss with one
buried cap and one fully exposed cap.
General curved fragment assembly remains open.
Perpendicular intersecting cylinders now have exact four-cap contact branch
and material queries. Native intersection retains both cylindrical supports
for equal and unequal radii when no finite end clips the result.
Skew orthogonal cylinders retain both supports for nested or strictly
overlapping radial projections. The exact graph also covers isolated tangency
and skew tangent nodes. Oblique intersecting equal-radius cylinders now
have a native common with two certified bisector seams when all caps clear
the intersection. The general oblique chart also assembles unequal radii
and unequal raw axis norms with the same strict cap-clearance requirement.
Selected oblique skew placements also retain both circular supports when the
exact section map and axis separation are representable.
A separate perpendicular shell supports an exact
equator cap or a positive cap with a representable rim ordinate.
Signed algebraic caps, two-cap slabs, and exact caps on both perpendicular
axes are supported in bounded cases. Equal-radius perpendicular finite
cylinders also have a native union with all four exposed cap disks.
Equal-radius oblique cylinders have native union and subtraction when the
finite caps strictly clear the common. Perpendicular and oblique subtraction
retain two closed bodies and report their two intrinsic closure contacts.
Subtracting a wider perpendicular or oblique tool keeps two strictly
separated stock bodies, with an exact squared lower bound on their gap.
Finite cylinder common now dispatches across the supported unclipped, clipped,
parallel, and empty families. Exact axial overlap handles reversed and scaled
coordinate-parallel axes; clipped perpendicular caps accept signed and scaled
coordinate axes without changing raw cap ownership. Six independent OCCT cases
check the dispatcher’s geometric results. The native JSON recipe retains the
raw inputs, construction choice, face ownership, and canonical boundary.
Perpendicular equal-radius pairs now support two, three, or four interior
clipping caps with retained cylindrical and planar charts. Exact containment
also retains the original smaller cylinder in supported signed-coordinate
perpendicular placements, including a skew axis gap. Unequal radii also
support finite axial windows when the axes cross perpendicularly in signed
coordinate directions. Parallel oblique cylinders can retain a native common
when the similarity basis, cap positions and transverse offsets are exactly
representable. Independent OCCT primitives verify these mass properties.
Finite union and difference retain selected separated and fully contained
coordinate-parallel stocks. A narrower perpendicular cylinder can also cut
one connected through-bore through wider stock. That path requires signed
coordinate axes crossing at a representable center, both caps of each cylinder
beyond the opposite radius, and an exactly representable shoulder. The native
result preserves the two stock caps and an inward cylindrical bore wall.
A smaller coaxial cutter covering both stock caps also retains a native
annular body for signed coordinate axes, including equal cap locations.
The same construction accepts a parallel eccentric through-bore when the
exact transverse disk gap and axial coverage prove a fully enclosed opening.
An axially and radially enclosed parallel cutter instead retains a separate
inward cavity shell, with cutter wall and both original floor IDs.
A contained parallel cutter reaching exactly one stock cap retains one
eccentric blind bore, including the original floor and cap ordering.
Equal-radius coaxial stock with overlapping or coincident end spans now
joins as one cylinder, retaining the original exposed caps and wall owners.
Unequal coaxial stock preserves an analytic step where the larger disk ends,
including annular cap ancestry and exact common volume.
Coordinate-parallel cylinders with identical cap planes and a transverse
disk crossing retain the analytic union or cut with signed cap ancestry.
A transverse cut also accepts an axially longer parallel cutter that exactly
covers the stock span. Only original stock caps and the original cutter wall
appear on the retained crescent.
One-ended cuts and interior side notches keep native lens floor patches on
the original cutter cap planes. Exact axial ordering retains the original
stock caps and the cutter wall across all three represented orientations.
With transverse disks and positive axial overlap, union also retains
native shoulders for one common cap, staggered spans, and nested spans.
An exactly shared terminal lens has both original cap owners.
Exact represented similarities extend these transverse cuts and unions
to qualified oblique parallel axes without reassigning source cap IDs.
Exactly abutting transverse cylinders also join across a positive-area
cap lens. The internal lens is removed from the boundary. An exact point
query on the constructed set distinguishes exposed faces from canceled
source incidence, including interior lens points on the old cap plane.

STEP inspection for the retained off-axis bore now verifies its oriented
incidence graph, exact one-turn winding, both continuous spline lift bounds,
and source rim endpoint gaps. Separate exact decimal support residuals cover
the floor circle and cylindrical seam. These reports preserve the source seam
mismatch and uncertainty metadata; the STEP importer still rejects this shell.

## Continuing implementation

The remaining rectangular Hole geometry includes near-contact unequal-radius
exchange, interacting oblique axes, unequal-radius floors inside their
intersection, and broader mixed-axis cutter groups. The selectable app
build passes its unit and browser gates at the pinned provider revision.
Later geometry increments record their own focused regressions.

The two short-mode exclusions for nested quartic material and transverse
shell operations passed separately earlier. The opt-in run for
`TestQualifyNestedQuarticShell` and `TestStraightTubeMassInvestigation`
also passed in 133.593 seconds. Its JSON log is
`/tmp/plainsolid-integration-native-two-opt-ins-20261113.jsonl`.

For immediate work, follow the PlainCAD gates above. Within the remaining full
replacement scope, the critical path is general curved Boolean assembly. The current exact planar
BSP can serve as an independent topology oracle for planar cases; curved
surfaces must keep their geometry through the operation. Continue with:

1. Expand exhaustive tube discovery and branch enumeration over independently
   parameterized splines. Mixed-axis junctions and tube-union coverage are
   available as proofs; automatic discovery of many closed curves, tangent
   candidates and partial coincidence beyond the shared-graph knot arrangement
   still require work. Signed affine graph pairs with partially overlapping
   projections can now prove exclusion outside the exact overlap and verify
   branch coverage within it in XY, XZ, or YZ. A separate spline/plane path
   proves open branches against the original rectangles and preserves exact
   endpoint ownership. Native shell assembly consumes those open sections,
   preserves simultaneous source-edge ownership, and independently rejects
   omitted contacts. Overlap-guided closed contours now seal single and
   multiple native graph regions after rechecking coverage on the original
   supports. Extend complete transverse arrangements beyond rectangular input
   supports and handle curved edge-edge crossings, tangencies and coincident
   intervals. The algebraic imprint now assigns closed loops to the exact
   fragments of proper open cuts on the same original face. This uses strict
   separation from conservative tube boxes and an exact winding proof, then
   independently checks both original shell incidences. It retains the input
   charts through common, union, cut and reload. Exact Bernstein signs also
   certify isolated corner contacts
   of supported affine graph pairs; use that evidence for source vertex
   ownership before accepting transverse vertex fans in assembly. A complete
   bounded fixture now proves all original face pairs and an oriented section
   cycle with the existing corner vertex and three edge-interior events.
   The bounded clip assembles common and difference when one convex cutter
   plane is active and every other cutter plane strictly contains the stock.
   It keeps original stock and cap supports and records the reused corner,
   retained edge intervals, and generated section edges.
   The same single-plane assembly now accepts a complete interior-edge section
   cycle without original vertex contacts or two distinct certified corner
   fans on the same cutter plane. A separate exact quadratic graph path
   retains nonaffine section curves on original stock and cap supports.
   In either original UV direction it can now traverse bounded free-axis
   knots, provided exact C0 agreement holds over the complete knot seam.
   A separate single-span polynomial path retains representable cubic and
   quartic graph trims with exact whole-curve source/cap agreement.
   The piecewise polynomial clip extends this to free-axis knot spans,
   preserving each exact model and chart knot junction. The creased cubic
   fixture passes native reload, mesh closure, and OCCT comparison.
   Both polynomial clips also retain oblique cuts when the exact cutter
   projection admits the same regular graph proof. All retained faces keep
   their original charts, including the oblique cutter cap.
   The rational graph clip handles affine projections with a normalized
   homogeneous plane residual linear in the dependent coordinate. Its
   bounded nonzero coefficient may vary quadratically. Exact quadratic,
   cubic and quartic sections remain on both original supports. The
   piecewise variant retains the branches and original knot junctions.
   General nonaffine crossings and tangencies remain unresolved.
   Two independently certified opposite parallel planes now form a retained
   slab. Both caps must lie strictly in the other kept halfspace; exact stock
   UV intersections preserve surviving source faces and edge intervals.
   The same native assembly accepts nonparallel planes when their exact convex
   caps are disjoint. A bounded transverse crossing now retains one cap-cap
   edge with both original UV intervals. Three through eight independently
   proved affine cutters can now share exact cap junctions in the native solid.
   The clipped stock and cap faces keep their original charts; all result
   edges have an original stock, stock/cap, or cap/cap owner. An uncut curved
   face can be retained intact. Arbitrary curved crossings and coincidence
   still require a separate proof.
   A separate nonparallel band path clips an already embeddable curved stock
   where both exact sections remain strictly in the other retained halfspace.
   It sews continuous original graph trims against independently placed cap
   charts. A cubic fixture retains both trims through native reload. The
   path rejects intersecting or touching caps. A second path handles two
   transverse cuts across one simple original section each. It isolates all
   cap-boundary contacts and shares one exact native cap edge. Both cap ends
   can land on planar stock, or one can land directly on a cubic stock face
   with two retained exact graph trims. An opt-in quadratic path persists
   nonbinary rational vertices without promoting their rounded display points
   to topology. Original vertices on the opposite plane, irrational junctions,
   intersections with inner wires, and multiple retained runs remain open.
   A complete inner wire strictly to either side is retained or omitted with
   its original faces and cap coedges. A crossing can select the upper or
   lower envelope of two same-direction graphs, joining both original trims
   at the proved cap endpoint.
   For polygonal sections, an exact line arrangement also handles crossings
   through inner wires and multiple material intervals on the same line.
   A cut square sleeve opens its void into a notch, leaving two independent
   cap/cap edges. The native result retains its original surfaces and reloads.
   The rational arrangement also classifies curved inner wires without a
   polygonal approximation. Exact plane residuals and composed model curves
   handle selected stock with nonaffine projections. A quadratically sheared
   sleeve retains the actual parabolic boundary of its void on both caps,
   with two exact seam edges, volume 13/32, and native reload.
   A single curved trim piece can now contribute several regular rational
   contacts. Repeated cap-only clipping retains parameters in that original
   directed piece, including the two ends of a retained middle interval.
   Two separated intervals on a shared free axis can become separate native
   faces from the same original curved stock chart. A quadratic floor fixture
   retains two such faces in one solid without subdividing the source chart.
   A lens-shaped cap with two equal endpoint pairs can now be subdivided on
   the original retained piece before both sides sew. A cap section can now
   keep irrational contacts with exact original parameters, projected roots,
   and ordered material intervals. Exact clipping now joins their retained
   original curve intervals and line seams into closed world-space cap
   boundaries, with separate occurrences and original parameter ownership.
   A stock-backed native recipe reloads and re-proves the cut and its
   original owners, and exact point queries preserve the gap across a void. Polynomial
   boundaries expose area bounds at the true algebraic endpoints. An
   additive rational quadrature path also accepts varying weights.
   Opposite caps independently recover and pair each whole exact seam,
   including distinct chart parameters at a shared irrational endpoint.
   Each retained cap interval now also binds to the original stock UV trim,
   with exact model equality across the entire interval. The retained
   stock faces now close into an indexed algebraic boundary, including
   several distinct retained components on a single original stock chart.
   All edges have opposed uses and every vertex link is a cycle. The
   sheared sleeves rebuild after native reload, preserve either an intact
   through-void or a notch, and have certified material queries. The
   notch fixture has certified volume and a watertight affine-projection
   display mesh. Exact cap-line queries and vertical fibers preserve
   nonbinary and irrational parameters through rational rigid placement. Ordinary solid conversion and general mixed-axis trim
   arrangements remain open.
2. Extend periodic and singular chart proofs. Sewing supports canonical seams,
   circular closed edges and collapsed poles of selected analytic primitives.
   Extend that work to general periodic spline faces, arbitrary inner loops,
   and embedding certificates on broader input trim domains.
3. Classify and reassemble exact curved fragments, return generated/modified
   entity ancestry, and add adversarial OCCT differential fixtures.
4. Extend general fillets, offset, draft and sewing after Boolean assembly;
   use the same topology for curved STEP/IGES B-reps and assembly metadata.
   Convex planar hollowing supports two separated complete openings, including
   nonparallel original supports. It retains exact cap coincidence and
   opening ancestry. Rectangular, sheared, and inclined ducts pass independent
   OCCT comparisons.
   The same proof now handles two to 32 pairwise separated complete ports.
   A three-port fixture forms one closed genus-two wall shell.
   Planar edge chamfers also accept compatible independent endpoint setbacks.
   Exact coplanarity keeps the entire tapered bevel on one plane, with
   original edge direction and continuous width metadata.
   A convex planar draft can rotate each selected support about its own
   original edge using a signed half-angle tangent and an exact rational
   rotation. The support reconstruction reports which original faces survive.
   Exact planar lofts now connect corresponding convex polygon stations.
   The separate band operation accepts a reentrant profile when every
   adjacent slab is convex and the assembled shell passes exact validation.

The broader qualification, packaging, and policy requirements in the
[specification](SPECIFICATION.md) remain in scope.

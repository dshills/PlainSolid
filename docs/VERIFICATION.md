# Verification record

The [latest combined development run](evidence/plaincad-reconciled-development-178df69-2026-11-13.json)
passed all 344 original tests on PlainCAD `50432ed` with clean combined kernel
`178df69` and a real Go WASM build. All 841 pinned sources verified. The original
suite ran for 43.9 minutes with zero skips, retries or failures. The staged
project-open and display edits were independently replayed and checked by
harness `6857a11`; no browser fixture or timeout was changed. This is separate
from the independently successful integration worktree run below.

The [earlier reconciled development baseline](evidence/plaincad-reconciled-development-baseline-2026-11-13.json)
ran all 344 original cases on clean `c1ba0f3` with pinned PlainCAD
`e65f07b`: 324 passed, 20 failed, and none skipped or retried. It
predates the arc and divider fixes. Eighteen of its failed originals passed
in separately pinned [arc](evidence/plaincad-reconciled-arc-regressions-2026-11-13.json)
and [divider](evidence/plaincad-reconciled-divider-regressions-2026-11-13.json)
retests. These retests do not change the baseline count. The [combined
development run](evidence/plaincad-reconciled-development-6d76880-2026-11-13.json)
completed on clean `6d76880`: 342 of 344 originals passed in 60 minutes,
with no skips or retries. Thread and CNC coach were the two failures. The
CNC coach passed its [separate unchanged retest](evidence/plaincad-reconciled-cnc-coach-2026-11-13.json).
The original Thread then passed twice, in 76.9 and 76.4 seconds, using
allocation and dense manifold validation changes. The [Thread comparison](evidence/plaincad-thread-performance-2026-11-13.json)
retains the prior timeouts, exact external mesh hash, and distinct clean
build IDs. The [next full development run](evidence/plaincad-reconciled-development-e060654-2026-11-13.json)
finished on `e060654` with 343 of 344 original cases passing in 51.4 minutes,
with no skips or retries. Thread passed; the CNC coach was the sole failure.
The run's only untracked files in PlainSolid were earlier verification reports;
recorded source hashes and the WASM SHA-256 bind the actual inputs.

The [full production suite](evidence/plaincad-reconciled-production-e579e5c-2026-11-13.json)
passed all 66 original cases on clean `e579e5c` with PlainCAD `e65f07b`.
The build verified 841 original sources. There were no skips, failures or
flaky outcomes. The separate backend preflight passed on port 5440; the
unchanged production fixtures used their original port 5280.

The [full cross-browser run](evidence/plaincad-reconciled-cross-browser-e060654-2026-11-13.json)
ran 15 unchanged cases on `e060654`: three passed and 12 failed in Chromium,
Firefox, and WebKit, with no skips or retries. All three engines passed the
native face-card and dimension case. Each of the 12 UI failures matches a
recorded pristine OCCT control using the same app source, at the same selector
or assertion. The unchanged cross-browser gate remains failed.

The three local trimmed-kernel short-mode shards passed on `e579e5c`: 478
top-level cases passed and four skipped by design. The integration worktree
also ran two of those short-mode exclusions explicitly in full mode, with
separate evidence below. These are pinned native results, distinct from the
browser cases.

The [production run on current PlainCAD `50432ed`](evidence/plaincad-reconciled-production-0312d69-50432ed-2026-11-13.json)
passed all 66 original cases in 356 seconds on clean `0312d69` with 841
original sources verified. The newer app's separate unchanged cross-browser
pass is recorded below. The [older-app supplemental comparisons](evidence/plaincad-prior-cross-browser-supplement-2026-11-13.json)
passed 15 cases each against pristine OCCT and PlainSolid after updating UI
entry points in disposable copies. They do not change the older original gate.
The [CNC coach trace and supplemental comparison](evidence/plaincad-cnc-readiness-supplement-2026-11-13.json)
prove the failed original checked the success state of an empty prior document.
Waiting for the imported fixture and enabled command passed on both providers,
with the same material assertion and deadlines. A [separate Parts Browser
audit](evidence/plaincad-independent-parts-readiness-audit-2026-11-13.json)
passed the original with staged asynchronous-open handling but reproduced an
older in-flight real rebuild publishing success during a held import.

On clean reconciliation `9b12a41`, all three original viewer cases passed
with the staged camera settling repair: pan/idle (53.8 s), source markers
(31.1 s), and ORBIT gesture/export (about 72 s). The separate Go/WASM
backend preflight passed. [Evidence](evidence/plaincad-reconciled-viewer-runtime-2026-11-13.json)
binds the build, original 841-file source inventory and exact display
runtime edits. This is an app adaptation result, separate from the earlier
unchanged viewer result.
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

On 2026-11-13, the CGO-disabled run pinned to `f8b543f` passed all 49
packages with tests other than `brep/trimmed`; the three remaining packages
had no tests. The run reported 3,540 passing and 293 skipped test events,
including subtests. The separate short-mode `brep/trimmed` run at `245b07a`
passed in the three CI shards (542 passing and seven skipped test events,
including subtests; 554, 393 and 473 seconds elapsed). A prior unsharded run
hit the default ten-minute package timeout. The wrapped bore test also passed
in isolation; that timeout does not establish an individual test failure.
The subsequent CGO-disabled run at `0309862` again passed all 49 tested
packages outside `brep/trimmed`, with three packages reporting no tests;
it recorded 3,672 passing and 301 skipped test events, including subtests.
The later CGO-disabled non-trimmed qualification at `61faf72` passed all 49
tested packages, with three reporting no tests. It recorded 3,751 passing
and 310 skipped test events. The three CGO-disabled short-mode trimmed
shards at the same revision all passed: 613 passing test events, seven
skipped, and no duplicate test events across shards (507, 620 and 477 seconds).
The CGO-disabled short run at `551df78` passed all 49 tested packages outside
`brep/trimmed`, with three packages reporting no tests. It recorded 3,782
passing and 316 skipped test events. The trimmed shards for this revision
also passed separately: 637 passing and seven skipped test events, with
elapsed package times of 655, 938 and 798 seconds.
At `d25b642`, all 49 CGO-disabled packages outside `brep/trimmed` passed the
short run (3,816 passing and 323 skipped test events; three packages without
tests). All three trimmed shards passed (658 passing and 14 skipped test
events; 358, 400 and 343 seconds). This verifies the pinned revision; later
milestones have separate focused qualification below.
The archived `e1e871c` CGO-disabled non-trimmed snapshot also passes all 49
packages with tests and reports 3,873 passing and 332 skipped test events;
three packages have no tests. The three trimmed shards at the same revision
also passed, with 696 passing and 19 skipped test events (659, 650 and 782
seconds). These are short-mode counts, including subtests.
The [CGO-disabled snapshot at `cd28630`](evidence/native-short-cd28630-2026-11-13.json)
passed all 49 tested packages outside `brep/trimmed` with 3,997 passing and
342 skipped test events. All three trimmed shards passed with 777 passing
and 22 skipped test events (650, 586, and 846 seconds). The run includes
subtests and short-mode skips; newer milestones have focused checks below.
The [CGO-disabled run at `e35945b`](evidence/native-short-e35945b-2026-11-14.json)
passed all 49 tested packages outside `brep/trimmed` (4,149 passing and 354
skipped test events; three packages without tests). The three trimmed shards
passed with 818 passing and 23 skipped test events, including subtests
(395, 498, and 383 seconds).
The [CGO-disabled run at `abb7965`](evidence/native-short-abb7965-2026-11-14.json)
passed all 49 tested packages outside `brep/trimmed` (4,277 passing and 361
skipped test events, and three packages without tests). All three trimmed
shards passed with 832 passing and 23 skipped test events, including subtests
(756, 731, and 928 seconds).
On 2026-11-14, the exact multi-cap wedge (`a0aa095`) matched a pinned live
OCCT halfspace common in volume, area, centroid, and its 6 vertices, 9
edges and 5 faces. The exact volume is 19/1536; the three generated seam
edges independently identify the same native apex on their original cap
charts. A second native fixture retains its original curved upper face
with volume 35/48. A nonparallel cubic band passes native reload, both cap
orders and chart orientation reversal, the independent volume 2969/122880,
and an independent area integral. OCCT agrees on all topology counts; its
volume and area differ by about 5.5e-8 and 2.7e-6 respectively.
A crossing cubic band sews a cap chord between exact points
`(1/2,0,3/16)` and `(1/2,11/16,3/16)` on the unchanged supports. Its
independent volume is 233/61440. Both operand orders reload natively;
pinned OCCT agrees on topology and differs from the analytic volume by
about 1.65e-8. Touching an original section vertex returns no partial solid.
The crossing assembler also preserves exact nonbinary quadratic seam endpoints
`(2/3,0,3/16)` and `(2/3,19/36,3/16)` through native reload. Two sleeve
fixtures prove independent volumes 67/64 and 3/64, keeping or dropping a
complete inner wire with the corresponding genus change. A polygonal
arrangement now also cuts through the inner wire. Its two native cap seams
exclude the gap across the through-void; volume is 13/32. Native reload and
both cutter orders preserve topology and exact seam ownership. A pinned
OCCT cut verifies volume, area, centroid, and Euler characteristic.
An independently specified shear `Y = y + x²/8` preserves sleeve volume
and moves both inner/outer cap boundaries to parabolas. The opt-in composed
section proof retains noncollinear exact controls and two seams in the
native result; the original affine-projection mode rejects the same input.
An independent section fixture encloses its exact retained area 341/48.
An additional parabolic segment has contacts at original parameters `1/4`
and `3/4`, despite its equally signed endpoints; the retained area encloses
`1/3` exactly. A second clip with mutated diagnostic output re-proves the
contact at original parameter `3/8` and does not reuse caller-owned data.
The two-run floor fixture retains two disjoint fragments on one quadratic
stock support and reloads one exterior shell. Independent volume is
461/61440; pinned OCCT agrees with all topology counts and differs in volume
by about 3.1e-8. Both original cutter orders reproduce the same result.
With both cuts keeping the lower side, the same original support has two
graph owner changes. An exact interior split in the curved lens boundary
appears identically on cap and stock; the native result reloads and matches
the independent envelope volume 221/30720.
The 3–8 face tensor fan and persistence tests (`0c87cc1`, `075b869`) reject
missing, misassigned and stale original contacts on reload. Rational graph
occurrences (`8da0749`, `fc8e3bc`, `6e73660`) retain native face and rim IDs
through world line queries, reflected/sheared bounds, and all volume moments
under nonbinary exact placements. The bounded STEP cylinder repair (`d454fa5`)
passes the pinned source and repaired-output OCCT mass comparisons, Go 1.23,
race and bounded fuzz; it reports exact displacement without weakening the
strict source import.
The `a2e35e9` signed offset native fixtures independently compare blind and
through area and volume against closed-form circle segment integrals and
verify original wall ancestry after native JSON reload. The exact-wall STEP
writer/reader (`980bdac`, `f2ddecc`) passed live OCCT reimport, race and Go 1.23
regressions; the older three reader domains still reject that product class.
The output keeps analytic cylinders and the native reader recovers exact
homogeneous coefficients including `26/3`. Affine camera picking and document
round trip tests in `bb9597f` passed under the race detector.
The offset STEP writer (`f7fff38`) passed eight live OCCT comparisons across
both signed placements, cutter orders, and depths. Independent blind volume
and area are 2282.90274417 and 1405.43256969; through results are 2165.80548834
and 1332.80030216. The native offset line and point tests cover algebraic
intersections, exposed cutter owners, and rational probes separated by 2^-90.
Pinned PlainCAD Chromium candidate `57f40ad` passed the supplemental zoom
test (19.330 seconds for the entire test), with 841 original application
sources, the unchanged test hash, and a passed Go/WASM preflight. The display
certificate's distance, normal angle, and collision guards all remain active.
The same pinned build also passed both supplemental blind and through save,
reopen, STL and STEP workflows without retries or skips.
Chromium candidate `2851e23` separately passed the larger offset blind and
through workflows (40.285 and 49.604 seconds), including the same public STL
volume check and native STEP readback. Public fabrication uses the accepted
model mesh, so tightening only the direct kernel STL writer did not satisfy
the workflow. The narrower circular-wall angle applies to that model family.

Different-center cuts prove the retained join and the other intersection
below the stock before sewing. Live OCCT comparisons verify signed placements
and both depths (`c09769b`, `660ca95`, `454b2f3`). Their native STEP reader
derives the join from the one axial edge shared by the two cylindrical walls,
and rejects altered owners and vendor-rounded graphs. Certified meshes repeat
after reload, with exact collision and independent volume segment bounds.
The PlainCAD review branch records the passing blind workflow at `50dd15f`
and the passing through rerun at `1c1c70d` (43.304 seconds for the test).
Both runs used the same 841 pinned application source files and identical
supplemental assertions. The through rerun retained the original eight-second
worker inactivity watchdog. On the native quarter-arc fixture, the optional
bounded rim schedule reduced 1,040 triangles to 784 at the same distance,
angle and collision gates. The independent volume-error bound was below 0.205.

For broader curved contacts, `aa2f953` and `c1f0aca` preserve nonbinary source
charts through certified transverse branch coverage and independent algebraic
section reload. The curved fixture retains original coordinates 1/3 and 2/3.
An independently refined periodic toroid sews with exact chart correspondence
(`b0ff31f`), and nonconstant or fractional wrap shifts reject. Exact periodic
overlaps and bounded chart arrangements retain original owner UV intervals;
the arrangement independently cancels shared interior edges (`a0d72b1`). The optional
circular wrapped cylinder/frustum mesher (`2ab2135`) additionally bounds the
continuous facet normal deviation, including taper. Bore meshing adds material
normals on each inner/outer wall and the two caps. Lossless signed-axis rigid
placement transfers the proof to the world mesh (`8450183`, `d62e758`); mesh
translations requiring coordinate rounding reject.

`3c72c14` builds original-support wrapped face cells for the qualified overlap;
`bce61a1` connects the cells in an immutable open union/intersection graph.
Certified shared edges have opposing uses and the exterior follows the first
face orientation. Focused race checks cover guarded topology failures and
immutable accessors. The exact directed G1 STEP codec (`66c15f5`) retains
joins with unequal speeds. The independently transferred OCCT model has one
wire, two edges, and length 3.0259717630258898; the C1 codec rejects that input.
The exact roof shell regression uses a degree-elevated homogeneous chart. It
proves a closed section and a two-face common volume
on the original roof, then reconstructs both from native JSON. A modified
recipe chart fails to bind to the original face. The rational volume enclosure
narrows on refinement and contains an independent bracket of π(5/16)^4/2.
The same exact shell classifies roof, inside and outside points. An independent
cubic fixture checks exact point membership, a sub-ULP rejection, line
parameters and the minimum distance, including translated bounds and regularity.
Wrapped Difference (`2a35e85`) qualifies an annulus with original cut-edge
ownership, plus an empty result and a partially overlapping lift. A separate
true seam crossing proof (`d09b8b7`) rejects a fold even though each side
independently has an injective projection. Mixed-trim display qualifies an
exact cubic after reload and a multispan model with a true cubic derivative
crease (`c008e53`, `6573299`). The split cylinder STEP fixtures (`aec0684`,
`977b2da`) transfer as valid four-face OCCT solids with rational and explicit
parameter-trimmed rational axial curves; readers retain original parameters
and senses and reject edits to the represented geometry.
The true seam proof is used end-to-end by opt-in wrapped face operations
(`aaf6fe0`), including continuous native agreement across the physical seam.
The native two-face lens conversion (`36ea13e`) reconstructs both original
faces and proves every implicit model/UV edge use. The two-lobe shell regression
then qualifies two independent closures and an analytic total volume bracket
for its two disjoint circular sections. Independently authored and Go-written
IGES type 186 fixtures (`23c2931`) both transfer through pinned OCCT as valid
tetrahedra with four faces, volume 4/3 and centroid (1/2, 1/2, 1/2).
The reflected oblique IGES hexahedron (`bd81c25`) transfers both ways as one
valid six-face OCCT solid, with volume 13 and centroid (2.5, 5, 6.5). The
shared algebraic Boolean display (`9983659`) has 56 model edges and 180
certified straight display segments at deflection 0.02. Opposite coedges
share indices, and edits to diagnostic views do not change the proof. The
two-image cylinder proof (`92a3723`) separately distinguishes two physically
disjoint intersection components from the connected annular union. The YZ
shell regression (`6abcc6b`) uses overlapping unequal projected rectangles and
closes the resulting union on the original sources.
The biquadratic IGES shell (`8bf3e78`) preserves a curved roof and retains
original spline charts through shared-edge proofs and global native embedding.
Both the independent fixture and Go output transfer through pinned OCCT as
one valid solid with volume 37/36 and centroid (1/2, 1/2, 951/1850).
The curved-rim IGES solid (`8226fd9`) independently transfers in and out of
OCCT with two quadratic model edges, volume 4/3, and centroid (9/16, 1/2,
7/10). The first-moment and world inertia bounds (`ce5ad8f`, `7030f52`)
enclose independent volume integrals of a signed, translated warped slab
and a separate box. The exact-lens point/line test (`66eb459`) passed race;
the finite line suppresses buried crossings and rejects a simultaneous rim
contact atomically. The closed graph-lens mesh (`f0ccdaa`) passed race in
both XY and YZ. Its 1,520 and 1,696 triangles satisfy bounds of 0.097219 and
0.072267 respectively and pass exact display non-incidence checks. The
exact annular union and its native adapter (`41a1275`, `94cff4e`) keep two
oppositely directed boundary loops; nonbinary native vertices are rejected
without losing the exact complex.
The capped circular frustum erosion matched independent pinned OCCT
intersections of the shifted analytic cone and cap slab for both taper
directions. Native mass, continuous rim agreement and race checks passed.
Closed frustum walls also match the independently subtracted OCCT cavities.
Exact parallel-axis containment admits an eccentric interior frustum with an
irrational axis distance and rejects equality at the lateral contact.
The reordered, translated IGES roof (`d250f71`) preserves original entity
and directed owner identities; both the source and canonical Go re-export
pass pinned OCCT transfer. A seam-crossing diamond hole and a separately
authored nonconvex outer loop (`955ef3e`) pass exact trim embedding and
continuous native edge correspondence. The grouped line-event API (`7020ef7`)
retains both original rim hits and verifies the different adjacent material
states for common, union and cut; the simpler transition API still fails
on simultaneous contacts.
The placed-shell regression (`66b2456`) passed race on a reflected and
translated original spline shell, including native reload and changed-recipe
placement rejection. The certified degree-five mesh fixture (`c5bff88`)
uses an original homogeneous X control of 11/40 and retains its source
through section, Boolean, and closed tessellation. Its 1,520 triangles
meet a two-sided distance bound of 0.097164 and pass exact contact checks.
The full exact roof line-event regression also passed race, including both
source hits at simultaneous contacts and the adjacent Boolean material states.
The wrapped quadratic native face (`0d6a13e`) splits a curved owner at exact
parameter 2/3, retains the original lifted chart across the seam, and
proves continuous model agreement with independently refined reversed edges.
A changed curve with identical endpoints, an irrational seam parameter and
exhausted proof limits reject without producing a face or an edge agreement.
The leaf-placed IGES roof (`64c3792`) keeps original local model and UV curves
and returns a separately proved world solid. Pinned OCCT reads both the
independently authored source and Go output as one valid six-face solid,
with volume 4/3 and the posed centroid (10.7, -3.1875, 0.5). It ignores a
root-only transform on the comparison fixture; the Go reader rejects it.
The mixed placed IGES roof (`6f062ea`) retains both baked world-space leaves
and leaves with an exact placement pointer, keeping unplaced original UV
curves. Both independent source and Go output again transfer as one valid
solid with the same world mass properties. The two placement readers have
separate bounded fuzz checks.
The unequal projected lens (`b9e7a63`) keeps an exact, degree-five original
roof and its narrower projected domain. The rational volume, three centroid
coordinates and selected world inertia terms enclose the independent disk
integrals. The unequal exact, XZ and independent affine-chart tests all passed
race; a small budget returned no partial mass values.
The convex trim extension (`eae3f58`) verifies an exact common trim corner
at (1/3, 1/6), the complete footprint area of 167/384, and the independent
volume, centroid and inertia bounds. The same cell integration now works
with bounded convex straight trims and their aligned rectangular supports.
The disjoint-region measures (`438411f`) separately bound two lobe volumes
on unequal original rectangles, retaining an exact cubic owner with
nonbinary controls. The common footprint is clipped before mass is assigned
to the original section branches. Both extensions pass focused race tests.
The mixed STEP assembly (`15575b7`) keeps two shared planar/spline product
definitions and three placed occurrences. The pinned OCCT reader independently
accepts the OCCT-authored file, Go export and Go re-export. All report 18
faces, volume 16, and centroid (-7/16, 175/24, 139/144). Local original
curved supports and separate occurrence identities survive the Go round trip.
The heterogeneous cylinder assembly (`988b2e2`) adds two occurrences of
one shared analytic cylinder. Independent OCCT source, Go output and source
re-export each transfer as five solids and 24 faces, with three shared
definitions, volume 16+4π and the independently derived world centroid.
The bounded periodic face recipe (`4b4be54`) stores just the fundamental
surface and authored quadratic loops. Reload re-proves the seam, trim
and native model edge agreement; both the retained seam parameter 2/3 and
the inclined-line query survive reload. Unknown or duplicate fields, damaged
geometry, trailing data and exhausted limits reject.
Line, plane, and boundary distance queries on the nonbinary exact roof
(`a0c6365`) now use its original support. They recover independent rational
line endpoints 1/4 and 3/4, plane rim endpoints 3/16 and 13/16, and zero
distance at the analytic crown. This and the existing XZ fixture passed race.
The vertical periodic fiber classifier (`7e44fad`) proves ordered exact
trim crossings and overlap intervals using an original-chart rational
witness. Signed traversals through a seam hole, isolated boundary contacts,
shared-edge endpoint ownership and the same algebraic seam parameter survive
focused race checks. Unrecovered roots and coincident spans reject in this
milestone rather than changing hole parity.
The mixed IGES material collection (`92f76f0`) reuses independent full
tetrahedron and curved roof proofs and preserves each source owner's entity
IDs. The live pinned OCCT oracle transfers source and Go re-export as two
solids, ten faces, volume 8/3 and centroid (21/32, -1/2, 21/10).
The three-family fixture (`5edb394`) adds two independent tetrahedron roots,
one oblique affine hexahedron and one original curved roof in a single file.
OCCT qualifies four solids, 20 faces and volume 17; the Go reader checks
unique source ownership and deterministic provenance across all roots.
The nested spline Boolean (`7e8283e`) proves the arrangement of two
closed original section rims, native annular closure, and JSON reload
from the original sources. An analytic volume integral independently verifies
the annular common; the subtraction retains both cut rims.
The annular display with a nonbinary original owner (`eab4c3a`) passes
normal qualification: 13,416 triangles, genus one, exact contact exclusion
and display volume 0.00044855648 against 0.00046287642 analytically. Its
full race invocation hit the 600-second test timeout; it is not a race pass.
The open frustum wall and point classification (`170c638`) retain an
annular shoulder on either original cap and an inward floor. Four independent
pinned OCCT cuts agree in area, volume and centroid for both taper directions
and both openings. Focused normal and race tests pass.
The open frustum display (`17fe67c`, `7df6c0c`) keeps the native face IDs
and both original wall sample rings at the opening. The independent volume
comparison, global exact contact exclusion, continuous material normals,
and exact placed display passed race. An inexact translation rejects.
Its line query (`ae41070`) keeps both annular rim owners and the inward
floor, including coincident generator and cap overlap intervals; the
removed opening disk never appears as a boundary. It passes focused race.
The nested Boolean display (`9f8e2fd`) joins the two original rims into
one contact-free genus-one mesh with 13,416 triangles. Display volume is
0.00011216016 against the analytic 0.00011571911. It passed the full focused
normal check and a race check of the guard paths. A separate combined
nested Boolean race invocation timed out while reconstructing imprints, so
its full nested test is not counted as a race pass.
The exact circular IGES material codec (`d596190`) preserves all three
original face supports and the full degree-based periodic side schedule.
OCCT accepts the independent exact source and Go output as one three-face
solid of volume 12π and area 20π. The retained OCCT vendor output with
rounded seam coordinates is intentionally an unsupported fixture.
The mixed IGES circular material extension (`cd76083`) retains independent
planar, curved spline and circular bodies, with explicit source owner IDs
for each. The source and Go output transfer in OCCT as three solids and
13 faces, with volume 8/3+12π.
The shared nested Boolean imprint (`4108844`) avoids rebuilding the same
proof for common and cut. A focused race run of the original nested section,
joint operation, and native annular conversion passed; the complete normal
fixture also checks exact volume and native recipe reload.
The signed YZ annular display (`9faca14`, `96f8114`) preserves the original
world shell, both native rings and source vertex IDs through its exact
projection. It produces 13,416 triangles, no unintended contacts, and
display volume 0.00011209655 against the same analytic 0.00011571911.
The eccentric frustum bore (`d6f9165`, `35e207d`) retains both original
axis locations and checks exact strictly positive clearance. Independent
OCCT mass, race checks of the material and line queries, and a closed
display with original wall normals passed. An eccentric inner tangent
does not split the cap overlap intervals.
The algebraic periodic fiber (`e40f27a`, `34e3160`) carries exact irrational
point roots, original owner trims and original line parameters. An opt-in
mode also preserves affine coincident intervals in the same ordered proof.
The sole external spline shell touch (`25d9d47`) qualifies an interior
quadratic contact in XY and YZ. Normal, race and OCCT tests verify empty
common and unchanged cut; native union fails on the nonmanifold point join.
The unequal-rectangle variant (`ec71388`) first clips both original
affine chart domains to their exact physical overlap and proves the contact
there. The analytic world point and both original owner UV coordinates
survive; normal and focused race checks passed.
The placed circular IGES material (`02cd1c4`) puts the same proper signed
type 124 placement on all six support/model leaves. It preserves local
analytic charts and verifies authored world vertices. OCCT source and output
agree on one solid, three faces, volume 12π, area 20π, and centroid
(8.5, -4, 2).
Mixed placed circular material (`8474571`) keeps the original type 124 as
a uniquely owned dependency of its own cylinder and preserves its directory
ID through exact rebasing. An independent planar/spline/placed-cylinder
source and Go output transfer in OCCT as three solids and 13 faces.
The signed XZ annulus (`14926ac`) qualifies the odd projection permutation.
The independently checked world stock normal and both original boundary
traversals agree with the native shell; its full normal test and focused
guard race test passed.
Wrapped composition proof cells (`5b39e64`) retain an irrational seam
root, the original and shifted UV occurrences, exact original curve
parameters and a proved model endpoint join. Reversing the authored
curve or refining original knots preserves the attained model image.
The native open-face constructor still requires its representable boundary.
The shell Boolean dispatcher (`538dee7`) now selects the sole-contact proof
after independently separating all other face pairs. The existing
separated, closed, affine and boundary-contact dispatch checks pass.
The isolated quartic contact (`d25b642`) verifies empty common and an
unchanged unit cut even though the Hessian vanishes at the touching point.
The full exact numerator proof, focused race and independent OCCT results passed.
The wrapped model comparison (`e26ecd3`) proves complete continuous agreement
across independent original knot refinements, opposite traversal and an
integer-period owner lift. Same-endpoint curves with different interiors reject.
The signed and translated YZ disjoint lens mesh (`63c1dc9`) preserves
world-space native IDs across its two independently closed components. It
passes normal qualification with 3,392 triangles and an exact collection
contact check. The limit and ancestry guard paths pass race.
Analytic blind and through IGES bores (`813909d`, `2c75ecf`) preserve
original analytic walls, opening rims, degree-based UV ownership and
the retained floor where present. OCCT transfers the independent source
and Go results with volumes 34π and 32π, and areas 46π and 48π.
The rounded vendor copies are documented negative cases for the exact codecs.
Mixed IGES solids (`4150e8b`) now include both analytic bore types with a
tetrahedron in an independent three-solid file. Pinned OCCT transfers 13
faces, volume 4/3 + 66π and area 6 + 2√3 + 94π, with the independent centroid.
Go 1.23, race, bounded fuzz and cross-root ownership guards passed.
The STEP disk-hole assembly (`8b9d11d`) retains two local definitions and
three placed occurrences, including a reused definition. Pinned OCCT
transfers the independent XCAF fixture and Go output as three solids and
14 faces, with volume 100π, area 140π and centroid (0.9, 6.8, 1.34).
The independent vendor file remains a negative case for the strict canonical
body reader. Focused Go 1.23, normal, race and bounded fuzz passed.
Native open wrapped algebraic faces (`23c2218`, `019f243`) preserve authored
boundary edges and exact model cells across irrational interior seams. Their
standalone bounded JSON re-proves composition, edge/vertex ownership and
embedding on reload. Altered interiors with identical endpoints, malformed
witness data and nonrepresentable boundary vertices reject. Focused normal
and race passed; the legacy face recipe remains strict.
The XZ disjoint Boolean display (`529dc1e`) preserves both component and
global winding and original owner traversal under a signed translation.
The full fixture verifies 3,392 triangles and 473,718 component and collection
collision pairs. The signed XZ exact-owner annulus (`5b962ca`) retains the
original `41/24` homogeneous control, world normals and 13,416 triangles;
full normal and bounded placement/guard race passed.
Reindexed native frustum bores (`9c3fa87`, `43126f1`) re-prove face ownership
after JSON reload. Horizontal section retains the exact original `V=3/7`;
axis, annular and cavity line tests distinguish both opening disks from both
retained inner caps and preserve original face IDs. Normal and race passed.
The through-bore and enclosed eccentric frustum displays (`9f7124c`) preserve
shared annular rim indices, all native face IDs and continuous wall and cap
normals. The cavity has a separately proved strict shell parent and inward
orientation. Both display volumes agree within the analytic fixture bounds.
The through, blind, placed and eccentric normal mesh regression ran under
race (188.5 seconds); the nested cavity and mesh locator passed focused race.
The paired wrapped algebraic edge (`cbdece9`) rechecks both original p-curves,
exact endpoint ownership and opposing incidence across an irrational event.
The bounded two-face shell (`449035b`) retains six vertices, seven edges and
the six-use exterior. Normal and race tests verify independent refinements,
reversal, undeclared contacts, and witness and work guards. It does not
establish global embedding of two arbitrary supports.
Prepared graph Boolean queries (`26aa008`) distinguish exact rational probes
separated by `2^-200` and retain native bracketing faces, exact fiber heights
and rim owners in annular and signed XZ disjoint material. Full fixture runs
and concurrent reuse with caller result mutation passed race.
The opt-in STEP reader for alternate disk-hole bounds (`e58b59e`) admits a
fully reversed source loop and coedge order together with a negative bound
sense. A pinned OCCT representation and canonical re-export match one solid,
six faces, volume 94π and area 100π. Partial reversals and rounded vendor
geometry reject. Native mapped planar STEP (`12cc319`) independently imports
an OCCT-authored box with two exact rigid mapped uses. Source and write-back
transfer as two solids and 12 faces with volume 48, area 104 and centroid
(6.5, 11.25, 1.75). Model/p-curve mutation, ownership, normal, race and bounded
fuzz checks passed. The mapped writer validates and re-emits the source graph.
Reconciled PlainCAD candidate `754ae8e` passed the real backend probe with no
OCCT request and both different-center openings in Chromium (38.5 and 46.6
seconds), on the same 841 pinned application source files and unchanged
supplemental fixture. Both cases rebuild, reopen and export STL and STEP.
The algebraic mixed arrangement (`fa11c4c`) puts a full closed loop and a
proper open cut on the same original stock face. In independent left- and
right-wall examples it preserves one full and one half lobe. Common has
Euler characteristic 4, cut has 2, and union has 0. The independent pinned
OCCT right-wall fixture agrees on topology and mass, and the exact native
enclosures contain the analytic volume of each operation. Both fixtures
passed focused race; the left-wall Boolean also reloads with the same owners.
The expanded shell contact certificate (`b8ef72d`) verifies a separable
fourth-/second-order contact at an original knot and two distinct isolated
knot contacts on one original face pair. All remaining pairs are separated.
Native and live OCCT confirm empty common and unchanged cut; the zero-side
negative and focused race passed.
Exact graph normal-line clipping (`15d91a6`) preserves caller `t`, real rim
contacts, and original faces under signed nonbinary line scale. The window
depth query (`77c05e9`) independently proves uniform depth on nonbinary
annular windows and signed XZ placements; nested hole windows prove empty,
and crossing windows reject. Focused normal and race passed.
The mapped STEP collection (`5bdb9a6`) retains two independent maps and
three rigid occurrences. Native document conversion (`ea3ae75`) preserves
shared local assets, actual transformed bounds and all 12 original local
face bindings across reload. Pinned OCCT source and export transfer as
three solids and 18 faces, with volume 54 and area 126. Normal, race and
Go 1.23 passed. Wrapped shell reload (`0d05fd5`) re-proves original mating
through the irrational seam. Global embedding certificates (`972e7a6`,
`f137d9f`) qualify the adjacent two- and three-face common chart families;
overlapping nonadjacent owners reject, with focused normal and race passed.
Representable algebraic solid conversion (`e1e871c`) binds every section
endpoint on both original charts and retains source edge ranges. The warped
box reloads in ordinary native JSON with original indexed faces and volume
1/8. A mixed circular result with nonrepresentable junctions rejects the
conversion; its algebraic topology remains available. Normal and race passed.
Mixed projected volume windows (`f0bba49`, `036febd`) preserve certified
material/empty cells and charge unresolved area to the upper enclosure.
Contracting all 24 original tubes into 48 verified intervals reduced
unresolved area from 27/1024 to 99/4096 on the signed XZ fixture without
changing the spatial budget. Positive race and full focused normal passed.
Native wrapped chains (`7027b5e`, `a0ba988`, `f7e9c97`) retain globally
embedded original faces, exact mate edges, continuous agreement and indexed
ancestry. The dedicated three-face reader re-proves the whole original shell
and rejects altered lifts and ownership. The 2–8-face constructive chain
passes independent knot, orientation, overlap and fan tests under race.
Native mapped STEP writing (`7ecc377`, `97913f8`, `dda5d32`) supports
edited occurrence placements and cardinals directly from native planar
assets, including holed material. Rebuilt exact model/p-curve ownership
and live OCCT validate three occurrences and world bounds from
(-10, 0, 0) to (12, 22, 4). Source-retaining edit and new native document
authoring remain separate APIs. Both have focused normal, race and Go 1.23
coverage. The new STEP and mixed IGES fuzz targets ran 100 cases each in
their bounded qualification and are registered in CI (`588262b`).
The retained frustum bore distance (`40d8e76`) bounds the minimum squared
distance globally. Live OCCT agrees on five through and cavity probes,
including an irrational rim distance, a retained annulus and both inward
floors. Normal and race checks pass, including native reindexing and limits.
The full reconciled production build `967e3c3` passed all 66 original PlainCAD
tests in 242.4 seconds, without skips, retries or failures. All 841 original
sources matched the pinned `50432ed` archive; the separate browser preflight
confirmed the Go/WASM backend and no OCCT requests. Both TypeScript checks
and the original controlled rebuild/STL timing case passed.
The current development STEP subset (`0bdb699`) passed five original tests
in 23.226 seconds and the independent Go/WASM browser probe in 5.464 seconds.
All 841 tested source hashes match pinned PlainCAD `50432ed`. The archived
record includes the six exact titles and result hashes.
The development modeling selection on `ff15d87` passed three original
PlainCAD tests in 25.488 seconds, with zero retries, skips, or failures.
The separate real Go/WASM browser probe passed in 5.317 seconds. Evidence
records typechecking, all 841 original source checks, and hashes of the test
outputs for that reconciled revision.
The rational frustum section (`b894c4b`) keeps the original V parameters
7/12 and 11/21 at Z=7/3 and returns exact area (275/64)π. Point probes on both
sides of the original wall differ by 2^-120. The exact line path (`30d8254`)
keeps an original cap-plane inner-rim tangent distinct from those sub-ULP
offsets, isolates a separate wall hit just beyond a known cap rim, and
returns all four exact cavity cap contacts after native reload and reindex.
The retained graph distance (`020cdd6`) proves squared distance exactly 1 to
an original cap and includes analytic 49/4096 between separated lobes without
using the buried support plane. An exhausted budget preserves a conservative
interval. Both query paths passed focused normal, race and Go 1.23.
The graph common window (`955f335`, `38b98c3`, `d98a1db`) excludes original
unselected trim and integrates an exact retained polynomial cell to
1171747/48000000000000. Signed XZ first moments and the exact local centroid
match direct integration, while the two-lobe centroid stays enclosed with an
unresolved rim remainder. New wrapping proofs (`5835de5`, `2fd4220`,
`a0e132c`, `62b9455`) retain original maps and exact indexed seam ancestry
through cyclic fans and separately parameterized shells. Exact certified
separators reject extra contacts, and native reload re-proves the recipe.
Pythagorean planar local charts (`d0d87b7`) match independent OCCT volume
6006 and area 2622. The exact mapped occurrence and document paths
(`f541219`, `0b0f467`, `269b659`) retain 3/5 and 4/5 rotation coefficients,
10^-40 translation and shared original assets after native reload. Native
baking rejects rounding. Pinned OCCT reads the bounded nested source and
re-export as six solids with 36 faces, volume 108, area 252 and bounds
(-10,-8,0)–(106,22,4). Focused normal, race, Go 1.23 and 100-case mapped fuzz
passed, including graph-cycle and stale-native-asset rejection.
Mixed STEP groups (`232bf59`, `0cb6011`, `3011537`) preserve two or more
original mapping steps and independently validate all expanded leaf paths.
Pinned OCCT reads the two-level fixture as 12 solids and 72 faces with volume
216 and area 504, and the mixed graph as 11 solids and 66 faces with volume
192 and area 452. The explicit shared-pose edit re-exports exact bounds
(-10,-10,0)–(204,156/5,104); unrelated paths and local assets stay unchanged.
Both formats pass native reload, live OCCT, normal/race and bounded fuzz.
Exact wrapped model-line queries (`802b3f1`, `8e9ddea`) keep rational
finite parameters, both original seam occurrences, oriented crossings,
trim-hole exclusion, and two irrational vertical trim events. The vertical
probe rejects an XY offset of 2^-120. Different-support chains (`ef79bc9`,
`57c3dc2`, `ab39904`) test exact nonadjacent gaps for 2, 5 and 8 supports;
the separate three-face envelope re-proves each mate and indexed owner.
Graph inertia (`50458f6`) matches all six independent signed-XZ tensor
entries and encloses the two-lobe centroidal result. Retained surface area
(`49cbefb`) proves exact horizontal area and conservative curved area using
positive rational support derivatives, including zero area for a proved
hole. Whole measurements (`cd28630`) derive the exact support window and
share one certified partition for area and volume. The oblique line
(`c9b4f7f`) proves two distinct retained intervals, rejects a buried cap
crossing, and encloses all four irrational contacts using refined full rim
covers. Normal, targeted race, and focused compatibility checks passed.
Exact rim overlaps (`7d85451`) produce two cap spans and four exact rim
owners; algebraic rim and roof contacts retain original polynomial root
identity. Near-tangent roots closer than the requested tolerance separate
strictly. Bore distance (`f57b6cd`) distinguishes correct nearest owners
on either side of an open rim by exact squared difference 2^-240.
Oblique wrap mates and chain persistence (`817a6e3`, `726e6ca`, `b2352d1`)
prove complete face separation with rational covectors and preserve the
original indexed seam. The bivariate finite triangle path (`6fdf645`)
matches diagonal seams across original knot partitions. Whole graph mass
and refinement (`d1c0b4e`, `7813f69`) reuse one pass for area/volume/moments
and intersect separate bounded multi-resolution proofs where requested.
Original OCCT XCAF NAUO products (`65492d2`, `8522043`) preserve exact
relationship and product ownership after native reload and STEP export.
Pinned OCCT reads the single-product fixture as two solids and the two
independent products as three solids with volume 54, area 126, centroid
(85/18,86/9,5/2) and bounds (-10,-5,0)–(12,22,10).
The nested original XCAF source (`4f20b6e`) re-exports six solids and 36
faces with volume 108, area 252, and bounds (0,-5,-5)–(112,112,22).
The independent shared-group source (`9c48832`) re-exports seven solids and
42 faces with volume 114, area 274, and bounds (0,-5,0)–(112,112,72).
Distinct root/shared pose edits (`8288785`, `d157cfa`) preserve source IDs
and unselected records. Live OCCT confirms the edited rational root pose
and all affected shared paths; the latter moves the upper bounds to
(122,122,72). Each path has focused normal, race, Go 1.23 and fuzz evidence.
Exact noncardinal rational occurrences (`dd2e2b7`, `d45e1b7`, `ea16cfe`,
`e1cbf3a`, `357f244`, `f5b2344`, `ce80740`, `98334c6`) preserve original
caps, full generator and toroid material spans, exact signed squared
distances, fourfold tangent multiplicity, analytic annular section radicals,
and world centroid/inertia. An independent weighted-frustum cap probe
projects exactly at squared distance 1/81. A generic surface probe keeps
a squared distance 2^-240 that would round to zero in input binary64.
The exact tensor adapter and open shell recipes (`a2ce204`, `013f645`,
`e921692`, `b6c2e40`, `ced4616`, `6507324`) retain original multiknot charts,
lossless nonbinary controls, and re-proved indexed diagonal seams and
vertex fans. The verified three-face chain retains all nine original edge
uses; native reload rejects stale individually valid face recipes.
Affine graph measures (`a5dbdb1`, `5fa8fbd`, `0b6ff26`, `1ee0d1e`, `ff68870`)
use exact convex exclusion and scaling or per-cell transformed normals,
with one original material partition for requested volume, centroid, raw
moments, inertia and surface area. Independent two-lobe covariance and
curved density checks pass. This does not reconstruct oblique Boolean
faces in world space.
Distinct original spline contact pairs (`753c316`) agree with live OCCT on
empty common and unchanged cut, with 58 other pairs proved separated. A
missing pair and exhausted shared budget reject atomically. Normal and
focused race passed. Exact frustum bore mass properties (`e8b53d9`) match
independent OCCT area, volume, centroid and inertia for an eccentric tube
and a tapered enclosed cavity; the original cap and wall owners are retained.
The two-lobe Boolean display (`d26665d`) keeps one global set of source face,
edge and vertex indices for 3,392 triangles in two closed components. Its
two-sided distance bound is 0.015637139; 482,854 contact checks qualify the
whole display. The fixture passes under race and checks display volume
against the independent total analytic volume of the two lobes.
The corresponding exact-owner fixture (`8a484ac`) retains a homogeneous
1/3 control on the original opposite plane and gives the same independently
checked closed display. The wrapped-cylinder point and line queries
(`f034043`, `b8396ac`, `60eda3f`) preserve original seam occurrences,
exact rational parameters, and signed oriented normals. They check holes,
inclined directions, and nonbinary chart heights under race; vertical
directions and unresolved supporting-line contacts remain outside this path.

Blind and through equal-depth overlapping side pockets now have a direct
material query in the certified cardinal frame. It distinguishes the actual
exposed circular seam from a buried individual cutter boundary, and resolves
opposite rational probes separated by less than a float64 ULP. Closed-form
volume, area and centroid agree with independent integration of the stored
curved faces on translated and rotated stock, and with the optional OCCT
blind-pocket oracle. These results do not use a mesh.

The same blind and through stock fixtures pass closed display qualification
at model deflection 0.1 with a rectangular seed that preserves shared exact
boundary parameters. The 3,928- and 3,526-triangle displays independently
pass manifold, oriented volume, Euler and non-incidence checks. The two
curved wall seeds converge without the legacy uniform refinement explosion.
This proof bounds distance and display embedding; it does not bound normal
angle. Separately, six signed poses of the rooted model-only STEP writer
passed OCCT `TransferRoots`, shell validity and area, volume and centroid
comparisons. Native reimport reproves the full emitted product graph and
rejects tampered endpoints, supports, entity ownership and metadata.

Composed circular event tests now cover whole coincident cells, exact
stationary roots inside those cells, stationary roots shared by consecutive
source cells and constant circle-point intervals. Neighboring noncoincident
endpoint events retain their component ownership; actual isolated crossings
remain separate. The existing stricter entry points retain their prior gates.

On 2026-11-13 the expanded pinned OCCT reference suite passed 155 test cases
(including subtests) with no skips. Both STEP import and transverse spline
shell operations were enabled. New cubic and
quartic rational plane cuts match independent OCCT prism Booleans on shell
count, area, volume and centroid. Independent strip integrals agree, and native
reload retains exact UV coefficients, rejects a sub-tolerance coedge edit and
produces closed contact-free display meshes. The original knot ownership is
also checked on a two-span cubic with a crease.
For the quadratic denominator `(1+3X²)/4`, complementary cuts agree with
independent arctangent/log strip integrals and OCCT. A second quadratic with
positive endpoint values and an interior double zero is rejected.
A positive rational weight depending on the free coordinate is also retained
exactly. For the weighted wedge, the Go result matches an independent
one-dimensional volume integral and the two independently swept and cut OCCT
solids. Nonaffine projected charts and weights that depend on the other
coordinate are rejected.
Ray checks on a rational quartic trim distinguish whole inside and outside UV
boxes even when both overlap the unsplit control hull. A one-box budget leaves
the result unresolved, as does an enclosure straddling the actual edge.
Distance checks attain the exact planar cap bound and enclose a valid closest
distance near the curved edge using an exact rational trim witness.
The reusable scalar plane-residual enclosure preserves full-subpatch UV bounds
in the circle regression and encloses independently derived points on a
weighted oblique chart. The isolated curved-distance case passed under the
race detector in 8.72 seconds after the change, against its unchanged
30-second deadline. This is a local targeted measurement.

Exact spline section checks retain original edge parameters across an
oblique weighted shell, reverse every directed wire when plane orientation
reverses, preserve the six pieces and internal knot positions of the creased
cubic cut, and preserve the exact nonbinary terminal heights 1/3 and 2/3 on
an affine box. The supported tests passed under the race detector. Separate
tests retain exact curve-plane contact ownership at algebraic knots and report
whole coincident spans without duplicating their endpoint events.
The projected ring proof recovers a square tube's outer wire and four-edge
hole from the directed source branches. Independent unordered rings exercise
disjoint components and an island within a hole; touching rings and proof
exhaustion return no arrangement.
The cap reconstruction preserves the square-tube void in a reversed affine
chart. A second oblique weighted fixture verifies identical rational model
points on the source and reconstructed planar cap; too small a cap support
rejects without returning partial faces. Both passed under the race detector.
Direct splits close both square-tube halves with one holed cap and volume 4.
Oblique weighted splits preserve the original support, validate opposite cap
orientations, and match complementary independently integrated volumes.
The exact source-edge/section-edge ancestry partitions every output edge.
Both halves of the creased cubic split survive native JSON reload with their
two cubic cap pieces and all six section edges. Their display meshes remain
closed, positive-volume, and free from non-incidence triangle contacts.
Section area enclosures match a logarithmic integral for the oblique weighted
slice, an exact area of 8 for the square tube, and an exact cubic Green
integral for a polynomial boundary, without relying on display samples.
The optional quadratic endpoint path splits the affine box at
`z=(1+x)/3`, retaining the four vertices at exact heights 1/3 and 2/3 in
each native reload. Both halves have volume 1/2. The strict binary path
rejects the same source instead of silently rounding it.
Two parallel cuts of the weighted curved stock match the independent volume
integral with both caps attached. A square-tube band preserves two holed caps,
has volume 2, and assigns every output edge to exactly one original edge or
cut plane. Focused race checks passed.
A second oblique band uses two genuinely nonbinary section boundaries.
All eight exact model vertices satisfy one of the authored plane equations
and the volume is 1/8. The same quadratic rational endpoint proofs used by
single-plane splitting certify the shared topology before sewing.
The same-side overlapping circular cutter constructor matches independent
OCCT volume, area and centroid for the favorable blind case. Analytic tests
cover both blind and through cuts, and native reload retains verbatim
weights of both exact cylindrical charts. The existing sampled display
currently reaches its budget; this is not a browser or display pass.
An independently authored exact four-face split-cylinder STEP and a translated
signed-axis variant both pass OCCT solid validation and exact-domain import.
The rounded OCCT-generated split fixture is an independent rejection case.
The rooted writer also transfers one valid solid through OCCT's conventional
product-root reader. Exact finite line tests retain irrational knot contacts,
nonbinary line parameter mappings and closed cap hits on coincident curves.

The dense mesh validator agrees with the prior implementation over 300
deterministic topology mutations and a pinched-vertex fixture. A 16,000-triangle
sphere validation benchmark went from 31.18 ms and 10.26 MB to 8.89 ms and
3.23 MB on the local host; the representative external thread mesh is
byte-identical. This benchmark measures validation alone.

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
When the projection derivative changes at a knot, the cap uses each
original knot cell's exact affine map. A fixture verifies the original
`U=1/2` and model `X=3/8` junctions, independent piecewise-Jacobian mass
integrals, and native reload.
The regular rational graph fixture cuts the polynomial surface
`Z=X/16+X²/32+(1+X)Y/4` at `Z=3/16`. Its nonconstant denominator is
proved strictly positive, and both original trims retain the complete
rational section. Independent integration gives common volume
`49 ln(2)/512−69/2048`; OCCT agrees on area, volume and centroid for both
Boolean results. Native reload and closed contact-free meshes pass.
A denominator with an interior zero and a tampered trim with unchanged
endpoints are rejected.
The piecewise rational fixture adds an actual `|X−1/2|/64` crease and
uses two rational branches. Native mass agrees with exact strip integrals;
native reload and mesh checks preserve both original knot junctions.
OCCT validates the unsplit Boolean shells and their external areas.
Separate OCCT half-solids split at the crease for first moments; both
volumes agree within `1e-7` model units cubed, and centroids within
`3e-7` model units. The unsplit OCCT integrators showed larger residuals.

A reverse-axis cylindrical cutter with a larger axial span independently
qualifies the transverse crescent difference against OCCT area, volume, and
centroid. Native ancestry records the two original stock caps and the
cutter wall; temporary clipping planes do not acquire cutter cap ancestry.
Independent OCCT cuts cover an upper, lower, and interior transverse
notch. All retain one shell and agree on area, volume, and centroid.
The upper and lower cuts retain two analytic floor patches; the interior
notch retains four patches on the two original caps in original parameter
order, including for a reversed cutter axis. Native tests also verify
opposite floor normals, closed display meshes and no undeclared contacts.
Independent OCCT union checks cover common lower and upper caps, staggered
ends, and nested axial spans for intersecting transverse disks. All four
native results agree on shell count, area, volume and centroid. Both
operand orders and signed frames preserve the original cap ownership;
common terminal lens fans retain both owners.
The oblique asymmetric fixtures apply an exact scale-five similarity to
the same native constructions. Independent OCCT cylinders verify the
covering cut, blind cut, interior notch, and common-cap, staggered, and
nested unions. All agree on area, volume and centroid. Native tests check
original signed cap IDs, native reload, and strict failure when original
cap coordinates or the similarity cannot be represented exactly.
End-to-end transverse cylinders independently match OCCT on one shell,
area, volume and centroid. The abutting cap lens cancels in union, has no
positive common volume, and leaves difference unchanged. The exact point
classifier distinguishes the hidden lens interior from its exposed rim,
and reports canceled source cap incidence separately from the result boundary.

Convex planar through hollowing retains a closed genus-one shell for a
rectangular duct with four distinct wall thicknesses and for the corresponding
sheared stock. Both have exact ideal volume 60 and match independent OCCT
prism subtraction on area, volume and centroid. A separate test rejects an
opening whose rounded cap would move off its original plane by less than one
binary64 ULP, even though the usual output displacement tolerance is larger.
An independent wedge-prism cut verifies a duct opening on each of the
nonparallel original supports `X=0` and `X+Z=4`. It agrees on area,
centroid and volume `35/8`. Exact source-face separation accepts a
`2⁻²⁰` gap and rejects a shared opening edge.
The three-port hollow uses distinct original diagonal supports on a
seven-sided prism. Independent OCCT prismatic subtraction agrees on area,
centroid and material volume `263/16`. The native shell has Euler
characteristic −2 and retains all three original opening identities.
An independently swept and sheared OCCT cutter qualifies the tapered planar
chamfer's area, centroid and volume `569/24`. The native result retains
the original vertical edge's direction and both endpoint width pairs.
Constant widths reproduce the earlier chamfer result. Incompatible ratios
fail before any chamfer is returned.
The independent hinge draft fixture sweeps a trapezoid in OCCT. It matches
the native result on area, centroid and exact ideal volume `1136/105`.
The two original hinge axes retain their vertex IDs, exact rotation
matrices and output face mapping. Reversing an original edge and its
signed angle leaves the reconstructed solid unchanged.
Independent OCCT face sewing qualifies the variable-width rectangular
loft (volume `70`), the convex three-station loft (`56/3`), and a reentrant
waist loft assembled from convex bands (`14/3`). All agree on area,
volume, and centroid. Native station seams remain explicit, including
when adjacent planar bands share the same support.
Two independently fused OCCT boxes qualify the nonparallel miter elbow
with exact volume `5` and centroid `(13/8,7/8,0)`. Native tests retain all
four original miter edges and reject a station whose corner crosses the
miter plane even when its center is on the expected side.

Standalone STEP and IGES revolutions preserve the original signed coordinate
axis, angular and meridian ranges, units, and exact bounded profile geometry.
Independent OCCT read/write fixtures cover polynomial and rational meridians
and non-cardinal meridian phase. The native `exchange/revolution` leaf
package retains the standalone representation through versioned JSON.
IGES type 102 also retains bounded ordered direct mixed curve chains with
exact joints. These are support and wire codecs without a solid claim.
IGES type 122 retains the authored directrix and terminal point exactly.
Independent OCCT cases for both displacement signs agree after mapping
normalized displacement parameters to the physical distance used by OCCT.

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

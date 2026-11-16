# Implementation status

The full OCCT replacement is **not complete**. This document distinguishes the
implemented, tested kernel from the much larger specification. Full replacement
and general-purpose production use remain unqualified.

The latest completed paired run is [Go](evidence/main-b6335d3c-qualification-2026-11-15.json)
and [PlainCAD](evidence/plaincad-main-b6335d3c-2026-11-15.json) on
`b6335d3c`. Newer independent runs passed full Go on `01d12dab`
and all eight [PlainCAD gates on `a3b7548d`](evidence/plaincad-main-a3b7548d-2026-11-15.json).
The refreshed Go run on `a3b7548d` is still in progress; vet and
Linux/Windows amd64 builds pass that revision.
The native Linux ARM64 run pins `7569bd36`. Native Windows runtime,
broader curved modeling, independent exchange and the full release
matrix remain unqualified.

The previous paired full runs used kernel `af569683` and PlainCAD `efae4d2`.
The [Go run](evidence/main-af569683-qualification-2026-11-15.json)
accounted for all 5,714 top-level tests: 5,199 passes and 515 explicit
optional skips across 52 tested packages. All eight trimmed groups
finished, with 1,371 passes and 46 skips. Four packages have no tests.
`go vet ./...` and a Windows amd64 cross-build passed separately.
The [PlainCAD run](evidence/plaincad-main-af569683-2026-11-15.json)
passed 345 development, 66 production, 15 cross-browser, 1,691 unit and
four WASM contract cases. The build, typechecks and bundle budget passed,
the Playwright driver is pristine, and the production WASM matches the
built asset. Later commits have focused differential and race checks.

Three transverse nodes now qualify native Common, Fuse and both component
Cut orders. The new mixed crossing and tangency proof retains five signed
regions on the original analytic roofs through Common, Fuse, both Cut
orders and source-backed native reload. Canonical and independently authored
unsplit mapped STEP preserve both synchronized and asynchronous support-knot
cuts. Exact
planar STEP point-contact transfer authenticates original model lines, planes
and both incident pcurves when supplied. The optional PlainCAD STEP repair
bridge preserves authored metre and inch units while reporting physical
measurements and display coordinates in millimetres. Single and grouped
repairs passed typed Node WASM and real Chromium worker verification.
Planar IGES now transfers concave material with through-holes, one exterior
with disjoint enclosed cavities, and flat groups with independent planar
bodies and exact vertex contacts. STEP and IGES support selected original
point contacts on cavities, with independent shell containment witnesses.
The native pair proof supports complete or partial contact along two
original straight edges. It retains both directed rational ranges, native
reload and shared point and line ownership. Complete and partial edge
contact also round-trip through independent STEP and IGES.

Four transverse crossings qualify native Common, Fuse and both Cut orders,
including exact source-integrated Cut volumes and source-based reload.
Geometry also proves two crossings plus one quadratic tangency in any
original event order. The v7 mapped proof retains three exact support
cuts in one original cell, including explicitly split, unsplit foreign,
and source-native paths. The separate converted-metre and converted-inch
STEP fixtures prove their changed original owner charts and pass native,
Node WASM and Chromium qualification in their retained source units.

The active release gaps include general curved Boolean and blend assembly,
more complete STEP and IGES, current native race and runtime platform
qualification, and end-to-end acceptance of the remaining specification.
One earlier paired run used kernel `bafe7e81` and PlainCAD `efae4d2`.
The [ordinary Go run](evidence/main-bafe7e81-qualification-2026-11-15.json)
accounted for all 5,801 top-level cases, with 5,272 passes and 529 explicit
optional skips. All 1,438 trimmed cases were accounted for, with 1,391
passes and 47 optional skips. The [PlainCAD run](evidence/plaincad-main-bafe7e81-2026-11-15.json)
passed the build, typechecks, bundle budget, 345 development browser cases,
66 production cases, 15 cross-browser cases, 1,691 ordinary units and four
WASM contracts. The [newer ordinary Go run on `509cdbd3`](evidence/main-509cdbd3-qualification-2026-11-15.json)
accounted for all 5,921 top-level cases: 5,374 passes and 547 optional
skips, including 1,411 trimmed passes and 47 optional trimmed skips.
The [ordinary Go run on `a1030049`](evidence/main-a1030049-qualification-2026-11-15.json)
accounted for all 6,006 cases with 5,448 passes and 558 optional skips,
including 1,433 trimmed passes and 48 optional trimmed skips. All nine
lanes returned zero, with 52 tested packages and four without tests.
The [full native race on `d5906fde`](evidence/native-main-d5906fde-race-2026-11-15.json)
accounted for all 5,769 cases: 5,247 passed and 522 optional cases were
skipped. All 52 tested packages and the eight trimmed groups finished.
The [previous full PlainCAD run on `83e18008`](evidence/plaincad-main-83e18008-2026-11-15.json)
passed all eight stages: 345 development, 66 production, 15 cross-browser,
1,691 ordinary unit and four real WASM cases. The production WASM is
byte-identical to the build and the Playwright driver is pristine.
On `ab33a412`, the first seven PlainCAD stages passed. The ordinary
Vitest suite runs OCCT explicitly and timed out in its four-start lead
screw and nut test. The same unmodified test passed on a fresh focused
run in 39.644 seconds under the unchanged 45-second deadline. The actual
PlainSolid adapter passed the same body in 13.793 seconds with the first
candidate mesher optimization. The separate [lead fixture](evidence/plaincad-lead-fit-committed-2026-11-15.json)
now qualifies all three original one-, two- and four-start tests through
typed Go WASM at their original 45-second limit. The [failure record](evidence/plaincad-main-ab33a412-lead-control-2026-11-15.json)
and [reconciled case coverage](evidence/plaincad-main-ab33a412-partitioned-2026-11-15.json)
preserve the separate attempts. The full loopback unit rerun passed 1,690
cases and failed only the same OCCT timing control. The remaining case passed
unchanged in both a focused run and a separate full run without loopback.
All 1,691 case identities on that snapshot are accounted for. Both full
Go and PlainCAD later passed on `b6335d3c`, along with vet and the
Windows amd64 build.
The Linux amd64 cross-build on that same source also passed; neither
cross-build establishes native runtime coverage. The [corrected full
ordinary Go run on `ab33a412`](evidence/main-ab33a412-qualification-2026-11-15.json)
passed all nine lanes, with 5,557 passes and 580 explicit optional skips
across 6,137 listed cases. All eight trimmed lanes passed, with 1,468
passes and 51 optional skips. The newer ordinary and corrected race
runs were then refreshed separately. The [full ordinary run on `b6335d3c`](evidence/main-b6335d3c-qualification-2026-11-15.json)
passed 5,630 of 6,216 listed cases, with 586 optional skips and no
failures. It includes 1,502 trimmed passes and 51 optional skips. It
pairs with the full PlainCAD pass on that same kernel. The [full ordinary
Go run on `01d12dab`](evidence/main-01d12dab-qualification-2026-11-15.json)
later passed 5,666 cases with 588 optional skips across 6,254 listed
cases. The corrected `ab33a412` macOS race and the full native Linux
ARM64 `7569bd36` archive run are still in progress.
The browser suites on `b6335d3c` required scoped loopback access after
the default sandbox denied their local test servers; preserve those startup
logs separately from the browser results. Scoped browser reruns then
passed all 345 development, 66 production and 15 cross-browser cases;
build, typechecks and all four actual WASM cases also passed. A full
loopback ordinary run passed 1,690 cases and again timed out in the
unchanged OCCT control. After stopping an older known-red race snapshot,
the identical focused control passed in 25.214 seconds. A quieter full
ordinary rerun then passed all 1,691 cases; its four-start control took
23.028 seconds against the unchanged 45-second limit. The [reconciled full
PlainCAD report on `b6335d3c`](evidence/plaincad-main-b6335d3c-2026-11-15.json)
records all eight passing stages and preserves the original failures and
actual scoped rerun inputs. The production WASM is byte-identical to the
build; the Playwright driver is pristine.
The [newer uninterrupted PlainCAD run on `a296b700`](evidence/plaincad-main-a296b700-2026-11-15.json)
passed all eight gates and all 1,691 ordinary cases in the same staged
run, with scoped loopback available from the start.
The [refresh on `a3b7548d`](evidence/plaincad-main-a3b7548d-2026-11-15.json)
also passed all eight gates without a rerun: 345 development, 66 production,
15 cross-browser, 4 Go WASM and 1,691 ordinary cases.
Vet and the Windows amd64 cross-build on `016354ee` pass; native
Windows execution remains unqualified.

Planar contact now supports concave faces and holes, folded and differently
subdivided regions, different original seam stations, and one or more
strict inset footprints on an original host. STEP and IGES keep independent
authored parts. Document recipes carry named exterior faces and exact parent
edge intervals through splits, joins, edits, undo and reopen.

Native Common, Fuse and Cut cover mixed simple/order-three crossings,
distinct second- and fourth-order contacts, and three simple crossings
with a quadratic tangency, with original-source reload. The general
geometry arrangement proves exact multiplicities through degree five. Five
transverse crossings qualify source-backed Common, Fuse and both Cut orders.
The cubic-crossing and quadratic-tangency pair qualifies original-source
Common, Fuse and both Cut orders in both event orders. Native output keeps
the unmodified independent roof charts, including reversed affine stations.
Mapped v8 retains four to six exact support cuts in one original cell, including
the original unsplit author graph with both basis families and raw knots.
Mapped v9 retains four events in two original cells, five in three cells,
or three events on either side of a complete original edge with no event.
It preserves both sides of each joint and the separate stored and inferred
stations. One unsplit authored Bézier now retains two asynchronous inferred
events or one jointly inferred event with different original owner stations.
The original and converted metre and inch three-fillet STEP bodies pass
the native bridge and real Node and Chromium WASM. Four separated fillets
now qualify exact STEP transfer in the original millimetre charts and real
PlainCAD worker runtimes in native millimetres and in converted metres and
inches. The original metre proof keeps separate raw and proved finite axial
stations; the inch proof retains explicit units and bounded surface laws. Native
extrusion, canonical STEP and canonical IGES retain directed minor arcs and
separate curved cap holes. The original two-hole OCCT STEP also reopens from
a source-backed native receipt. Original IGES preserves 16 transformed arcs
and 32 explicit cylindrical pcurves with exact chart and endpoint proofs.
Directed profiles keep stable named walls and rims across edits, placement,
undo, redo, and reopen. Orthogonal reentrant fillets also regenerate. Exact
lateral offsets retain line and arc ownership and cap elevations, proving
clearance for the complete offset path before returning material. Exact
curved inset joins keep original host caps, guest walls, and separate
exposed floors inside through-openings. They qualify compact display, native
and document replay, and analytic STEP and IGES with supported placements.
The native multi-inset operation joins independent guests on either cap
and re-proves native receipts with separate floor and rim ownership.
Editable multi-inset documents resolve each named guest after reorder and
placement. The joined solid transfers through strict analytic STEP and IGES.
Named corner rounds on mixed line-and-arc profiles retain the original arcs,
shortened straight spans, and independent holes through edits and reopen.
Rounds at an axis-aligned line meeting an original circle cover internal
and external tangency, keeping the shortened circle's exact parameter map.
Two incident original arcs can also be rounded in the represented domain,
with separate parameter maps even when the arcs turn in different directions.
Rounding now accepts oblique original lines with rational unit directions,
including a corner between one such line and an original arc.
Long directed circular spans, including half-turns and authored full circles,
can now be split into exact native quarter-turn or shorter pieces. Ordered
authored stations also support minor pieces when a coordinate axis is
not exactly representable.
Complete equal curved caps also join, leaving one common original rim.
Different original rim partitions are now refined to exactly shared stations;
native and document replay re-prove the two directed source parameter maps.
Named document cap joins resolve original walls, split rims and vertices
through edits and placement.
Complete cap overlay exports one analytic solid through STEP and IGES.
Strict import uses the two original profiles to re-prove the seam maps.
Both formats also export editable named occurrences after exact composition
of the supported assembly placements.

Native Common, Fuse and both Cut orders now keep two independently projected
original roof charts. Common, Fuse and both Cut orders also qualify a
translated nonunit axis-aligned footprint with a sloping original base.
Common, Fuse and both Cut orders also preserve both original roof and wall
trims over a nonorthogonal parallelogram with a sloping original base.
Native Common also covers proved partial and nested axis-aligned footprint
overlaps, retaining the original roofs and the owning walls.
Independently authored unsplit STEP Béziers, including a spatially
nonplanar cubic, keep three inferred events with mixed joint and independent
support ownership. The independently authored rational cubic keeps exact
non-dyadic model stations without inserting new stored source knots, even
when the two original owner curves have independent projective laws.
Both original resolved roofs may also carry distinct rational off-seam
controls; strict source replay and closed display retain the same station
ownership.
Five separated fillets on an original OCCT tilted pentagonal prism now qualify
original millimetre, metre and inch STEP with independent antipodal owners.
Native race on `491b18f1` and ordinary Go on `5181d219` both found a
missing exact axis extremum in the same five shaded STEP repair meshes.
All five now pass the focused ordinary, native race and actual Node/WASM
checks with the original radial normals restored. Full ordinary Go, native
race and PlainCAD runs on immutable `ab33a412` test that correction.
An independent tilted STEP bore now qualifies strict source authentication
of both original cap holes and the inward-cylinder seam, exact native
material, and strict canonical STEP reopen. PlainCAD's optional bridge
(`8e9cae2c`) retains the open bores through typed worker import, native
receipt replay, display and strict re-export. Native race and actual
Node/WASM pass on main; isolated real Chromium workers also pass.
Vet and Windows amd64 cross-build pass on `ab33a412`. The converted five-fillet
inputs passed the focused native bridge and both actual WASM runtimes, with
one legacy timing retry recorded. Native stepped Fuse (`810e2014`,
`8a0b3e80`) keeps the original roofs and base across one- and two-axis
partial and nested footprint offsets, including a concave union base.
The exact complement proof (`35bff0d0`) retains chamber volumes, walls,
strip adjacency and original point contacts. It distinguishes contacts
between separate components from a pinch inside one connected component.
Native Cut (`bce76c45`) builds separate exact shells for admitted layouts
and refuses a proved pinch before publishing geometry. The source mesher
for stepped Fuse (`068bb176`) retains original vertices, closed topology
and measure across one- and two-axis layouts. Cut previews (`9fa3ce82`)
prove that each displayed shell is outward and embedded and that the shells
meet only at the original points. The existing Common preview passed all 32
checked layouts (`81308396`). Exact two-transverse Cut (`ace75343`) now
builds separate closed native components with the qualified tighter preview
(`5d6a15c1`). Two transverse contacts also qualify exact clipped Fuse
(`3eac2245`) and Common (`a25a8fc8`), with their original wall and source
recipes. Three exact transverse contacts qualify native Fuse (`0c8baacf`),
separate native Cut (`b33ae2a6`), and native Common (`916f4a5c`). Cut
retains all three contact pairs between distinct positive components and
refuses a contact inside one connected component. Common keeps one closed
shell, six original roof sectors and both uses of every source edge. Adjacent
rational STEP model cells (`8672cf02`) retain inferred roots and both
independently authored sides of a stored joint through native replay,
strict STEP and closed display. Distinct transverse rational roof weights
passed independently authored transfer and display (`cb3bcd42`). The
stepped Fuse transfers through analytic STEP (`936ef7be`) and IGES
(`67db7b54`) with an explicit decimal pole bound where needed. Three original
rational model cells (`c3fb8b82`) now retain two original support joints.
Native v12 (`b6335d3c`) independently authenticates one shared and one
unilateral support change. Separate clipped Cut components transfer to
analytic STEP (`56fc65ef`) and IGES (`c75fa2fd`), retaining distinct exact
vertex occurrences at their original shared points. The STEP companion for
three transverse Cut contacts (`1478182f`) preserves four or five separate
closed solids, and three-contact Common (`2e5712a9`, `90fbc893`) transfers
one valid 15-face solid in both checked STEP and IGES layouts. Three-contact Cut also passes
[independent IGES transfer](evidence/clipped-three-transverse-cut-iges-2026-11-15.json)
(`a6763a04`), preserving separate vertex occurrences and the exact third
contact. Source-aware Cut and Common previews
(`32b1df05`) retain the original vertices and closed incidence through
native replay. Three-contact Cut optionally proves a per-component
display volume bound against its exact source integral (`effd488a`).
Native four-contact Fuse (`7586a15c`) passes all 12 exact material and
source reload cases, including closed original-source previews (`11e475fb`).
Its two independent STEP and IGES readbacks also passed (`13f1d870`).
The transverse two-contact Common transfers through STEP
(`c705e026`) and [IGES](evidence/clipped-common-iges-2026-11-15.json)
(`6e72ff02`) with original roof and wall charts and both owner pcurves.
The higher-order mixed Common now transfers by adding one exact seam to
the original lens roof (`a5df4358`, `80c3a768`, `fe3d5380`). STEP and
IGES keep the same vertices, material and original supports. The pinned
reader refused the old unsplit wire in both formats; those failures
are preserved as negative controls.
The exact projection filter (`d883a33f`)
and helical mesher (`61ee3e80`) reduce qualification and display time.
The original unilateral support kink now passes native v12 and strict
STEP (`38d53120`), with a pinned closed solid and unchanged original
homogeneous weight ratios. A moved original knot still rejects. Native v13
and strict STEP now retain ordered support changes across four to seven
original cells (`7a196ec6`), including an independently authored
four-cell source and three different joint masks. Its closed exact display
is qualified (`821b8bcd`). The opt-in native path also proves a
0.35 radian facet-to-source angle and a distance below 0.09 mm after
refining the original closing edges and their neighboring spline cells.
The PlainCAD angular control measures normal turning across a facet and
needs a separate proof. Four original eccentric frustum rim chamfers
now preserve both circular axes and prove the shell gap at all runouts
([independent evidence](evidence/eccentric-frustum-rim-chamfer-2026-11-15.json),
`b600bde2`). The [diagonal cap fallback](evidence/eccentric-annular-cap-nesting-2026-11-15.json)
(`a296b700`) proves both whole wall chains stay separated before filling
the rings. [Exact STEP and IGES output](evidence/eccentric-annular-chamfer-neutral-2026-11-15.json)
keeps both axes, the original four chamfers and all ten conical wall
partitions for aligned and diagonal placements. [Strict independent
STEP](evidence/eccentric-annular-chamfer-step-import-2026-11-15.json)
also reconstructs the original full-rim variant with exactly authored
generators and preserved source IDs. The [eccentric bore inner rim fillets](evidence/eccentric-cylinder-bore-rim-fillet-2026-11-15.json)
(`d930469f`) retain the unchanged outer cylinder, original axes and cap
planes with exact quarter-toroid transitions and positive clearance.
The [strict independent STEP reader](evidence/eccentric-bore-rim-step-import-2026-11-15.json)
now reconstructs the pinned upper fillet from the original analytic
STEP curves and incidence, preserving source IDs through exact circular
subdivision. It also reopens all three existing STEP exports with
byte-identical re-export. The [strict independent IGES reader](evidence/eccentric-bore-rim-iges-import-2026-11-15.json)
reconstructs all three lower, upper and paired variants from original
OCCT files. It preserves original file units, directory and edge
provenance and the signed volume on independent re-export.
The [explicit PlainCAD v13 repair](evidence/plaincad-v13-ordered-step-2026-11-15.json)
(`399957cd`) passed the native and actual Node/WASM source, closed mesh,
strict STEP export and fresh reopen. [Two typed Chromium workers](evidence/plaincad-v13-worker-chromium-2026-11-15.json)
(`0debedd8`) independently passed the same strict reopen on the pinned
app. Both loaded Go WASM, with no OCCT asset requests. The public route
currently certifies distance, with no finite normal-angle claim.
An [opt-in native knot-aligned mesh](evidence/mapped-v13-knot-aligned-native-2026-11-15.json)
(`66d273d2`) proves a tighter distance in the original one-sided roof
cells. It retains the previously qualified public route and mesh bytes.

Native variable-count Cut (`0091b243`), Common (`548ee371`) and Fuse
(`72f5b907`) discover one through five original transverse contacts.
Exact material and original-source replay pass, and Cut refuses
a contact inside a single connected component. Single-body Common
and Fuse transfer through STEP and IGES (`b8364dae`), preserving the
original four rays at every accepted junction. The shared Cut
STEP and IGES writers (`5a03aaca`) qualify one through three. Higher
counts still need [thin-shell orientation portability](evidence/variable-transverse-cut-transfer-2026-11-15.json):
pinned default readback automatically reversed some separate positive
four- and five-contact lenses even though its topology validator passed.
The [independent orientation diagnostic](evidence/clipped-cut-occt-shell-orientation-2026-11-15.json)
isolates the default shell-orientation repair on unchanged input bytes.
The public writers refuse those counts before they produce records.

Nodal cubic discovery (`9f054eb5`) independently recovers a closed loop
and two attached arms on original projected spline supports. Native
Common (`4cf4205c`) closes the original roof sectors with one exact
owner-chart seam where the exterior face needs separate wires. Both
source orders retain the four node rays, exact material and display.
Native Fuse (`8debec04`) reuses the same sector proof and [STEP and IGES](../exchange/NODAL_CUBIC.md)
qualify the one-body Common and Fuse in both source orders (`d3a3187b`).
Native Cut (`85dcc1ba`) keeps the separate loop and arm layers with one
original point contact; its reverse order rejects a proved pinch. The
admitted order now also transfers through STEP and IGES (`bffac9bc`),
with independent positive signed volume for both separate components.
Exact cuspidal cubic discovery (`ffc10247`) separately proves a genuine
zero first jet, a shared second jet and opposite third jets in the
original owner charts. Nodal and cuspidal discovery also prove the same
signed equation on each original knot cell. This accepts an actual
shared carrier crease away from the required jets and refuses a changed
residual or an ambiguous cusp or node tangent. A cusp on a longitudinal
crease can also be proved when both germs depart on the same original
side, including a reversed owner clock. Native cuspidal Common,
Fuse and both Cut orders now also qualify on an actual shared carrier
crease in reversed charts, including native replay and independent
volume checks. The geometry also distinguishes an isolated real double
point from both a crossing node and a cusp. When the same cubic has a
separate regular branch in the footprint, the proof keeps its two
original exits and shows the isolated point is not on that branch.
Native Common and Fuse use the complete section and preserve the isolated
contact inside an unsplit original face. The regular branch, when present,
can exit through any original stock side. Both operations re-prove source
ownership on native replay. Independent STEP and IGES readback confirms
the original signed material and classifies the isolated point in
exactly one reopened face interior without placing a seam there.
Native Cut retains the separate regular cap on all four exit sides.
The reverse order is refused because it pinches at the isolated point.
The dedicated Cut preview refines until exact represented volume and
collision checks pass. The original cap and both directed roof curves
also reopen in STEP and IGES.
Curved trim quadrature splits at the certified rational source knots.
[Creased cuspidal transfer](evidence/cuspidal-creased-neutral-2026-11-15.json)
now qualifies both orders of Common, Fuse and Cut in STEP and IGES,
retaining both original supports and the exact cusp while introducing
only the supported rational seam splits.
Native Common (`fea3d0b4`) and Fuse (`750c9e80`)
preserve this singularity in both source orders and in closed displays.
[Independent STEP and IGES readback](../exchange/CUSPIDAL_CUBIC.md)
now qualifies Common, Fuse and both Cut orders, with the zero first,
shared second and opposite third derivatives on the actual reopened
model curves. Native cuspidal Cut qualifies in both
source orders with exact closed shells and original cusp derivatives.
The dedicated source-driven preview now closes both Cut orders on the
qualified polynomial roofs, retaining the exact cusp vertices and
enforcing a source volume bound. The generic solid preview remains
separate. Broader contact discovery, creased-source display and advanced
edge modifications remain in progress.
Full ordinary Go on `01d12dab` and PlainCAD on `a3b7548d` have passed.
Wider independent STEP and IGES graphs are still required.
Wider contact discovery, advanced modeling,
exchange and native platform gates remain open.

The two PlainCAD integration branches are in `main` through implementation
merge `8f76f16b` and verification merge `012096ee`.
The [full ordinary Go run](evidence/main-6c4ab7c4-qualification-2026-11-14.json)
on `6c4ab7c4` accounted for 4,567 passing top-level cases, 421 explicit
optional skips and five packages without tests. All 1,081 trimmed-solid
cases were accounted for, with 1,068 passes and 13 optional skips.
The [earlier full PlainCAD qualification](evidence/plaincad-main-acf376e1-2026-11-14.json)
uses provider `acf376e1` and app `efae4d2`. Its WASM build, typecheck, bundle
budgets, 345 development cases, 15 cross-browser cases, 1,691 ordinary
units and four WASM contract cases passed. The production control passed
65 of 66. Chromium evicted a WASM response from the inspector cache. All
66 production cases passed when the isolated test driver's capture buffer
was enlarged. The driver was restored after that run.

The [earlier native race qualification](evidence/native-main-18204a56-2026-11-14.json)
passed all 50 packages with tests outside `brep/trimmed`. Three original
trimmed watchdogs elapsed. The resumed run selects exactly the 804
remaining top-level cases using the same production source and the stale
degree-ceiling assertion corrected in `313dfb62`. The continuation finished
all 804 cases. The combined trimmed inventory has 1,023 passes and 13
optional skips, with no duplicate or unresolved cases.
General curved Boolean assembly, advanced surface modeling and broader CAD
exchange remain required under the specification.

The [ordinary Go rerun on `4078f85a`](evidence/main-4078f85a-qualification-2026-11-14.json)
accounted for 4,807 top-level passes and 440 optional skips across 52
tested packages. All three trimmed groups completed with 1,204 passes
and 15 optional skips, matching all 1,219 listed cases. Three independently
authored AP242 fixture inventories initially failed because the STEP
preflight catalog did not recognize `SPECIFIED_HIGHER_USAGE_OCCURRENCE`.
The entire pinned STEP package passed after the one-line catalog correction
in `36380200`.

The [full refreshed macOS run](evidence/main-35c79311-qualification-2026-11-14.json)
on `35c79311` passed 52 tested packages and 4,923 top-level cases, with
460 optional skips and four packages without tests. All three trimmed
groups matched their independent inventories, with 1,266 passes and
24 optional skips across 1,290 cases. The [full Linux arm64 qualification](evidence/main-35c79311-linux-arm64-qualification-2026-11-14.json)
on the same snapshot matched those totals after its complete geometry
package was rerun with the exact parameter-domain correction in
`c855ecaa`. The original run's four geometry failures are preserved.
All three original Linux trimmed groups passed. The pinned Windows
amd64 cross-build passed. [Linux geometry qualification](evidence/linux-geometry-go126-2026-11-14.json).
The newer `0402eb63` platform run passed 36 focused Linux arm64 cases
with 10 optional OCCT skips, and its Windows amd64 cross-build passed.
[Focused platform qualification](evidence/main-0402eb63-focused-platform-2026-11-14.json).

The [preceding full PlainCAD qualification](evidence/plaincad-main-35c79311-2026-11-14.json)
uses the same pinned kernel and app `efae4d2`. The corrected toolbar
locator passed the full 345-case development run on its configured
origin. The direct asset-capture fixture passed all 66 production
cases with the standard Playwright driver. The WASM build, typecheck,
bundle limit, 15 cross-browser cases, 1,691 ordinary units and four
WASM contracts also passed. The original failure controls and the
interrupted alternate-origin development run are retained in the record.
The [latest full PlainCAD qualification](evidence/plaincad-main-42ab9727-2026-11-14.json)
on `42ab9727` includes the two committed test fixture corrections and
the native oblique fillet bridge. The WASM build, typecheck, four
contracts, 345 development cases, 66 production cases, 15 cross-browser
cases and 1,691 units passed. The standard driver matches its pristine
copy. The production WASM hash matches the built package. The temporary
output from the follow-up Go run on `abb80f9f` is unavailable. Fresh Go
and PlainCAD runs on `48b0ce58` started on 2026-11-15 with logs under
`.worktrees/qualification/2026-11-15` in this workspace.

New focused milestones on `main` qualify transverse spherical-lens erosion and
dilation, a closed display with an exact shared mapped-spline seam, Common and
Fuse through one original transverse curved node, and two bounded planar
healing operations. The latter orient nested material/cavity shells after
exact embedding checks and contract one selected coplanar short edge while
preserving material and source face indices. Arbitrary short edges, intersecting
shell repairs, and modification of original curved supports remain open.

The mapped shell now persists its original source recipe, separately proves
continuous embedding, and converts to a native Solid for the rectangular
family. Native reload, display, exact rational queries, mass and independent
OCCT construction pass. The explicit native Cut result preserves two regular
components and their one exact shared point; the ordinary solid admits no
unproved touching shells. The opt-in PlainCAD bridge STEP repair and its
compact signed-frame mesh passed the five-fixture Go race and actual WASM runs.
The pinned OCCT reference has been rebuilt, and the new live spherical,
curved-node, mapped-shell, and original STEP repair comparisons passed.

The [fresh full PlainCAD qualification](evidence/plaincad-main-48b0ce58-2026-11-15.json)
passed 345 development, 66 production, 15 cross-browser, 1,691 ordinary
unit and four WASM contract cases on kernel `48b0ce58` and app `efae4d2`.
The build, both typechecks and bundle budget passed, and the production
WASM bytes match the fresh build. The [full pinned Go qualification](evidence/main-48b0ce58-qualification-2026-11-15.json)
passed 5,143 top-level cases across 52 tested packages, with 503 explicit
optional skips and four packages without tests. The independent inventory
matches all 5,646 cases. Group zero combines its passing original prefix
through the long triple-radial case with 231 disjoint complementary cases.
Newer changes have focused checks; `f5bfaac2` also passed `go vet ./...`
and a Windows amd64 cross-build.

Two transverse nodes now qualify native Common, Fuse, and a separate Cut
component result with exact contact graphs and deterministic reload. A general
planar vertex-contact assembly proof preserves separate indexed components.
Canonical mapped shell STEP reopens with all 34 original coedges, and the
foreign reader accepts the independently authored and reordered OCCT graph.
Native tilted STEP repair records preserve the original source bytes and
re-prove the exact rigid placement, changed curves and original owner ranges.
Analytical queries and a separate certified display recipe cover irrational
sphere-lens rims. The next active work includes exact tangency arrangements,
multi-span mapped roof supports, and canonical transfer of rationally tilted
STEP repairs. Broader curved assembly, exchange and qualification remain
required.

Ordered roof Common and matching-footprint Fuse retain genuine original
spline creases and imprint exact axis-aligned crossings inside original
knot rectangles. Adjacent cap tiles share the same exact edge. Crossings
that require nonrepresentable native vertex coordinates or general curved
imprints remain open. Native algebraic reconstruction retains original
quadratic and cubic source p-curves on outer and hole loops and re-proves
mixed exact and implicit tube uses on JSON reload when both chart paths
are affine, including curved model paths. The bounded automatic path
proposer now certifies the original curved fixture without supplied seeds.
Ordered roof Cut retains creases
in full-footprint bands and strictly enclosed cavities. The noncircular
rational station loft has exact point classification, isolated line
events, coincident cap and selected wall spans, exact sections, and
certified volume, centroid and boundary-distance bounds. Proper
rational placements preserve measures, contacts and original owners.
Coaxial loft Common, Fuse and strict through-bore Cut preserve a shared
original weighted profile. The resulting bore supports exact annular
sections and splits, isolated lines and cap coincidences, and certified
closed display with original rim ancestry.

The STEP reader now qualifies standard named product colors on an
independently authored AP242 assembly, including repeated product use.
The flat contextual occurrence reader authenticates whole-body override
ownership. Native persistence re-proves the original STEP. Creating,
editing and removing overrides retains unrelated shape and pose records.
Bounded nested ordinary and SHUO overrides now support shared and
path-specific whole-body appearance. Face-specific occurrence styles
remain open. Wrapped spline p-curves require exact original pole and
knot coordinates before normalization.

The [current PlainCAD runs](evidence/plaincad-main-18204a56-2026-11-14.json)
use kernel `18204a56`. All 15 Chromium, Firefox and WebKit cases, four
selected WASM contracts, 1,691 ordinary units, typechecks and bundle
budgets pass. The original production run passed 65 of 66. Enlarging
the local Chromium response capture buffer recovered the failed CSP
case and all 66 passed in the full rerun. Vite denied fonts through the
original dependency symlink, causing three development failures. All
three passed when the same dependencies were placed inside the isolated
app. The corrected full development run passed all 345 with the original
fixtures, configuration and port. On newer kernel `1d938079`, the same
PlainCAD app separately passed the three modeling cases and four WASM
contracts. The [newer qualification checkpoint](evidence/main-6c4ab7c4-qualification-2026-11-14.json)
records the full normal Go and PlainCAD reruns on `6c4ab7c4`. The WASM
build, type checks, JavaScript bundle budget and four selected kernel
contracts passed. All 345 development, 66 buffered production and 15 cross-browser
cases passed, with four WASM and 1,691 ordinary unit passes. The ordinary
production run reproduced the inspector capture eviction and passed 65 of
66. All refreshed normal Go groups finished successfully.

The native prism Boolean tier now preserves original parent faces for
nested, abutting, covered, and side-opening profile transitions. Forty-eight
OCCT comparisons include through, one-ended and interior side openings in
two exact poses. Simultaneous affine source edges now authorize the full
face-pair graph and fragment all original affine faces. A pinched original
region can be subdivided along certified noncrossing auxiliary edges. The
original two shells reassemble from 60 triangles and 35 paired auxiliary
edges. Convex affine Common, Fuse and manifold Cut select fragments by
exact opposite-material occupancy and preserve original face ownership.
Original weighted rational trim clipping and fragmentation now
cover degree eight, while exact distance witnesses include original trim
splines crossing an irrational support-knot event. Coincident original
coedge spans contribute closed-face section intervals with original
algebraic endpoint identities. Mapped whole spline face equality retains
the original outer and hole identities in all eight supported chart
symmetries, including bounded one-to-many trim refinements and hole
permutations. Exact orthogonal and convex polygon chart overlays reconstruct
material boundaries and hole loops. Convex affine Cut can certify two
separate manifold components meeting at a single proved original point.
Inclined cap joins with nonparallel original extrusion directions now pass
all face-pair checks and preserve original source faces and overlap-edge
owners. Exact closed ellipse tangency produces a regularized empty Common
and a source-preserving Cut. Rotated tangent sections retain both originals
across knot events, and nonlinear double-contact arcs retain original coedge
parameters through rational spline trims.
Closed original spline contacts also qualify external and nested material
when a definite repeated conic can be proved over the complete common
chart. Nested Common and Fuse retain the original inner and enclosing
shells respectively. The opt-in native Boolean entry discovers the contact
pair and projection from the source geometry. Coincident nonorthogonal charts now reconstruct
concave regions with multiple independent straight holes, including
concave hole boundaries and disconnected overlap. Convex polygonal cap
joins support original-model native Fuse and exact source UV such as
`8/9` through the rational endpoint option. Exterior closed-contact Fuse
keeps two distinct original components. Contained Cut keeps the original
outward stock and the inward cavity shell, with opposite oriented
crossings at the same contact. Exact crossing polygon shells now reach
native Common, Fuse and Cut under the original-fragment dispatch option.
The polygon proof can retain paired junctions on both original source
edges, including four-germ meetings with exact owner incidence.

Independently authored anisotropic filled and bored NURBS round-trip in
STEP as individual solids and flat or nested shared-product assemblies.
The exact rational occurrence path retains original frame and use records.
A source with compound oblique parent transforms retains its exact
`(-7/25, 24/25)` planar direction components on native reload. Generated
flat occurrences accept lossless STEP decimal frames and reprove every
original material support and occurrence before writing. Generated
nested occurrences additionally reprove the composed leaf placements.
The independently authored quadratic planar AP242 reader retains
degree-two tensor supports and coedge p-curves under a complete exact
affine identity, and reloads their original STEP source. The higher-degree
reader extends those identities to independent single-span degrees from
one through eight. Multispan polynomial tensor supports, p-curves and
three-dimensional model curves retain their original knot and ownership
records after exact Greville certification. The combined rational AP242
route preserves positive variable-weight supports and p-curves alongside
independently knotted model edges. Supports with one simple interior knot
in each direction certify the homogeneous identity on all four original
rectangles. The polygon mesher preserves
exact homogeneous supports and original UV-use lineage on concave trims,
including certified two-sided bounds across knots and shared native
sample IDs on a closed multispan shell.

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
The arbitrary-direction line sweep isolates up to eighth-degree restrictions,
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
inertia have separate original OCCT comparisons. Direct column integration
now encloses the volume to `1/1000` for all three original operations and
certifies raw second moments, centroid and inertia, including exact transport
through a proper rigid occurrence.
Prepared roof queries also admit original single-span positive rational
surfaces through degree four in each chart axis. The weighted cavity
kept its exact roof hit, quotient normal and floor distance, and an
independently sewn OCCT rational model gave the same line parameter.
Separate original bicubic and biquartic roofs retain their different sixth-
and eighth-degree line roots. The specialized polynomial mesh proof keeps
its existing scope. Exact knot refinements also qualify when every original
span reduces to the same homogeneous polynomial on the whole chart. Genuine
piecewise heights also qualify for bounded point, line, normals and volume or
mass queries once the original six-face roof-box proof passes. The line
sweep does not create artificial faces at interior knots; an exposed crease
can return both one-sided normals under the original face ID.
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

Transverse coaxial torus-cylinder Common, Cut, and Fuse build native analytic
solids with algebraically identified rims. Paired STEP read and write preserve
analytic supports and prove the replacement error for irrational rim heights.
Fuse retains the actual caps; Common and Cut expose that those cap positions
cannot be recovered from the retained STEP faces. The writer also preserves
proper finite-decimal rational placements. Exact planar, finite cylinder and
ring torus bodies now exchange in a bounded two-level product hierarchy,
keeping local bodies, original occurrence paths and exact placements through
source-bound native reload. Both exchange paths have independent live OCCT
comparisons.

The [native short suite](evidence/plainsolid-native-981161dd-short-2026-11-14.json)
completed across all packages on integration revision `981161dd`.
It records 39 package passes, four packages without tests and no
failures. Short-mode exclusions and full qualification remain open.

## Immediate priority: PlainCAD compatibility

The [selectable PlainSolid provider qualification](evidence/plaincad-provider-integration-verified-2026-11-14.json)
pins the PlainCAD app at `9779044` and its initial verified PlainSolid build
at `f571a964`. Full provider reruns on [kernel 65441a2d](evidence/plaincad-provider-kernel-65441a2d-2026-11-14.json)
and [kernel e60f34e7](evidence/plaincad-provider-kernel-e60f34e7-2026-11-14.json)
passed 345 development, 66 production and 15 cross-browser cases with no
skips, retries, or flaky results. The ordinary app suite passed 1,691 cases
and four actual-WASM contracts. The isolated PlainCAD provider branch
selects PlainSolid in the app and its workers, with a fingerprinted WASM
file and the matching Go runtime. Later kernel milestones have focused
browser and exchange evidence.
The [latest selectable provider qualification](evidence/plaincad-provider-kernel-fa857e59-2026-11-14.json)
pins app `efae4d2` and kernel `fa857e59`. It independently passed all
345 original development, 66 production and 15 cross-browser cases,
1,691 ordinary tests and four actual-WASM provider contracts.


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
exchange their elliptical intersection arcs through STEP. One, two or all
three Holes can stop beyond the full intersection and retain their planar
floors from either entry side of each cardinal axis. Disjoint through cutters on two or three
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
supports and recovering the native B-rep on readback. Adaptive STEP spans qualify the 2 mm and 2.5 mm offsets for 5 mm and
4 mm radii. The 0.75 mm covered-width offset also round-trips with bounded
local STEP interpolation. Smaller clearances and general oblique cutter intersections
remain open. A blind Hole
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
with compact STEP and watertight STL. A floor may now stop within the
other cutter’s full-radius bound if exact transverse clearance proves
it is beyond the actual crossing. Two closer-floor models pass
rebuild, STEP, project reopen and watertight STL. Bounded conforming
refinement handles the model that previously exceeded the worker deadline. A covered-width unequal crossed
pair, including finite stops beyond the crossing, can retain up to two
disjoint cardinal through companions. Some placements reach the existing
adaptive mesh budget. A single oblique through cutter in rectangular stock now
retains two complete elliptical apertures on opposite or adjacent faces.
The native bridge verifies the finite tool covers both openings before
scoped Cut, retains the stock face IDs, and generates watertight STL.
Its STEP exchange preserves both analytic ellipses and the cylinder support.
An equal-radius cardinal bore and a concurrent oblique bore in local
4:3, 3:4, 12:5 or 5:12 axis ratio retain their exact crossing ellipses and all four
full stock openings through STEP and STL. The cardinal axis may lie
along X, Y, or Z. Both slope signs and transverse coordinate directions
retain the same exchange proof. The cardinal Hole may retain its blind floor
beyond the complete crossing. Both entry directions exchange exact STEP,
and a subsequent coaxial cut can deepen or open the imported floor.
Grouped oblique crossings discard fully covered coaxial tools after
proving the retained spans. Their STEP matches the visible-only pair.
The oblique Hole itself can carry a circular blind floor after the
complete crossing. Both entry directions and a later deeper Hole
retain canonical STEP. Both equal-radius tools can be blind in the same
part when their floors remain beyond the entire crossing.
Two or three separate through Holes can now combine oblique and cardinal
axes in one rectangular stock body. Native sewing preserves the stock
faces, and STEP preserves each circular or elliptical opening and its
cylinder axis. Matched angular strips preserve the exact elliptical rims
and bound the mesh work for steeper and three-component axes. The authored
2:3 pair reopens through STEP and exports a watertight 276-triangle STL.

The existing PlainCAD Arc Extend workflow now exports and reimports the
quarter-sector extrusion with its exact cylindrical wall and circular
edges. Circle Trim also exchanges its half-disk extrusion. Both authored
models reopen and export watertight STL.
The driven arc-and-chord workflow retains its solved near-semicircular
angle through STEP. The reader matches the axial seam parameters and
complete represented graph. Qualified minor and major segments keep the
circular support and chord. Equal-circle and unequal circle/arc lens
extrusions also retain both analytic cylindrical walls and their shared
vertical seams. Both original XY workflows exchange exact solids and
reopen with watertight STL. The arc-only lens retains all four original
sketch fragments and produces the same STEP as the equivalent circle lens. The construction-axis Revolve workflow
round-trips the exact toroidal surface and planar caps of its quarter
sweep. Full rings and larger signed sweeps also pass the native STEP
gate, including reflection and non-millimeter readback. The Edit Revolve
quarter sweep with a coaxial through cut preserves both cylinders, the
planar caps and the radial walls. A STEP-reimported part can enlarge
the bore and reproduce the canonical output of a fresh cut. Cylindrical
Revolve sectors with the axis intact likewise retain their partial angle
and planar radial faces through STEP and accept a subsequent coaxial cut.
Full concentric tubes use the periodic two-wall STEP graph and retain
the coaxial cut recipe after import, including independent and metric files.
Circular rim fillets on XY, XZ and YZ exchange their exact rational
quarter-torus bands and preserve the native single- or dual-rim geometry.
Equal-setback circular rim chamfers on all three working planes exchange
the exact ruled frustum bands and the retained cylinder. The metric and inch
socket-head fastener blanks also retain their stepped cylindrical shaft,
annular shoulder, and blind polygonal socket through STEP and STL.

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
The [unequal crossed pair and companion reconciliation](evidence/plaincad-reconciled-3a36a277-development-skew-companion-holes-step-2026-11-14.json)
adds worktree revision `e658813e` to review `3a36a277`. Native and
live STEP checks and all 17 original browser cases passed; 841 app and
2,023 kernel source hashes matched. The [authored model](evidence/plaincad-reconciled-3a36a277-supplemental-skew-companion-step-2026-11-14.json)
retained both blind floors and its separate through passage in STEP and
watertight STL. Independent OCCT readback reproduced the reference measures.
The [cardinal and oblique crossing reconciliation](evidence/plaincad-reconciled-0cc754b3-development-crossed-oblique-holes-step-2026-11-14.json)
adds worktree revisions through `7a6f097f` to review `0cc754b3`.
The focused Go and live STEP checks and all 17 original browser cases
passed, verifying 841 app and 2,035 kernel source files. The [authored model](evidence/plaincad-reconciled-0cc754b3-supplemental-crossed-oblique-step-2026-11-14.json)
retained its 16 analytic elliptical trims, reopened through STEP, and
produced watertight STL. The actual exported STEP matched live OCCT
reference measures.
The [disjoint cardinal and oblique reconciliation](evidence/plaincad-reconciled-c2c00be9-development-disjoint-oblique-holes-step-2026-11-14.json)
adds worktree commit `b284a071` to review `c2c00be9`. Native STEP
checks and all 17 original browser tests passed, verifying 841 app and
2,042 kernel source files. The [authored separated-bore STEP](evidence/plaincad-reconciled-c2c00be9-supplemental-disjoint-oblique-step-2026-11-14.json)
reopened successfully and preserved its watertight STL. Live OCCT readback
reproduced volume, area, and centroid.
The [compact oblique bore reconciliation](evidence/plaincad-reconciled-fab4062f-development-compact-oblique-holes-step-2026-11-14.json)
adds worktree revision `e521a39c` to review `fab4062f`. Native checks and
all 17 original browser cases passed with 841 app and 2,049 kernel source
blobs verified. The [authored cardinal and steep bore model](evidence/plaincad-reconciled-fab4062f-supplemental-compact-oblique-step-2026-11-14.json)
produced byte-identical STEP and watertight STL from both checkouts. Live
OCCT STEP readback reproduced volume, area, and centroid, and matched the
independently constructed Boolean's volume and area within `1e-6`.
The [reciprocal oblique crossing reconciliation](evidence/plaincad-reconciled-7c71270e-development-reciprocal-oblique-holes-step-2026-11-14.json)
adds worktree revision `6c0c783e` to review `7c71270e`. Native tests and
all 17 original browser cases passed on a clean archive, with 841 app and
2,052 kernel source blobs verified. The [authored crossing](evidence/plaincad-reconciled-7c71270e-supplemental-reciprocal-oblique-step-2026-11-14.json)
produced byte-identical STEP and 404-triangle watertight STL from both
checkouts. Live OCCT readback confirmed the exported solid's volume,
area, and centroid.
The [multiple blind floors reconciliation](evidence/plaincad-reconciled-ef8179f7-development-multi-floor-triple-holes-step-2026-11-14.json)
adds worktree revision `e60f34e7` to review `ef8179f7`. Native tests and
all 17 original browser cases passed, with 841 app and 2,056 kernel source
blobs verified. The [authored three-floor crossing](evidence/plaincad-reconciled-ef8179f7-supplemental-multi-floor-triple-step-2026-11-14.json)
produced identical STEP and watertight 588-triangle STL in both checkouts.
Live OCCT independently confirmed the exported solid's volume, area, and
centroid.
The [Arc Extend and Circle Trim reconciliation](evidence/plaincad-reconciled-b88d0aa9-development-arc-circle-holes-step-2026-11-14.json)
adds worktree revisions through `39a151c6` to review `b88d0aa9`. Native
round trips and all seven original browser cases passed, with 841 app and
2,065 kernel source blobs verified. The [authored quarter sector](evidence/plaincad-reconciled-b88d0aa9-supplemental-quarter-sector-step-2026-11-14.json)
and [authored half disk](evidence/plaincad-reconciled-b88d0aa9-supplemental-semicircle-step-2026-11-14.json)
produced byte-identical analytic STEP and watertight STL in both checkouts.
Independent OCCT readback confirmed both solids' volume, area, and centroid.
The [driven circular-segment reconciliation](evidence/plaincad-reconciled-d97fd910-development-circular-segment-holes-step-2026-11-14.json)
adds worktree revision `11d57e9f` to review `d97fd910`. Native and all nine
original browser checks passed, with 841 app and 2,070 kernel source blobs
verified. Both [authored workflows](evidence/plaincad-reconciled-d97fd910-supplemental-circular-segment-step-2026-11-14.json)
passed. The exported STEP structure matched the worktree's; numerical tokens
differed by at most `8.89e-15`. The 264-triangle STL was watertight, and
independent OCCT reproduced volume, area, and centroid.
The [circular-lens reconciliation](evidence/plaincad-reconciled-2154f73d-development-circular-lens-holes-step-2026-11-14.json)
adds worktree revision `e5f4a249` to review `2154f73d`. Native checks and
all eight original browser cases passed, with 841 app and 2,080 kernel source
blobs verified. Both [authored lens exports](evidence/plaincad-reconciled-2154f73d-supplemental-circular-lens-step-2026-11-14.json)
produced identical STEP and watertight STL in both checkouts. Independent
OCCT reproduced the volume, area, and centroid of the equal and unequal lenses.
The [construction-axis torus reconciliation](evidence/plaincad-reconciled-f947e588-development-torus-sweep-holes-step-2026-11-14.json)
adds worktree revision `ff55e0f6` to review `f947e588`. Native checks and all
five original browser cases passed, with 841 app and 2,085 kernel blobs verified.
The [authored quarter torus](evidence/plaincad-reconciled-f947e588-supplemental-torus-sweep-step-2026-11-14.json)
retains byte-identical STEP after reimport and reopen and a watertight
24,448-triangle STL. Independent OCCT confirmed volume, area, and centroid.
The [bored quarter Revolve reconciliation](evidence/plaincad-reconciled-673b6d51-development-quarter-annular-holes-step-2026-11-14.json)
adds worktree revision `f5b7e760` to review `673b6d51`. Native checks and
all six original browser cases passed, with 841 app and 2,093 kernel source
blobs verified. The [authored Edit Revolve export](evidence/plaincad-reconciled-673b6d51-supplemental-quarter-annular-step-2026-11-14.json)
retained byte-identical STEP after reopen and a watertight 3,072-triangle
STL. Independent OCCT reproduced volume, area and centroid.
The [fragmented arc-only lens reconciliation](evidence/plaincad-reconciled-9899716f-development-arc-only-lens-holes-step-2026-11-14.json)
adds the worktree's arc-only lens through `7686c8c8` and full provider evidence
through `c7acf144` to review `9899716f`. Native checks and all 11 original
browser cases passed; 841 app and 2,093 kernel source blobs were verified.
The [authored four-arc lens](evidence/plaincad-reconciled-9899716f-supplemental-arc-only-lens-step-2026-11-14.json)
produced the same STEP and a separately verified 348-triangle watertight STL.
Live OCCT reproduced the volume, area, and centroid of the exported solid.
The [cylindrical sector and full tube reconciliation](evidence/plaincad-reconciled-ce54d341-development-cylinder-tube-holes-step-2026-11-14.json)
adds worktree revisions through `316dc8ec` to review `ce54d341`. Focused
native checks and all ten original browser cases passed, with 841 app and
2,104 kernel source blobs verified. The [authored sector](evidence/plaincad-reconciled-ce54d341-supplemental-cylinder-sector-step-2026-11-14.json)
and [authored full tube](evidence/plaincad-reconciled-ce54d341-supplemental-full-tube-step-2026-11-14.json)
matched the worktree's STEP and watertight STLs byte for byte. Each exported
solid's volume, area, and centroid were independently confirmed by OCCT.
The [all-plane rim fillet reconciliation](evidence/plaincad-reconciled-830e4932-development-rim-fillet-holes-step-2026-11-14.json)
adds worktree revisions through `bc85d42c` to review `830e4932`. Native checks
and all seven original browser cases passed, with 841 app and 2,113 kernel
source blobs verified. The [authored XY](evidence/plaincad-reconciled-830e4932-supplemental-rim-fillet-step-2026-11-14.json)
and [authored XZ/YZ](evidence/plaincad-reconciled-830e4932-supplemental-placed-rim-fillet-step-2026-11-14.json)
cases each reproduced the source worktree's STEP and watertight STL.
Independent OCCT confirmed volume, area and the placed centroids.
The [circular rim chamfer reconciliation](evidence/plaincad-reconciled-f4a05329-development-rim-chamfer-holes-step-2026-11-14.json)
adds worktree revision `5dba4625` to review `f4a05329`. Native checks and all
eight original browser tests passed; 841 app and 2,124 kernel source blobs
were verified. All three [authored plane cases](evidence/plaincad-reconciled-f4a05329-supplemental-rim-chamfer-step-2026-11-14.json)
reproduced the worktree's STEP and STL byte for byte. The STL edges closed
in opposite directions and independent OCCT confirmed volume, area and
centroid.
The [socket-head fastener reconciliation](evidence/plaincad-reconciled-4c071ff9-development-socket-fastener-holes-step-2026-11-14.json)
adds worktree revisions through `281f55b0` to review `4c071ff9`. Native
checks and all six original browser cases passed; 841 app and 2,131 kernel
source blobs were verified. The [authored M3 and 4-40 UNC round trips](evidence/plaincad-reconciled-4c071ff9-supplemental-socket-fastener-step-2026-11-14.json)
produced byte-identical local STEP re-exports and watertight 276-triangle
STLs. Serialization differed from the original worktree run. STEP entity
type counts and OCCT volume, area and centroid agreed for both cases; M3
retained the same oriented triangle set, and four inch triangles differed.
The [near-floor skew hole reconciliation](evidence/plaincad-reconciled-578e17c5-development-partial-near-floor-holes-step-2026-11-14.json)
adds worktree revision `1be125f1` to review `578e17c5`. It passed focused
native tests and all 17 original browser tests; 841 app and 2,144 kernel
source blobs were verified. The [authored clearance case](evidence/plaincad-reconciled-578e17c5-supplemental-partial-near-floor-step-2026-11-14.json)
reproduced the source worktree STEP and watertight STL byte for byte. OCCT
transfer and the independent construction agreed on volume, area and
centroid. This fixture uses the actual crossing extent, which leaves a
valid blind floor where the old full-radius bound rejects.
The [tight-floor tessellation reconciliation](evidence/plaincad-reconciled-2b087b72-development-close-floor-mesh-holes-step-2026-11-14.json)
adds worktree revision `fa857e59` to review `2b087b72`. Focused native
race checks and 17 original browser tests passed, with 841 app and 2,150
kernel source blobs verified. The [authored tight-floor case](evidence/plaincad-reconciled-2b087b72-supplemental-close-floor-mesh-step-2026-11-14.json)
completed within the rebuild deadline that the baseline missed and
reproduced the original STEP and watertight 1,106-triangle STL. OCCT
transfer and direct construction also agreed.
The [all-axis oblique hole reconciliation](evidence/plaincad-reconciled-9b93d85d-development-placed-oblique-crossing-holes-step-2026-11-14.json)
adds worktree revision `6ee8a176` to review `9b93d85d`. Native race
checks covered 16 placements, and all 17 original browser cases passed.
The run verified 841 app and 2,155 kernel source blobs. The
[authored oblique crossing](evidence/plaincad-reconciled-9b93d85d-supplemental-placed-oblique-crossing-step-2026-11-14.json)
reproduced the original STEP and watertight 404-triangle STL;
OCCT transfer and direct construction agreed.
The [5–12–13 oblique hole reconciliation](evidence/plaincad-reconciled-27fa07c2-development-pythagorean-oblique-holes-step-2026-11-14.json)
adds worktree revision `aa57c278` to review `27fa07c2`. Native race and
all 17 original browser cases passed; 841 app and 2,163 kernel source
blobs were verified. The two [authored shallow and steep cases](evidence/plaincad-reconciled-27fa07c2-supplemental-pythagorean-oblique-step-2026-11-14.json)
reproduced the worktree's STEP and watertight STL byte for byte, with
independent OCCT transfer and direct geometry agreement.
The [oblique blind-floor reconciliation](evidence/plaincad-reconciled-cc2030ea-development-blind-oblique-holes-step-2026-11-14.json)
adds worktree revision `8a9f5d22` to review `cc2030ea`. Focused STEP
and bridge race checks, including a further cut after import, and all 17
original browser cases passed with 841 app and 2,167 kernel source blobs
verified. Both [authored blind-floor cases](evidence/plaincad-reconciled-cc2030ea-supplemental-blind-oblique-step-2026-11-14.json)
reproduced the worktree's STEP and watertight STL. Independent OCCT
transfer and direct construction agreed on their physical properties.
The [grouped concealed-cutter reconciliation](evidence/plaincad-reconciled-dd49d102-development-concealed-oblique-holes-step-2026-11-14.json)
adds worktree revision `19ca5756` to review `dd49d102`. Focused native
race checks and all 17 original browser cases passed. The run verified
841 app and 2,168 kernel source blobs. The [authored concealed-cutter case](evidence/plaincad-reconciled-dd49d102-supplemental-concealed-oblique-step-2026-11-14.json)
confirmed that a wholly covered cutter leaves the canonical STEP unchanged.
The reconciled STEP and watertight STL matched the worktree's files, and
independent OCCT confirmed the resulting geometry.
The [oblique-axis floor reconciliation](evidence/plaincad-reconciled-ade2dfc1-development-oblique-own-floor-holes-step-2026-11-14.json)
adds worktree revision `b88b4665` to review `ade2dfc1`. It passed 17
original browser cases and native STEP and bridge race checks. The
[authored lower and upper floor cases](evidence/plaincad-reconciled-ade2dfc1-supplemental-oblique-own-floor-step-2026-11-14.json)
reproduced the worktree STEP and watertight STL files. Independent OCCT
transfer matched direct construction for both floors.
The [double-blind oblique reconciliation](evidence/plaincad-reconciled-23613ca2-development-double-blind-oblique-holes-step-2026-11-14.json)
adds worktree revision `988ddd4d` to review `23613ca2`. It passed all
17 original browser cases and the focused native race suite. The
[authored lower and upper cases](evidence/plaincad-reconciled-23613ca2-supplemental-double-blind-oblique-step-2026-11-14.json)
preserve both floors and reproduce the worktree STEP and watertight STL.
Independent OCCT transfer and direct construction agree in both cases.
The [tighter partial-hole reconciliation](evidence/plaincad-reconciled-6a3d7c88-development-tight-partial-holes-step-2026-11-14.json)
adds worktree revision `23f6b13f` to review `6a3d7c88`. All 17 original
browser cases and the focused native race suite passed. The run verified 841
app and 2,180 kernel source blobs. The [authored 2 mm offset case](evidence/plaincad-reconciled-6a3d7c88-supplemental-tight-partial-step-2026-11-14.json)
reproduced the 258,205-byte STEP and watertight 1,294-triangle STL, including
eight bounded spline edge records. Independent OCCT transfer matched direct
construction.
The [tighter covered-width reconciliation](evidence/plaincad-reconciled-8417c286-development-tight-covered-holes-step-2026-11-14.json)
adds worktree revision `e2e9a253` to review `8417c286`. All 17 original
browser cases and the focused native race suite passed; 841 app and 2,186
kernel source blobs were verified. The [authored 0.75 mm offset case](evidence/plaincad-reconciled-8417c286-supplemental-tight-covered-step-2026-11-14.json)
reproduced the 378,805-byte STEP and watertight 1,672-triangle STL. Independent
OCCT transfer matched direct construction.
The refreshed review `f87dd4f3`, which includes the same external revision
`e2e9a253` and the subsequent native kernel milestones, passed all [66 original
production cases](evidence/plaincad-reconciled-f87dd4f3-production-2026-11-14.json)
and all [15 cross-browser cases](evidence/plaincad-reconciled-f87dd4f3-cross-browser-2026-11-14.json).
Both runs verified 841 pinned app files and 2,200 kernel source blobs, with
unchanged original acceptance configurations.
The second agent's worktree through `e2e9a253` was integrated into `main` by
merge `8f76f16b`, preserving both histories. The full development run on
review `f87dd4f3` passed 340 of 344 cases. Two original cases require the
default local port and two drawing cases failed during import or setup.
[The targeted controls](evidence/plaincad-f87dd4f3-drawing-targeted-controls-2026-11-14.json)
passed all three unchanged drawing cases on both PlainSolid and the app's
OCCT backend; the original failure runs remain distinct.
The [exact merged-kernel report](evidence/plaincad-kernel-main-8f76f16-2026-11-14.json)
separately records original and controlled production runs, all 15
cross-browser cases and focused native verification. A clean full
development run remains an open qualification gate.
The broader `ac27211d` snapshot, through worktree revision `19ca5756`,
passed all [66 original production cases](evidence/plaincad-reconciled-ac27211d-production-2026-11-14.json)
and all [15 cross-browser cases](evidence/plaincad-reconciled-ac27211d-cross-browser-2026-11-14.json).
Both runs verified 841 pinned app files and 2,171 kernel source blobs,
including unchanged acceptance configurations. The later floor update
has targeted evidence above.
The [earlier complete production reconciliation](evidence/plaincad-reconciled-c0e81c4-production-2026-11-14.json)
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

### Exact triangular residual total degree

Rational Bézier triangle error proofs now size their exact polynomial from the
nonzero homogeneous coefficients, including the denominator times the affine
display target. This avoids needless nominal tensor-degree elevation for sparse
polynomial graphs while retaining every genuine mixed term. Original support
charts, degree admission, positive-weight proof and arithmetic limits are
unchanged. This is an image-distance calculation, not a surface embedding or
volume certificate. Higher nominal degree and complete nested-region display
qualification remain separate work.

### Selected original graph region display collections

`AlgebraicTubeRegions.TessellateSelectedRegionsClosedCertified` accepts up to
eight nonadjacent complete regions from one proved XY graph pair. One certified
boundary network serves disks and annuli, including a disk nested inside a
separate annulus hole. Exact represented display rings must reproduce the
original containment forest; selected regions sharing a rim reject. Polygon,
residual, triangle and collision limits are cumulative. Every component and the
combined display must pass closed manifold and exact nonintersection checks.
The output preserves source region/chart indices without coordinate welding.

`AlgebraicShellBoolean.TessellateTubeRegionComponentsCertified` independently
rebuilds source coverage/containment and binds selected regions to original
native face orientations, edge parameters and indexed vertices. This adapter
currently requires one complete model-XY graph pair with whole section loops.
Its focused native qualification is a single original lens; the nested
collection qualification uses an independently authored exact degree-six roof.
The original nominal degree-eight triple-roof native mesh remains a separate
flagship qualification. Image-distance bounds do not certify volume error or
general source embedding; displayed and analytic volumes remain distinct.

### Explicit degree-eight exact residuals

The positive-weight homogeneous coefficient/Jacobian and multispan triangular
residual paths now accept explicit degree limits through eight per axis. Existing
defaults and requests through six retain their original degree gates. Exact
nonzero-coefficient total degree controls residual storage; genuine denominator
products through total degree seventeen are retained. Existing operation/bit
limits are unchanged. The original native triple-roof mesh qualification is
explicitly enabled with `PLAINSOLID_TRIPLE_RADIAL_MESH=1`; a passing small
residual proof does not itself claim that whole native mesh has qualified.

The explicitly gated original nominal-degree-eight native flagship is now
qualified: complete original source section and material, two native components,
exact source mass properties, and a shared-boundary closed display of the disk
plus annulus. Its 10,066 vertices/20,128 triangles retain all four source faces
and 228 original edge/vertex bindings. Global exact display nonintersection and
per-component topology pass. This is an expensive proof-oriented qualification;
it does not establish interactive latency, general multi-pair mesh assembly,
native recipe persistence of all enlarged rings, or certified volume error.

## Exact radial tube coverage (2026-11-14)

Added an independent `SplineGraphRadialContours.CertifyTubeCoverage` proof for
original identity-XY support charts. Every tube support is exact-compared with
the original support on the identical UV rectangle; both endpoint model roots
must satisfy the selected exact radius equation. Coverage requires all four
original endpoint UV roots strictly inside another same-radius tube. Uniform
uniqueness establishes incidence, and the finite nonempty open/closed circle
cover establishes completeness. No sampled endpoints or overlapping boxes are
accepted as membership evidence.

The proof permits redundant arcs and does not publish branch order or native
topology. Anchored tubes, endpoint-only joins and changed chart correspondence
remain outside this sufficient method. Next integration must independently
certify overlapping free intervals: the existing abutting radial proposals do
not automatically satisfy this stronger coverage criterion.

## Independent overlapping radial coverage strategy (2026-11-14)

`DiscoverOverlappingRadialGraphContourTubes` is an additive, independently proved
proposal lane. `SplineTubeCoverageOptions.RadialOverlap` explicitly selects its
complete exact circle cover, followed by bounded exact local transfer of every
cover cell into the supplied original tube union. Thus independent proposals
cannot conceal an omitted native branch. Nil retains generic full-domain
isolation unchanged. Existing original branch ordering, trim classification,
material assembly and native edge checks are untouched.

A genuine original unit-chart plane/dome shell passes section, Intersection and
native two-face component construction with original support ownership. The
strategy currently requires the radial certificate's identity-XY original
charts; it is not an automatic affine-chart fallback. Overlapping proposals
alone are not ordered topology. No new persistence or flagship qualification is
claimed. Next work may align independent original charts, then requalify larger
nested families through the same transfer proof.

## Exact affine transport for radial coverage (2026-11-14)

`geom.CertifyAffineRadialGraphContours` proves signed-axis affine XY maps,
computes their exact common physical footprint, and restricts original tensor
controls at rational UV values before proving the complete radial identity.
The first explicit footprint scope is the physical unit square; larger and
unequal original projected rectangles are accepted when their intersection is
exactly that square. Shears, other footprints and multiple spans remain outside
this API. Original chart scales, reversals and axis permutations are retained.

`RadialOverlapCoverageOptions.AffineCharts` opts into this transport. Outward
mapped boxes are proposal enclosures only: every uniform tube is re-proved on
original supports; exact original endpoint roots establish radius and strict
membership on both original domains. The independent cover is then transferred
into the original supplied tube union as before. No endpoint identity is inferred
from a rounded mapped box. The default identity-only route remains strict.

A native plane/dome lens with stock map X=5U/4-1/8,Y=5V/4-1/8 and a unit roof
qualifies end-to-end. Its exact restriction uses nonbinary original UV1/10 and
9/10. This is a compact source qualification, not a repeat of the long triple
flagship or a new persistence claim.

## General rational rectangular radial footprint (2026-11-14)

The affine radial certificate now computes any positive exact common physical
rectangle, rather than requiring [0,1]^2. Center and radii remain in physical XY.
The coefficient proof explicitly substitutes X=x0+wx*u and Y=y0+wy*v into every
circle factor, retaining unequal scales and offsets. It never treats a circle
in normalized UV as a physical circle. Every full circle must lie strictly
inside the exact common rectangle; clipped circles and boundary tangencies
remain unsupported. Original signed-axis chart maps and rational restriction
values remain immutable. `PhysicalRectangle` returns owned rational bounds.

The public overlapping affine proposal lane accepts this certificate while
re-proving original tubes and endpoint membership. Proposal centers/radii still
must be losslessly represented; the coefficient proof and rectangle endpoints
can be nonbinary rationals. Existing unit-domain radial mass APIs retain their
strict public contract; the generalized coefficient variant is private to the
affine coverage certificate. No new native solid or mass formula is inferred
from a normalized proof chart.

## Exact physical radial transfer (2026-11-14)

The explicit affine radial strategy now transfers coverage by an exact physical
XY arrangement, through `AffineRadialContours.CertifyCellInTubes`. Every target
is freshly verified as a uniform tube on the original supports. Exact affine
source identities map both closed original UV domains to rational physical
rectangles; their intersection gives precisely the possible contact locations.
A bounded partition includes vertices, open edges and open cells. Uncovered
strata are excluded by exact circle-distance extrema with endpoint
attainability, retaining isolated tangent contacts and tiny arcs. Midpoints
select constant rectangle membership only; they never sample curve incidence.

This resolves a concrete generic 4D quartic exclusion limitation (one unresolved
cell at 205,780 operations, not arithmetic exhaustion) without weakening the
generic proof. `RadialOverlapCoverage.TransferMode` identifies the selected
proof; `TransferStrata` and `TransferTests` report its bounded arrangement work.
The generic original-cell union remains available on the non-affine strategy.
The opt-in complete affine triple-region test passed on the original fixture:
248 overlap cells certify 228 retained edges, two native components, exact
original material moments, and a collision-free 20,128-triangle certified
display. The generic transfer remains separately qualified on this fixture.

## Direct regularized measures for coincident roof boxes (2026-11-14)

`BoundRoofBoxVolume` and `BoundRoofBoxMass` evaluate the original roof-box pair
directly, without assembling a result shell. Common, Fuse and both directions
of Cut accept supported shared floor, wall and roof supports. Mass includes
world first moments and inertia. Zero-volume intersections have exact zero
moments and no defined centroid. This adds measure queries; construction and
source topology for coincident Boolean results remain separate.

`PrepareRoofBoxPointQuery` also classifies the same original source pair on
coincident supports. It uses local occupied layers to distinguish an internal
union seam, a face-only intersection, and a genuinely exposed contact. Exact
one-sided polynomial proofs include shared rational roofs and piecewise roof
creases. A harder tangential order that affects the location fails closed.
Its exact vertical partition covers the complete clipped row with exterior,
interior and boundary intervals and retains original source height incidences
even on a face removed by regularization.

## Non-unit native radial annulus qualification (2026-11-14)

A separate original-source fixture uses the roof rectangle [2,4]×[-1,2]
and stock footprint [1.5,4.5]×[-1.5,2.5]. For
q=(X−3)²+(Y−1/2)², the exact height is −3(q−25/1024)(q−25/256), against
stock bottom Z=0. The retained annulus has physical radii 5/32 and 5/16.
The source floor Z=−256 satisfies the existing sufficient wall injectivity
proof and does not alter the retained material.

Native assembly retains 176 ordered original tube edges, the original UV
owners and two original support objects. Each edge has two opposed uses;
there are two inner wires and Euler characteristic zero. Removing an inner
wire fails atomically. Separate tests cover exact transfer work limits.

`AlgebraicShellBoolean.PhysicalRadialTubeRegionMassMoments` selects the occupied
physical bands from the original immutable Boolean, then re-proves source
support identity and branch orientation. The non-unit annulus selects band 1
with exact volume/π 421875/2147483648, area/π 75/1024 and centroid
(3, 1/2, 3375/2097152). The point and moment methods retain independent limits.

`IntersectOrderedRoofBoxes` now builds the native Common when roof order is
proved on the entire shared footprint and the selected roof is a single
positive-weight rational graph strictly above the retained floor. Exact
redundant knots can be removed after recovery of a single global function.
Exact homogeneous restriction and ruled walls keep the original geometry
and a face ownership map, including equal roofs. A bounded Bernstein sign
proof handles selected non-switching tangencies. Changing order and pinched
material remain outside this constructor.

`PrepareNestedMixedDisplay` verifies a single native rectangular spline
material with nested cavity and island shells, retaining the original
face, edge, and vertex identities after sewing and reload. Proper rational
placement, including composed placement, must preserve a proved clearance
between the original boundary shells after accounting for display error.
The current clearance test is sufficient and may reject close curved shells.
Canonical wrapped frusta now also have original-source display preparation
for eccentric cylindrical through and blind bores. The mesh retains annular
cap rims, periodic seams, original curve parameters and coedge directions.
Blind bores retain the inward floor and distinguish both entry ends.
Unbored circular frusta and cylinders use the same proved source binding
and retain both original cap orientations, rim coedges and seam coedges.
Circular station lofts retain exact shared indexed rows through original
wall creases, caps, and rational display placement.
Hollow station lofts retain independently partitioned outer and inward
walls, with their original shared rims and annular cap ownership.

`FuseOrderedRoofBoxes` builds the exact native Fuse for matching XY
footprints when rational roofs have a proved height order and the vertical
bands meet across the whole rectangle. It keeps the higher original roof,
lower original floor, and the two original contributors to each merged
wall. The opt-in dispatcher falls back for unsupported geometry. Height
order changes and gaps covering only part of the footprint remain outside
this constructor. A strict gap across the whole footprint retains both
original solids with no welding. Strictly separate projected footprints and
fully contained footprint/height combinations preserve their original solid
supports without reconstruction.
Stacked roof boxes that share part of a flat horizontal cap now use the
proved original interface join, preserving the exposed cap fragments.
An exact full-footprint Cut with a cutter wholly between the stock floor
and roof returns two closed solids. Each retains one of the two cutter
caps, reversed, and all remaining stock wall fragments keep their sources.
The fully enclosed weighted cutter instead retains all original surfaces
and reverses the cutter shell to form the native cavity.

`CertifyQuarticGramGraphPointContact` also certifies a single isolated
order-four contact of exact graphs when the complete homogeneous height
difference has a positive definite rational Gram form. Odd mixed terms
are accepted; the proposed witness and Gram entry are verified on the
original charts. This proof does not yet assemble a Boolean from the contact.
`CertifyProjectedQuarticGramGraphPointContact` qualifies independently
signed, swapped and scaled rectangular charts with equal physical
footprints, retaining both original UV witnesses and world tangents.
The intersect package now exposes the complete isolated point with both
original patch owners; it does not require or emit a transverse tube.
`CertifyGraphSemicubicalCusp` proves a complete cusp section in a common
injective chart, with two regular germs and exact rational boundary exits.
`CertifySplinePairCuspSection` exposes those germs and their terminal
ownership on the original finite surface pair, with exact rational samples.
`CertifyProjectedGraphSemicubicalCusp` also proves independently scaled
and signed charts and preserves each original terminal parameter.

Exact hierarchy queries now find the nearest original retained boundary
across the planar, sphere, cylinder and torus occurrence families. All
exactly tied occurrences and their complete nearest sets remain distinct,
including internal boundaries of overlapping occurrences. The document
facade supports the verified neutral, vendor and affine-pcurve conventions.
The separate `IndependentFrustumHierarchy` exchanges exact wrapped
circular frustum solids with existing planar, spherical, cylindrical and
toroidal products under proper signed-coordinate occurrence placement.
Its native envelope re-proves the original STEP graph, local topology,
original support IDs and exact placement paths before returning geometry.
It also returns tight rational world bounds for every original use and
their aggregate, including noncircular wrapped frustum profiles.
Exact circular frusta also support symbolic volume, centroid and full
inertia for each original occurrence and their occurrence sum.
The exact circular point classifier retains original caps, walls, rim and
seam edges, and vertex incidences for each placed occurrence.

### Coplanar noncontacting polygon ring selection

`intersect.SelectNoncontactingAffinePolygonRings` provides complete planar
intersection, union and directed difference boundary cycles for validated
polygon/hole trims whose cross-owner boundaries are globally disjoint. Every
original edge pair is checked exactly in a common original affine chart before
whole-ring membership is used. Retained cycles preserve authored owner/ring,
vertex order, rational UV/model coordinates and explicit traversal direction
relative to owner A. Nested holes, islands and disconnected regions are covered;
crossing, touching and coincident boundaries remain unresolved rather than
receiving an inferred arrangement. Exact trim validation is shared with
transverse line contacts.

### Proper-crossing coplanar polygon arrangements

`intersect.ArrangeTransverseAffinePolygons` constructs complete planar Boolean
cycles for coplanar polygon and hole trims with proper cross-owner intersections.
It verifies model coordinates on both original charts, then partitions every
edge at exact rational contact parameters. It retains each selected piece's
original owner, ring, edge, UV endpoints and directed parameter interval.
Closed-cycle assembly verifies exactly one incoming and outgoing edge at every
node. Noncontacting rings also qualify. Vertex touches, endpoint-on-edge events
and coincident spans remain outside this entry point.

### Certified weighted station boundary area

Filled rational station lofts and nested through-bores return outward exact
rational area intervals for every original face and the total. The lateral
method integrates the height direction analytically. A bounded Bernstein
integral encloses the remaining weighted profile derivative on a shared exact
subdivision. End caps use the authenticated profile area. Annular caps apply
the exact difference of squared scales before multiplying the profile interval.

### Shared-edge and endpoint polygon arrangements

`intersect.ArrangeGeneralAffinePolygons` includes exact collinear overlaps,
shared endpoints and endpoint-on-edge events. It partitions every source
side and evaluates symbolic infinitesimal winding on both sides. Retained
coincident area edges carry every original use and its exact parameter
interval. Closed common boundary spans report intersection material on
both sides. Isolated common points are returned separately. Point-connected
branching area boundaries remain unresolved.

### Weighted station loft STEP exchange

The strict filled-loft writer retains original weighted wall supports, exact
rim and seam p-curves, and a single topological edge at every shared station
rim. Its paired reader re-authenticates each original wall and both terminal
caps before returning native geometry. See the [weighted loft STEP contract](../exchange/step/WRAPPED_RATIONAL_STATION_LOFTS.md).

### Certified weighted station inertia

Filled lofts and strictly nested through-bores return a unit-density
centroidal inertia tensor in the original model axes. Exact rational
profile polygons enclose the area and first and second moments. The
through-bore subtracts correlated axial factors before propagating the
intervals. Source wall and terminal face IDs accompany the bounds.

### Native straight extrusions of exact polygon arrangements

`trimmed.ExtrudeAffineArrangement` builds native prism components from
regularized planar cycles with a common straight extrusion offset. Exact
containment groups holes by component. A native wall retains all original
arrangement pieces through collinear simplification. The extrusion checks
that final base and top coordinates are exactly representable and verifies
its native topology and exact volume.

### Weighted station through-bore STEP exchange

The strict weighted bore reader and writer retain both original wall
families, including independent station grids, both annular caps and
original rim and vertex identities. Native reload re-authenticates
strict containment and inward tool orientation. See the
[weighted bore STEP contract](../exchange/step/WRAPPED_RATIONAL_STATION_BORES.md).

### Weighted station measures under exact placement

A filled loft occurrence preserves the exact per-face and total boundary
area intervals. Its centroidal inertia uses exact rational rotation in
world axes. The required original integration width follows from the
coefficient sums of the symmetric tensor transform.

### Exact polygonal prism Booleans from native operands

`trimmed.CombineMatchingNativePrisms` authenticates original cap and wall
topology for two straight coordinate-axis extrusions with an equal axial
interval. It recovers outer and hole loops, then solves and materializes
the arrangement. Result walls retain original operand walls and exact
intervals on both original cap edges. The general Boolean dispatcher
retains its existing face history contract.

### Independently authored planar NURBS assemblies

Strict AP242 readers authenticate finite original planar NURBS supports,
B-spline model edges, owned p-curves and nested product use links. Separate
entry points cover unit, scaled and supported oblique affine charts. The
oblique route proves a rational rigid basis per product, restores exact
original model coordinates, and preserves each occurrence placement.
See the [oblique STEP contract](../exchange/step/OBLIQUE_NURBS_PLANAR_ASSEMBLY.md).

### Exact trimmed topology sewing

`trimmed.SewExactTopology` reassembles independently indexed faces whose
original finite model edges admit exact rational descriptions. It checks
original UV loops and coedge agreement, compares entire edge intervals and
welds only vertices linked by a proved shared edge. It verifies vertex fans,
retains input incidence maps and can repair relative face orientation.
Global embedding, cavity classification and nonconforming edge splitting
remain separate from this sewing operation.

### Native weighted lofts with independent station scales

Original weighted rational bands can vary X and Y scale independently
while maintaining a fixed center and increasing height. Native construction
reproves both original scales and complete shared station rims. Exact
sections retain the independently varying scale, curve and original wall
and crease owners after native reload and reindexing. See the
[anisotropic station contract](../brep/trimmed/WRAPPED_ANISOTROPIC_STATIONS.md).

### Oblique native prism Booleans

`trimmed.CombineAffineNativePrisms` derives bounded extrusion candidates
from the original face and edge graph. It proves an exact affine map and
inverse, verifies every paired cap and side in the canonical chart, and
maps the native result back to exactly represented model coordinates.
Original cap IDs, trim-edge parameters and source wall uses survive the
map. Exact volume includes the affine determinant.

### Convex Euclidean inset with irrational supports

`draft.InsetStableEuclidean` models ball erosion of convex planar solids
with stable simple incidence. A support may have an irrational normal
length in the common quadratic field. Every displaced plane triple is
classified exactly, and native conversion returns a proved displacement
bound alongside algebraic vertices, volume and original face ancestry.

### Certified anisotropic station mass

The exact independent station scale factors transfer one original weighted
profile enclosure to volume and first moments. Axial centroid is exact.
Horizontal centroid intervals retain the uncertainty from the original
profile integration and carry the original source topology.

### Mixed-owner prism cap ancestry

`CapContributions` partitions native prism caps into exact A-only, B-only
and shared regions with holes. It identifies the original controlling cap
faces, both original ring orders and the exact stored parameters of boundary
edges. An oblique prism retains exact original model coordinates and an
exact squared area scale. The region areas are checked against each output
component's cap area.

### Native prism Common across unequal axial intervals

`trimmed.CommonNativePrismIntervals` independently proves both cardinal
extrusions and intersects their exact height intervals and planar profiles.
Positive material returns native components with original controlling caps
and clipped original wall fractions. Closed contact and empty intersection
are explicitly distinguished. Unequal-interval fuse and cut require separate
shoulder sewing.

### Nested AP242 occurrence appearance

The bounded two-level reader expands repeated shared-group uses and proves
ordinary whole-body contextual style ownership. An internal override is
applied to each independently identified use of that context. Exact paths,
geometry, placements and appearance revalidate on native reload. See the
[nested occurrence contract](../exchange/step/NESTED_OCCURRENCE_COLORS.md).

### Exact anisotropic station point ownership

Rational point classification uses the original independent station scales
and weighted profile. Results retain original cap, rim, vertex, crease and
periodic seam owners. Interior section disks never create an artificial
material boundary.

### Path-specific AP242 appearance

A separate reader authorizes one original two-link SHUO in the bounded
repeated-group structure. It retains both original use identities and
applies the override only to the selected path. Native reload rechecks
geometry, placement, exact source ownership and all sibling appearances.
See the [SHUO exchange contract](../exchange/step/SHUO_OCCURRENCE_COLORS.md).

### Nested-profile blind native prism Cut

`trimmed.CutContainedNativePrismBlind` proves strict profile containment
and a tool entering exactly one stock cap. It sews the opened cap, unused
stock cap, original stock walls, internal floor and reversed clipped tool
walls. The result retains original cap and wall ancestry and exact axial
fractions on the tool walls.

### Nested-profile native prism boss Fuse

`trimmed.FuseContainedNativePrismBoss` authenticates a contained profile
with a tool crossing one stock cap. It retains the exact shoulder, terminal
cap and original stock and tool walls. The new wall history records the
surviving fraction of each original extrusion interval.

### Strictly nested native prism cavities

`trimmed.CutContainedNativePrismCavity` proves positive footprint and axial
clearance of a complete cutter. It retains the stock boundary and reverses
only the original cutter faces. Both shell graphs keep original face, edge
and vertex ancestry. The exact volume subtracts the entire nested prism.

### Contained native prism through Cut

A separately authenticated contained tool can span both stock caps.
`trimmed.CutContainedNativePrismThrough` opens both original caps and
sews reversed original tool walls with exact clipped height intervals.
Original cap, wall and edge ancestry remains available.

### Depth-three AP242 SHUO appearance

The independent OCCT-specific reader validates a unique two-SHUO chain,
three original assembly use links and the original prefix-anchored style
context. It preserves all twelve expanded leaf paths and applies appearance
to exactly one. The distinct native format re-proves the original STEP.
See the [depth-three format contract](../exchange/step/DEEP_SHUO_COLORS.md).

### Crossing native prism Cut transition

`trimmed.ImprintCrossingNativePrismCut` proves retained cap and exposed
floor regions when the tool crosses the stock footprint. It preserves
original cap and clipped wall ancestry. Shared interface pieces pair by
exact directed parameter intervals and account for every stock edge.

### Closed crossing native prism Cut

The transition certificate now builds a complete native Cut. Exact cap
triangles and wall quads share all boundary atoms across the internal
floor and retained profiles. Multiple retained and floor regions qualify
with holes through bounded exact cap triangulation. Original support and
clipping history survive the conforming subdivision.

### Shared curved weighted station STEP products

The weighted station assembly reader retains a single authentic noncircular
NURBS product under repeated proper signed occurrences. The original bands,
rims, caps and occurrence link ancestry remain available after transfer.
See the [weighted station assembly contract](../exchange/step/WRAPPED_RATIONAL_STATION_ASSEMBLY.md).

### Native staggered crossing-prism Fuse

A separate exact transition certifies the exposed underside and shoulder,
then sews one native boundary across the three axial layers. Original
walls, caps and holes retain exact source ownership. The native conversion
requires lossless represented output vertices.

### Unified authenticated cardinal prism operations

`CombineNativePrisms` authenticates the original cardinal prism faces and
routes Common, Fuse, and Cut to the applicable exact construction. The result
includes original operand ordering and the typed construction history. It
distinguishes empty and contact-only Common, preserves a stock body for
zero-volume Cut, and returns both original components for disjoint Fuse.
Unsupported contact joins return an error. Classification, construction, and
publication each have resource limits.

### Strictly nested crossing prism Fuse and Cut

When one axial interval is strictly inside another and the footprints
cross, exact cap and shoulder subdivision closes the three material
layers. Fuse keeps the original outer end caps and builds a Union in the
middle. Cut keeps stock at both ends and builds Difference in the middle.
Both exposed cutter floors and clipped walls retain original support and
edge intervals. The dispatcher reports the selected construction.

### Exact straight border subdivision during sewing

`SewExactStraightSubdivisions` accepts independently indexed faces whose
single-use straight borders have different subdivisions. It identifies
positive-length collinear overlap, records exact original parameter
ranges, and re-proves the original and split coedges before joining the
closed graph. UV and model endpoints must fit binary64 exactly. Point
contacts leave disconnected vertex links distinct.

### Crossing prism through Cut

`CutCoveringCrossingNativePrisms` extrudes the exact Difference footprint
through both original stock caps. It supports separate retained components
and retains original stock and cutter wall intervals, including reversed
cutter hole walls. The unified dispatcher selects the construction for
qualified tool-coverage cases.

### Abutting crossing prism Fuse

Prisms meeting on an exact cap plane can be fused when their crossing cap
footprints have positive-area overlap. The native result removes the
shared cap interior. Its exposed shoulders keep the original cap IDs,
and both original wall bands retain their full source intervals.

### Exact spline border subdivision during sewing

`SewExactSplineSubdivisions` recovers unique rational endpoint preimages
on supported single-span spline borders with affine UV coedges. It
proves equality over the full common interval before subdividing.
The multispan option compares original knot partitions and can split
at an existing knot or across it. A source knot reported by two
incident spans authorizes one preimage only when their exact parameters
are equal. The original model parameter can be nonbinary when the new
native UV and model vertex coordinates are losslessly representable.
The result retains original supports and exact source parameter ranges.

### Planar tolerance repair with fixed supports

`SewPlanarWithinTolerance` repairs explicitly paired linear trim borders
while preserving every original plane. Corner positions are exact
intersections of the incident original planes. Two-plane junctions use
the exact projection of the original samples' mean. The report records
each original vertex displacement, the shared edges, any repaired face
orientation, and exact volume. Publication requires a closed embedded
shell and displacement within the requested bound.

### Equal and nested abutting prism Fuse

The general abutting entry handles equal and strictly nested cap
footprints. It proves complete source-wall coverage across the seam
and accepts an empty shoulder when the exact Difference is empty.
Matched annular openings preserve both sets of original hole walls.

### Tied-cap and side-wall prism Fuse

Crossing profiles with one tied terminal plane preserve both original
cap parents on the exact Common region. The owner-only regions and
interior shoulder retain their own cap IDs. Side-abutting prisms can
also fuse over an exact positive-area wall interface. The wall builder
retains both sets of original caps and records the surviving original
wall fractions.

### Profile-covering axial Cut

A cutter that covers the complete stock footprint can remove an
interior or one-ended axial interval. The retained native slabs
preserve the stock's holes and original wall ranges. Forming caps
retain the reversed original cutter-cap provenance.

### Automatic planar seam repair

`SewPlanarAutomaticallyWithinTolerance` proves a unique partner and
direction for every eligible original border. Discovery uses the
necessary twice-tolerance separation bound. The fixed-support repair
checks the requested actual displacement and final embedding, and the
report retains each inferred pair.

### Cubic and quartic trimmed spline sections

Verified spline-pair tubes can contact cubic and quartic rational trim
coedges in the original face charts. The proof retains original knot
parameters, crossing signs, multiplicity, clipped section runs, and
both fragment uses. Exact Bézier section files use a versioned native
recipe and re-prove the original trims on reload.

### Algebraic coordinate images through degree eight

The exact composed-curve coordinate query handles nonlinear rational
images of isolated roots through degree eight. Its quotient-algebra
characteristic polynomial uses exact rational matrix operations and
the output is independently isolated and compared in the original
coordinate system.

### Exact model queries, source-bound curved imports, and conforming seams

`LocateRationalSplineMaterialPoint` and `SplineRayRational` preserve
the caller's exact model coordinates and direction scale. The regular
spline contact isolator and the original-edge contact proof use the
same exact line. Point membership can recover nonbinary original UV
from an affine-placed nonlinear graph. The cavity case classifies
inside, outside, and exact boundary points with inward and outward
source faces. Tangencies and unresolved trim-boundary crossings still
need a complete proof before contributing to a material winding.

The nonplanar native STEP routes retain original 3D supports and actual
owner pcurves under complete model and UV agreement. The independent
parabolic roof passes with both linear and multispan quadratic original
pcurves. Source-bound native reload re-proves the imported records.
The bicubic fixture varies along both graph axes. A further source route
proves that the original rational model and both owner curves use the
same strictly increasing endpoint-fixed map. The native material uses
the certified normalized speed, and the original coefficients and
map remain in source provenance. The proof is bounded to a six-face
material family.

Closed mixed meshes accept original weighted curved seams with separate
source domains and independently inserted knots. The complete exact
identity authorizes one shared sample network. The checked cases cover
all active source spans, reject mutations in later cells, and retain
the original knots through native reload. The sample ledger records both
normalized and authored source parameters on weighted original curves.
The curved seam split retains independently knotted source curves with
their original reversal and restricted raw ranges. Exact complementary
range merges restore the authored knot, pole and weight records. Native
version 9 accepts a proved positive Möbius speed map on one original
cubic or quartic spline owner. The closed mesher evaluates the nonlinear
source stations on both owners and proves their common positions. Native
version 10 preserves two actual original cubic or quartic knot spans with
independent positive speed maps and forces the original internal station
into both owners' shared mesh ledger. The full raw basis is re-proved on
reload. More general speed laws remain open.
Exact split and merge now preserve the v10 source on either original
span and at the original knot, with forward and reversed raw traversal.
Native v11 supports three to eight independently mapped original spans,
exact splitting and merging, and different original knot partitions on
both owners. A separate boundary recipe validates composed curves whose
geometry changes at a model knot and retains both raw histories on their
common certified polyline. The same proof supplies mandatory seam cuts
to a certified single-face fill. Closed assembly for these piecewise
model edges is in progress.

The opt-in polygon Cut with separated vertex links retains one connected
boundary with two circular vertex links at one isolated original point.
All resulting face pairs are checked for additional contacts. The
reported contact retains both native vertex IDs and the original source
event. Native serialization preserves the two IDs. The original source
pair is required to re-prove the global contact report after reload.
This topology has an isolated self-contact, so embedded-manifold
operations require their own embedding proof.

### Convex planar offsets and wall thickness

`brep.Solid.OffsetConvex` certifies the complete convex source boundary,
translates its support planes by signed normal distance, and constructs
the result from their exact halfspace intersections. It returns the
original face map and removes facets with only lower-dimensional
contact. Boundary collapse regularizes to empty material.
`ThickenConvexOpenFace` retains the selected support as the lip, keeps
the other outer faces, and sews the inward offset walls around one
opening. `ThickenConvex` also qualifies separate openings or a closed
inward cavity. `ChamferConvexEdge` constructs an exact planar transition
from possibly unequal setbacks on its two original incident supports.
The translated supports require rational normal lengths and exactly
representable vertices. `FilletConvexPrismEdge` constructs a cylindrical
transition on one original vertical edge of a convex right prism. It
proves the exact represented tangencies and uses the bounded analytic
arc-loop endpoint contract. Adjacent original vertical edges can retain
independent fillet radii on a shared wall. The ordinary PlainCAD bridge
resolves grouped axial chamfers through exact support reconstruction and
qualifies native fillet STEP round trips on convex line/circle profiles.
Spherical offsets retain exact support ancestry through cavity fill,
transverse breakthrough, union of expanding components and removal of
collapsed components. General offsets and rolling ball transitions
remain open.

### Original curved loops and STEP cavities

The automatic graph Boolean uses bounded rational direction recovery
for circular and rotated elliptical contacts and re-proves the full
original conic. The rotated fixture has separate knot directions on
the two original supports. A separate two-slab family proves both
closed section loops and excludes contacts at the separator. Common
and Fuse retain two trim holes and the forward Cut returns two closed
components. The reverse Cut retains one shell with four inner wires.
When the legacy triangulator rejects a near-collinear C0 wall, the
bounded exact UV fallback proves its full directed boundary and area.
Successful legacy triangulations keep their original ordering.
An opt-in original-knot search qualifies two and three closed loops on
either chart axis. Native reload preserves all selected boundary owners.

Open parabolic contours work on both original chart axes. The piecewise
mode proves each original knot slab separately and retains the exact
common points and one-sided tangents. An independent fixture with a
changed quadratic at a C0 knot and a separate refinement knot produces
three original contour edges. Common, Fuse and both Cuts retain the source
surfaces and pass native revalidation and OCCT volume checks.
The original polynomial mode also qualifies genuine cubic and quartic
contours in both chart directions. Original dependent knots can cross
at irrational curve parameters. Exact evaluation supplies the shared
representable wall vertices and the ordinary continuous edge proof
still rejects a coherently perturbed one-ULP endpoint.
The piecewise polynomial route certifies changed cubic and quartic
branches across original knots and carries separate one-sided tangents
through all native Boolean orders. The ordered two-contour mode certifies
complete cubic or quartic factors and three material strips on the
original supports. The forward Cut yields two components and the reverse
Cut yields one. Crossings between curved branches remain a separate
contact problem.

The nonplanar STEP cavity route authenticates the original outer and
reversed inner shell from `BREP_WITH_VOIDS`. Each shell retains its
complete owner graph. Exact original separation and containment are
re-proved after source-bound native reload and writer generation.
The same proof qualifies up to four original inner shells. One strict
reader dispatches these cavity families and the four qualified exterior
spline shell forms according to their actual source graphs.

The explicit OCCT prism-fillet repair authenticates independent original
model-edge ownership, bounds each required tangent displacement, and
revalidates the corrected full STEP graph. The signed-coordinate pose
variant also proves each correction in the original world coordinates.
Both return the original and corrected bytes and copied repair evidence.
Ordinary exact readers continue to reject the inconsistent original data.
The grouped model-only repair proves separate tangent ownership on two
cylinders. When the source has opposite circle axes, it proves both
incident coedge reversals along with the corrected directed minor arc.
The independent default single-fillet STEP with pcurves enabled also
qualifies. Its source-bound record re-proves all 24 original owner lifts
under the caller's tolerance. The grouped default repair also authenticates
both rational cap curves, isolates finite trim endpoints, and proves their
radial correspondence to the original angular model edges. Native reload
checks the full original STEP evidence for the unposed, translated, and
signed cardinal cases. The corrected output uses the separately validated
model-only representation. Explicit repair at the bridge import entry
point is in progress.

### Uniform periodic spline geometry

Periodic curve windows retain the original rational physical lattice and
owner spans even when a knot or cyclic phase cannot be represented as a
binary64 parameter. Exact plane intersection reports isolated crossings,
tangencies and coincident pieces with separate one-sided incidences at
knots. Certified point-distance bounds retain an exact original witness
parameter. Certified tessellation keeps original knots and seams, with
position and parameter-linear chord error bounds on every output interval.
Uniform periodic rational surfaces now store one or two cyclic axes with
their original pole net and represented weights. Their native geometry
record round-trips the original parameter ranges and optional ordinary
knot axis. Exact tensor cells retain ownership and clipped rational ranges
on both axes and adapt to the normalized exact surface engine.
Exact jet, point-distance, and line queries preserve the physical knot
owners and report unresolved line contacts. The certified fundamental
patch also supports plane contours through C1 seams and an opt-in
one-sided continuation proof at C0 creases.
Uniform knot refinement now bisects one or both original cyclic axes
without changing the curve or surface. The exact homogeneous refinement
keeps the physical period and leaves ordinary knot axes in place. It
returns a numerical spline only when every resulting pole and weight
fits the original binary64 contract.
Direct rational curve evaluation now gives an exact point and first
derivative in the original physical parameter units, with the exact
cycle, knot interval and one-sided seam ownership.

# Verification record

## Full Go and PlainCAD rerun (2026-11-15)

The [pinned Go report](evidence/main-af569683-qualification-2026-11-15.json)
reconciles all 5,714 independently listed cases on `af569683` with zero
failures: 5,199 passes and 515 explicit optional skips. The eight timed
trimmed groups account for 1,417 cases, with 1,371 passes and 46 skips.
Four of the 56 packages have no tests. The logs, per-group input lists,
inventory, scheduler record and exit codes are preserved in the workspace
qualification archive. A separate `go vet ./...` and Windows amd64
cross-build succeeded on the same pinned source.

The [pinned PlainCAD report](evidence/plaincad-main-af569683-2026-11-15.json)
reconciles all eight gates on that kernel and PlainCAD `efae4d2`. There
were 345 passing development, 66 production, 15 cross-browser, 1,691
ordinary unit and four WASM contract cases, with no unexpected results,
skips or flaky cases. Both documented test fixture corrections were
applied to the isolated app copy. The Playwright capture driver matches
its pristine hash and the production WASM matches the build byte for byte.

Separate exact STEP repair checks ran the five signed and three tilted
source fixtures through the Go bridge under the native race detector
and in a pinned `GOOS=js GOARCH=wasm` build with a single Go scheduler
thread. The concrete TypeScript adapter passed three actual WASM tests.
A real Chromium module worker returned the same typed certificate as the
raw Go dispatcher and a second worker strictly reopened and re-exported it.
The focused logs are in `.worktrees/qualification/2026-11-15`.

Focused qualification after the paired full run:

- Native three-crossing Common and Fuse passed the main race in 67.018
  seconds. Half-gap pinned OCCT returned one valid solid in all eight
  chart/order/operation combinations and volume matched the analytical
  result within `9.64e-11`. The separate full-gap exact native topology
  check preserves the source sectors. Full-gap raw OCCT differs by
  `5.33e-7` in one chart and order; its face counts are not used as
  equivalent topology evidence.
- Three-crossing Cut preserves two exterior components sharing all
  three nodes, or four positive components linked by three exact points.
  The full new and previous two-crossing Cut race passed in 356.942
  seconds. Independent rational slab integration resolves volumes as low
  as `1.28875293239510e-10`. Pinned OCCT merged the two exterior components
  and missed the two smallest lenses, so its Cut results are diagnostics.
- Mixed transverse and tangent geometry preserves five signed regions
  for either contact order and both chart axes. Both original source
  functions and all incident pieces are retained, with a separate
  exact-coordinate gate. The focused main race passed in 11.547 seconds.
- Two synchronized mapped support knots at `1/8` and `3/16` retain original
  raw station pairs `(41,51)` and `(65,43)`. Native v4 re-proves 46 exact
  coedges; output STEP reopens with one seven-face solid and independently
  measured OCCT volume `0.958986651152372` and area `6.22813463037912`.
  Main mapped native and exchange races passed in 12.429 and 9.720 seconds.
- Exact planar STEP point-contact exchange passed its live pinned race in
  2.315 seconds. Three independently authored box bodies retain one exact
  corner contact. Rounded vertex coordinates, near model lines or planes,
  or reversed, missing or detached edge pcurves reject before publication.
- Independently authored metre and inch tilted STEP imports preserve exact
  physical conversion and world geometry under pinned OCCT readback.
  The bridge passes an explicit source-unit assertion, rejects missing or
  mismatched units, and keeps physical quantities tagged in millimetres.
  The focused main unit race passed in 46.490 seconds; the earlier complete
  repair bridge cases passed in 177.235 seconds. Five typed adapter fixtures
  passed with actual Go WASM in Node.
- Mixed-contact native Common and Fuse passed all 16 cases and source-reload
  tamper checks in 96.917 seconds under race on main. Mixed Cut passed
  both original contact orders, original axes and operand orders under
  race in 231.431 seconds. Its smallest positive material piece has
  independently integrated exact volume `335/457396837154816`.
  Pinned OCCT's empty and wrong-volume outputs remain diagnostic only.
- Asynchronous mapped repair retains one changing original support at
  each of `1/8` and `3/8`, with raw stations `(17,27)` and `(65,21)`.
  Both native and explicit STEP races passed, and the foreign unsplit
  original with both separately authenticated owners passed its main live
  race in 20.964 seconds. Pinned OCCT reads one seven-face solid with
  volume `0.901441276073456` and area `5.93259130227283`.
- The planar IGES reader and writer passed the live concave prism,
  one- and two-void solid, and separate point-contact group races. Pinned
  OCCT independently reopened the original and native IGES with identical
  material measures and respective topology. The through-hole solid has
  volume 50 and area 102; the enclosed-void models have volumes 448 and
  496. The group retains three separate solids and their single corner
  contact, with volume 3 and area 18. The focused planar IGES fuzz run
  passed its bounded 20-second invocation.
- The single-repair Chromium gate passed three exact unit cases on main.
  The isolated pinned app run then passed all seven Node WASM cases and
  all five Chromium cases, including independently authored grouped
  metre/inch. A fresh worker rederived the exact receipt and re-exported
  identical original-unit STEP. The grouped main Go bridge race passed
  in 155.745 seconds. The matched Node and Chromium WASM hash is
  `a89aa08a9e3bb7b6de1d3a839d5287e67c99b12ceda38e19a2bc68baafd31d2e`.
  Durable grouped logs are under `.worktrees/grouped-step-repair-unit-evidence`.

The [refreshed full ordinary Go run](evidence/main-bafe7e81-qualification-2026-11-15.json)
and [PlainCAD run](evidence/plaincad-main-bafe7e81-2026-11-15.json)
use immutable `bafe7e81`, with PlainCAD pinned at `efae4d2`. Go accounts
for all 5,801 top-level cases with no missing or duplicate results: 5,272
passes and 529 optional skips across 52 tested packages, with four packages
without tests. Eight trimmed groups account for 1,391 passes and 47 skips.
The package and trimmed inventories were independently listed and compared
to their respective lanes. PlainCAD records 345 development, 66 production,
15 cross-browser, 1,691 ordinary unit and four WASM contract passes. All
eight app stages returned zero; the production WASM and freshly built asset
are byte-identical, the driver is pristine, and the bundle gate passed.
The separately archived `d5906fde` full race result is recorded below.
Original reports, logs and case inventories are under
`.worktrees/qualification/2026-11-15`.

A separate [newer ordinary Go run](evidence/main-509cdbd3-qualification-2026-11-15.json)
on immutable `509cdbd3` passed all nine lanes. The repository verifier
independently accounted for 5,921 top-level cases: 5,374 passes and 547
optional skips, including 1,411 trimmed passes and 47 explicit trimmed
skips. There are 52 tested packages and four without tests. Raw results,
launcher and package/case inventories are retained in
`.worktrees/qualification/2026-11-15/go-509cdbd3`. This is a full ordinary
Go pass on the recorded snapshot.

The later full ordinary run on `5181d219` exposed a shaded STEP regression
in five signed-frame fixtures. Generic compact tessellation had taken
priority over the source-aware sampler and missed the exact axis
extremum. The focused ordinary run passed all five after restoring that
sampler, including exact cylinder normals at the extrema, closed mesh
volume and stable canonical STEP. This is a focused correction; the
failed snapshot is preserved in `.worktrees/qualification/2026-11-15/go-5181d219`.
The signed-frame native race passed in 293.258 seconds; the targeted
ordinary run with explicit source normal assertions took 70.077 seconds
and the actual Node/WASM test on the pinned corrected source took
175.783 seconds. A combined caller used its ten-minute timeout after the
rational bridge suite had already passed; the signed-frame suite runs
separately. The renewed full Go and PlainCAD snapshots use `ab33a412`,
where vet and the Windows amd64 cross-build also passed.

The [newer PlainCAD run](evidence/plaincad-main-05ce6e82-2026-11-15.json)
uses immutable `05ce6e82` and PlainCAD `efae4d2`. All eight app stages
returned zero. The browser reports record 345 development, 66 production,
and 15 cross-browser passes with no failures, skips or flaky results. The
unit reports record 1,691 ordinary passes and four real WASM contracts.
The staged driver matches its pristine SHA-256, both documented test-only
patches are recorded, the JavaScript budget passed, and the production
WASM is identical to the asset built in the first stage. Its independent
reports and archived logs are under `.worktrees/qualification/2026-11-15`.

The [refreshed native macOS arm64 race](evidence/native-main-d5906fde-race-2026-11-15.json)
on immutable `d5906fde` completed all nine lanes. Independent package and
case inventories account for all 5,769 cases: 5,247 passes and 522 optional
skips. The trimmed lanes have 1,381 passes and 47 skips. The ordinary lane
has 3,866 passes and 475 skips. The optional checks were not counted as
passes. The source, launcher and original result logs are under
`.worktrees/qualification/2026-11-15/go-d5906fde-race`. This replaces the
earlier running status for this same immutable snapshot.

The [latest full ordinary Go run](evidence/main-a1030049-qualification-2026-11-15.json)
on immutable `a1030049` finished all nine lanes with zero failures. Independent
inventories account for all 6,006 top-level cases: 5,448 passes, 558 explicit
optional skips, 52 tested packages and four without tests. The separately
inventoried trimmed lanes account for 1,433 passes and 48 optional skips.
Raw runs, source, and input lists are under
`.worktrees/qualification/2026-11-15/go-a1030049`.

The [latest full PlainCAD report](evidence/plaincad-main-83e18008-2026-11-15.json)
uses immutable `83e18008` and the same pinned app revision. All eight
stages returned zero: 345 development, 66 production, 15 cross-browser,
1,691 ordinary unit and four real WASM tests passed without failures,
skips or flaky cases. Build, typechecks and bundle budgets passed; the
production browser used the exact just-built WASM and the Playwright driver
kept its pristine hash. Raw results remain in
`.worktrees/qualification/2026-11-15/plaincad-83e18008`.

The [PlainCAD `ab33a412` control record](evidence/plaincad-main-ab33a412-lead-control-2026-11-15.json)
retains the initial failure and the unchanged focused reruns. Build,
typechecks, selected PlainSolid WASM, and all 345 development, 66 production
and 15 cross-browser cases passed. The pinned ordinary Vitest config forces
OCCT, so its `1690/1691` result is an OCCT control failure. The four-start
lead screw and nut case timed out at its original 45-second limit; it also
timed out when first rerun alone. The same unmodified case passed at 39.644
seconds on a second focus run. An instrumented run passed at 36.577 seconds,
with 34.286 seconds in OCCT construction. Separately, the same body with
the real PlainSolid adapter and first optimized WASM passed in 13.793 seconds;
its original TypeScript mesh checks took 1.378 seconds combined. A second
full run with scoped loopback passed 1,690 cases and failed only that same
OCCT control. The [partitioned verification](evidence/plaincad-main-ab33a412-partitioned-2026-11-15.json)
reconciles the same 1,691 file/test identities from the full coverage and
unchanged 39.644-second focused pass. A separate full attempt without
loopback passed the same control in 40.737 seconds but lost 15 gateway
tests because the sandbox denied their listeners. All reports and failure
statuses are retained; case coverage across retries is not an uninterrupted
suite pass. No deadline or geometry tolerance was increased. Raw phase records are in
`.worktrees/qualification/2026-11-15/plaincad-lead-phase-evidence`.

The [full PlainCAD report on `b6335d3c`](evidence/plaincad-main-b6335d3c-2026-11-15.json)
reconciles passing build, type and bundle gates, four actual WASM
contracts, 345 development, 66 production, 15 cross-browser and 1,691
ordinary unit cases from the same immutable archive. An initial browser
attempt could not open local test servers in the default sandbox; all
three reruns with scoped loopback passed. The first full scoped unit run
lost only the unchanged four-start OCCT timing control. A focused run
after the superseded race stopped passed in 25.214 seconds. The next
full scoped run passed every unit case; the original one-, two- and
four-start controls took 5.397, 11.612 and 23.028 seconds. No deadline
or app assertion changed, the Playwright driver is pristine and the
production WASM matches the build. The evidence distinguishes the
original attempts from each passing phase.

The [uninterrupted PlainCAD run on `a296b700`](evidence/plaincad-main-a296b700-2026-11-15.json)
passed all eight gates using the same immutable app and the two documented
test fixture corrections. Scoped loopback was available from the start.
It passed 345 development, 66 production, 15 cross-browser, 4 actual Go
WASM and 1,691 ordinary cases. The unchanged one-, two- and four-start
OCCT controls took 3.218, 10.861 and 16.776 seconds. The bundle WASM again
matched the Go build byte for byte and the Playwright driver was pristine.

The [uninterrupted PlainCAD refresh on `a3b7548d`](evidence/plaincad-main-a3b7548d-2026-11-15.json)
also passed all eight gates with the same pinned app and staged test
corrections. It passed 345 development, 66 production, 15 cross-browser,
4 actual Go WASM and 1,691 ordinary cases. The unchanged one-, two- and
four-start OCCT controls took 3.925, 9.748 and 19.138 seconds. Production
served the identical Go WASM bytes and the Playwright driver hash matched
the pristine install.

The [full ordinary Go run on the same kernel `b6335d3c`](evidence/main-b6335d3c-qualification-2026-11-15.json)
finished all nine lanes: 5,630 passes, 586 optional skips and no
failures across 6,216 top-level cases. Its eight trimmed lanes passed
1,502 tests with 51 optional skips. Vet and Linux/Windows amd64
cross-builds also passed on this snapshot; these do not substitute
for native runtime qualification on those platforms.

The periodic geometry race passed in 8.758 seconds after adding exact
uniform refinement. Independent rational span evaluations agree on
position and first derivative across several cycles for a cubic
polynomial and a curve with unequal rational weights. Tensor refinement
preserves both cyclic axes and an alternative repeated ordinary knot
axis, including a cycle at distance `2^100`. Nonbinary new poles,
unresolvable physical knots and truncated work refuse without output.
Direct exact periodic curve evaluation also agrees with independently
restricted rational Bézier spans in an original cycle of magnitude
`2^100`. A weighted degree-one seam selects its exact right derivative.
Output bounds enclose each jet, and one-short work returns no geometry
or cycle ID. The combined periodic and cubic geometry race passed in
12.219 seconds.
Nonuniform cyclic insertion also preserves the exact position and first
derivative against the original independent rational Bézier restrictions
at all changed knots, span midpoints and far negative cycles. The cubic
case inserts next to both sides of the seam and raises an original knot
to maximum multiplicity. An independently authored weighted linear
curve checks the physical derivative and owner at a nonuniform knot.
Invalid input, cancellation and short pole/work budgets publish no edit.
All 32 periodic curve and surface cases passed under the race detector
in 6.225 seconds.

The [full ordinary Go run on `01d12dab`](evidence/main-01d12dab-qualification-2026-11-15.json)
later passed all nine lanes on macOS ARM64. The independent inventory
contains 6,254 cases: 5,666 passes and 588 explicit optional skips,
including 1,520 passes and 51 optional skips across the eight trimmed
lanes. No failed or unfinished cases remain on this snapshot.

The multispan cubic section race check passed on main in 2.280 seconds.
An ordinary node and a true cusp give the same exact original charts,
vertices, jets and signed regions after independent knots are inserted
in both owners and across both source orders. A physically creased
pair reproduces the exact cusp and the changed original exit heights.
Changing only the later knot cell's residual, placing an ambiguous
crease at the cusp, or constraining the cell budget produces no result.
At a shared longitudinal C0 knot both cusp germs can instead use a proved
one-sided source jet. The exact fixture uses the right cell in the first
roof and the left cell of the reversed second chart. Both first
derivatives remain zero and the shared second derivative is
`(6,0,15/4)`. Opposite departure sides, a C0 terminal ambiguity and
ordinary or isolated nodes at C0 remain refused. Main passed the
combined geometry race and the native and pinned exchange race groups
in 12.219, 35.435 and 27.958 seconds respectively.
An original-stock Common fixture now places that crease at the cusp in
both owner orders. Native replay is byte-identical, with 9 faces, 20
edges, 13 vertices and signed volume `284613/22528`. STEP and IGES add
only two exact stock-edge stations, reopen as 9 faces, 22 edges and 15
vertices, and give the same positive volume. Transfer uses zero pole
rounding tolerance. Pinned OCCT recovers zero first derivative, outgoing
second derivative `(4.5,0,81/128)` and opposite third derivatives
`(0,±4.5,±27/512)`. Main's native and pinned exchange race groups passed
in 5.385 and 7.256 seconds, including the older creased transfer.

A separate isolated real double-point proof passes with and without a
disjoint regular branch, including source order swaps and rotated
projection axes. The original surface tangents agree at the isolated
point, with no sign switch there. The regular arc keeps its exact
turning point, exits and signed regions. A real crossing node, a cusp,
boundary turning point, clipped arc, irrational exit and changed
coefficient refuse. The main cubic geometry race passed in 2.716 seconds.

The native isolated-contact Common and Fuse race passed in 6.670 seconds.
Both operand orders keep one Euler-2 shell with both old source charts
retained in their selected regions. The point remains interior to an
original roof and the optional arc keeps its two original coedges with a
nonzero turning tangent. Independent volume integrals match with and
without the arc; rotations and reflections cover all four stock sides.
Native replay reproduces the receipts and refuses altered contact,
tangents, owner IDs, a missing regular arc and truncated entity budgets.

The isolated-contact Common and Fuse STEP and IGES race passed with all
three live pinned comparators in 7.475 seconds. Both operand orders and
both branch configurations reopen in both formats (16 transfers).
The isolated-only cases have 6 faces, 12 edges and 8 vertices; the
regular-arc cases have 9 faces, 20 edges and 13 vertices. Independent
signed volume matches the source integrals, and the pinned
`acnodal_cubic_interior_contact` probe projects the exact original model
point into exactly one reopened face interior and zero face boundaries.
Altered STEP curve ownership and truncated writer budgets refuse.

The native isolated-contact Cut race on main passed in 20.587 seconds
with adjacent Common and Fuse. Cut keeps one 4-face, 7-edge, 5-vertex
cap on any original stock side. Its exact source volume is
`4703/1614807040`; doubling the original height difference doubles
that result. The exterior order is refused at the isolated pinch, and
native replay re-proves the original coedges and turns. The generic
coarse preview closes but materially understates the volume. The
dedicated volume check now refines the same coarse request to 24
closed triangles, with 0.585522% error against the exact source
integral and no unintended contacts. Replay preserves the mesh bytes.
Truncated work returns no partial mesh. Focused main race: 1.739 seconds.
The Cut STEP and IGES live race passed in 2.209 seconds. Both reopened
solids keep four faces, seven edges and five exact original vertices,
with signed volume matching `4703/1614807040`. The pinned probe proves
that the isolated point lies outside the cap and checks the nonzero
regular turning derivative and both directed original owner curves.

The native shared-crease follow-up retains the two original cusp roof
charts, one closed Euler-2 shell and byte-identical native replay for all
six Common/Fuse/Cut results. The original unsplit mass correction strips
exhausted the budget on these shapes. Splitting at exact represented knot
events now gives independently calculated volumes 12.49531693892045 for
Common and 12.58134321732954 for Fuse in both orders, and the unchanged
1695/22528 and 243/22528 for the two Cuts. The combined trimmed race run
including existing cuspidal, nodal and knot integration cases passed in
38.010 seconds. Geometry and algebraic cell regressions passed under race
in 2.562 seconds. The [separate creased neutral record](evidence/cuspidal-creased-neutral-2026-11-15.json)
contains all twelve independent STEP and IGES readbacks of the original
creased roofs. Each carries positive signed material, retains the exact
support-knot joints and reproduces the original cusp jets after their
parameter range is scaled for the split. Legacy uncreased transfers in
both formats match byte for byte. The combined pinned exchange race
passed in 21.885 seconds. The native extraction rejects cuts with
irrational clocks or unrepresentable model vertices before output.

Both native cuspidal Cut orders passed the main branch race run alongside
the existing Common, Fuse and nodal regressions (25.450 seconds). Exact
source integration gives `1695/22528` for the exterior and `243/22528`
for the pointed Cut. Closed native shells retain two original cusp edges
and exact zero first derivative. The pointed preview at deflection
`0.005` and angle `0.1` closed with 1,536 triangles and 0.509% relative
volume error. The exterior test confirms the generic preview returns an
explicit error and no mesh when closure fails; finer probes exceeded the
current budget. The dedicated source-driven Cut preview now qualifies
both orders at deflection `0.05` and angle `0.15` on the original
polynomial roofs. It returns 318 closed exterior triangles with 0.1392%
volume error and 1,022 pointed triangles with 0.6729% error. A shifted
cusp and reversed roof chart pass a 0.6% volume bound with 1,150
triangles. All three displays retain the exact cusp vertex and pass
collision and replay checks. Sampled deflection and angle are distinct
from the exact mesh-to-source volume gate. The main focused native race
passed in 13.426 seconds.

The [cuspidal Fuse and Cut exchange record](evidence/cuspidal-fuse-cut-neutral-2026-11-15.json)
contains all eight transfer and derivative readbacks against pinned
OCCT. Both formats preserve positive signed material and the original
two zero-first-derivative germs on each shell. Fuse retains 9 faces,
20 edges and 13 vertices. Exterior Cut retains 7/16/11 and pointed
Cut 4/7/5. The main branch combined Common, Fuse and Cut race passed
with the live pinned probes in 7.222 seconds. The writers consume the
analytic native solids directly; their verification takes a typed source
and does not reconstruct an independently authored foreign shell.

The older full native race `491b18f1` reproduced the same five shaded
signed-frame failures as ordinary `5181d219`. Its `packages.jsonl` is
retained in `.worktrees/qualification/2026-11-15/go-491b18f1-race`.
The superseded race was stopped after recording all five signed-frame
failures. Its unfinished lanes do not qualify. The renewed native race
uses `ab33a412`, with eight independently listed trimmed lanes and the
ordinary package lane. The [corrected ordinary report](evidence/main-ab33a412-qualification-2026-11-15.json)
reconciles 6,137 top-level cases, with 5,557 passes, 580 optional skips,
52 tested packages and five without tests. All eight trimmed lanes pass:
1,468 tests and 51 optional cases. Keep the older failed snapshots
separate from the refreshed ordinary, race and PlainCAD runs.

- Exact one-axis stepped Fuse (`810e2014`) passed the main-branch race in
  336.976 seconds. The original planar wall atoms bind low and high roof
  traces and tapered vertical stations. The tested shell has 28 vertices,
  46 edges, 20 faces and one cyclic link at each vertex; independent exact
  integration and source replay agree. Seventy earlier recipe hashes stayed
  unchanged in isolation. Generic tessellation of the 20-face fixture
  exhausted its angle budget at 0.05 deflection and 0.15 radians;
  the source mesher below fixes that case.
- The two-axis stepped native Fuse (`8a0b3e80`) passed the complete
  one- and two-axis regression under the main race in 778.167 seconds.
  The 16 diagonal cases have a concave base with two reentrant corners
  and physical area `93/16`. The native topology retains 35 vertices,
  58 edges and 25 faces, exact tapered risers, and two exact subdivisions
  shared by adjacent walls. Four two-axis containment cases also passed.
  Independent material and source reload passed, and all 18 one-axis
  recipe hashes matched in isolation.
- The clipped Cut component graph (`35bff0d0`) passed its main focused
  race in 13.943 seconds. It proves which open material chambers attach
  through exclusive strips, checks stock-side vertex links and integrates
  exact per-component volumes. It reports two original point contacts
  even when the higher-roof Cut joins their incident chambers through
  the same outside strip. That case is a self-contact inside one positive
  component rather than contact between separate shells.
- Source-aware stepped Fuse display (`068bb176`) passed the main race
  together with exact boundary extraction in 441.230 seconds. At 0.05
  deflection and 0.15 radians, the 16 one-axis meshes have 304–744
  triangles and the 16 diagonal two-axis meshes have 426–458. All are
  closed, retain every native vertex and have no detected non-incidence
  contacts. Four two-axis containment cases and strict replay passed;
  the two-axis extension preserved all 16 earlier mesh hashes exactly.
- Native unequal-footprint Cut (`bce76c45`) keeps the original base and
  cutter-roof floor pieces in strip-attached components. The main Cut,
  recipe refusal and dependent-footprint race cases passed in 143.91,
  38.93 and 32.39 seconds. Independent exact per-component volumes,
  original roofs and walls, local contact sectors and source replay
  qualified 20 configurations in isolation. All 38 prior Fuse recipe
  hashes matched after the shared native builder change.
- Adjacent rational model cells (`8672cf02`) keep the inferred model
  stations `8/7` and `64/5`, and two independently authored occurrences
  of the true stored joint at `8`. The exact-law bridge preserves these
  as separate owners through strict STEP and native replay and returns a
  closed 2,020-triangle display. A foreign source that omits the required
  joint rejects even though pinned OCCT reads it. Main native and live
  STEP races took 565.449 and 654.165 seconds. Native volume agrees with
  the pinned pre-transfer value `0.6352660806774346` to about `5e-13`;
  pinned original and canonical STEP readbacks agree with each other at
  `0.63526701737966`. The readback value is not the exact material oracle.
- The two adjacent model cells with different positive rational roof
  weights (`cb3bcd42`) preserve both independently owned laws and the
  original joint in a 4,932-triangle closed display. The pinned isolate
  live and compatibility race passed in 642.026 seconds. The independent
  pre-transfer material volume and area are `0.64050827338950` and
  `4.65185532305421`; the pinned original and canonical STEP readbacks
  agree at `0.640509222445185` and `4.651865358117015`. Independently
  authored joint and support-weight controls reject.
- The stepped Fuse analytic STEP (`936ef7be`) uses exact native model
  and paired coedge laws, and one original supported wall chart per face.
  Pinned OCCT reopens one valid 20-face solid: volume `16.4362535417463`
  and area `40.443802639426494`. The focused main source and tamper race
  passed in 67.615 seconds. A nonterminating decimal pole requires an
  explicit coordinate bound (the fixture uses `1e-14`); vertices and
  weights retain exact values. Changes to curve ownership or declared
  units fail the strict source verification. Unrelated independent STEP
  graphs are outside this source-backed mode.
- The analytic IGES transfer (`67db7b54`) reuses those native coedges and
  exact spline wall charts. Pinned OCCT opens a valid 20-face solid with
  the same material. The verification rejects changed units and a changed
  use orientation, with strict decimal and input limits. Main pinned IGES
  race took 97.115 seconds. No independent arbitrary IGES import is claimed.
- Three original rational model cells (`c3fb8b82`) retain raw original
  domains `[0,8]`, `[8,16]`, `[16,32]`, inferred stops `8/3`, `48/5`,
  `160/9` and both true original joints. The independently moved second
  support joint is a valid pinned OCCT solid but is refused by exact
  native and STEP paths. The direct 2,958-triangle closed display and
  full pinned isolate race passed (744.966 seconds); native source and
  live STEP ran together on main in 227.259 seconds. Native volume and
  area agree with the pre-transfer `0.64045099678815 / 4.655759693110`.
- Native v12 (`b6335d3c`) retains the two original v11 joint stations
  with change masks `[both, north]`. Missing and false masks, a moved
  source support event, and relabeling the receipt as v11 reject. The
  pinned isolate race including the 1,890-triangle closed mesh took
  613.811 seconds; the main source, live and legacy v8 race passed in
  463.855 seconds. Independent original chart volume and area are
  `0.643137172506171 / 4.65101745648361`; original and canonical OCCT
  STEP readbacks agree at `0.643141460766049 / 4.6510825384331`.
- Analytic Cut STEP (`56fc65ef`) opens in pinned OCCT as three separate
  solids with 17 faces, or four with 16 faces. Independent native total
  volume agrees, and each declared shared point appears as two separate
  exact vertex entities. A changed coordinate rejects even when the STEP
  graph still parses. The combined main pinned Cut and Fuse race passed
  in 111.743 seconds.
- The exact projection collision filter (`d883a33f`) passed the full
  main collision race in 14.171 seconds. The same 4,932-triangle mesh
  and 658,508 charged pairs, including 394,339 narrow pairs, produced a
  complete zero-contact result in 0.194 seconds versus 62.951 seconds.
  Regenerated source display has identical bytes. The full pinned
  isolate STEP recalculation passed under race in 266.65 seconds.
- The [helical mesher performance result](evidence/plaincad-lead-mesher-performance-2026-11-15.json)
  (`61ee3e80`) preserves the exact four-start screw and nut WASM responses
  while reducing direct meshing time from 19.505 to 13.213 seconds. The
  [clean lead fixture](evidence/plaincad-lead-fit-committed-2026-11-15.json)
  separately ran the three original typed app geometry tests through the
  actual Go provider in 13.674, 13.577 and 13.472 seconds. It asserts the
  provider without changing the ordinary OCCT control or its deadline.
- Two-transverse native Cut, Fuse and Common cases and Cut display
  (`ace75343`, `5d6a15c1`, `3eac2245`, `a25a8fc8`) passed the final
  integrated focused run in 83.483 seconds. The transverse strip display
  that differed by 0.613% now differs by 0.229% while using the same
  error and resource budgets; tight budgets retain the verified original
  display. Three-transverse native Fuse (`0c8baacf`) passed the main race
  in 301.390 seconds with 12 exact source cases and explicit negative
  controls. All 50 prior Fuse recipes stayed byte-identical in isolation.
- Separate three-transverse native Cut (`b33ae2a6`) passed with the prior
  Fuse and display regressions in 504.465 seconds under the main race
  detector. Eight cases retain four or five shells and each of three
  original point contacts. The four connected self-contact cases return
  no material. Native Common (`916f4a5c`) retains six original roof
  sectors and all three crossings in 12 cases and passed with two-contact
  Common and three-contact Cut regressions in 63.622 seconds on main.
  The isolated native Common race passed in 57.131 seconds and 82 prior
  native recipe hashes remained identical.
- The three-transverse STEP Cut companion (`1478182f`) passed its pinned
  focused readback and the previous two-contact Cut in 6.341 seconds.
  Replacing the third contact's coordinate entity by an equal entry
  from a different component rejects even though its physical point has
  not moved. The earlier [IGES Cut](evidence/clipped-cut-iges-2026-11-15.json)
  (`c75fa2fd`) keeps three bodies with 17 faces and 25 exact vertex
  occurrences, or four with 16 faces and 22 occurrences. The main pinned
  IGES Cut and Fuse race completed in 155.432 seconds.
- [Three-transverse IGES Cut](evidence/clipped-three-transverse-cut-iges-2026-11-15.json)
  (`a6763a04`) transfers one group with four solids, 19 faces and 29
  exact vertices, or five solids, 18 faces and 26 vertices. Pinned OCCT
  total volumes are `1.915649475022965` and `1.936160890551037`. The
  third original contact keeps both coordinate occurrences. Aliasing
  one across components or omitting its source ledger entry rejects.
  The main live regression passed in 33.953 seconds; full IGES and the
  focused live race passed in isolation in 123.182 and 141.910 seconds.
  Four earlier analytic IGES byte streams remained identical.
- Two-transverse Common transferred through STEP (`c705e026`) and
  [IGES](evidence/clipped-common-iges-2026-11-15.json) (`6e72ff02`).
  Independent pinned OCCT reopens one valid 14-face solid with 32 edges,
  20 vertices, volume `11.970411420799792`, and area
  `32.98325572738854`. Both owner uses of all 32 edges are retained.
  The integrated live IGES group passed in 28.834 seconds; the isolated
  live IGES race and full package passed in 93.793 and 117.115 seconds.
  The STEP Common's tests passed under the race detector in 37.79 seconds
  within a broader run whose distinct kinked-roof live test timed out.
  The raw higher-order mixed Common probes produced the same
  `SelfIntersectingWire` and `UnorientableShape` in pinned OCCT for both
  formats. The initial public writers therefore refused that input;
  the exact seam below replaces the portability refusal.
- An [exact original-roof seam](evidence/clipped-mixed-common-seam-2026-11-15.json)
  (`a5df4358`) divides one lens into two wires, retaining all original
  vertices, geometry and mass. Both independent formats and the public
  STEP writer (`80c3a768`) pass all 16 chart, contact-order, footprint
  and operand combinations. The public IGES writer (`fe3d5380`) also
  passes the pinned mixed fixture with one 15-face solid, 33 edges and
  all 20 original vertices, volume `11.97070312418097`, and area
  `31.65512202993392`. The unsplit wire and truncated proof reject;
  the earlier transverse STEP and IGES output bytes remain identical.
  The main STEP live race passed in 95.137 seconds with three-contact
  Common and the streamlined kink test. Main live IGES passed in
  11.171 seconds with both transverse families.
- The STEP three-transverse Common (`2e5712a9`) keeps each exact vertex,
  four incident source rays, and both pcurves for every model edge.
  Pinned OCCT reads one valid 15-face solid in each original fixture.
  The partial layout has volume `11.97070306403954` and area
  `32.98520166320661`; the dependent layout has volume
  `11.97069946101146` and area `31.65507482163024`. Changing the third
  original contact rejects. The main STEP batch with prior clipped
  Common, Cut and Fuse passed in 28.987 seconds.
- Closed clipped previews (`32b1df05`) passed on main in 34.595 seconds
  for all eight admitted three-contact Cut assemblies and twelve
  three-contact Common cases, with previous Fuse displays and resource
  limits. The Cut component shells retain exact component-local contacts
  through source replay. Default strip-bearing components differ in
  preview volume by under 0.3%, and Common by under 0.023%; default
  absolute deflection can still give about 13.7% relative error on a
  thin overlap-only shell.
- The optional volume-qualified Cut display (`effd488a`) limits every
  component to 2% against its exact rational source volume, including
  components about `7.7e-10` cubic model units. The eight complete
  assemblies use 410–2,316 total triangles; resource shortages return
  no meshes. Thirty default display outputs stayed byte-identical.
  Main focused opt-in/limit tests passed, and the isolate race passed
  in 44.769 seconds.
- Four-transverse native Fuse (`7586a15c`) passed 12 cases in 55.35
  seconds on main. Independent rational integration, source area,
  exact four-way contact binding and original recipe replay agree.
  All 12 closed displays passed in 13.16 seconds (`11e475fb`).
  In the [exchange test](../exchange/clipped_polynomial_four_fuse_test.go)
  (`13f1d870`), pinned STEP and IGES each read one valid solid with
  22 or 23 faces. Their respective volumes are `16.55526781814345`
  and `16.35005710049252`; respective areas are `40.25608465715294`
  and `39.78849337256642`. The main two-format test passed in 4.523
  seconds.
- The independently kinked original rational STEP (`38d53120`) retains
  one-sided seam slopes `(1/2,5/8)` and `(-1/2,1/4)` at a real
  unilateral support change. Original and canonical pinned transfer agree
  with native volume `0.6434452454600076` and area `4.6427728126414856`.
  The moved-support original rejects even though it remains readable in
  OCCT. The pinned main source and live run passed in 37.225 seconds.
  The pinned isolate race and closed display passed in 925.637 seconds.
  A concurrent mixed main race reached its fixed 120-second live-test
  context after both standalone exact source assertions passed. Both raw
  main runs are kept; the contention timeout was not recorded as a pass.
- Ordered v13 source/native/STEP (`7a196ec6`) proves four original
  model cells, four separate interior cuts, and ordered joint masks for
  both owners, north only and south only. Pinned original and canonical
  measures agree with independent integration at volume
  `0.6448671346810776` and area `4.651334753118443`. A moved authored
  joint and forged source ledger reject. Five and seven model cells
  pass the native bounds test; eight reject. The isolated live race
  and the integrated main source cases passed. Exact UV fallback
  (`821b8bcd`) returns a closed 2,822-triangle v13 display with every
  original joint and no unintended pair contacts. Both the earlier
  2,730-triangle kink and the v13 display passed on main. The dedicated
  isolated v13 display race passed in 394.206 seconds.
- [Eccentric frustum rim chamfers](evidence/eccentric-frustum-rim-chamfer-2026-11-15.json)
  (`b600bde2`) retain both axes and prove original wall clearance at
  every runout before assembling the complete tube. Pinned OCCT's
  independent chamfer author returned one valid eight-face solid with
  volume `726.98826561019723` and area `593.89732690669496`. Main
  focused material and wall-contact tests passed, and the isolated
  modeling and trimmed races passed in 83.948 and 7.942 seconds.
- The [eccentric bore inner rim fillets](evidence/eccentric-cylinder-bore-rim-fillet-2026-11-15.json)
  (`d930469f`) retain the original axis placement and both cap planes
  while adding exact quarter-toroidal transitions. Pinned BRepFilletAPI
  matches all three original lower, upper and paired selections in
  topology, material and rim dimensions. Sampled closed displays use
  768, 768 and 1,280 triangles and pass the complete exact collision
  checks. Tangency, insufficient axial span and a changed interior
  meridian reject. Main focused native and the isolated live race passed.
- The [strict eccentric bore fillet STEP import](evidence/eccentric-bore-rim-step-import-2026-11-15.json)
  accepts an untouched, model-only pinned OCCT upper fillet, with 5
  source faces/8 edges/5 vertices. It keeps 23 native edges and 20
  vertices, with every subdivided rim mapped to its source edge. The
  independent signed volume is 879.5727456807722. The exact reader
  rejects original foreign off-support and opposite-owner controls,
  along with changed torus radius, meridian and unit. All three
  PlainSolid STEP writer variants re-export byte-identically. Main
  exchange and STEP pinned race runs passed in 28.040 and 46.356
  seconds. No generic reader dispatch was changed.
- The [strict eccentric bore fillet IGES import](evidence/eccentric-bore-rim-iges-import-2026-11-15.json)
  reconstructs all three untouched model-only OCCT selections. The
  paired input has 6 original faces, 10 edges and 6 vertices; the
  reconstructed shell has 6 faces, 28 edges and 24 vertices, with
  exact full-circle split provenance and positive signed volume
  878.8609059330245 on independent re-export. Original off-support
  data and inconsistent shoulder use, meridian transformation and
  directory controls reject. The pinned main race runs passed in
  45.638 seconds for exchange and 78.688 seconds for IGES.
  Both strict readers then passed 64-execution Go fuzz checks seeded by
  the untouched foreign positives and structural negatives. A failing
  read never returns partial material or source IDs. The same bounded
  checks run in the Go fuzz CI lane.
- The [eccentric annular chamfer neutral report](evidence/eccentric-annular-chamfer-neutral-2026-11-15.json)
  records the four pinned STEP/IGES readbacks: aligned and diagonal,
  both with positive signed volume, 12 faces, 58 edges and 48 exact
  vertices. STEP uses exact straight generators of revolution; IGES
  keeps the original rational supports. The main race passed in
  157.440 seconds after rebuilding the stale author that previously
  supplied aligned circles to the diagonal check. The original failure
  is retained. A rational enclosure independently proves the default
  OCCT decimal cone angles do not have the slope forced by their exact
  shoulder circles. Those default foreign files are refused by the
  strict reader.
- The [independent exact chamfer STEP import](evidence/eccentric-annular-chamfer-step-import-2026-11-15.json)
  reconstructs the pinned OCCT authored body with exact two-pole wall
  generators and seams. It preserves both cap holes and maps all
  14 original edges and 8 original vertices into the reconstructed
  8-face, 38-edge, 32-vertex solid. The independent canonical re-export
  retains positive signed material. The live author regenerates
  both fixture DATA sections byte for byte; changed support, seam,
  orientation and resource controls refuse. Main new and existing
  bore STEP races passed in 50.099 and 47.305 seconds respectively.
- The [independent exact chamfer IGES import](evidence/eccentric-annular-chamfer-iges-import-2026-11-15.json)
  recovers the same native JSON as the independent STEP fixture. It
  verifies six rational type-128 supports, original full rims and seams,
  the source edge and vertex lists, and both cap holes. Pinned OCCT
  reopens the original and canonical IGES with signed volumes
  `726.98826535488854` and `726.98826576241345`. The pinned author
  regenerates the original directory and parameter sections. The default
  rounded source and a hybrid with exact surfaces but the original
  off-support seam both refuse. Main's focused IGES race including
  existing bore rim regressions passed in 101.015 seconds.
- The [diagonal circle cap and wall proof](evidence/eccentric-annular-cap-nesting-2026-11-15.json)
  (`a296b700`) qualifies the independent offset `(1.5,2)` with exact
  circular rim laws and shell-wide clearance. A targeted raw constructor
  probe used to accept two nested caps with an interior crossing, an
  interior tangent, or a crossing at an inner-only station; the final
  proof rejects all three before returning geometry. The original
  1,024-triangle and chamfered 1,536-triangle meshes have complete zero
  self-contact and keep all native vertices. Four previous native and
  four previous display streams are byte-identical. Main focused live
  races passed both the modeling and trimmed packages.
- The [explicit PlainCAD v13 bridge](evidence/plaincad-v13-ordered-step-2026-11-15.json)
  (`399957cd`) qualified four exact interior cuts and three original
  joints in a closed 2,822-triangle display. It certifies display distance
  to 0.36 mm within its supported options. A fresh actual Node/WASM
  import strictly reopened the canonical STEP and reproduced the same
  native source receipt. Main focused native with the existing signed
  and tilted controls passed in 136.648 seconds.
  The [typed Chromium worker gate](evidence/plaincad-v13-worker-chromium-2026-11-15.json)
  (`0debedd8`) passed in 166.186 seconds on the audited pinned app
  archive, with both app and Playwright typechecks. Two fresh workers
  received Go WASM with HTTP 200 and no OCCT requests. Both reproduced
  the canonical STEP and exact native receipt digests; ordinary import,
  finer distance, and finite angular requests rejected. All 14 main
  Node harness tests passed.
- An [exact knot-aligned native display](evidence/mapped-v13-knot-aligned-native-2026-11-15.json)
  (`66d273d2`) returns 4,162 closed triangles for the tighter 0.30 mm
  request. Its certified two-sided distance is below 0.173 mm, with
  every roof triangle in an original one-sided support cell. It
  retains the original joints and has no unintended contacts. An
  exact original derivative check gives a counterexample to a
  0.2-radian angular request on each roof, so that option remains
  rejected. The earlier 2,822-triangle mesh bytes are identical. Main
  focused native passed in 44.310 seconds and the isolated race passed
  in 261.541 seconds.
- On the authored v13 source, the opt-in native finite facet angle path
  returned 2,126 closed triangles covering both original roofs and five
  planar sides. It keeps three exact source joints, checks 116,690
  candidate collision pairs with no unintended contacts, and certifies
  squared distance at most 0.007986104052 mm² and squared facet-to-source
  tangent at most 0.132665466495 for the 0.35 radian request. The
  main branch test passed in 163.754 seconds. Exact one-sided normals
  disprove a 0.35 radian facet angle on the unsplit closing edge;
  synchronized closing-edge subdivision is necessary. This proof is
  distinct from PlainCAD's normal-turn control across a facet.
- The native variable-count clipped Cut (`0091b243`) independently
  checks one, four and five contacts across two axes and two original
  footprint layouts. It agrees with the older two- and three-contact
  APIs. The corrected whole-source reader refuses a structurally
  consistent forged lower contact count, and a four-contact display
  is closed. Main focused native passed in 32.925 seconds.
  [STEP and IGES transfer](evidence/variable-transverse-cut-transfer-2026-11-15.json)
  (`5a03aaca`) verifies 18 separately indexed vertices, 14 faces and
  each positive component volume on the one-contact fixture with three
  solids. Default pinned readers considered the raw higher-count shapes
  valid but reversed the signed volumes of some thin lens shells. Both
  public writers return a scope error with no bytes above three contacts.
  The [pinned diagnostic](evidence/clipped-cut-occt-shell-orientation-2026-11-15.json)
  restores the positive volumes on identical source bytes when only
  the shell-orientation repair is disabled. Two repeated repair calls
  on affected shells toggle their signed volumes. Exact midpoint seams,
  owner-order changes and physical-size-preserving placement attempts
  did not qualify with the default pinned readers.
- Native variable-count Common (`548ee371`) and Fuse (`72f5b907`)
  extend the exact original-source domain to one through five transverse
  contacts. Common passed its 36 one-, four- and five-contact cases
  across original axes, footprint layouts and operand orders, with
  source replay, both legacy counts and the displayed four/five-contact
  shapes. The common scoped batch with nodal Fuse passed in 51.549
  seconds. Fuse passed the new one- and five-contact cases with independent
  integration of both original stock heights and their Common, plus
  closed display and legacy four-contact replay, in 63.566 seconds.
- Exact nodal cubic discovery (`9f054eb5`) recovers two arms, a closed
  loop and all four original owner rays. It rejects a cusp, an isolated
  acnode, a clipped loop, and irrational original endpoint examples.
  Native Common (`4cf4205c`) retains a closed 12-face body in both
  independently represented source orders, with exact volume
  `807933/65536` and area `32.5713789018693` from independent
  original-region integration. Its 128-triangle display passes exact
  contact checks. Main native passed in 0.907 seconds; the isolated
  nodal, clipped Common and cubic-tangency race passed in 150.237 seconds.
  A separate broad intersect run reached its cumulative ten-minute
  limit in the existing triple radial tube test.
- Native nodal Fuse (`8debec04`) keeps the selected original roof on
  each side of the node, closes 12 faces in both source orders and
  reproduces exact volume `821259/65536`. Its 512-triangle closed mesh
  is about 0.080% high in volume; the two Common recipe hashes are
  unchanged. [STEP and IGES](../exchange/NODAL_CUBIC.md) (`d3a3187b`)
  retain all 18 vertices, 28 edges, 12 faces, four node rays and the
  original seam for both operations and source orders. Pinned readback
  matches exact volume and independent area. The main full scoped
  exchange regression passed.
- [Native nodal Cut](evidence/nodal-cubic-cut-2026-11-15.json)
  (`85dcc1ba`) retains two distinct positive components with volumes
  `1/100925440` and `4619/100925440` and original separate point
  contacts. Its reverse order refuses the exterior pinch. The combined
  main nodal family race passed in 18.870 seconds. Both coarse previews
  are closed with only the declared contact. Their relative volumes
  differ by about `-20.6%` and `+13.8%`; at deflection 0.001 the
  same controls differ by about `-5.69%` and `+0.25%`. Exact source
  integrals remain independent of those preview limits.
  The [neutral-format Cut](../exchange/NODAL_CUBIC.md) (`bffac9bc`)
  passes each independent signed-volume test, with two separate
  solids, six faces, nine indexed vertices, and two incident rays
  on both original copies of the exact shared node.
- Exact cuspidal cubic discovery (`ffc10247`) proves the whole
  original triple-factor locus, two original boundary exits and both
  signed sides. The zero first cusp derivatives remain zero in the
  certificate, with equal second and opposed third sided jets.
  Focused main nodal and cuspidal geometry passed, as did the full
  isolate geometry and focused intersection races. An ordinary node,
  wrong footprint, irrational or clipped exits and altered original
  coefficients all reject.
- Native cuspidal Common (`fea3d0b4`) and Fuse (`750c9e80`) preserve
  the true singularity through exact original coedges, jet binding and
  source replay in both roof orders. Exact volumes are `277485/22528`
  and `279423/22528`, with independently integrated areas
  `32.5442218530794` and `32.8681291383202`. Both 352-triangle
  displays are closed and pass all exact contact checks. The main
  Fuse/Common and nodal regressions passed in 5.391 seconds and both
  earlier Common receipt digests stayed identical.
- [Cuspidal Common STEP and IGES](../exchange/CUSPIDAL_CUBIC.md)
  (`5a296f9c`) reopen one valid solid with nine faces, 20 edges and
  13 exact original vertices. An independent pinned derivative probe
  measured two zero first jets, identical second jets and opposed
  third jets on the actual reopened germs in both formats and roof
  orders. The main scoped neutral-format and nodal regression passed
  in 2.832 seconds.
- The [tilted through-bore bridge](evidence/plaincad-tilted-bore-bridge-2026-11-15.json)
  (`8e9cae2c`) keeps both cap openings in a closed 644-triangle display and
  re-proves the original source graph on native reopen. The integrated
  exchange and bridge races passed in 60.001 and 328.767 seconds; the
  latter includes all five repaired signed-frame previews. The pinned
  canonical OCCT test passed in 4.016 seconds and returns one valid
  11-face solid. Actual Go/Node WASM on immutable `8e9cae2c` passed both
  bore tests in 47.856 seconds. The isolated pinned browser and typed
  Node adapter evidence includes byte-identical strict re-export from
  a fresh worker; all 36 older exact STEP bytes were checked in isolation.

- Native projected Common/Fuse (`d77c55df`) and both Cut orders (`1618e9db`)
  retain the two independent original roofs, ordered contacts, and source
  replay. Focused main race times were 182.651 and 151.126 seconds for the
  trimmed package. The smallest Cut component is independently integrated
  as `1/38386270208`; 24 legacy byte comparisons remained unchanged.
- Mapped STEP (`b3d44702`) keeps three inferred events on one original
  unsplit Bézier. The raw owner pairs are `[17,40]`, `[37,32]` and
  `[65,24]`. Independent original material, exact stations, strict re-export
  and an actual redundant stored knot rejection passed in the focused main
  race (87.618 seconds) and full mapped STEP race (520.971 seconds) on the
  isolated patch. The isolated native mapped race passed in 209.857 seconds.
- Original five-fillet millimetre STEP (`7e647e57`) passed the main live
  race with four-fillet converted regressions in 73.750 seconds. It preserves
  original antipodal owners `#324` and `#633` and rejects the distinct
  OCCT-authored nonbinary control at its exact original local point.
  The focused main PlainCAD bridge race passed in 44.385 seconds. In the
  isolated [bridge evidence](evidence/plaincad-five-fillet-bridge-2026-11-15.json),
  all 18 typed Node WASM tests and 16 real Chromium two-worker tests passed.
  Exact native and browser receipts match, and on strict reopen the workers
  re-export the same source-unit STEP bytes. The five-fillet result has a
  400-triangle display and rejects the original nonbinary control and the
  tighter `1e-13 mm` tolerance before returning a shape.
- Native rounding of selected original orthogonal spans on a mixed line
  and arc sweep (`10df5ff0`) passed the focused pinned OCCT check in 5.382
  seconds. The editable document round, original extrusion, chained lateral
  offset and inset role tests passed together under race in 164.463 seconds.
  Both added radii and all unchanged arc names survived placement, radius
  edit, undo, redo and reopen. Duplicate identities, a curved incident span
  and an oversized round failed without publishing a document edit.
- Named complete cap overlay and existing planar face and edge history
  passed together under the main race in 43.025 seconds. The two split
  original rims share seam edge identities with both named wall sets,
  retain the curved parent's exact `8/11` midpoint and the line's affine
  law after a cyclic source reorder, placement and reopen, and reject an
  edit that separates the cap planes.
- Both analytic cap overlay formats reopen in pinned OCCT as one 22-face
  solid with volume `125π−20` and area `100π+32`. Strict reads with the
  supplied original profiles retain both wall/rim intervals and reject a
  mismatching cap. Main combined STEP and IGES overlay race passed in
  40.684 seconds; the pinned inset controls also passed. The standard
  exported solid carries the common partition; the native receipt keeps
  the independent originals.
- Three independent curved insets share the two host cap supports while
  retaining 40 faces, 96 edges, 64 original vertices and three separate
  exposed floors. Their closed display and owner incidence are verified.
  Independent OCCT agrees on volume `200+8.6875π` and area `284+20π`.
  Crossing guests reject before publication; canonical native source
  reopen rejects an altered guest edge offset and respects input limits.
  Analytic placed STEP and IGES both reopen as one 40-face solid in pinned
  OCCT and retain distinct original floors. The main overlay and multi-inset
  exchange race passed in 133.437 seconds, including rejection of a
  changed guest order or cap height in the supplied original sources.
  Named multi-inset document roles and strict placed STEP/IGES export
  passed with the existing inset and complete-cap document regressions
  in 368.403 seconds. Reorder and file reopen retain named original
  guest rims on the exposed floors and duplicate guest IDs reject.
- Named document occurrence export, the original editable inset exchange,
  and named circular fillet regeneration passed the main document race
  together in 324.615 seconds. STEP preserved both original source
  partitions and a composed tilted occurrence; IGES preserved both
  source partitions after an in-plane rotation and translation. The
  tests reject unsupported and inexact placements before writing bytes.
- Nonunit rectangular Common and Fuse retain the two exact affine roof
  charts and a reflected or positive sloping original base. The new
  translated `[2,4] × [-1,2]` cases and earlier projected envelope and
  Cut races passed together on main (2.251 seconds in geometry, 283.213 in
  trimmed). Independent physical integration uses area factor six and a
  sloping-base wall Jacobian; 36 legacy recipe digests were unchanged.
- Axis-aligned line-to-arc rounds passed their main race and pinned OCCT
  comparison in 37.574 seconds. Both internal and external tangencies
  qualify in either traversal order. Native source-clock samples and
  independent Green integral volume and area agree. OCCT independently
  measured volume changes of `-1.19703047079247` and `+0.817619549959659`.
  The exact tangent solver rejects irrational coordinates instead of
  fitting a nearby round.
- Two incident original arcs now retain both exact tangent points and both
  finite source maps through the rounded sweep. Pinned adaptive OCCT
  integration agrees within `1e-9` on the signed volume and area changes for
  both internal/internal and internal/external support contact. Default
  nonadaptive OCCT integration was off by `2.2e-6` for volume; no Go
  tolerance was widened. The focused fillet and editable document races
  passed in 47.832 and 59.198 seconds.
- Exact rounds with oblique original lines preserve the straight and
  circular source intervals and agree with pinned adaptive OCCT. This covers
  original line-to-line and line-to-arc corners and rejects a genuine
  irrational line normal. The expanded fillet main race passed in 76.788
  seconds, including the earlier circular and orthogonal regressions.
- Major and half-turn arcs and explicitly authored full circles retain
  their direction and exact start/end positions when split for native
  trims. Pinned OCCT independently authored a single 270-degree edge and
  a full circular opening; both native sweeps match their material. The
  main race passed in 16.616 seconds and rejected an unrepresentable
  required extremum without rejecting a short span that avoids it.
  The alternate authored-station proof accepts the same long support
  through exact nonaxis stations and rejects reordered, insufficient or
  off-circle stops. The main pinned race passed in 6.926 seconds.
- Nonunit Cut and prior native Boolean cases passed the main trimmed race
  in 284.486 seconds. Both Cut orders retain the original sloping base and
  both independent roof owners. Independent physical integration agrees
  on every component, including `3/19193135104`; 44 legacy recipe digests
  matched in isolation. The source-backed proof rejects skew charts,
  inexact physical contacts and an altered contact ledger.
- A single original spatially nonplanar mapped STEP cubic has four model
  poles with exact determinant `9/1048576`. Both original bicubic owners
  retain their mixed three inferred stations; the original and strict
  canonical cases agree with pinned OCCT material and enclose 414 display
  triangles. A changed exact interior pole rejects atomically. Focused
  main live race passed in 119.413 seconds and the isolated mapped native
  race passed in 247.649 seconds. The broader pinned mapped STEP race
  passed in 669.019 seconds.
- A separately authored rational nonplanar STEP cubic keeps raw exact
  model station parameters `8/7`, `8/3` and `24/5`, with the same original
  owner stops and 700 closed display triangles. The strict repeat
  export, changed interior weight rejection and existing receipt digest
  suite passed in 203.624 seconds under the main race.
- Original converted five-fillet metre and inch STEP, with both antipodal
  owners, passed the full main repair and original comparator race in
  491.593 seconds. The independent unscaled inch original fails the exact
  support gate at seven sites even though pinned OCCT accepts its
  approximate geometry. The isolated exchange proof keeps all 30 prior
  receipt, corrected-source and export digests. The [converted PlainCAD
  bridge evidence](evidence/plaincad-five-converted-bridge-2026-11-15.json)
  accounts for all 18 Chromium cases and 20 distinct typed Node cases.
  An older four-fillet metre case took 67.578 seconds under concurrent load
  on the first Node run and passed in 42.845 seconds on an unchanged 60-second
  budget on retry. The two new tests passed both times and preserved the
  original units, exact native certificates and strict re-export bytes.
  The integrated native bridge race passed in 157.372 seconds.
- On an oblique determinant-five footprint, Common and Fuse retain both
  independently charted original roofs and their exact sloping base, with
  unnormalized original wall direction witnesses. Both axes and operand
  orders agree with independent volume and area calculations. The main
  native race with rectangular regressions passed in 359.033 seconds,
  and the independent geometry volume race passed in 5.200 seconds.
  All 48 earlier native recipe digests matched in the isolate.
- Both oblique Cut orders and native rectangular regressions passed under
  the main race in 307.478 seconds. Both original roof and wall owners
  survive with their exact shared sloping base. The smallest independently
  integrated component measures `5/38386270208`. Main source reload
  rejects changed physical chart and component ledgers; all 56 previous
  receipt digests matched independently.
- The strictly reopened rational nonplanar STEP also retains independent
  first-owner raw stops `32,68,112` and reversed second-owner stops
  `126,82,46` at model parameters `8/7,8/3,24/5`. Native replay proves
  distinct original owner laws and a 700-triangle closed display. Both
  false owner weights reject. The main live and byte compatibility race
  passed in 302.752 seconds.
- Both off-seam rational roofs preserve the three model and independent
  owner stations. The closed display retains all three exact stations
  in 3,790 triangles at 0.10 face and 0.0125 boundary deflection. The
  stricter 0.04 face setting exhausted the existing work budget. Pinned
  original and canonical solids agree, and an independently authored
  false interior support weight rejects before publication. The main
  focused and legacy receipt race passed in 568.600 seconds.
- Independently authored tilted STEP authentication recognizes both
  original interior cap wires, the inward cylinder and its paired periodic
  seam. Its corrected transfer reopens in pinned OCCT as one eight-face
  solid with volume `28.096231102869` and area `73.084237522361`.
  The focused main race passed in 16.957 seconds, including the original
  blind control and edited owner rejection. The source proof alone does
  not widen the default STEP reader. Exact native reconstruction and
  strict canonical reopen passed the broader main race in 98.134 seconds,
  including preserved exterior-only paths. Canonical OCCT readback retains
  one 11-face solid with four inward bore patches and matching volume,
  area and centroid. Altered cap winding, borrowed loops and missing or
  reversed bore walls reject.
- Native Common on distinct partially overlapping or nested axis-aligned
  footprints retains the exact original roof charts, the original sloping
  base and the source ownership of all clipped walls. One-axis and two-axis
  physical volumes are independently `493395797792490953145/41216943789694779392`
  and `370046848335928793529/41216943789694779392`. Main exact clipping
  and trimmed races passed in 14.903 and 169.686 seconds; 60 previous
  recipe digests matched in isolation. Face-touch-only footprints,
  gaps, nonbinary owner bounds and edited original extents reject.
- Exact geometry on unequal footprints now partitions both complement
  regions and records every original footprint side and directed wall
  interval for Common, Fuse and both Cut orders. Independent integrals
  preserve positive overlap regions down to
  `112545421/22540516134989332480`. The separately checked main geometry
  races passed in 10.678 seconds for the arrangement and 23.949 seconds
  for the volume and wall ledger. Native stepped-wall assembly remains
  open for Fuse and Cut.

- Quintic native Common/Fuse and Cut with three transverse crossings and a
  quadratic tangency, plus exact curved inner wires, passed a combined main
  race: 34 top-level cases across `geom` and `brep/trimmed`. Pinning geometry
  `400e24a4` to the 31 ordered degree-five multiplicity patterns passed its
  main race in 56.722 seconds. Native five-crossing Common/Fuse passed its
  main race in 107.633 seconds, with all five contacts and original-source
  reload.
- Original directed STEP `98648522` passed the circular STEP main race in
  86.355 seconds. OCCT independently authored the two-hole model and read
  its original, source-preserving and canonical files as one 14-face solid
  with volume `48−π` and area `88+3π`. The source-backed native receipt
  `f1e7830d` passed the focused main race in 8.204 seconds, including
  failure on changed cap-bound IDs, ring offsets and source circle sense.
- The two-cell mapped v9 live main test retains all four source stations and
  a closed display of 1,262 triangles. Pinned OCCT read the original and
  re-export with volume `0.951533576007932` and area `6.22315111712234`.
  Earlier v7/v8 native and STEP records passed unchanged. The four-fillet
  live STEP main race also passed. It verifies 48 original owner charts and
  rejects the automatic cap's exact `1/5000000000000 mm` displacement.
- Five-crossing Cut `99e2923f` passed its main race in 283.796 seconds.
  The forward Cut preserves two solids with five shared points; the reverse
  preserves six positive components, with a smallest exact volume of
  `1015625/1152921504606846976`. The ordinary display collision diagnostic
  reports an incomplete result at its million-pair limit. The exact contact
  and source reload proofs passed independently.
- Mapped v9 through three original cells `d857b12b` passed its main live
  and compatibility race in 74.384 seconds. It retains event counts `2,2,1`,
  60 proved edge uses, and a closed 1,310-triangle display. OCCT read the
  original and canonical re-export with volume `0.954463263507933` and
  area `6.26467813350028`; five earlier recipe pairs stayed byte-identical.
- Canonical directed IGES `ec0f5558` passed its focused main race in
  23.400 seconds, including pinned OCCT readback of the nine-face reentrant
  solid and the 14-face two-hole solid. It rejects edited source arcs,
  cylinder sense, cap loop order, and orphan data.
- Original directed IGES `f91cd74e` passed the focused main race in 18.275
  seconds. The exact local-chart author preserved all 16 original model
  arcs and 32 explicit cylinder pcurves. OCCT read the original, byte exact
  replay, and canonical export as equivalent 14-face solids. The default
  shared-chart author has rounded original arc endpoints; the strict mode
  rejects it. The test also rejects a lone 360-degree owner phase shift,
  an edited transform, and a borrowed pcurve.
- The four-fillet application harness `e9832653` passed the main native
  race in 33.513 seconds; its independent pinned runtime qualification
  passed Node 15/15 and real Chromium 13/13. The typed workers and native
  source receipt share certificate SHA-256
  `4893ea75e2b32523a443a2625c1c1b44b0b919b9da31fdb5fece8177d14ab7c5`.
  The Node and Chromium bridge exports match byte for byte. Standalone
  STEP has the same DATA records and different file header formatting.
  The browser reports, build hash, and manifests are retained under
  `.worktrees/four-convex-arc-bridge-evidence`.
- Mixed cubic-crossing and quadratic-tangency envelopes `5bca7749` passed
  their main race in 120.543 seconds, retaining the signed source regions,
  exact original restrictions and both pcurve owners. Pinned OCCT returned
  empty shapes for these near-gap stocks; independent rational integration
  supplied the positive volumes. Original affine chart alignment `2319d1f3`
  passed the geometry race in 50.736 seconds, including both source axes and
  orders, six common refinement cells and an independently integrated unit
  footprint. Its scope is geometry and does not substitute aligned supports
  for original native owners.
- The directed document naming and reentrant fillet regression passed the
  main race in 57.043 seconds. Named walls and rims survived source wire
  reordering, resized holes, placement, failed edits, undo, redo, and
  canonical reopen. Complete `go vet` and Windows amd64 cross-build on
  immutable `12ab5dbf` passed; Windows runtime remains unqualified.
- Mapped v9 with an event-free middle original edge `2f0a3932` passed the
  main race in 75.627 seconds. The three retained raw event pairs are
  `(17,40)`, `(37,32)`, `(858,−1)`, with the last on an original unsplit
  Bézier. Pinned OCCT returned one seven-face solid and the closed display
  retains all three events in 4,022 triangles. Existing v7–v9 source DATA,
  canonical STEP, and native recipe checksums stayed unchanged.
- Signed lateral offsets `336bb29a` passed the main race in 9.486 seconds,
  including independent material for two holes, the reentrant tangent arc,
  unchanged cap elevations and a zero offset. Collapsing arcs, colliding
  original holes and inadequate separation work reject atomically.
- Converted four-fillet metre and native exchange `b1eaeaee` passed the
  main live race in 24.619 seconds. The original shifted spline owner
  rejects at `1e−13 m`. The unequal axial trim keeps the unmodified projected
  endpoint `5000+4e−26` and the separately proved finite station `5000`.
  Twelve earlier receipt, corrected STEP, and canonical STEP checksums
  remained identical. The isolated native-four full STEP package coverage
  separately reconciled all 1,514 named cases (1,255 passes, 259 optional
  skips) across the timed prefix and same-commit tail replay.
- Curved inset join and compact regression `83e18008` passed the main
  race in 21.289 seconds. The original host through-hole survives, and an
  independent floor on the same cap support is exposed inside the guest
  through-opening. Both cap directions preserve 48 edges and 20 faces,
  reject touch and gaps, and match independently authored OCCT material.
  The compact mesh closes with 112 triangles under the bounded display
  test. Native JSON replay preserves the resulting exact cap and floor trims.
- Curved inset STEP `96598a09` passed its main race in 29.556 seconds, with
  independent OCCT readback of both cap directions as one 20-face solid.
  Four prior signed circular writer fixtures were byte-identical. Curved
  inset IGES `ff9b4dd0` passed its main race in 42.597 seconds and the same
  independent topology and material checks. Both readers reject a changed
  original floor sense. The bounded 15-second fuzz campaigns executed 136
  STEP inputs and 87 IGES inputs without failures.
- Named document offset and inset joins regenerate original walls, rims,
  host floors, and deleted guest caps through edits and reopen. The composed
  placement export `13e3cc22` passed the main race in 43.746 seconds. STEP
  preserves the feature and assembly pose, and IGES preserves a proper planar
  rotation and offset cap elevations. Unsupported poses and inexact placement
  composition produce no output.
- Independent roof chart envelopes `21e54f8d` and Cut `427496fc` passed
  main races with original roof, wall and seam owners. The aligned Cut and
  envelope test passed in 52.914 seconds. Its exact volume integrals cover
  both Cut orders and remain attached to the separate native components after
  source reload; existing same-chart recipes were checked byte-for-byte.
- Two asynchronous inferred roots on one original Bézier `22d763a4` and
  one shared inferred root `9aef542f` passed their live STEP races in 42.894
  and 31.363 seconds. Both reject an independently authored redundant stored
  model knot. Closed displays retain the original stations, and prior native
  and canonical recipes stayed unchanged.
- Four-fillet converted metre and inch bridge harnesses passed their latest
  combined main native race in 65.402 seconds. Pinned scaled inch and metre
  STEP passed together in 37.411 seconds. The isolated runtime qualification
  reports 17 typed Node cases and 15 Chromium worker cases. Inch native and
  worker exact certificate SHA-256 is
  `857dd1a5b99beb1c75206a9c3e215a94af986a90d70e5f0d4ac4b17039f442c4`.
  The real unscaled inch file fails exact reconstruction at three original
  tangent supports, and tighter source budgets reject atomically. Prior
  receipt and corrected/canonical hashes stayed unchanged.
- Complete circular cap contact `991dfed0` passed the main race in 19.378
  seconds, removing both internal caps and retaining the 12 common rim
  edges, 26 faces, and separate original walls under either input order.
  Pinned OCCT independently confirms material volume `240−2.5π` and
  area `236+9π`. A valid source with an alternative circle at the same
  original endpoints is rejected.

Focused main verification adds two bounded cases:

- `d54df03f`: the exact full-face join checks both winding and every original
  perimeter ID, proves the complete overlap without mesh welding, removes
  the two interface faces, and revalidates the resulting embedded boundary.
  The independently assembled concave L prisms have 18 result vertices,
  30 edges, 14 faces, exact volume 10 and area 34. Point classification
  checks the removed interface, its seam, and the concave exterior notch.
  Shifted, wrong and same-oriented interfaces, quota exhaustion and
  cancellation publish no result. Both `collision` and `csg` focused
  race tests passed.
- `f490779d`: the live OCCT v7 three-support-knot STEP group passes under
  race. Native and canonical STEP round trips retain twelve independently
  authored positive-rational bases and their exact model and chart domains.
  Pinned OCCT independently reads one seven-face body, volume
  `0.93318617483601` and area `6.09166222908489`, from original and output.

- `c8776436`: matching inner and outer planar wires close two opposing
  square pockets into one shell and one inward void, with exterior volume
  32, cavity volume -6, and total area 84. An exact centroid in every
  component of the boundary outside the common face proves no hidden
  overlap. The `collision`, `csg` and `csg/poly` race suites passed.
- `7f3cc2a3`: two crossings and one quadratic tangency qualify both native
  directed Common and Fuse across all 24 cases. The original stored roofs
  retain six signed regions, 12 section edges and three typed junctions
  through canonical native reload. Main race passed in 279.553 seconds.
  On these narrow mixed fixtures the separately authored OCCT probe
  returned empty output; exact original source integration and independent
  area samples are the qualification witnesses.
- `8837b5d1`: the grouped converted-metre author and reread passed the
  targeted STEP exchange suite under race in 90.518 seconds. An independent
  OCCT source has eight original cylindrical spans, two rational caps and
  18 planar owner charts. Correction and canonical readback retain the
  original metre model and agree with independent OCCT at about
  `29077978807.11554 mm³`; the `1e-12 m` proof passes and `1e-13 m` fails.

- `c809c6a7`: in the twelve main mixed Cut cases, the two operand
  orders return two outside components or four ordered inner components.
  Three exact contacts, original owner curves and independently integrated
  rational component volumes survive native reload. The main race passed
  in 105.103 seconds. The optional pinned OCCT diagnostic omitted every
  positive interior reverse layer in the twelve independently clipped
  input slabs.
- `f49b393b`: two quadratic tangencies in independent cubic/quartic
  original roofs qualify five signed regions, six exact original
  restrictions and both directed sets of contact rays. Two fixtures
  gave independently integrated lens areas `1/960` and `7/7680`.
  The focused and earlier mixed main geometry races passed in 78.385 seconds.
- `6d7d1764`: opt-in v7 mapped STEP preserving one shared rational
  owner basis and six independent full bases on the other owner passed
  main race in 35.054 seconds and the isolated broad mapped STEP race in
  91.025 seconds. Native and canonical replays are byte-stable in either
  roof order; original and re-export agree on the pinned OCCT material.
- `49b5c1f0`: grouped converted-metre app qualification passed seven
  native source-unit checks, all ten Node WASM tests and eight Chromium
  tests. Both workers independently reproduced receipt SHA-256
  `03303d736d014b2f6cf8200bd2c43833d50f9492aeddff6b4d5b64e7165e1c64`
  and the exact canonical metre STEP. Common WASM SHA-256 is
  `d5e75cdf0e4862c07410a4af3f981ae98fff88f5674787c2822fc36433fb197e`.
  Durable original app reports are under `.worktrees/grouped-converted-metre-bridge-evidence`.
- `6d32b6c5`: independent original and canonical STEP and IGES readback
  retain two separate square tubes, 20 faces, volume 24 and area 96.
  Joining after import removes the shared annulus and produces one volume-24
  body with area 72. Original finite degree-one planar pcurves are bounded
  by their exact source knots, model line speed and both lifted endpoints.
  A same-line shortened pole, shifted knot, reversed owner and altered
  speed all reject without returning material. STEP and IGES targeted
  main races passed.
- `5d9d9067`: matched complete original planar faces can fold around a
  seam or fill an open pocket. The seam case retains volume 4 and area 16
  with 14 vertices, 22 edges and 10 faces. The five-face pocket case
  removes exactly four original buried vertices and eight buried edges
  from each operand and retains exact volume 16 and area 48. Prior
  planar cavity and broad `collision`, `csg` and `csg/poly` races passed.
- `71a77941`: grouped converted-inch transfer passes the live original
  metre/inch exchanges under race (26.363 seconds on main; 77.679 seconds
  in isolated regression). The source checks 18 planar owners, eight
  cylindrical spans and two rational caps. `1e-11 in` accepts and
  `1e-12 in` rejects. Rounded unscaled OCCT input loses two original
  plane/curve tangencies and is rejected. Main app integration remains open.

- `00f2030f` and `a5b3483c`: exact native Common, Fuse and both Cut
  orders across two quadratic tangencies pass the combined main race in
  388.622 seconds. Six exact restrictions survive as eight model edges
  in the envelopes. The directed Cuts retain two or three components,
  both distinct tangent contacts, and independently integrated rational
  component volumes before and after canonical reload.
- `b3bb412c`: independently authored unsplit mapped v7 STEP retains one
  stored model knot and both owner-only cuts. Reload preserves nonnormalized
  exact weights and incoming/outgoing raw values 457 and 700 at the same
  original model vertex. Main live STEP race passed in 36.289 seconds;
  isolated broad mapped STEP, geometry, parametric and mapped native
  races also passed. Source re-export is qualified separately.
- `572f5b22`: grouped converted-inch native race passed in 61.292 seconds.
  An unchanged isolated Node replay passed all 11 after an unrelated
  grouped-metre case hit its 60-second deadline in the earlier concurrent
  run. Chromium passed 9/9; both workers reproduced receipt SHA-256
  `9fb8c4fd15953606c0b2bc522108ea50aca5e39916e92c5b235ecbb99deb2f67`
  and canonical inch STEP. Common WASM SHA-256 is
  `343ec4fa996b5b40429e24165323188dcf7b3f82e509aad3f398767dc08a4553`.
- `a052e2c4`, `42c44383`, `addd1829`, and `c7756665`: exact regions
  can join across different original face divisions, and discovery reproduces
  the explicit proof. Native reload rejects modified owner, seam, area, or
  volume claims. Live independent STEP and IGES originals and readback
  retain two original solids, 15 faces, volume 96 and area 248. Their
  refined join has area 152. An earlier untranslated OCCT IGES diagnostic
  correctly failed because original finite line #49 differed from its
  endpoint vertex by approximately `2.2e-16` in model units.
- `c5dffc88` and `509cdbd3`: planar boundary split checks preserve
  original face and vertex IDs, split stations on outer and inner wires,
  and exact `1/3`, `2/3` parent intervals. An exact overlay combines
  different original seam stations, mapping three result edges back to
  both original owners with no geometric displacement. Foundations and
  focused join races pass. Work and station limits reject atomically.
- `016354ee`: original cubic/quartic roofs with an isolated order-three
  crossing reproduce equal first and second jets, a strict third
  derivative gap and complete source-region proofs. Main geometry and
  prior profile regressions passed under race in 35.073 seconds. Vet and the
  Windows amd64 cross-build on this exact revision returned zero.

- `eaa2e7ed`, `96d938d5`, and `bd2f861e`: the native order-three,
  mixed simple/order-three, and fourth-order Common/Fuse/Cut cohorts
  passed on main under race in 452.650, 190.780, and 165.728 seconds,
  respectively. Each case compares exact original source integrals and
  re-proves the stored topology on reload. Independently authored OCCT
  probes omit positive components or return inaccurate stock-scale
  volumes on these narrow profiles; they are diagnostic comparisons.
- `42c473e8` and `ed026efc`: captured original unsplit mixed STEP
  re-export preserves the authored `0/2/4` graph and normalized native
  v7 bytes. A scaled, reversed original also retains its distinct raw
  stations. Native v8 admits the separate four-cut original with raw
  `1,2,4,6` on the stored parent domain `0/4/8`; only raw 4 is stored.
  The four exact owner pairs, closed 4,922-triangle display and pinned
  original/canonical material pass under race in 185.226 seconds.
- `b1bf97b9`: independently authored three-fillet STEP retains ten
  repair sites, two finite axial maps and 36 original owners, including
  two rational caps. Native focused race passed in 43.356 seconds.
  Node passed 12/12 and Chromium 10/10 with matching actual WASM
  `03bb9ebe36ed79fb968164344d4f25e341c9bac2469488b2c74fb676350fdcff`.
  The 0.5-mm display has 376 triangles and both workers reproduced
  the exact original repair and canonical file.
- `05ce6e82`, `b463b416`, `b7c1d824`, and `e547daae`: a whole
  original tool face can meet the strict interior of an original host
  face. Several disjoint footprints become distinct holes in the same
  retained face. Live STEP and IGES preserve the independent 16 source
  faces and volume 25; the joined bridge has 14 exterior faces and area
  76. Document face/edge histories survive edit, invalid commit, undo
  and reload. Separate named edge split recipes compose exact thirds
  across a two-owner seam, with three fully owned result edges.
- `8e8c39f0`: `tools/ci/verify-go-evidence.py` independently
  reconciles top-level test names, packages, terminal status and lane
  selection. It reproduced the complete `bafe7e81` totals and rejected a
  deliberately duplicated terminal event without emitting a manifest.
  The `d5906fde` three-contour discovery race completed in 2978.900
  seconds; the independent omitted-inner-contour check and the remaining
  ordinary intersection cases also completed.

Further main milestones use the same pinned optional OCCT reference:

- Four-crossing Common and Fuse passed the exact source region and independent
  measure tests under race in 212.406 seconds. The four-crossing Cut main
  race in the combined mapped run finished in 304.339 seconds and verifies
  exact signed volumes of all parts before and after native reload. OCCT
  diagnosed empty or inaccurate outcomes on the narrow lens inputs; the
  qualified native volumes are exact integrals of the original roofs.
- The planar contact suite independently verifies STEP physical unit
  factors, source face direction, grouped IGES owner lists, through-holes
  and separate cavities, plus each open line span at a shared edge.
  Full-edge STEP and IGES races passed in 6.137 and 5.914 seconds.
  Native partial-edge proofs and JSON reload retain both original
  rational ranges and reject edited ranges. Source knots are not moved.
- The native three-support-cut mapped v7 source retains original
  stations `1/16`, `1/8`, `3/16`; the raw parameter pairs are
  `(93,443)`, `(197,387)`, and `(317,283)`. Independently authored
  pinned OCCT and native STEPs contain one valid seven-face solid
  with volume `0.93318617483601` and area `6.09166222908489`.
  The foreign unsplit input retains one original material knot and
  recovers both other stations by proving the original owner restrictions.
- Converted-metre and scaled converted-inch original transfers passed
  native, Node WASM and real two-worker Chromium verification. The inch
  import accepts a `1e-11` inch repair budget and refuses `1e-12` because
  of the proved original chart drift. The separate unscaled source has
  an independently measured exact tangency obstruction and stays rejected.
  Durable logs remain in `.worktrees/converted-metre-step-evidence` and
  `.worktrees/converted-inch-bridge-evidence`. The focused main converted
  bridge races passed in 46.413 and 29.591 seconds.
- An immutable `aa59655c` checkout passed `go vet ./...` and a Windows
  amd64 cross-build. A build does not qualify Windows runtime behavior.

## Full baseline and recent exact modeling qualification (2026-11-14)

The [normal Go record](evidence/main-6c4ab7c4-qualification-2026-11-14.json)
accounts for all 1,081 trimmed top-level tests on `6c4ab7c4`, with 1,068
passes and 13 explicit skips. Across all packages, 4,567 top-level tests
passed and 421 optional tests skipped. The [race continuation record](evidence/native-main-18204a56-2026-11-14.json)
retains the original watchdog logs and the exact selection of 804
remaining cases. The continuation completed with 792 passes and 12
optional skips. The combined trimmed inventory has 1,023 passes and
13 skips, with every original top-level test accounted for exactly once.
The [full PlainCAD record](evidence/plaincad-main-0e57c2d8-2026-11-14.json)
includes the passing development, cross-browser, unit, WASM contract,
type and size runs on `0e57c2d8`. All 66 production cases passed after
the test driver's temporary Chromium response buffer increase. The
unadjusted control retained one inspector-cache eviction.

The [Go `4078f85a` rerun](evidence/main-4078f85a-qualification-2026-11-14.json)
records the three preflight fixture failures and the complete pinned STEP
package rerun with the exact one-line catalog correction `36380200`.
The corrected STEP run passed 1,098 top-level cases with 214 optional
skips in 665.561 seconds. The three unchanged trimmed lanes finished
with 1,204 passes and 15 optional skips across their exact 1,219-case
inventory.

The independent OCCT prism comparator passed 48 original constructions,
including through, one-ended and interior side cuts against a shared stock
wall. Exact affine imprint subdivision produced 60 triangles with 35
opposite auxiliary pairs across 12 original faces, in both operand orders.
Sewing each original shell preserved exact volumes 32 and 6, each with
Euler characteristic two. The integrated affine and mapped coincidence
race run passed in 3.824 seconds. The independently authored rounded
nested STEP group, including native reload and live XCAF checks, passed
under race in 83.160 seconds.

Independent anisotropic OCCT wall and cap authoring compares 40 filled
and 80 bore surface samples and exact native material enclosures. It
passed under race in 36.221 seconds together with the affine face
fragment tests. The separate STEP tests import and re-export original
filled and bored NURBS, including different inner and outer station
grids, and compare each result with independently authored OCCT source
geometry. The original raw and converged OCCT area measures are recorded
in the material oracle notes. Live XCAF checks verified full original
filled and bored flat assemblies, one shared product, and cyclic and
proper negative-axis occurrences. The integrated group passed under
race in 60.875 seconds. The coincident trim section test covers a
reversed original cubic over knot range `[2,6]`, the opposing face's
hole, two retained closed-face runs, and source-bound native reload.

Independent OCCT split the reversed simultaneous affine Cut into two
solids, with volumes agreeing with the exact `7/32` and `77/64` result.
The native arrangement proves their sole shared vertex from opposite
original halfspaces. Two inclined cap Fuse fixtures with genuine
nonparallel extrusion directions produced volumes `39/4` and `25/2`
in both operand orders. Their integrated dispatch and live comparisons
passed under race. Independent OCCT returned an empty regularized Common
for the quartic closed tangency ring and a Cut with unit volume, area six
and the original stock centroid. A rotated nonplanar repeated loop
and a rational double-contact arc clipped against a slanted original
hole passed their focused race checks, including exact algebraic roots.

## Native qualification and piecewise reconstruction (2026-11-14)

The 65-minute pure-Go short run at `3a488eb5` completed every package except
`brep/trimmed`. The package timeout interrupted the selected tube collection
test 51 seconds into that case. The immutable branch proof is now reused
for the original XY projection, with the new point and event proof still
charged to the caller's exact arithmetic allowance. Focused original and
permuted projection race checks passed in 1.489 seconds.

The full pure-Go `18204a56` run is recorded in the
[native run checkpoint](evidence/native-main-18204a56-2026-11-14.json).
The two completed full trimmed normal groups took 4,245.252 and
3,907.519 seconds. The normal run outside trimmed passed all 50
packages with tests. The selected tube region collection passed in
122.88 seconds under the concurrent qualification load. The selected
disk and annulus fixture passed in 974.48 seconds and produced 20,128
triangles on 10,066 vertices. Shard 2 reported a stale assertion
checking degree seven as invalid when the production ceiling was
eight. Commit `313dfb62` uses degree nine. On the pinned source with
only that test correction, the fixture passed both normal and race.
The original result remains in the log.

Ordered Common of a genuinely creased original roof retains two distinct
roof fragments and their one-sided slopes. The 10-face solid has one shell
and Euler characteristic two. Analytic area and volume agree with live
pinned OCCT, and native reload rechecks the boundary. A second fixture
switches original roof owner exactly at the knot. Subsequent Common and
Fuse tests imprinted crossings within single and multiple original spans.
The creased stock crosses the other roof at X=1/4 and X=3/4. Common and
Fuse each sew one 18-face shell and agree with independently authored
OCCT on area and analytic volumes 35/16 and 37/16. A nonrepresentable
crossing rejects before publishing topology. The four focused original
knot and crossing tests passed under race in 11.301 seconds.

The corresponding Cut retains whole creased upper bands, both bands
of a fully separating cutter, and creased source faces on either side
of a strictly enclosed cavity. Authored OCCT results agree with the
analytic volumes 3/4, 11/4, 2, and 45/8. A terminating cut band or
cavity contact remains a separate topology proof.

Exact native source-use restriction preserves quadratic and cubic controls,
outer and hole ownership, and orientation under nonbinary restriction.
The integrated focused race passed in 1.343 seconds. Original cusp recipes
reload exact terminals `5/72` and `59/72` and re-prove both original supports.
The integrated cusp persistence and sampling races passed in 1.357 seconds.

The noncircular rational station loft preserves the original three-station
crease and exact shared rims after native reload. An independent weighted
profile integral gives material volume 28.5738627004. The 288-triangle
display has volume 27.8423645320 at deflection 0.2. Changing only interior
rim weights rejects the loft even when station corners agree. The focused
integrated race passed in 8.156 seconds.

The mixed original source section-to-native pipeline retains 12 exact
source uses. The resulting six faces and 18 edges enclose volume 0.125.
Native JSON rechecks every original edge and the affine tube endpoints.
One-ULP vertex displacement fails on read and on write before publishing
output. The focused integrated race passed in 53.640 seconds. The
noncircular loft's separate exact point tests distinguish rational points
inside, on, and outside the original weighted profile, and check original
cap and crease owners.

The quadratic-warped source fixture retains eight genuinely curved
tube uses and passes native write and reload using an exact paired
chart-path proof. The full six-face, 30-edge result holds the original
volume 0.125. The initial focused integrated pipeline race passed in
47.795 seconds. The bounded automatic paired-chart fallback eliminates
caller tubes on the same original fixture. Its independent coverage
rejects an omitted interval and an off-lattice second branch. Its full
integrated race passed in 161.849 seconds. Exact quarter-position
discovery passed separately. Returning an immutable verified tube
and diagnostic report from a single proof reduced one 32-slab
discovery from 95,013 to 54,053 exact operations. The focused
geometry and discovery race groups passed in 1.468 and 36.107 seconds.

The wrapped rational station loft line test independently
checks two disconnected interior spans, an oblique rational exit,
irrational root brackets, cap and crease ownership, and multiplicity.
Its integrated race passed in 20.526 seconds.
The cap and selected generator coincidence tests preserve original
endpoint owners. Exact section extraction retains both wall IDs
at the waist, and retains the weighted chart at nonbinary heights.
These integrated race checks passed in 25.278 seconds. The separate
exact rational mass proof bounds the sectional area with inner chords
and outer control polygons. Independent symmetric and asymmetric
integrals passed under race in 7.920 seconds with a volume enclosure
width below 0.01.

AP242 product-local palette input and the canonical STEP round trip
agree with live OCCT XCAF on both local colors, three occurrences,
volume 54 and area 126. The integrated race passed in 1.467 seconds.
The wrapped spline p-curve exact decimal gate rejects distinct source
coefficients below a binary64 ULP and excessive exponent spellings.
Original and re-emitted wrapped tube STEP still agree with OCCT's
adaptive area and volume. The integrated live race passed in 4.408
seconds.
Flat AP242 contextual styles retain exact occurrence, placement, body,
preceding style and context ownership. The source and re-export preserve
red, blue and green occurrence colors, volume 54 and area 126 in live
OCCT XCAF. The integrated occurrence race passed in 2.253 seconds.
The source-backed native appearance envelope revalidates original
geometry, poses, styles and contextual record ancestry on reload. Its
integrated live race passed in 2.489 seconds.

Independent weighted loft tests cover exact crossing Common and Fuse,
strict through-bore Cut, split across different inner and outer knot
grids, and exact annular lines preserving both cap-rim ownership sets.
Tests distinguish cap-plane boundary intervals from exposed material
interior. Weighted bore display and the prior circular display passed
the integrated race in 73.863 seconds. Proper rational placements also
preserve exact squared distance, original owner IDs and certified mass
intervals.

The flat AP242 override writer can create a contextual style, edit it
without changing unrelated entity records, and remove it to restore
the original product color. The source-to-native-to-STEP sequence
preserves exact source ownership and has independent OCCT XCAF
readback. The final create/edit/remove integrated race passed in
4.588 seconds, and removal completed 40 bounded fuzz iterations.

Original affine patch fixtures exercise full coplanar overlap, tangent
point and segment contacts and noncoplanar transverse intersections.
The source patch contact API re-evaluates each published vertex on both
original charts and returns original side masks. The focused integrated
race passed in 1.169 seconds.

## PlainCAD qualification on kernel 18204a56 (2026-11-14)

The [run record](evidence/plaincad-main-18204a56-2026-11-14.json) links
the raw reports and build hashes for app `efae4d2` and kernel
`18204a56`. The original development run passed 342 of 345. The
three modeling failures were font requests denied by Vite because the
dependency directory was symlinked outside the workspace. The same
WASM and three original fixtures passed with dependencies physically
inside a clean stage. The original production run passed 65 of 66.
Chromium evicted the 16.5 MB WASM response before the CSP test could
hash it. The focused control passed, and the full production rerun
with the same app, port and fixtures and the larger DevTools capture
buffer passed all 66 cases with zero skips, retries or flakes.
Chromium, Firefox and WebKit passed all 15 configured cases. The
unit suite passed 1,691 tests in 220 files once its legacy OCCT fixture
assets were accessible in a local dependency stage. The four selected
Go WASM contracts passed separately. Both TypeScript checks and all
72 JavaScript bundle budgets passed. The corrected full development
run passed all 345 original fixtures in 35.1 minutes with zero skips,
retries or flakes. A newer build at `1d938079` separately passed the
three original XY/XZ/YZ modeling cases and four selected WASM contracts.

## PlainCAD acceptance on the main merge 8f76f16b — 2026-11-14

PlainSolid merge `8f76f16b` combines the committed main snapshot
`d1e9a5dd` with the verified reconciliation `f87dd4f3`, including
integration work through `e2e9a253`. The isolated PlainCAD copy at
`efae4d2` selected PlainSolid explicitly. The browser build used the
matching Go runtime and a stripped WASM. All ten standard WebAssembly
sections match the ordinary build byte for byte.

[The merged-kernel report](evidence/plaincad-kernel-main-8f76f16-2026-11-14.json)
records 23 passing original modeling cases, including multi-edge
Fillet/Chamfer, the long feature chain, guided and grouped Hole work,
and STEP import and export. All 15 Chromium, Firefox and WebKit cases
pass. Targeted native race checks with the independent OCCT reader
also pass. The original production suite passed 63 of 66 tests.
The unchanged PNG export and performance cases passed on focused
reruns, with the performance case taking 43.5 seconds. The original
CSP case exceeded Chromium's response capture buffer. The unchanged
assertions passed after increasing that buffer in a temporary local
harness. The ordinary harness result and the diagnostic control are
preserved separately. The full production rerun with only the local
Chromium capture buffer enlarged passed all 66 tests with no skips or
retries. Its benchmark finished in 26.3 seconds and checked all 25
edits and 11 STL downloads. The full ordinary unit and component suite
passed 1,691 tests. All four WASM provider contracts passed with the
original timeout after test contention subsided. The full development
run passed 344 of 345 cases. The first AI canvas test timed out during
its initial click and passed unchanged in a focused rerun on the same
development server. The report records both outcomes.

## PlainCAD tighter covered-width Hole STEP — 2026-11-14

Local interpolation bounds also qualify the 5 mm and 4 mm radius
crossing at a 0.75 mm transverse offset, where the wider cylinder
covers the narrower cylinder across its full transverse width. The
regular quadratic radicals and original analytic supports remain in
the kernel. Signed offset cases use the same bounded STEP fallback.
The [authored case](evidence/plaincad-tight-covered-step-native-2026-11-14.json)
round-trips the 378,805-byte STEP and exports 1,672 watertight STL
triangles. Independent OCCT matches the direct Boolean. The
[17 original PlainCAD cases](evidence/plaincad-tight-covered-step-browser-2026-11-14.json),
focused race checks, and vet pass.

## PlainCAD tighter partial-overlap Hole STEP — 2026-11-14

The uniform fourth-derivative bound previously rejected the tighter
valid crossing. The fallback partitions the angular interval and uses
a local lower radicand bound in each block. A shared model-space spline
trims each pair of cylinders within the existing 0.0000001 mm STEP
uncertainty. The canonical reader reconstructs the analytic intersection
and verifies the entire exported graph. Existing successful uniform
files keep their paired parameter curves.

[Seventeen original Hole and STEP cases](evidence/plaincad-tight-partial-step-browser-2026-11-14.json)
and the [authored 2 mm offset case](evidence/plaincad-tight-partial-step-native-2026-11-14.json)
pass on the same WASM. Its 258,205-byte file fits the existing PlainCAD
import limit and the reopened project exports 1,294 watertight STL
triangles. OCCT agrees with the direct Boolean and independently
verifies the two native tighter-offset cases. Focused race and vet pass.

## Full PlainCAD provider rerun on kernel fa857e59 — 2026-11-14

The [pinned production build and complete run](evidence/plaincad-provider-kernel-fa857e59-2026-11-14.json)
pass 345 development, 66 production and 15 Chromium, Firefox and WebKit
cases with zero skips, failures, retries or flakes. Both TypeScript checks,
1,691 ordinary unit and component tests, four actual-WASM contracts, and
all 72 JavaScript bundle budgets pass. The development and production
benchmarks took 18.5 and 32.4 seconds and verified 25 edits and 11
STL downloads in each environment. The isolated PlainCAD branch records
the pin in `efae4d2` after independent internal review.

## PlainCAD two blind floors in an oblique Hole crossing — 2026-11-14

The cardinal and oblique tools each retain a circular floor when both
stops are beyond the entire equal-radius crossing. STEP recovers both
stops from the appended plane and cylinder supports, then checks the
entire regenerated boundary. Native tests deepen the cardinal cut
after import and compare its STEP to the corresponding fresh cut.

[Seventeen original Hole and STEP cases](evidence/plaincad-double-blind-oblique-step-browser-2026-11-14.json)
and the authored [lower](evidence/plaincad-double-blind-oblique-lower-step-native-2026-11-14.json)
and [upper](evidence/plaincad-double-blind-oblique-upper-step-native-2026-11-14.json)
oblique entries pass. Each imported part retains the two floors and
exports 396 watertight triangles. Independent OCCT checks both authored
files and 16 native two-floor combinations. Focused race and vet pass.

## PlainCAD blind floor on an oblique Hole — 2026-11-14

One oblique cutter may terminate inside the rectangular stock beyond
its complete equal-radius cardinal crossing. Its terminal face is an
exact circle normal to the tool. Both entry directions preserve the
elliptical stock opening and crossing edges. Import checks the full
STEP graph and preserves its represented floor station. The native
bridge can deepen the imported cut and reproduce the fresh STEP.

[Seventeen original Hole and STEP cases](evidence/plaincad-oblique-own-floor-step-browser-2026-11-14.json)
and the authored [lower](evidence/plaincad-oblique-own-floor-lower-step-native-2026-11-14.json)
and [upper](evidence/plaincad-oblique-own-floor-upper-step-native-2026-11-14.json)
entry cases pass in the browser. The exports retain 12 exact ellipses
and read back to one solid with 400 watertight STL triangles. Independent
OCCT confirms eight native signed cases and both authored exports.
Focused race and vet checks pass.

## PlainCAD concealed cutters in grouped oblique Hole crossings — 2026-11-14

Grouped cuts retain a cardinal and an oblique cutter after proving
the other tools share one of those lines and remain covered by a
qualified through or blind span. The canonical STEP boundary omits
the redundant tools. Native checks cover blind and through cardinal
spans, reversed oblique tools, input order, repeated tools and rejection
of off-axis tools.

[Seventeen original Hole and STEP cases](evidence/plaincad-concealed-oblique-step-browser-2026-11-14.json)
and the [supplemental browser WASM and PlainCAD import case](evidence/plaincad-concealed-oblique-step-native-2026-11-14.json)
pass. The grouped and visible-only STEP are byte-identical and the
imported stock reports zero overlap with the concealed tool. The
42,261-byte STEP matches the independently measured two-tool Boolean.
Readback exports 400 watertight STL triangles. The earlier side-opening
concealed STEP regression also passes with the OCCT oracle enabled.

## PlainCAD cardinal blind floors on oblique Hole crossings — 2026-11-14

For equal-radius crossings, the cardinal Hole can stop beyond the
entire intersection. Shortening its far wall leaves an exact circular
floor, preserves all crossing ellipses, and restores the uncut far
stock face. Import recovers the entry side and depth and verifies the
complete STEP graph. The imported shape can deepen the floor or open
it with an equal-diameter coaxial Hole. The resulting STEP matches
the corresponding fresh cut byte for byte.

[Seventeen original Hole and STEP cases](evidence/plaincad-blind-oblique-step-browser-2026-11-14.json)
and the authored [12:5](evidence/plaincad-blind-oblique-shallow-step-native-2026-11-14.json)
and [5:12](evidence/plaincad-blind-oblique-steep-step-native-2026-11-14.json)
fixtures pass. Each authored STEP preserves the 48 mm blind Hole,
reopens as one solid, and exports 400 watertight triangles. Independent
OCCT checks both authored files and 16 native signed entry cases.
Focused race and vet checks pass.

## PlainCAD 5–12–13 oblique Hole crossings — 2026-11-14

The equal-radius cardinal/oblique intersection retains its exact
ellipses for local ratios 12:5 and 5:12. The analytic planar reference
area accounts for the same projected opening used in STEP. Sixteen
new signed coordinate placements pass canonical re-export and retained
face area checks. Four new signed STEP placements pass OCCT readback.

[Seventeen original Hole and STEP cases](evidence/plaincad-pythagorean-oblique-step-browser-2026-11-14.json)
and the authored [12:5](evidence/plaincad-pythagorean-oblique-shallow-step-native-2026-11-14.json)
and [5:12](evidence/plaincad-pythagorean-oblique-steep-step-native-2026-11-14.json)
fixtures pass on the same WASM. Both exports contain 16 exact ellipses
and reopen in PlainCAD. Their 404-triangle STLs are watertight. OCCT
STEP readback and independent Boolean agree within 0.000001 mm³ and
0.000001 mm². Focused native, race and vet checks pass.

## Integration native short suite on 981161dd — 2026-11-14

The [native aggregate](evidence/plainsolid-native-981161dd-short-2026-11-14.json)
completed with no failures within a 60-minute package budget. The
trimmed package completed in 1,714.188 seconds. Go reported 3,893 passing
and 197 skipped test and subtest events. Four packages contain no tests.
This run closes the earlier aggregate timeout at this integration checkpoint.

## PlainCAD cardinal/oblique Hole crossings on placed axes — 2026-11-14

The bridge chooses a signed coordinate chart from the authored cardinal
and oblique tool axes. The existing 4:3 and 3:4 ellipse proof applies
locally. STEP reconstructs the stock chart from the two cylinder supports
and the six stock faces, then verifies the entire graph. Sixteen native
placements cover X and Y cardinal axes, both transverse axes, and both
slope signs. Existing Z-axis cases remain covered.

[Seventeen original Hole and STEP cases](evidence/plaincad-placed-oblique-crossing-step-browser-2026-11-14.json)
and the [authored YZ stock workflow](evidence/plaincad-placed-oblique-crossing-step-native-2026-11-14.json)
pass. Its 42,187-byte STEP has 16 exact ellipses and reimports after a
project reopen. The 404-triangle STL is watertight. OCCT readback agrees
with the independent Boolean measures under the same coordinate cycle.
Focused race, vet, and the existing OCCT exchange cases pass.

## PlainCAD near-floor skew Hole tessellation — 2026-11-14

A narrower valid gap between a skew intersection and its retained floor
can use conforming refinement without the costly diagonal flips and
final cleanup passes. It keeps the original fallback, deflection and
orientation checks, and validates the finished mesh as one solid.

[Seventeen original Hole and STEP cases](evidence/plaincad-close-floor-mesh-step-browser-2026-11-14.json)
and the [previously timing-out authored case](evidence/plaincad-close-floor-mesh-step-native-2026-11-14.json)
pass on the same WASM. The authored case rebuilds within the unchanged
8-second worker deadline and round-trips STEP, project reopen, and
watertight STL. Independent OCCT agrees with its direct Boolean on
volume, area, and centroid. Focused native, race, and vet checks pass.

## PlainCAD close blind floors on skew Hole crossings — 2026-11-14

A finite skew Hole checks the squared axial and transverse clearance
with exact rational arithmetic. A floor can stop beyond the actual
intersection while lying inside the full radius of the other cutter.
Native tests cover both entry caps and signed offsets and reject floors
that intersect the other bore.

[Seventeen original Hole and STEP cases](evidence/plaincad-partial-near-floor-step-browser-2026-11-14.json)
and the [authored closer-floor case](evidence/plaincad-partial-near-floor-step-native-2026-11-14.json)
pass. The 224,892-byte STEP and 1,220-triangle watertight STL retain
the through passage and a floor at Z=16.25 mm. The earlier full-radius
bound was Z=15 mm. Independent OCCT STEP readback agrees with its direct
Boolean. Five native OCCT STEP examples, including two closer floors,
pass. The 5 mm offset and 5/2 mm radii case with a 24.5 mm blind depth
passes native STEP and times out in the browser rebuild worker. Focused
race and Go vet checks pass.

## PlainCAD inch socket-head fastener STEP exchange — 2026-11-14

The original 4-40 UNC blank with a 0.5-inch shaft completes the same
[authored STEP and STL workflow](evidence/plaincad-inch-fastener-step-native-2026-11-14.json)
on the previously qualified WASM. Its 42,036-byte STEP reexports
byte-identically after project reopen. The 276-triangle STL is
watertight. Independent OCCT agrees with the authored inch geometry
to better than 0.0000001 mm³ and 0.0000001 mm².

## PlainCAD stepped shaft and hex socket STEP exchange — 2026-11-14

The native shoulder, shaft and head cylinders, and blind polygonal
socket are exchanged together. Import derives the stock dimensions,
socket wire, and floor and checks the complete regenerated graph.
Native signed placement and centimeter normalization pass.

[Six original fastener, Thread, Revolve and STEP tests plus provider preflight](evidence/plaincad-socket-fastener-step-browser-2026-11-14.json)
and the [authored M3 fastener STEP and STL extension](evidence/plaincad-socket-fastener-step-native-2026-11-14.json)
pass. The 39,536-byte STEP reexports byte-identically after import and
project reopen. The imported STL has 276 watertight triangles.
Independent OCCT verifies the authored file and native shaft examples
with and without the socket. Focused race checks and Go vet pass.

## PlainCAD equal-setback circular rim chamfer STEP exchange — 2026-11-14

A complete station-loft proof validates the exact ruled frustum bands,
cylindrical middle, and shared rims. Import requires the axial and
radial setbacks to agree and reconstructs the original native chamfer
geometry. Both rims, individual rims, and all six signed coordinate
directions pass native checks.

[Eight original edge and STEP regressions plus provider preflight](evidence/plaincad-rim-chamfer-step-browser-2026-11-14.json)
pass. The authored [XY](evidence/plaincad-rim-chamfer-xy-step-native-2026-11-14.json),
[XZ](evidence/plaincad-rim-chamfer-xz-step-native-2026-11-14.json) and
[YZ](evidence/plaincad-rim-chamfer-yz-step-native-2026-11-14.json)
cases reexport byte-identically after project reopen and generate
watertight 3,968-triangle STLs. Independent OCCT validates each authored
file and three translated dual-rim native files. Focused race tests and
Go vet pass.

## PlainCAD circular rim fillets on XZ and YZ — 2026-11-14

Coordinate placement cycles the three model-space coordinate lexemes
while retaining the 2D pcurves, knots, rational weights and shared
topology. Import reverses that placement before running the existing
whole-surface and graph proof. Native checks cover one or two rim
fillets on cylinders along all six signed coordinate directions.

[Seven original edge and STEP regressions plus provider preflight](evidence/plaincad-placed-rim-fillet-step-browser-2026-11-14.json)
pass. The authored [XZ](evidence/plaincad-placed-rim-fillet-xz-step-native-2026-11-14.json)
and [YZ](evidence/plaincad-placed-rim-fillet-yz-step-native-2026-11-14.json)
workflows both reexport byte-identically after project reopen and
generate watertight 7,040-triangle STLs. Three placed native OCCT
round-trips, independent readback of both authored files, focused
race checks and Go vet pass.

## PlainCAD circular rim fillet STEP exchange — 2026-11-14

A cylinder about signed world Z retains the one- or two-rim recipe and
exchanges the exact rational quarter-torus bands. The strict wrapped
reader validates the shared rims and surface charts before the bridge
reconstructs its original analytic fillet and display patches.

[Seven original edge and STEP regressions plus provider preflight](evidence/plaincad-rim-fillet-step-browser-2026-11-14.json)
and the [extended original XY grouped circular perimeter workflow](evidence/plaincad-rim-fillet-step-native-2026-11-14.json)
pass on the same WASM. Its 11,584-byte STEP reexports byte-identically
after project reopen. Its 7,040-triangle STL is watertight. OCCT
volume agrees within 0.000001 mm³ and area within 0.000001 mm².
Existing independent wrapped fillet tests, focused race checks and Go
vet pass. Rim exchange on the other working planes remains open.

## PlainCAD full coaxially bored Revolve STEP exchange — 2026-11-14

The complete tube uses two periodic cylindrical faces and two annular
plane caps. The strict analytic reader restores the concentric native
recipe before the generic disk-hole reader. An independently authored
tube and centimeter source units pass. Widening the bore after import
gives the same STEP as cutting a fresh cylinder.

[Seven original Revolve, Hole, outline and STEP cases plus provider preflight](evidence/plaincad-full-tube-step-browser-2026-11-14.json)
and the [authored full-sweep Edit Revolve extension](evidence/plaincad-full-tube-step-native-2026-11-14.json)
pass on the same WASM. The 6,670-byte STEP reexports byte-identically
after project reopen. The 8,192-triangle STL is watertight. OCCT agrees
with analytic volume and area. The existing native and independent tube
exchange tests, focused race checks, and Go vet pass.

## PlainCAD cylindrical Revolve sector STEP exchange — 2026-11-14

Simple solid cylindrical sectors use the existing five-face analytic
STEP format. The bridge recognizes the validated sector returned by the
strict reader and retains the partial angle for subsequent coaxial cuts.
A negative sweep past the half-turn, an independent STEP fixture, and
centimeter source normalization pass native round-trip checks.

[Seven original Revolve and STEP cases plus provider preflight](evidence/plaincad-cylinder-sector-step-browser-2026-11-14.json)
and the [authored solid quarter sweep](evidence/plaincad-cylinder-sector-step-native-2026-11-14.json)
pass on the same WASM. The 9,144-byte STEP is byte-identical after
import and project reopen. Its 2,528-triangle STL has two uses per edge.
OCCT agrees with the authored volume and area, and the existing
independent sector fixtures, malformed-graph tests, race checks and vet pass.

## PlainCAD fragmented arc-only lens STEP exchange — 2026-11-14

Adjacent fragments with the same exact circle center and radius are
joined only along their shared anchored seam. The signed sweep is
preserved. PlainCAD keeps all four authored sketch arcs and their
references through edit and project reopen.

[Eleven original sketch and STEP regressions plus provider preflight](evidence/plaincad-arc-only-lens-step-browser-2026-11-14.json)
and the [extended arc-only case](evidence/plaincad-arc-only-lens-step-native-2026-11-14.json)
pass on the same WASM. Before the change, STEP generation left the
download disabled. Its 17,602-byte file equals the whole-circle lens
exchange byte for byte. The reimported STL has 348 watertight triangles.
OCCT readback, the existing independent lens corpus, race checks and Go
vet pass.

## PlainCAD Edit Revolve quarter-bore STEP exchange — 2026-11-14

The coaxially bored quarter sweep retains two opposed cylindrical
supports, two planar annular-sector caps and two radial walls. Import
checks the full regenerated STEP graph and restores the native annular
sector. A wider coaxial cut after import gives the same STEP as a
fresh cut. Signed coordinate placements and centimeter source units
are covered by six independent OCCT checks.

[Six original Revolve and STEP cases plus provider preflight](evidence/plaincad-quarter-annular-step-browser-2026-11-14.json)
and the [extended Edit Revolve case](evidence/plaincad-quarter-annular-step-native-2026-11-14.json)
pass on the same WASM. The 11,556-byte STEP reexports byte-identically
after project reopen. The 3,072-triangle STL has two uses per edge.
Independent OCCT agrees with the analytic volume and surface area.
Focused race and vet checks pass.

## PlainCAD construction-axis Revolve STEP exchange — 2026-11-14

Circular Revolve retains the recipe for a complete ring torus or bounded
elbow. The STEP path uses the existing toroidal writer and verifies the
face and seam graph on import before rebuilding the native display and
fabrication patches. Signed sweeps, rigid reflection, non-millimeter
units and an independently authored elbow pass the bridge checks.

[Five original Revolve and STEP cases plus provider preflight](evidence/plaincad-torus-sweep-step-browser-2026-11-14.json)
pass on the same WASM as the [authored construction-axis round-trip](evidence/plaincad-torus-sweep-step-native-2026-11-14.json).
The 3,990-byte quarter-torus STEP reexports byte-identically after
import and project reopen. Its STL has 24,448 triangles and a
two-use edge count throughout. OCCT agrees with the analytic volume
and area. The existing independent ring and elbow exchange checks,
focused race checks and Go vet pass.

## PlainCAD circular lens STEP exchange — 2026-11-14

Exact disk and circular-arc intersections extrude two cylindrical walls
with shared axial seams and planar caps. The public importer checks the
represented endpoints and seam angles on both supports and regenerates
the complete STEP graph. The qualified profile has two convex minor arcs
in a signed coordinate frame.

[Eight original sketch and STEP cases plus provider preflight](evidence/plaincad-circular-lens-step-browser-2026-11-14.json)
pass with the same WASM as the authored [equal-circle](evidence/plaincad-circular-lens-equal-step-native-2026-11-14.json)
and [unequal circle/arc](evidence/plaincad-circular-lens-unequal-step-native-2026-11-14.json)
extensions. Both extensions export STEP, import, save and reopen, and
export watertight STL. The equal lens writes 17,602 bytes and 356 STL
triangles. The unequal lens writes 17,404 bytes and 308 STL triangles.
Independent OCCT agrees with the analytic volume and area of the two
app files and thirteen signed-placement native cases. Focused race and
vet checks pass.

## PlainCAD driven arc-and-chord STEP exchange — 2026-11-14

STEP for a bounded circular segment recovers the represented angles from
the cylinder's axial pcurve lines. The reader checks each intermediate
seam and the full regenerated graph. Qualified minor and major segments
retain the exact circle and closing chord. The graph rejects modified
seam values and intervals that cross the supported angular branch.

[Nine original sketch and STEP cases](evidence/plaincad-circular-segment-step-browser-2026-11-14.json)
and an [extension of the original driven arc case](evidence/plaincad-circular-segment-step-native-2026-11-14.json)
pass on the same WASM. Before the fix, the authored case reached STEP
generation and the download remained disabled. The 13,169-byte STEP
now exports, reimports and reopens after radius and offset edits. Its
264-triangle STL is watertight. Eighteen independent OCCT minor and major
segment comparisons, the original solver fixture, race checks, and Go
vet pass.

## PlainCAD Circle Trim half-disk STEP exchange — 2026-11-14

The exact half-disk produced by the original XY Circle Trim workflow
retains its two circular rim arcs and planar diameter face through STEP.
The importer verifies the complete native graph before restoring the
analytic extrusion. Signed height, reflected section placement and a
representable non-millimeter source pass native bridge checks.

[Seven original curved-sketch and STEP cases](evidence/plaincad-semicircle-step-browser-2026-11-14.json)
and an [extension of the original Circle Trim workflow](evidence/plaincad-semicircle-step-native-2026-11-14.json)
pass on the same WASM. The 9,185-byte STEP reimports and reopens. The
imported STL has 256 watertight triangles. Twelve independent OCCT
placements, authored STEP measures, malformed-graph rejection,
focused race tests and Go vet pass.

## PlainCAD Arc Extend quarter-sector STEP exchange — 2026-11-14

The exact quarter-sector produced by the original YZ Arc Extend workflow
round-trips its analytic cylindrical wall and circular edges. The retained
recipe supports signed cardinal placement, clockwise authored order and
negative extrusion. The importer verifies the complete regenerated graph.

[Seven original curved-sketch and STEP cases](evidence/plaincad-quarter-sector-step-browser-2026-11-14.json)
and an [extension of the original Arc Extend workflow](evidence/plaincad-quarter-sector-step-native-2026-11-14.json)
pass on the same WASM. The 8,992-byte STEP reimports and reopens after
the authored edit and Undo/Redo. The imported STL has 132 watertight
triangles. Twenty-four independent OCCT placements, the authored STEP
measurements, malformed graph rejection, focused race and Go vet pass.
The representable non-millimeter bridge fixture also passes.

## PlainCAD multiple finite floors in a three-Hole crossing — 2026-11-14

Two or three concurrent cardinal cutters can retain their complete blind
floors beyond the overlap. Both entry directions remain available on
each axis. STEP recovers floor order and axes from the missing stock
openings and floor planes. Prior through and single-floor exports are
byte-identical.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-multi-floor-triple-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-multi-floor-triple-step-native-2026-11-14.json)
pass on the same WASM. The 115,577-byte STEP reimports and reopens.
Ray checks identify all three retained floor positions in a watertight
588-triangle STL. Thirty-two independent OCCT STEP comparisons, four
translated decimal-depth round-trips, grouped and successive native
cuts, focused race tests, and Go vet pass.

The [recut follow-up](evidence/plaincad-multi-floor-recut-2026-11-14.json)
reopens that STEP in PlainCAD, extends the X blind Hole from 30.5 mm to
34 mm, and exports the new STEP. The 588-triangle STL retains the new
and both unmodified floors. Native coverage also opens X through the
opposite cap and compares STEP byte-for-byte with a fresh cut. Independent
OCCT measures and the focused race tests pass.

## Full PlainCAD provider rerun on kernel e60f34e7 — 2026-11-14

The isolated app provider at `9779044` passes 345 development, 66
production and 15 Chromium, Firefox and WebKit cases with zero browser
skips, retries or flaky cases. All 1,691 ordinary unit and component
cases and four selected actual-WASM contracts pass. Both TypeScript
checks and the bundle gate pass. The largest of 72 JavaScript bundles
measures 496.50 kB and the built WASM hash matches the source manifest.

The performance case validates all 25 edits and 11 STL exports in each
environment. The development and production cases finish in 25.188 and
85.975 seconds while validation shares the test host. The unchanged
production case passes in 32.1 seconds after native and unit tests finish.
The separate native `brep/trimmed` aggregate hits its 35-minute budget.
The in-flight multispan elliptical contact test passes individually in
0.60 seconds. The native aggregate result remains open.

[Reports and build hashes](evidence/plaincad-provider-kernel-e60f34e7-2026-11-14.json).

## Full PlainCAD provider rerun on kernel 65441a2d — 2026-11-14

The selected PlainSolid provider at PlainCAD app revision `9779044`
passes 345 development, 66 production and 15 cross-browser cases with
zero skips, retries and flaky results. The full development run takes
36.4 minutes. Development and production performance cases take 17.6
and 20.3 seconds. Both verify 25 edits and 11 STL downloads.

Both TypeScript checks, four actual-WASM contracts, and the 72-bundle
production budget pass. The largest JavaScript bundle is 496.50 kB of
the 500 kB limit. The built WASM matches the SHA-256 in the kernel
manifest. All 1,691 ordinary unit and component cases pass with one
worker, up to two retries and a 15-second timeout. The release stages
were executed individually on the same pin. Later geometry milestones
have focused browser and exchange coverage.
[Reports and build hashes](evidence/plaincad-provider-kernel-65441a2d-2026-11-14.json).

## PlainCAD reciprocal cardinal–oblique Hole crossing — 2026-11-14

The exact equal-radius crossing also accepts a 3:4 local axis ratio when
both elliptical openings stay on opposite X faces. The conic coefficients
are checked across their full intervals. The existing 4:3 STEP fixture
remains byte-identical. Separated same-ratio bores on the Z caps continue
through the disjoint path.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-reciprocal-oblique-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-reciprocal-oblique-step-native-2026-11-14.json)
pass on the same WASM. The 42,187-byte file contains 16 analytic ellipses
and reopens through STEP. The STL has 404 watertight triangles. Both
signed slopes, grouped and successive cuts, covered tool overlap, six
independent OCCT STEP checks, and the authored direct Boolean pass.
Focused race and Go vet pass.

## PlainCAD bounded oblique Hole tessellation — 2026-11-14

Paired elliptical wall rims share angular samples. The analytic gap
bound qualifies every generator between the rims, including cases with
both sine and cosine coefficients. The existing model-edge deflection
and facet normal checks continue to apply.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-compact-oblique-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-compact-oblique-step-native-2026-11-14.json)
pass on the same WASM. The 2:3 model reopens through its 29,656-byte exact
STEP, matches independent OCCT measures, and exports watertight
276-triangle STL. Four native directions, including a three-component
axis and reversed entry, pass float32 closure and remain below a
400-triangle budget. An interior-crossing trim rejects even with
positive endpoint gaps. Focused race and Go vet pass.

## PlainCAD separate cardinal and oblique through Holes — 2026-11-14

Two or three through cutters with separated wall bounds retain their
complete circular and elliptical openings in one rectangular stock
body. Grouped and successive cuts keep exact measures. Canonical STEP
recovers the individual cylinder axes and stock cap ownership.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-disjoint-oblique-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-disjoint-oblique-step-native-2026-11-14.json)
pass on the same WASM. The 29,347-byte STEP reimports, reopens and
matches independent OCCT Boolean volume, area and centroid. Watertight
STL has 1,372 triangles. Five canonical STEP variants, native race tests
and Go vet pass. Steeper or three-component axes can exhaust adaptive
tessellation at the existing mesh budget.

## PlainCAD exact cardinal–oblique Hole crossing — 2026-11-14

A cardinal cylinder and a concurrent equal-radius oblique cylinder at a
4:3 local axis ratio retain analytic ellipse arcs on the two angle
bisectors. Strict stock bounds keep both circular and both elliptical
openings interior. Bounded coefficient checks qualify all points of the
shared conics. STEP stores the exact ellipses and cylinder supports.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-crossed-oblique-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-crossed-oblique-step-native-2026-11-14.json)
pass on the same WASM. The project uses a guide face to set the oblique
axis, exports the machined body, reimports, reopens and exports
watertight STL. Twelve signed native cases, three independent OCCT STEP
comparisons and the authored direct Boolean pass. Focused race, Go vet
and the corrected existing adjacent-opening native regression pass.

## PlainCAD unequal crossed Holes with disjoint companions — 2026-11-14

An unequal-radius centered or covered-width skew pair accepts one or two
separate cardinal through bores. Complete projected envelopes prove
clearance. The original radical trims, stock faces and finite floors
keep their indices. STEP appends canonical companion wall groups and
retains the bounded compact fit at the crossing.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-skew-companion-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-skew-companion-step-native-2026-11-14.json)
pass on the same WASM. The 454,657-byte authored STEP reimports and
reopens, and its watertight STL verifies both finite floors and the
separate passage. Thirty-two native mass and ownership cases, eight
float32 mesh checks and three independent OCCT STEP imports pass.
Focused race and Go vet pass. Other placements can exhaust the existing
adaptive triangle budget.

## PlainCAD finite floor in a three-Hole intersection — 2026-11-14

One cutter in a concurrent equal-radius three-axis group can stop beyond
the complete intersection. Its full planar floor retains its eight
circular rim arcs and face reference. All saddle arcs and triple vertices
stay exact. The native bridge accepts either side of any cardinal axis.
The through-only STEP graph remains byte-identical.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-blind-triple-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-blind-triple-step-native-2026-11-14.json)
pass on the same WASM. The authored STEP reimports and reopens, and
watertight STL retains both open passages and the blind floor. Twenty-four
independent OCCT finite STEP checks and an authored direct Boolean pass.
Native, exchange and bridge race tests and Go vet pass.

## PlainCAD finite blind floors on partial-skew Hole crossings — 2026-11-14

A partial transverse overlap can retain one or both finite floor planes
beyond the complete crossing. The shared radical-boundary constructor
stops the scaled cylinder caps without changing the original trim loop.
Canonical STEP records the circular floor and its entry direction, then
regenerates the compact spline graph on import.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-partial-finite-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-partial-finite-step-native-2026-11-14.json)
pass on the same WASM. The authored 367,577-byte STEP preserves the full
through passage and a 10 mm floor in watertight STL. Seventy-two native
mass cases, six closed meshes and three independent OCCT STEP cases
cover different radii, signed offsets and entry sides. Direct OCCT
Boolean measures agree with the authored STEP readback. Focused exchange
and bridge race tests and Go vet pass.

## PlainCAD compact STEP for unequal finite crossings — 2026-11-14

Finite crossed Hole STEP uses checked interior pole rounding and join
compaction within the existing 0.0000001 mm declared uncertainty. The
same interpolation and resource guards apply. Previous canonical
through STEP bytes remain identical. The reader also verifies and
accepts the earlier complete finite graph.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-unequal-compact-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-unequal-compact-step-native-2026-11-14.json)
pass on the same WASM. The 3 mm and 2 mm radius model exports in
412,000 bytes, reimports below the 512 KiB app limit, reopens and retains
its floor in watertight STL. Nine independent OCCT STEP comparisons
cover signed centered and covered-skew stops. The largest sampled model
curve deviation was 0.000000002429 mm. Legacy readback, race tests and
Go vet pass.

## PlainCAD three intersecting cardinal through Holes — 2026-11-14

Equal-radius X, Y and Z cutters with one common crossing retain the
original six stock faces and eight three-surface junction vertices.
The exact intersection arcs are planar ellipses. Signed native charts
retain the same geometry, and canonical STEP verifies every surface,
edge and orientation on readback.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-triple-crossed-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-triple-crossed-step-native-2026-11-14.json)
pass on the same WASM. The authored browser case reimports and reopens
the STEP and finds all three open passages in watertight STL. Native
signed vertex links, twelve independent OCCT STEP comparisons, and an
independent authored Boolean pass. Focused race and Go vet pass.

## PlainCAD crossed Holes with disjoint companion bores — 2026-11-14

An equal-radius perpendicular through pair can retain additional cardinal
through bores. The padded cutter envelopes prove each companion is
separate from both crossing cylinders and from other companions. Native
sewing preserves the original stock faces and saddle edge identities.
The canonical STEP graph orders the companion wall groups by their
geometric sites and recovers all exact holes on reimport.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-crossed-companion-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-crossed-companion-step-native-2026-11-14.json)
pass on the same WASM. The authored model reimports and reopens the
55,240-byte STEP and verifies all three clear passages in watertight
STL. Native signed poses, a 16-site limit case, and four independent
OCCT STEP imports pass. Focused race and Go vet pass.

## PlainCAD unequal-radius perpendicular blind Holes — 2026-11-14

Either unequal cutter can stop beyond the full cylinder intersection.
The exact original cylinder supports and radical trim curves remain. The
native constructor moves only the cap rim and straight axial edges, adds
a planar floor, and retains the other face indices. STEP recovers the
represented floor supports and uses the existing bounded spline fit at
the cylinder intersection.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-unequal-finite-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-unequal-finite-step-native-2026-11-14.json)
pass on the same WASM. The 480,631-byte authored STEP reimports in PlainCAD,
reopens and preserves the blind floor in watertight STL. One hundred
forty-four native combinations and nine independent OCCT STEP comparisons
include signed placements, both cutter assignments and nested skew.
The authored direct Boolean agrees with STEP readback. Focused race and
Go vet pass. The app's 512 KiB limit still excludes some close-radius
and skew exported files.

## PlainCAD exact far-tangent blind Hole floor — 2026-11-14

The blind floor can stop at the far tangent plane of a perpendicular
equal-radius through Hole. Its complete circle remains, and two saddle
vertices touch the floor rim. The collapsed cylinder strips preserve a
single cyclic boundary link at every retained vertex. A floor at the near
tangent plane has a nonmanifold contact diameter and is rejected.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-far-crossed-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-far-crossed-step-native-2026-11-14.json)
pass on the same WASM. The authored model reimports and reopens its
exact conic STEP and checks the floor in watertight STL. Six signed
vertex-link cases and three independent OCCT STEP comparisons pass.
The direct authored Boolean agrees with STEP readback. Focused race
tests and Go vet pass.

## PlainCAD disjoint finite Hole cylinders with overlapping bounds — 2026-11-14

The independent finite-hole path can prove strict clearance when padded
cylinder boxes overlap. The proof expands the finite axial intervals and
radii, then compares squared distances between the surviving radial
sections using exact rational arithmetic. The geometry and exchange
format retain the existing analytic walls, opening arcs and blind floors.
Explicit tangent and insufficient-clearance fixtures still reject.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-disjoint-radial-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-disjoint-radial-step-native-2026-11-14.json)
pass on the same WASM. The authored project reimports STEP, reopens,
and checks both retained floors in watertight STL. Twelve independent
OCCT STEP comparisons and prior byte-preservation checks pass.

## PlainCAD disjoint blind Holes with crossing projections — 2026-11-14

Two or more independent finite cutters can have intersecting through-axis
projections. The kernel stops each blind donor first, proves separation
of actual padded finite bounds, and transfers the retained analytic walls,
openings and floors. Contact and overlap fail the independent-boundary
proof. Earlier disjoint-through and blind STEP graphs retain their bytes.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-disjoint-finite-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-disjoint-finite-step-native-2026-11-14.json)
pass on the same WASM. The authored model preserves both floor locations
in STEP reimport and watertight STL. Twelve independent signed and mixed
OCCT STEP checks pass. Focused race and Go vet pass.

## PlainCAD blind stops before and at a perpendicular crossing — 2026-11-14

A blind floor inside the near half of the crossing retains two disconnected
circular segments. A stop exactly through the crossing axis consumes the
planar floor and retains the analytic cylinder intersection. A shared stock
and cap-wire builder preserves the previous far-half STEP bytes.

[Seventeen original PlainCAD Hole and STEP scenarios](evidence/plaincad-before-crossed-step-browser-2026-11-14.json)
pass on the same WASM as the supplemental
[near-half stop](evidence/plaincad-before-crossed-step-native-2026-11-14.json)
and [exact-axis stop](evidence/plaincad-center-crossed-step-native-2026-11-14.json).
Both authored projects export analytic STEP, reimport, reopen and produce
watertight STL. Thirty new signed native checks and six independent OCCT
STEP comparisons cover these cases. The two authored models also match
direct OCCT Booleans in volume, area and centroid. The focused race checks
and Go vet pass.

## PlainCAD STEP for disjoint mixed-axis blind Holes — 2026-11-14

Canonical STEP retains up to 32 independent interior bores on two or three
cardinal axes. Blind bores keep their entry sides and exact planar floors.
This increment qualifies tools whose conservative full-through bounds are
separated. The existing through-only STEP bytes are unchanged. Grouped and
successive cuts, scaled STEP import, retained cutter overlap and meshes pass.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-disjoint-blind-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-disjoint-blind-step-native-2026-11-14.json)
pass on the same WASM. The authored project reimports and reopens STEP,
then extracts both blind floor locations from watertight STL. Twelve
independent OCCT STEP imports pass on signed, mixed and wholly blind
fixtures. The focused native race tests and Go vet pass.

## PlainCAD STEP for a split blind Hole floor — 2026-11-14

An equal-radius perpendicular through Hole splits a blind floor into
two circular segments when the stop passes the crossing axis and
remains within the through cylinder. Both segments retain their
planar supports and exact circle arcs. The intersection edges retain
analytic ellipse arcs. A plane-only face query detects the disconnected
floor reference as ambiguous.

[Seventeen original PlainCAD Hole and STEP cases](evidence/plaincad-clipped-crossed-step-browser-2026-11-14.json)
and a [supplemental authored project](evidence/plaincad-clipped-crossed-step-native-2026-11-14.json)
pass on the same WASM. The authored project reimports the 69,597-byte
STEP, reopens, and checks both floor segments and the through passage
in a watertight STL. Twenty-four signed native cases and three
independent OCCT STEP imports pass. The authored OCCT Boolean and
OCCT STEP readback agree within 0.00000004 in volume and area.
Focused race tests and Go vet pass.

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

The original three-contour radial Common was reconstructed from all 228
source proposals in a CGO-free run (877 seconds). Its four native faces
form two material components with two inner wires. The exact total volume
divided by π is `63333935546875/864691128455135232`, centroid Z is
`106610222140625/232841378390867968`, and the second Z moment divided by π
is `8952130846239958703517913818359375/401413413573551216547642928907670507107647488`.
The [independent original OCCT model](../tools/occtref/TRIPLE_RADIAL_ROOFS.md)
verifies the component topology and volume within `1e-10`.

Direct volume bounds over the original XY footprints contain the independent
OCCT Common, Fuse, and Cut volumes for the crossing-roof shells. Each
enclosure is at most `1/1000` cubic model units wide after exactly integrating
the cells with proved polynomial thickness. The weighted cavity
contains its independent logarithmic integral within `1/100`, preserves
the bounds under a proper rigid placement, and returns a conservative
enclosure when refinement is limited. Its `27/64` Common is exact, and the
separately sewn weighted OCCT Common, Fuse, and Cut all agree. Cancellation
returns no bounds.

The direct column mass proof also encloses all three crossing-roof OCCT
centroids and full origin inertia tensors, including negative coordinates.
The weighted Common has exact volume, centroid and all six independent
inertia coefficients from its polynomial tool. The weighted Cut bounds
tighten under refinement and contain the independent OCCT mass properties.
The same raw second moments transport through a proper rigid occurrence.
Cell and arithmetic limits and cancellation return no partial tensor.

The original creased roof-box cavity preserves a single original roof ID
over two distinct spline spans. Exact line checks cover both directions,
clips at the crease, and a coincident run; the crest exposes two one-sided
normals. Separate OCCT stock halves and a buried cutter verify volume
`117/64`, centroid, and all inertia entries. Exact cuts at the original
knot and footprint edges give 12 polynomial cells and no unresolved
integration. Independent rational integration matches all ten native raw
moments exactly; numerical OCCT agrees within `1e-12`.

Strict nested roof boxes retain complete original faces. The direct area
proof preserves a 12-entry original face ledger for Common and Cut,
including zero for deleted faces and reversed orientation for the cutter
shell. It integrates the floor and wall domains in model coordinates and
bounds the graph area using rational quotient derivatives. The weighted
fixture agrees with the independent OCCT surface area, while a transverse
crossing rejects this whole-face proof. Rigid placement preserves the
rational enclosure; exhaustion and cancellation expose no partial ledger.

`BoundCrossingSurfaceArea` separately clips all twelve original faces
against the opposing graph box. It partitions at both footprints and all
genuine roof knots, integrates vertical strip heights, and bounds the
retained graph area with rational derivative and square-root enclosures.
The independent crossing Common, Fuse, and Cut surface areas are contained
in intervals of width at most 0.1 square units. Their lower/upper endpoints
are approximately 3.94383/4.04382, 8.43968/8.53968 and 8.00647/8.10647.
The Common floor has exact area 1. A separate weighted cavity check exercises
the quotient derivatives. The live OCCT and Go race checks passed. Source
floors and parallel wall planes must be distinct; the complete regular
Boolean proof excludes coincident boundary areas.

When separate height ranges overlap, the area, volume and raw mass proofs
can now compare the two roofs on a common exact chart. Subtracting the
homogeneous height products cancels shared terms before testing order.
A shifted source chart with a positive nonconstant weight selects the
correct original polynomial roof and integrates its exact volume
`57/32` and centroid in one cell. The original weighted Common likewise
uses one cell instead of three with identical moments. Live crossing,
weighted, and creased OCCT regressions passed under the race detector.

`BoundRoofBoxSurfaceArea` extends the direct measurement to independently
proved source boxes with shared boundary areas. Coincident floors and roofs
are counted once or canceled. Shared vertical strips use the material sides
to distinguish overlap from adjacent solids. Nine independent OCCT cube
Booleans cover same-side, opposite-side and identical material; all areas
match the Go intervals' exact endpoints. Identical rational roofs also
contribute only once. This query measures the regularized boundary without
constructing a native topology for the coincident operation.

The signed-axis spherical STEP hierarchy materializes as an editable native
document with one planar and three curved assets. Its seven material
occurrences retain their proved world poses and original relationship paths;
both sphere instances share the same local asset. STEP and source-bound native
imports produce the same document nodes and provenance labels. Reopening the
editable document is byte stable, and moving one sphere leaves the other
unchanged; Undo restores the original path placement.
Placement and name edits now re-export against the original STEP graph.
The moved sphere retains its product, local body and relationship numbers;
the independent OCCT readback sees seven solids, 22 faces and 88 oriented
edge uses, unchanged volume, and the expected one-unit shift in the left
extent. Editing an original shared torus relationship moves both expanded
paths. Its shared reference datum remains untouched and the writer adds
one private target frame. A single inconsistent edit to one path of that
relationship fails without writing any exchange data.
The editable snapshot also persists per-native-face labels referencing the
original STEP face entities. Both STEP and source-bound native import recover
these labels on the shared asset definitions rather than duplicating them
on each occurrence. The original transfer, document reopen and live OCCT
edit regression passed under the race detector.

The immutable rational hierarchy document keeps exact oblique occurrence
matrices separate from local geometry. Both convention fixtures retain seven
paths to four shared assets. The tests verify caller mutation isolation,
canonical source and byte-stable native round trips, separation of the
two native tags, exact world bounds, symbolic volume, and point and line
queries. The focused document race tests passed.
Exact relationship edits return a new proved document in either frame
convention. A shared cylinder relationship moved both expanded occurrences
while preserving product, body, face, and relationship IDs and retaining the
original child datum. Both edited transfers matched OCCT volume, body/face
counts, and world extents. Improper rotations and nonterminating STEP
translations were rejected before output.

Document planar face history now propagates authored box roles through
the same geometry evaluation that builds each planar Boolean. The
regeneration test covers fragment ownership after subtraction, cutter
face reversal, signed reflection, deletion after moving the cutter away,
and exact re-resolution after Undo and reopening. Unrelated and
unsupported dependencies return distinct errors. Existing document
transaction, sketch, label and STEP assembly race regressions passed.
Named planar box edges now retain exact source and output parameter
intervals across multiple Booleans. The directed interval maps compose
through each output edge and survive a reflected placement. The regression
verifies both ends against the original analytic segment, the surviving
source lengths `5/8` and `3/4` before and after an edit, and exact
re-resolution after Undo and reopening. The focused race tests passed.

The [newer full trimmed run](evidence/native-full-trimmed-b7e62101-2026-11-14.json)
accounted for all 796 top-level cases on clean `b7e62101`, with CGO
disabled: 783 passed and 13 skipped. Initial shard 0 reached its wall
limit while running the offset dual-disk mass case. That case passed in
isolation on the same archive, and four disjoint continuation shards
passed every remaining case that was runnable. The evidence preserves
the original timeout as well as the bounded continuations.

The [updated full-mode native run outside the trimmed package](evidence/native-full-nontrimmed-c7a5af46-2026-11-14.json)
passed 49 tested packages on a clean `c7a5af46` archive with CGO
disabled. Four packages have no tests. Go recorded 4,495 passing test
events and 386 skips, including opt-in comparators. The module list
contains only PlainSolid.

A [later count-one Go package summary](evidence/native-full-nontrimmed-c5067418-summary-2026-11-14.json)
from the working checkout based on `c5067418` also passed all 49 tested
packages outside `brep/trimmed` with CGO disabled. The four remaining
packages report no tests. Separate focused qualification covers the
subsequent changes; this summary does not record per-test events.

The [full-mode native run outside the trimmed package](evidence/native-full-nontrimmed-d6fe29e4-2026-11-14.json)
passed 49 tested packages on a clean `d6fe29e4` archive with CGO disabled;
three packages have no tests. Go reported 4,414 passing test events and
381 skips, including opt-in external comparators. The previously short-only
trimmed cases are tracked separately. The exact Go module list contains
only PlainSolid.
The [entire trimmed suite in full mode](evidence/native-full-trimmed-139c431-2026-11-14.json)
passed on a separate clean `139c431` archive with CGO disabled. Four
disjoint shards accounted for all 765 top-level tests: 752 passed and 13
skipped. Counting subtests, Go reported 886 passing events. The longest
shard took 2,218 seconds. The evidence names each skip and records the
command, selection hashes, and raw report hashes for all four shards.
The [selected trimmed full-mode run](evidence/native-full-selected-trimmed-2026-11-14.json)
passed all 14 top-level cases and their eight nested test events, including
the expensive nested exact annulus meshes and closed transverse Boolean
operations. It was a working-checkout diagnostic; the pinned entire trimmed
suite is recorded above.

The [current cross-browser reconciliation](evidence/plaincad-reconciled-56b4633-cross-browser-2026-11-14.json)
passed all 15 original cases on PlainCAD `50432ed` and clean review
revision `56b4633`: five each on Chromium, Firefox, and WebKit, with no
skips or retries. The verifier checked all 841 app source blobs and all
1,795 kernel and harness source blobs. The original Playwright config
was byte identical, and the separate backend preflight confirmed Go WASM
without OCCT requests.

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
The [CGO-disabled run at `6e7de12`](evidence/native-short-6e7de12-2026-11-14.json)
passed all 49 tested packages outside `brep/trimmed` with 4,335 passing
and 368 skipped events. All three trimmed shards passed, with 850 passing
and 23 skipped events (441, 597, and 563 seconds). These short-mode
counts include subtests; later focused results are recorded separately.
The [CGO-disabled run at `60ad6754`](evidence/native-short-60ad6754-2026-11-14.json)
passed all 49 tested packages outside `brep/trimmed` with 4,379 passing
and 373 skipped events. All three trimmed shards passed with 864 passing
and 23 skips (832, 786, and 514 seconds), for 5,243 passing test events
overall. This is short mode; full release qualification remains separate.
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
Algebraic cap section tests preserve two irrational roots on the same original
quadratic edge, exactly reverse their order with the cutting orientation,
and exclude the through-void gap when four contacts are present. Tangency
and insufficient cumulative work return no partial section. Constructive
cap clipping joins one retained parabolic interval to an exact restricted
line and handles the opposite side's two pieces on the original arc. The
holed fixture joins its two original rings into one notch with two seams.
Tests prove equality of every neighboring world-space endpoint across the
original and seam parameterizations and reject exhausted clipping without
publishing the earlier section. A stock-backed JSON reload of the sheared
tube at Y=11/8 independently confirms the X=√3 root on the original void
edge. It restores the same owner and exact joins after mutation of the
diagnostic report. Exact seam, void, inside and discarded points agree
before and after reload. Focused normal, race and Go 1.23 passed.
Simultaneous conical fillets (`105312d`) keep five native faces, unchanged
original end caps and circular bases, and exact original cone ownership.
Both slope directions and equal/unequal radii match independent volume and
area integrals and reload. Touching or overlapping runouts fail atomically.
The polynomial cap area proof measures Green integrals on exact
algebraic intervals with no chord sampling. Both signs of a parabolic cut
enclose 2√2/3 and its complement; after native reload, the sheared
through-hole clip encloses 14/3 + √3/4 at a width of 2⁻²⁴. Mutated
diagnostic wires cannot affect the area. Normal, race, vet and Go 1.23 pass.
The rational extension bounds the original weighted conic segment above
Y=1/2 in a circular quadrant. Its area encloses π/6 − √3/8 to width 2⁻²⁰;
reduced work fails without an interval. No weight is discarded.
The crossed-roof Boolean (`21fac2a`) independently proves coincidence on
original planar supports with nonlinear parameterization, assembles
intersection, union and difference, and retains topology and original
owners. Intersection reloads as an exact algebraic boundary. Broad
positive volume bounds enclose all three independent integrals; conversion
to an ordinary representable solid still rejects unsupported endpoints.
Mixed original charts now yield the full shared boundary (`1950d1b`):
98 exact edges and 602 certified segments at deflection 0.02. All 180,901
display segment pairs pass exact contact checks, and the filled curved
faces require a separate proof. Exact trapezoid chart inversion
(`59615993`) completes the original XY/64 wall/roof pair in both
operand orders; an omitted tube and a one-ULP false anchor reject.
That whole-shell fixture still stops at its final wall/wall pair.
The eccentric through-bore on this filleted stock (`c207731`) retains all
stock vertex, edge and face IDs, proves exact clearance, and matches
independent differences −6π in volume and +10π in area after reload.
The genuine OCCT STEP elliptical disk face (`e58145b`) retains its full model
and planar trim curves, verifies their original owners and matching
parameterizations, and native-reloads. Pinned OCCT confirms one face with
area 8π, centroid (1,2,3), and outward normal +Y on both source and re-export.
The genuine elliptical STEP annulus (`9015c45`) preserves its independent
inner curve and reversed bound; pinned OCCT verifies one face, two edges,
area 6π and the same oriented plane after native reload. Mixed bores
(`a28448c`) verify a through bore and two opposed blind bores with the same
radial centre and strictly separated floors. Their volume loss is 11π/4
and total area gain 21π/2; axial or radial contacts reject atomically.
Exact conical material queries (`eee0af8`) retain the native owners on both
toroidal bands and blind floors, and distinguish offsets of 2⁻¹⁰⁰ with
identical binary64 representations. The band display certificate verifies
two-sided deflection at 0.1 (512 and 4,096 triangles), exact original rim
parameters, and shared seam indices. A separate certified closed result
uses 2,112 triangles at deflection 0.2 on the unbored two-fillet frustum,
shares original rim indices, and proves no unintended triangle contacts.
The eccentric through-bore display (`238a439`) uses 2,160 triangles at
the same deflection. Its two annular caps and inward bore wall share
original rim indices and pass genus-one closure and global contact checks.
Mixed through/blind display (`a6fd2fb`) closes with 2,256 triangles,
original cap holes and two distinct blind floors. Exact oblique
queries (`5b2cd321`) separate retained toroidal quartic roots from the
inner branch and from trimmed support events under a cumulative budget.
Prepared material persistence (`d9d6bc15`) re-proves native owners and
those queries; changed depth/radius and stale source geometry fail.
Pinned OCCT native cone fillets and three cylinder cuts (`efd8d4b3`)
agree in solid count, genus, mass, centroid and 14 line contacts.
Placed conical queries (`52d8708e`) retain algebraic parameters and
native owners. The trim-aware closest-distance query (`a2f14ea8`) now
certifies squared distance 1/4 exactly at a through-hole entry rim and
returns both original owners.
The exact clipped planar cap mesh triangulates the original coplanar
boundary samples with the void open or intact. It proves chord ownership,
display ring topology, oriented coverage and a two-sided fill error.
The two-cap algebraic pairing fixture independently recovers both
seams of the sheared sleeve on opposing planar charts, including the
shared X=√3 junction. It proves opposite traversal and exact model
endpoint agreement and excludes the gap through the original void.
Normal, race and Go 1.23 pass. Independently composed stock coedges recover
the exact original UV restriction and prove equality by homogeneous
cross-multiplication over the whole shared domain. Reload rechecks the
same owner and domain. The stock loops close into one oriented shell
(`709144a`), 24 vertices, 36 edges and 14 faces. The two-plane native
recipe (`5739ea0`) preserves the original root IDs and material queries.
Exact flux encloses 697/960 − 3√3/640 to width 2⁻³⁰. Pinned OCCT
independently gives 0.71792267910286089, within 6×10⁻¹⁰ of the analytic
value, and Euler characteristic 2 on its coarser face partition.
Every indexed model edge has an exact original domain (`ffe0921`),
and one shared chord network (`15f7044`) feeds a closed planar or
affine-projected curved fill (`69bc0d7d`). The latter keeps the original
surface charts, certifies two-sided source error, and independently
checks all display triangle contacts. Exact cap nesting (`8b7dc323`)
preserves both holes in the second sheared-sleeve fixture (Euler 0).
Two disconnected original floor fragments have separate indexed loops,
keep their irrational contacts, and reload the same closed topology. Rational vertical fibers
(`5661de68`) distinguish coordinates offset by 2⁻¹²⁰. Finite cap lines
(`ee24cf25`) retain the original irrational void exit; nearest polynomial
cap queries (`976a70df`) prove the squared distance 6−4√2 at an
irrational trim tip. A rational rigid occurrence (`7ee0cf62`) keeps these
line parameters and global mesh ownership through native reload.
The eccentric, quarter-rotated STEP hole (`4285219`) strictly clears its
original outer ellipse and preserves both owners. Pinned OCCT confirms
native reload, two original edges, and area 38π. The two-hole STEP face
(`07988e3`) likewise reimports one face and three original edges with
area 56π; exact separation of the independently owned inner trims is
at least 3 model units. The bounded STEP ellipse extrusion repair
(`c2c6b7b`) changes only the rounded period and re-proves both original
seam uses. The closed prism shell extension authenticates the two shared
rims and three original faces. Pinned OCCT measures volume 24π;
the exact promotion (`db873281`) sews four rational tensor walls and
two caps at zero continuous edge error. The independent native STEP
writer (`327e0e49`) retains analytic curves and independently reads
in pinned OCCT at the original and a translated placement.
Strict model-only import (`dadfc986`) round-trips that native material
without seam repair. The original Pythagorean STEP specimen
(`893aebbe`) retains exact 3/5 and 4/5 frame components and measures
150π at zero native continuous edge discrepancy.
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
The [latest full original production record](evidence/plaincad-reconciled-7b037e2d-production-2026-11-14.json)
passes all 66 cases in 239.9 seconds with no retries, skips or failures.
The original controlled rebuild and validated STL performance test
completed in about 23 seconds. Separate Go/WASM/no-OCCT preflight,
both typechecks, clean production build, original Playwright config,
and all 841 pinned files were verified on that reconciliation revision.
The [full original development record](evidence/plaincad-reconciled-development-178df69-2026-11-13.json)
on review revision `178df69` passed 344 cases in 2,632 seconds with no
retries, skips, or failures, plus separate Go/WASM preflight and typechecks.
All 841 pinned source hashes, the original Playwright config and both
result hashes were checked independently.
The development modeling selection on `ff15d87` passed three original
PlainCAD tests in 25.488 seconds, with zero retries, skips, or failures.
The separate real Go/WASM browser probe passed in 5.317 seconds. Evidence
records typechecking, all 841 original source checks, and hashes of the test
outputs for that reconciled revision.
Reconciliation `89f244c` combines the `1466e87` PlainCAD integration history
with the current main kernel. Ten original development Hole and STEP export
tests, the browser backend probe, both typechecks and 841 source comparisons
passed without retries or skips. A separate focused regression keeps the
strict nonbinary recipe rejection and proves the independently reconstructed
analytic result's volume, STEP readback and closed display mesh.
Reconciliation `586dcd3` combines `cc1c41f` and the integrated narrow
exact-first dispatch. Its [supplemental record](evidence/plaincad-reconciled-586dcd3-development-side-holes-2026-11-14.json)
contains seven Chromium cases (six blind or through overlap workflows, and
one close-up pick/display regression), one independent Go/WASM backend
probe, both typechecks, and 841 source hash comparisons. The seven
supplemental cases took 199 seconds with zero retries, skips or failures.
The focused native bridge and STEP suite also passed with CGO disabled.
The [grouped-opening reconciliation](evidence/plaincad-reconciled-d52a3af3-development-holes-step-2026-11-14.json)
passed ten unchanged Hole and STEP cases in 81.3 seconds on clean `d52a3af3`
with no retries, skips, or failures. The report verifies 841 original
PlainCAD sources and 1,745 kernel and harness source files against their
Git blobs, including the unchanged browser configuration. A separate
focused native regression confirms the exact two-cut body can continue
into a three-cut grouped body, preserve measures, and reload from STEP.

The [original unit run](evidence/plaincad-reconciled-bf37aa5-original-units-2026-11-14.json)
on `bf37aa5` recorded 1,653 passes and 36 failures out of 1,689, with no
pending tests. All 841 pinned files and both runner setup hashes were
verified. Six prior failures passed and no new failures appeared. The
remaining failures and their first diagnostics are listed in the report;
they are not counted as passes.
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

### Exact triangular residual total degree

`TestExactSexticTriangleTotalDegree` independently checks the exact squared
Bernstein bounds 1 for `u^6+v^6` minus its affine interpolant, and `1/924^2` for
`u^6*v^6` on the unit right triangle. The latter forces retention of actual total
degree twelve despite degree-six storage on each axis. Exact accepted operation
counts are 811 and 1284; one-operation deficits return no certificate. Existing
rational quartic and knot-crossing residual tests continue to check independent
exact evaluations and positive denominator handling.

### Selected nested graph mesh and original native binding

`TestCertifiedSelectedNestedDiskAnnulus` independently authors the exact
homogeneous degree-six triple-radial roof, proves all 228 original tube proposals
cover its three branches, and proves the complete containment hierarchy. The
retained disk plus nonadjacent annulus yielded 10,066 vertices and 20,128
triangles, component Euler characteristics 2 and 0, no shared component vertices
and a complete exact global no-contact report. The run used 106,185,142 residual
operations and 5,204,760 collision pairs with the pre-optimization residual
implementation (1399.94 seconds). Display volume 0.0002195165898933681 agrees
within the test's comparison allowance with independent analytic volume
0.00023010462359255695; this is not a certified volume-error statement.

`TestCertifiedSelectedTubeRegionCollection` qualifies two disjoint disks, a
selected subset without isolated unused vertices, late cumulative collision
exhaustion, cancellation, invalid selections and a changed-but-still-valid
represented nesting forest. `TestNativeTubeRegionCollectionMesh` requalifies an
original native graph lens and every indexed edge endpoint, ignores mutable
public diagnostic copies, and rejects reversed original material orientation
and a late exact endpoint-work deficit atomically. No test substitutes rounded
curve endpoints or welds vertices by coordinates.
The focused native and disjoint collection race run passed in 711.632 seconds
(331.35 and 378.96 seconds). The native regression's coarser requested display
has 630 vertices/1256 triangles and retains 320 exact endpoint operations;
the disjoint collection has 1172 vertices/2336 triangles and 273494 cumulative
collision pairs. The long nested qualification is a normal run, not a race run.

### Explicit degree-eight residual admission

`TestExactOcticBezierCertificates` independently checks X=u,Y=v,Z=u^8 v^8,
including exact derivative norm bounds 130 and 18 and owned coefficient copies.
`TestExactOcticTriangleResidual` checks the independent triangular squared bounds
`1/12870^2` for the polynomial and `2/24310^2` for homogeneous coordinates
(u,v,u^8 v^8,1+u^8 v^8), whose affine-target residual truly has degree seventeen.
The tests cover original multispan dispatch, default/lower-degree rejection,
upper degree rejection, and cumulative arithmetic exhaustion. Focused normal
and race tests pass (geom race 1.227s, trimmed race 4.068s), together with prior
quartic rational/knot and sextic exact-degree regressions.

The separate long original-source mesh command is:
`PLAINSOLID_TRIPLE_RADIAL_MESH=1 go test ./brep/trimmed -run '^TestAlgebraicShellTripleRadialMaterialComponents$' -count=1 -timeout 75m -v`.
It first rebuilds the original source/material and mass tensor, then independently
requalifies the pair and complete retained native boundary during mesh binding.
The environment gate preserves the existing ordinary test cost. Whole-mesh
success must be recorded separately from the focused degree-eight tests.
The flagship explicitly enables the existing affine-chart transport because the
original stock plane spans [-1/8,9/8] while the roof spans [0,1]. An initial run
without that option was deliberately stopped after original section coverage;
it was not a mesh qualification and its log is retained separately. No identity
chart assumption or proof gate was weakened.

Setting both `PLAINSOLID_TRIPLE_RADIAL_MESH=1` and
`PLAINSOLID_TRIPLE_RADIAL_AFFINE_COVERAGE=1` separately exercises the newer
radial-cover strategy in both original section construction and native mesh
reproof. The first full attempt reached 228 original proposals and exhausted
local transfer proof work before section completion in 664.23 seconds. The
generic affine-chart flagship above remains independently qualified. The compact
affine radial strategy passes; no triple radial transfer or mesh claim is made
for the new strategy until its own full run finishes.

The corrected original-source flagship passed on commit
`72682d7a63a351bf6793da48214e1a470d2c47ea` with the explicit environment gate:
2376.98 seconds for the complete test (2377.206 seconds package total), under
`GOMAXPROCS=2`. The original nominal-degree-eight surfaces produced the same
10,066-vertex/20,128-triangle disk-and-annulus display as the independent
exact-degree-six fixture. All four native face owners and all 228 original
edge/vertex bindings were retained, with closed component Euler characteristics
2 and 0 and a complete global exact no-contact report. Counters were 59,134,086
residual operations, 5,204,760 collision pairs, and 2280 endpoint operations.
The residual work is lower than the earlier independent degree-six run's
106,185,142 operations; this is not a controlled wall-time benchmark.
Display volume remains 0.0002195165898933681 against independent analytic
0.00023010462359255695, without claiming a certified volume-error bound.
The same run independently rechecked original section coverage, native
component construction, the tight exact volume enclosure and full mass tensor.
Full log: `/tmp/plainsolid-original-triple-native-mesh.log`.

## Exact radial tube coverage (2026-11-14)

Focused normal tests prove a 32-tube original circle cover with 542420 exact
operations and 1024 neighbor tests. Tests reject a partial cover, endpoint-only
joins, changed radius/source, and missing circles on a genuine two-radius
quartic source; two unrelated-radius arc neighborhoods cannot stand in for
coverage. Reordered occurrences remain valid. Result slices are owned; canceled
work and insufficient tube, operation or neighbor limits publish no certificate.
Exact reported operation and neighbor budgets succeed, one less fails.

Commands (isolated `codex/exact-radial-coverage` checkout):

```sh
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./geom -run '^TestExactRadialTubeCoverage' -count=1 -v
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./geom -run '^TestExactRadialTubeCoverage' -count=1
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go vet ./geom
```

This independently qualifies exact coverage only, not a new native topology or
an enlargement of the full replacement claim.

## Integrated independent radial cover (2026-11-14)

`TestNativeLensIndependentRadialCoverage` uses radius5/16 on the complete original
plane/dome unit charts, retains existing abutting topology hints, and builds the
native Intersection component (two original faces, Euler2). Independent radial
coverage uses36 overlapping cells and998191 total exact operations, including
coefficient recognition, proposal verification, immutable re-verification,
endpoint cover and exact transfer into the original supplied union. Removing
half the original hints rejects atomically despite a complete independent cover.
Exact reported work succeeds; one operation less, proposal quota and canceled
context publish no accepted cover. Generic isolation still succeeds64 candidates.

One normal comparison measured radial coverage0.49s and generic0.77s; these are
local observations, not a controlled benchmark or flagship estimate. Focused
intersect coverage/trace/legacy radial rejection regressions and vet passed.
The complete native regression also passed under the race detector in63.22s.
Commands:

```sh
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./brep/trimmed -run '^TestNativeLensIndependentRadialCoverage$' -count=1 -v -timeout 4m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./brep/trimmed -run '^TestNativeLensIndependentRadialCoverage$' -count=1 -v -timeout 4m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./intersect -run '^(TestSplineTubeCoverage|TestSplinePairTrace|TestRadialContourDiscoveryAtomicRejections)' -count=1
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go vet ./intersect ./brep/trimmed
```

## Affine original radial coverage (2026-11-14)

The compact original plane/dome native shell passes affine radial coverage,
complete native section, Intersection material and a two-face Euler2 native
component preserving both original surface objects. Stock physical footprint
[-1/8,9/8]^2 uses original UV[0,1]^2; the roof uses physical/UV[0,1]^2.
The exact common-footprint restriction therefore requires UV1/10 and9/10,
which remain rational in the coefficient proof. The complete coverage charges
998170 operations. Focused affine race passed (geometry 1.235s, native 51.53s),
and vet passed for geom/intersect/trimmed. Exact total quota succeeds; one less, omitted original
tubes and use of the old identity-only path reject atomically.

Independent geometry tests compare transported canonical coefficients exactly
against the original physical plane, test a signed swapped chart and outward
(nonexact) mapped proposals, reject a clipped common footprint and a height
perturbation, and verify cancellation/exact arithmetic quotas. No test trusts
box overlap as incidence. The topology test explicitly re-proves original
B-free hints rather than rounding a new original free endpoint.

```sh
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./geom -run '^(TestAffineRadialOriginalTransport|TestExactRadialTubeCoverage)' -count=1 -v
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./brep/trimmed -run '^TestNativeLensAffineRadialCoverage$' -count=1 -v -timeout 4m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./geom ./brep/trimmed -run '^(TestAffineRadialOriginalTransport|TestNativeLensAffineRadialCoverage)$' -count=1 -v -timeout 4m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go vet ./geom ./intersect ./brep/trimmed
```

## Physical non-unit radial footprint (2026-11-14)

Exact circle coverage qualifies common physical rectangle [2,4] x [-1,2], with
independent larger original footprint [1,5] x [-2,3], physical center(3,1/2) and
radius1/4. Original charts have different scales4/5 versus2/3. The independent
proposal/coverage/transfer lane returns36 arcs; omission of half the original
occurrences and proposal quota exhaustion reject atomically. Coverage charges
1095792 exact operations. Focused race passed (geometry 1.414s, intersect 14.103s);
vet passed for geom/intersect. This is a complete
pair-coverage qualification, not a claim of new native non-unit solid assembly.

A separate coefficient fixture uses rational rectangle [2,4] x [-1/3,8/3],
center(3,7/6), unequal physical widths2 and3. It verifies exact owned rectangle
bounds and rejects both a normalized-UV circle impostor and an actual circle
of radius5/4 extending beyond the rectangle. Existing unit radial volume,
centroid, moments, tensor, factor and coverage regressions passed together.
Both existing native lens routes passed again (35.033s combined); the affine
route now charges997027 operations after removing redundant canonical reproof.

```sh
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./intersect -run '^(TestAffineRadialPhysicalRectangle|TestRadialPhysicalRectangleCoverage)$' -count=1 -v -timeout 3m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./geom ./intersect -run '^(TestAffineRadialPhysicalRectangle|TestAffineRadialOriginalTransport|TestRadialPhysicalRectangleCoverage)$' -count=1 -v -timeout 3m
```

## Exact radial transfer strata (2026-11-14)

Focused geometry tests independently exercise an uncovered tangent on an open
edge, a vertex-only tangent, a covered boundary-only contact, and an uncovered
radius-one arc inside a rectangle of half-width 2^-40. Public original-support
checks reject the corresponding tangent/tiny-arc candidates without targets,
freshly prove a valid uniform target, reject a forged broad target, and enforce
cancellation and exact/one-less operation quotas. Focused race passed for the
strata, original target certificates, and non-unit pair integration. The
physical rectangle pair uses 36 cover cells and 1,104,858 exact operations on
the integrated revision, including fresh target proofs.

```sh
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./geom ./intersect -run '^(TestRadialTransferClosedStrata|TestRadialPhysicalRectangleCoverage)$' -count=1 -v -timeout 3m
GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test -race ./geom -run '^TestRadialTransferOriginalCertificate$' -count=1
PLAINSOLID_RADIAL_TRANSFER_TRIPLE=1 GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./intersect -run '^TestTripleRadialPhysicalTransferCoverage$' -count=1 -v -timeout 15m
```

The opt-in direct original triple test passed in 75.60 s on the isolated
implementation at `0c2b241`. It verified 228 original tubes and 248 overlapping
cover cells, with 47,917,293 exact operations, 15,124 partition strata and
7,240 uncovered-stratum checks. This qualifies the original coverage transfer;
native assembly and meshing with this strategy have their own end-to-end test.

## Direct coincident roof-box volume and mass (2026-11-14)

`TestCoincidentRoofBoxMaterialMoments` compares direct rational volume, first
moments, centroid and world inertia intervals with independent pinned OCCT
construction and `GProp` evaluation for nine Common, Fuse and Cut results.
The fixtures include a face-only intersection, overlapping boxes with shared
walls, floor and roof, and equal boxes. Tests check exact rational values and
atomic cancellation and fail-closed source rejection. The live focused race
run (`PLAINSOLID_OCCT_COINCIDENT_ROOF_AREA` with `mass` mode) passed; the separate
surface area regression passed in the same run.

`TestCoincidentRoofBoxPointLocation` checks 36 source classifications against
independent OCCT regularized box Common, Fuse and Cut, including both sides
of an exposed seam and a zero-volume contact. `TestRoofBoxNonlinearPointGerms`
checks a nonplanar wedge and cancellation on coincident rational and creased
roofs. The tangent regression uses `(Y-X)^2-X^3`, where diagonal cancellation
is necessary to see both signs. Work exhaustion and cancellation return no
location result. The focused live race run passed in 4.533 s, including the
shared boundary measures and independent creased-roof check.
`TestCoincidentRoofBoxVerticalPartition` also passed under race with the live
point oracle enabled. It checks a reversed nonunit line, all six exact
stations, opposite use of the shared wall under Common/Fuse/Cut, a singleton
clip through two cancelled roofs, and atomic operation exhaustion.

## Original non-unit native annulus (2026-11-14)

`TestNativeNonunitRadialAnnulus` checks the original roof height in the annulus,
hole and exterior without meshing. Full section, intersection and native
assembly retain two original supports, 176 original edges, two inner wires
and Euler characteristic zero, with two opposed uses per edge. Removing a
loop fails. The earlier complete normal run passed in 108.16 s; the final
race run with ownership and mutation checks passed in 589.60 s.
`TestNonunitRadialExactTransfer` checks 124 independent cover cells against
176 tubes and exercises operation, stratum and cell limits; it passed in
32.09 s under the race detector. Both ran together on isolated `84b54eac`'s
test tree in 623.161 s. Independent integration of the original height field
gives volume π·421875/2147483648; this test qualifies native topology.

## Complete affine triple-region qualification (2026-11-14)

The integrated affine strategy at `ca88d4ad` passed the full original
`TestAlgebraicShellTripleRadialMaterialComponents` in 1,339.65 s (package
1,339.900 s). The 248 independently covered overlap cells transferred to
228 original tubes with 47,917,325 exact operations. Native assembly retained
four original faces and two components. The exact volume coefficient was
63333935546875/864691128455135232 times π, and the exact centroid Z was
106610222140625/232841378390867968.

The certified degree-eight native display retained 10,066 vertices and
20,128 triangles, checked 5,204,760 collision candidates with zero collisions,
and spent 59,134,086 exact residual operations. Display volume was
0.0002195165898933681 against analytic 0.00023010462359255695. This qualifies the
same retained material as the earlier generic run, which took 2,376.98 s.

```sh
PLAINSOLID_TRIPLE_RADIAL_AFFINE_COVERAGE=1 PLAINSOLID_TRIPLE_RADIAL_MESH=1 GOMAXPROCS=2 GOCACHE=/tmp/plainsolid-go-cache go test ./brep/trimmed -run '^TestAlgebraicShellTripleRadialMaterialComponents$' -count=1 -timeout 75m -v
```

Local log: `/tmp/plainsolid-affine-triple-physical-full-2026-11-14.log`, SHA-256
`7ca811e33294fab9574071ff8340960268a93604b9d8ddf1ca3b12ca98b8053a`.

## Physical annulus moments with independent beta integral (2026-11-14)

`TestAffineRadialPhysicalAnnulusMoments` uses exact Bernstein roofs over
[2,4]×[−1,2] with a swapped, reversed chart and common nonradial height
7+X/3−Y/5. For q=(X−3)²+(Y−1/2)², a=25/1024, b=25/256 and L=b−a,
the gap is H=3(q−a)(b−q). Independent integration gives volume/π=L³/2,
integral(H²)dq=3L⁵/10, mean q=(a+b)/2, and centroid Z=79/10+3L²/10.
The new physical volume, planar and mass APIs agree on the exact volume,
second moments, centroid and first Z moment, including work limits and output
ownership. The integrated focused geometry race passed in 1.680 s. These
methods accept explicit radial bands and do not independently select native
Boolean material.

## Physical radial mass from original native occupancy (2026-11-14)

The native annulus selected band 1 directly from the original two wires and
charged 12,127 binding and exact integration operations. The regression
rejects a missing hole, reversed material, unrelated support, wrong circles,
and one-less quota. Changing public diagnostics and returned band labels does
not change selection. The original proof also accepts independently decoded
exactly equivalent support recipes. The qualifying native race at `4613e35`
passed in 590.20 s; final normal with decoded-support evidence passed in
86.39 s. A separate original unit-chart regression passed under race in
36.408 s. No mesh was run for this query.

## Exact physical radial tensor (2026-11-14)

`AffineRadialContours.MassTensorBands` and
`AlgebraicShellBoolean.PhysicalRadialTubeRegionMassTensor` integrate XZ, YZ and
ZZ, as well as all planar moments. The tilted annulus checks all nine second
moments and world-origin inertia against independent angular and beta
integrals. The integrated geometry race passed in 1.655 s. The original
native annulus at `54dfd84` confirmed automatic material band selection and
exact XZ/YZ/ZZ in 77.74 s, including quota and source mutation negatives.
The existing source degree, product degree, and integrand degree bounds stay
in force. The test did not repeat mesh qualification.

## Native ordered roof-box Common (2026-11-14)

Tests verify six output faces, twelve shared edges, Euler characteristic two,
zero continuous edge disagreement and deterministic owners for Common with
coincident supports. A curved clipped result has exact volume 193/512 and
matches independent pinned OCCT volume, boundary area and centroid. An
identical weighted pair retains exact height (2+x²)/(1+x), analytic volume
3 log 2 − 1/2, and stable volume after native reload. A parabolic roof tangent
to a flat roof at X=1/2 retains exactly the unit box; live OCCT agrees. A real
crossing, face-only contact, and exhausted arithmetic distinguish unsupported,
regularized empty, and atomic budget results respectively. Focused live race
passed in 4.946 s.

## Exact nonradial projected tube coverage (2026-11-14)

The checked common XY chart reduces contact to the zero set of a rational
height numerator. `CertifyCommonXYGraphCellInTubes` verifies every original
uniform target. An exact partition checks uncovered closed vertices, open
edges and open rectangles separately; weak Bernstein sign plus one strict
coefficient excludes zeros in the relative interior. It never heals a target
or treats a boundary zero as harmless.

Two original curved branches used 12 target tubes and 342,743 operations,
covering 663 strata (353 uncovered tests); the generic proof also passed
with 385,617 operations and exhausted the lower quota. Removing either
branch, forging a tube, changing the XY projection or exceeding the budget
fails closed. The integrated focused race passed in 6.838 s. This is the
direct projected proof; the section driver is separately qualified.

`SubtractOrderedRoofBoxes` separately qualifies a covered-footprint Cut of
an exact weighted stock. The original reversed cutter roof becomes the lower
face, and rational ruled walls share a positive homogeneous denominator and
both original boundary curves. The result volume is 9/2 − 3 log 2 and the
area agrees with the independent OCCT Boolean. A fully covering cutter
returns empty; floor-only contact retains the original stock. The fixed
one-band constructor rejects an unresolved or multi-band difference. The
focused live race run for Common and Cut passed in 27.850 s.

`SplineTubeCoverageOptions.CommonXYProjected` now opts into the fresh
projected proof before generic coverage. Both strategies share the bounded
local work allowance, including unsuccessful attempts, and both still require
independent branch topology. The full native section on intersecting original
footprints found two curved branches and four original boundary terminals.
It used 128 projected candidates, 22 generic fallbacks and 1,141,823 local
operations. Omitting one branch or allowing one fewer operation failed;
unsupported projections still use the generic route and duplicate tube
topology fails. The originating focused race passed in 404.491 s, and the
integrated non-race rerun passed in 25.557 s.

## Original nested spline cavity and island display (2026-11-14)

The warped stock with an inward cavity and a depth-two material island
matches analytic material volumes 4.5 and 4.5 + 1/16 within the display
error test. The displayed nesting, signed orientation and triangle owners
agree with the original native components, including after source reload and
a proper rational rotation. The source shell gap guard rejects one rounded
placement that passes generic mesh embedding. Too-close original boundaries,
wrong cavity polarity and insufficient face-pair, triangle or intersection
allowance fail atomically. The originating focused race passed in 62.647 s;
the integrated non-race rerun passed in 3.308 s.

## Source-bound through and blind frustum bore displays (2026-11-14)

Reindexed and reloaded tapered eccentric stock retains all 18 original
edges, 36 coedge occurrences and 16 corners for the through and blind
variants. The through case remains genus one; both blind entry directions
retain the original reversed floor and opposite floor normals. Each mesh
has 320 triangles. Independent ideal volumes and exact boundary error
checks are separate; the display volumes differ at the prescribed 0.5
deflection. Rational placement, forged or missing trims, wrong polarity,
cancellation and budget rejection are exercised. The integrated focused
race passed in 20.165 s.

The unbored stock path uses the same private reproof and ownership binder.
A reindexed, reloaded descending frustum and cylinder each have 160
triangles and retain all nine edges, 18 coedge occurrences and eight corners.
Both original caps, Euler characteristic two, a proper rational placement
and independent ideal volume checks pass. Forged caps, a missing seam,
cancellation and quota exhaustion fail. The originating combined race
passed in 27.268 s and the integrated stock race passed in 6.017 s.

The circular station loft display re-proves both sides of an original
reentrant waist with radii 3, 1, and 2. It shares all junction indices
only after comparing original samples exactly. Original quarter edges,
28 coedge occurrences, 12 corners and the sign change of adjacent wall
normals survive reindexing and native reload. The 288 triangles match
the existing public mesher output; analytic volume `9π` independently
checks the display at the 0.2 deflection setting. Integrated race passed
in 10.678 s; the originating combined adapter race passed in 37.503 s.

The hollow variant independently re-proves two outer and three inner
bands, all 66 original coedge uses, inner orientation and both annular
caps after reindexing and native reload. Twelve original interior quarter
rims each retain exactly two adjacent wall owners. The 704 triangles
are unchanged from the existing certified public mesher. The independent
integral is `627π/32`; the mesh's lower volume at 0.3 deflection is
recorded separately. Combined integrated station races passed in 51.044 s.

## Native ordered roof-box Fuse (2026-11-14)

The independent weighted-stock Fuse reproduces exact height
`(2 + X²)/(1 + X)` on its original roof and analytic volume `3 log 2 + 1/2`.
Pinned OCCT adaptive integration agrees on volume, area and centroid. OCCT
retains subdivisions of the walls; both kernels return one solid and Euler
characteristic two. Operand swap verifies original roof/floor ownership
and both contributors to every merged wall. A parabolic roof meets the
other floor tangentially on an interior line, which vanishes from the
regularized output. Crossing order, footprint mismatch, a real inter-band
gap and insufficient arithmetic return no topology. The combined live
Common/Cut/Fuse race suite passed in 7.492 s.

The subsequent full-footprint Cut with a suspended weighted cutter retains
two components, each with six faces and Euler characteristic two. Its
volume `11/2 − 3 log 2` agrees with the analytic height integral, and
independent pinned OCCT agrees on area and centroid as well. The two
cutter caps reverse and each stock wall maps to two fragments. The live
split regression passed under race as part of the 16.903 s run.
An additional interior floor contact fixture distinguishes an empty Common
and unchanged Cut at tangency from a pinched positive-volume intersection,
which this constructor rejects. The complete original-order roof suite
passed under race, with live OCCT, in 10.550 s.
Adaptive exact Bernstein sign checks add a contact at X=3/8 and its
nearby strictly separated variant. The former qualifies Common while
Cut correctly rejects the pinch. The latter produces a positive Cut
band with independently integrated volume. The full focused live roof
race passed in 10.621 s.
The complementary top Cut retains the reversed original cutter floor as
the new roof. The remaining unit-footprint prism has volume 1/2 and area
4, matching pinned OCCT. A roof-only touch keeps the original stock;
crossing the cutter floor through the roof fails atomically.
The strict vertical-gap Fuse preserves both original six-face shells and
face ancestry under operand reversal, with Euler characteristic four and
an analytic volume check. Edge-only contact is distinguished from strict
separation. The focused roof suite passed under race in 12.731 s.
Projected footprint containment preserves the original weighted stock and
its face ownership under operand swap; strict XY separation preserves both
shells, and partial overlap still fails this path. The focused live Fuse
race passed in 6.330 s.
The partially shared stacked cap removes just the interior interface and
retains each original exposed fragment under both operand orders. The
single solid has Euler characteristic two. Its analytic volume and live
OCCT mass, centroid and area qualified in the focused 5.849 s race run.
Disjoint curved stocks also qualify when only their overlapping projected
rectangle proves the strict vertical gap. A neighboring exact edge touch
fails the strict proof and returns no topology.
The fully enclosed weighted cutter produces an inward cavity with original
edge and face geometry. Its exact volume difference, area and centroid
match the pinned OCCT cavity. A cutter touching the stock floor is rejected
by this enclosure path. The integrated live Cut race passed in 7.730 s.
An exactly knot-refined weighted roof now reduces to the same global
rational chart for native ordered Booleans. Nonbinary point and derivative
checks match the original evaluator, while a genuine crease is rejected.
The complete focused live roof race passed in 15.502 s.
For one-coordinate height differences, exact real-root isolation now
distinguishes genuine crossings from even-multiplicity contacts anywhere
in the original footprint. The authored `X=1/3` tangency retains the flat
Common and does not qualify a strictly separated Cut band.
The ordered Cut dispatcher also carries the reversed output face IDs into
its ancestry report, verified for the two-band case in the live 6.587 s
Cut race run.

## Isolated quartic graph contact with odd mixed terms (2026-11-14)

The represented spline controls are dyadic but the original contact
parameters are exactly `(1/3, 2/5)`. Independent sums of fourth powers
verify the polynomial and Gram minors. Exact tangent jets and both height
orientations agree, including a positive rational weight variant. Extra
terms, saddle and semidefinite forms, a folded projection, shifted witness,
and insufficient allowance fail. The integrated focused race including
the existing coupled quartic regression passed in 1.899 s. Vet passed.

The integrated exact hierarchy boundary minimum matches independent live
OCCT on the original seven-solid neutral-child and vendor-frame fixtures.
The exact reducer distinguishes strict minima and retains rational/algebraic
ties, including repeated uses. Boundary-zero queries, cancellation and
insufficient budget have dedicated checks. The integrated exchange and
document races passed in 4.910 s and 8.889 s respectively.

The five-family frustum STEP fixture keeps nine expanded occurrences,
including two placements of one independently built rational frustum, and
re-proves source and local native solid reload. The independent pinned
OCCT adaptive oracle agrees on assembly volume and area before and after
emission. Old hierarchy readers and native tags retain their limits.
The integrated focused race including legacy torus and aggregate distance
regressions passed in 16.291 s.

The distinct source-bound frustum hierarchy envelope round-trips
canonically through native and source STEP. It rejects forged local native
geometry, support provenance, definition IDs, roles, pose paths, source
bytes, tag and version. Native-to-STEP transfer remains valid in the live
OCCT assembly oracle. The integrated native/frustum/legacy-torus race
passed in 21.393 s; the originating bounded fuzz run exercised 40 inputs.
World bounds remain exact after signed-axis placement and native reload.
Circular and noncircular frusta use attained support extrema and retain
every original occurrence path. The aggregate agrees with pinned OCCT;
forged paths and exhausted source limits fail. The integrated live race
passed in 14.041 s.
The circular-only mass query keeps rational-plus-π coefficients throughout
local section integration and exact signed-pose transport. The independent
OCCT oracle agrees on volume, centroid and all entries of both inertia
tensors. Local analytic coefficients, native reload and rejection of a
forged or noncircular support were checked. The integrated frustum and
legacy rational hierarchy mass races passed in 16.734 s.
The circular frustum point query independently validates both signed-axis
poses against live OCCT, retaining cap and wall at the same rim, seams,
and original vertices. The nonbinary seam and ±2^-120 side tests use
exact comparison; tolerance-based OCCT is used for separated controls.
The integrated point race passed in 29.065 s.

The [full original development run on the integration review snapshot](evidence/plaincad-reconciled-f87dd4f3-development-2026-11-14.json)
finished with 340 passes and four failures. The original two focused
constraint repair assertions seed storage at port 5279, whereas the first
run used port 5753. Two original shop-drawing cases failed in document
or fixture readiness and passed separate unchanged PlainSolid and OCCT
controls. The full run remains failed; its source and WASM were checked
against the recorded revisions and its backend probe passed. A clean
original-port [targeted rerun](evidence/plaincad-f87dd4f3-default-origin-controls-2026-11-14.json)
passed both original constraint fixtures and all three drawing fixtures
on the same WASM, with the pinned original Playwright configuration.
The original assertions are preserved; only the declared provider label
substitutions apply. The full run's four failures remain recorded.

The projected Gram variant uses a complete common XZ footprint of two
differently signed, scaled, and ordered charts. It verifies a nonbinary
witness and maps it back to both original UVs. Wrong axes and witness,
operand reversal and atomic exact-work exhaustion are independently
checked. The integrated projected and unprojected quartic races passed
in 1.989 s.
`CertifySplinePairQuarticPointSection` separately re-proves that contact
on the actual finite patch pair, returns copied exact coordinates and
the two original owners, and exposes no partial point on error. XY/XZ,
operand reversal, clipped charts and a one-less work limit are covered.
The integrated focused race passed in 1.220 s.

The finite semicubical cusp certificate verifies the full original
height numerator, both regular germs, the one singular point, and both
exact terminal witnesses without sampling the interiors to claim
completeness. Nonbinary coefficients, operand reversal, source
perturbations and exact budget failures are checked. The integrated
focused race passed in 1.311 s.
The immutable intersect-level cusp section independently evaluates both
supports for rational samples on both labeled germs. It distinguishes
the shared cusp from each finite boundary terminal and preserves original
owner identity. The integrated focused race passed in 1.209 s.
The projected cusp proof maps original UMax terminals to VMin on an
independent XZ chart, and verifies both owners' original coordinates.
Changing projection or exceeding the shared quota fails atomically.
The integrated focused race passed in 1.245 s.

### Coplanar polygon/hole ring qualification

`TestNoncontactingAffineRings*` checks a refined oblique support against a reversed
independent chart with rational nested outer/hole loops. Intersection retains the
inner outer ring plus original hole, union suppresses the covered hole, and
Difference reverses the tool boundary. A smaller island inside the stock hole
has empty intersection and three union rings. Disjoint domains, shared-point,
crossing and collinear-contact rejection, exact model evaluation, detached result
mutation, cancellation and exact/one-short work bounds are independently checked.
The extracted shared trim validator retains the transverse hole regressions.
The integrated original-affine, convex, transverse-hole and coplanar-ring race passed in 1.424 s.

### Proper-crossing arrangement qualification

Fixtures verify signed exact intersection, union and difference areas of
15/64, 223/256 and 83/256 on overlapping squares with distinct holes. A
cutter crossing an original hole gives 5/32, 7/8 and 19/32 respectively
and preserves exact half-edge source parameters. Tests re-evaluate original
UV and model endpoints and exercise cycle indexing, reversed charts, result
mutation, exact and one-short budgets, cancellation and unsupported contact
rejection. The integrated affine polygon and patch race passed in 1.542 s.

### Weighted station boundary area qualification

The area tests re-read native JSON and integrate the original spline partials
independently for every wall. They verify the asymmetric filled case and
both wall families of a bore with unequal station subdivisions. Every cap
uses an independent profile integral and retains its original face identity.
The filled and hollow enclosures meet total width requests of 1/64 and 1/20
respectively at 256 spans. Cancellation, limited source proof and one-short
integration budgets return empty results. The combined area, mass, bore
and rigid occurrence race run passed in 67.058 s.

### Shared-edge arrangement qualification

The general affine tests cover whole and partial shared edges, T-junctions,
coincident polygons, independently reversed charts, filled holes and
isolated corner contact. They compare proper crossing results with the
strict API, preserve all original directed uses and exercise result mutation
and exact or one-short budgets. The integrated affine polygon and patch
race passed in 1.919 s.

### Weighted station loft STEP qualification

The filled-loft suite checks one and two bands, preserves original poles,
weights, nonuniform knots and shared rim IDs, and rejects edited p-curves,
orientation and detached faces. Pinned OCCT reads the asymmetric two-band
export with volume 28.0579957962 and axial centroid 1.314814814753. The
integrated weighted and circular loft and bore race passed in 152.864 s
using an oracle built with the station source-authoring modes.

### Weighted station inertia qualification

Native JSON reload, independent Green integrals over the original weighted
profile and independent axial quadrature check all raw moments and tensor
entries for an asymmetric filled loft and a bore with distinct station
partitions. The filled case retains its signed coupled entries and gives
identical bounds after an exact large translation. Both fixtures meet
per-entry widths of 1/20 and 1/10 respectively. The combined inertia,
boundary area and mass race passed in 25.650 s.

### Native arrangement prism qualification

Intersection, union and difference match analytic volumes 1/2, 7/4 and 5/8,
and the existing exact BSP path agrees on the overlapping box fixture.
A separate holed Difference yields two components and one inner wire with
total volume 83/128. Native reconstruction, oblique and signed offsets,
original side ledger coverage and atomic representability failures are
checked. The integrated race passed in 1.694 s.

### Weighted station bore STEP qualification

Canonical one- and two-band weighted bores round-trip with original
supports and face orientation. The two-band reference retains distinct
outer and inner partitions. Pinned OCCT reads volume 24.3558991065 and
axial centroid 1.276666666394, agreeing with independent profile and
axial integrals. Focused main race covers deterministic re-export, native
reload and source mutation rejection.

### Weighted occurrence measure qualification

A non-cardinal rational rotation and nonbinary translation preserve the
original area and enclose all world tensor entries within 1/15. The test
compares against an independently integrated and rotated tensor, changes
the translation exactly, and checks independent intervals and atomic
transform exhaustion. The integrated focused race passed in 7.667 s.

### Independently authored weighted station STEP qualification

Pinned OCCT authors the filled solid and a through-bore Cut from original
dyadic weighted B-spline supports. Both readers accept the exported files
and revalidate native rim ownership and original support data. Canonical
re-export and OCCT transfer preserve mass and every centroid coordinate.
The integrated live race passed in 35.131 s. The independent author and
reproduction commands are [in the oracle notes](../tools/occtref/weighted_station_author.md).

### Native prism operand qualification

Tests cover all three operations, disconnected holed output, signed axis
permutations and slanted profile edges. They reconstruct original directed
source intervals on both caps and reject unequal height intervals, tapered
walls and exhausted budgets. The integrated native prism and arrangement
race passed in 2.008 s.

### Oblique NURBS planar assembly qualification

Independent OCCT fixtures cover original and shifted local spline charts
on two oblique products with three original uses. Native material retains
the original geometry and placement graph. OCCT verifies volume 1750,
area 1350 and placed centroid (69/14, 66/7, 99/14). The integrated unit,
scaled and oblique live race passed in 4.092 s.

### Exact trimmed sewing qualification

The source fixture is a weighted two-band loft exploded into independent
face-local edges, including the periodic chart seam. One original curved
edge is reversed and reparameterized and one face is flipped. The sewn
output retains every original chart and p-curve and passes the independent
loft embedding validation. Missing faces, altered interiors with identical
endpoints and a one-ULP endpoint shift reject. Two point-touching components
retain separate vertex fans. The integrated race passed in 4.877 s.

### Independent weighted station scale qualification

The original noncircular fixture has XY scales (3,1), (1,2), (2,1) at
heights 0, 1 and 3 and independent nonuniform original parameter domains.
A profile integral, analytic axial factors and generic integration agree
on volume and centroid. Changed interior weights with the same rim
corners, invalid scales and forged source geometry reject. The integrated
geometry race passed in 1.286 s and the combined trimmed race in 32.270 s.

### Oblique native prism qualification

The original fixtures have oblique cap planes and extrusion vectors.
Common, fuse and cut preserve authored side and cap ancestry. Additional
cases verify disconnected holed Difference, a reflected determinant-two
geometry map, competing extrusion directions and bounded or invalid
sources. The combined oblique, cardinal and arrangement materialization
race passed in 2.487 s.

### Irrational convex inset qualification

The triangular prism independently verifies the exact volume
`297/32 − (21/16)√2` and the Euclidean shift of the hypotenuse. The cuboid
keeps exact rational coordinates. Collapsed incidence, incompatible
quadratic fields, insufficient conversion accuracy and cancellation
reject without geometry. The full draft package race passed in 7.760 s.

### Source-bound planar NURBS assembly persistence

Unit, scaled and oblique planar NURBS assemblies each reload through the
matching original STEP authentication path. Mutated native geometry, source
records, appearance, placement and affine proof fields reject. The three
independent OCCT chains preserve volume, area, centroids and occurrence
colors after native reload and STEP transfer. The integrated live race
passed in 14.234 s. See the [native envelope contract](../exchange/step/NURBS_PLANAR_ASSEMBLY_NATIVE.md).

### Independent weighted station mass qualification

The signed asymmetric fixture checks independent factors 43/6, 37/3,
32/3 and 10 and exact axial centroid −112/43 after translation. Green
integrals over the original profile check each horizontal component.
Native reload, deterministic refinement and atomic limit failures pass.
The integrated native and mass race passed in 20.167 s.

### Native cap contribution qualification

All three Boolean operations check exact material contributions and original
boundary parameters separately. Tests cover disconnected holed Difference,
oblique area scale, independently reordered source rings, output mutation,
cancellation and exact work limits. The integrated native prism race passed
in 2.973 s.

### Convex inset with independent irrational supports

An independently integrated pentagonal prism has supports involving √2
and √5. Its ten ideal algebraic coordinates and native volume match the
intersection of five independently shifted lines. Exact √6 and √24 loads
reuse the √2 and √3 generators and verify symbolic cancellation. The
single-class fixtures still pass. The full draft race passed in 8.196 s.

### Unequal-interval native prism qualification

Fixtures cover staggered, contained and tied caps, rational clipped wall
fractions, a disconnected nonconvex profile intersection, and separate
axial or footprint contact cases. The integrated race passed in 1.563 s.

### Convex erosion through topology changes

The full convex entry reconstructs the native box after a diagonal
support disappears. The fixtures cover both an irrational support
beyond extinction and a Pythagorean support at exact extinction. Native
volume, exact volume and original support maps agree. A collapsed height
regularizes to an empty solid. The full draft race passed in 9.166 s.

### Shared-group occurrence appearance qualification

Independent OCCT authors two group uses with six distinct leaf paths and a
shared internal override. The original, reloaded and regenerated source
all retain the six native placements and colors, volume 108 and area 252.
The combined flat and nested live race passed in 4.696 s.

### Anisotropic point classification qualification

Exact quadrant and sub-float radial probes exercise filled, boundary and
outside results with original topology after reload and index changes.
Both wall owners survive at a crease. Exact and one-short query limits
are checked. The integrated geometry race passed in 1.199 s and the combined
trimmed race in 44.829 s.

### Path-specific AP242 style qualification

Independent OCCT reads both complete repeated-group paths with SHUO mode
enabled. The selected path is blue and the other shared leaf use remains
red. Retargeting the upper use swaps that single override. Exact native
reproof and STEP transfer retain six placed leaves, volume 108 and area
252. The combined flat, nested and SHUO live race passed in 7.350 s.

### Blind native prism qualification

Both entry directions, exact volume, centroid and closed topology pass.
A nonconvex stock and slanted triangular tool preserve the original wall
sources. Touching profiles and unsupported axial trims reject atomically.
The integrated race passed in 1.529 s.

### Exact horizontal anisotropic line qualification

Horizontal clips in one original band preserve weighted algebraic line
parameters, open material intervals and original wall IDs. Independent
weighted diagonal, scaled secant, tangent, clipped and sub-ULP exterior
fixtures pass. The combined anisotropic and homothetic line race passed
in 40.723 s for the trimmed package.

### Native prism boss qualification

Both entry directions, an oblique profile side and a triangular boss on
nonconvex stock preserve native closure, independent volume and centroid,
and original support ownership. The integrated prism shoulder and cavity
race passed in 1.684 s.

### Native prism cavity qualification

Off-center volume and centroid match independent integration. Results retain
separate outward and inward shell graphs, including a triangular cavity in
signed-axis nonconvex stock. Touching or unsupported shells fail before
publication. The integrated prism shoulder and cavity race passed in 1.684 s.

### Exact nonhorizontal anisotropic band line qualification

Finite nonhorizontal lines confined to one band reduce to weighted
quartic sectors. Original model parameters and wall IDs survive exact
and isolated nonexact hits. Generators, ambiguous brackets and exhausted
budgets reject. The integrated horizontal and nonhorizontal race passed
in 33.471 s.

### Disjoint ordinary and SHUO appearance qualification

A genuine OCCT assembly retains two shared cyan paths and one path-specific
blue override. Six exact poses, volume 108 and area 252 survive source-native
reload and fresh STEP observation. Retargeting the SHUO preserves the two
ordinary paths. Overlapping ownership rejects even when colors agree. The
combined appearance race passed in 9.942 s. Current public XCAF authoring
calls rejected both attempted face-local occurrence authoring paths. The
[combined appearance contract](../exchange/step/COMBINED_OCCURRENCE_COLORS.md)
includes a reproducible probe for that fixture limitation.

### Convex Euclidean footprint offset qualification

The 3-4-5 triangle verifies the exact diagonal support shift, original
corner centers and Steiner volume `27 + (25/32)π` for both height signs
and original winding orders. Both placed analytic results survive native
reload. The exact point tests include a two-owner tangent junction and
rational inside and outside probes below binary64 resolution. Nonconvex and
unrepresentable profiles reject. The full offset race passed in 170.800 s.

### Native prism through-cut qualification

Tests cover covering spans, exact cap ties, a nonconvex stock with a
contained triangular profile, rational source-corner reconstruction and
unsupported contact rejection. The combined blind, boss, cavity and
through race passed in 1.942 s.

### Multiband anisotropic line qualification

The line sweep preserves original parameters across both terminal planes
and internal station creases. An exact cardinal root shared between sectors
rejoins only after an affine parameter and quartic identity proof. Both
original crease faces and shared rim vertices survive ownership recovery.
One-short cumulative query limits reject. The combined line race passed in
33.650 s.

### Depth-three AP242 SHUO qualification

Independent OCCT reads twelve placed leaves, volume 216 and area 504.
Retargeting the middle or outer link moves the single colored path. The
exact native reproof retains the original graph and refuses competing
continuations or wrong terminal ownership. The combined appearance race
passed in 11.562 s.

### Crossing prism transition qualification

Both entry directions verify exact retained and exposed regions, original
source intervals, paired interface directions and independent area and
volume. The integrated transition and sewing race passed in 4.174 s.

### Rounded footprint line qualification

Independent exact secant parameters and quadratic signs verify rational
and irrational corner contacts. Tests distinguish a single corner tangent,
a supporting edge span and an open material interval. The line scale and
original corner and edge IDs remain intact. Insufficient bracket precision
returns no partial clip. The combined focused race passed in 1.271 s.

### Full native CI partition

Intersection has a separate job because its measured full race run lasted
71.7 minutes. The three longest measured trimmed fixtures also have separate
jobs, with the remaining tests split across six deterministic groups. A local
inventory comparison covered 1,123 top-level tests, examples and fuzz seed
entries exactly once. The three original complete-group selectors still
produce the same combined inventory. Hosted jobs are unverified until run.

### Closed crossing native prism qualification

Single and multiple positive cycles close under both axial entry signs.
A sheared U-profile cutter retains three upper regions and two floor
regions joined through the lower stock. Independent volume, centroid,
Euler characteristic and original cap/wall ancestry are checked. The
integrated crossing prism race passed in 1.942 s.

### Anisotropic terminal cap line qualification

Horizontal terminal-plane clips retain closed cap material spans, tangent
and singleton contacts, original rim and wall ties, and no volume-interior
intervals. Exact lower-degree sector guards are normalized before root-sign
proof. The combined cap and wall line race passed in 37.879 s.

### Original holes in native crossing prism Cuts

Exact constrained cap triangulation retains both stock and tool hole
incidences, including a retained tool-hole island. Independent mass,
centroid and Euler checks cover retained and floor holes. Original hole
wall edges and parameter ancestry survive. The integrated crossing-prism
and generator race passed in 12.595 s.

### Original anisotropic wall generator qualification

The selected original wall generator is proved by constant exact normalized
coordinates and weighted profile membership. Tests preserve periodic seam,
cap, crease, rim and vertex ancestry through native reload and reindexing.
Near but distinct directions reject at sub-ULP separation. The combined
crossing-prism and generator race passed in 12.595 s.

### Shared weighted station STEP qualification

Pinned OCCT authored and re-read a single two-band weighted product with
two placements. Original and generated files retain the shared product,
eight faces, original spline coefficients, placed volume about 56.115992
and surface area about 178.858999. The source and regenerated assembly
live race passed in 24.015 s.

### Exact rounded prism supports and placed line qualification

The exact 3-4-5 footprint extrusion preserves one original wall per side
or rounded corner. Original rational evaluations satisfy the circle
identity, and nonbinary controls survive native persistence. Signed-axis
placement preserves line parameters, cap and corner ties, singleton hits,
coincident generators and oblique material intervals. The combined focused
race passed in 2.729 s.

### Staggered crossing-prism Fuse qualification

Both original operand orders, exposed shoulders, original tool holes,
independent mass and centroid, exact orientation and atomic conversion
failure pass. The combined original transition, closed native Fuse and
anisotropic mesh race passed in 9.184 s.

### Unified native prism operations

Integrated race coverage exercised actual closed volumes and ancestry for
matching and unequal Common, staggered crossing Fuse and Cut, contained
cavity, blind and through Cut, boss Fuse, and exact disjoint and containment
outcomes. Unsheared boxes exercised axis selection. Contact, publication
limits, and cancellation checked atomic failure. The combined dispatcher,
legacy interval, and anisotropic area race run passed in 13.776 seconds.

### Nested crossing prism material

The strict nested Fuse and Cut tests passed on both relevant operand
orderings and on reflected placement. Independent volume and centroid,
closed incidence, original cap and wall orientation, holed profiles, and
atomic rejection of nonbinary native coordinates were checked. The focused
native and sewing race run passed in 6.008 seconds.

### Exact nonconforming straight border sewing

A box with intentionally different edge partitions and a reversed
original coedge sews at exact fraction 1/3. Original surfaces, endpoint
ancestry, signed volume, native reload, and continuous edge agreement
were checked. One-short work limits, one-ULP drift, cancellation, and an
independent point contact were also exercised. The straight and original
sewing race tests passed in 3.939 seconds.

### Full pure-Go and PlainCAD runs on the same pinned kernel

Commit 6c4ab7c40724f26fc721c26d1e2c8a4417537898 passed the complete
pure-Go suite without `-short`. All 51 packages with tests passed.
The 1,081 enumerated trimmed B-rep cases appeared exactly once across
the three shards. The full run recorded 4,567 top-level passes and 421
optional skips. The independent PlainCAD app snapshot passed development,
cross-browser, unit, and selected actual-WASM contracts. Production
passed all 66 tests after increasing only Chromium's DevTools network
capture buffers in the copied test dependency. The unadjusted 65/66
production result is also recorded. This evidence is bound to the pinned
Go and app revisions. See the [combined evidence](evidence/main-6c4ab7c4-qualification-2026-11-14.json).

### Crossing prism through Cut qualification

The native dispatcher and crossing-family race run passed in 6.612
seconds. Cases cover equal and enclosing tool intervals, disconnected
retained components, and a tool hole that leaves a material post.
Independent volume, source-support agreement, topology, operation quotas,
and rejection of nonbinary native vertices were checked.

### Abutting crossing prism Fuse qualification

Both operand orders give independent volume 16 and centroid
(3/2, 1, 17/8), with closed incidence and Euler characteristic two.
Tests confirm shared cap removal, original wall ancestry, and rejection
of isolated point and line contacts. The combined main race run for the
dispatcher, abutting Fuse, and anisotropic bore measures passed in
62.052 seconds.

### Exact nonconforming spline border sewing

A spline roof and wall with different subdivisions sewed into the
original closed shell. Tests checked complete curve identity, source
ownership, physical volume, native reload, and exact work limits.
A curve with the same endpoint anchors and a different interior was
rejected. Combined spline, straight, and original sewing race tests
passed in 4.355 seconds.

### Guided planar tolerance repair qualification

A cuboid's shifted and reversed top trim was repaired against its
original adjacent planes. The four vertex displacements and volume
were verified exactly. A subdivided two-plane edge retained its
projected midpoint, with both source displacements exactly at the
limit. Native reload and continuous agreement passed. Inconsistent
planes, duplicate seams, orientation defects without repair, and
collision or displacement quota exhaustion were verified. The combined
planar repair and abutting prism race tests passed in 4.871 seconds.

### Equal and nested abutting prism qualification

Tests passed for both operand orders, equal and nested footprints,
and retained annular passages. Independent volumes are 80 and 44 for
the filled profiles and 160 and 85 for the holed profiles. Cap areas,
wall ownership, closed incidence, and rejection of unsupported contacts
were checked in the integrated 4.871-second race run.

### Tied-cap and side-wall prism Fuse qualification

Both tie directions and operand orders passed independent volume,
centroid, topology, and contribution checks. The tied Common triangles
carry both authored cap IDs. The side-wall tests independently check
the removed shared area and surviving original wall fractions for
staggered and nested spans. The combined main race run passed in
42.309 seconds. Lower-dimensional contact and unrepresentable native
vertices were checked separately.

### Profile-covering Cut qualification

The exact retained-slab tests exercised one and two components,
original stock holes, reversed forming caps, tied heights, and
construction quotas. The pinned OCCT comparator also independently
verified volume, area, centroid, and topology for both slab configurations
under two placements. The combined comparator and native prism race
run passed in 5.971 seconds.

### Automatic planar seam qualification

Automatic discovery recovered every authored cuboid seam after a cap
trim shift. The repair preserved volume and the exact displacement
bound. A one-short work budget, missing counterpart, ambiguous near
edges, and an over-tolerance repair all returned no geometry. The
combined planar and exact subdivision sewing race run passed in
13.932 seconds.

### Independent native prism Boolean corpus

The OCCT comparator built inputs from authored profiles and heights and
ran its own Common, Fuse, and Cut. All 42 matched cases passed source
family routing, volume, surface area, centroid, solid and shell count,
and per-solid Euler characteristic. Cases include a shared annular cap,
an interior tool hole that leaves a connected post, a sealed cavity,
and two kinds of disconnected Cut. The same fixed corpus passed under
race instrumentation. See the [comparator guide](../tools/occtref/README.md).

### Higher-degree original coedge qualification

A reversed cubic B-spline coedge in knot units [2, 6] and a weighted
quartic Bézier coedge split an exact transverse section into two open
runs. Each original contact was checked against its independent
algebraic equation. XY and XZ projections, original face partitions,
opposite fragment uses, and native reload all passed. The legacy
quadratic contact and native knot-section race assertions also passed.

### PlainCAD application rerun on 0e57c2d8

The original PlainCAD revision efae4d2ffa6065875d3a76fc5926f358b5936a29
was rebuilt against kernel 0e57c2d854ff53c7ee523ef19b52c9b60255a9e8.
Build and type checks passed. Development passed 345 tests, Chromium,
Firefox and WebKit passed 15 tests, ordinary units passed 1,691 tests,
and selected actual-WASM contracts passed four tests. The ordinary
production run passed 65 tests and reproduced DevTools response
eviction in the CSP capture test. An isolated Chromium capture-buffer
adjustment passed all 66 production tests. The copied dependency bytes
were restored and checked. See [the staged-run evidence](evidence/plaincad-main-0e57c2d8-2026-11-14.json).

### PlainCAD application rerun on acf376e1

PlainCAD `efae4d2` built against kernel `acf376e1` in an isolated app
checkout. The WASM build, type checks and 500 kB bundle ceiling passed.
All 345 development, 15 cross-browser, 1,691 unit and four actual WASM
contract cases passed. The ordinary production capture reproduced the
single Chromium inspector-cache eviction. All 66 production cases passed
with the isolated driver buffers enlarged. The driver was restored and its
original SHA-256 was verified.
[Staged-run evidence and log archive](evidence/plaincad-main-acf376e1-2026-11-14.json).

### Original cap, trim and STEP source qualification

- The pinned OCCT comparator confirms the original convex polygonal cap
  contact and native Fuse in both operand orders. The exact output volume
  is `60639/4096`, with 36 faces, 54 edges and 20 vertices. Original UV
  `8/9` and its model witness survive native reload. Altering the exact
  p-curve midpoint with the endpoints fixed fails revalidation.
- Independently mapped nonorthogonal concave charts retain two slanted
  holes, all 16 exact source uses and area `11/16`. Holes contributed by
  both parents yield three retained holes, and clipping across an original
  concave hole yields two components of area `1/128` each.
- The nested rational AP242 writer round-trips filled and bored source
  material. The original document, regenerated STEP and native reload
  agree on the composed leaf placements and material properties with
  the pinned XCAF reader. A separate original AP242 fixture verifies
  twelve degree-two planar tensor supports and 48 quadratic p-curves.
  All original owner records survive native and STEP round trips.
  Interior pole and sub-binary decimal deviations fail exact identity.
- Independent cubic and mixed-degree AP242 originals passed STEP and
  native round trips with three solids, eighteen faces, volume 10,800
  and area 4,860. Genuine rounded oblique poles fail the exact identity.
- Independent exterior and nested rotated double conics passed OCCT
  material checks. The nested Common and Fuse volumes are `20207/20480`
  and `25/8`. Native opt-in dispatch found the sole original face pair
  and YZ projection after placement, and retained the original boundary
  and edge ownership. The combined race run passed.
- Certified polygon tessellation uses the original nonbinary homogeneous
  support `z=uv/3` with a concave outer and a slanted concave hole.
  Exact UV area, original use sample parameters, both face orientations
  and independent sagitta samples passed.

### Full pure-Go refresh on 4078f85a

The pinned source accounted for all 1,219 listed trimmed-solid cases in
three groups, with 1,204 passes and 15 optional skips. The original
package run found three independent AP242 inventory subtests missing
`SPECIFIED_HIGHER_USAGE_OCCURRENCE` in the STEP preflight catalog. Repeating
the entire STEP package on the pinned source with that one-line correction
accounted for the same test inventory. Together the runs passed 52
tested packages, 4,807 top-level cases and 8,188 cases including subtests.
The [qualification record](evidence/main-4078f85a-qualification-2026-11-14.json)
retains the original failure and the repaired package and trimmed results.

### Original cavity, crossing material and conforming tensor shell checks

The independent OCCT comparator returns two valid material solids for
exterior closed-contact Fuse. Their combined volume is `40707/20480`.
Nested Cut retains one solid with the original stock and cavity shells.
For the axis-aligned and rotated inputs the volumes are `43773/20480`
and `43793/20480`. Native reload re-proves the source contact and original
orientation. Exact spline rays through the native cavity verify preserved
material, removed material and the two opposite contacts on the ellipse.

Two independently authored oblique triangular prisms produce five exact
section junctions across all 25 original face pairs. Common, Fuse and
both Cuts match six valid OCCT results. The exact volumes are
`9675/8192`, `56373/8192` and `23349/8192` respectively.
The original native dispatch exposes retained and reversed faces, source
edge rows, section edges and generated subdivision edges.

Exact multispan polygon faces have two-sided error bounds, original UV
samples at the knot crossing and continuous shared sample identities
after native shell reload. All 24 original coedges participate in the
closed-shell fixture with volume `37/36`.
Independently authored AP242 tensor, UV and three-dimensional model
splines preserve their records and exact original bases through source
and native reload. Live OCCT and focused race checks passed.

### Linux geometry and downstream qualification

The complete geometry package reproduced four failures on the original
Go 1.26.0 Linux arm64 toolchain. The original source passed those same
cases when optimization was disabled for the diagnostic. One source
change retains the exact fixed-axis domain through triangular inversion.
The other passes scalar original limits from the certified seed into the
tube search. Both changes preserve the original proof and error checks.
The complete repaired package passed 945 top-level tests and had one
optional skip, using standard optimization.
[Original, diagnostic and repaired logs](evidence/linux-geometry-go126-2026-11-14.json).

The original convex paired-edge reverse Cut preserves two independently
closed material solids meeting at one exact original point. Native and
independent OCCT volumes are `7/32` and `77/64`. The forward Cut has two
circular link fans at one vertex, which the strict native result reports.
Multispan border sewing retains the exact original model split at
`1/3` with lossless original chart and output vertex coordinates. The
knot and coedge agreement are verified on native reload.

### Exact model rays, polygon link normalization, and curved STEP

The rational spline contact test retains original coordinates `1/3`
and `3/5`, finds the exact nonnormalized line parameter `7/6`, and
certifies negative crossing orientation. It also exercises an original
support edge and confirms a coefficient limit leaves a pending cell.
The original quartic cavity point query proves both occupancy signs and
nonbinary boundary height. An exact ray through the two shells returns
three correctly oriented crossings with the stock exit at parameter
two. The focused geometry and trimmed race tests passed in 2.533 and
58.424 seconds respectively.

The forward full-width polygon Cut retains 42 original triangles, 63
edges and 23 native vertex indices with Euler characteristic two. The
two indices at `(0,-1,0)` have circular links of four and six faces.
The exact all-pair proof classifies 861 triangle pairs. Native reload
preserves separate indices and the original source comparison repeats
the contact proof. The independent OCCT comparison returned one solid
of volume 27.296874999999996 and one vertex identity at that point.
The exact native volume is `1747/64`. Strict operations still reject
the pinched embedding.

The independent parabolic STEP roof preserves 24 original pcurves.
Source, native reload and generated STEP match OCCT volume 28, area
58.149412499881279 and six faces. A second authored source uses
quadratic pcurves with distinct quarter and half interior knots. Whole
original spline identities retain each ownership record, and the
combined main qualification passed in 12.861 seconds under race with
both reference oracles active.

The weighted curved seam fixture passed native reload with 420
triangles and no unexpected contacts. The independently knotted curved
seam passed with 288 triangles and 12 common seam samples. Both have
exact source volume `23/448`. Mutated later spans, an excessive span
count and exhausted work allowances reject before publishing a mesh.

### Convex miter, thick-solid, and authored spline qualification

The independent `3-4-5` triangular prism has volume 30. The outward
and inward normal offsets of `1/2` each retain five faces and agree
with OCCT volumes 81 and 6 and surface areas 135 and 27. Eroding the
clipped-corner prism by `3/2` removes the corner facet at exact edge
contact and leaves a six-face box of volume two. Further collapse to
a segment returns empty material. The direct OCCT offset at the exact
extinction event retained a seventh degenerate face and failed its
shape validity check. The native tilted stock with an open top gives
one nine-face shell, exact volume `93/4` and area 99. This agrees with
the independently authored OCCT thick solid. The full planar BRep
package passed under race with the OCCT comparator enabled.

The exact rational query classifies the shared vertex `x=1/3` on its
original support. Its stored float64 display position is outside that
stock and both point-query APIs preserve the distinction. The weighted
seam ledger retains an independently evaluated original raw parameter
on both owners, including reversed traversal. Native reload and closed
meshing passed with 420 triangles and 23 shared seam samples. Splitting
a distinct cubic seam at its exact original station retains the source
poles and knots on both child ranges. The reloaded closed display has
288 triangles, no unexpected contacts, and the same `23/448` source
volume. The standalone main seam split race passed in 9.045 seconds.

The bicubic AP242 roof measures volume 90 with centroid
`(159/50,159/50,359/280)`. Independent OCCT measured area
139.72366383868012 and six faces. The shared-speed rational roof
independently checks positive source ratios two and one half. Each
model edge and both pcurves satisfy the same exact homogeneous map.
Both source replays and generated output returned the original six
faces, volume 28 and area 58.14941249988127. The focused rational
source suite passed in 14.304 seconds under race on main.

The unequal-setback convex chamfer uses setbacks `1/2` and `5/8` on
the original acute oblique edge. Both source-face orders give six
facets, volume `945/32`, and area `1089/16`. Pinned OCCT returned a
valid matching solid with the oblique reference face. Its opposite
reference-face order returned an invalid solid in the control. The
closed inward cavity has two oriented shells and volume 24. The
opposite-cap opening has two lips on one connected shell and volume
`45/2`. Both area-99 outputs match independent OCCT constructions.

The rotated elliptical graph Boolean uses original knot partitions
on different axes. Native reload re-proves the retained original model
edges for all operations and both operand orders. Independent OCCT
volume differences were below `6e-11`. Two disjoint circular loops on
one original roof produce two independent four-edge lens boundaries,
while Common and Fuse retain both holes on the original exterior roof.
The independent OCCT comparison returns the same two forward Cut
solids and volumes within `2e-10`. The exact UV fallback completes the
reverse Cut with six faces, four inner wires and Euler characteristic
`-2`. Its native volume is 0.08930865706060948. The pinned OCCT
result has one solid and volume 0.08930865722429136. Native reload
re-proves the exact model-edge ownership and near-collinear wall.

The nonplanar AP242 `BREP_WITH_VOIDS` fixture has two original
six-face spline shells. Original source, source-bound native reload and
generated STEP retain inward orientation and strict containment. The
independent OCCT comparison reports volume `441/16`, area
61.783750781123864 and 12 faces. The focused main STEP cavity race
passed in 37.133 seconds.

The [completed PlainCAD record](evidence/plaincad-main-35c79311-2026-11-14.json)
retains all original and corrected browser results. The configured
development run passed 345 tests. Production passed all 66 with the
unchanged Playwright driver and in-flight WASM response hashing. The
original inspector-cache control and the test-only repairs are recorded
with the WASM, driver and archive hashes.

The original acute prism edge fillet has exact radius `5/16`, tangent
locations `(49/16,0)` and `(13/4,9/16)`, and center `(49/16,5/16)`
in its original profile. Four analytic cylinder charts and their cap
trims survive native reload. The independent OCCT fillet returns one
solid of volume 29.145041881053835 and area 66.186284791166074.
The native mass integrals agree within `1e-7` and independently agree
with the analytic cross-section. Circular evaluation retains the
bounded endpoint tolerance of the analytic arc extrusion.

Independent OCCT checks qualify two- and three-loop Boolean results
under both original U and V knot partition searches. Three-loop Cut
returns three components and volume `3π/2097152` for the original
inward choice. Reverse Cut returns one connected component with Euler
characteristic `-4`. Native reload re-proves owner identity. The V-knot
and weighted speed-map suites passed together under race in 39.665
seconds. A mapped weighted seam keeps 288 triangles with zero contacts,
including after source-bound native reload. Reversed traversal and ten
nonlinear source stations agree with independent exact evaluation.

The unified nonplanar STEP reader passed all six authenticated source
families and live OCCT checks under race in 42.469 seconds. The prior
multiple-void suite independently tested two, three and four voids,
with 18, 24 and 30 faces. The source-bound proofs preserve nesting and
owner constraints on write and reload.

The ordinary PlainCAD bridge qualifies grouped axial chamfers on the
oblique `3-4-5` triangular prism in XY and YZ. Two adjacent original
corners remove a combined volume of `25/64`. Native output retains
the unrelated original vertical edge and agrees with exact sectional
volume and analytic surface area. The oblique fillet also survives
STEP export, import, and byte-identical canonical re-export on both
axes. Independent OCCT transferred the canonical analytic file and
matched the fillet's volume, area and one-solid boundary. The focused
STEP and bridge suites passed under race with the live reader enabled.

The [full ordinary macOS qualification](evidence/main-35c79311-qualification-2026-11-14.json)
on pinned `35c79311` passed all 52 packages with tests. The logs account
for 4,923 top-level passes and 460 optional skips. The three independently
listed and disjoint trimmed inventories contain 1,290 cases. All were
accounted for, with 1,266 passes and 24 optional skips. The record includes
the archive and per-lane log SHA-256 checksums.

The U and V original parabolic roof contacts pass live OCCT checks
under race. In the piecewise test, the two source branches meet at a
true C0 knot and keep separate one-sided tangents. A separate original
refinement produces a third exact coedge. All six operations and
operand orders pass native reload and reproduce the independent OCCT
volumes within `2e-9`.

The two-span speed test uses a raw quartic with a real multiplicity-four
knot and separate Möbius maps. The model station at the original source
knot appears under one vertex ID for both original owners. The
native v10 reload retained a closed 288-triangle mesh with zero contacts.
Changes to the raw source, either map, the station or the versioned
payload fail the focused proofs. The same runs exercise the older v9
single-span branch and its source-preserving split/merge.

The periodic tests compare exact homogeneous curve controls against the
independently evaluated cyclic cubic basis across two seams and nonbinary
knots. Plane hits include an irrational crossing, a double tangency, a
coincident span and separate incident seam owners. Distance bounds retain
an exact witness on cycle `2^100`. Certified chord bounds contain the
rational basis samples and preserve the original knot cuts. The one-operation
short cases exercise partial distance and tessellation output. These tests
passed under the race detector.

Focused Go WASM tests passed the original oblique axial fillet, STEP
round trip and grouped chamfer on XY and YZ. The standard Go launcher
initially rejected the inherited environment size. The rerun used a
minimal environment and the same Go WASM test binary and assertions.

The [full Linux arm64 qualification](evidence/main-35c79311-linux-arm64-qualification-2026-11-14.json)
uses the pinned Go 1.26.0 image and the original `35c79311` test
inventory. All three original trimmed groups passed. The complete geometry
package rerun with `c855ecaa` closed four failures in the original output.
After replacing that package's results with the corrected rerun, the record
accounts for 4,923 top-level passes and 460 explicit optional skips across
52 tested packages. The archive includes the original log and the exact
correction patch. The corrected geometry and native callers are also
recorded independently.

The [full PlainCAD run on `42ab9727`](evidence/plaincad-main-42ab9727-2026-11-14.json)
passed build, typecheck, four WASM contracts, 345 development cases, 66
production cases, 15 cross-browser cases and 1,691 units. The two test
fixture patches, original configured origins, pristine-driver hash and
matching production WASM hash are recorded. The 72 JavaScript bundles
passed the 500 kB limit, with a largest bundle of 496.50 kB.

The original cubic and quartic graph fixtures passed all 24 combinations
of degree, chart axis, operand order and Boolean operation. The original
dependent knot at `5/16` crosses the contact at an irrational parameter.
Strict exact endpoint evaluation keeps the original representable wall
vertex. The regression that perturbs its model and chart coordinates by
one ULP is still rejected by continuous edge agreement. Both independent
OCCT fixtures agree with the native and analytical volumes.

The two-span seam repair tests cover cuts in both original intervals and
at their common knot for both traversal directions. Native reload retains
the full original curve and uses the existing single-span representation
when a knot cut separates the owners. All 12 split and merged meshes
are closed and contact-free. The focused race suite also passes.

The original two-fillet STEP input uses two oppositely oriented circle
frames on one cylinder. The grouped bounded repair independently
revalidates the complete model and both coedges for each changed arc.
Live OCCT original, corrected and generated models agree on one solid,
seven faces, volume, area and centroid in original and rotated poses.

The independent default OCCT single-fillet source retains 24 owner
pcurves. The tests verify every owner lift, the source-bound native
reload, and rejection after owner, direction or model changes. One
cylindrical parameter needed a small rigorous trigonometric interval.
The qualified tolerance accepts the proved lift and a tighter tolerance
rejects it. The original, corrected and re-exported solids pass live
OCCT transfer and the focused race suite.

Periodic surface tests use independent rational tensor basis evaluation
across both seams. Mixed-axis extraction retains an ordinary knot and
the cyclic owner on cycle `2^100`. Native reload preserves the original
pole and weight data, including period and parameter units. Exact grids
reject out-of-domain, past-budget and altered input records. The focused
geometry and parametric suites passed under race.

The two ordered curved-contour cases qualify native Boolean results on
both original chart axes and in both operand orders. All 12 combinations
pass exact owner checks and reload. Independent OCCT checks from the
2026-11-14 run agreed within `2.68e-9` in volume. The forward Cut has two
components, while the reverse Cut has one. The exact side-wall split
handles the middle strip where its two roof endpoints coincide.

The piecewise boundary fixture retains exact agreement between original
owners whose common model curve changes polynomial at a knot. The native
boundary recipe re-proves both owner charts and reproduces the certified
sample ledger. Its curved single-face fill retains all 19 prescribed seam
vertices and reports no nonincident triangle contacts. Independent
samples check the original source parameters and the `z=xy` support.

Spherical cavity and separated-sphere offsets pass exact radius, face
ownership, tangent rejection, regularized collapse, and analytic mass
checks. The cavity survives signed offsets and can fill or open through
the outside sphere. Independent simple OCCT offsets matched the retained
cavity. Separate OCCT sphere Booleans verified the transition supports.
The OCCT comparator uses adaptive mass integration for the trimmed
transverse case. The first nonadaptive reference integration exceeded
the comparison allowance on that case despite the native result matching
the analytic formula.

The fresh 2026-11-15 focused race run passed the ordered-contour, piecewise
boundary and face-fill, and spherical offset cases in `geom`,
`brep/parametric`, and `brep/trimmed`. Three optional live comparisons
were skipped in that run while the pinned OCCT reference was rebuilding.
The run log is under `.worktrees/qualification/2026-11-15` in the workspace.

### Additional November 15 focused checks

The rebuilt pinned OCCT reference at `4dbfdcc448c0b3e565488afcd626c84483172db9`
qualified the spherical cavity, separated sphere, and lens-erosion comparison
cases under race. Transverse lens erosion keeps the two original sphere supports
at exact shifted radii and regularizes the point of collapse to empty. Outward
lens dilation separately checks equal and unequal radii against the analytic
Steiner formula. It retains the original contact circle and two spherical
caps joined by a torus belt, with certified display and native reload for the
represented circular rims in scope.

`f80ee663` passed the 105.624-second focused trimmed race run. Two nonlinear
roofs, the original affine walls and the floor share one seam ledger. The
normal 1,742-triangle mesh and the different-budget 2,714-triangle mesh both
have no nonincident triangle contacts. The tighter owner adds a refinement
round to the shared seam rather than creating a second model-edge sampling.

`9f8eec67` passed all three native transverse-node tests under race in 40.523
seconds, including the live OCCT comparison. The eight Common/Fuse cases span
both graph axes and operand orders. Each preserves four exact paired coedges
meeting at one original vertex and reloads as one closed Euler-2 shell.
Changed source trims, nonrepresentable node coordinates, and the point
contacting Cut remain rejected by this entry point. Pinned OCCT returns two
solids for the Cut, which needs a separate component contract.

`3eea7df3` and `39f5a619` passed the `heal/...` and `collision` race suites.
The sewing test reorients one cavity, an island inside it, and a disjoint
component with inconsistent input face winding; the strict classifier
validates the result and exact signed shell volumes give material volume 305.
The short-edge test contracts in both directions on an internal planar seam,
preserves unit volume and area six, and rechecks complete embedding. It
rejects a budget one ULP below the required distance, work exhaustion,
crease contractions, and contractions that erase faces.

The four live STEP repair comparator cases passed under race in 21.078
seconds, using the rebuilt reference and its standard Part 21 writer.

The PlainCAD bridge imports the five original default OCCT convex-arc
specimens only when explicitly enabled. It preserves the original signed
placement and repair proof, prepares a closed mesh, and regenerates a
canonical file whose normal strict import is byte-stable on export. The
first implementation's complete native race exceeded its 12-minute budget
in the translated two-fillet case. After `380f5ed7` replaced the generic
preview refinement with exact signed-frame profile and sagitta checks, all
focused main races passed in 165.712 seconds. An isolated Node WASM smoke
reduced the mesh phase from over 132 seconds to under 40 milliseconds.
The full focused Go WASM tests on the pinned `f5bfaac2` snapshot passed
in 72.263 seconds. The earlier launcher attempted to start two WASM runtime
threads; the staged rerun used one and verified all test assertions.

The complete mapped-shell recipe is 5,675 bytes and stores original source
data only. A closed-source tessellation is reconstructed after reload;
malformed loop pairing and an interior control change on the planar floor
are rejected. The separate continuous embedding certificate rejects a roof
that crosses the floor while preserving every seam. Its restricted native
Solid roundtrip matches independently authored pinned OCCT: 7 faces, 17 edges,
12 vertices, volume 0.890625, area 5.82544462100276 and the centroid. The
reloaded native material preserves exact rational point locations, seam
incidence and line crossings. Focused race logs are in the qualification
directory above.

The native point-contact Cut race on `bd14f09d` passed in 251.371 seconds,
including independently measured pinned OCCT component volumes within
`2.8e-11`. Both axis and operand orders return individually embedded positive
shells, exact node ancestry and a source-only reload certificate. The generic
collision checker still reports their shared point when the two displays are
joined without the component contract.

Large lens dilations in `acbcc2b1` retain only the regular outer rolling band
of the underlying horn or spindle support. The race tests reject reflected
quartic roots and retain an exact fourth-order meridian endpoint contact.
Native reload, deflection, conservative bounds, rays and independent parallel
body volume/area checks pass at the original rim radius and at three times
that radius. `f5bfaac2` additionally passes exact point classification across
the original cap/rim joins and on opposite sides separated by one ULP. The
point proof supports an oblique irrational rim without a native floating
support. Vet and the Windows amd64 cross-build on the pinned snapshot passed.

The [November 15 complete PlainCAD run](evidence/plaincad-main-48b0ce58-2026-11-15.json)
passes all eight gates on provider `48b0ce58` and app `efae4d2`. The reports
account for 345 development, 66 production, 15 cross-browser, 1,691 ordinary
unit and four selected WASM cases with zero failed, skipped or flaky cases.
The generated WASM matches the production asset, the browser driver matches
its pristine hash, and all 72 JavaScript bundles pass the 500 kB budget.
The manifest identifies both already documented test-only corrections and
the durable workspace archive.

The exact lens line tests (`2cd46dba`) include rational cap-to-rim joins,
isolated tangency, clipped interior ranges, both operand orders of an unequal
sphere pair whose radical plane lies beyond a center, and exact algebraic
contact parameters for an oblique irrational rim. `b0752ce1` independently
verifies area and volume against pinned OCCT rolling offsets of 0.5, 4 and 12.
The reference returned one closed shell of three faces at every distance;
the comparator validated closure, made an oriented solid, and checked it
with OCCT's standard analyzer before measuring.

The two-node original arrangement (`6b45e67e`) passed its focused race
suite, with two exact transverse vertices, six original branch pieces,
five signed regions, both chart axes and both operand orders. Its native
admission and representability gate is separately exercised. The rational
tilt STEP reader (`c491b216`) passed the standard convex-arc focused race
suite in 32.811 seconds with the live pinned reference. The single-fillet
source and translated source preserve original world placement, 24 original
owner claims, and eight independently bounded finite planar owner intervals.

The [November 15 full Go run](evidence/main-48b0ce58-qualification-2026-11-15.json)
passed 5,143 of the 5,646 pinned top-level cases, with 503 documented optional
skips, 52 tested packages and four packages without tests. The independently
listed 1,398 trimmed cases account for 1,357 passes and 41 skips. The original
group-zero prefix includes the full 1,349.190-second triple-radial test. Four
complementary lanes verify all 231 cases after that prefix against the same
revision. The logs and selection manifests were reconciled with zero duplicate
or unaccounted cases. Other groups and the package lane use the original run.
The original unsplit group zero later finished independently with the same
272 passes and eight skips. The primary run's full archive is included in
the manifest with the same inventory and totals.

`ebc3d1f7` qualifies native Common and Fuse for two original transverse nodes.
Both narrow and wider fixtures pass native construction and exact source
reload across each axis and operand order. All eight wider live cases return
one valid OCCT solid with volume errors under `4.8e-11`. The narrow reference
diagnostic returned no solid; independent native volume and topology checks
remain recorded separately. The main focused race passed in 86.728 seconds.

`ed815c5e` writes and reads an exact 21,713-byte mapped shell file. It keeps
17 model curves and 34 paired pcurves, including the two whole source spline
bases and their six trimmed uses. The original supports, lifts, and speed
maps are re-proved on import. Canonical re-export is byte-identical; the
native focused races reject changed model interiors, direction, owner and
vertex identities. The live pinned OCCT transfer returns one valid solid,
seven faces and the expected volume, area and centroid.

The tilted grouped STEP race passed in 33.390 seconds with independent live
OCCT readback. The 440-entity default file preserves its exact pose and proves
eight model changes, one bounded unequal axial endpoint repair, eight finite
planar owner ranges, and both original rational cap trims. The original and
corrected world shapes transfer as seven-face solids with matching measures.

`651f50cb` proves area and volume without tessellation using the exact
positive cap heights and the source rim normal angle. The focused race cases
check irrational oblique centers/radii, the obtuse angle branch, work and
precision limits, and matching native integrals. `6791dc9a` binds the source
circle, line solver, measure proof and native represented-radius assembly to
the same exact original sphere equations.

The two-node Cut contract preserves either two shells sharing both original
nodes or three positive shells joined by a chain of two exact contacts. The
wide pinned OCCT comparator matches independently clipped original slabs to
within `3.1e-11`; the whole-stock reverse OCCT grouping differs by chart
axis, so it does not qualify the native point-contact grouping. The main
two-node and earlier single-node Cut races passed together in 346.376 seconds.

The original OCCT mapped-shell author emitted separate standard AP242 graphs
with invariant exact model geometry, including a reordered shell and rotated
roof wires. The focused main race passed in 6.823 seconds for the recorded and
two live authoring cases, exact planar chart transfers, owner mutations, and
canonical STEP reopen. The first helper build lacked its optional STEP flag;
the successful run uses the documented STEP-enabled build.

The lens display recipe keeps the original irrational rim and continuously
bounds the model chart deviation, before reserving that allowance from the
mesh deflection budget. The source-only native record re-proves all bounds
and regenerates a closed mesh. Its race tests reject oversized input, changes
to the original source or witness and unfinished arithmetic.

The tilted STEP native proof race passed in 26.402 seconds, including actual
single, translated single and grouped OCCT specimens. The original STEP is
byte-identical on reload; all repair and owner certificates are recomputed
before being compared with the stored source proof. Pinned OCCT transfers the
corrected world geometry after the same reload.

The novel planar component assembly certificate passed `collision` and all
`heal` race suites. A concave prism and an independent diamond prism share two
distinct original vertices and no other boundary. The verifier proves exact
volume `29/2`, and rejects an incomplete ledger, duplicate claims, edge/face
touch, crossing interiors, hidden containment, inward shell and work limits.

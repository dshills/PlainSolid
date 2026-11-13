# Implementation status

The full OCCT replacement is **not complete**. This document distinguishes the
implemented, tested kernel from the much larger specification. No production or
OCCT compatibility qualification is claimed.

## Immediate priority: PlainCAD compatibility

Development now prioritizes the operations exercised by PlainCAD's existing
acceptance tests before expanding unrelated OCCT domains. The full specification
remains the eventual target. See [the prioritized plan](IMPLEMENTATION_PLAN.md)
and [PlainCAD readiness](PLAINCAD_READINESS.md) for application-specific gates.

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
while retaining annular caps and an unchanged outer wall.
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
cylinder end rims have exact toroidal rolling blends. Complete circular
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

## Continuing implementation

For immediate work, follow the PlainCAD gates above. Within the remaining full
replacement scope, the critical path is general curved Boolean assembly. The current exact planar
BSP can serve as an independent topology oracle for planar cases; curved
surfaces must keep their geometry through the operation. Continue with:

1. Expand exhaustive tube discovery and branch enumeration over independently
   parameterized splines. Mixed-axis junctions and tube-union coverage are
   available as proofs; automatic discovery of many closed curves, tangent
   candidates and partial coincidence beyond the shared-graph knot arrangement
   still require work. Extend
   complete transverse arrangements beyond rectangular input supports and
   handle curved edge-edge crossings, tangencies and coincident intervals.
2. Extend periodic and singular chart proofs. Sewing supports canonical seams,
   circular closed edges and collapsed poles of selected analytic primitives.
   Extend that work to general periodic spline faces, arbitrary inner loops,
   and embedding certificates on broader input trim domains.
3. Classify and reassemble exact curved fragments, return generated/modified
   entity ancestry, and add adversarial OCCT differential fixtures.
4. Extend general fillets, offset, draft and sewing after Boolean assembly;
   use the same topology for curved STEP/IGES B-reps and assembly metadata.

The broader qualification, packaging, and policy requirements in the
[specification](SPECIFICATION.md) remain in scope.

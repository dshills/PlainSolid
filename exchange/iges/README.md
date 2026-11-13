# IGES geometry exchange subset

`exchange/iges` reads and writes actual fixed-record ASCII IGES geometry. It
supports Cartesian points (type 116, form 0), bounded line segments (type 110, form 0), constant-Z circular arcs and
full circles (type 100, form 0), and rational B-spline curves (type 126, forms
0–5), and rational tensor-product B-spline surface patches (type 128, forms 0–9).
This is a geometry subset, not general IGES CAD/topology support.

## APIs

- `Read(io.Reader, Limits) (Model, error)` imports supported entities.
- `Write(io.Writer, Model) error` emits deterministic IGES 5.3 ASCII records.
- `Model.LengthUnit` is meters per raw coordinate unit. Coordinates are not
  rescaled. Supported units are inches, millimeters, feet, miles, meters,
  kilometers, mils, microns, centimeters and microinches.
- Each `Entity` has exactly one of `Polyline`, `Point`, `Line`, `Arc`, `Spline` or `Surface`, plus a directory
  label, subscript and form number. Optional `Transform` preserves a referenced
  type-124 affine placement; `Entity.Eval(t)` applies it while mapping normalized
  parameters onto the curve trimming interval.
- `Arc.Eval` accepts a normalized parameter in `[0,1]`; `Sweep` returns its
  counterclockwise angle. Equal endpoints encode a full circle.
- `Spline.Curve` is the immutable rational `geom.BSpline`; `Start` and `End`
  preserve the IGES trimming interval. Evaluate only within that interval when
  using the imported entity. Planar/closed/polynomial/periodic flags are retained;
  periodic metadata does not introduce wrapped extrapolation. Planarity and
  polynomial weight consistency are checked, but the closed/periodic metadata
  alone is not a proof of geometric closure or periodic continuity.

```go
model := iges.Model{
    LengthUnit: 0.001,
    Entities: []iges.Entity{{
        Label: "EDGE",
        Line: &iges.Line{
            Start: geom.Vec3{},
            End: geom.Vec3{X: 10, Y: 20, Z: 30},
        },
    }},
}
err := iges.Write(w, model)
```

Callers own their streams and must close files. No external dependencies are
used beyond PlainSolid geometry and the Go standard library.

## Record and validation contracts

Readers require 80-column ASCII records, with LF or CRLF line endings, ordered
Start/Global/Directory/Parameter/Terminate sections and consecutive section
sequence numbers. Directory entries occupy two records. Parameter pointers,
parameter back-pointers, claimed line counts and termination counts must agree.
Overlapping or unclaimed parameter records are rejected.

Hollerith strings, declared punctuation delimiters and decimal real values with
E or D exponents are supported. Units must have consistent explicit standard
flags/names, and model-space scale must explicitly equal 1. Directory labels are
preserved; presentation metadata such as positive line-font/color numbers is
not transferred. Associativity/property pointers must be absent or zero.

B-spline control points, expanded knots, positive weights, degree, trimming
interval and normal are validated. Planar normals must be unit length within
`1e-8`, and normalized control-plane residuals must be at most `1e-10`. Arc endpoint
radii must agree within relative `1e-10`; angular distinctions below float64
resolution fail instead of becoming a full circle silently.

Zero-valued limits select 64 MiB, one million records, 100,000 entities, four
million scalar/string values and one million spline control points. Parsing is
buffered within the byte cap. These are resource-count limits, not execution
deadlines; use application deadlines and tighter limits where appropriate.

## Surface patches

`Surface.Geometry` stores an immutable `geom.BSplineSurface`; `UStart`, `UEnd`,
`VStart`, `VEnd` preserve a rectangular interval inside its knot domain.
`Entity.EvalSurface(u,v)` maps normalized parameters and applies affine placement.
The decoder explicitly transposes IGES U-fast control nets to Go geometry order.
Control-point limits count curves and surface nets together. Closed flags are
metadata rather than closure proofs; periodic flags are rejected. Polynomial
flags require equal weights. Boundary trimming, holes and shell topology are
not represented by type 128 alone and remain unsupported.

## Unsupported domains

Other entity types—including other surfaces, topology, annotations,
assemblies and referenced style entities—fail explicitly. Nonzero directory
structure/view/label-display/status fields, infinite line forms,
user-defined units, non-unit model scaling, binary/compressed records and
non-ASCII text are unsupported. Arcs use a local XY plane; referenced type-124
matrices permit world-space placement. Nested, singular, unreferenced or nonzero-form
transformation matrices are rejected. No geometry healing or silent
entity skipping is performed.

## Verification and references

The independent fixture `testdata/curves.iges` encodes a line, quarter-circle arc
and rational quadratic quarter-circle directly from mathematical coordinates.
Tests verify geometry against analytic values, trimmed intervals, all supported
unit flags, custom delimiters, pointer/record failures, malformed numbers,
resource limits, I/O errors and roundtrips. Fuzzing targets the bounded reader.
Optional pinned-OCCT tests verify both directions for a line, circular arc and
rational quadratic curve, including an OCCT-exported translated arc. They compare
sampled coordinates and independently validate exported curves and total length.
Enable with `PLAINSOLID_OCCT_ORACLE=/path/to/oracle PLAINSOLID_OCCT_IGES=1`.
Broader CAD application/fixture qualification remains outstanding.

The implementation follows the primary descriptions in
[NASA's IGES geometry exchange specification](https://ntrs.nasa.gov/api/citations/19950005054/downloads/19950005054.pdf?attachment=true),
[NIST's global-section mapping](https://www.govinfo.gov/content/pkg/GOVPUB-C13-0b5a50289017f430ec68d41e957a5683/pdf/GOVPUB-C13-0b5a50289017f430ec68d41e957a5683.pdf),
and the [NIST IGES version 4 reference](https://www.govinfo.gov/content/pkg/GOVPUB-C13-7b81ba8b0f709555f162cb496aa63b3b/pdf/GOVPUB-C13-7b81ba8b0f709555f162cb496aa63b3b.pdf).
No external source code or fixture was copied.

Surface tests include independent bilinear-plane parameters and a rational
quarter-cylinder net, trimming, affine placement, malformed dimensions and limits.
Type-128 mapping was checked against the pinned OCCT IGES surface translator.
Both directions also pass the optional pinned-OCCT surface oracle for a rational
quarter cylinder of radius 1 and height 2: one valid face, area pi within `1e-6`,
and sampled imported points on the analytic surface. This qualifies that patch;
it does not establish arbitrary trimmed-surface or solid interoperability.

## Exact planar curve-on-surface records (type 142)

`Model.CurvesOnSurfaces` retains explicit indices into `Model.Entities` for the
support surface, UV spline and model-space spline, plus creation/preference
metadata and label/subscript. Both Read and Write support this narrow graph.
Referenced geometry remains available in Entities; subordinate ownership and
all directory references are checked. Writers mark support geometry subordinate
and type142 as parametric-surface geometry, so it is not emitted as unrelated
root shapes by downstream readers.

The accepted support is a nondegenerate affine bilinear type128 patch on unit
U/V knot domains, with equal weights. Its UV and model curves must be type126
splines with identical degrees, knots, positive weights and trimming intervals.
UV poles must lie in the patch rectangle with Z=0. `ValidateCurveOnSurface`
proves every model pole is the affine image of its UV pole using exact rational
arithmetic on the stored float64 values; even a one-ULP mismatch fails. No
sampling, fitted geometry or correspondence tolerance is used. Transforms,
nonaffine support surfaces and absent/alternate curve representations are
explicitly unsupported in this subset.

An independent mathematical fixture and a real OCCT-produced rational pcurve
verify import; exported curves match the independent OCCT fixture's edge length
within `1e-9`. OCCT's default quadrature differs from the analytic quarter-ellipse
length by about `2.11e-6` on both files, so that separate analytic comparison uses
`5e-6`. Pointer/ownership, bounds, one-ULP mismatch and malformed tests are included.
General rational-surface pcurve correspondence remains outstanding; type142 alone
does not establish an outer loop or hole. The polygonal type144 subset below
adds explicit boundary semantics.


## Polygon-trimmed planar faces (type 144)

`Model.TrimmedSurfaces` records the support entity, explicit outer type142 contour
and optional inner contour indices. Read and Write preserve that ownership graph.
Contours must use full-domain clamped degree1 rational splines with positive
weights and a repeated closing pole. Their segment loci are exactly straight;
smooth splines and circles are never approximated to make a trim acceptable.

`ValidateTrimmedSurface` checks connectivity, simple loops, nonzero edges,
noncollinear corners, counterclockwise outer winding, clockwise hole winding,
strict containment and pairwise disjoint boundaries using exact orientation
predicates and rational signed area. Touching/nested holes fail. At most
`MaxTrimVertices` (256) UV vertices are allowed across all loops. Type142's exact
affine planar UV/model correspondence proof remains mandatory. Natural-boundary
flags without an explicit outer contour and nonplanar supports are unsupported.

An independent square-with-hole model and an OCCT-produced type144 fixture pass
read/write qualification. OCCT validates one face with area 3 for both direct
Go output and imported/reexported geometry. Tests also cover wrong winding,
self-crossings, touching/outside holes, duplicate/dangling references, unsupported
boundary flags, and hole-free faces. `write-iges-trimmed` is the optional oracle
fixture mode; no shell/solid topology or curved trimming support is implied.

## Explicit rectangular rational spline faces

Type144 additionally supports a single counterclockwise rectangular type142
boundary on a rational type128 surface. Its UV and model contours are type102
composites with exactly four direct members: straight type110 UV lines and
rational type126 model curves. The UV edges must connect exactly at the four
corners of an axis-aligned nonzero rectangle within the stored surface domain.
Each model curve must agree identically with its exact extracted surface iso
curve, allowing affine parameter-domain normalization and reversal. Acceptance
uses rational polynomial identities, not samples or proximity tolerances.

`NewRectangularSurface(support, domain, metersPerUnit)` constructs this graph
while preserving the original support poles and knots. Exact extracted boundary
coefficients must be representable by the supported float64 spline records;
otherwise export fails with an explicit error rather than rounding geometry.
`Model.RectangularDomain(trim)` returns the certified original UV rectangle.
Read and Write preserve the composite references as `Entity.Composite.Curves`.
Standalone or nested composites, arbitrary pcurves,
nonrigid entity transforms and shell topology are unsupported. Each patch must also pass
a bounded global projected-injectivity certificate; unresolved patches fail.

The checked-in `testdata/rectangular-spline-face.igs` was independently generated
by pinned OCCT using a nonplanar rational bilinear graph and strict interior
bounds U=[0.3125,0.75], V=[0.25,0.75]. Its support remains the full original net,
and its model edges include reversed and shifted parameter intervals. Optional
`write-iges-bounded-spline-face` qualification regenerates it and checks both
imports with OCCT's face validity and area computation. Regressions include
one-ULP model/UV errors, malformed ownership/composite pointers, resource limits,
folded patches and unrepresentable exact boundary coefficients. The type126
informational planar normal uses the existing 1e-8 unit-normal angular precision
because OCCT exports that metadata with fewer digits; rectangular trim mapping
is still proved exactly.


### Lossless rigid placement

`NewPlacedRectangularSurface(support, domain, metersPerUnit, placement)` retains
an explicit type124 on the type128 support and each type126 model boundary.
UV lines and the composite containers remain untransformed. Read and Write
preserve the local poles, knot vectors and placement matrices, rather than
baking transformed coordinates. The exact homogeneous iso/model comparison
applies each entity's own placement, so independently baked model edges are
also accepted when their represented geometry agrees exactly.

This subset requires **exact proper rigid matrices**: the stored binary entries
must have exactly orthonormal columns and positive determinant. Rounded
approximate rotations, scale, reflections, shear and singular matrices are
rejected. This conservative restriction is deliberate: the pinned OCCT IGES
importer ignored a nonuniform affine placement on a type128 support while still
reporting a valid face. No nonuniform-affine interoperability is claimed.
A rigid transform preserves the already certified local patch injectivity.

The checked-in `placed-spline-face.igs` is generated through OCCT's entity model
with a shared type124 for a 90-degree rotation and translation. Live tests using
`write-iges-placed-spline-face` confirm both import directions, unchanged area,
and the independently transformed surface-area centroid. Negative tests cover
missing/mismatched support or boundary placement, transformed UV lines,
one-ULP translation differences, rounded rotations and nonrigid matrices.


### One interior rectangular hole

`NewRectangularSurfaceWithHole(support, outer, hole, metersPerUnit)` retains the
original rational support with two separate type142 contours. Each has paired
four-edge type102 UV/model loops. The outer rectangle winds counterclockwise;
the inner rectangle winds clockwise. Exact strict parameter inequalities prove
containment and separation from the outer boundary; touching, outside and nested holes are rejected. Every model edge still requires exact
iso-curve correspondence and lossless coefficient export. `RectangularDomain`
returns the outer domain and `RectangularHole` returns the certified inner one.
The outer patch's global injectivity certificate also covers the remaining face.

`testdata/holed-spline-face.igs` is independently generated through OCCT's face
and wire builders. Its original rational graph is z=x*y/4, with outer model
bounds x=[0,42], y=[0,2] and removed rectangle x=[20,36], y=[0.5,1.5]. Live OCCT
validates both directions; exported and original area/centroid agree within
1e-8. Independent analytic-in-x/Simpson-in-y integration gives area
342.9390573124855. OCCT's default integration differs by about 0.00194 in area
and 0.00066 in centroid distance, so the separate analytic comparisons use
0.003 and 0.001 respectively. These integration tolerances do not affect exact
trim matching. Regressions cover winding, reference ownership, duplicate contours, touching/outside holes, unrepresentable boundaries and one-ULP
model discrepancies. No arbitrary curved trim or shell support is implied.


### Multiple disjoint rectangular holes

`NewRectangularSurfaceWithHoles(support, outer, holes, metersPerUnit)` and
`Model.RectangularHoles(trim)` extend the explicit rectangular graph to zero
through `MaxRectangularHoles` (16) holes. Generic Read/Write now accepts that
bounded subset; `NewRectangularSurfaceWithHole` still constructs one hole and
`RectangularHole` still requires exactly one. The original support and source
contour order are preserved; each hole has its own paired type102/type142 graph.

Each hole requires the existing exact clockwise UV/model identity proof.
Strict coordinate comparisons prove containment and pairwise separation of the
closed UV rectangles. Overlap, nesting, edge/corner touching and duplicate
contour references fail. Outer-patch injectivity makes these parameter-space
separations valid on the surface. No contour is approximated or merged.

The independent OCCT `multiholed-spline-face.igs` fixture uses two separated
rectangular wires on the warped rational graph. Live
`write-iges-multiholed-spline-face` qualification preserves both holes and confirms
valid one-face topology with source/export area and centroid agreement within
1e-8. Separate analytic integration uses the previously documented OCCT
quadrature tolerances. Native round trips, all contact/overlap rejection cases,
count limits and the extended parser fuzz corpus are included.

Exact-certified spline trim readers reject source weights whose maximum-weight normalization requires rounding (for example, weights 1 and 3), even when the rounded support and boundary would compare equal. This applies to rectangular and holed faces, including composite model edges. Generic standalone spline geometry import retains its ordinary float64 normalization behavior; it does not establish preservation of exact source weight ratios. Such a certified trim input is explicitly unrepresentable in the current normalized spline representation.

`NewOrthogonalSurface` and `Model.OrthogonalBoundaries` support exact orthogonal
UV polygons, including concave boundaries, on injective spline supports. The
outer loop traverses counterclockwise; zero to 16 interior loops traverse
clockwise. Each loop contains 4–64 alternating horizontal/vertical edges, with
paired type102 composites of UV lines and exact model splines referenced by
142/144. Overlapping, nested, touching or outside holes, diagonal edges,
redundant collinear corners and any unrepresentable model coefficients fail.
Source normalization provenance and exact model/UV identities are checked before
certification. Generic `Read`/`Write` support this bounded graph;
`RectangularDomain` and `RectangularHole(s)` retain their four-edge requirements.
This is a standalone face subset, not a closed-shell reconstruction algorithm.

`NewQuadraticOvalSurface` and `Model.QuadraticOvalBoundary` add a bounded curved
trim on an affine bilinear type128 support with unit-square parameters. The
single CCW outer contour is one closed nine-pole degree-two rational type126
curve, four quarter spans with double interior knots and positive common quarter
weight at most one. Its axis-aligned UV control rectangle must lie strictly
inside the support domain. Disjoint quadrant interiors and exact monotonic
quarter controls prove simplicity and winding; no sampling accepts a loop.
Paired type142 model controls must equal the exact affine lift, including source
weight fidelity. The single-loop API rejects holes; reversed outer loops, arbitrary quadratics
and lossy lifted coefficients are explicitly unsupported. A general quarter weight defines a
rational oval; a rounded circular weight is not claimed to be an exact circle.

The independently authored OCCT `write-iges-quadratic-trim` fixture uses quarter
weight .75 on a 2×3 plane and has area 1.1905776125224596. Both directions preserve
one valid face and agree in area. Qualification caught an interoperability issue
in physical parameter records: decimal tokens split across 64-column boundaries
could silently lose trimming in OCCT. Writer parameter records now break at
commas, preserving complete numeric tokens; all existing live IGES tests pass.

`NewQuadraticOvalSurfaceWithHoles` and `QuadraticOvalBoundaries` support up to16
clockwise oval holes. A hole is certified inside by either exact concentric
homothety (identical quarter weight and a common strictly smaller axis scale),
or strict inclusion of its entire control rectangle in the outer oval's
inscribed diamond. Hole control rectangles must be strictly separated on at
least one axis. These sufficient exact tests deliberately reject other layouts
whose disjointness has not been proved. Duplicate, touching, overlapping, nested
or incorrectly oriented holes fail; paired UV/model correspondence is checked
for every contour. The old `QuadraticOvalBoundary` rejects any holed face.

The independent `write-iges-quadratic-annulus` OCCT fixture uses a half-scale
concentric hole and has exactly three quarters of the original profile's area.
Both-direction live qualification verifies that curved annulus, while native
roundtrips also retain two separated nonconcentric holes. The stored fixture
and parser fuzz corpus cover the new graph.

`NewQuadraticOvalSheet`, `CertifyQuadraticOvalSheet`, `ReadQuadraticOvalSheet`
and `WriteQuadraticOvalSheet` support exactly an affine quadratic-oval annulus
and a disk filling its sole hole. The two faces retain distinct support charts,
paired type142 curves and type144 records. Exact reversed whole-curve comparison
certifies the common model seam, including winding; the noncollinear common oval
also proves the support planes coincide. The existing simple-loop and strict
hole-containment certificates prove the interiors complement each other.
The result records both model-curve indices. It is a connected planar sheet
certificate, not an IGES shared-edge topology or material-solid claim. Extra
faces/entities, unowned contours, placements, mismatched units and inconsistent
model curves are rejected. Output is recertified before writing. Live OCCT reads
two valid faces and their combined area matches the independently authored outer
oval fixture, including a disk support with different UV scale.

The additive `QuadraticOvalFilledSheet` API extends this certificate to 1–16
strictly contained, disjoint oval holes, each filled by its own affine disk.
`NewQuadraticOvalFilledSheet` accepts a perforated base and independently charted
disk models in arbitrary order. `CertifyQuadraticOvalFilledSheet` determines the
unique disk for each hole by exact reversed whole-curve identity, retaining the
hole ordinal, face index and both model-curve indices in deterministic hole order.
All faces, curves and supports must be uniquely owned; duplicate/missing disks,
additional geometry and mismatched model units fail. Reader and writer use the
same proof. Existing two-face APIs remain strict. Live OCCT qualification covers
three faces, distinct UV chart scales and two different quarter weights; total
area agrees with the independent outer-oval fixture. This does not add arbitrary
knot-refined oval trim recognition or IGES shared-edge topology.

`NewCurvedOvalSurface`, `CertifyCurvedOvalSurface`, `ReadCurvedOvalSurface`, and
`WriteCurvedOvalSurface` provide an additive exact curved-face subset. One
biquadratic Bézier support on `[0,1]²` has a canonical rational quadratic oval
outer boundary and up to 16 clockwise oval holes. Each contour retains a full
multispan type-126 UV curve and a separate model curve, joined through type 142;
type 144 retains the boundary ownership. Every model quarter must equal the
exact rational composition of its UV quarter with the original support.
An exact XY-projection injectivity certificate proves the face is embedded;
existing conservative oval containment and pairwise separation proofs qualify
holes. This is a surface-face API, not a solid or shell importer.

The reader accepts model degree up to 16 and 128 poles per contour, subject to
normal input limits and bounded exact-arithmetic work. The constructor emits
losslessly representable composed curves and fails if required coefficients or
weight normalization would round. Rounded source normalization, transforms,
unowned curves, invalid winding, and uncertified containment are rejected.
Generic `Read`/`Write` retain their previous scope. The stored OCCT fixtures
`curved-oval.igs` and `curved-oval-annulus.igs` were independently constructed
using homogeneous Bernstein products (`tools/occtref/iges_curved_oval.hxx`).
Optional live tests validate source and regenerated faces and compare areas.

`NewCurvedOvalSheet`, `CertifyCurvedOvalSheet`, `ReadCurvedOvalSheet`, and
`WriteCurvedOvalSheet` retain an annulus and a disk on two distinct original
biquadratic supports. Each face receives the curved-oval certificate above;
the complete authoritative model curves at the hole/fill boundary must agree
exactly in opposite directions, including independently partitioned curves.
No extra curves, shared contour owners, or transformed entities are admitted.
The returned source indices describe certified geometric adjacency between
two type-144 faces. They do not claim IGES topological edge entities, global
pairwise interior disjointness, or closed material. The independent OCCT
`curved-oval-sheet.igs` fixture uses different chart domains and UV coordinates
for the same bowl, and tests both-direction validity, face counts, and area.

`ReadCurvedOvalShell` accepts a separate topological subset: one explicitly
open type-514 shell (form 2), two type-510 faces, three four-edge type-508 loops,
one type-504 edge list and one type-502 vertex list. An annulus and its fill disk
retain distinct curved supports and per-owner pcurves; the four inner edges
have actual shared source identities and opposite uses. Exact source endpoint,
loop connectivity, winding, complete UV/model composition and ownership checks
precede acceptance. Positive-weight UV control hulls must lie within each
source type-128 declared finite interval, which may be smaller than the full
support knot domain. The public graph retains source IDs and vertex/edge uses;
its `Sheet` contains derived full contours for inspection. Each face is proven
embedded independently; global face-interior intersection is not certified.

`WriteCurvedOvalShell` provides lossless, source-backed write-back, rechecking
geometry, incidence and all exposed ancestry before emitting any bytes. It
rejects edited cached geometry/IDs; it is not a general native shell builder.
The generic readers still reject these topological entities. The independent
OCCT fixture is sewn from the two curved charts and exported with B-rep mode.
OCCT's exporter defaults shell form to 1 even for this open source, so the
fixture generator explicitly applies `IGESSolid_Shell::SetClosed(false)`.
The reader requires form 2 and rejects contradictory closed declarations.
Live qualification checks one shell, two faces, eight unique edges/eight unique
vertices, validity, and filled area. No manifold-solid claim is made.

`NewCurvedOvalShell` now constructs this explicit topology from a certified
`CurvedOvalSheet`. It emits four losslessly representable quarter curves per
boundary, shares exactly four inner model edges and their vertices, preserves
both original supports/finite intervals, owner pcurves, length units and labels,
and validates the generated records through the strict reader before returning.
The disk's independently certified opposite model curve is represented by
reversed uses of the annulus's authoritative inner edges. Each emitted quarter
must have a single Bézier basis; unsupported interior knot partitions or
nonrepresentable split coefficients fail explicitly. This adds native graph
construction without broadening the shell's global embedding guarantee.

`ReadSplineLensSolid` and `WriteSplineLensSolid` implement a separate exact
closed material family: one type-186 manifold solid, one closed type-514 shell,
two oppositely oriented type-510 graph disks, and four shared conic quarter
edges with explicit loops and vertices. The two original unit-XY quadratic
spline support charts are retained. Their height difference must have exactly
the specified interior ellipse as contact, with upper above lower
inside and both above z=0. The existing exact native graph Boolean certificate
proves global embedding and positive material orientation; the decoder returns
that native `trimmed.Solid`. Closed edge incidence alone is never sufficient.

Each owner retains a quadratic rational quarter with asymmetric weights
`(.5,.5,1)` (an exact ellipse), and complete UV-to-model composition is verified.
The source must have canonical east-to-north counterclockwise quarter order;
the lower face is reversed through the shell face flag. Only one outer shell,
no voids, no transforms, no extra entities, and losslessly representable source
weights/coefficients are supported. The writer emits full unit-domain support
intervals after proving material and checks its complete graph through the
reader before output. Length units and support coefficients round-trip;
source IDs are inspection-only. This explicit solid API is separate from the
oval-sheet family and from generic IGES geometry APIs. Independent OCCT
source/native exports qualify validity, one solid/two faces/four edges/four
vertices and adaptive area/volume, including analytic volume π/512 for the
stored fixture.

The lens descriptor now also accepts `Shear`: its conjugate UV basis is
`(Radii.X,0),(Shear,Radii.Y)`, not principal ellipse radii. Exact rational pole
construction and the native quadratic-ellipse material certificate verify this
non-axis-aligned family. Independently refined continuous degree-two support
knots are retained (at most 256 control points per support); every full conic
quarter must stay within one original knot cell. `SourceKnotCells[owner][edge]`
records that source lineage. Source weight normalization must remain exact;
arbitrary raw weight scales that require rounding are rejected. Interior
quarter knot crossings and disconnected full-multiplicity knot breaks remain
unsupported. Separate independently authored OCCT sheared and refined fixtures
qualify original support bases, topology, and tight adaptive area/volume.

The document package provides `ReadSplineLensIGES`, `FromSplineLensIGES`, and
`State.WriteSplineLensIGES` for one identity-placed native material asset.
Dedicated IGES labels retain solid/shell/face/model-curve source IDs, both
owners' knot-cell lineage, and a binding to exact native geometry. The chain
survives native document save/reload and re-proves geometry and lineage before
export. Length-unit changes are supported; placements, extra occurrences,
changed material and stale provenance fail explicitly. No schema expansion,
STEP source identity, or IGES assembly is inferred by this adapter.

`ReadRationalSplineLensSolid` / `WriteRationalSplineLensSolid` form an explicit
raw-weight variant. `RationalSplineLens.Curves[edge][owner]` retains model,
upper-UV and lower-UV raw poles, knot vectors and positive weights independently.
Exact homogeneous splines prove canonical conic identity and complete model
agreement without maximum-weight normalization. Owner UV splines remain degree
two; model curves may be degree 1–16, each with at most 64 poles and continuous
clamped domain `[0,1]`. Existing support-knot single-cell/material proofs apply.
Source model and both owner curve IDs are retained for inspection; export
rebuilds canonical topology IDs while retaining the supplied curve coefficients.
The ordinary lens writer rejects a raw-recipe body to prevent accidental loss
of source bases. Raw surface weights that cannot normalize exactly remain
unsupported. Independent OCCT fixtures cover different model/owner knot bases
and a cubic model with raw weights `[3,3,4,6]`, whose normalized coefficient
`4/6` is not exactly float-representable. The raw variant round-trips those
weights exactly and supports concurrent independently refined support charts.

The document package additionally provides `ReadRationalSplineLensIGES`,
`FromRationalSplineLensIGES` and `State.WriteRationalSplineLensIGES`. A bounded
64-KiB recipe retains all twelve raw curve bases, bound to the source provenance
and exact native material. Provenance includes both owner pcurve IDs and knot
cells. Native save/reload and regeneration preserve the original coefficients;
export re-proves raw curve identities and rejects stale/rebound mismatches,
extra recipe rows/owners, invalid weights, unsupported placement or changed
material. Ordinary document IGES export rejects recipe-bearing assets.

`ReadPartitionedRationalSplineLensSolid`,
`WritePartitionedRationalSplineLensSolid`, and `NewPartitionedSplineLensSolid`
add a separate axis-aligned or X-sheared conjugate-axis lens family with 4–32 shared seam pieces. The exact
union of both original support charts' rational knot cuts determines the
partition. Each piece retains its original quarter, exact rational parameter
interval, both source knot cells, model curve ID and owner pcurve IDs. Complete
raw model/pcurve identities and closed topology are verified before the native
material certificate. The canonical native solid still has four ellipse edges;
source piece IDs are not treated as native edge IDs.

The native constructor extracts each exact rational segment and chooses a
common integer weight scale, preserving geometry without normalized-weight
rounding. Irrational cuts, unresolved cell coverage, unrepresentable Euclidean
poles/weights, missing or extra source cuts, and stale interval/cell recipes
fail explicitly. Source curve refinements remain intact in the recipe writer.
The four-edge APIs reject these graphs and recipe-bearing bodies. Independent
OCCT fixtures qualify six-edge single-owner and seven-edge concurrent
both-owner knot partitions, raw curve bases, closed topology and adaptive mass.
An independently authored six-edge sheared fixture additionally qualifies original
refined support bases and exact parameter progression on the shared arcs. The
canonical first endpoint and opposite Y-extreme quarter endpoints recover the
represented ellipse; these candidates are accepted only after whole-curve
identity of every source model and owner curve and exact material proof.

The document package exposes `ReadPartitionedSplineLensIGES`,
`FromPartitionedSplineLensIGES`, and `State.WritePartitionedSplineLensIGES`.
A bounded 128-KiB recipe retains source topology IDs, all raw model/owner bases,
exact rational piece intervals and both chart cells, bound to the native
material. Piece counts are limited to 4–32, rational coefficients to 8192 bits,
and JSON owner/cell arrays have explicit cardinality checks. Export re-derives
the exact knot partition from unchanged native charts and verifies every source
curve before writing. Native reload and six-/seven-edge OCCT chains retain
geometry, represented shear and lineage; stale/rebound mismatches, extra entries, oversized recipes,
placements and extra occurrences are rejected. Ordinary/four-edge document
exporters explicitly reject partitioned recipes rather than discarding them.

`ReadAffinePartitionedSplineLensSolid`, `NewAffinePartitionedSplineLensSolid`,
and `WriteAffinePartitionedSplineLensSolid` add a separate rotated source-basis
mode. The represented conjugate axes are `(Radii.X,Tilt)` and
`(Shear,Radii.Y)`, with positive diagonal components and positive exact
determinant. Exact source tangents and antipodal quarter vertices recover the
candidate basis, then every complete model and owner curve must equal its
re-derived quarter segment. Neither bounding vertices nor incidence alone
accept a solid. Original chart knots, positive raw weights and parameter
progression remain intact; writer source recovery must reproduce the supplied
basis exactly. The same 32-piece/resource limits apply.

This is bounded exact material exchange, not arbitrary rotated conic inference.
The native quadratic material proof may reject an otherwise rational source
ellipse if its canonical native coefficients require irrational or rounded
values. Irrational knot cuts and unrepresentable source poles likewise fail.
The independent OCCT fixture uses conjugate axes `(6r,4r),(-8r,3r)` and a
refined lower chart with three exact pieces of the first quarter; its original
six-edge source and native reconstruction pass OCCT validity, area and volume
checks. Older readers and writers reject nonzero `Tilt`;
they do not discard the affine source basis.

The document package's separate `ReadAffinePartitionedSplineLensIGES`,
`FromAffinePartitionedSplineLensIGES` and
`State.WriteAffinePartitionedSplineLensIGES` preserve that rotated source basis
through native save/reload. Its distinct 128-KiB recipe retains `Tilt`, `Shear`,
exact cuts, both knot-cell ancestries and all raw curves. Export reconstructs the
native material with `iges.AffineSplineLensFromSolid` and compares the complete
canonical native topology/geometry serialization before recovering supports.
Only an unchanged single identity occurrence is admitted. Altered basis, stale
geometry, conflicting recipe modes, excessive rational sizes and extra owner
entries fail before output. An independent OCCT chain qualifies the six-edge
fixture after native reload, preserving model units and all source bases.

## Cartesian datum points

`Entity.Point` is a finite `*geom.Vec3` in definition coordinates. `Eval(t)`
returns the placed point for any normalized `t` in `[0,1]`; this constant
parameter convention does not make the point a curve or allow it as a trim.
Type 116 requires a zero display-symbol pointer. Nonzero forms and referenced
symbols fail explicitly. Points consume ordinary entity/value/record budgets;
they do not consume the spline-control-point budget. The independent authored
`testdata/points.iges` fixture and mixed placed roundtrip tests qualify this
subset. The optional OCCT oracle writes an independent type 116 vertex and
reads a placed PlainSolid point back in model coordinates.

## Copious 3D polylines

`Entity.Polyline` supports type 106 **form 12**, interpretation flag 2, as an
ordered sequence of at least two finite XYZ points. Form 2 represents a point
set and remains unsupported; it is never silently connected. Consecutive
identical vertices and nonfinite segment differences are rejected. Explicitly
repeated first/last points can close a polyline; no closure or simplicity proof
is implied. Points count against the same cumulative `MaxControlPoints` budget
as spline poles, checked against declared count and available parameters before
allocation. `Eval` assigns each segment an equal normalized parameter span,
preserves endpoints, and applies the existing type-124 placement. It is not an
arc-length parameterization. Referenced trim/composite APIs retain their prior
line/spline-only scope.

The independently authored `testdata/polyline.iges` fixes form and flag semantics.
The optional OCCT oracle independently writes an ordered form-12/IP2 entity
and verifies that a placed PlainSolid export preserves each transformed point.

# STEP planar B-rep subset

This package adds useful planar solid exchange and a bounded Part 21 syntax
parser. It does **not** implement general STEP, complete AP203/AP214/AP242,
general curved B-reps or assemblies. A strict complete-sphere import API is
available separately. OCCT qualification covers the fixtures described below.

## Public APIs

- `Parse(io.Reader, Limits) (*File, error)` parses one HEADER and one DATA section,
  preserving IDs, encounter order, records and parameter types. Forward references
  must resolve. Simple and complex instances, nested aggregates, typed parameters,
  comments, quoted strings and X2/X4 Unicode scalar escapes are supported.
- `Read(io.Reader, Limits) (Model, error)` imports the supported geometric graph.
  `Model.LengthUnit` is meters per coordinate unit; coordinates are **not** rescaled.
- `Write(io.Writer, Model) error` writes deterministic AP203-style
  `CONFIG_CONTROL_DESIGN` files with explicit SI units, geometric contexts,
  faceted shape representations, explicit supporting planes and product/shape-definition links.

`WriteAdvanced(io.Writer, Model)` uses the same units and product metadata but
emits `MANIFOLD_SOLID_BREP`/`ADVANCED_FACE` with shared `VERTEX_POINT` and linear
`EDGE_CURVE` topology. Both writers preserve planar holes and validate the solid.
The advanced writer has optional pinned-OCCT box and holed-prism qualification
covering validity, surface area and volume. It does not emit curved surfaces.

Example with coordinates measured in millimeters:

```go
s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 10, Y: 20, Z: 30})
if err != nil { return err }
return step.Write(w, step.Model{
    LengthUnit: 0.001,
    Bodies: []step.Body{{Name: "block", Solid: s}},
})
```

The writer retains polygon faces and holes; it does not turn each face into a
set of STEP triangles. Unicode names are encoded with X4 escapes. Output uses a
fixed timestamp for reproducibility. Callers own and close streams.

## Geometry mapping

| STEP entity | Imported representation |
| --- | --- |
| `FACETED_BREP_SHAPE_REPRESENTATION` or `ADVANCED_BREP_SHAPE_REPRESENTATION` | One or more independent `Body` values with a shared length unit |
| `FACETED_BREP` or `MANIFOLD_SOLID_BREP` | One connected, outward-oriented `brep.Solid` shell |
| `CLOSED_SHELL` | Face collection, checked for manifold closure |
| `FACE`, planar `FACE_SURFACE` or planar `ADVANCED_FACE` | Planar polygon face |
| `PLANE`, `AXIS2_PLACEMENT_3D`, `DIRECTION` | Validated supporting plane and orientation |
| `FACE_OUTER_BOUND` | Outer oriented edge-use loop |
| `FACE_BOUND` | Inner oriented loop, subject to BRep hole validation |
| `EDGE_LOOP`, `ORIENTED_EDGE`, `EDGE_CURVE`, `VERTEX_POINT` | Explicit shared straight-edge topology |
| `LINE`, `VECTOR`, linear `SURFACE_CURVE` and planar `PCURVE` | Validated 3D and parameter-space edge loci |
| `POLY_LOOP` | Ordered point references, with implicit shared straight edges |
| `CARTESIAN_POINT` | Three finite coordinates; exact coordinate duplicates welded within each body |

`.F.` bound orientation reverses loop traversal. There must be exactly one
explicit outer bound per faceted face. Advanced faces may omit it: the largest
loop is selected and all hole containment constraints are still checked. Hole loops must have the opposite winding,
remain strictly contained and satisfy the BRep implementation's size limits.
A closed shell must be connected and have finite positive signed volume.

Length units must be explicit SI metres with an optional standard prefix.
Mixed-unit representations and repeated body instances are rejected.
Non-geometric product metadata is parsed but discarded on import. Assemblies,
mapped items and transformation relationships are rejected to avoid silently
losing placement or instance semantics. Representation placement items are validated as coordinate-system metadata;
body placement transformations remain unsupported. Nonplanar face surfaces,
curved edge geometry, void-shell entities,
conversion-based units, styles and material metadata transfer.

`Parse` is a syntax reader, not an EXPRESS schema validator. It does not support
multiple DATA sections, edition-3 ANCHOR/REFERENCE/signature sections, binary
literals, legacy code-page string escapes or external references. Unsupported
syntax fails explicitly. Successfully parsing another entity type does not
mean `Read` can import it.

## Resource bounds

Zero-valued `Limits` fields select 64 MiB, one million entities, four million
parameter values, nesting depth 64, one million imported vertices, 100,000 faces,
one million expanded edge uses and 1,000 bodies. The byte cap is checked before
parsing. Nesting cannot exceed 256 even when configured. Expanded edge-use limits
prevent repeated loop references from multiplying topology without bounds.
These are memory/work-count limits, not execution deadlines; polygon validation
can be superlinear. Applications handling hostile files should choose tighter
limits and apply process-level execution budgets where needed.

## Evidence and references

Tests include an independently authored tetrahedron fixture, outward orientation
and analytic volume, negative loop orientation, exact graph/volume roundtrips,
holed prisms, SI units, Unicode, malformed syntax/references, I/O failures,
resource limits and parser/import fuzz targets. The optional pinned-OCCT oracle has imported exported box and holed-prism files,
passed its unchanged `BRepCheck_Analyzer` validity check, and matched area, volume
and centroid. An independently authored advanced tetrahedron also tests explicit edge topology.
The optional oracle exports an OCCT box, imports its planar advanced graph,
checks area/volume/centroid and reexports it for OCCT validity verification.
Broader CAD fixtures and independent application roundtrips remain
outstanding. Exported faces use `FACE_SURFACE` with explicit `PLANE` geometry:
OCCT skips implicit `FACE` records even though the faceted schema permits them.
The importer still accepts implicit faces and validates explicit planes against
polygon bounds and surface sense.

The entity mapping follows the primary schema descriptions for
[poly loops and face bounds](https://www.steptools.com/stds/smrl/data/resource_docs/geometric_and_topological_representation/sys/5_schema.htm)
and [faceted B-reps](https://www.steptools.com/docs/stp_aim/html/t_faceted_brep.html).
[OCCT's STEP guide](https://github.com/Open-Cascade-SAS/OCCT/wiki/step) supplies
reference context for STEP-to-topology mapping. The
[Part 21 reference](https://www.steptools.com/stds/step/IS_final_p21e3.html) informs
the lexical and section rules. No OCCT source or test fixture
was copied.

## Independent spline curve geometry

`ReadCurves` and `WriteCurves` exchange `CurveModel` values through
`GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION` and `GEOMETRIC_CURVE_SET`.
Each `SplineCurve` retains its name, immutable `geom.BSpline`, and informational
closed/self-intersection logical flags (`T`, `F`, `U`). Explicit-knot polynomial
and complex rational B-spline instances are accepted; positive weights, sorted
knots, multiplicities, degree and total control-point limits are checked.
`DecodeBSplineCurve(file, entityID, maxPoints)` exposes that mapping for callers
using the bounded syntax parser directly. Repeated control-point references are
valid; repeated curve instances and unrepresented spline entities are rejected.

This API exchanges independent full-domain geometry, not connected topological
wires or curved solid edges. Forward `TRIMMED_CURVE` wrappers are accepted only when explicit parameter
selectors equal the full basis domain; any supplied endpoint coordinates must
agree. Actual clipping, reverse trims, Cartesian-only trimming, implicit knot
families, assemblies,
periodic wrapping and arbitrary curve types are unsupported. `MaxVertices`
limits total imported curve control points. Units retain the same SI/raw-coordinate
contract as solid exchange. Tests include an independently authored rational
quarter circle, a polynomial counterpart, malformed knot/weight data, resource
limits, roundtrips and bounded fuzzing. The entity mapping follows the primary
[STEP spline schema](https://www.steptools.com/stds/smrl/data/resource_docs/geometric_and_topological_representation/sys/4_schema.htm).

Optional pinned-OCCT curve tests pass in both directions for a rational unit
quarter circle, including OCCT full-domain trim wrappers, sampled coordinates,
one valid edge and length pi/2 within `1e-6` integration tolerance.

## Independent spline surface geometry

`ReadSurfaces` and `WriteSurfaces` exchange `SurfaceModel` patches through
`GEOMETRIC_SET` under `GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION`.
`DecodeBSplineSurface(file, entityID, maxPoints)` also decodes a support surface
from a separately parsed graph. Simple polynomial and complex rational
`B_SPLINE_SURFACE_WITH_KNOTS` instances preserve both knot domains and the
U-major rectangular control net. Positive weights, rectangular dimensions,
knot ordering/multiplicities, degree, logical flags and total control-point
limits are checked. `MaxVertices` counts all imported surface control points.

These APIs represent full-domain geometry; they do not infer boundary trimming,
face orientation, wire connectivity, closed shells or periodic wrapping. Closed
and self-intersection flags are metadata. General face/shell STEP graphs remain
outside `ReadSurfaces`, even when their support geometry is decodable.

Evidence includes an independent rational quarter-cylinder fixture, a polynomial
variant, malformed dimensions/weights/knots, bounded fuzzing and I/O failures.
The pinned OCCT oracle accepts exported surface geometry as one valid face with
area pi within `1e-6`. Separately, the decoder reads the rational support surface
from an OCCT-exported quarter-cylinder face and verifies sampled analytic points.
That second test does **not** claim import of the OCCT face's trimming/topology.

## Minimal analytic spherical solids

`ReadAnalyticBodies` imports exact complete spheres into `brep/parametric.Solid`.
The accepted graph is deliberately strict: one `MANIFOLD_SOLID_BREP`, one
`CLOSED_SHELL`, one outward `ADVANCED_FACE` on a `SPHERICAL_SURFACE`, and one
forward `FACE_BOUND` or `FACE_OUTER_BOUND` containing a `VERTEX_LOOP` whose
`VERTEX_POINT` lies at either parametrization pole. The pole check uses a
radius-relative `1e-10` tolerance. Edge-loop trims, partial spheres, extra faces,
void shells, unsupported solid types, repeated instances and assembly placements fail.
The imported spherical locus is exact; the internal seam is canonicalized.
Units remain explicit raw-coordinate SI units. Limits count reconstructed
vertices and boundary uses (two vertices and four uses per sphere).

Independent north/south and rotated-axis fixtures check analytic area, volume
and centroid; malformed poles, trim kinds, orientations and frames fail. The
optional pinned-OCCT sphere export passes import and the same mass checks.
The minimum topology follows the STEP implementor agreement for a
[complete sphere with a pole vertex loop](https://www.steptools.com/stds/impforum/isslog.pdf).

`DecodeAnalyticSurface` separately maps `PLANE`, `CYLINDRICAL_SURFACE`,
`SPHERICAL_SURFACE` and ring `TOROIDAL_SURFACE` entities to exact geometry. It
validates dimensions and placement frames, but does not interpret their face
trims or claim that other analytic solids can be imported. Spindle/horn tori and general curved BReps remain unsupported here. Complete ring
tori have the separately documented strict body mapping below.

`WriteSpheres` accepts explicit `SphereModel` / `SphereBody` values containing
center, radius and name. It validates each sphere and emits the same exact
minimal spherical graph; it never guesses a primitive from arbitrary topology.
Multiple independent sphere bodies and Unicode names are supported. Deterministic
roundtrips and optional OCCT export qualification check validity, area, volume
and centroid for a translated sphere.

## Complete ring tori

`ReadAnalyticBodies` also accepts one outward `TOROIDAL_SURFACE` face with the
canonical full double-seam graph: two closed circular `SEAM_CURVE` edges share
one outer vertex and each appears twice with opposite senses in the face loop.
Each seam has two explicit linear UV pcurves at zero and `2*pi`; their directions,
unit vector magnitudes and surface references are checked. Circle radii,
centers, axes and reference directions must agree with the torus frame. This
establishes the full periodic parameter rectangle; arbitrary partial tori,
other seam layouts and unvalidated trimming graphs are rejected. Geometric
comparisons use relative `1e-10` and angular/UV `1e-10` tolerances.

`WriteTori` accepts explicit `TorusModel` / `TorusBody` values (frame, major and
minor radii, name, units) and emits that exact graph. Horn/spindle tori are
unsupported. Optional pinned-OCCT tests pass in both directions for a translated
ring torus, with independent validity, area, volume and centroid checks.

## Complete finite cylinders

`WriteCylinders` accepts explicit `CylinderModel` / `CylinderBody` values with
frame, radius, positive height and name. `ReadAnalyticBodies` accepts the matching
strict three-face graph: one cylindrical side and two plane caps, shared closed
circular rim edges, and one longitudinal line seam used twice. Both side rim
pcurves and the seam's full-period UV lines are verified; each plane cap must
supply the same circular edge, correctly oriented, with a matching circular 2D
pcurve. Surface/circle/line frames, radii, endpoints, parameter extents and senses
must agree. The supported side interval starts at frame Z=0 and ends at positive
height. Partial cylinders, missing caps and other trim layouts are rejected.

The pinned OCCT `write-step-cylinder` fixture (radius 2, height 4, translated
base) passes import, and exported cylinders pass unchanged OCCT validity plus
area, volume and centroid comparisons. Default tests cover rotated frames,
malformed caps/rims/seams, resource limits and I/O failures. Analytic input limits
count all reconstructed vertices, faces and rectangular boundary uses across
sphere, torus and cylinder bodies.


## Finite cones and conical frusta

`WriteCones` takes explicit `ConeModel` / `ConeBody` values with a frame,
`BaseRadius > TopRadius >= 0`, and positive height along frame +Z. This covers
complete narrowing frusta and cones with an apex; reversing a frame covers the
opposite physical orientation. Equal radii use `WriteCylinders`. Unrepresentable
semi-angles fail. The side is a `CONICAL_SURFACE`, circular rims share cap edges,
and a generator seam supplies the full periodic boundary. An apex uses a
three-use side loop and one cap; no zero-radius circle is synthesized.

`ReadAnalyticBodies` validates the same canonical graph, including the conical
radius law, generator direction, rim/cap geometry, full-period side pcurves and
apex location. Its exact output retains conical and planar patches through
`parametric.Revolve`. The decoded `ConicalSurface` exposes STEP's reference-circle
and axial parameter convention (`radius(v)=referenceRadius+v*tan(semiAngle)`),
with evaluation restricted to the nonnegative-radius sheet.

OCCT cone and frustum exports both import successfully; both Go exports pass
OCCT validity plus analytic area, volume and centroid checks. Default tests
exercise rotated frames, conical parameter evaluation, malformed angles/rims/
seams/pcurves and I/O failures. Arbitrary conical trims and oblique caps remain
unsupported.

## Explicit rigid assembly graphs

`ReadAssembly` reads the AP214 `AUTOMOTIVE_DESIGN` / AP242 managed-model schema
subset using `PRODUCT_DEFINITION`, product formation/name records,
`NEXT_ASSEMBLY_USAGE_OCCURRENCE`, `CONTEXT_DEPENDENT_SHAPE_REPRESENTATION`, and
complex representation relationships with `ITEM_DEFINED_TRANSFORMATION`.
Both transformation items must be `AXIS2_PLACEMENT_3D` records belonging to the
child and parent representations respectively. Each occurrence maps child
coordinates into its parent by `parentFrame * inverse(childFrame)`. Reused
products remain one definition with multiple occurrences; roots and names are
retained. Consumers compose transforms along their chosen occurrence path.

`Assembly.Products` use product-definition IDs as keys; `Occurrences` carry
parent/child IDs, source occurrence ID, name, identifier and rigid transform.
`Source` retains the bounded parsed geometry graph. `PlanarBodies` and
`AnalyticBodies` decode supported product solids in **local coordinates**;
assembly-only products return no bodies. No API silently expands or flattens
occurrences. Existing shape-only readers continue rejecting assembly graphs.
The graph reader validates structure/placements, while product geometry is
validated when the caller invokes a geometry decoder.

The accepted subset requires explicit uniform SI units, represented products,
one placement relationship per occurrence, and an acyclic graph. `MaxSolids`
bounds product definitions, `MaxEdgeUses` bounds occurrences, and `MaxDepth`
also bounds graph depth. Mapped-item/scaled placement, external references,
alternate occurrence entities and colors/materials are not supported. Descriptions in this subset must be strings rather than omitted.

An independent assembly fixture checks transform direction, nonidentity source
frames, cycles, malformed links and limits. An optional real OCCT XCAF export
contains one named box part reused in two named occurrences (translation and
rotation/translation); tests verify names, instancing, transforms and the decoded
part's local volume. The mapping follows the primary
[STEP assembly structure guide](https://www.steptools.com/stds/smrl/data/modules/item_definition_structure/sys/f_guide.htm).


`WriteAssembly` accepts an `AssemblyModel` with named `AssemblyPart` products
(string ID, name, local planar `Bodies`) and `AssemblyInstance` occurrences
(string ID, name, parent/child product IDs, `geom.Affine` transform). Product IDs
and occurrence IDs must be unique. Products without local bodies must contain
child occurrences. Cycles, missing references, reflections and scaling fail;
rigid transform checks use `1e-10`. Placements are normalized to orthonormal
frames before serialization. Nested and repeated instances remain explicit.

The writer emits AP214 product structure and planar advanced BRep geometry,
including multiple local bodies per product. Geometry staging uses bounded
parser defaults. Analytic-body, color and material assembly export are not yet
supported. Independent roundtrips check nested/repeated products, placements,
local multi-body geometry and failure cases. A real OCCT XCAF read validates
all named products/occurrences, each placement, aggregate shape validity, volume
and centroid for a nested assembly with a reused part.

`ReadTrimmedBodies` returns `TrimmedModel` with exact `brep/trimmed.Solid`
geometry. It accepts the already validated complete analytic-body graphs and
one additional curved trim graph: a full circular cylinder bounded by a
perpendicular base plane and an oblique plane that crosses every generator
strictly above the base. This graph requires three outward faces, complete
`CIRCLE` and `ELLIPSE` rims, one shared axial `LINE` seam used twice in opposite
directions, and direct edge geometry. The ellipse's projection must be the
cylinder circle; both cap planes, seam endpoints, edge incidence and orientations
are checked. STEP decimal round-off is checked at relative geometric tolerance
1e-9. Reconstruction preserves the analytic loci using exact parametric patches,
which may subdivide the imported faces. Model coordinates retain their declared
unit. OCCT's optional approximate cylinder p-curves are deliberately outside this
subset; export the reference with `write.surfacecurve.mode=0`.

General partial conic arcs, noncircular bores, multi-hole curved faces and arbitrary
advanced-face graphs remain unsupported by this entry point. The persisted
`testdata/oblique-cylinder.step` was generated by the pinned optional OCCT oracle;
default tests check its result against closed-form area/volume/centroid integrals.
`write-step-oblique-cylinder` regenerates it for optional live qualification.

`WriteObliqueCylinders` exports this same cut subset from
`ObliqueCylinderModel` / `ObliqueCylinderBody{Name, Frame, Radius, Plane}`.
The base is Frame's XY plane, and the solid lies below the outward cap Plane;
all cap heights must remain strictly positive. The writer derives the ellipse
axes algebraically and shares rim/seam topology, without emitting approximate
p-curves. Optional `step-mass-precise` OCCT qualification uses adaptive mass
integration (requested tolerance 1e-10), and requested reader precision 1e-10. OCCT still differs from closed-form
mass on its own direct-edge export, consistent with reconstruction of the omitted
non-rational cylinder p-curve. Tests therefore compare independently generated
and PlainSolid-exported identical-seam graphs to 1e-7 area/volume and 1e-8
centroid, plus explicit absolute analytic envelopes of 2e-4 area, 1e-4 volume,
and 2e-6 centroid. Native roundtrip analytic checks remain 1e-6 area/volume and
1e-7 centroid. BRep validity remains mandatory.

Whole planar-body color metadata has separate APIs:
`ReadColoredPlanar` / `WriteColoredPlanar`, with
`ColoredModel{Bodies []ColoredBody, LengthUnit}` and
`ColoredBody{Body Body, SurfaceColor *RGB}`. `RGB{R,G,B}` uses finite sRGB
components in [0,1]; nil means unspecified, not black. This follows OCCT's
`STEPConstruct_Styles` conversion to/from `Quantity_TOC_sRGB`.
The writer emits AP214 `MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION`
root styles through `STYLED_ITEM`, `PRESENTATION_STYLE_ASSIGNMENT`,
`SURFACE_STYLE_USAGE(.BOTH.)`, `SURFACE_SIDE_STYLE`, `SURFACE_STYLE_FILL_AREA`,
`FILL_AREA_STYLE`, `FILL_AREA_STYLE_COLOUR`, and `COLOUR_RGB`.
The reader requires each styled body to belong to the presentation context and
rejects conflicting/unowned styles, face/occurrence overrides, transparency,
visibility directives, named colors and other unsupported appearance graphs.
Geometry-only APIs retain their behavior and do not return color metadata.
Optional OCCT XCAF `write-step-color` / `step-color` modes verify both directions,
including BRep validity and sRGB components within 1e-6 (OCCT stores float colors).

`ReadConicArcs` / `WriteConicArcs` exchange exact bounded `PARABOLA` and positive
branch `HYPERBOLA` arcs through forward `TRIMMED_CURVE` parameter selectors in a
`GEOMETRIC_CURVE_SET`. `ConicArcModel` contains named `ConicArc` values with
`Geometry` (`geom.Parabola` or `geom.Hyperbola`) and native `Start`/`End` values.
STEP parabola parameter `u` is the local Y coordinate: native `t=u/(2*focal)`.
STEP hyperbola uses `(a*cosh(u), b*sinh(u))`: native `t=tanh(u/2)`.
The writer performs the inverse conversions. Units remain raw coordinates.
Optional Cartesian endpoint selectors are checked against the parameter value;
Cartesian-only trims, reversal, unbounded arcs and negative hyperbola branches
are unsupported. Nonfinite/overflowed parameters and saturated hyperbola mappings
are rejected. Persistent independent OCCT fixtures check parameter semantics,
and optional live tests require valid imported edges and matching lengths in
both directions. These APIs leave the spline-specific `ReadCurves` unchanged.

`ReadFaceColoredPlanar` / `WriteFaceColoredPlanar` extend this style subset with
explicit per-face fills for strict advanced planar bodies. `FaceColoredModel`
contains `FaceColoredBody{Body, SurfaceColor, FaceColors map[int]RGB}`. The body
color is an optional default; a face-map entry overrides it. The indices refer
to `Body.Solid.Faces()` and are resolved through the owning body's closed-shell
face list, never through a global entity order or a geometric proximity guess.
Shared-face ancestry, duplicate targets, invalid face indices and unsupported
style chains fail. The original whole-body color API still rejects face styles.
Face overrides are emitted in ascending face order for deterministic output.
The optional OCCT XCAF `write-step-face-color` / `step-face-color` modes verify
actual per-face attachment and RGB values in both directions. Indices are stable
within the imported body; they are not persistent IDs across later geometry edits.

Length contexts accept SI metre prefixes and dimensionally checked
`CONVERSION_BASED_UNIT` chains ending in an SI metre unit. Supported factors are
`MEASURE_WITH_UNIT(LENGTH_MEASURE(value),#unit)` and OCCT's simple
`LENGTH_MEASURE_WITH_UNIT(value,#unit)`. `NAMED_UNIT` must reference explicit
length dimensional exponents `(1,0,0,0,0,0,0)` for a conversion-based unit.
Decimal factors are multiplied as exact rational numbers and rounded to the
public float64 `LengthUnit` only once. Cycles, more than 64 conversion links,
factor literals longer than 1024 characters, nonpositive factors, incorrect
dimensions, unsupported measure forms and final
float64 underflow/overflow fail. Coordinates are never rescaled by this layer.

All STEP writer families now accept any finite positive meters-per-model-unit
value. Standard SI powers retain SI units; other values use an explicit conversion
factor to metres, with inch/foot labels for 0.0254/0.3048. Decimal serialization
roundtrips the supplied float64 value. Unit labels are descriptive, not trusted
as conversion factors. Live OCCT inch/foot tests verify imports and exports using
physical box volumes; malformed/cyclic chains have independent tests.

`ReadCavities` / `WriteCavities` exchange planar `BREP_WITH_VOIDS` using
`CavityModel` / `CavityBody{Name, Outer, Voids, Solid}`. Each
`OrientedVoidShell{Shell, Reversed}` keeps a separate source shell and explicit
orientation: this initial subset requires outward source shells with Reversed=true
(STEP `ORIENTED_CLOSED_SHELL` orientation `.F.`). `Solid` is derived combined
topology with inward cavity boundaries; writers derive it from Outer/Voids.
Source coordinates and advanced vertex/edge identities are retained per shell.

The outer shell and void shells must be convex, connected, closed and outward,
without face holes. Exact orientation predicates prove every void vertex lies
strictly inside every outer halfspace. Each pair of voids must have a strictly
separating shell face plane. Touching, nested, overlapping and uncertifiable
configurations fail; some disjoint skew polyhedra are intentionally unsupported.
A body permits at most 256 voids and at most 4 million total face/vertex pairs
for the convex proof. General nonconvex cavity containment and `.T.` oriented
inward source shells remain unsupported. Independent OCCT fixtures and live
bidirectional checks require valid BReps and matching volume/surface area.

`ReadAnalyticBodies` (and therefore `ReadTrimmedBodies`) additionally accepts full
concentric hollow cylinders: four advanced faces, two canonical cylindrical walls
and two planar annular caps. Each wall has its complete circle rims and shared
axial seam, with exact cylinder and plane p-curves. Cap bounds must use the same
rim edges; the inner wall and annular holes must have inward orientation.
Both walls undergo the complete-cylinder graph validation after orientation
normalization; their frames and heights must agree and 0 < inner < outer radius.
This subset requires aligned cylinder parameter frames based at the lower cap;
partial tubes, offset axes, shifted surface parameter origins and general smooth
face splitting are unsupported. Reconstruction preserves the exact tube locus,
using the native parametric representation's canonical patch subdivision.

`WriteTubes` accepts `TubeModel` containing
`TubeBody{Name, Frame, OuterRadius, InnerRadius, Height}` and emits shared annular
cap topology with the two complete seam/p-curve graphs. Independent OCCT tests
verify valid BReps, volume, area and centroid; persisted fixtures and malformed
orientation/rim tests also run without OCCT.

`WriteSteppedCylinders` and `ReadAnalyticBodies` exchange a narrowing stepped
shaft with two coaxial cylindrical walls, an annular shoulder and two planar
end caps. `SteppedCylinderBody{Name, Frame, LowerRadius, UpperRadius,
LowerHeight, UpperHeight}` requires lower radius > upper radius > 0 and positive
heights. Both walls retain full seam/circle/p-curve graphs. The reader proves
both virtual full cylinders, requires exactly one two-bound shoulder and two
one-bound end caps, checks every rim's ownership/orientation, and proves the
upper cylinder starts precisely where the lower ends. The native result is an
exact revolved profile. Gaps, overlaps, extra or missing shoulder coverage,
shifted axes/parameter origins and additional steps are unsupported.
Independent OCCT fuse fixtures and live bidirectional tests verify topology,
volume, area and centroid; malformed coverage tests run without OCCT.

Spherical caps are supported by `ReadAnalyticBodies` and `WriteSphericalCaps`.
`SphericalCapBody{Name, Frame, Radius, Latitude}` keeps the part above the plane
at `Radius*sin(Latitude)` along Frame's Z axis; latitude is strictly between
-pi/2 and pi/2. A hemisphere has latitude zero. The graph must have one outward
spherical face, a shared full latitude-circle rim, a meridian seam used twice,
a north-pole vertex and a downward-facing planar disk. Exact circle frames,
radii, endpoints and sphere/plane p-curves prove the trim coverage. Meridian
p-curves may shift their parameter origin by whole multiples of 2*pi, as in
OCCT exports, but arbitrary phase shifts fail. Near-tangent cuts within native
geometric tolerance are unresolved and rejected. Reconstruction uses exact
`parametric.ClipSphere`; its patch subdivision need not equal the source faces.
Live OCCT checks cover hemisphere import and both-direction hemisphere export,
plus exported caps with positive and negative latitude. General spherical bands,
multiple disk cuts and arbitrary spherical trim loops remain unsupported.

`WriteSphericalZones` / `ReadAnalyticBodies` extend this to a spherical band
between two latitude planes. `SphericalZoneBody{Name, Frame, Radius,
LowerLatitude, UpperLatitude}` requires -pi/2 < lower < upper < pi/2. The
boundary is one complete spherical zone plus two planar disks. The source
latitude rims must be distinct, each cap must share its exact rim edge, and
one meridian seam must connect the two rim vertices with opposite uses on the
sphere. Both rim/support/p-curve sets undergo the spherical-cap proof, and
latitude ordering plus cap orientation establish the finite band coverage.
The result uses native exact `parametric.SphereZone`, retaining shared topology.
OCCT-generated independent fixtures and both-direction live tests verify valid
BReps, volume, area and centroid. Degenerate spans, pole-touching zones, reused
rims, collapsed seams and reversed caps are rejected.

`WriteTorusElbows` / `ReadAnalyticBodies` support bounded ring-torus pipes with
two planar end disks. `TorusElbowBody{Name, Frame, MajorRadius, MinorRadius,
Angle}` sweeps a full circular minor section from frame X toward Y through
0 < Angle < 2*pi. The graph must contain one toroidal face, two distinct full
minor-circle rims and a shared outer major-circle seam. Exact frame/radius and
endpoint checks, paired surface/plane p-curves, ordered major-angle bounds,
opposite seam uses and outward cap orientations prove the closed elbow coverage.
Cap-plane origins may differ from circle centers when their 2D circle offset
maps exactly within geometric tolerance; this includes OCCT's axis-origin caps.
The result uses exact native `parametric.TorusElbow`. Near-coincident ends within
native tolerance are rejected. Independent OCCT fixtures, live 90° and >180°
export tests, analytic mass/centroid checks and malformed seam/cap tests qualify
this subset. Partial minor-circle sections and arbitrary toroidal trims remain
unsupported.

`WriteAxialProfiles` and `ReadAnalyticBodies` generalize stepped shafts
to 3–64 coaxial cylindrical sections. `AxialProfileBody{Name, Frame, Sections}`
contains `CylinderSection{Radius, Height}` entries in increasing axial order,
with distinct adjacent positive radii and positive heights. Local widening and
narrowing are both supported, including grooves and collars. Every wall retains
a complete seam/circle/p-curve graph; adjacent sections share one exact annular
shoulder. The reader requires N walls, N-1 two-bound shoulders, two end caps,
2N uniquely owned cap bounds, aligned frames, and consecutive nonoverlapping
height intervals. Each shoulder normal follows the local radius change; axial
order comes from proved lower circle trims, not radius rank. It reconstructs an exact
native revolved profile. Conical sections use the mixed-profile API below.
Equal-radius redundant splits, gaps, overlaps and unproved
shoulder coverage remain unsupported. The two-section `WriteSteppedCylinders`
API is unchanged. Live independent OCCT fusion tests qualify three sections,
and default tests also cover longer profiles and malformed coverage. Independent
OCCT fused collar/groove fixtures qualify both shoulder orientations in both
exchange directions. Cut-generated grooves with shifted cylinder/seam parameter origins are also
qualified. The reader verifies original seam model-space/p-curve phase agreement
before translating parameter origins, then applies the same complete wall proof.

### Hollow conical frusta

`WriteHollowCones(HollowConeModel)` and `ReadAnalyticBodies` support full coaxial
narrowing annular frusta with two conical walls and two planar annular caps.
`HollowConeBody` supplies `Name`, `Frame`, `OuterBaseRadius`, `OuterTopRadius`,
`InnerBaseRadius`, `InnerTopRadius`, and `Height`. Both walls must narrow with
positive end radii, and the outer radius must exceed the inner at both ends.
Linear radius variation then proves that the walls cannot cross in between.
Each wall passes the complete finite-cone seam/rim/p-curve proof, with shared cap
bounds and opposite inner-wall orientation checked separately. The native result
is an exact revolved four-segment profile. Apex holes, partial revolutions,
shifted cone parameter origins, and mixed cylinder/cone walls are unsupported.
A persisted independent OCCT cut fixture and live import/export tests check valid
BReps and analytic mass; malformed slope, cap coverage and inner orientation are
rejected. Existing cylinder-tube APIs retain their behavior.

### Mixed stepped axial profiles

`WriteMixedAxialProfiles(MixedAxialModel)` accepts `MixedAxialBody{Name, Frame,
Sections}` with 3–64 `AxialSection{BaseRadius, TopRadius, Height}` entries.
Equal radii describe a cylinder; decreasing positive radii describe a conical
frustum. Every consecutive pair must have a nonzero annular shoulder, whose
orientation follows the preceding top radius and following base radius.
`ReadAnalyticBodies` validates complete individual walls, shared cap ownership,
coaxial consecutive height intervals, and all shoulder orientations before
constructing an exact revolved profile. Cylinder walls may use the proven shifted
origins described above; cone origins must remain at the lower rim. Widening cone
walls, apex sections, equal-radius smooth junctions and general freeform profiles
are unsupported. The original `WriteAxialProfiles` API delegates to this writer
with equal section radii. Independent OCCT cylinder/cone/cylinder fusion fixtures
and live import/export tests verify topology, volume, area and centroid.

### Cylindrical sectors

`WriteCylinderSectors(CylinderSectorModel)` exports `CylinderSectorBody{Name,
Frame, Radius, Height, Angle}`. The angle runs from frame X toward Y and must lie
strictly between zero and a full turn, subject to native resolution tolerances.
`ReadAnalyticBodies` accepts the corresponding exact five-face solid: one bounded
cylindrical wall, two radial planar rectangles, and two planar sector caps.
The proof requires six distinct vertices, nine shared edges, canonical circle
frames, complete oriented face loops, exactly two owning surfaces per edge,
and matching model-space and surface-space curves with unit line parameter
scales. It reconstructs `parametric.CylinderSector` without tessellation.
Arbitrary angular offsets, shifted supporting-plane frames, extra split edges,
remain unsupported; conical sectors use the API below. An independent OCCT fixture covers
import; live OCCT validity, volume, area and centroid tests cover quarter-turn
and greater-than-half-turn exports. Invalid orientation, p-curve phase,
parameter scale and axis connectivity are rejected.

### Conical frustum sectors

`WriteConicalSectors(ConicalSectorModel)` exports
`ConicalSectorBody{Name, Frame, BaseRadius, TopRadius, Height, Angle}` with strictly
decreasing positive radii. `ReadAnalyticBodies` reuses the complete sector graph
proof, additionally checking the conical supporting frame, taper, unequal rim
radii, slanted generator lines, and reversed STEP angular/axial p-curve signs.
The result is an exact native `parametric.ConicalSector`. The two radial faces
are trapezoids; the end faces are circular sectors. Apex sectors, widening
frusta, shifted supporting frames and split faces remain unsupported. A pinned
OCCT fixture independently verifies import, and live quarter-turn and >180°
export tests check BRep validity, area, volume and centroid. Malformed taper,
rim radius, parameter scale, orientation and connectivity are rejected.

### Colored rigid assemblies

`ReadColoredAssembly` combines strict rigid assembly topology with advanced
planar product geometry and surface styles. Its `ColoredAssembly` contains the
ordinary `Assembly` plus `ProductBodies`, keyed by product-definition ID, holding
local-coordinate `FaceColoredBody` values. Repeated and nested occurrences retain
one product definition and their explicit transforms. Global geometry budgets
apply across products. `WriteColoredAssembly(ColoredAssemblyModel)` accepts
`ColoredAssemblyPart{ID, Name, Bodies}` products, ordinary `AssemblyInstance`
occurrences, and a shared `LengthUnit`.

Both body defaults and face overrides remain attached to local product geometry.
The reader additionally accepts OCCT's `OVER_RIDING_STYLED_ITEM` only when its
parent is a validated body-wide styled item for that face's own body; cycles,
body-target overrides and cross-body ancestry fail. Per-occurrence style overrides,
transparency, named colors and other presentation semantics remain unsupported.
Existing geometry-only assembly APIs retain their behavior. Live XCAF tests
qualify colored repeated-product import and nested repeated-product export,
checking names, transforms, valid assembly mass, default RGB and face RGB.

### Exact rectangular spline faces

`ReadSplineFaces` / `WriteSplineFaces` exchange `SplineFaceModel{Faces,
LengthUnit}` with `RectangularSplineFace{Name, Geometry}` entries. These are
non-solid faces, each in a separate one-face `OPEN_SHELL` inside a
`SHELL_BASED_SURFACE_MODEL` / `MANIFOLD_SURFACE_SHAPE_REPRESENTATION` graph.
The represented trim is the complete natural UV rectangle of a clamped rational
tensor-product B-spline, including nonunit parameter domains.

Every face has four shared corner vertices and four explicit model-space spline
edges with straight UV p-curves. Each edge must match its corresponding surface
control row/column through bounded exact homogeneous polynomial identity on
common parameter spans, including differing knot/degree representations. No
sampled curve equality is used. Loop senses,
endpoint ownership, supporting-surface references and UV parameter scales are
checked. A bounded exact projected-injectivity certificate additionally proves
that the patch is regular and embedded; unresolved certificates reject import
and export. Coordinate and simple integer-affine projections cover the qualified
quarter-cylinder and rational graph fixtures.

A persisted independently generated OCCT quarter-cylinder face imports with its
actual boundary graph; native exports pass OCCT validity and area checks. Bad
boundary poles/weights, UV offsets, topology, orientation, folds and resource
limits are tested. Reversed faces, arbitrary trim rectangles, holes, joined
multi-face shells, nonclamped patches, reparameterized boundaries, and unresolved
exact equality proofs are currently unsupported. This API does not
claim closed-shell or general curved BRep interoperability.

Callers can pass a returned geometry to `trimmed.RectangularSplineFace` and then
use the independent exact sewing API for compatible patch collections. Exchange
itself preserves separate faces and does not infer or claim a sewn solid.

### Exact rectangular subdomain faces

`ReadBoundedSplineFaces` / `WriteBoundedSplineFaces` use
`BoundedSplineFace{Name, Geometry, Domain}` and `BoundedSplineFaceModel`. `Domain`
is a `geom.ParameterRectangle` in the original supporting surface's parameters;
strict interior trims and the natural full domain are both supported. Existing
`ReadSplineFaces` / `WriteSplineFaces` retain their full-domain contract.

The reader derives the rectangle from four axis-aligned p-curves, extracts exact
homogeneous isocurves on the restricted patch, and compares each restricted
model curve by exact polynomial identity. A model curve may extend beyond its
edge interval. All corner vertices must equal exactly representable curve
endpoints, and the bounded patch must pass the injectivity certificate. The
writer retains the original support, emits restricted model curves, and returns
`geom.ErrUnrepresentable` before writing if required poles, normalized weights,
knots or endpoint coordinates need rounding. No approximate boundary is emitted.
The coefficient exporter currently uses maximum-weight normalization; alternative
weight scalings are not searched.

An independently generated OCCT rational graph with interior U/V bounds qualifies
both directions, including OCCT face validity and area comparison. Tests reject
out-of-domain/collapsed UV bounds, even one-ULP endpoint changes, inconsistent model
curves, reversed loops and nonrepresentable coefficient exports. Arbitrary curved
trim contours, holes, non-axis-aligned trims and multi-face shell reconstruction
remain outside this API.

### Certified rectangular spline shells

`ReadSplineShells` / `WriteSplineShells` exchange `SplineShellModel` with
`SplineShellBody{Name, Faces, Solid}`. `Faces` are bounded rectangular spline
patches; `Solid` is the derived `trimmed.Solid`. The writer uses `Faces` as its
input and does not trust a supplied `Solid` value.

Each body must have one closed shell of 4–64 positive-orientation rectangular
patches. Shared STEP vertex identities must agree exactly, each shared edge must
have two opposite loop uses, and its two p-curves must refer to exactly the two
owning supports. Every face passes the exact bounded-face proof. The resulting
patches are sewn with exact endpoint/curve matching, then must pass
`ValidateSplineRectangleEmbedding` with `Complete=true`: local injectivity,
connected vertex links, one shell, pairwise separation or certified boundary
contact, and outward orientation are required. Sewing uses a 1e-10 model-unit
validation tolerance; it does not weld nearby vertices or approximate curves.

The sufficient global certificate currently handles prism-like configurations
whose touching pairs have a planar supporting face. Unresolved certificates,
multiple shells/cavities, reversed charts, split edges, general curved-curved
contacts and nonaligned model/p-curve parameterizations fail explicitly. The
writer additionally requires lossless boundary coefficient export and aligned
forward edge parameters on both adjacent charts.

An independently generated OCCT six-patch prism with nonplanar bilinear top
`z=1+xy/4`, `0<=x,y<=2`, and bottom `z=0` qualifies both directions, including
validity, volume 5, centroid `(16/15,16/15,29/45)` and independent area comparison.
Tests reject a globally inward reflected shell, disconnected closed components,
invalid source ownership/orientation, missing p-curves and budget violations.
This is a globally certified bounded spline-shell subset, not general STEP CAD
solid interoperability.

`document.FromSplineShellSTEP` stores these certified bodies as native trimmed
assets with stable body-name labels and explicit coordinate units.
`State.WriteSplineShellSTEP` evaluates features and exports each explicitly
placed instance as a separate named body; it does not invent a STEP assembly.
Local and parent proper-rigid maps are applied sequentially in exact homogeneous
arithmetic, and unrepresentable placed coefficients fail rather than rounding.
Native persistence, shared assets, transformation-feature regeneration, exact
axis-permutation/translation placements, and inch-unit OCCT physical mass are
qualified. Scale/reflection placements and unsupported face graphs are rejected.

## Migration preflight inventory

`Preflight(reader, Limits)` performs the bounded Part21 parse and returns a
`GraphPreflight` without constructing geometry. It reports schema declarations,
entity and complex-entity counts, deterministic type-to-ID lists, cataloged
feature families, unclassified entity components and candidate reader names.
Complex entity components are inventoried separately while retaining their
shared source ID. Unknown types remain visible rather than disappearing during
partial import. Syntax errors, dangling references and parser limits fail.

This is deliberately a **declaration inventory**, not a support or validity
certificate. A known type can still have unsupported arguments, trim geometry,
placements, ownership, schema combinations or invalid topology. An empty
`Unclassified` list does not mean the file is importable. `CandidateReaders`
contains routing hints only; callers must run the selected strict reader and
accept its complete result before constructing document assets. No geometry is
approximated or silently flattened. Tests cover all checked-in independent STEP
fixtures, unknown simple and complex records, deterministic ordering, malformed
syntax/references and resource budgets.

## Certified straight tube geometry export

`WriteTubeLines(ctx, writer, TubeLineModel, TubeLineOptions)` connects native
`intersect.TubeSegment` provenance to standalone STEP degree-one spline geometry.
Each `TubeLine` supplies a name, the immutable certified segment and exact
rational paired endpoints `[uA,vA,uB,vB]` in traversal order. The helper invokes
`CertifyAffineLineExact`, requires a complete exact affine identity spanning the
entire segment, and requires lossless `ToBSpline` coefficient export. Nonbinary
UV coordinates are allowed when the resulting model curve is representable.
Model coordinates needing rounding fail with `geom.ErrUnrepresentable`.

The default segment limit is 256; each proof has explicit operation, pole and
coefficient budgets. Every line and the staged STEP stream are validated before
writing any output. Cancellation and writer errors propagate. Curves retain
names and units and use normalized traversal parameter `[0,1]`. A reversed
segment requires reversed proposed endpoints; reversing just the proposal fails.
This exports geometry only: it does not serialize the tube recipe/certificate,
infer p-curves, sew faces, or claim general implicit algebraic curve support.
The receiving application obtains an explicit spline, not a proof-carrying tube.

Tests include rational chart endpoints, reversed certified traversal, a curved
branch with matching endpoints but non-affine interior, unrepresentable model
coordinates, partial/mismatched proposals, budgets, cancellation, and atomic
validation before output. Live OCCT imports the exported curve as one valid edge
and matches the independently known 0.5-unit line length to 1e-12.

## Rectangular spline faces with one hole

`ReadHoledSplineFaces` / `WriteHoledSplineFaces` use
`HoledSplineFaceModel{Faces, LengthUnit}` with each `HoledSplineFace` carrying
`Name`, its original `Geometry`, and `Outer` / `Hole` parameter rectangles.
Each face belongs to a separate one-face open shell. Exactly one strictly
interior rectangular hole is supported; assemblies, general trim paths,
multiple holes, negative face sense and multi-face shells remain unsupported.
Existing `ReadBoundedSplineFaces` and `ReadSplineFaces` continue rejecting holes.

Both loops require four exact model spline / straight UV iso-edge pairs,
consistent source vertex identities, full endpoint agreement and the existing
bounded exact curve-identity proof. The outer loop is counterclockwise and the
hole clockwise after applying `FACE_BOUND` orientation. OCCT's false bound
orientation and an explicitly reversed edge loop are both handled without
changing the underlying model geometry. Exact parameter inequalities establish
strict containment and separation; the full outer patch must pass global
injectivity certification. All emitted edge coefficients and endpoints must
be losslessly representable, and the original support is retained.

`testdata/holed-spline-face.step` comes from OCCT's face/wire builders for the
same warped rational graph used in the IGES hole fixture. Live
`write-step-holed-spline-face` qualification confirms original and Go output
are valid one-face shapes whose areas agree within 1e-8. A separate analytic
area comparison uses 0.003 because OCCT's default quadrature differs by about
0.00194 on this fixture; the exact trim acceptance proof uses no such tolerance.
Tests cover native multi-face round trips, preserved original nets, incorrect
bound/face/edge senses, duplicate bounds, one-ULP vertex/model-curve changes,
strict containment, unrepresentable edges and resource limits.

## Rigid assemblies of certified spline shells

`WriteSplineShellAssembly` accepts `SplineShellAssemblyModel` with named local
`SplineShellAssemblyPart` products and explicit `AssemblyInstance` occurrences.
Each product may contain multiple independent `SplineShellBody` values; repeated
or nested occurrences reuse the product's geometry without baking placements.
Every body's face descriptors are staged through `WriteSplineShells` and its
complete exact shell/embedding validation. The derived `Solid` field cannot
bypass those checks. Placement uses the existing proper rigid assembly contract.

`ReadSplineShellAssembly` returns the explicit `Assembly` plus `ProductBodies`
keyed by product-definition ID. `Assembly.SplineShellBodies` decodes a selected
product in local coordinates. All represented bodies must satisfy the strict
rectangular spline-shell subset; geometry budgets apply across local assets,
not separately to every occurrence. Orphan bodies/faces/supports, duplicate body
ownership across products, cycles, scaled placements and invalid shells fail.
The shape-only `ReadSplineShells` continues rejecting assembly graphs.

The independent `spline-shell-assembly.step` fixture uses OCCT XCAF to instance
a warped six-patch body twice with different rigid placements. Live tests of
`write-step-spline-shell-assembly` and Go output preserve two products/two
occurrences, names, local geometry and transforms. OCCT validates the assembled
shapes and independently confirms volume 10 and centroid
(5, 10+16/15, 29/45). No document schema or standalone-face asset representation
was introduced.

`document.FromSplineShellAssemblySTEP` bridges this graph into existing shared
trimmed assets and occurrence-tree nodes, with bounded expansion and source
revalidation. Supplied product face descriptors must match the source graph;
prebuilt solid values cannot replace proof. Product/body names are retained in
existing labels and units remain explicit. `State.WriteSplineShellAssemblySTEP`
regenerates features, extracts certified local rectangular supports without
baking occurrence placement, and preserves shared leaf assets under an explicit
export root. Native document persistence and feature-regeneration regressions
cover the bridge; live XCAF qualification preserves the repeated geometry,
names, transforms and analytic assembly mass. No schema extension is required.

## Product-local spline-shell face colors

`ReadColoredSplineShellAssembly` / `WriteColoredSplineShellAssembly` add the
existing strict AP214 RGB fill graph to certified spline-shell assemblies.
`ColoredSplineShellBody` contains a `Body`, optional `SurfaceColor` default and
`FaceColors` overrides indexed by `Body.Faces` / source `CLOSED_SHELL` order.
Product definitions own geometry and styles; repeated instances reuse both.
The separate colored model uses named products and the same explicit rigid
occurrence structure. Uncolored readers retain their existing behavior.

Geometry must pass all spline-shell/embedding validation before styles are
resolved. The shared style parser verifies body/face ancestry, unique targets,
presentation contexts and supported override chains. It does not assume planar
geometry: certified source face counts establish the index mapping. Invalid
indices/RGB values, occurrence-level overrides and unsupported presentation
semantics fail rather than being discarded. Writer staging validates the
complete result before emitting bytes; style output follows source face order.

The independently generated XCAF `colored-spline-assembly.step` fixture has one
body fill and one face override on a shared warped part with two placements.
`write-step-colored-spline-assembly` live qualification preserves both RGB fills
within 1e-6, two products/two occurrences and independently checked volume 10.
Native exact RGB round trips, malformed targets/indices/colors and fuzz coverage
complement the live checks. No textured/material/occurrence style support is
implied.

## Multiple rectangular spline-face holes

`ReadMultiHoledSplineFaces` / `WriteMultiHoledSplineFaces` use
`MultiHoledSplineFace{Name, Geometry, Outer, Holes}` and explicit model units.
Each face has one through `MaxSplineFaceHoles` (16) inner rectangles. The older
single-hole and hole-free APIs retain their strict contracts. Source hole order,
original support nets and exact curve parameterizations are preserved.

Every boundary requires the existing exact model/UV/vertex proof and correct
winding after applying bound orientation. Strict parameter comparisons reject
nested, overlapping, edge-touching, corner-touching or outer-touching holes.
The outer patch's injectivity certificate then establishes model-space boundary
separation. Aggregate edge-use limits are checked before each face's proofs.
Boundary-proof views copy only the selected referenced graph closure, so
unrelated metadata is not recopied for every loop.

The independent OCCT `multiholed-spline-face.step` and
`write-step-multiholed-spline-face` mode qualify a two-hole rational warped face
in both directions. Both holes survive; source/export face areas agree within
1e-8. Tests independently generate invalid exact boundary graphs to verify
reader rejection, in addition to constructor checks, source sense/vertex
mutations and budget tests. Parser fuzz targets and allocation-reporting
benchmarks cover both this API and STEP preflight; IGES has a matching multi-hole
benchmark. These benchmarks measure specific fixtures, not general CAD capacity.

## Nonrectangular orthogonal spline faces

`ReadOrthogonalSplineFaces` / `WriteOrthogonalSplineFaces` preserve
`OrthogonalSplineFace{Name, Geometry, Boundary}` with 4–64 UV vertices on the
original rational spline support. The boundary is a simple counterclockwise
polygon of alternating constant-U/constant-V segments; concave L shapes are
supported. Diagonal edges, holes, redundant collinear vertices, negative face
sense and general curved UV trims are explicitly outside this API.

The decoder first rejects source weight normalization that changes represented
relative weights. It derives UV corners from adjacent source pcurve lines, verifies
shared source vertex identities and edge traversal, then compares each exact
restricted model curve with the corresponding surface isocurve. Every model
endpoint must match exactly. Exact signed area and closed segment intersection
checks prove the polygon's winding and simplicity. The full UV bounding patch
must pass the conservative global injectivity certificate, which also certifies
the polygonal subset. The writer retains source-local support geometry and
fails before output if any required spline coefficient or vertex needs rounding.

The independent OCCT `orthogonal-spline-face.step` fixture is a concave rational
warped face, constructed from explicit surface pcurves with exact representable
trim positions. Live `write-step-orthogonal-spline-face` qualification validates
both directions with area agreement within 1e-8. Some otherwise valid rational
trim positions cannot be represented exactly after the current maximum-weight
normalization; those remain explicit unsupported results, not tolerance-based
acceptances. Tests include one-ULP model/vertex/pcurve changes, sense changes,
nonfinite coordinates, self-contact, diagonal edges, winding, budgets and
unrepresentable coefficients. Existing rectangle-only readers remain strict.

Exact-certified spline trim readers reject source weights whose maximum-weight normalization requires rounding (for example, weights 1 and 3), even when the rounded support and boundary would compare equal. This applies to rectangular and holed faces and certified spline shells/assemblies. Generic standalone spline geometry import retains its ordinary float64 normalization behavior; it does not establish preservation of exact source weight ratios. Such a certified trim input is explicitly unrepresentable in the current normalized spline representation.

`ReadOrthogonalHoledSplineFaces` / `WriteOrthogonalHoledSplineFaces` add
nonrectangular orthogonal outer and interior polygons on an injective rational
spline support. `OrthogonalHoledSplineFace` retains `Boundary` and `Holes` as
counterclockwise UV polygons; STEP hole loops are emitted with opposite physical
traversal. Each polygon has 4–64 alternating axis-aligned edges, and each face
has 1–16 strictly interior, mutually disjoint holes. Concavity is supported;
contact, nesting, overlap, diagonal trims and collinear redundant corners are
rejected. Exact model isocurves, source weight ratios, source vertices and loop
senses are certified before acceptance. The earlier hole-free orthogonal API
remains strict. These are standalone surface faces, not inferred solid shells.

`ReadOrthogonalSplineOpenShells` reads connected multi-face `OPEN_SHELL` graphs
with up to 64 total orthogonal spline faces, optionally with disjoint interior holes. It retains source face
IDs, canonical edge/vertex IDs, use directions, names and length units. Each
face passes the exact spline/model/p-curve proof; an interior edge has exactly
two owning p-curves and opposite face uses. Boundary edges have one owning
p-curve. The importer checks connectedness, manifold vertex links and a nonempty
open boundary, and rejects missing/extra owners, coordinate-only unwelded
vertices and closed shells. The independent OCCT fixture joins a concave warped
patch to a complementary adjacent patch through one exact shared edge.

The result is an **open-shell incidence graph**, not a `trimmed.Solid`.
Individual face injectivity and shared-edge continuity are certified; global
cross-face nonintersection/embedding is not. The rectangle-based solid embedding
proof is not weakened or reused beyond its established scope. No solid conversion is provided by this API.


`WriteOrthogonalSplineOpenShells` stages exact local faces and merges edges and
vertices only by the explicit input topology IDs. Uses for each face must follow
its boundary order, with canonical endpoints and directions matching increasing
isocurve parameters. Original numeric STEP IDs need not survive reserialization;
shared incidence, geometry, names and units do. The complete staged output is
read back through the independent exact open-shell validator before any bytes
are written. Inconsistent endpoint identities, different shared curve interiors,
missing/misordered uses and nonmanifold connectivity fail without partial output.
Independent OCCT readback verifies the resulting two-face shell and its area.
The writer does not upgrade the local proof into a global embedding guarantee.


Open-shell `FaceHoles` is indexed by face and then hole; its contours use the
same counterclockwise convention as the standalone holed-face API. Each
`OrthogonalShellUse.Loop` is zero for the outer contour and one plus the hole
index for an interior contour. Source physical hole traversal remains clockwise.
The writer checks per-loop ownership/counts and preserves these incidences while
regenerating numeric STEP IDs. Exact per-face containment and disjointness,
shared-edge owner matching and vertex-link proofs cover all loops. An independent
OCCT fixture sews a concave holed warped face to a neighboring patch and validates
both import and exported area. This extension does not certify global embedding.

`ReadAxisAlignedSplineShells` (and its context-aware variant) is a separate
closed-solid importer for `MANIFOLD_SOLID_BREP` / `CLOSED_SHELL` graphs with
orthogonal loops and optional holes. Every support must be an exact nonrational
(or constant-weight rational) clamped affine bilinear rectangle aligned with two
coordinate axes. The source model curves, p-curves, vertices and shared-edge
ownership pass the same exact proofs as the open-shell reader. Closed manifold
incidence is mandatory. Conversion to `brep.Solid` occurs only after the planar
kernel's bounded exact embedding and outward-orientation validation succeeds.
No curved support is flattened or sampled into acceptance. Native coordinates,
source faces, holes, IDs, uses, body names and explicit units are retained.

The independent OCCT fixture is an eight-face concave L-prism (volume 36, area
72). Its imported native solid can be exported through the existing planar
`WriteAdvanced` API; that export intentionally uses planar supports rather than
claiming preservation of the original spline encoding. Curved closed shells
continue to require the existing separate certified spline-shell API.

`WriteAxisAlignedSplineShells` writes the supported closed subset back as exact
spline-supported `CLOSED_SHELL` / `MANIFOLD_SOLID_BREP`, retaining the source-local
supports, orthogonal boundaries/holes, shared incidence, names and units.
`Faces`, `FaceHoles`, `FaceIDs` and `Uses` are the authoritative source graph;
`Solid` is a derived read result and is not serialized. Numeric STEP IDs are
regenerated. The writer independently re-reads its staged output through the
closed-axis-aligned importer, including exact global planar embedding and
orientation checks, before producing any bytes. Malformed identity or missing
uses cannot fall back to tolerance welding or partial output. Live OCCT readback
checks the concave eight-face solid's area and volume.

`AxisAlignedSplineShellsFromPlanar(ctx, Model)` constructs the supported exact
spline-shell graph from native planar `brep.Solid` bodies. Each face must be an
axis-aligned orthogonal polygon, optionally with disjoint orthogonal holes.
Affine bilinear charts use world coordinate values directly as UV parameters;
source vertices and edges are preserved with globally offset positive IDs across
bodies. Physical hole loops remain clockwise while `FaceHoles` stores their
counterclockwise geometric contour. The native planar boundary must first pass
exact global embedding/orientation validation, and the completed graph must pass
the closed spline writer/readback proof. Oblique faces, nonorthogonal edges,
inward solids, disconnected per-body shells and unsupported source identity
configurations fail rather than being fitted or welded. The 64-total-face limit
and existing per-loop limits apply. Independent OCCT qualification covers a
12-face concave L-prism with a square through-hole (volume 33, area 82).

`ReadOrthogonalAffineSplineShells` / `WriteOrthogonalAffineSplineShells` are
additive closed-shell APIs for exact clamped bilinear affine supports whose
axes belong to three common mutually orthogonal direction families. Exact
rational dot/cross identities on represented source poles establish this gate;
axis lengths may be unequal or non-unit. No approximate orthogonality test,
normalization, inferred rigid placement or model-coordinate rescaling occurs.
Source UV parameters, model-space coordinates and explicit length units remain
unchanged. The original axis-aligned APIs retain their narrower gate. The model
and body aliases reuse the same source topology fields and derived planar solid.

The new reader also recognizes nonrational degree-one two-pole 2D spline
p-curves that prove exactly `(t,c)` or `(c,t)`: clamped knot endpoints must equal
the varying UV coordinates and the owning model curve's domain. This is an
exact proof-view normalization to a LINE, not curve fitting; other p-curves are
rejected. The independent OCCT fixture applies integer similarity matrix
`[[3,-4,0],[4,3,0],[0,0,5]]` and translation `(7,11,13)` to the L-prism.
Its source and re-exported volume/area are 4500/1800. This fixture is a scaled
rotation, not a claim that a rounded rigid matrix is exactly orthonormal.
Global embedding and outward orientation still require the exact planar proof.

`ReadAffineSplineShells` / `WriteAffineSplineShells` provide a distinct, broader
planar subset: nondegenerate clamped degree-one constant-weight affine bilinear
charts need not have orthogonal model-space axes or share direction families.
An exact rational parallelogram identity proves every support is affine; source
orthogonal UV loops/holes and model/p-curve identities are retained. Closed
manifold topology and exact global planar embedding/outward orientation are
still mandatory. Shear is accepted without changing model coordinates or units;
curved, warped, and merely approximately affine supports fail. The axis-aligned
and common-orthogonal reader/writer gates remain unchanged.

The independently authored OCCT fixture applies integer shear
`[[1,1,0],[0,1,1],[0,0,1]]` and translation `(7,11,13)` directly to all source
support poles before constructing/sewing the eight-face concave prism. Source
and exported mass checks give volume 36 and area `24 + 24√2 + 24√3`. These APIs
do not claim arbitrary curved spline shell support or general trimming curves.

`ReadWrappedSurfaces` / `WriteWrappedSurfaces` exchange a bounded standalone
clamped fundamental spline patch with explicit `u_closed`/`v_closed` flags.
`DecodeWrappedSurface` certifies an individually referenced support without
importing any surrounding face topology. At least one closure flag must be T;
unknown flags are rejected. Exact whole-seam rational identities establish C0,
and `WrappedSurfaceOptions.ContinuityOrder=1` additionally requires exact first
partial agreement. The default C0 proof does not imply C1, regularity,
non-self-intersection, face topology or solid closure. Generic `ReadSurfaces`
continues to treat logical flags as informational.

Lossless `WrappedSurfacePatch.ExportRecipe` supplies the original fundamental
knots, poles and weights. Source normalization requiring rounding and
unrepresentable exports fail. STEP does not preserve the wrapper's requested
continuity order, so readers explicitly choose and re-prove it. Proof options
bound surface/pole counts, cumulative arithmetic operations and coefficient
bits. This API does not synthesize OCCT periodic knot storage or equate a closed
clamped surface with a periodic control net.

The pinned OCCT [rational surface writer](https://github.com/Open-Cascade-SAS/OCCT/blob/4dbfdcc448c0b3e565488afcd626c84483172db9/src/DataExchange/TKDESTEP/GeomToStep/GeomToStep_MakeBSplineSurfaceWithKnotsAndRationalBSplineSurface.cxx)
sets flags from `IsUClosed`/`IsVClosed`. The independent fixture is a clamped
rational full-circle extrusion with U closed, U nonperiodic, V open; its C1 seam
is independently certified here. OCCT accepts the standalone re-export. Default
OCCT quadrature gives the same source/output area but is inaccurate for this
whole multi-span surface; adaptive integration agrees with analytic area `20π`
within `1e-6`. No geometry tolerances are used by the seam proof.

`DecodeWrappedSplineFace` reads an explicitly referenced full fundamental
`ADVANCED_FACE` with four oriented uses and exact spline isocurves. It preserves
canonical side edge and corner vertex IDs, including repeated seam identities.
The bounded proof checks positive unit LINE p-curves, source weight fidelity,
full-domain model curve identity, endpoint identity, cyclic incidence, and exact
support seam closure. The stored independent OCCT wrapped cylinder fixture
qualifies the singly closed case. This is a face topology adapter, not a solid,
regularity, self-intersection, or global embedding certificate. Arbitrary trimmed
periodic faces and general periodic knot conventions remain unsupported.

`WriteWrappedSplineFaces` accepts `WrappedSurfaceModel` and builds canonical
fundamental face topology from the certified axes. It emits one separate surface
model per face, with shared seam edges and vertices, stages lossless isocurve
geometry, and revalidates the complete face graph before output. Source IDs are
regenerated. Live OCCT qualifies both the cylinder and an exact represented
rational double-seam toroidal patch; the latter uses dyadic weights and is not
an ideal analytic torus. The cylinder additionally passes adaptive area checks.

`ReadWrappedSplineFaces` and its context variant read the matching standalone
surface-model graph, retaining source face/edge/vertex IDs, face names and an
explicit length unit. Multiple independent one-face open shells are supported;
assemblies, closed-shell material interpretations, unowned faces, and shared
support entities are rejected. Arithmetic budgets are divided among source
faces. Callers can pass the returned certified support/name pairs to
`WriteWrappedSplineFaces`; regenerated source IDs are intentionally different.
No sewing or global embedding is implied by this roundtrip.

`ReadWrappedToroidalBodies` (and context variant) and
`WriteWrappedToroidalBodies` provide explicit one-face `MANIFOLD_SOLID_BREP`
exchange for the canonical axis-aligned tensor rational toroid certificate.
They require a `CLOSED_SHELL`, positive face orientation, one vertex and two
oppositely used seam edges, exact p-curve/model support agreement, and the native
constructive quotient embedding/outward proof. Topological closure alone never
promotes arbitrary wrapped surfaces. Body/face names and length units survive;
source IDs are regenerated on write, and derived Solid/Embedding fields are
recomputed. This strict subset uses 9×9 quadratic quarter-profile control nets,
separable positive weights and major radius greater than radial minor radius.
General rotated/edited toroids and arbitrary periodic solids remain unsupported.

The stored `testdata/wrapped-toroid.step` is independently generated with pinned
OCCT by `write-step-wrapped-toroid`, using quarter weights .75 and tensor .5625
so source decimal serialization preserves exact separability. Live OCCT verifies
validity and adaptive area/volume agreement in both directions; this represented
rational toroid is intentionally not asserted to be an ideal analytic torus.

The document adapters `document.FromWrappedToroidalSTEP` and
`State.WriteWrappedToroidalSTEP` use existing exact trimmed assets and native
persistence. Shared references, parent/local translations, units and names stay
in the document; STEP export writes explicitly placed independent bodies rather
than inferring an assembly graph. Original asset topology and material embedding
are revalidated. Exact translations are supported; rotations and nonrepresentable
translated coefficients are rejected by this canonical subset.

`ReadWrappedCylindricalBodies` / `WriteWrappedCylindricalBodies` support explicit
three-face closed shells with one canonical singly wrapped rational wall and two
planar rational-profile caps. Each cap's 2D spline is lifted exactly through its
canonical +Z/+X plane frame and compared with the shared source model rim; cap
orientation, bound orientation and edge ownership are validated. Source full rim
edges may become four certified quarter fragments in the native solid. Material
acceptance additionally requires `trimmed.NewWrappedCylinder`'s constructive
embedding proof. This is not promotion from incidence or a circular approximation.

The independent stored `wrapped-cylinder.step` and live
`write-step-wrapped-cylinder` OCCT mode use .75 quarter weights, radius parameter
2 and heights -2 to3. Both-direction adaptive area and volume agree. Unsupported
plane frames, inward caps, mismatched model/UV profiles, missing caps and resource
overruns fail explicitly. Body and wall names and explicit length units survive;
cap names are retained and source numbering is regenerated. Wrapped face UV LINE parameters
are checked exactly; analytic-codec tolerance does not establish spline identity.

`WrappedCylindricalBody.CapNames` retains lower/upper planar face names in
addition to `Face.Name`. `document.FromWrappedCylindricalSTEP` and
`State.WriteWrappedCylindricalSTEP` preserve all three names, units and shared
native assets across save/reload. Export validates each existing asset with
`ValidateCanonicalWrappedCylinder`, then emits independently placed translated
bodies. Exact parent/local translations are supported; unsupported assets,
rotations and unrepresentable translated coefficients fail before output.

`ReadWrappedTube` / `WriteWrappedTube` exchange one four-face rational through-
bore: positive outer wall, reversed inner wall and two exact annular planar caps.
The source wall, rim, cap and p-curve ownership is checked before converting the
proof view into two canonical cylinders. `NewWrappedCylinderTube` then proves
same axis/height, exact normalized profile identity and strict radial nesting.
Source wall/cap names and units survive; native rim fragmentation is explicit.
The reader accepts polynomial spline isoparametric maps only when exact rational
Greville coefficient identities prove `(t,c)` or `(c,t)` over the full support
interval. It never accepts near-collinearity as parameter agreement.

The independent `write-step-wrapped-tube` OCCT mode constructs a .75-weight
rational capped cylinder, applies an exact half radial scaling for the bore,
and performs an OCCT Boolean cut. The stored `wrapped-tube.step` captures its
284-entity graph. Native tube construction and concentric difference both match
OCCT adaptive area, volume and centroid; STEP write/read also matches OCCT.
This API does not accept blind bores, internal cavities or general curved shells.

`ReadWrappedFrusta` / `WriteWrappedFrusta` share the exact three-face wall/cap
graph validator with cylinders, but require the distinct
`NewWrappedFrustum` material certificate: homothetic rational sections, affine
height-dependent scale, two strictly positive endpoint radii, and exact cap
agreement. The old cylinder reader and writer still reject taper. Body/all face
names and units survive, and unsupported apex degenerations remain rejected.

Independent stored and live OCCT fixtures use radii2→1 over heights-2→3, with
both .75-weight rational profiles and an exactly circular nonuniform-weight
profile `(1,1,2,2,4,2,2,1,1)`. Both directions agree on adaptive area/volume.
The circular case additionally matches analytic volume35π/3, area
3π√26+5π, and centroidZ=-1/28 in both native and OCCT calculations.

`ReadWrappedBlindBore` / `WriteWrappedBlindBore` support exactly one opening:
positive outer wall, reversed shorter inner wall, one annular cap, one full
outer disk, and one inward-material bore floor. Exact source cap/edge identities
are validated before the `NewWrappedCylinderBore` containment proof. Exactly one
height endpoint must coincide; through-bores and closed cavities are rejected by
this API. `FloorName`, both wall names, outer cap names, body name and units
survive. Either upper or lower openings are supported.

The independent `write-step-wrapped-blind-bore` OCCT fixture cuts a .75-weight
radius1 rational cutter spanningz=.5..3 from a radius2 body spanningz=-2..3.
The cutter is built from its transformed spline side and explicit analytic
planes bounded by exact rational rims. General affine-spline floor charts are
not silently treated as planes. Stored290-entity and live fixtures pass native
and OCCT adaptive area/volume/centroid comparison and STEP roundtrip checks.

`ReadWrappedCavity` / `WriteWrappedCavity` require one `BREP_WITH_VOIDS`, one
outer closed shell, and exactly one `ORIENTED_CLOSED_SHELL` void. Both underlying
three-face cylinder shells must be outward and the void orientation must be
explicitly `.F.`. Alternative source conventions, such as inward faces with a
`.T.` shell occurrence, are deliberately unsupported. Strict radial nesting,
coaxial profile identity and both strictly interior height endpoints are proved
by the native cavity constructor before accepting material. Touching/open bores,
noncoaxial cavities, repeated shells and multiple voids are rejected.

`write-step-wrapped-cavity` independently constructs an OCCT Boolean difference
with outer radius parameter2,z=-2..3 and inner radius parameter1,z=-.75..1.75,
using the exact .75 rational profile and explicit planar caps. Stored298-entity
and live fixtures qualify STEP both directions and native/OCCT adaptive area,
volume and centroid. All six face names, body name and length units survive.

Independent stepped wrapped-cylinder Boolean qualification uses exact rational
circle supports for radius2,z[0,4] and radius1,z[2,6], while OCCT separately builds
analytic cylinders and performs fuse/common/cut. The three
`write-step-wrapped-stepped-{union,intersection,difference}` modes verify volumes
18π,2π,14π; areas28π,6π,28π; centroidZ7/3,3,13/7; and native topology5,3,5 faces
with one shell and Euler characteristic2. These oracle modes qualify native
Boolean results; they do not imply a generic stepped wrapped STEP importer.

`ReadWrappedShoulder` / `WriteWrappedShoulder` provide a distinct five-face
stepped solid: two positive wrapped walls of unequal radii, one exact horizontal
endpoint contact, two exterior disk caps, and one annular shoulder. Both wall
supports, exact rim/p-curve agreement, cap ownership and shoulder orientation are
proved before the native axial union certificate is invoked. The intervals may
narrow or widen with height; gaps, overlap and reversed inner-wall bore graphs
are rejected. `LargerCapNames` identifies the larger wall's lower/upper cap;
`SmallerEndName` identifies the other exterior disk. Names and units survive.

The independent OCCT `write-step-wrapped-shoulder` mode fuses .75-profile rational
cylinders with radius parameters2/1 and touching heights[-2,3]/[3,5.5]. Its
stored290-entity graph and live source/roundtrip qualify native and OCCT adaptive
area, volume and centroid. This adapter supports one shoulder, not a general
multi-stage profile or arbitrary intersecting wrapped walls.

`ReadWrappedToroidalVoid` / `WriteWrappedToroidalVoid` exchange one explicitly
oriented toroidal void using the same strict `BREP_WITH_VOIDS` graph helpers as
wrapped-cylinder cavities. Both source toroids are outward, the inner occurrence
is `.F.`, and both supports require exact circular-profile embedding certificates.
They must share center and major radius with both inner minor semiaxes strictly
smaller. The nested-component material proof is mandatory; touching shells,
uncertified rational profiles and alternative orientation conventions fail.
Circular and coaxial elliptical tube sections are supported without approximation.

The independent OCCT `write-step-wrapped-toroidal-void` mode builds exact tensor
rational tori using nonuniform dyadic weights and performs a Boolean cut. Its
stored310-entity fixture and live both-direction tests match adaptive area/volume.
For center(1,2,3), major radius3 and circular minor radii1/.5, native and OCCT
volume is4.5π², boundary area18π², and centroid(1,2,3). The elliptical1×2/.5×1
native case additionally matches volume9π². Body/wall names and units survive.

`ReadWrappedMaterials` / `WriteWrappedMaterials` provide an explicit mixed
model of independent toroid, capped cylinder, positive-radius frustum, tube,
blind-bore, shoulder, cylindrical-cavity, toroidal-void and circular station-loft bodies.
Each `WrappedMaterialBody` contains exactly one typed payload. Read dispatch
uses explicit one-/three-face shell structure and exact support embedding/radius
certificates, never fallback probing. Dependency closures retain original source
face/edge/vertex IDs and `SourceBodyID`; names, order and common units survive.
Assemblies, shared topological ownership and unknown material graphs are
rejected. Tube/blind/shoulder dispatch additionally requires exact
source face counts, wall orientation counts and annular loop counts, followed by
the corresponding specialized ownership and material proof. Station loft dispatch
requires two disk caps, 2–64 positive spline walls and no annular shoulder; its
specialized validator proves consecutive circular station rims, exact shared
source edges and vertices, and canonical material closure. One-band frusta and
annular shoulders retain their distinct tags. Void dispatch
requires explicit BREP_WITH_VOIDS, one reversed shell occurrence, and matching
one-face or three-face outer/inner shell signatures before the specialized
toroidal or cylindrical containment proof.

The mixed writer regenerates graph IDs and validates the complete output before
writing. Live OCCT verifies that independent mixed-body area/volume equals the
sum of its independently authored source fixtures. This representation is not a
Boolean union and does not infer instance or assembly relationships. Mixed-reader
point limits conservatively include both spatial and parametric control points;
dependency traversal has an explicit depth bound.

Mixed-body qualification additionally combines all ten independently authored
families in one STEP representation and compares OCCT area/volume with their
separate source files. Explicit payload ambiguity, shared shells and even shared
vertex IDs across otherwise independent bodies are rejected. Names, source body
IDs and common length units are retained. Edge budgets count actual source uses.

`WrappedMaterialModel.SourceSchemas` preserves source FILE_SCHEMA strings as
read-only metadata. An AP242 identifier regression verifies that the same exact
supported graph remains readable and unsupported graph types remain rejected.
This is not a claim of full AP242 conformance. Writers retain their established
schema declaration and do not infer capabilities or change geometry from source
metadata. Mixed void tags enforce cumulative support, face, edge-use and point
limits and shared outer/inner shell ownership checks.

`ReadWrappedStationLoft` / `WriteWrappedStationLoft` exchange one closed circular
multi-station loft with2–64 full wrapped frustum bands and two exact planar end
caps. Each band requires a circular-profile embedding certificate. Adjacent
bands must share exact center, height, radius, source rim edge and vertex IDs;
each station rim has exactly the two expected wall/cap p-curves. Distinct seam
ownership, global face/support ownership and strictly ordered stations are
checked before `LoftWrappedCircularStations` constructs material. Source whole
circle edges correspond to native four-fragment station rims.

Explicit center/stations construct canonical bands. Optional original wall
supports are retained losslessly after exact certificate comparison; names and
units survive. Cached Solid and source topology IDs are never trusted.
`WrappedStationLoftFromSolid` first invokes native canonical topology validation
and recovers exact represented stations and original supports, including after
native reload/reindexing. Arbitrary trimmed bands or noncircular profiles remain
unsupported.

The independent OCCT `write-step-wrapped-station-loft` mode sews exact rational
bands through radii2→1→2 at heights0→2→4 and creates the two explicit planar caps.
Its stored226-entity and live both-direction fixtures match analytic volume28π/3,
area6π√5+8π, centroidZ2 in native and OCCT calculations. Three-band translated
roundtrip and unshared station rim/mismatched support rejection are also tested.

The document adapter `document.FromWrappedMaterialsSTEP` revalidates all ten
families before creating native trimmed assets with explicit family and body-name
labels. Native save/reload preserves shared assets and occurrence hierarchies.
`State.WriteWrappedMaterialsSTEP` supports all ten explicitly tagged wrapped material families, re-proving their current native topology and every
translated support. It preserves their face names and rejects missing/stale
asset labels, incorrect family tags, rotations, nonrepresentable translations
and unknown family tags explicitly. Source STEP entity IDs are not native
asset identities. Repeated document instances are written as independent placed
bodies; this API does not create or infer a STEP assembly. Independent live OCCT
qualification checks mixed toroid/cylinder/two-loft output against the sum of the
separately authored local fixtures.

`WriteWrappedMaterialAssembly` and `ReadWrappedMaterialAssembly` preserve explicit
rigid product/occurrence graphs for all ten local mixed wrapped families.
`WrappedMaterialAssemblyModel.Products` stores local tagged bodies once and
`Occurrences` refers to product IDs; the decoded `ProductBodies` map is keyed by
source product-definition IDs. Bodies retain their source body/edge/vertex IDs.
The reader validates all local bodies together, preserving cumulative budgets
and rejecting shared ownership across distinct products, extra unowned topology,
unknown material graphs and cyclic dependencies. Occurrence reuse does not
multiply local geometry or its proof budget. Proper rigid placements use the
existing assembly transform contract; no local support is transformed or rounded
by this adapter. The shape-only mixed reader continues to reject assembly input.

Independent OCCT XCAF qualification reads a reused product containing a capped
cylinder, toroidal void and circular station loft, checks repeated occurrence
placements, and compares adaptive assembly volume with twice the separately
authored local fixture volumes. This is a bounded assembly subset, not arbitrary
AP214/AP242 topology or style support.

`document.ReadWrappedMaterialAssemblySTEP` imports the authoritative STEP stream
and preserves all ten local families as shared trimmed assets. Explicit labels
retain source product-definition/body IDs, product identifiers/names and expanded
body-member ownership through native save/reload. Expansion has independent node
and depth limits. `State.WriteWrappedMaterialAssemblySTEP` currently extracts
all ten native wrapped material families, retaining multi-body products and
repeated rigid occurrences, including rotations without transforming local
supports. It requires one identity-placed root and matching provenance for every
node/member. Moved body members, stale ownership labels, inconsistent edits to
reused product definitions, feature edits and unsupported asset families are
rejected. Source numeric STEP IDs remain provenance; writer IDs are regenerated.
Live XCAF qualification compares product counts, occurrence names/rotations,
unit-scaled translations and adaptive volume before and after document reload.


Native mixed document export now recovers frusta using the existing canonical
cap/rim validator. `trimmed.Solid.ValidateCanonicalWrappedBore` additionally
recovers through-tube, blind-bore and enclosed-cavity supports only after
constructive containment proof and exact comparison of all original outer/hole
contours, shared model edges, vertices, cap pcurves and shell counts. Topology
reindexing is allowed; inward orientation and unsupported partitions fail.
Document standalone and product-assembly export use that proof, preserve the
wall/cap/floor names, and reject stale family tags. Independent OCCT tests cover
all four newly exported families, checking area, volume and translated centroid.


The final two native document families use
`ValidateCanonicalWrappedShoulder` and `ValidateCanonicalWrappedToroidalVoid`.
The former proves two unequal-radius positive cylindrical walls meet at exactly
one endpoint and reconstructs their annular shoulder; newly allocated reference
supports must pass exact whole-chart identity before comparison. The latter
proves nested circular-profile toroids with matching centers and major radii,
strictly smaller inner radial/axial radii and an explicitly reversed inner shell.
Both recoverers compare complete source topology and exact curves against the
constructive result. Independent live shoulder/void tests verify translated
area, volume and centroid. All nine families now export through both named mixed
document APIs; arbitrary edited topology still fails the native proof.

`WriteQuadraticOvalFaces` / `ReadQuadraticOvalFaces` exchange standalone affine
spline faces with the certified rational quadratic oval outer loop and optional
holes described by the IGES adapter. Every loop has one closed model spline,
one exact UV spline, an authoritative PCURVE_S1 relation, one closed EDGE_CURVE
and a shared start/end vertex. Original face/edge IDs are retained on read;
writers regenerate IDs. The reader checks complete loop ownership, unit-square
affine support, exact UV/model coefficients and weights, source endpoints,
positive face/bound/use senses, winding, containment and hole separation.
Source shape assemblies, unowned edges/curves and unknown graph forms fail.

`ReadPartitionedQuadraticOvalFaces` additionally accepts exactly four directed
quadratic quarter edges per loop, as independently emitted by OCCT. Their
clamped parameter spans must be [0,1], [1,2], [2,3], [3,4], with exact shared
source vertex IDs and common endpoint weights. Joining drops only duplicate
endpoint coefficients and introduces no rounding or fitted geometry. Both
readers retain source edge IDs; the writer emits the canonical single-loop
form. The single-loop reader continues to reject partitioned input. FACE_BOUND
outer loops in the quarter form are classified by the independently certified
positive winding; only one such outer loop is permitted.

Stored independently authored full-loop and live OCCT quarter-loop annuli both
pass the readers and independent OCCT validity/adaptive-area comparison after
canonical export. The oracle mode `write-step-quadratic-annulus` independently
creates the source from its rational IGES fixture and OCCT transfer. Fixed
quadrature can differ on these rational trims; qualification uses the existing
`step-mass-precise` adaptive integration mode. No arbitrary quadratic trims,
non-affine supports, closed material shells or sampled acceptance are implied.

`WriteQuadraticOvalOpenShell` / `ReadQuadraticOvalOpenShell` exchange a connected
open planar sheet comprising exactly one annulus and one disk filling its hole.
The common closed EDGE_CURVE/model spline is authoritative and shared by two
opposite uses; its SURFACE_CURVE owns exactly two pcurves, one for each distinct
support entity. Independent affine support charts are retained. Exact reversed
whole-curve identity proves that the noncollinear shared oval lies in both
support planes with compatible orientation. Combined with the separate simple
oval/containment proofs, the disk fills precisely the annular hole without
interior overlap or gaps. No solid or generic sewing claim follows.

Reading verifies complete shared-edge incidence, distinct owner pcurves, source
vertices, loop senses, bounded acyclic dependencies and absence of unowned
geometry/topology. Source face/edge IDs and `SourceSharedEdge` survive inspection;
export regenerates IDs. Standalone face readers still reject this shell graph.
An independently authored fixture has a disk chart scaled relative to the
annulus chart; live OCCT verifies two faces and equal adaptive source/export
area. The resulting area also matches the unpartitioned outer oval. Same-sense
uses, duplicated owner pcurves, duplicated unshared rims and extra shells fail.

`WriteWrappedRimFillet` / `ReadWrappedRimFillet` exchange one canonical circular
cylinder with one or both exact circular rim fillets. `WrappedRimFilletBody`
stores stock base `Center`, `Radius`, `Height`, two `Fillets`, ordered wrapped
walls, cap names and units. Zero at one end retains that sharp rim; both zero,
fillet radii at least half the stock radius, and overlapping blend heights fail.
The graph contains a cylindrical middle, one or two independent rational
quarter-toroid supports, full shared circular rims, owner pcurves and two planar
caps. Source whole rims correspond to four native rim fragments.

Readback checks exact source support/edge agreement, shared rim and vertex IDs,
unique seams, complete cap orientation and pcurve ownership. Endpoints only
recover candidate stock dimensions; acceptance requires whole-chart exact
identity against the constructive `FilletWrappedCylinderRims` material proof.
An interior control edit that leaves all boundary curves unchanged is rejected.
`WrappedRimFilletFromSolid` first invokes the native topology recoverer and
retains original support charts. Existing cylinder/frustum/station-loft APIs
remain strict; none silently accept toroidal meridians. The internal cap graph
stager is not a public material validation bypass.

The independent OCCT `write-step-wrapped-rim-fillet` fixture authors separate
9×3 rational toroid bands and a 9×2 cylinder, then sews exact planar caps. Both
STEP directions pass validity and agree with circular-fillet analytic area and
volume within1e-7. Lower-only, upper-only, translated, names and native-recovery
roundtrips are tested; altered interior controls, duplicated pcurve ownership,
unwelded rim IDs and stale provided wall geometry are rejected.


Mixed materials now include the explicit `RimFillet` payload as a tenth family.
Dispatch requires two disk caps, all positive walls, exactly one linear cylinder
band and one or two quadratic bands before invoking the complete specialized
fillet validator. This remains distinct from linear meridian station lofts;
classification never substitutes endpoint samples for material proof. Existing
nine-family behavior and explicit void source types remain unchanged.

Both document mixed adapters preserve the rim-fillet family and wall/cap names.
Native extraction revalidates original topology before recovering stock/radii;
standalone translated output preserves exact represented center and support
coefficients, rejecting lossy placements. Product assembly keeps local geometry
shared and retains independent rigid occurrence transforms. Native reload,
repeated translated instances, a rotated shared XCAF product, and independent
ten-family aggregate OCCT area/volume are qualified.

`ReadColoredWrappedMaterialAssembly` / `WriteColoredWrappedMaterialAssembly`
add product-local body RGB defaults and named face overrides to the twelve-family
wrapped material assembly graph. Face color keys are explicit, nonempty unique
`ADVANCED_FACE` names within a body, so shell reordering and canonical geometry
regeneration cannot silently move a cap or blend style. Unknown/ambiguous names,
invalid RGB components, occurrence-level styles and unsupported style graphs
fail. Explicit void-shell faces are enumerated outer shell first, then voids,
only after the full material geometry and ownership proof; names remain the
public bindings. Existing uncolored APIs retain their geometry-only behavior.
The writer stages and re-reads all geometry and styles before output.

The document adapters `ReadColoredWrappedMaterialAssemblySTEP` and
`State.WriteColoredWrappedMaterialAssemblySTEP` preserve these bindings in
existing shared-asset label attributes, alongside source product/body provenance.
No schema change is required. Native save/reload keeps repeated occurrences and
local styles; stale source IDs, renamed styled faces and malformed style payloads
are rejected. Optional XCAF tests qualify a repeated filleted cylinder with body,
cap and blend fills, including export after native document reload. General
materials, textures, alpha and per-occurrence style overrides remain unsupported.

`ReadWrappedStationBore` / `WriteWrappedStationBore` support an explicit closed
circular through-bore with 1–64 frustum bands on each of its independent inner
and outer lofts (at least one wall must have multiple bands) and two planar annular caps. The reader checks original directed
wall/cap incidence and source ownership, normalizes the reversed inner walls in
separate positive loft proof views, then requires exact coaxial nesting and
`ValidateWrappedCircularStationBore`. Both families must span the same height;
intermediate station partitions may differ. Original wall charts, rim/vertex
IDs and cap names are retained. `WrappedStationBoreFromSolid` independently
validates native topology before recovering losslessly representable stations
and original supports. The writer recertifies the full joined graph before
output. Blind/cavity topology, intersecting walls, unsupported chart forms and
unowned source geometry fail. An independently built OCCT rational loft Boolean
fixture and live bidirectional tests compare exact frustum area/volume formulas.
The additive `WrappedMaterialBody.StationBore` tag also includes this exact subset
in mixed materials and shared product assemblies. Its two positive/negative band
families and exactly two annuli distinguish it from solid loft, fillet and tube
graphs; the specialized proof remains mandatory. It is the eleventh family.

The mixed document adapter imports `station-bore` assets and recovers native
exports through `ValidateWrappedCircularStationBore`. Outer/inner wall names and
annular cap names remain explicit asset attributes. Standalone export permits
only exact translations; shared assembly occurrences retain rigid transforms
separately. Named face styles apply to both wall families. Live XCAF tests cover a
reused two-body product containing a station bore and a rim fillet, including
native save/reload, a rotated occurrence and per-face colors. The independent
mixed mass corpus now includes all twelve families.

Single-band station-bore walls use the existing exact capped-frustum proof;
the opposite wall remains multi-band, and the standalone station-loft contract
is unchanged. Both-single inputs are rejected by the station-bore API; the
existing cylindrical tube family retains its own strict domain. Independent
OCCT fixtures qualify a two-band exterior and a single tapered interior.
The ordinary wrapped tube writer now converts staged disk bounds to
`FACE_BOUND` when they become annular holes; its reader rejects a hole falsely
marked `FACE_OUTER_BOUND`. Existing independent tube fixtures and live area/mass
comparisons cover this correction.

`ReadWrappedAnnularFrustum`, `WriteWrappedAnnularFrustum` and
`WrappedAnnularFrustumFromSolid` cover exactly one circular frustum band per
wall, with at least one tapered wall, common heights and strict coaxial nesting.
The four-face graph retains exact pcurves, model seams and both annular cap roles.
Two cylindrical walls are explicitly excluded and remain in `WrappedTube`.
Native recovery validates complete topology and retains original wall supports.
An independent OCCT sewn-loft cut fixture qualifies import and export against
analytic frustum area and volume, including the stored source edge ancestry.

`WrappedMaterialBody.AnnularFrustum` is the twelfth mixed family. Classification
uses exact support embedding and represented endpoint radii, followed by the
specialized graph/material proof; no fallback-reader guessing is used.
The document `annular-frustum` asset path retains outer/inner wall and cap names,
exact standalone translations, shared rigid occurrences and named surface styles.
Native save/reload and live XCAF repeated-product style qualification are covered.
Positive-radius widening, narrowing and one-cylinder/one-taper configurations are
supported; apexes, intersecting walls, differing heights and additional bands
are rejected by this distinct API.

`ReadCurvedOvalFace` / `WriteCurvedOvalFace` exchange one positive open face on
an exactly represented rational biquadratic Bézier support over `[0,1]²`.
The outer boundary is a canonical rational quadratic oval; `Holes` allows up to
16 clockwise interior ovals. Each loop has four explicit shared endpoint
vertices, four authoritative model splines and four independently owned pcurves.
The reader proves whole-curve equality to exact tensor composition, including
raw source rational weights, and proves global support injectivity through its
XY projection. A curved graph can therefore be accepted without flattening or
sampling. These APIs do not construct or assert a material solid.

Containment requires concentric homothety or strict inclusion of the hole's
control rectangle in the outer oval's inscribed diamond. Hole control rectangles
must be strictly separated on at least one axis. This conservative subset rejects
other arrangements even when they might be geometrically valid. Source bounds
must list the outer counterclockwise loop first, followed by clockwise holes;
cyclic quarter rotations are normalized, while incorrect roles, ownership,
source vertices, source coefficients and unused topology are rejected. Surface
and UV weight normalization must be lossless. Model weights are checked directly
as exact raw homogeneous coefficients, without a rounding normalization.

The writer derives all model curves again and requires exactly representable
Euclidean poles and a common integer weight scale. Unrepresentable coefficients
fail before output. Options bound pole counts, arithmetic, coefficient size and
injectivity cells; source limits also bound all points and edge uses. The older
affine `ReadQuadraticOvalFaces` contract remains unchanged. `ModelCurves`,
`HoleModelCurves`, `SourceEdges` and `SourceHoleEdges` describe read provenance,
not editable writer inputs.

Pinned OCCT independently constructs the stored `curved-oval-face.step`,
`rational-curved-oval-face.step` and `curved-oval-hole.step` fixtures from rational
quadratic UV arcs and separately derived quartic model coefficients. Both-direction
live checks require OCCT validity and matching adaptive surface area, including
a non-flat polynomial bowl, a genuinely rational support and a curved annulus.

`ReadCurvedOvalOpenShell` / `WriteCurvedOvalOpenShell` add a two-face connected
sheet: a curved oval annulus and a disk with a distinct rational biquadratic
support chart. Four common `EDGE_CURVE` records have opposite directed uses and
two independently owned pcurves. Each owner passes the exact composition and
injectivity checks above; the four complete seam curves must agree exactly in
reverse direction. The graph requires exactly eight unique edges, twelve uses,
two faces and one open shell. Unknown ownership, duplicate bounds, extra topology,
dependency cycles and inconsistent vertices fail. Original face and canonical
edge IDs are retained; the writer derives geometry from the supplied supports
and trims and validates its finished graph before output.

This API proves per-face embedding and the stated connected seam topology. It
does not assert a general global shell embedding certificate or material closure.
The independent `curved-oval-shell.step` fixture uses a nonplanar bowl annulus and
a separately constructed restricted/reparameterized bowl fill chart. OCCT sews
the faces independently; both-direction checks require valid topology and matching
adaptive area. Source face ordering is irrelevant. The single-face reader still
rejects this graph.

`SplineLensBody`, `ReadSplineLensBody` and `WriteSplineLensBody` add a distinct
closed material family: two original rational biquadratic graph supports with a
common unit XY chart, sharing an interior elliptical rim. `Shear` defaults to
zero; the exact conjugate UV basis is `(Radii.X,0),(Shear,Radii.Y)`. The upper
support is strictly above the lower inside that ellipse and below it outside.
Both complete supports must lie strictly above the zero plane, enabling reuse
of the native exact graph-contact Difference proof. Four unequal-end-weight
rational quadratic pcurves (`[1,1,2]` before normalization) represent the exact
ellipse without rounding an irrational circle weight. Exact support composition
proves each complete model seam; all four source vertices and shared ownership
are checked before native material construction.

The STEP graph has one `MANIFOLD_SOLID_BREP`, one `CLOSED_SHELL`, two faces,
four shared edges and eight directed uses. Upper face/bound senses are positive;
lower face/bound senses are reversed. OCCT's reuse of one `EDGE_LOOP` by two
oppositely oriented bounds is accepted explicitly. Names, units and original
body/face/canonical edge IDs are retained. Unknown graphs, inverted material,
wrong ellipse, mismatched seams and extra topology fail. This API does not extend
the equal-end-weight `CurvedOvalFace` family or claim general lens recognition.

The stored `spline-lens.step` is independently constructed by OCCT from two
nonplanar paraboloids and exact rational circular seams. Native mass is compared
with the analytic lens volume. Both-direction OCCT validity and mass agree;
OCCT's own adaptive trimmed-face integration has approximately `1e-9` volume and
`5e-8` area residuals against the analytic fixture despite a requested `1e-12`
integration epsilon. PlainSolid-exported geometry matches analytic mass at a tighter `1e-10` test
tolerance; the independently authored source comparison accounts for the observed
integration residual. Exact support coefficients survive the roundtrip unchanged.

The document bridge `ReadSplineLensSTEP` preserves validated source body, upper/
lower face and four canonical seam IDs in typed asset metadata. Its native
trimmed asset survives ordinary document save/reload. `FromSplineLensSTEP`
re-proves supplied descriptors through a staged graph, whose IDs become the
provenance. `State.WriteSplineLensSTEP` requires one identity-placed occurrence;
it retains editable body/face names and units, checks the stored geometry/role
binding, and invokes `SplineLensFromSolid` to re-prove the complete canonical
native supports, trims, edges, vertices, ordering and tolerance. Unsupported
placements, additional occurrences, stale source roles, changed geometry and
reindexed assets fail before output. The binding detects stale metadata; it is
not an authentication mechanism. No generic document schema was added. A live
OCCT → STEP → document → native reload → STEP → OCCT chain checks validity and
analytic lens mass.

`SplineLensAssemblyModel`, `ReadSplineLensAssembly` and
`WriteSplineLensAssembly` preserve named local lens products and explicit repeated
occurrences. Each product has at most one lens body; assembly-only products and
nested occurrences are retained. This exact placement subset accepts proper
signed-axis rotations and finite represented translations. Source
`ITEM_DEFINED_TRANSFORMATION` starts at the identity frame, so frame inversion
or origin subtraction cannot round the local map. Near-rigid, reflected, scaled
and other source-frame configurations are rejected. Geometry is never flattened
or rotated into the common-XY support charts.

The reader proves each local material once, rejects cross-product shared topology
or unowned bodies, and applies aggregate point/face/edge/body and arithmetic
limits. `document.ReadSplineLensAssemblySTEP` stores a single shared native asset
per product, with separate product/member provenance and occurrence maps.
`State.WriteSplineLensAssemblySTEP` re-proves local materials and checks consistent
reused products, source roles and unmoved body members before exporting. A native
save/reload preserves sharing and rigid placements. The independent XCAF
`spline-lens-assembly.step` fixture has one local lens used twice, including a
90-degree axis rotation with translation. Both-direction XCAF checks verify
product reuse, exact occurrence transforms and analytic aggregate volume.


The sheared lens extension retains exact rational quadratic form coefficients
and uses the native quadratic-ellipse material proof. `Radii` are positive
triangular chart scales, not principal-axis radii. Every affine trim control must
be exactly representable and strictly inside the unit support domain. General
rational-native poles/endpoints without an exact codec representation fail with
`ErrUnrepresentable`; a common homogeneous scale does not make a nonrepresentable
Euclidean pole representable. This is not an arbitrary rational-decimal STEP
backend. Source support/model/pcurve ownership and material orientation remain
unchanged. `AffineSplineLensFromSolid` re-proves sheared canonical native assets;
the original axis-only recoverer retains its zero-shear contract.

Document lens and lens-assembly metadata preserve `step.lens.shear`; legacy
zero-shear assets without this attribute remain supported. The independently
constructed `sheared-spline-lens.step` fixture has a rotated principal-axis
ellipse and two nonplanar original supports. Both-direction OCCT validity,
analytic volume and independently integrated area qualify the extension.

Lens supports may now retain independently refined clamped degree-two knot
partitions. Interior knot multiplicity must be at most two; full-multiplicity
breaks are rejected. Each complete seam quarter must be certified inside one
recovered exact knot cell, including exact endpoint knot contacts. The composed
model curve is proved on the full original parameter interval. Interior crossings
that need several model pieces or unresolved algebraic parameters remain explicit
unsupported cases; no curve sampling or support flattening substitutes for proof.
The existing pole, cell and arithmetic budgets bound these checks.

The independent OCCT `refined-spline-lens.step` fixture inserts a U midpoint knot
once and a V midpoint knot twice in only the lower support. Read/write and native
document save/reload preserve all original poles, weights, knot values and
multiplicities. Additional tests retain different partitions on both supports,
reject a single-ulp lower-pole change, reject full-multiplicity breaks and interior
seam crossings, and check OCCT validity plus analytic lens mass in both directions.

`ReadPartitionedSplineLensBody` / `WritePartitionedSplineLensBody` add a separate
bounded lens path for exact rational crossings of original support knot lines.
They split the canonical conic quarters at the union of both charts' certified
cuts, retain both original spline supports, and compare each complete source
model curve and both owner p-curves to exact surface composition. Each
`LensSeamPiece` records the source edge, canonical quarter, rational parameter
interval, and original upper/lower knot cells. The native material still uses
four canonical conic quarters; source pieces are distinct provenance, not native
edge IDs. The old four-edge reader and document adapters remain strict.

This path accepts at most 128 seam pieces and rejects unresolved algebraic knot
cuts, extra/missing subdivisions, unsupported source parameterizations, and
coefficients that cannot be written without rounding. Raw represented rational
curve weights are retained before any floating normalization. An independently
constructed six-edge OCCT fixture crosses a lower-chart knot at exact quarter
parameters `1/2` and `1/3`; live checks validate the exported solid and analytic
mass in both directions. Its OCCT exporter explicitly uses 17 significant digits:
the default decimal precision rounds this fixture's support coefficients and is
correctly rejected by the exact reader. As with the unsplit fixture, pinned OCCT's
source adaptive mass integration has a small residual (volume below `2e-9`, area
below `1e-7`); regenerated output is checked more tightly.

The additive document functions `ReadPartitionedSplineLensSTEP`,
`FromPartitionedSplineLensSTEP`, and `State.WritePartitionedSplineLensSTEP`
preserve that source partition through native document persistence. They keep a
real canonical trimmed asset plus bounded source-body/face/edge metadata,
rational quarter intervals, and both original knot-cell lineages. Export checks
the native geometry binding and re-derives every interval and cell; editing a
metadata checksum does not bypass geometric lineage validation. Names and units
may change; unsupported placement, extra occurrences, stale geometry and stale
source roles fail before output. Source identifiers describe the import graph,
not newly assigned export entity numbers. This standalone adapter does not infer
assembly occurrence semantics and does not broaden the existing four-edge lens
adapters. A live chain checks independent OCCT STEP → document → native reload →
STEP → OCCT validity and analytic mass.

`ReadPartitionedSplineLensAssembly` / `WritePartitionedSplineLensAssembly`
extend the explicit product staging path to partitioned local lenses.
`ProductPieces` retains each product's source seam ancestry; repeated occurrences
share a product and canonical native asset. Source topology is exclusive to its
product, cumulative edge-use budgets are checked, and proper signed-axis
rotations plus finite translations remain separate occurrence data. Existing
four-edge assembly APIs stay strict. The corresponding document functions
`ReadPartitionedSplineLensAssemblySTEP` and
`State.WritePartitionedSplineLensAssemblySTEP` retain the shared asset and its
six-edge provenance through native reload, reject inconsistent product reuse,
moved body members and stale lineage, and export explicit occurrences. An
independent XCAF fixture uses one six-edge lens twice, translated and rotated;
OCCT accepts regenerated assembly/document output with the expected aggregate
area and volume. No source-chart flattening or rounded general rotation is used.

`ReadRefinedSplineLensBody` / `WriteRefinedSplineLensBody` provide a separate
source-basis-preserving lens path. Each seam piece carries three independently
partitioned splines in model/upper-UV/lower-UV order. The reader proves their
complete parameterized curves equal the exact seam composition, even when their
internal knot partitions differ; the writer retains original poles, knots and
losslessly normalized weights and proves them again before output. The original
single-Bézier partitioned APIs remain strict. This variant accepts clamped,
continuous unit-domain curves of degree at most 16 and at most 64 poles per
curve, subject to existing operation/coefficient/parser limits. Source weight
normalization requiring any rounding is explicitly unsupported, as are arbitrary
reparameterizations and unproved curve/model edits. It is not automatically
promoted into the existing document/assembly adapters.

The independent OCCT fixture refines each model curve at `3/4`, its upper p-curve
at `1/2`, and its lower p-curve at `1/4`. Its radius `105/512` permits exact dyadic
refined poles. Roundtrip tests compare every original curve basis and reject
one-ULP changes in each role. Pinned OCCT accepts regenerated original-basis and
canonical output with tight analytic mass checks. Its own source STEP shows a
small transfer/integration residual (about `4.32e-9` volume and `3.14e-7` area),
unchanged by span-aware tighter quadrature; the source-only bound is `5e-9` /
`5e-7`. This numerical reference limitation does not relax the exact source
curve or native material proofs.

The explicit refined-lens document adapters now retain those three source bases
in a bounded native recipe: `ReadRefinedSplineLensSTEP`,
`FromRefinedSplineLensSTEP`, and `State.WriteRefinedSplineLensSTEP`. Degree,
poles, knots and already losslessly normalized weights survive native reload.
The recipe is bound to both canonical native geometry and source ownership;
export independently re-proves all three curves, so recomputing a checksum after
changing a model/owner pole does not make the edit acceptable. Recipe size is
limited to 64 KiB; curves requiring rounded weight conversion fail closed.
Existing non-refined document exporters explicitly reject recipe-bearing assets.

`ReadRefinedSplineLensAssembly` / `WriteRefinedSplineLensAssembly` expose
`ProductCurves` / `SeamCurves` alongside local material and source seam roles.
The matching document assembly adapters retain one recipe per shared native
asset, explicit product ancestry, and separate signed-axis rigid occurrences.
An independent XCAF fixture repeats one refined lens at translated and rotated
placements. Native save/reload and regenerated STEP preserve all owner knot bases
and sharing; OCCT validates aggregate area/volume. No native document schema
change, implicit instance flattening or general rotation rounding is involved.

`RationalSplineLens` / `RationalLensCurve` and
`ReadRationalSplineLensBody` / `WriteRationalSplineLensBody` provide an additive
raw-weight route when a source maximum-weight normalization is not exactly
representable. Positive represented source weights are converted directly to
exact homogeneous spline controls; neither their scale nor Euclidean poles are
rounded. The original normalized `RefinedSplineLens` contract is unchanged.
The new route still proves full owner/model composition, rational cut coverage,
material orientation and exact source topology with bounded curve degree, poles,
operations and coefficient sizes. Invalid weights and one-ULP source changes fail
before output. Irrational cap/rim vertices and arbitrary algebraic STEP trims are
not introduced by this API.

An independent seven-edge fixture concurrently refines the upper support in V,
the lower support in U, and model/owner curve bases on distinct pieces. Radius
`263340/2^22` makes the support coefficients and midpoint-refined Euclidean poles
representable while raw weights such as `4,4,5` cannot be normalized losslessly.
All original supports, poles, knots and raw weights survive read/write exactly.
Both independent-source and regenerated OCCT solids meet analytic volume/area
checks at `1e-10` / `1e-9`; no sampled geometry acceptance or relaxed source mass
bound is used for this fixture. The native vertical-plane capped algebraic lens
remains a separate certificate and is not silently converted to this STEP family.

Raw-weight lens document adapters (`ReadRationalSplineLensSTEP`,
`FromRationalSplineLensSTEP`, `State.WriteRationalSplineLensSTEP`) retain a bounded
64 KiB native recipe without maximum-weight normalization. Source seam intervals,
both support-cell ancestries and original model/owner coefficients are bound to
the canonical exact native material. Export re-proves the raw curves after native
reload, rejecting stale geometry or even rehashed coefficient tampering.
`ReadRationalSplineLensAssembly` / `WriteRationalSplineLensAssembly` and their
matching document assembly adapters retain one such recipe per shared product,
separate proper signed-axis rotations/translations, names and units.

`LensStyle` adds strict product-local RGB body fill and upper/lower face overrides
to the raw-weight family. These editable styles survive native reload without
changing geometry bindings. Existing non-raw exporters reject unsupported recipes
or styles rather than discard them. Occurrence overrides, textures, alpha and
lighting remain unsupported. A fresh independently authored XCAF fixture repeats
the seven-edge lens at translated/rotated placements with distinct face fills;
OCCT verifies regenerated styles (within its existing `1e-6` RGB conversion
precision) and tight aggregate analytic mass. Invalid colors, unowned/occurrence
style targets, oversized recipes and changed native assets after reload are
rejected before output. Native document schema and core topology are unchanged.

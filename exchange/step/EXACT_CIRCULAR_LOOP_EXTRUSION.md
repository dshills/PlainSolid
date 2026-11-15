# Exact line/circle extrusion STEP output

`WriteExactCircularLoopExtrusion` exports the independently certified native
`trimmed.ExtrudeExactCircularLoop` family as a standard rooted analytic STEP
B-rep. It reconstructs from the bounded authoritative recipe before writing;
it is not a general native solid recognizer or a new general STEP reader.

The domain is 3..64 ordered CCW line/minor-circle segments, strictly positive
circular radius or zero for a line, exactly coincident represented endpoints,
nonzero signed height, and a proper signed-coordinate frame. Native complete
trim and continuous shared-edge proofs must succeed. Circle endpoints lie
exactly on their authored center/radius, with positive cross product selecting
a sweep strictly between zero and pi. STEP uses the same original vertices,
3N shared edges and N+2 faces. Negative height keeps the native low/high cap
order. Each arc owns one cylindrical wall and two circular rim edges; lines
own planar walls and linear rims. No surface approximation is performed.

The existing rounded-rectangle writer is not widened: it depends on cardinal
quarters and angular pcurves. Here nonbinary rational native middle controls
are never exported as rounded spline poles. Exact standard `CIRCLE`, `LINE`,
`PLANE` and `CYLINDRICAL_SURFACE` entities describe their complete supports;
`EDGE_CURVE` endpoints/senses and model-only face loops select the finite
boundary. Native rational and STEP angular curve parameters differ: this is
exact geometry and directed topology transfer, not identical parameterization.
Oblique planar wall frames use exact tangent/normal direction ratios, allowing
STEP's standard normalization without a rounded binary64 normal. No pcurves
are invented. Unknown profiles, failed native proof, unsupported placement,
unrepresentable posed vertices/centers or bounded scalar conversion fail
before the output writer is called. Scalars use exact finite decimals with
256-bit rational numerator/denominator caps. Existing unit writer rules apply.

## Independent qualification

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` independently constructs
an analytic rounded 3-4-5 triangle: source triangle (0,0),(4,0),(0,3), outward
radius 5/8, height ±2. It builds actual circular edges, a planar face and an
OCCT prism; no PlainSolid output is used to construct this reference. Its
original STEP is retained at
`testdata/occt-exact-rounded-triangle-extrusion.step`. A second reference uses
negative height and rigid pose (x,y,z)→(z+1,x−2,y+3).

The native profile contains exact nonbinary homogeneous controls and explicitly
fails ordinary float spline conversion. Original reference and generated STEP
both pass conventional OCCT `TransferRoots`, BRepCheck, and unique topology
(1 solid, 8 faces, 18 edges, 12 vertices). Independently derived measures are
V=27+25π/32 and A=51+105π/32; adaptive OCCT integrals agree within 1e-8 and posed
centroids agree within 1e-9. This proves actual transfer, not only STEP syntax.
The retained original's angular pcurves are not claimed exact-importable by
this writer-only API.

```
sh tools/occtref/build_exact_circular_loop_extrusion.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-exact-circular-loop-extrusion
PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP=/tmp/plainsolid-exact-circular-loop-extrusion \
 go test ./exchange/step -run '^TestExactCircularLoopExtrusionSTEP' -count=1
```

Tests also cover exact native reload, incomplete/contradictory endpoints,
wrong radius, canceled construction, invalid units, output atomicity and short
writes. No new external modules or changes to general native/STEP acceptance.

## Strict canonical readback

`ReadExactCircularLoopExtrusion` is a separate bounded reader for the exact
rooted model-only writer grammar. It recovers the signed height from the two
cap stations and original wall origins, and recovers each original segment
from the lower rim's model support and endpoints. It reconstructs the exact
native material and compares the entire generated entity graph, including
shared references, raw exact numeric values, direction ratios, all face/bound/
edge senses, units and registered product metadata. This prevents an apparent
circle or plane match from hiding inconsistent endpoints, complements or
ownership. Detached entities and unsupported topology fail atomically.

The result retains original body/shell, low/high cap, ordered wall, both rim,
vertical-edge and both vertex role IDs. `Native` is local to the exact recovered
`Descriptor.Frame`; it is not silently baked. Numbering/record order and exact
equivalent numeric spellings may differ; source role order and topology may
not. Limits cap source bytes/entities/values/depth, and context cancellation is
checked before reconstruction and return. Native reload and signed source →
read → write → conventional OCCT transfer are qualified.

## Official OCCT model-only probe

The same independent author now supports `write-model-only output.step` using
`DESTEP_Parameters.WriteSurfaceCurMode=false`. Pinned OCCT documents that member
as controlling parametric curves; it is the parameter equivalent of
`write.surfacecurve.mode=0`. The retained
`testdata/occt-model-only-rounded-triangle-extrusion.step` has 242 entities,
no PCURVE or SURFACE_CURVE, and passes ordinary TransferRoots with the same
analytic measures and topology. It was authored directly from the independent
OCCT geometry, not normalized from our output.

This original is **not yet accepted by the canonical reader**: it has wall-first
faces, FACE_BOUND records, differing original face/bound senses, normalized
oblique direction ratios and standard OCCT product/representation metadata.
There is no demonstrated numeric impossibility here: (-0.8,0.6,0) is the exact
3:4:5 direction ratio, and the decimal cap frame origin remains on the same
exact plane. Those equivalences require a separate source-topology proof;
rejection is a grammar limit, not an approximation claim. Existing generic
readers are unchanged. Live tests retain both original vendor variants and
compare the strictly reconstructed canonical output against OCCT geometry.

## Independently authored AP214 model-only input

`ReadOCCTExactCircularLoopExtrusion` now supplies the separate positive route
for the retained original 242-entity OCCT file. It does **not** call the
canonical whole-file matcher or tolerate rounded geometric discrepancies.
The first scope is one AP214 product, metre/millimetre SI context, simple
line/minor-circle profile, unposed model Z extrusion and binary-lossless
vertices, circle centers and radii. Arbitrary exact decimal plane chart origins
and proportional line/plane direction ratios are evaluated as rationals.
The source is normalized to a positive height and a local frame translated to
the low cap; historical signed extrusion intent is not inferred.

The proof recovers the cap pair from exact plane normals/stations and the CCW
profile from the low cap's material-oriented cycle. Original FACE_BOUND sense,
ORIENTED_EDGE direction and EDGE_CURVE direction are distinct checks. Every
original vertex and edge maps bijectively to a native role; walls are recovered
from rim incidence. Each face cycle must equal its native material cycle up to
cyclic rotation. Every shared edge has exactly two opposite source uses.
Original support equations, circle endpoint membership/minor directed sweep,
line collinearity, plane containment and outward face normals are checked
exactly. Native complete trim and continuous shared-edge proof then establish
material geometry. Original records, shell face order and loop starting index
may change without losing role IDs.

The genuine file uses **−Z cylindrical support axes** and outward face sense.
Both ±Z axes have the same outward radial normal (both cylinder parameter
partials change consistently); the reader proves this separately from the
+Z model circles' directed arcs. It does not infer arc direction from the wall
axis. Circle reference directions and source EDGE_CURVE same_sense remain
strict. No authored pcurves, arbitrary poses, holes, unknown geometry, extra
products or tolerance repair are admitted.

The complete single-product dependency closure and exact SI context are
validated, with unknown/detached entities rejected. Original product metadata
and all cap/wall/rim/vertical/vertex IDs remain accessible separately from the
native topology. `WriteOCCTExactCircularLoopSource` re-proves source before
re-emitting its original IDs/metadata. Canonical generation remains via
`WriteExactCircularLoopExtrusion` and produces its own new product graph.

Live qualification covers fresh independent OCCT model-only authoring → strict
native import → native JSON reload and original/canonical STEP export → OCCT
TransferRoots, topology and analytic measures. Adversaries cover tiny exact
line/plane ratio changes, radius and endpoint discrepancies, wrong normals,
coedge directions, malformed curves, unit declarations and product ownership;
record/face/cyclic-loop reorder and a nonbinary exact collinear LINE origin
are positive cases. The earlier canonical reader remains strict and continues
to reject this vendor grammar.

## Source-bound native persistence

`Write/ReadExactCircularLoopExtrusionJSON` and
`Write/ReadOCCTExactCircularLoopExtrusionJSON` use distinct version-one tags:
`plainsolid-step-exact-circular-loop-extrusion` and
`plainsolid-step-occt-exact-circular-loop-extrusion`. The original STEP remains
authoritative. One complete mode-specific source reproof checks the stored
native material, exact recipe/frame, original body/shell, cap/wall/rim/vertical/
vertex identities and (in vendor mode) original product metadata. All original
support and metadata records remain in the retained source; native claims
cannot substitute different geometry or ownership. Canonical signed height
and placement are retained; vendor positive-height normalization remains
explicit and does not invent historical intent.

Reload rebuilds from STEP and compares every claim, rejecting wrong tags,
versions, unknown/trailing JSON, forged poses/units/roles, altered native
geometry and stale source supports. Source and envelope limits are separate
(1 MiB source maximum; default/maximum 8 MiB envelope). Cancellation and
all-or-none reads/prevalidated writes are preserved. Existing native formats
are unchanged. `WriteExactCircularLoopExtrusionSource` complements the vendor
source writer; both re-prove before preserving original metadata and IDs.

Tests cover stable native bytes, positive/negative canonical sources, genuine
vendor source, source/canonical re-export, corruption, source/byte quotas,
cancellation before/during input, and short writes. Pinned OCCT live tests now
include native envelope reload before regenerated STEP transfer in both modes.

## Separate proper signed-axis vendor pose mode

`ReadPosedOCCTExactCircularLoopExtrusion` adds a separate exact rotation mode.
A source CIRCLE frame must provide proper signed-coordinate axes. Every
coordinate-bearing body dependency (all 3D CARTESIAN_POINT and DIRECTION
records) is inverse-rotated using exact rational arithmetic into a temporary
proof view. The unchanged local vendor gate then authenticates all supports,
loops, owners and metadata. The original representation frame must be neutral;
a coordinate dependency shared with it must remain invariant. No UV data is
rotated (the underlying model-only gate rejects pcurves), and no original
source records are changed.

The result retains the actual original source, role IDs, selected source CIRCLE
ID and exact rotation from normalized axes to original world axes. `Native` remains local and the
reconstructed `Descriptor.Frame` maps it to original world geometry. Height is
positive-normalized; translated profile coordinates and the cap station carry
the source location, without asserting an authored construction history.
Arbitrary rotations/scales, inconsistent axes and unrepresentable source
conversion fail closed. The old unposed reader and its JSON tag are unchanged;
they reject the oblique posed source. No posed native envelope is claimed yet.
`WritePosedOCCTExactCircularLoopSource` re-proves the original graph before
re-export; canonical emission uses the existing exact writer with the recovered
proper signed-coordinate frame.

The independent OCCT author supports `write-model-only-posed output.step` and
retains `testdata/occt-model-only-posed-rounded-triangle.step`. It constructs a
negative-height prism and applies (x,y,z)→(z+1,x−2,y+3) before exporting through
the official model-only setting. Fresh source, preserved source and generated
canonical STEP all transfer through OCCT with matching topology, analytic
volume/area and world centroid within 1e-9. Tests also assert untouched original
records, native reload, old-mode rejection, mismatched wall/circle axes,
unsupported oblique ratios, metadata-frame guards, limits and cancellation.

### Posed source-bound native envelope

`WritePosedOCCTExactCircularLoopExtrusionJSON` and
`ReadPosedOCCTExactCircularLoopExtrusionJSON` use the separate version-one
`plainsolid-step-posed-occt-exact-circular-loop-extrusion` format. Original
source, product and topology roles, selected reference circle, exact axis
rotation, local native bytes and world recipe are claims compared against one
complete posed-source reproof. The local height remains normalized positive;
no historical extrusion sign is recovered. The envelope is bounded to 8 MiB,
source to 1 MiB, with caller source limits and cancellation. Prior native tags
remain unchanged. Independent live qualification now reloads this envelope
before original-source and generated canonical STEP transfer.

### Shared rounded product assembly

`ReadExactCircularLoopAssembly`, `WriteExactCircularLoopAssembly`, and
`WriteExactCircularLoopAssemblySource` form a separate AP214 subset: one
geometry-free root, one original model-only line/circle prism product, and
2–16 flat shared occurrences with proper signed-coordinate rigid transforms.
The local product retains its source cap/wall/rim/vertical-edge/vertex IDs.
Apply `Body.Descriptor.Frame` to local native coordinates, then the selected
occurrence transform; no geometry is flattened and no assembly union is claimed.
The body source is the original assembly file, not a standalone product envelope.

The writer accepts an authenticated standalone unposed OCCT product source and
explicit product names/occurrences. It stages unchanged local geometry through
the standard assembly writer and re-proves all output before writing. The reader
shares the existing weighted station assembly's strict flat relationship/closure
proof, then independently authenticates the rounded body's complete geometry.
Uniform metre or millimetre SI contexts, radians/steradians, and neutral child
frames are required. Nested products, mixed units, pcurves, unsupported local
geometry, detached entities, and nonneutral child transforms fail closed.
Existing standalone and weighted station public APIs keep their prior domains.

The pinned OCCT author now provides `write-assembly` and `read-assembly` modes.
The retained `occt-model-only-rounded-assembly.step` is independently created
from analytic OCCT construction and XCAF shared instances with
`WriteSurfaceCurMode=false`, not generated from PlainSolid STEP. The two uses
are identity and `(x,y,z) -> (z+10,x-2,y+3)`. Live XCAF import verifies both uses
refer to the same original product. Original, preserved and Go-authored files
have 2 solids, 16 faces, 36 edges and 24 vertices; aggregate volume is
`2*(27+25*pi/32)` and area `2*(51+105*pi/32)`, with matching world centroid.
Native local reload, ownership/geometry/limit/cancellation negatives and a
40-execution bounded decoder fuzz campaign qualify this selected graph.

### Shared assembly native persistence

`WriteExactCircularLoopAssemblyJSON` and `ReadExactCircularLoopAssemblyJSON`
use the distinct version-one `plainsolid-step-exact-circular-loop-assembly`
format. One original STEP source and one local native solid are stored; every
product, cap/wall/rim/vertex role, NAUO/CDSR/relationship/frame ID, and exact
occurrence matrix is independently re-proved on reload. Stored geometry and
placements are claims, never authoritative replacements. The nested body claim
is deliberately not a standalone native format. All prior tags remain unchanged.

The reader enforces an 8 MiB envelope, 1 MiB source, caller STEP limits,
cancellation, and atomic failure. Tests reject altered source, local recipe,
native geometry, role and occurrence identities, transforms and tags. Live XCAF
qualification reloads native before source re-export and before regenerating
analytic local STEP geometry plus standard shared occurrence records; both
outputs retain shared product identity, topology and world measures.

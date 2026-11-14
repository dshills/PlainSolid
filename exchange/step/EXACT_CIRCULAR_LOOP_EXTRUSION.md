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

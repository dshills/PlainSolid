# Exact same-side overlapping disk cuts

`WriteSideOpenDualDiskProduct` is an additive, rooted AP203 writer for
`trimmed.CutSideOpenDualDisks`. It builds the certified native shape from the
supplied descriptor and writes that actual sewn topology. It is not a generic
trimmed-solid exporter or an importer for rounded vendor pcurves.

The supported stock is a rectangular prism in a translated signed-coordinate
frame. Two equal-radius, strictly intersecting disks have their centers on the
same `Min.Y` side. Their crossing point, opening endpoints, frame placement and
native chart coefficients must pass the exact native constructor. Cuts begin at
local z=0 and end at `Depth`; `Depth == Height` is through. Cutters must clear the
other stock sides. The input cutter order remains the native wall ownership
order. Unequal disks, tangency, nonrepresentable construction, non-cardinal
placement and other cut arrangements fail rather than being approximated.

The writer retains every native vertex, shared edge, coedge, planar subdivision
and face orientation. Each rational native wall is replaced with its original
exact analytic cylinder; its clockwise native chart orientation is accounted
for explicitly. Circular rims use direct `CIRCLE` model curves with exact
endpoint equations and a proved minor-arc direction. Straight seams and stock
edges use direct cardinal `LINE` model curves. There are no `PCURVE` or
`SURFACE_CURVE` records, so no rounded radian value is treated as an exact
endpoint. Native exact sewing remains the authority for continuous wall/cap
agreement and material topology.

Names are preserved on the product and solid. Length units must be a supported
SI power of ten. Model coordinates are written as exact finite decimal values
of the represented binary inputs, bounded by the strict standalone numeric
reader (including its 64-character scalar limit). There is no unit conversion.
The writer accepts at most 128 vertices, 128 edges and 32 faces, and validates
its staged Part 21 output under a 1 MiB / 2048 entity limit. Invalid geometry,
limits and pre-write cancellation leave the caller's writer untouched; ordinary
I/O errors or short writes are propagated and can leave an external sink partial.

## Independent qualification

`TestOCCTWriteSideOpenDualDiskProduct` uses the optional standalone
`tools/occtref/overlapping_side_pockets.cxx` helper in `read` mode. It requires
normal `STEPControl_Reader.TransferRoots()` to transfer one root and
`BRepCheck_Analyzer` to accept the result. The helper does not call PlainSolid.
The tests cover blind and through cuts in three signed-coordinate poses,
including swapped cutter order, and compare volume, area and centroid against
independent analytic formulas. For the retained radius 2.5 fixture:

- Blind: 16 faces (14 planes, two cylinders), volume 309.48192308272155,
  area 321.2144615338227.
- Through: nine faces (seven planes, two cylinders), volume 258.9638461654432,
  area 296.7502051227931.

The blind native shell retains conforming planar subdivisions, so its face
count differs from OCCT's independently unified nine-face blind-cut reference.
No shape simplification or merging is claimed.

Set `PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS` to the built helper for the
optional test. The helper also supports `side`, `interior` and `side-through`
independent source generation. See `OVERLAPPING_SIDE_POCKETS_EVIDENCE.md` for
source pcurve discrepancies and why the vendor fixture is not an exact import.

## Strict canonical re-import

`ReadSideOpenDualDiskProduct(ctx, reader, limits, tolerance)` reconstructs the
native body from recoverable exact source geometry and accepts it only after a
complete regenerated entity-graph comparison. That comparison binds both AP203
roots together, including product/body names, source units, shared context,
vertex/edge identity, circle/line geometry, plane/cylinder support, face bounds,
coedge orientation and face senses. No detached entities, unknown graph records,
owner pcurves or alternate partitions are ignored. Documentary header timestamps,
file names and descriptions do not determine geometry; the AP203 schema is
checked explicitly.

Entity numbering, record encounter order and equivalent exact decimal spellings
may differ. Reversed senses already emitted by the writer are verified; changing
an individual sense or choosing another geometrically equivalent sense encoding
is not supported. The imported descriptor normalizes the local origin to the
stock lower corner, has `Min == (0,0)`, and orders the two cutters by local X.
An original choice of frame origin versus rectangle offset, or original cutter
input ordering, is not uniquely stored by this geometric dialect. Returned source
face/edge IDs are source provenance, distinct from native topology identifiers.
The supplied tolerance is a native construction policy, not an inferred source
uncertainty. Output is all-or-none on failure.

The reader has the writer's 1 MiB and 2048-entity hard bounds in addition to caller
parser limits. It refuses source scalar values or recovered local differences
that cannot be represented exactly by the native constructor. It does not import
the retained independently authored OCCT side-pocket file, whose rounded pcurves
remain outside this exact model-only dialect. Optional live tests compare native
re-imported body measures with OCCT's independent read of the regenerated STEP.

### Opt-in reversed direct lines

`ReadSideOpenDualDiskProductWithOptions` with
`SideOpenDualDiskReadOptions{AllowReversedModelLines:true}` also accepts direct
model `LINE` direction reversal paired with `EDGE_CURVE.same_sense = false`.
The proof requires exact represented endpoint coordinates differing along one
cardinal axis, a line origin equal to the edge-start point, a direction exactly
opposite that endpoint axis, and vector magnitude exactly one. The line, vector
and direction each require exclusive source ownership. Thus reversal traverses
precisely the same finite segment; there is no snapping or approximate test.
The private parsed graph is then normalized and subjected to the same complete
canonical reconstruction comparison. `ReversedModelLines` reports the source
edge IDs whose paired encodings were proved and normalized.

Zero options and the existing reader remain canonical. Flipping only direction
or sense, changing magnitude/origin, sharing the direction entity, or using a
non-cardinal direction is rejected. Circle frames/senses, coedges and faces
remain canonical; this option does not widen pcurve or vendor import support.
The optional OCCT test independently validates the alternate directed topology
and measures for both blind and through bodies.

### Opt-in circle reference changes

`AllowRephasedModelCircles` separately permits a signed cardinal circle reference
axis rotated by 90°, 180° or 270° about the unchanged normal. This changes only
the angular origin of the same oriented supporting circle. The reader proves
both exact source endpoints lie on that circle and in its plane, excludes
coincident/antipodal endpoints using a nonzero exact radial cross product, and
requires `same_sense` to select the corresponding directed arc shorter than π.
The source line option is not implicitly enabled.

The circle, placement and reference direction must be singly owned. The private
reference direction is then normalized before complete graph reconstruction
comparison, which additionally binds the exact circle center, normal, radius,
cylinder support and both coedges. `RephasedModelCircles` returns source edge IDs
whose reference changes were proved. Changes to the normal or edge sense are
not normalized: complementary arcs and opposite-normal encodings remain outside
this option, even if other declarations could make them geometrically equivalent.
Non-cardinal reference directions and rounded pcurves remain unsupported.

Independent OCCT read/validity/measure tests cover all three reference changes
for blind and through shapes. Native tests also exercise signed coordinate axis
permutations and composition with the separate reversed-line option.

### Opt-in opposite circle normal with paired sense

`AllowOppositeModelCircleNormals` permits the exact negative of the canonical
normal only when the source `EDGE_CURVE.same_sense` is also inverted. The normal
reversal changes the circle's orientation; the paired sense reversal restores
the same model-space traversal. The proof converts the source sense into the
canonical normal orientation and requires its sign to agree with the exact
endpoint radial cross product. It therefore selects the same strict minor arc,
not its complementary arc. The center, radius, endpoint plane equations,
reference frame and complete reconstructed graph remain independently checked.
The normal direction, like the placement and reference, must be singly owned.

`OppositeNormalModelCircles` records the normalized source edge IDs. A changed
reference axis still requires `AllowRephasedModelCircles`; neither circle option
enables the line option. Zero options preserve the original reader contract.
Tests reject unpaired normal/sense changes, shared normals and altered supports.
Independent OCCT validation and analytic measures cover the opposite-normal
encoding at all four cardinal phases for blind and through cuts.

### Opt-in paired planar normal and face sense

`AllowOppositePlaneNormals` permits a retained planar face to reverse its exact
plane normal and `ADVANCED_FACE.same_sense` together. The plane origin and X
reference must equal the independently reconstructed canonical support. Its
right-handed Y direction is then exactly negated, giving the complete support
identity `P_source(u,v) = P_canonical(u,-v)`. Inverting the face sense restores
the same material normal. This is an identity of evaluated affine surfaces, not
a sample-based test.

Model-space boundary uses and their ownership remain unchanged. The original
face-bound orientation and all coedge adjacency are verified by the complete
canonical graph comparison after normalization. No pcurves are present in this
dialect; UV trims are never silently reflected. Plane, placement, normal and
face ownership are checked before normalization. `OppositeNormalPlanes` contains
the proved source `ADVANCED_FACE` IDs.

Default reading remains unchanged. Origin/reference changes, unpaired flags,
changed bound orientation and shared normals reject. Independent OCCT checks
accept blind and through bodies with all planar faces reversed in this paired
form, both alone and composed with the independent line and circle options,
with unchanged analytic area and volume.

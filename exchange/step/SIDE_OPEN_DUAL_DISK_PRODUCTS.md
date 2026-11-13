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

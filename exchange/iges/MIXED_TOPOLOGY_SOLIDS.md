# Independently owned mixed IGES material roots

`ReadMixedTopologySolids` and `WriteMixedTopologySolids` support a collection of
2–8 type186 solids containing at least two of the exact tetrahedral, convex
affine-hexahedral, canonical circular-cylinder, and original parabolic spline-roof
families. Repeated roots
of any supported family are allowed within the same cumulative caps. Every root retains its native material
proof; there is no faceting of the roof or generic approximate surface fitting.
This is a multi-solid file, not IGES assembly placement, instance sharing, or a
Boolean union. Bodies may overlap; inter-body noninterference is not claimed.

The reader traverses only declared topology references in
186→514→510→508→504/502 plus the original type126/128 geometry or the canonical cylinder
100/110 curves and 190/192/116/123 supports/datums. Each source entity
must belong to exactly one root. Cross-root topology/curve/support sharing,
orphans, missing targets, unsupported entities and invalid topology fields fail.
A component is reindexed in source directory order solely to invoke its existing
strict material decoder. `LocalToSource` reverses every proof-local ID to the
actual authored file ID; `SourceSolidID` is always the actual source root.
Nested tetrahedron/roof objects explicitly contain proof-local IDs. Original
geometry, labels and subscripts are preserved with that map. Raw directory
style/status/transform conditions are checked before reindexing, so proof staging
cannot erase unsupported metadata. All global units are shared and preserved.

Component structure chooses one decoder, never a best-effort fallback. A 24-entity component must pass the complete canonical circular-cylinder
scalar, support, full-chart and material proof. A
34-entity component must pass the complete tetrahedron topology, continuous
curve/UV, convex halfspace and closed-material proof. A 58-entity component whose six original support bases are all bilinear must
pass the complete exact affine-hexahedron geometry and convex outward halfspace
proof. Other 58-entity components must pass the role-independent original roof
topology, exact continuous composition, source-weight and complete global spline
embedding proof. There is no trial-decoder fallback from a failed hexahedron. Neither entity count
alone is evidence of a valid shape. Arbitrary shells, voids, leaf/root transforms,
other spline families and disconnected subgraphs within a body remain unsupported.
Existing single-body APIs remain strict and reject these mixed files.

The aggregate caps are 512 KiB, 8192 records, 464 entities, 32768 values and
832 control points; caller limits may tighten them. Proof-local input uses each
existing family's additional caps. A writer uses explicit family descriptors,
rejects irrelevant payload fields, validates every local material, remaps only
typed reference positions, and validates the final ownership graph before I/O.
Read failures are atomic. Cancellation is checked before and between bounded
component proofs.

## Independent qualification

`tools/occtref/author_iges_mixed_topology.py` combines independently authored
reference tetrahedron and mapped-roof records using IGES pointer semantics;
it consumes no Go writer output. The retained 92-entity fixture is
`testdata/mixed-topology-solids.igs`. Ordinary TransferRoots/BRepCheck with pinned
OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts source and Go output as
2 solids with 10 faces. Independent expectations are volume 8/3,
centroid (.65625,-.5,2.1), and area
`6+2*sqrt(3)+20/3+sqrt(5)/2+asinh(2)/4`.
The external numerical area comparison uses 1e-8, not an acceptance tolerance.
Optional live tests use `PLAINSOLID_OCCT_IGES_SOLID`.

Tests cover both body orders and units, original ID maps, cross-root references,
invalid source geometry/orientation, ignored-looking metadata, cumulative budgets,
writer staging, cancellation and bounded decoder fuzzing. No external dependency
is required at runtime.

## Conical STEP boundary identified during scope review

This milestone deliberately does not add an advertised exact conical assembly
family. The existing STEP cone codec writes a semi-angle and reconstructs slope
with `tan`; for example `tan(atan(.5))` is `.49999999999999994` in the tested Go
implementation. A binary equality check for selected values would still not prove
mathematical equality between a decimal semi-angle and a rational slope. Such a
representation needs an explicit supported numeric/error contract rather than
silently treating altered cap-radius relationships as exact. Existing cone APIs
and their documented behavior are unchanged.


## Three families and repeated independent roots

`MixedTopologyHexahedron` adds the existing exact convex six-quad family via
`HexVertices`. The returned `Hexahedron` payload uses the same explicit local-to-
source ID convention. The writer rejects irrelevant fields for every tag.
An all-bilinear support basis only selects the hexahedral validator: non-affine
nets, invalid whole-edge ownership or nonconvex/inward material still fail.

`author_iges_three_family_topology.py` independently authors two tetrahedra,
the second translated by (10,0,0), an oblique reflected affine hexahedron with
origin (3,4,5) and vectors (-2,-1,0),(0,3,1),(1,0,2), and the mapped curved roof.
The retained `three-family-topology-solids.igs` contains 184 entities and 304
controls. Pinned OCCT validates input and output as 4 solids and 20 faces, with
volume 17 and centroid (193/68,193/51,2723/510). Area is
`2*(6+2*sqrt(3))+20/3+sqrt(5)/2+asinh(2)/4+2*(sqrt(41)+sqrt(46)+sqrt(21))`.
Repeated material definitions are deliberately separate roots with disjoint
source IDs, not shared occurrences. Root order and every provenance map are
deterministic. Tests tighten the aggregate cap one below the exact control and
entity counts, preserve old two-family input, and reject a bilinear support
whose altered corner breaks the full affine/material proof.


## Canonical circular material in the collection

`MixedTopologyCylinder` and its `Cylinder` payload add the separately verified
24-entity analytic family without widening that family. Directory order is
normalized only by removing other roots' records; the local graph must still
match the canonical circular-cylinder codec exactly. Typed reference rebasing
now includes 190/192 support datums and 508 loops with zero or one pcurve per use.
It never interprets coordinate values or curve parameters as references.
The datum graph has no transform; full circular type100 rims and exact
0–360-degree side coverage remain mandatory. Vendor-rounded variants still fail.

`author_iges_mixed_circular_material.py` combines independent tetrahedral and
original mapped-roof source with a canonical circular cylinder translated by
(8,0,0). The retained 116-entity fixture has 156 spline controls; the analytic
cylinder has no B-spline control net. Both source and Go output pass pinned OCCT
TransferRoots/BRepCheck as 3 solids and 13 faces. With `V=8/3+12*pi`, the expected
centroid is `((7/4+96*pi)/V,(-4/3)/V,(28/5+18*pi)/V)`; expected area is the
original two-body area plus 20*pi. Bounds remain cumulative across every root.

Repeated cylinder roots have separate decoded parameter and source-ID maps;
tests mutate one result and verify the other is unchanged. Additional negative
fixtures redirect a cylinder rim into another body's geometry, invalidate a
cap's declared pcurve count, change radius or chart period, and tighten aggregate
limits. Independent root collections still do not imply shared occurrences,
Boolean union or inter-body noninterference.


## Explicit placed circular members

`MixedTopologyPlacedCylinder` uses `MixedTopologyInput.Cylinder` plus a proper
signed-coordinate `Placement`. Its decoded `PlacedCylinder` retains the local
canonical source graph, world vertices, exact placement and world native solid.
The existing unplaced cylinder tag rejects a placement payload. Placed and
unplaced cylinders count as one material family for the collection's requirement
of at least two different families.

Each of the six geometric leaf directory transform pointers participates in
root reachability and exclusive ownership. The terminal type124 is included in
`LocalToSource` (local ID49); its actual original source address is therefore
`body.LocalToSource[body.PlacedCylinder.SourceTransformID]`. Both parameter
references and directory transform references are rebased, never numeric scalar
fields. A shared cross-root transform, orphan matrix, missing leaf placement,
transformed pcurve or topology record fails atomically. Original per-component
25-record proof remains mandatory. Aggregate limits are unchanged.

The independent Python-authored `mixed-placed-circular-material.igs` combines the
retained tetrahedron and spline roof with a cylinder under `(x,y,z) ->
(z+7,x-4,y+2)`. Pinned OCCT ordinary TransferRoots accepts both this source and Go
output: three solids,13 faces, volume `8/3+12*pi`; the cylinder center is
`(8.5,-4,2)`. Tests compare total area and all centroid coordinates analytically.
This is an independent material collection, not a shared-instance assembly or a
Boolean union, and does not prove that distinct members do not intersect.


## Attached blind and through openings

`MixedTopologyBlindCylinder` and `MixedTopologyThroughCylinder` have explicit
`BlindCylinder` and `ThroughCylinder` descriptor/result payloads. They are distinct
material families, with39- and36-record structural discriminators respectively.
Record count only selects the candidate validator: the complete independently
recovered blind/through support, loop, sense and native material proof must pass.
Neither falls back to a plain cylinder, and unused tagged payloads are rejected.
Original local-to-source maps preserve all opening/floor/cap/inner-wall roles.
The source graph has no new reference grammar: existing typed186/514/510/508/504
edges prove exclusive ownership, including every hole loop and full-circle rim.

The retained independently authored `mixed-bored-material.igs` contains one
non-cylinder tetrahedron, a radius3/radius1/height4 blind cylinder at X=-10
(floor2), and a radius3/radius1/height4 through cylinder at X=10. Pinned OCCT
TransferRoots and BRepCheck accept source and Go output: three solids,13faces,
volume `4/3+66*pi`, area `6+2*sqrt(3)+94*pi`, and exact analytic first moments
`(2/3-20*pi,2/3,2/3+130*pi)`. This independently exercises both annular cap
forms together. Repeated members have separate mutable parameter maps and source
provenance. Cross-root hole-loop/model-edge links, invalid inner-wall sense,
radius mismatch, aggregate limits and inappropriate payloads fail atomically.

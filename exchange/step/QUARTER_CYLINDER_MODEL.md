# Exact quarter-cell cylinder BRep

`ReadQuarterCylinderModel` and `WriteQuarterCylinderModel` support a distinct
six-face model-only domain: four ordered quarter-cylinder faces, two disk caps,
eight cardinal rim vertices and twelve shared edges. The descriptor, exact
signed-cardinal frames, source decimal checks, units and bounds are those of the
half-split model. The returned native cylinder is canonical; six source face
and twelve edge IDs remain separate ancestry.

The private angular role proof is shared with the two-cell implementation,
but each public reader fixes its own cell count. Half-split and rooted product
routes remain unchanged and reject this six-face graph. Bare readers still reject product wrappers; the explicit rooted route is
documented below. Generic STEP dispatch is unchanged.

Local source rim points must be exactly (+r,0), (0,+r), (-r,0), (0,-r).
Direct increasing CIRCLE edges must advance exactly one role, so a three-quarter
arc, reversed sense or diagonal skip is rejected. Each lateral face owns its
bottom quarter, next axial line, reversed top quarter and reversed initial
line. The two caps own all four ring arcs with outward orientations. Unique
roles and continuous endpoint incidence prove complete coverage and opposite
coedge ownership. All supports and lines are checked exactly; no sampled
geometry authorizes import.

The independently hand-authored retained fixture passes OCCT direct body
transfer/BRepCheck, preserving six faces, volume12π and area20π. Independently
loaded native-writer output in a translated -X axis/-Z reference frame has the
same measures and centroid(8.5,-4,7). Use the existing optional helper with
`quarter` mode; `PLAINSOLID_OCCT_SPLIT_CYLINDER` enables both live tests.


## Explicit rooted and native routes

`ReadQuarterCylinderProduct` and `WriteQuarterCylinderProduct` use the same
strict nine-record AP203 envelope validator as the half-split product codec,
with fixed product identifier `quarter-cylinder`. The actual six-face proof is
still required: changing an identifier cannot convert a half graph to quarters
or vice versa. Product name remains bound to the solid name; all context,
formation, protocol and definition fields/references are checked.

`WriteNativeQuarterCylinderProduct` first uses the existing full immutable
canonical-cylinder classifier. It rejects foreign surface evaluators and
changed native contracts rather than inferring shape from samples. Metadata,
pose, native material and source-role ancestry survive the explicit roundtrip.

OCCT's conventional TransferRoots requires exactly one root and verifies six
faces from native output with local reference +Y, axis -Z and origin(-3,5,2).
Measured volume12π, area20π and centroid(-3,5,0.5) independently qualify the pose
and product envelope. The bare/direct-transfer tests remain separate.

# Exact quarter-cell cylinder BRep

`ReadQuarterCylinderModel` and `WriteQuarterCylinderModel` support a distinct
six-face model-only domain: four ordered quarter-cylinder faces, two disk caps,
eight cardinal rim vertices and twelve shared edges. The descriptor, exact
signed-cardinal frames, source decimal checks, units and bounds are those of the
half-split model. The returned native cylinder is canonical; six source face
and twelve edge IDs remain separate ancestry.

The private angular role proof is shared with the two-cell implementation,
but each public reader fixes its own cell count. Half-split and rooted product
routes remain unchanged and reject this six-face graph. No quarter product
wrapper or generic STEP dispatch is introduced.

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

# Exact model-only split cylinder BRep

`ReadSplitCylinderModel` accepts a separate finite BRep domain with four faces:
two complementary semicylinders and two disk caps. The returned native cylinder
has equivalent material geometry, while `Faces` and `Edges` retain all four
source face and six source edge identities in documented geometric role order.
The native constructor does not preserve the artificial lateral split itself.

Version-independent source restrictions are deliberate: one bare
ADVANCED_BREP_SHAPE_REPRESENTATION, one MANIFOLD_SOLID_BREP/CLOSED_SHELL,
origin-centered +Z cylinder, ±X antipodal seam vertices, positive height/radius,
explicit +Z/+X support frames, direct CIRCLE/LINE EDGE_CURVE geometry with true
curve sense, and no pcurves, placements of products, tolerance repair or extra
unowned records. Coordinates must convert from exact source decimals without
rounding. Only exact SI length contexts are admitted. The entire graph is
bounded to 256 entities and 32 reference levels in addition to caller limits.

Each circle's distinct antipodal endpoints and increasing curve sense define a
semicircle. Opposite starting endpoints select complementary angular halves.
The two side loops must have the exact oriented bottom-arc / generatrix /
reversed-top-arc / reversed-generatrix pattern, modulo cyclic rotation.
Disk loops use both arcs once with the outward cap sense. Unique vertex roles,
six unique edges and four unique face roles prove complete opposite coedge
ownership, closure and angular coverage. Exact line geometry and exact cap
planes prove continuous joins. This constructive cylinder domain is embedded
and outward; no sampled acceptance criterion is used.

The hand-authored `testdata/split-cylinder-model.step` uses no decimal π and is
independent of the parser. OCCT transfers its explicit MANIFOLD_SOLID_BREP entity,
validates the resulting BRep, and reports four faces, volume 12π and area 20π.
The helper intentionally transfers the body directly: this bare geometric
representation has no product/application graph and TransferRoots does not
find a product. Set PLAINSOLID_OCCT_SPLIT_CYLINDER to the executable built by
`tools/occtref/build_read_split_cylinder.sh` for this optional qualification.

The rounded independently generated ShapeDivideClosed fixture remains rejected
(see SPLIT_CYLINDER_EVIDENCE.md). Existing generic and analytic readers are not
broadened. Neither arbitrary split angles nor an approximate external source is
silently normalized into this exact domain.

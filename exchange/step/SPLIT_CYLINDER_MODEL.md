# Exact model-only split cylinder BRep

`ReadSplitCylinderModel` accepts a separate finite BRep domain with four faces:
two complementary semicylinders and two disk caps. The returned native cylinder
has equivalent material geometry, while `Faces` and `Edges` retain all four
source face and six source edge identities in documented geometric role order.
The native constructor does not preserve the artificial lateral split itself.

Version-independent source restrictions are deliberate: one bare
ADVANCED_BREP_SHAPE_REPRESENTATION, one MANIFOLD_SOLID_BREP/CLOSED_SHELL,
exactly translated signed-coordinate cylinder, local ±X antipodal seam vertices,
positive height/radius, explicit common signed-cardinal support frames, direct CIRCLE/LINE EDGE_CURVE geometry with positive
physical circle direction and forward physical axial direction, and no pcurves, placements of products, tolerance repair or extra
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


The source cylinder frame is returned in `Frame`. Normalization into its local
coordinates uses exact rational subtraction and signed permutation, never
floating-point subtraction or tolerance. Every derived coordinate must also be
exactly representable within the existing bounded scalar codec. All original
support frames must agree after this normalization. Native material is built
in the original frame; IDs and length-unit metadata remain unchanged.

The independently posed raw fixture uses origin (10,-4,7), local X=(0,0,-1),
axis=(-1,0,0). OCCT independently accepts the same four-face topology and mass.
Tests also verify changing only the declared unit leaves numeric geometry and
pose unchanged, and reject an almost-cardinal rounded source direction.

## Exact writer

`WriteSplitCylinderModel(w, SplitCylinderDescriptor{...})` constructs the six
model curves, four directed face loops and shared topology from an explicit
name, frame, radius, height and length unit. It does not accept stale `Solid` or
source IDs as evidence. Re-reading exposes deterministic source-role IDs.

World-coordinate sums and scalar decimals are generated with rational
arithmetic from the represented inputs. Every result must be losslessly
representable under the reader's bounded numeric contract; rejected examples
include a tiny translated origin whose radius-offset point would round. The
writer stages the whole file and invokes the strict reader before writing.
Invalid input leaves output untouched; short writes and other I/O errors are
reported. Canonical repeated writes are byte-stable.

This milestone preserves the bare representation envelope; its independent
OCCT qualification explicitly transfers the body entity rather than claiming
TransferRoots/product interoperability. Product ownership is not yet part of
this exact reader/writer domain. SI length units accepted by the reader are
preserved without numeric scaling; conversion-based units remain unsupported.

Paired reversed circle normals and false curve senses are additionally accepted
as described in CYLINDER_CELL_CURVE_SENSE.md; canonical writers remain unchanged.

Equivalent paired axial and bound orientation encodings are documented in
CYLINDER_CELL_BOUND_ORIENTATION.md; default writer declarations are unchanged.

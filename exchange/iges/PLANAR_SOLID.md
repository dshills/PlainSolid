# Closed planar material in IGES

`ReadPlanarSolid` imports one type 186 solid and its type 514 exterior shell.
Up to 64 separately declared void shells may be directly inside the exterior.
A type 402 form 1 wrapper may identify the single material root.
The supports are type 190 form 1 planes. Edges use type 110 model segments;
each type 508 loop refers to the shared model edge list and records the
direction of each use. A type 510 face supplies an explicit outer loop
followed by its inner loops. Faces can be concave, and through-holes can
traverse the material. Original vertex, edge, loop, plane and face
identities are available in the import result, along with the face range
for each exterior or void shell.

The importer checks original decimal vertex and line endpoint agreement.
Topological vertex coordinates must be exactly representable as binary64.
Plane origins and direction ratios are read as exact rationals; the
plane origin itself need not be exactly representable. Original face
orientation, coedge continuity, embedding and material orientation are
checked before native publication. Coordinates stay in file units. The
reader bounds source size and exact arithmetic; the material validator
has separate collision and work budgets. The exact nesting tree must
agree with the original declared shell roles and face membership.

`WritePlanarSolid` emits the same model-only dialect and sheared faces
are supported without rounded direction ratios. The writer checks exact
native topology and units on readback before publishing bytes. It does
not attempt to reconstruct arbitrary original directory labels or entity
numbers. It writes the exterior before its voids, even when native faces
are interleaved. Transformed instances, parametric edge curves, multiple
solid roots and material islands inside cavities are outside this entry point.

The pinned OCCT author and consumer are in
[`tools/occtref`](../../tools/occtref/README.md). The fixture is a concave
prism with a through-hole; only the writer author's name was normalized
in the checked-in specimen. The original and the PlainSolid export
independently transfer as one valid solid with 12 faces, volume 50,
area 102 and centroid `(3.22, 2.22, 1)`.
The same oracle authors a box with one cavity and a box with two disjoint
cavities. For those fixtures original and canonical output transfer with
volumes 448 and 496, areas 480 and 432, and the original shell counts.

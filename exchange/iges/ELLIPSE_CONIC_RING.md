# Exact ordered IGES ellipse ring

`ReadEllipseConicRing` and `WriteEllipseConicRing` exchange one type102 model
composite containing exactly four distinct type104 form1 quarter ellipses.
The first quarter may be any cardinal quarter; subsequent members must advance
cyclically in positive local order. Original coefficient scaling, local Z,
member labels/subscripts, composite metadata, length units and original entity
IDs are retained on read. Source IDs are provenance, not writer-assigned IDs.

All leaves have identical exact coefficients and share either no placement or
one proper signed-coordinate type124. The transform is owned by this composite
and referenced by every leaf; the root cannot carry another transform. The
reader rejects nested composites, duplicate members, additional/unowned records,
nonadjacent/reversed endpoint selections and differing supports. Source decimals
must satisfy the existing lossless atom gate. A proper rotated frame can reverse
the world-plane normal while retaining positive local parameterization.

The quarter proofs establish exact endpoint continuity and one complete ordered
ellipse, with each member retaining its own exact half-angle curve parameter.
This is a model wire, not a face or material boundary; no shared parameter speed
or stronger composite continuity flag is invented. Existing full/quarter atom
and generic composite APIs are unchanged. Inputs are additionally capped at
32KiB, 256 records, six entities and 1024 parameter values. Writers validate a
staged file before writing; ordinary I/O failures may still partially write.

The independent `tools/occtref/iges_ellipse_ring.cxx` creates the retained fixture
using OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 directly: four 5-by-2 ellipse
quadrants and a common placed frame `(x,y,z) -> (z+1,x+2,y+3)`. Ordinary
IGESControl TransferRoots accepts the original and generated output as valid
four-edge geometry (length 23.01311259606501 in model millimetres). The optional
`PLAINSOLID_OCCT_IGES_ELLIPSE_RING` test uses that standalone executable.

Focused verification: ordinary and race tests, Go1.23 focused tests, package vet,
and `FuzzEllipseConicRing` with `-fuzztime=100x -parallel=1`.

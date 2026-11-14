# Canonical disk-hole product assembly

`WriteDiskHoleAssembly` / `ReadDiskHoleAssembly` preserve explicit shared local
products from the already-supported `DiskHoleExtrusionBody` family. This adds an
assembly workflow, not another standalone bore representation. Circular stock
may contain strictly contained, disjoint circular through holes and blind
pockets. Their native constructor, signed extrusion/entry-cap semantics, analytic
supports, periodic seam conventions and canonical pcurves are unchanged.

Products retain names, body names, local frames, dimensions, source body IDs and
native trimmed materials. Occurrences retain names, parent/child identity and
proper signed-coordinate rigid placement; they are never baked into support
coefficients. Empty products are explicit assembly nodes. The graph can nest and
repeat supported products under the existing bounded assembly rules.

Each body is recovered from its actual source shell, reconstructed through the
existing native disk extrusion, and compared bijectively to the complete original
body dependency graph. Numeric tokens compare as exact rational decimals;
references, curve senses, per-owner pcurves, supports and sharing all participate.
Source material entities cannot be shared across different product owners or
remain unowned. A repeated occurrence shares one product and is deliberately not
a second source body. There is no new numerical acceptance tolerance and no
promotion of a vendor-rounded body into the canonical family. Existing canonical
standalone and cylinder/planar/spline assembly readers remain unchanged.

Source limits are cumulative for all material definitions: at most32 bodies,
with caller vertex/face/coedge/entity/value/byte/depth limits enforced before
body reconstruction. Occurrence counts and ancestry are bounded by the original
assembly parser. No partial result escapes on failure; output is staged and
reimported before writing. A source file with multiple products may contain
multiple bodies per product, but each body must pass this one material family.
This API does not union occurrences or prove inter-product separation.

## Independent pinned qualification

`tools/occtref/step_disk_hole_assembly.cxx` constructs OCCT Boolean cylinder cuts
independently: R3,H4 stock with an R1 depth2 opening at z0, and an R1 through
opening. XCAF places the blind product twice, once translated and once cyclically
rotated, and the through product once. The retained445-entity
`occt-disk-hole-assembly.step` independently validates material/assembly semantics.
Its alternate vendor body graph is explicitly rejected by the canonical reader;
that negative is distinct from accepting the material in OCCT.

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 STEPControl TransferRoots,
BRepCheck and independent XCAF import accept Go output and that independent
source: two referenced material definitions,three occurrences,three solids,
14faces,volume100*pi,area140*pi,centroid(0.9,6.8,1.34). Enable
`TestOCCTDiskHoleAssembly` with `PLAINSOLID_OCCT_DISK_ASSEMBLY` pointing to the
standalone helper. No shared oracle dispatcher changes are needed.

Tests verify roundtrip source IDs and names, repeated product identity,
noncardinal-transform rejection, cumulative limits, support/radius/coedge
mutations, cross-product support ownership, vendor graph rejection and bounded
fuzz. Original standalone APIs still reject assembly input.

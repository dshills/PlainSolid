# Rooted split-cylinder dialect

`WriteSplitCylinderProduct` wraps the exact `SplitCylinderDescriptor` geometry
in one declared AP203 configuration-controlled product. `ReadSplitCylinderProduct`
validates that whole rooted dialect and returns the unchanged model ancestry
plus product, definition and shape-definition IDs. The existing bare-model
reader still rejects product records.

The envelope contains exactly one each of APPLICATION_CONTEXT,
APPLICATION_PROTOCOL_DEFINITION, MECHANICAL_CONTEXT, DESIGN_CONTEXT, PRODUCT,
PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE, PRODUCT_DEFINITION,
PRODUCT_DEFINITION_SHAPE and SHAPE_DEFINITION_REPRESENTATION. Every reference,
fixed identifier, context string, description, source enumeration and protocol
year is checked. The product name must equal the solid name. Additional or
missing records, foreign names, repeated products, cycles and links to a wrong
representation are rejected. These fixed fields are a deliberate narrow dialect;
arbitrary product metadata is not discarded or guessed.

After proving this exact reference graph, only its nine records are removed
from a private parsed copy. All remaining geometry must pass the original
whole-graph split-cylinder reader, including reference closure and unique source
roles. Geometry cannot depend on a discarded product record. Parsing is bounded
to 1 MiB and 265 entities, together with the caller's limits. Failure returns an
empty product; writers stage and validate before I/O.

The dedicated optional OCCT helper's `roots` mode invokes conventional
STEPControl_Reader.TransferRoots, requires exactly one transferred root and
validates its BRep. The posed writer output retains four faces, volume12π,
area20π and centroid(8.5,-4,7). Tests also cover missing/foreign/cyclic/duplicated
product graph, dimension limits and the unchanged rejection by the bare reader.
This is one actual rooted BRep exchange dialect, not general product assemblies,
AP203 conformance certification or approximate STEP healing.

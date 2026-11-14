# Exact original planar NAUO relationship source

`ReadExactPlanarNAUO` is a separate strict source/material decoder for one root
product, one original cardinal planar local product, and exactly two direct
NAUO occurrences. It does not use MAPPED_ITEM or invent representation maps.
Existing `ReadAssembly` verifies the AP214/AP242 product/CDSR/relationship graph;
the new layer independently proves original model vertices, complete planar
owner pcurves, global local material embedding and exact from/to placement poses.

Each occurrence retains original NAUO, CDSR, representation relationship,
ITEM_DEFINED_TRANSFORMATION, parent/child and placement IDs. Relationship and
transformation records are exclusive to their occurrence in this bounded subset.
The exact rigid pose is to∘from⁻¹ using rational plane-frame normalization, not the
older floating assembly transform cache. Local model vertices remain binary64-
exact; original plane/model-line/pcurve support uses the strict cardinal proof.
Source uncertainties are retained metadata, never snapping/repair tolerances.
Separate source SI contexts may have different IDs but must declare exactly the
same length prefix. Conversion-based or mixed unit declarations are outside this
entry point. Source geometry must belong to the local body or explicit
representation placements; detached/unowned geometry and records reject.

`WriteExactPlanarNAUOSource` revalidates and preserves original entity identities,
record scalars, metadata and topology (not original whitespace). The separate
`ExactPlanarNAUODocument` version1 native envelope retains one shared local asset,
product names/units, and original relationship occurrence evidence. Reload proves
source and canonical native local geometry again. It provides exact world bounds
without recasting source occurrence IDs as map IDs. Existing mapped and generic
floating native formats remain separate.

## Real OCCT source

`author_exact_nauo_planar.cxx` uses XCAF and STEPCAFControl_Writer to independently
create a2×3×4 box product reused twice: translation(10,0,0), and cyclic placement
(x,y,z)→(z,x+20,y). The untouched382-entity source contains NAUO, CDSR, complex
representation relationships and distinct millimetre contexts. It is generated
by pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`, not by Go token synthesis.

The source and native reload→source STEP pass ordinary OCCT TransferRoots and
BRepCheck, with2solids,12faces,volume48,area104,centroid(6.5,11.25,1.75)
and world bounds(0,0,0) to(12,22,4). Enable
`TestOCCTExactNAUONative` with `PLAINSOLID_OCCT_MAPPED_BOUNDS`. Tests retain complete
product/relationship ancestry and reject foreign placement ownership, a tiny
nonorthogonal direction component, changed source units, wrong owner pcurve,
unowned bodies, stale native assets and cumulative limits. A bounded100-run fuzz
campaign exercises the actual independent source. Nested NAUO trees, multiple
local products, curved supports, mapped targets and general vendor repair remain
outside this scoped decoder; generic assembly APIs are not widened.

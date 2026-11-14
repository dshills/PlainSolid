# Reindexed exact elliptical prism material

`ReadReindexedEllipticalPrismMaterial` accepts the existing filled, concentric
annular, and properly signed-coordinate placed elliptical prism graphs with
independently reordered IGES directory and parameter records. The original
fixed-order readers remain unchanged. This is not a general type 186 importer.

A bounded traversal starts at the unique manifold solid. It follows declared
reference fields, including leaf transformation pointers, and constructs a
bijection between original entity addresses and the existing supported role
graph. Shared references must remain shared; distinct roles cannot alias. Every
record must belong to this graph. Shell face lists may be permuted: each role is uniquely classified by actual
support kind and shell sense (positive 122 outer wall, negative 122 inner wall
for hollow material, positive 190 upper cap, negative 190 lower cap). Missing
or duplicate roles fail. The existing geometric proof then authenticates all
supports, heights, trim curves and shared incidence. A lateral loop may start at any cyclic use, preserving its complete directed
cycle. Reversal, changed senses, duplicate uses and pcurve changes remain
unsupported. Cap outer/inner loop order stays strict because outer-first carries
format semantics. Vertex-list ordinals may be permuted: each source
vertex must equal exactly one typed wall endpoint (upper terminal or lower
directrix start), including an exact rational application of its authored leaf
transform. Equality is unique and unsnapped; all type 504 endpoint ordinals are
remapped before edge classification. Type 504 entries may be
permuted: typed curve kind and directed endpoint incidence must uniquely match
each rim/seam role, and every type 508 ordinal is remapped before exact geometry
reproof. Vertex coordinates, model curves, senses and complete incidence must
still match the supported material; incidence classification is not geometric
acceptance on its own. Only reference addresses
are normalized for the existing exact geometry and native material proofs;
source geometry scalars are retained unchanged. Unknown, recursive, detached,
incorrectly shared and malformed graphs fail atomically.

`SourceForCanonical` maps the nested proof object's canonical addresses to the
original source addresses. `SourceSolidID`, `SourceShellID`,
`SourceTransformID`, `NativeFaceSourceIDs` and `SourceRimCurveIDs` refer to the
original file. `SourceVertexOrdinals` maps canonical native endpoint roles to
original type 502 ordinals, derived anew from the retained source on reload. The nested material carries the established complete geometry
proof and its canonical role addresses. No rounded spline is promoted to an
analytic ellipse.

`WriteReindexedEllipticalPrismSource` revalidates both original graph and exposed
proof before writing the unchanged original file. The version 1 native JSON
recipe stores that source, role map, tolerance and existing typed material
recipe; reload redoes all proofs and rejects stale mappings or geometry. There
is no serialization of an unchecked native topology.

Hard caps are 32 KiB source, 512 records, 29 entities, 2048 parameter values and
512 graph traversal steps, additionally bounded by caller limits. Native JSON
is limited to 1 MiB. Cancellation is checked before and during graph traversal
and propagated into native construction.

## Qualification

`tools/occtref/reorder_iges_elliptical_prisms.py` independently reverses the
original four qualified source files and rewrites declared references and
parameter ownership without changing geometry. The retained fixtures exercise
filled, hollow, placed filled and placed hollow material. Tests verify original
address recovery, exact source re-export, canonical native reload, analytic
volume/centroid, wrong-root recursion, foreign pointers, inner/outer owner
aliasing, forged persisted role maps and resource limits. The optional pinned
OCCT comparator transfers all four original/re-exported files and checks solid
and face counts, independently integrated area, volume and centroid.

Run the focused live check with
`PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY` pointing at the existing standalone
OCCT elliptical prism comparator. `FuzzReindexedEllipticalPrism` has all four
positive fixtures as seeds; the qualification campaign uses `-fuzztime=100x`.

The additional `face-order-*` fixtures reverse shell face lists as well as
record addresses, authored by the independent script with `--faces`. Both
source variants transfer through pinned OCCT and preserve original bytes through
native reload. Mutating a face sense without its geometry fails. This does not
admit alternative support parameterizations or arbitrary shell topology.

The `edge-order-*` fixtures also reverse all type 504 entries and independently
rewrite every loop ordinal (`--edges`). Pinned OCCT transfers these files with
unchanged area, volume and centroid. Original source bytes preserve original
edge ordinals; nested proof geometry uses canonical ordinals. Wrong referenced
ordinals, endpoint roles or aliases cannot be repaired by the remapping.

The `vertex-order-*` fixtures additionally reverse type 502 entries and update
all type 504 endpoint references, independently authored with `--vertices`.
Pinned OCCT checks cover all four material variants. Source/native round trips
retain original bytes and exact ordinal provenance. Incorrect terminal points,
source coordinates, incidence indices and stale exposed ordinal maps fail. The
native envelope version is unchanged: source bytes already retain this complete
information, and the new ordinal map is derived rather than trusted on reload.

`SourceUseOrdinals` maps original type 508 IDs to canonical-to-original use
ordinals. It is derived from retained source on native reload and revalidated
before source output. The independent `--loops` fixture rotates lateral uses
and combines record, face, edge and vertex permutations. All four positives
transfer in pinned OCCT; focused tests exercise every cyclic start and reject
reversed cycles, changed senses and duplicate uses.

# Reindexed exact elliptical prism material

`ReadReindexedEllipticalPrismMaterial` accepts the existing filled, concentric
annular, and properly signed-coordinate placed elliptical prism graphs with
independently reordered IGES directory and parameter records. The original
fixed-order readers remain unchanged. This is not a general type 186 importer.

A bounded traversal starts at the unique manifold solid. It follows declared
reference fields, including leaf transformation pointers, and constructs a
bijection between original entity addresses and the existing supported role
graph. Shared references must remain shared; distinct roles cannot alias. Every
record must belong to this graph. Face lists, loop uses, edge-list indices and
orientation remain in the previously supported order. Only reference addresses
are normalized for the existing exact geometry and native material proofs;
source geometry scalars are retained unchanged. Unknown, recursive, detached,
incorrectly shared and malformed graphs fail atomically.

`SourceForCanonical` maps the nested proof object's canonical addresses to the
original source addresses. `SourceSolidID`, `SourceShellID`,
`SourceTransformID`, `NativeFaceSourceIDs` and `SourceRimCurveIDs` refer to the
original file. The nested material carries the established complete geometry
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

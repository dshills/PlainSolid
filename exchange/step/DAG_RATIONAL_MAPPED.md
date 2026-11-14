# Bounded mixed-group rational mapping DAG

`ReadDAGRationalMappedPlanarMaterials` accepts independently shared groups through
different parent branches, rather than requiring one identical group at each
level. It retains the existing exact local planar/pcurve/material proof and
rational rigid operators. Mapping origins must be identity and every group must
use the same original context. The explicit limits are eight nonroot group
definitions, four mapping steps per path, sixteen children per representation,
sixteen unique leaf-use records and128 expanded occurrences. Ordinary source
byte/entity/geometry/depth limits also apply. A source must contain a group;
this entry point does not replace the flat API.

Traversal rejects cycles on the active representation path and duplicate child
IDs within a group. Reuse through different branches is allowed. Every parsed
record must belong to the original reachable graph before making a private flat
proof view with unique leaf-use records. That proof checks exact model/owner
curves, material, contexts and exclusive local body ownership. Exact composed
poses are attached to independently owned source path arrays. A leaf item may
be shared across group definitions, so complete paths identify expanded uses.
No inter-instance nonoverlap or Boolean union is implied.

`WriteDAGRationalMappedPlanarSource` is validated source preservation. The separate
native DAG envelope stores original source and bound shared local native assets;
reload re-proves both and reconstructs all paths. `DAGOccurrencePaths` returns
deep copies aligned with occurrences. Exact bounds and explicit lossless baking
remain available. Fixed-depth native formats and the one-level edit route are
not silently widened. No fresh arbitrary native DAG authoring API is claimed.

## Independent source

`author_dag_rational_mapped.py` uses the retained independent OCCT local models
and authors three different root branches: the previous nested group, a new
group sharing two original leaf records, and direct reuse of the original inner
group. It creates no Go geometry. Two local assets produce11occurrences with
path lengths two and three. Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9` TransferRoots/BRepCheck observes11solids,
66faces,volume192,area452. Exact world bounds are(−10,−10,0) to(204,22,54).
Native reload→STEP passes the independent bound oracle with
`PLAINSOLID_OCCT_MAPPED_BOUNDS` and `TestOCCTDAGMappedNative`.

Tests retain shared leaf identity through different ancestors, deep-copied native
paths and exact bounds. They reject cycles, detached geometry, duplicate children,
limits and a fifth mapping step, while accepting an explicitly added fourth
step. A bounded100-execution parser fuzz campaign uses the independent DAG seed.

## Explicit root versus shared definition edits

`WriteDAGRationalMappedRootPlacements` edits a nonempty selected subset of root
uses. A source item used internally as well is deliberately excluded: changing
it has wider effects. `WriteDAGRationalMappedSharedPlacements` explicitly edits
internal definitions, affecting every incoming path containing that item—even
if it is also a root use. Neither API silently clones a use to hide shared
semantics. Duplicate, foreign or wrong-role IDs reject atomically.

Selected items receive private newly authored operators. Every unselected item,
local geometry record, name, context and membership remains unchanged. The input
source is immutable; old unused operator dependencies are removed only when no
remaining source owner references them. Complete DAG proof runs after editing.
New ancestry names actual new target IDs while item/map/path identities remain.

Native `WithDAGRootPlacements` and `WithDAGSharedPlacements` return new separately
validated documents and require unchanged local asset bytes. Existing native
version/format contracts remain. Tests prove all four incoming paths of a shared
leaf update, all unaffected poses and retained records stay unchanged, and the
root API rejects a source item reused both at root and internally.

The live fixture moves one root group to Z100 and the shared selected leaf to
X30. Native reload→STEP→pinned OCCT gives bounds(−10,−10,0) to(204,156/5,104), with
unchanged shared local materials. Run `TestOCCTEditedDAGMappedNative` through
`PLAINSOLID_OCCT_MAPPED_BOUNDS`. The bounded edit fuzz target checks successful
re-read, unchanged local asset vertices and no partial output on failure.

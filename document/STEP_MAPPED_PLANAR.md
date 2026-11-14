# Native document bridge for exact mapped planar STEP

`ReadMappedPlanarMaterialsSTEP` and `FromMappedPlanarMaterialsSTEP` bridge the
selected exact multi-map STEP representation to the existing document schema.
They do not invent original STEP product definitions or NAUO relationships.
One local planar asset is retained per source representation map, with one
explicit root node and one placed child node per MAPPED_ITEM. Repeated mapped
uses refer to the same asset. Names, original length unit and local signed rigid
transforms remain separate from geometry.

The From adapter treats the complete Source as authoritative: it revalidates that
graph and compares every cached map/body ID, name, unit, occurrence transform and
native vertex/edge/face array before creating any document. Stale or forged
geometry/placement caches fail atomically. STEP parser limits and existing
STEPAssemblyOptions node/depth limits bound the conversion. No schema version or
external dependency is added.

Existing labels preserve source map/body IDs on assets, source item/operator IDs
on occurrence nodes, and source face/surface IDs on each native face index.
The exact decoder retains CLOSED_SHELL source face ordering, so this is a direct
index binding, not a closest-face or name guess. Tests independently verify each
linked native face's vertices lie on its original source plane. These labels are
historical provenance; after arbitrary document edits they are not themselves a
new proof of unchanged source geometry. The adapter does not install a generic
source regeneration recipe.

The original full STEP graph is not embedded into native document JSON. Therefore
source-only mapped write-back remains the separate exchange API using its retained
validated Source. The document's existing WriteSTEPAssembly can instead author a
new product assembly from the current local assets and nodes; that is explicitly
a new representation, not a claim to preserve original mapping entity IDs.

## Independent fixture and native chain

The fixture is the independent OCCT-authored two-box/multi-map source documented
in exchange/step/MULTIPLE_MAPPED_PLANAR_MATERIALS.md. Import retains two local
assets and three uses. Exact baked bounds are:

- first translated: (10,0,0)–(12,3,4)
- first rotated: (0,20,0)–(4,22,3)
- second translated: (-10,0,0)–(-9,2,3)
- combined: (-10,0,0)–(12,22,4)

Native JSON write/read retains the shared asset references, node names/placements,
units and all twelve face/surface provenance labels. Native flattened mesh has
volume54 and area126. A new document STEP assembly exported after native reload
passes pinned OCCT ordinary TransferRoots/BRepCheck: three solids,18faces,
volume54,area126,centroid(85/18,91/9,31/18).

Enable TestOCCTMappedPlanarDocument with PLAINSOLID_OCCT_DISK_BOUND_MASS. Regressions
cover forged native geometry, changed transforms/body IDs/map order/names/units,
node/depth/geometry limits, independent source-plane linkage and native reload.

## Authoring edited mapped placements

`State.WriteEditedMappedPlanarMaterialsSTEP` takes the separately retained source
and authors changed collection/occurrence names and proper signed-cardinal
occurrence transforms after native save/reload. It preserves the original shared
local products and exact model/pcurve geometry. This is a new mapped graph, not a
NAUO exporter or unchanged source write-back. Its lower-level counterpart is
`step.WriteEditedMappedPlanarMaterials`.

The bounded scope keeps the original two-to-eight map definitions and up to
sixteen occurrence identities. The document must retain the imported flat root
and child layout, exact local vertex/edge/face arrays, asset associations and
length unit. Changed local geometry, unit, hierarchy, membership, reflection or
scale is rejected atomically. Historical labels are not proof inputs. Asset and
face labels are not exported as newly authored product metadata. Local product
names and all other source metadata remain unchanged; only the collection name
and mapped occurrence names are editable by this API.

Each new operator receives private point/direction records. Existing source
datums, including any shared with local geometry, are never modified. Orphaned
old operator dependencies are removed by a validated root closure, while all
still-owned geometry remains. The complete staged output is re-read under caller
limits before writing. Exact binary translations are written as finite exact
decimals, including values whose short decimal spelling would change the source
rational value. The input source graph is unmodified and may still be exported
separately with the source-only API.

The independently OCCT-authored two-box fixture is imported, saved natively,
reloaded and edited by adding 2.5 to each occurrence's X translation. New mapped
output passes pinned OCCT TransferRoots and BRepCheck: three solids,18faces,
volume54,area126,centroid(65/9,91/9,31/18). Its rotated occurrence retains its
nonidentity signed frame. `TestOCCTEditedMappedPlanarDocument` uses the existing
`PLAINSOLID_OCCT_DISK_BOUND_MASS` helper. Tests also bind source map/item identity,
reject stale native geometry and invalid membership, verify immutable source and
deterministic bytes, enforce limits, and propagate short writes.

## Native documents without imported sources

`State.WriteNativeMappedPlanarSTEP` creates a new mapped representation directly
from native planar assets. It requires one asset-free identity root and a flat
set of direct placed children, two to eight local planar assets and at most
sixteen uses. Every asset must be used. Asset IDs supply local product names;
root/node names supply the collection and mapped-use names. Deterministic sorted
asset/node IDs determine entity generation order. Original STEP source graphs
or provenance labels are unnecessary, so edited native local geometry is
accepted when it passes the cardinal planar writer's complete proof.

Units remain explicit. Nonplanar assets, unevaluated features, nested hierarchy,
nonidentity root placement, orphan assets and unsupported placement frames fail
before publication. Labels are not transferred as STEP metadata. The existing
NAUO writer and source-bound edit writer remain separate APIs.

An independently authored native two-box document is saved and reloaded before
export. The pinned OCCT ordinary TransferRoots/BRepCheck and AddOptimal world
bound check confirms minimum(−10,0,0),maximum(12,22,4), including the rotated reuse.
`TestOCCTNativeMappedDocumentBounds` uses `PLAINSOLID_OCCT_MAPPED_BOUNDS`; build its
standalone helper with `tools/occtref/build_mapped_planar_bounds.sh`. The helper
uses the same pinned OCCT revision as the native mapped writer qualification.

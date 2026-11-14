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

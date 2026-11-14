# Distinct exact local STEP representation maps

`ReadMappedPlanarMaterials` extends the mapped-material workflow through a
separate API:2–8 distinct local planar representations, at most16 explicit mapped
uses total, one root representation and one declared product shape. Each local
map enters the full `ReadMappedPlanarMaterial` proof independently: original
cardinal planes, model lines, per-owner line pcurves, exact endpoints, outward
embedded native boundary, identity mapping origin and signed-coordinate proper
unit-scale target operator. This is not an inferred multi-product NAUO assembly.

`Materials` is keyed by actual source representation-map ID. `MapOrder` follows
the original entity order; each nested result retains original body/map/item IDs,
local Body and named rigid occurrences. The overall Source retains the complete
authored graph. Nested Source values are independently parsed proof partitions
with the relevant root items; no original model geometry or source ID is changed.
The output does not flatten occurrences or alias native material geometry.

Input limits apply cumulatively before partitioning: all source faces, vertices,
coedges, bodies, bytes, entities and values count against caller budgets. Bounds
also constrain proof partition depth and the number of local proofs. Every source
entity must occur in a supported partition. Local material geometry, including
points/directions/placements, cannot be shared across distinct map owners; common
representation contexts and occurrence-target datums may be shared. An unused
map, repeated local body under a different map, cross-map curve dependency,
unknown/detached geometry or mismatched source unit context fails atomically.
The old single-map reader is unchanged.

`WriteMappedPlanarMaterialsSource` is complete validated source write-back only.
It preserves original header records, entity IDs and semantic numeric tokens;
it is not a writer for edited cached Body/Transform fields. The shared private
serializer also preserves the prior single-map writer behavior. Source failure
produces no output.

## Independent geometry-first qualification

`author_mapped_planar_boxes.cxx` independently constructs and exports native OCCT
boxes2×3×4 and1×2×3. No Go writer supplies either geometry. The Python graph author
copies each original advanced representation's geometry closure with exact ID
rebasing, retains all model/pcurve numeric tokens, and explicitly assigns the
common MM context to both local representations. It adds two maps and three uses:
first box translated(10,0,0), first box cyclically posed(x,y,z)->(z,x+20,y),
second box translated(-10,0,0). The original first product definition points at
the new root; this does not claim two independently retained product definitions.

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 reports no load or transfer
warnings. Ordinary TransferRoots/BRepCheck accept retained source and Go source
write-back: three solids,18faces,volume54,area126,
centroid(85/18,91/9,31/18). Native local proofs separately yield volumes24 and6.
`PLAINSOLID_OCCT_DISK_BOUND_MASS` enables the live differential using the existing
standalone independent property helper.

Regressions verify distinct geometry/IDs, repeated first-map identity, header
preservation, old-reader rejection, cumulative limits, unused map, duplicate
representation, cross-map model line, orphan geometry, atomic output and bounded
fuzz. Scale, reflection, nested mapping and nonidentity source-origin restrictions
are inherited without broadening their interpretation.

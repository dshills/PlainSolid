# Exact proper placements of elliptical IGES material

`ReadPlacedEllipticalPrismMaterial` is an additive21/29-record typed route for
filled/hollow prisms. Exactly one of `Filled`/`Annular` retains the proved local
source topology. One uniquely owned124 is shared by every104 conic,110 seam
line and190 plane. Datums, topology and122 supports have no transform. The502
vertices and122 terminal points are exact world values. This is important:
transforming both122 and its shared104 directrix would apply the pose twice.
The codec rejects that encoding rather than guessing which transform to ignore.

The transform must be a proper signed-coordinate rigid map (determinant+1),
with losslessly represented source-decimal translation. Inverse mapping of all
world vertices/terminals is performed with exact rational arithmetic and must
remain representable. The resulting original local graph is re-proved by the
strict filled/hollow reader. Native quarter poles are generated directly in
the exact placed frame through the shared neutral builder, never transformed
with rounded arithmetic. The two explicit writers author this same checked
encoding and re-import before output.

A rational3:4:5 rotation is not silently normalized: decimals0.6/0.8 do not
represent exact binary64 orthonormal vectors in these scalar APIs. Likewise,
translation0.1 is outside this lossless source gate; dyadic decimal7.5,-4.25,
2.125 is accepted only when every required sum remains exact. Reflections,
scales, shears, partially placed leaves, extra/unused transforms, placement on
owner topology and noncanonical record ordering fail. This milestone addresses
placement; harmless record renumbering remains a separate unimplemented gate.
No existing local or approximate-vendor reader is broadened.

The result retains original world vertices, terminal points, matrix/metadata,
local product coefficients, source role IDs and native face/rim partitions.
Version1 `Write/ReadPlacedEllipticalPrismMaterialJSON` re-proves the original
placed source and compares all snapshot fields plus canonical native geometry.
Source32KiB/512records/29entities/2048values and envelope512KiB limits apply.
Stale poses/ancestry and forged native payloads fail atomically. Writers stage
validation before I/O; an output failure may still partially write.

Independent `author_iges_placed_elliptical_prisms.py` authors both standardized
graphs without Go codec helpers. Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9
loads original and native-reloaded output with unchanged area/volume and world
centroid(12.5,-4.25,2.125). Source matrix maps(x,y,z)→(z+7.5,x−4.25,y+2.125).
The optional comparator variable remains
`PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY`. Native exact continuous joins,
positive and negative-axis poses, native recipe stability, strict legacy gates,
normal/race/Go1.23/vet and100x bounded fuzz tests qualify this subset.

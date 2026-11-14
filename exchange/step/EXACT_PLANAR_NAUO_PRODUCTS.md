# Exact flat multi-product NAUO assemblies

`ReadExactPlanarNAUOProducts` is additive to the deliberately unchanged
one-product/two-use `ReadExactPlanarNAUO`. It accepts one root and 2–7 independent
local cardinal planar products, each with one advanced B-rep, and at most 16
original NAUO occurrences directly beneath that root. Every child must be used.
There is no nesting, mapped-item inference, approximate frame normalization, or
inter-product Boolean/material-union claim.

Each local body receives the existing exact model LINE / owner PLANE pcurve
proof and global planar embedding check. Geometric dependency records cannot be
shared across local products; representation placement datums may be shared.
Vertex, face and edge-use limits are cumulative across local bodies. The normal
STEP parser and assembly budgets also apply. All representations have the same
exact SI prefix declaration, although their context entity IDs may differ.
Every occurrence retains its original NAUO, CDSR, relationship, transformation,
parent/child product and from/to placement IDs. Poses are computed from source
rational frames, with copied exact matrices and explicit failure for requested
binary64 world points that cannot be represented. The general assembly reader's
floating pose cache is not used as an exact witness.

`WriteExactPlanarNAUOProductsSource` re-proves and emits retained source, preserving
IDs and numeric tokens. It is not an authoring/edit API. Invalid input returns no
partial result; the writer validates before writing.

`document.ReadExactPlanarNAUOProductsSTEP` and
`ReadExactPlanarNAUOProductsDocument` retain shared source-local planar assets in a
separate version-1 `plainsolid-exact-planar-nauo-products` envelope. Native reload
re-proves the source and binds the canonical local assets and labels to it.
`Parts`, `Occurrences`, and `ExactWorldBounds` expose products, exact placements
and rational world bounds. No inexact generic document occurrence nodes are
created. The old one-product envelope remains distinct.

## Independent qualification

Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9` authored
`testdata/occt-exact-nauo-products.step` using
`tools/occtref/author_exact_nauo_products.cxx` and XCAF/STEPCAFControl_Writer.
Only line-end whitespace was removed from the output. It contains two distinct
local box shapes (2×3×4 and 1×2×3), three named uses, a translated occurrence and
a cyclic proper coordinate rotation. There are 735 original source entities.

Ordinary independent OCCT TransferRoots and BRepCheck accept both the retained
source and STEP → native save/reload → source export: 3 solids, 18 faces,
volume 54, area 126, centroid (85/18, 86/9, 5/2), and world bounds
(-10,-5,0)–(12,22,10), in source model units (millimetres).
Live checks use `PLAINSOLID_OCCT_DISK_BOUND_MASS` and
`PLAINSOLID_OCCT_MAPPED_BOUNDS` with the existing standalone oracle executables.

Focused tests cover independent identities, shared use, original relationship
IDs, exact poses, stale native assets, old API rejection, cross-product body
ownership, changed pcurves/units/axes/relationship ownership, orphan geometry,
and cumulative limits. `FuzzExactPlanarNAUOProducts` uses a 1 MiB / 1000 entity
bounded input and a fixed-work campaign (`-fuzztime=100x`).

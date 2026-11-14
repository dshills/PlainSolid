# Exact shared intermediate NAUO assembly

`ReadExactNestedPlanarNAUO` adds a separate bounded real assembly domain:
one root product uses one intermediate group exactly twice; that group contains
1–6 independently owned cardinal planar leaf products and 1–16 direct leaf
occurrences. Every leaf is used. The graph has exactly two relationship edges on
each root-to-leaf path, so at most 32 leaf occurrences are expanded. The existing
single-product and flat multi-product readers retain their original strict scope.

The file must use the original NAUO / context-dependent shape representation /
representation relationship / item-defined transformation graph. Parent and child
representation ownership is checked before exact source from/to frames are
composed. The intermediate group has its own shape representation and coordinate
datums; no identity group pose is assumed. All contexts declare the same exact SI
prefix, although their IDs may differ. The group contains no local material.

Every leaf receives exact original LINE / planar owner-pcurve verification and
closed embedded planar B-rep validation. Cross-product geometric record sharing,
detached geometry, orphan products, cycles, shortcut root-to-leaf links, missing
relationships and mismatched frames are rejected atomically. The STEP parser and
assembly budgets apply; local vertex, face and edge-use limits are cumulative
across distinct leaf bodies. Expanded occurrence count has its own fixed cap.
The native solids remain in their source local coordinates.

`Relationships` retains each original direct source use once. Each expanded
`ExactNestedNAUOOccurrence.Path` contains the ordered outer and inner records,
including NAUO/CDSR/relationship/transformation/from/to/product IDs. The same
inner relationship appears on each incoming group path, while the outer use
remains distinct. Exact immutable world matrices are separately composed from
those two poses. `ApplyRepresented` explicitly rejects nonrepresentable binary64
world points; `ApplyExact` preserves rational coordinates.

`WriteExactNestedPlanarNAUOSource` validates and re-emits original source IDs and
numeric tokens. It does not synthesize or edit arbitrary assemblies.
`document.ReadExactNestedPlanarNAUOSTEP` and
`ReadExactNestedPlanarNAUODocument` use a distinct version-1
`plainsolid-exact-nested-planar-nauo` envelope. They persist each local native asset
once, retain original group and per-path ancestry, and re-prove source/local
geometry binding on reload. Exact world bounds are available without fabricating
floating occurrence nodes. There is no inter-occurrence material-union or
noninterference claim.

## Independent qualification

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` generated the retained
767-entity `testdata/occt-exact-nested-nauo.step` with
`tools/occtref/author_exact_nested_nauo.cxx`. This is genuine XCAF output; only
trailing whitespace was removed. The intermediate group contains a 2×3×4 box
used twice and an independent 1×2×3 box. The root reuses that complete group under
a translation and a cyclic proper coordinate rotation. All source coordinates
and exact relationship frames are retained.

Independent ordinary STEP TransferRoots and BRepCheck accept both the original
file and native save/reload/source export. They report six solids, 36 faces,
volume 108, area 252, centroid (965/18,2057/36,217/36), and bounds
(0,-5,-5)–(112,112,22) in model millimetres. Optional checks use
`PLAINSOLID_OCCT_DISK_BOUND_MASS` and `PLAINSOLID_OCCT_MAPPED_BOUNDS`.

Focused tests include both product paths, immutable copied definitions/poses,
native ancestry retention, source writer failure atomicity, wrong pcurves,
units, frames, cross-body ownership, cycles, shortcut relationships, cumulative
limits and old API/schema rejection. The bounded fuzz target
`FuzzExactNestedPlanarNAUO` uses 1 MiB / 1000 entities and a fixed-work 100x run.

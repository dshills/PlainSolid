# Independent exact ellipse material compound

`ReadExactEllipticalPrismCompound` / `WriteExactEllipticalPrismCompound` handle
exactly two independently rooted standard STEP products. This is a multi-root
material file, not a custom container, NAUO assembly, inferred occurrence or
Boolean union. Original model-space frames encode each body's placement; no
geometry is baked, snapped or moved by the compound adapter. Spatial overlap
is not tested or prohibited. The independently qualified fixture is disjoint.

Each member passes the complete seven-variant ellipse family proof and returns
its own typed material, units, original body/edge/face lineage and exact native
solid. Both members share the strict millimeter unit contract. Parsing is
bounded cumulatively to 128 KiB, 236 entities, 8192 values and depth 16; native
face, vertex and edge-use limits are charged across members. Dependency cycles,
orphans, additional roots, missing product metadata and unsupported material
fail with an empty result.

The only allowed cross-root sharing is the exact identity representation
placement and its three point/direction records. Both complete leaf proofs
must authenticate these in their neutral representation role. Material curves,
supports, vertices, edges, shells, product records, contexts and units remain
independently owned. Shared geometry cannot masquerade as this identity frame.
This accommodates the specific genuine OCCT multiple-product output without
introducing general graph aliasing.

The writer accepts two retained source Files and re-proves both before output.
It rebases record IDs deterministically into disjoint output copies and checks
the complete combined graph before any write. Original sources are unchanged;
output readers expose the new source IDs. This is source-geometry authoring,
not serialization of arbitrary edits to decoded solids. Native reload uses
ordinary trimmed JSON independently for each material, retaining rational
endpoints and complete continuous edge proofs.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 independently authors a
solid tilted elliptical prism at (0.1,2,3), radii 5/2, and a distinct orthogonal
elliptical annular prism at (20.1,2,3), outer 5/2 and inner 1.5/0.5. Both extrude
by (0,3,4). The actual standard output shares its neutral representation frame
and has 190 records. Go re-export has 194 records because each product receives
its own identity copy. Both files pass ordinary two-root TransferRoots and
BRepCheck: two solids, seven faces, eighteen edge uses, aggregate volume
302.3782929080196 (96.25π), area 384.49467617125026 and area-weighted center
(10.846430206961054,3.5,5).

The optional live test uses `PLAINSOLID_OCCT_EXACT_ELLIPSE_COMPOUND` with the
separate `tools/occtref/step_exact_ellipse_compound.cxx` helper. Focused normal,
race, Go 1.23, vet and 100 fixed-work fuzz executions cover the scoped workflow,
including shared shell/body/unit adversaries and cumulative native budgets.

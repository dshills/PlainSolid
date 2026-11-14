# Bounded independent ellipse material collection

`ReadExactEllipticalPrismCollection` and
`WriteExactEllipticalPrismCollection` extend the independently owned multi-root
STEP workflow to one through sixteen products. The previous compound API
still requires exactly two. This is standard STEP product/root data, not a
custom wrapper, inferred assembly occurrence or Boolean operation.

The reader parses once under cumulative bounds: at most 1 MiB, 1888 entities,
65536 values and depth 16, further restricted by caller Limits. Root count
also obeys MaxSolids. Native faces, vertices and edge uses consume one shared
budget across all decoded members. Every local material independently passes
the existing exact seven-variant family proof. Reader failure returns no
partial members; writer validation finishes before any output.

Only the exact neutral representation frame may be shared, with all owners
proving its designated role. Material supports, curves, topology, units,
contexts and product metadata remain independently owned. Duplicate shared
model dependencies, orphan records, cycles, unsupported roots and malformed
later members fail. Original rational model frames and coordinates are
retained literally. The writer deterministically rebases each product into
independent records, without changing any geometry or constructing occurrences.
There is no cross-product spatial separation claim.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 independently authors three
products: a tilted 5/2 prism at x=0.1, an orthogonal 5/2–1.5/0.5 annular prism at
x=20.1 and another tilted 3/1 prism at x=40.1, all with origin y=2,z=3 and displacement
(0,3,4). Its 262-record output shares one neutral representation frame across
all three roots. The pure-Go output uses 270 records with independent identity
copies. Source and output pass TransferRoots(3) and BRepCheck: three solids,
ten faces, twenty-four edge uses, volume 349.50218271186532 (111.25π),
area 470.16869819555723 and area-weighted center (16.177008342760743,3.5,5).

Native reload is tested independently per member. Focused ordinary/race,
Go 1.23, vet and 100 fixed-work fuzz executions cover the three-root source,
old pair behavior, one-root acceptance, root-limit rejection, cross-owner
mutations and cumulative material budgets. The optional live test uses
`PLAINSOLID_OCCT_EXACT_ELLIPSE_COLLECTION` and the separate
`tools/occtref/step_exact_ellipse_collection.cxx` helper.

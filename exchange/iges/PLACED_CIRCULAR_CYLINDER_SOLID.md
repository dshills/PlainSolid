# Proper signed placement of circular IGES186 material

`ReadPlacedCircularCylinderSolid` and `WritePlacedCircularCylinderSolid` add
one explicit type124 placement to the canonical analytic cylinder material.
The matrix is a proper signed-coordinate permutation (determinant +1) with
finite translation. Radius and height are unchanged; arbitrary rotations,
scales, shear, reflections, transform chains and tolerance-based frame fitting
remain outside scope.

All six analytic/model leaves share the same terminal matrix, source ID 49:
the type192 side, both type190 cap supports, both type100 circle models and the
type110 seam. Support datums, four degree-domain pcurves, and every topology
entity have zero transform pointers. The two type502 vertices are authored in
world coordinates. This layout is independently verified in OCCT; a transform
on a root or shell is neither accepted nor inferred.

Each source world vertex is inverted using exact rational signed-coordinate
arithmetic. Any required rounding fails. The local 24-entity source then passes
the entire original circular-cylinder proof, including integer lexical fields,
complete 0–360-degree side coverage, full circular model edges, continuous
owner correspondence and outward cap/side ownership. Every local support and
model edge receives the same proper isometry, so those whole-curve identities,
embedding and material orientation are preserved. The world native cylinder
uses the exact transformed origin, signed reference and signed axis; there is
no reconstruction by sampled fitting.

`Local` is explicitly the normalized local proof graph. `Placement` retains
all authored matrix coefficients. `SourceVertices` retains the actual world
vertices, rather than silently replacing them with local points. `WorldFrame`
and `Solid` describe the world material. Original directory labels/subscripts
are retained, including separate transform metadata. This is a typed geometry
workflow, not a byte-preserving opaque IGES editor.

The bound is 25 entities, 512 records, 2048 values and 32 KiB; tighter caller
limits apply. Writers require lossless local and world seam coordinates and
stage complete typed re-import before output. Existing standalone circular and
mixed collection APIs are unchanged and do not infer this new placed family.

## Independent qualification

`author_iges_placed_circular_cylinder.py` independently authors the positive
source using `(x,y,z)->(z+7,x-4,y+2)`. The radius-2 height-3 cylinder's axis is
therefore world +X; its center is (8.5,-4,2). Pinned OCCT
`4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots/BRepCheck accepts
source and Go output as one solid/three faces, volume 12π and area 20π. Go output
with an additional nonzero local origin also verifies that placement does not
discard source-local offsets. Numerical measure comparisons are separate from
exact source acceptance.

Tests verify exact frames and retained world vertices, alternate proper signed
poses, local geometry roundtrip, absent/foreign/UV/root/datum placements, chained
matrices, inconsistent world points, invalid translation/scale, cumulative syntax
limits, cancellation, staged writer failure and bounded fuzz input. Optional
live checks use `PLAINSOLID_OCCT_IGES_SOLID` with the existing topology oracle.
No external runtime dependency or conical-family expansion is introduced.

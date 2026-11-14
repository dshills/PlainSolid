# Independently owned mixed IGES material roots

`ReadMixedTopologySolids` and `WriteMixedTopologySolids` support a collection of
2–8 type186 solids containing both the exact tetrahedral planar family and the
original parabolic spline-roof family. Every root retains its native material
proof; there is no faceting of the roof or generic approximate surface fitting.
This is a multi-solid file, not IGES assembly placement, instance sharing, or a
Boolean union. Bodies may overlap; inter-body noninterference is not claimed.

The reader traverses only declared topology references in
186→514→510→508→504/502 plus the original type126/128 geometry. Each source entity
must belong to exactly one root. Cross-root topology/curve/support sharing,
orphans, missing targets, unsupported entities and invalid topology fields fail.
A component is reindexed in source directory order solely to invoke its existing
strict material decoder. `LocalToSource` reverses every proof-local ID to the
actual authored file ID; `SourceSolidID` is always the actual source root.
Nested tetrahedron/roof objects explicitly contain proof-local IDs. Original
geometry, labels and subscripts are preserved with that map. Raw directory
style/status/transform conditions are checked before reindexing, so proof staging
cannot erase unsupported metadata. All global units are shared and preserved.

Component structure chooses one decoder, never a best-effort fallback. A
34-entity component must pass the complete tetrahedron topology, continuous
curve/UV, convex halfspace and closed-material proof. A 58-entity component must
pass the role-independent original roof topology, exact continuous composition,
source-weight and complete global spline embedding proof. Neither entity count
alone is evidence of a valid shape. Arbitrary shells, voids, leaf/root transforms,
other spline families and disconnected subgraphs within a body remain unsupported.
Existing single-body APIs remain strict and reject these mixed files.

The aggregate caps are 512 KiB, 8192 records, 464 entities, 32768 values and
832 control points; caller limits may tighten them. Proof-local input uses each
existing family's additional caps. A writer uses explicit family descriptors,
rejects irrelevant payload fields, validates every local material, remaps only
typed reference positions, and validates the final ownership graph before I/O.
Read failures are atomic. Cancellation is checked before and between bounded
component proofs.

## Independent qualification

`tools/occtref/author_iges_mixed_topology.py` combines independently authored
reference tetrahedron and mapped-roof records using IGES pointer semantics;
it consumes no Go writer output. The retained 92-entity fixture is
`testdata/mixed-topology-solids.igs`. Ordinary TransferRoots/BRepCheck with pinned
OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts source and Go output as
2 solids with 10 faces. Independent expectations are volume 8/3,
centroid (.65625,-.5,2.1), and area
`6+2*sqrt(3)+20/3+sqrt(5)/2+asinh(2)/4`.
The external numerical area comparison uses 1e-8, not an acceptance tolerance.
Optional live tests use `PLAINSOLID_OCCT_IGES_SOLID`.

Tests cover both body orders and units, original ID maps, cross-root references,
invalid source geometry/orientation, ignored-looking metadata, cumulative budgets,
writer staging, cancellation and bounded decoder fuzzing. No external dependency
is required at runtime.

## Conical STEP boundary identified during scope review

This milestone deliberately does not add an advertised exact conical assembly
family. The existing STEP cone codec writes a semi-angle and reconstructs slope
with `tan`; for example `tan(atan(.5))` is `.49999999999999994` in the tested Go
implementation. A binary equality check for selected values would still not prove
mathematical equality between a decimal semi-angle and a rational slope. Such a
representation needs an explicit supported numeric/error contract rather than
silently treating altered cap-radius relationships as exact. Existing cone APIs
and their documented behavior are unchanged.

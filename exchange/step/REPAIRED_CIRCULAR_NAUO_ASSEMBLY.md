# Opt-in mixed circular NAUO native import

`ReadRepairedCircularNAUOAssembly` is a separate opt-in family. It requires both
exact cardinal-planar local products and explicitly reconstructed canonical
cylinder or concentric through-tube products. Classification comes from each complete original shell's
support graph, never a product name or marker. Other wires, open surfaces,
splines, sectors, cones and unknown product items are rejected atomically.

Planar bodies retain their existing exact model/pcurve and embedded boundary
proof. Circular bodies go through `RepairCircularNAUOProduct` with the supplied
`MaxDistance`; its exact source residuals, representability gates, displacement
bounds and limits are unchanged. Each local circular product is repaired once,
not once per instance. Repeated use does not duplicate local assets or evidence.
All old exact readers remain strict. This is not source-equivalent exact import.

The original flat or DAG product graph retains exact rational occurrence poses
and full NAUO/CDSR/relationship/transformation/datum paths. Limits are 8192 source
entities, 8 products, 32 direct relationships, 4 relationship edges per path and
64 expanded leaf occurrences, in addition to ordinary parse limits. The graph
must have one root; every group/product must be reachable and used. Each product
has one representation and each material product one body. Cross-product
geometric dependency sharing, orphans, cycles and detached material fail.
Separate contexts must declare identical exact SI prefixes. Vertex/face/edge-use
limits are cumulative across native local assets, including the new cylinder's
polar cap topology. No floating approximation is installed as an exact pose.

## Native envelope and separate evidence

`document.ReadRepairedCircularNAUOSTEP` returns a
`RepairedCircularNAUODocument`, one displacement report per repaired asset, and an
error. The local generic native document stores shared `Planar` and `Parametric`
assets, keyed by original `product-<definition-ID>`. Labels preserve product/body
IDs and explicitly distinguish `exact-planar` from `repaired-circular`.
There are no fabricated floating `State.Nodes`. `Instances` supplies immutable
exact occurrences with deterministic IDs from the ordered original NAUO path.
`LocalAssets` returns copied native maps; `Parts`, `Groups` and `Relationships`
retain source identity. `ExactWorldBounds` computes rational extrema; if a
cylinder's projected radius needs an irrational square root, it fails explicitly
rather than returning rounded values. The exact poses/assets remain available.

Version 1 of `plainsolid-repaired-circular-nauo` stores original source, the
explicit repair tolerance declaration, and native local assets. It does not
serialize or trust the displacement reports inside that native payload.
`ReadRepairedCircularNAUODocument` requires the caller's matching tolerance,
re-proves source and native binding, and returns newly computed reports outside
the payload. Unknown fields, stale geometry/provenance and unsupported versions
fail with no partial document or report. Mutating a previously returned report
cannot change the immutable document or its later regenerated evidence.

`WriteOriginalSTEP` deliberately returns the original retained file, including
its measured defects. It is not export of reconstructed native geometry. Native
assets can instead be deliberately authored through existing qualified geometry
writers; those newly constructed files have their own topology/metadata contract.
No source face-trim equivalence, global assembly Boolean material or whole-source
Hausdorff bound is claimed.

## Genuine mixed XCAF DAG qualification

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` authors the 567-entity
`testdata/occt-circular-nauo-dag.step` using
`tools/occtref/author_repaired_circular_nauo_dag.cxx`. Only trailing whitespace is
removed. Root uses groups A and B; B also uses A and the same cylinder leaf used
inside A. The original planar box is used four times and the cylinder three
times, through both two-edge and three-edge relationship paths. Repair at 1e-9
model units produces one report with the four original failed conditions intact.

Original source, native save/reload, and a deliberately reconstructed export via
`WriteCylinderMaterialAssembly` are compared with ordinary pinned OCCT
TransferRoots/BRepCheck. Results agree: 7 solids, 33 faces,
volume `96+60π` ≈ 284.495559215388, area `208+84π` ≈ 471.893782901543,
centroid ≈ (47.4165735404823,60.724648447686,22.6658581603059), and bounds
(0,-7,-2)–(112,112,72), in source model millimetres. Native world measures sum
actual local B-rep/parametric geometry over retained occurrence paths; they are
numerical validation, not a new exact quadrature certificate.

Focused tests cover flat and DAG input, shared asset counts, source ancestry,
report isolation, native reproof, caller-tolerance binding, stale payloads,
unsupported families/wires, cross-product ownership, exact pose violations,
orphan bodies, cumulative limits and unchanged exact-reader rejection.
Normal, race, Go 1.23, vet and fixed-work
`FuzzRepairedCircularNAUOAssembly -fuzztime=100x` qualify the bounded import.

## Separate through-tube evidence and native version 2

`RepairedNAUOTube` adds the independently qualified four-face tube graph as a
distinct local family. `TubeRepairs` contains one complete
`RepairedCircularTubeNAUO` per tube product, with both original wall reports,
cap roles, selected master pcurve IDs and model seam parameter scales. The
existing cylinder `Repairs` table and cylinder fields keep their meanings.
Classification counts the complete support graph, then invokes the independent
strict tube repair; it never treats a tube as a cylinder or tries unknown shapes
until a tolerance-based decoder accepts. Cumulative native limits count all
sixteen tube patches/vertices and sixty-four uses.

New `document.ReadRepairedCircularNAUOSTEPWithEvidence` and
`ReadRepairedCircularNAUODocumentWithEvidence` return separate `Cylinders` and
`Tubes` tables in `RepairedNAUOEvidence`. Both tables are removed before the
immutable native document is retained. Reload re-proves and regenerates them;
no report is serialized or trusted. The original cylinder-only entry points
reject any tube-bearing source/payload, rather than silently discarding reports.
Tube-bearing payloads use version 2; cylinder-only payloads remain version 1.
Forging either version against the actual source family rejects atomically.

Each tube stays one parametric local asset shared by all exact occurrence paths.
`Parts` retains its distinct tube descriptor and original product/body identity.
Exact world bounds use the tube's outer cylindrical extrema; the existing
rational square-root gate still rejects unrepresentable exact projected bounds.
No floating occurrence nodes are fabricated. `WriteOriginalSTEP` retains its
explicit source-only meaning. This milestone does not add a general repaired
mixed assembly authoring codec.

The independent `author_repaired_tube_nauo_dag.cxx` uses a real OCCT cylinder
Boolean cut alongside an independent cylinder and box. Its 772-entity retained
`occt-tube-nauo-dag.step` has three local materials: four box uses, one cylinder
use and three tube uses through reused intermediate groups. Pinned OCCT ordinary
TransferRoots/BRepCheck returns eight solids, thirty-nine faces, volume
`96+140π` ≈ 535.822971502571, area `208+196π` ≈ 823.752160103600,
centroid ≈ (64.1069563124328,53.9353241155802,18.7184054266645), and bounds
(0,-8,-3)–(202,112,72). Independently summing actual native geometry over exact
paths after save/reload agrees with those world aggregates. The separate local
tube source/new-output transfer remains qualified by the tube milestone.

Tests retain original cylinder cases and add mixed family identity, repeated
tube asset counts, report mutation isolation, native byte stability, exact bounds,
version/tolerance/stale-geometry/serialized-evidence rejection, cross-family
pcurve ownership, wrong material sense/support and cumulative budgets. The mixed
fixed-work fuzz target includes this new positive source under sufficient native
limits; normal, race, Go 1.23 and vet remain required.

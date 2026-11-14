# Opt-in mixed circular NAUO native import

`ReadRepairedCircularNAUOAssembly` is a separate opt-in family. It requires both
exact cardinal-planar local products and explicitly reconstructed canonical
cylinder products. Classification comes from each complete original shell's
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

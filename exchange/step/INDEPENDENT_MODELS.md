# Independent model partitioning

`JoinIndependentModels` combines complete source graphs by renumbering their
entities. `SplitIndependentModels` returns each supported shape representation's
reference closure and, when present, its complete registered nine-record AP203
product envelope. This preserves real per-body STEP readback for strict rooted
dialects such as side-open dual-disk cuts and two-/four-cell cylinders.

The registered envelope is the `APPLICATION_CONTEXT`,
`APPLICATION_PROTOCOL_DEFINITION`, `MECHANICAL_CONTEXT`, `DESIGN_CONTEXT`,
`PRODUCT`, `PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE`,
`PRODUCT_DEFINITION`, `PRODUCT_DEFINITION_SHAPE` and
`SHAPE_DEFINITION_REPRESENTATION` form emitted by PlainSolid's AP203 writers.
Record signatures and reference relationships are checked. Product identifier
and name strings are retained; this partitioner does not assert their geometric
meaning. Each body's strict reader still validates the geometry and its own
name/product contract.

Metadata-less sources remain metadata-less. Legacy multi-body planar output may
share explicitly classified declarative unit, representation-context and
application-context records; these are copied into each partition. Shape
entities and product definitions cannot be owned by multiple partitions.
Assembly mappings, incomplete or unknown product envelopes, detached geometry
and orphan metadata are rejected. No partial output is returned after failure.

Compared with the earlier splitter, registered product metadata is now retained
rather than discarded. Previously ignored orphan/unsupported metadata and
cross-model shape sharing now fail closed. No general STEP assembly splitting
or arbitrary vendor metadata preservation is claimed.

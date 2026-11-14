# Source-bound torus assembly persistence

`WriteIndependentTorusAssemblyJSON` and `ReadIndependentTorusAssemblyJSON` use
the distinct version-1 tag `plainsolid-step-independent-torus-assembly` for the
bounded flat assembly described in `INDEPENDENT_TORUS_ASSEMBLY.md`. Existing
collection/native tags and geometry acceptance remain unchanged.

The envelope retains original STEP source, product/root/representation IDs,
occurrence IDs and exact poses, local material ancestry and canonical native
geometry. Local materials are sorted by product-definition ID and stored once
per product, not once per occurrence. The torus's local frame is explicitly
encoded as an affine matrix; its original seam and vertex roles are retained.
The planar reconstruction tolerance is explicit. The torus's independent native
constructor tolerance remains in that product's canonical geometry encoding.

Writing performs one complete strict source proof, then compares all public
claims (including private frame data through structural comparison), local
source graphs and native encodings to the regenerated result. No bytes are
written on validation, cancellation or stale-claim failure. An underlying I/O
write failure is not a rollback guarantee. Reading similarly reconstructs the
assembly from the original STEP once and compares every stored claim and native
encoding. Stored native JSON is never trusted as input geometry. Repeated
occurrences therefore continue to refer to the same original local product.

`IndependentMaterialJSONLimits` bounds the envelope (default/hard maximum 16 MiB)
separately from the original STEP parser limits and strict assembly caps.
Cancellation is checked before/after bounded input and between proof stages;
the underlying I/O reader and existing parser are not themselves interruptible.
Invalid envelopes return no partial graph or material. Unknown fields, extra
JSON values, different tags/versions, inconsistent units, wrong product/body or
seam IDs, occurrence rewiring, shifted poses and changed native geometry fail.

Tests verify byte-stable native re-encoding, source/native ID retention, strict
old-tag rejection, stale writer transactionality (including private local frame
changes), limits/cancellation and bounded fuzz. With
`PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY` set to the separately built pinned
comparator, original STEP and native-reloaded source export both transfer as
three solid occurrences/eight faces with the independently expected area,
volume and posed world bounds. This is source-bound persistence, not an edited
assembly reconstruction API or acceptance of rounded vendor torus pcurves.

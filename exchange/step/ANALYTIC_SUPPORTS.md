# Mixed independent analytic supports

`ReadAnalyticSupports` / `WriteAnalyticSupports` exchange an ordered mixture of
plane, cylinder, sphere and ring-torus supports in one generic
`REPRESENTATION` → `GEOMETRIC_SET` graph. `AnalyticSupport` has exactly one
non-nil typed variant. Representation/set names, each support's placement/datum
names and explicit length unit are retained. Source set order is preserved.
This adds actual mixed-file coverage; existing homogeneous readers remain strict.

All source geometry dependencies must be uniquely owned by one support. The
reader first validates the common unit context and partitions the entire graph;
shared frames, points/directions, repeated roots, cycles, unknown roots and
unclaimed entities fail. Then each root and its unchanged numerical dependency
records are staged through the existing strict specialized codec. Units/context
are the only shared graph portion. Source identifiers and header/product metadata
are not part of the returned model. No products, occurrences, faces or material
are reconstructed; periodic supports retain their natural conventions only.

The writer stages each typed support using its specialized codec and merges
only its owned geometry under one common context. It strictly re-reads the
whole result before any caller I/O. Validation errors return an empty model or no output;
caller I/O errors propagate. Duplicate-valued independent definitions are
allowed; shared source identities are deliberately not inferred or flattened.

Limits include 64 roots, 512 entities, 4 MiB input, 8192 values, nesting depth 16,
reference depth 64 and 1024-byte entity strings. Supplied smaller parser limits
remain effective. These conservative bounds are narrower than generic STEP.

The independent OCCT fixture contains four different support types with distinct
centers and explicit frames. Optional `PLAINSOLID_OCCT_ANALYTIC_SUPPORTS` tests
check original and Go-authored files by per-root analytic points/partials in
both axis signs, preserving root order. This is geometry-entity qualification,
not bounded-face or solid transfer. Preflight hints are declaration-level only.

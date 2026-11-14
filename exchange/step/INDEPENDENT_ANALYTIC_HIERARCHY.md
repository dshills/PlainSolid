# Bounded analytic STEP product hierarchy

The `IndependentAnalyticHierarchy` read/write/source-export APIs generalize the
previous fixed two-level DAG in a separate mode. They retain exactly one proved
planar, one finite split-cylinder and one model-only ring torus local product.
Additional assembly-only groups may form a reachable DAG with:

- 4–16 total products and at most 64 original NAUO relationships;
- at most eight occurrence edges on any root-to-material path;
- at most 64 expanded material paths;
- at least two expanded uses of the torus and at least one use of each material.

Each group must have children, each product must be reachable from the single
assembly-only root, and material products cannot themselves own children.
Active-path cycle detection and a final reached-product comparison complement
the standard relationship reader. Shared groups are expanded independently per
incoming path without duplicating local native geometry. Original product IDs,
NAUO path IDs and exact composed root-space matrices remain explicit.

The traversal additionally caps work at 513 visited occurrence nodes: one root
plus at most eight nodes for each of 64 leaf paths. A valid graph cannot exceed
this bound because every group occurrence has at least one material descendant.
Exact rational matrix composition rejects any unrepresentable prefix translation;
there is no intermediate rounding. Applying a resulting exact matrix to an
arbitrary model point is not a promise of a representable baked binary64 point.
No Boolean union or inter-instance separation is inferred.

The underlying local material and exclusive geometry-closure proofs are reused
unchanged, as are the one-common-metre/mm-unit and proper signed-coordinate frame
requirements. Source parsing remains capped at 1 MiB, 4096 entities, 65536 values
and syntax/reference depth 16, in addition to lower caller limits. Native claims
retain the shared 1024-entity cap per local source view. The source
occurrence depth bound of eight is independently enforced. All original groups
and material declarations must be owned and reachable; unsupported source
pcurves, families and placements remain rejected.

`ReadIndependentAnalyticHierarchyJSON` and its writer use the distinct version-1
tag `plainsolid-step-independent-analytic-hierarchy` and a separately tagged
inner claims record. One complete source proof reconstructs local bodies and all
paths before stored metadata/native bytes are compared. Local bodies are encoded
once, with canonical stable reload. The envelope ceiling remains 16 MiB with
separate source limits. Old flat and exactly-two-level APIs/native tags keep
their original meaning and reject higher-level input. Existing atomic-validation
and I/O/parser cancellation qualifications are unchanged.

The independent standard AP214 fixture is authored by
`tools/occtref/author_step_independent_analytic_hierarchy.py`, using retained
untouched OCCT box geometry and independently authored exact model-only torus
and cylinder geometry. Its carrier group is instanced twice at the root; a
nonidentity (0,0,5) carrier-to-analytic-group placement gives genuinely three-edge
paths. The analytic group contains its torus and cylinder, while a planar leaf
remains directly at the root.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`, through the existing
`step_independent_torus_assembly` comparator, accepts the original source,
Go-authored output and source export after native reload:

- Five solid occurrences, twenty faces and eighty edge uses.
- Volume `24+12*pi²+24*pi`, area `52+24*pi²+40*pi` in mm.
- Bounds `[-10,-4,-2]` to `[30,30,9]`, with the established 2e-7 numerical OCCT
  bounding margin threshold.
- Correct independently composed surface first moments.

Use `PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY` for live qualification. Boundary
regressions accept depth eight and 64 expanded leaves, reject depth nine and a
35-link graph expanding to 65 leaves, and exercise cycles, unreachable groups,
material parents, native path/pose/owner forgery, limits and cancellation. Both
STEP and native JSON fuzz targets enforce fixed parse/native budgets. This is a
bounded hierarchy over three supported material families, not general CAD or
arbitrary-affine assembly support.

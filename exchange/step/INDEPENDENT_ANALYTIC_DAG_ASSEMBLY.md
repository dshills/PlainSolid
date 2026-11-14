# Exact two-level analytic STEP assembly DAG

`ReadIndependentAnalyticDAGAssembly`, `WriteIndependentAnalyticDAGAssembly` and
`WriteIndependentAnalyticDAGAssemblySource` support one explicit bounded topology:

- One assembly-only root, with one planar leaf occurrence and two occurrences of
  the same analytic group.
- That assembly-only group has one local torus occurrence and one local finite
  split-cylinder occurrence.
- Exactly five products, five original NAUOs, three independently owned local
  materials, and five expanded material paths.

The original planar, model-only torus and two-half/four-quarter cylinder proofs
are reused unchanged. Geometry stays in each product's local coordinates and is
proved once per local product. Group reuse does not clone the geometry or infer a
Boolean union. No inter-occurrence separation or nonintersection is claimed.
Every body closure remains exclusive to its local owner; grouping adds no route
for borrowing geometry from another product. One common explicit metre/mm unit,
proper signed-coordinate relationship frames and the existing bounded source
parse/native topology limits remain mandatory.

`Assembly` retains the original product and NAUO graph. `Instances` separately
records each expanded leaf's product-definition ID, one- or two-element path of
original NAUO IDs, and root-space transform. Paths are sorted by original IDs for
determinism. Both local relationship frames and their compositions are proved
using exact rational arithmetic. A composed translation that requires rounding
(such as 2^54+1) is rejected even if each input translation is binary64-exact.
An exact occurrence matrix does not imply that every subsequently evaluated
world point is binary64-representable; local geometry is not baked or rounded.

The old two-family torus and three-family flat APIs retain their original scope
and native format meaning. DAG persistence uses the distinct version-1 tag
`plainsolid-step-independent-analytic-dag-assembly`, containing original source,
a separately tagged set of graph/material claims, and all composed paths.
`WriteIndependentAnalyticDAGAssemblyJSON` and its reader perform one complete
source material/ownership proof and compare native encodings, original source
IDs, every local occurrence and every composed path. Native geometry is stored
once per local product. Stale or forged claims fail before any output; invalid
reads return no partial products or paths. The envelope ceiling is 16 MiB and
original source limits remain separate. Existing I/O/parser cancellation limits
and lack of rollback for an underlying partial I/O failure are unchanged.

The independent AP214 author script
`tools/occtref/author_step_independent_analytic_dag_assembly.py` composes untouched
retained OCCT planar geometry with independently authored exact model-only
cylinder and torus geometry, then authors standard NAUO/CDSR group relationships.
It uses no Go output and does not repair vendor-rounded pcurves. The cylinder's
group-local offset is (8,0,0); group uses are translated by (20,0,0), and by
(0,20,0) with the proper signed rotation cycling the axes.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts original source,
Go-authored output and source export after native reload through TransferRoots
and BRepCheck. The existing `step_independent_torus_assembly` comparator reports:

- Five solids, twenty faces and eighty edge uses.
- Volume `24+12*pi²+24*pi`, area `52+24*pi²+40*pi` in mm.
- Bounds `[-10,-4,-2]` to `[30,30,6]`, within the documented 2e-7 OCCT bounding
  margin threshold.
- Surface first moments matching the independently transformed local centers.

Use `PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY` for live qualification. Tests
cover shared leaf IDs/paths, canonical native reload, old flat-route rejection,
wrong parents, cycles, missing group reuse, material inserted into a group,
geometry borrowing/orphans, nonrepresentable composed translation, forged paths,
poses and native/source claims, limits, cancellation and atomic failure. STEP
and native fuzz targets have fixed work budgets. General DAGs, additional group
levels, arbitrary rotations and rounded analytic pcurves remain unsupported.

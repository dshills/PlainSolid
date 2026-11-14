# Explicit mixed-SI analytic hierarchy

`ReadNormalizedIndependentAnalyticHierarchy` is an additive metre/mm mode for
`IndependentAnalyticHierarchy`. It preserves the original source and a table of
original **representation IDs → units**, separately from the common-unit
`Hierarchy`. Its native tag is
`plainsolid-step-normalized-analytic-hierarchy`, version 1. Existing flat,
two-level, hierarchy and collection APIs and native tags are unchanged.

The material and graph domain is unchanged: one independently owned planar,
finite split-cylinder and model-only ring-torus local product; shared groups;
4–16 products, at most 64 original relationships, depth at most 8, at most 64
expanded material occurrences. Torus reuse, complete reachability, exclusive
geometry, source ancestry and exact signed-coordinate rigid composition are
re-proved. No material union or spatial disjointness is asserted.

## Dimensional proof

Each representation's closure is normalized from its **original** records with
an exact rational factor 1, 1000 or 1/1000. Model points, plane-UV points, LINE
vector magnitudes, circular/cylindrical radii and both torus radii scale. Axes,
directions, senses, topology, names, IDs and angles do not. Length uncertainty
scales as exact decimal metadata. Every length unit in the closure must agree
with the representation before rewriting; converted/vendor units are excluded.

All resulting geometric scalars must be finite and exactly representable in
binary64. Source decimals need not themselves be binary64-representable: for
example exactly 0.003 metres becomes exactly 3 millimetres, with no rounding.
Scalar parsing is bounded to 128 characters, 256 rational bits and the existing
bounded terminating-decimal formatter. Decimal underflow/exponent attacks fail
before arbitrary exponent expansion. Shared records must yield identical
normalized records for every owner; incompatible dimensional sharing fails.

Each parent target-frame origin uses the parent representation's length unit.
The child mapping frame is restricted to exact identity axes and zero origin.
Normalization precedes the existing exact from/to and root-path composition;
there is no implicit scaling in the returned rigid matrix. Local native assets
and each expanded instance transform use the explicitly selected target unit.
A changed scale cannot turn an unsupported support or trim into a repaired one:
the complete normalized model/UV/owner proof must still pass. Rounded vendor
torus pcurves remain unsupported.

All source parsing is bounded by 1 MiB, 4096 entities, 65536 values, depth 16
(or stricter caller limits). At most 16 representation closure passes occur.
Cancellation is checked during closure processing and native proof traversal.
Persistence uses a separate 16 MiB envelope limit and caller-controlled source
limits, retaining the existing 1024-record per-local-asset native claim limit.

## Independent consumer evidence and explicit limitation

`tools/occtref/author_step_independent_analytic_hierarchy_units.py` authors a
standard AP214 fixture independently from the Go writer. Root and analytic group
use metres, carrier/box/cylinder use mm, and the torus support and seams use
metres. Targets include a nonidentity axis permutation and nonzero translations
at several levels. The box originates in the retained genuine OCCT file; the
exact model-only cylinder/torus records are independently authored, not snapped
from vendor curves.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` ordinary TransferRoots
accepts original mixed source, normalized source and native-reloaded source:
5 solids, 20 faces, 80 edge uses;
V = 24 + 12π² + 24π mm³, A = 52 + 24π² + 40π mm²;
world bounds [-10,-4,-2]..[30,30,9] mm. Independent surface first moments are
also checked. Numerical oracle tolerances are comparison tolerances only and
never expand the exact source acceptance gate.

The companion `independent-analytic-hierarchy-units-nonzero-origin.step` is a
retained **unsupported** source. A 1 mm child mapping origin with compensated
parent metre targets produces a different rotated-branch translation in pinned
OCCT than the current exact from/to convention: the maximum world Y is 31,
not 30. This milestone does not resolve general nonzero-origin relationship
semantics. It rejects that source and all nonneutral child mapping frames,
rather than asserting source equivalence. The older same-unit APIs are not
broadened or changed by this mode.

## Persistence and re-export

`Write/ReadNormalizedIndependentAnalyticHierarchyJSON` retain original source,
original units, normalized source, graph, full occurrence paths and canonical
native local bodies. One original-source normalization/reproof compares all
stored claims; neither a stored tag nor a stored native body substitutes for
source proof. Writes validate before emitting; readers return no partial result.

`WriteNormalizedIndependentAnalyticHierarchySource` re-proves and emits original
mixed records/IDs. `WriteIndependentAnalyticHierarchySource` on
`result.Hierarchy.Assembly.Source` emits the separately proved normalized graph.
Neither route claims byte-identical lexical whitespace retention.

Qualification: focused live and legacy regression, race, Go 1.23, STEP vet,
malformed units/supports, unrepresentable conversion, nonneutral origins,
source/native limits, stale ancestry/native claims, cancellation and two fixed
100-execution fuzz campaigns. Live comparator environment:
`PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY` (binary
`/tmp/plainsolid-independent-torus-assembly-oracle`).

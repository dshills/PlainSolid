# Exact planar NAUO product DAG

`ReadExactDAGPlanarNAUO` is separate from the flat and single-intermediate APIs.
It admits one root, at least two intermediate shape-representation groups,
cardinal planar leaf products, and explicit NAUO/CDSR relationships. Limits are:
8 products total, 32 direct relationships, 4 relationship edges on any root-to-leaf
path, and 64 expanded leaf uses. Every product must be reachable; each intermediate
group must have children, and only groups/root may be relationship parents.
Shared groups and leaves are retained as original definitions, not duplicated
local geometry. Cycles, orphan products and unsupported item representations fail.

The reader independently proves each local body's original model and owner
pcurves and global embedded planar boundary. Model geometry cannot be shared
between distinct local body definitions. Placement datums may be shared.
Local vertex/face/edge-use budgets are cumulative across unique leaf products;
normal parser/assembly limits and the separate expansion cap also apply.
Separate contexts must have identical exact SI prefix declarations.

Every direct `Relationships` entry retains original product, NAUO, CDSR,
representation relationship, item-defined transformation and from/to datum IDs.
Each expanded `ExactDAGNAUOOccurrence.Path` is the ordered list of those actual
source records. Common inner records appear along every incoming path. World
poses are composed using exact rational source frames, independently of the
existing generic floating assembly pose cache. `Matrix` copies its rational
components; `ApplyExact` retains exact coordinates and `ApplyRepresented` fails
if the requested binary64 world result is not representable. No Boolean union or
inter-occurrence noninterference is claimed.

`WriteExactDAGPlanarNAUOSource` re-proves and preserves source IDs and numeric
values. It is not an edit or generic assembly authoring API.
`document.ReadExactDAGPlanarNAUOSTEP` and `ReadExactDAGPlanarNAUODocument` persist
one native asset per source-local leaf and use a distinct version-1
`plainsolid-exact-dag-planar-nauo` envelope. Reload validates source/native binding,
original intermediate definitions, direct relationships and all expanded paths.
Inspection copies the mutable path slices; exact world bounds remain rational.
Old APIs and native envelope tags retain their strict domains.

## Independent source and output

`tools/occtref/author_exact_dag_nauo.cxx` authors the 799-entity retained fixture
with pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` XCAF.
Only trailing whitespace was removed. Root has children A and B; B also uses A
and a small leaf shared with A. A contains two differently sized local boxes,
with its larger box used twice. Original translations and cyclic coordinate
rotations produce both two- and three-edge source occurrence paths.

Independent ordinary TransferRoots/BRepCheck accepts original source and
native-reloaded source export: 7 solids, 42 faces, volume 114, area 274,
centroid (1993/38,1129/19,1119/38), bounds (0,-5,0)–(112,112,72), in source
millimetres. The existing `PLAINSOLID_OCCT_DISK_BOUND_MASS` and
`PLAINSOLID_OCCT_MAPPED_BOUNDS` standalone executables run the optional checks.

Focused tests preserve repeated group and shared leaf ancestry, reject old API
cross-use, malformed source ownership/pcurves/units/frames/cycles and stale native
assets, prove copy ownership, and test cumulative limits. An independently
expanded complete source relationship graph is accepted for one added root use
and rejected when expansion exceeds 64 while direct relationships remain below
32. `FuzzExactDAGPlanarNAUO` bounds input to 1 MiB / 1000 entities, with a fixed
100x campaign. Normal, race, Go 1.23, vet and live OCCT checks qualify this slice.

## Source-retaining root pose edits

`WriteExactDAGNAUORootPlacements` takes a nonempty selected subset of
`ExactNAUOPlacement` records keyed by original NAUO ID. The origin and raw X/Z
ratios describe the desired child-to-parent proper rigid pose. Inputs are copied;
perpendicular ratios must have exactly rational normalizations. The new target
frame is **desired pose composed with the original from frame**, rather than an
assumption that the from frame is identity.

This operation preserves every entity ID, header, ordering, geometry record,
product and relationship record, including both from/to datum IDs. It changes
only the target datum's point, Z direction and X direction values. Those three
records must each have one incoming reference, from that datum. The target datum
must have exactly two incoming references: one from its root representation and
one from the selected item-defined transformation. Shared/geometry-linked targets
fail explicitly; the writer does not clone, detach, or silently edit other uses.
Internal relationship edits are not accepted by this API.

Origins must have exact terminating STEP decimal representations within 128
characters; nonterminating translations (such as 1/3) fail. Directions are emitted
as exact common-denominator integer ratios, so a rational 3/5,4/5 rotation can be
represented by source direction (3,4,0). Input rational numerator/denominator
limits are 4096 bits. The writer stages and re-reads the complete DAG and verifies
the requested exact relative poses before writing any bytes. Tests compare every
unselected source record, and verify that a changed root→A occurrence moves only
its paths while root→B→A remains unchanged.

`ExactDAGPlanarNAUODocument.WithRootPlacements` returns a new source-bound native
envelope. The original document and shared local assets remain unchanged. Native
save/reload then source re-export retains the edited paths and all ancestry.
Independent pinned OCCT verifies both translation and 3:4:5 rotation edits:

- Translating root→A by another +100 X gives bounds (0,-5,0)–(212,112,72),
  centroid (3793/38,1129/19,1119/38).
- Setting its pose to origin (200,0,0), X=(3/5,4/5,0), Z=(0,0,1) gives bounds
  (0,-11,0)–(1036/5,112,72), centroid (953/10,297/5,1119/38).

Both retain 7 valid solids, 42 faces, volume 114 and area 274. Adversaries include
internal/unknown/duplicate use IDs, shared targets, nonterminating/oversized
translations, irrational/nonorthogonal axes and reduced parser limits. A
nonidentity original from datum is explicitly tested. The fixed-work writer fuzz
target `FuzzExactDAGNAUORootEdit` verifies atomic failure and re-readable successes.

## Explicit shared internal-use edits

`WriteExactDAGNAUOSharedPlacements` and native
`ExactDAGPlanarNAUODocument.WithSharedPlacements` are separate opt-in operations
for internal group→group or group→leaf relationships. They reject root use IDs;
the existing root-only operation continues rejecting internal use IDs.
Selection identifies the original **NAUO relationship**, not its child product
or one expanded occurrence. Consequently every root-to-leaf path containing that
selected use changes together. Other NAUOs referring to the same child product
remain unchanged. Every original path and source entity ID is retained.

Exact pose, decimal and ownership gates match the root API, except the exclusive
target datum is owned by the selected use's actual parent representation. Shared
datum targets remain unsupported even when selected relationships share the same
requested pose. No implicit co-edit or geometry change is introduced. Input,
source staging, native immutability and output atomicity guarantees are unchanged.

The genuine XCAF fixture's A→large-leaf translation is changed from (10,0,0) to
(20,0,0). This use appears under root→A and root→B→A, so it changes one world X
placement and one world Y placement. Other references to the large product and
the independent small leaf remain unchanged. Native reload and independent
pinned OCCT re-transfer report 7 valid solids, 42 faces, volume 114, area 274,
bounds (0,-5,0)–(122,122,72), centroid (2073/38,1169/19,1119/38).
A separate group→group edit verifies all three descendant paths move and no other
paths do. Focused tests compare every unedited source record, reject root IDs,
duplicate/unknown/internal malformed poses/shared target datums, and exercise
bounded atomic writer fuzz (`FuzzExactDAGNAUOSharedEdit`, 100x), race and Go 1.23.

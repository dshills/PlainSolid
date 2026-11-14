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

# Path-specific AP242 whole-body appearance

`ReadSHUOOccurrenceColoredAssembly` authenticates one original
`SPECIFIED_HIGHER_USAGE_OCCURRENCE` (SHUO) in the already qualified two-level
repeated-group planar assembly grammar. Unlike a shared internal NAUO override,
this override selects **both** the outer group use and its inner leaf use.
The other incoming path to the same group and same leaf retains its default.

The positive fixture is genuine OCCT AP242 output, authored by
`tools/occtref/author_ap242_shuo.cxx` using `XCAFDoc_ShapeTool::SetSHUO` on an
explicit two-component label sequence and a color on the returned graph node.
No Go-generated entity or geometry templates are used. The retained 797-entity
file has only trailing whitespace normalized.

## Proof and scope

Both usage records must be ordinary NAUOs: the upper relating definition equals
the SHUO relating definition, the upper related definition equals the next
relating definition, and the next related definition equals the SHUO related
definition. The pair must identify exactly one expanded authenticated path.
The SHUO has one exclusively owned PDS and SDR. Its context representation,
whole-body style, default predecessor, body, geometry context and next-use
CDSR/IDT/placement are authenticated through the same strict contextual-chain
proof used by the earlier API.

A private proof view substitutes the already authenticated next-use PDS only to
reuse that chain proof. The original source is never overwritten or emitted as
that view. Source SHUO/PDS/SDR IDs and both original NAUO IDs are retained in
`SHUOSurfaceColor`. After local geometry and the complete hierarchy are proved,
default leaf colors are restored and the original two-condition path is selected.
No shared-use interpretation leaks into the returned effective appearance.

The result is immutable through public observations; `Paths` and `Overrides`
copy values, and `NativeAssembly` returns a fresh source-reproved local model.
`WriteSHUOOccurrenceColoredAssemblySource` re-proves and re-emits original source.

Exactly one SHUO is supported. Mixed ordinary/shared contextual overrides,
additional or recursive SHUOs, deeper assemblies, group styles, per-face
instance styles and general inheritance are rejected. Existing flat and shared
internal-use readers remain strict and reject this original source. The prior
local planar geometry, common unit, proper signed-axis frame and exact
composition restrictions remain unchanged. There is no new general geometry
or tolerance acceptance claim.

The bounded source/native limits remain 1 MiB/8 MiB, 4096 source entities,
16 products/bodies and 64 expanded leaf paths, with smaller caller limits
honored. Cancellation is checked around the bounded source proof and during
existing expansion/copy operations, not inside the legacy parser.

## Native persistence

`Write/ReadSHUOOccurrenceColoredAssemblyJSON` use the separate version-one tag
`plainsolid-step-shuo-contextual-appearance`. Source, original topology/geometry,
styles, both usage links, all contextual ancestry and effective paths are
re-proved and compared. The supplied stored geometry is not trusted to rebuild
shape. Wrong tags, forged native coordinates, changed ancestry/path/color or
source are rejected atomically. Earlier native formats keep their semantics.

## Independent observation

Pinned OCCT commit: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
The observer explicitly enables `STEPCAFControl_Reader::SetSHUOMode(true)`;
OCCT's default is false and would omit SHUO appearance. It recursively retains
original XCAF component-label paths and calls `FindSHUO` for each path, then
reads the referenced graph-node color. Flattened `TopoDS` location lookup is
not used to guess path identity.

BRepCheck, ordinary mass/area, all six world translations/rotation matrices,
component-color presence and RGB are checked for retained source, native reload
and source re-export, and freshly independently authored source. Assembly
occurrence sums are V=108 and A=252, not regularized union measures. Only
(100,20,0) is blue; (0,100,20), its counterpart through the other group use,
remains red. A valid source mutation choosing the other upper NAUO reverses
exactly those appearance assignments, also checked in OCCT.

```sh
sh tools/occtref/build_author_ap242_shuo.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib /tmp/plainsolid-ap242-shuo
PLAINSOLID_OCCT_AP242_SHUO=/tmp/plainsolid-ap242-shuo \
 go test ./exchange/step -run '^TestSHUOOccurrenceColors' -count=1
```

Negative source tests include wrong root/leaf, reversed links, recursive SHUO,
wrong next product, wrong placement, borrowed body, duplicate PDS/SDR ownership.
The source fuzz smoke target is bounded to 64 KiB/1024 entities:
`-run '^$' -fuzz '^FuzzSHUOOccurrenceColors$' -fuzztime=40x -parallel=1`.

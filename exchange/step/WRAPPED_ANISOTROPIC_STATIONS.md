# Exact weighted anisotropic station loft STEP

`ReadWrappedAnisotropicStationLoft`, `WriteWrappedAnisotropicStationLoft`, and
`WrappedAnisotropicStationLoftFromSolid` are separate from the circular and
homothetic station APIs. They admit 1–16 original positive weighted-quadrant
9×2 spline bands with independent positive X/Y station scales, a common center,
strictly ordered height slabs, two planar caps, and shared full weighted spline
rims. Sections are not assumed to be circles or ellipses.

The existing capped and station graph code supplies exact model/pcurve agreement,
two-use shared rim ownership, opposite cap orientation, original seam/vertex
identity and bounded parsing. Each band is independently certified by
`geom.CertifyWrappedAnisotropicEmbedding`; both station scales must agree at a
join. `trimmed.NewWrappedAnisotropicStationLoft` proves complete native material
and continuous shared edges from the retained original supports. Writer input is
those supports and names/units; cached `Solid` is not authority. No rounding,
resampling, CIRCLE replacement, or homothetic-radius substitution occurs.

This follows the existing represented-binary64 STEP spline coordinate and knot
dialect. Additionally, authored rational curve/surface weight decimals must equal
the represented weights exactly, and normalization must preserve all exact ratios.
Near-binary weight decimals such as `0.50000000000000001` fail closed. Unrepresentable
normalized weights or output weights fail rather than becoming new geometry.
General arbitrary NURBS solids, tilted axes, holes, assemblies and native source
provenance envelopes are outside this milestone. Older public readers retain
their prior gates.

## Independent qualification

Pinned OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9` authors the original
fixture without Go geometry input. Build:

```sh
sh tools/occtref/build_anisotropic_station_author.sh OCCT_INCLUDE OCCT_LIB OUTPUT
```

Set `PLAINSOLID_OCCT_ANISOTROPIC_STATION=OUTPUT` for the live STEP test. The tool
accepts `OUTPUT source.step report.json filled` or `OUTPUT input.step report.json
read`. It builds and sews the original weighted NURBS bands and exact planar caps;
ordinary STEP transfer and BRepCheck validate source and Go-generated output.

The retained original has center `(1/4,-1/2)`, heights `0,1,3`, X/Y scales
`(3,1),(1,2),(2,1)`, knot stations `-2,-1,1,4,8`, and nonuniform quarter weights
`1/2,3/4,3/4,3/4`. Source/import/native reload/re-export preserve original controls,
weights and knots. Independent OCCT measures are approximately volume
`22.342478123125012`, area `51.618998781620647`, centroid
`(0.22903978809319786,-0.51812767050040487,1.3953488372247866)`.
Native rational mass enclosures contain the independent volume and centroid;
axial factors independently equal `43/6` and `10`. Generated/reloaded STEP has one
solid and one shell and agrees with independent surface area and centroid.
Tests reject changed pcurve ownership, face orientation, original controls,
source weights, discontinuous stations, and insufficient parser/proof budgets.

## Distinct anisotropic through-bore material

`ReadWrappedAnisotropicStationBore`, `WriteWrappedAnisotropicStationBore`, and
`WrappedAnisotropicStationBoreFromSolid` add a separate through family. Each side
has 1–16 original bands; inner/outer station grids and aspect ratios may differ.
Common center, equal end planes, identical normalized weighted profiles and
strict positive X/Y scale gaps at every merged station are mandatory. The native
`CutWrappedAnisotropicStationLofts` proof establishes complete containment using
the monotone canonical quarter profile and strict diagonal contraction. Touching,
crossing, different profiles, blind bores and sealed cavities are unsupported.

The actual inner walls have reversed material orientation. Both terminal planes
carry true annular cap loops; source model rims and per-owner pcurves are checked
before capped local proof views are assembled. Original rim/seam/vertex IDs are
mapped back from those views. Native validation re-proves the inward walls and
annular caps, and writer output is fully staged before any bytes are emitted.
The filled and prior homothetic/circular entrypoints keep their domains.

Independent `anisotropic_station_author` mode `bore` performs a genuine OCCT Cut
using the filled stock above and inner stations at heights `0,2,3` with scales
`(1/2,1/4),(1/4,1/2),(3/8,1/4)` and the same nonuniform weighted profile. The retained
original has 418 records. OCCT measures volume `21.140920405972565`, area
`57.889397470106566`, centroid approximately
`(0.22809463656249926,-0.51889721655644283,1.3928571428060916)`.
Exact native source bounds enclose the volume; the difference axial factor is
`217/32` and Z centroid is exactly `39/28`. Source→native reload→generated STEP
preserves both original wall families, one closed shell, two cap holes and the
independent volume/area/centroid. Mutations cover owner, inward orientation,
nonrepresentable authored weights, touching walls, limits and proof budgets.

## Flat original-product occurrence exchange

The separate `WrappedAnisotropicStationAssembly` and
`WrappedAnisotropicStationBoreAssembly` read/write/source APIs accept one original
local material product and one geometry-free root with 2–16 shared flat uses.
Each use must be a proper signed-coordinate rigid placement with a neutral child
frame. Product, original body/face IDs, all original wall/rim/vertex roles and
NAUO/CDSR/relationship/placement IDs remain local and distinct from the occurrence
matrices. Every original support is re-proved once per shared local product,
not reconstructed from displayed instances. No regularized assembly union is
claimed. The filled and bore paths cross-reject each other's material graphs.

This mode uses uniform metre/millimetre source units and the established strict
flat ownership/closure proof. Unknown geometry, orphan entities, pcurve ownership
changes, cycles, improper or noncardinal frames, mixed units, nested groups and
appearance records are unsupported. The returned local native asset can be saved
and reloaded with its exact original geometry. The separate JSON APIs below
persist and re-prove the full assembly. Original source export re-proves
the complete original graph before retaining its entity IDs. Canonical output
stages the explicit local source supports through the existing assembly writer
and validates the entire resulting graph before output.

Independent OCCT XCAF `filled-assembly` and `bore-assembly` modes author AP242
sources (258 and 450 records respectively) with identity and
`(x,y,z)->(z+10,x-2,y+3)` occurrences. `filled-assembly-negative` additionally
qualifies the proper negative-axis rotation `(-x+10,y-2,-z+3)`.
`read-assembly` uses XCAF to require that both components refer to the same local
product. Live original-source preservation and native-local-reload→new STEP
output retain that sharing and agree on aggregate volume, area and world
centroid. Local source data is not flattened into world control points. Focused
negative tests include source owner/sense-class changes, unrepresentable weights,
parser limits, cancellation, unit mismatch and reflection rejection; a bounded
two-family fuzz target uses the actual strict readers.

## Source-bound flat assembly native formats

`Write/ReadWrappedAnisotropicStationAssemblyJSON` use version-one tag
`plainsolid-step-anisotropic-station-assembly`;
`Write/ReadWrappedAnisotropicStationBoreAssemblyJSON` use the distinct tag
`plainsolid-step-anisotropic-station-bore-assembly`. Older homothetic, standalone
and other native tags are unchanged. Each envelope stores one original STEP
source, one local native solid, original product/face IDs, original weighted
support data and edge/vertex identities, and all original assembly and exact
occurrence relationship/matrix claims. Bore mode stores both original wall
families and its actual annular material, not two unrelated positive solids.

Reload invokes the corresponding strict original STEP reader once and compares
all supplied claims with the regenerated result. The serialized native solid,
public IDs and poses are never replacement authorities. Writers similarly
reject stale in-memory claims before output; changing cached geometry or a
matrix does not request an edit to the authenticated source. Author and reimport
changed STEP to establish a new accepted model. Bounds are 16 MiB native / 1 MiB
source, existing bounded graph and geometry rules, caller source parse limits,
and cancellation checks. Failure returns no partial source, material or graph.

Corruption tests cover source, tag/version, original body/face/product/use IDs,
relationship matrices, weighted supports, native geometry, cross-family payloads,
limits, cancellation before and during reads, short writes and stale edits.
Live filled/bore and negative-axis XCAF tests now traverse the full envelope
before original-source re-export and native-local regeneration into new STEP.
Both routes retain one shared local product and independent world measures.

## One shared intermediate assembly

The separate `Read/WriteNestedWrappedAnisotropicStationAssembly` and bore
counterparts admit exactly one geometry-free root, one shared intermediate
product and one original local material product. The root has two to sixteen
uses of the intermediate product, which has exactly one use of the leaf.
Each expanded instance retains its original two NAUO IDs and leaf product ID.
Composed transforms are computed exactly and must remain losslessly represented
proper signed-coordinate placements. Unrepresentable sums reject before output.
The previous flat readers and native JSON tags remain strict; nested persistence uses the separate tags described below.

Original-source writers re-prove the full graph. Generated STEP preserves the
three products and original local supports rather than baking world geometry.
The independent OCCT XCAF `filled-nested-assembly` and
`bore-nested-assembly` fixtures contain 281 and 473 records. Their subgroup has
a nonzero local translation and is reused under two distinct root placements.
`read-nested-assembly` independently verifies shared subgroup and leaf identity;
source re-export and native-local reload followed by generated output agree on
volume, area and centroid. Tests reject cycles, incorrect pcurve ownership,
parser limits, cancellation and nonrepresentable composed placements. Arbitrary
depth, multiple local products, contextual styles and mixed units remain outside
this explicitly bounded grammar.

### Nested source-bound native formats

The separate `Write/ReadNestedWrappedAnisotropicStationAssemblyJSON` and bore
counterparts now persist the complete nested result. Their version-one tags are
`plainsolid-step-nested-anisotropic-station-assembly` and
`plainsolid-step-nested-anisotropic-station-bore-assembly`. The preceding flat
tags retain their original meaning and reject these payloads.

Reload re-proves the complete original STEP once, including local original
weighted supports, topology and each relationship, then compares the stored
native solid, all original role IDs, intermediate product ID and expanded
source paths/composed matrices against that proof. Cached geometry and instance
edits are not instructions to alter the source; stale claims fail atomically.
The existing 16 MiB envelope / 1 MiB source limits and source parse quotas apply.
Unknown fields, trailing data, wrong tags, corrupted ancestry/supports/material,
limits and cancellation are tested. Live filled and bore XCAF chains pass
through the full envelope before source and regenerated STEP export, preserving
shared subgroup/leaf identity and independent world measures.

## Exact rational flat filled occurrences

`ReadRationalWrappedAnisotropicStationAssembly` is a separate filled-material
route for proper rational orthonormal placement frames whose normalized entries
have finite decimal expansions and at most 256 numerator/denominator bits. It
retains the common metre/mm, neutral child-frame, one-local-product and two to
sixteen-use restrictions. It does not widen the signed-coordinate APIs, admit
mixed units or nonneutral source mapping origins, or accept approximate frames.

The original local material is independently proved through the same original
NURBS graph. A private neutral-placement view permits reuse of the flat graph
ownership proof only after proving every replaced frame dependency disjoint
from all material dependencies. All original placement IDs and exact normalized
direction ratios are then restored in immutable `ExactNAUOOccurrence` values.
`Matrix` and `ApplyExact` return owned rational values; `ApplyRepresented` rejects
world coordinates that binary64 cannot represent exactly. There is no rounded
public affine matrix or world geometry baking.

`WriteRationalWrappedAnisotropicStationAssemblySource` re-proves and emits the
original standard STEP graph; it never exports the private neutral view. The
separate version-one native tag
`plainsolid-step-rational-anisotropic-station-assembly` stores original source,
local material/support claims, product/root/face identities and rational
relationship matrices. Its `Write/ReadRationalWrappedAnisotropicStationAssemblyJSON`
APIs re-prove the original source and reject stale or forged claims, with the
same 16 MiB native / 1 MiB source ceilings. This is source-bound persistence,
not an arbitrary pose editing or new assembly authoring API.

The genuine pinned XCAF `filled-assembly-oblique` mode authors AP242 with an
identity use and the frame `X=(0.6,0.8,0)`, `Z=(0,0,1)` at `(10,-2,3)`.
The original 258-record source admits exact ratios 3/5 and 4/5. Live source →
native envelope → original-source STEP retains shared product identity, volume,
area and centroid. Tests distinguish exact rational evaluation from refused
binary64 conversion, mutate nonorthogonal and rounded direction ratios, reject
material/frame dependency sharing and nonneutral child frames, and check
source limits, cancellation, tags, support/native/provenance corruption and
immutable matrix copies. The through-bore rational route uses the separate API below.

### Rational flat through-bore occurrences

`ReadRationalWrappedAnisotropicStationBoreAssembly` separately admits the
already-qualified anisotropic through-bore material under the same exact
rational flat occurrence rules. The filled route still rejects bore material;
the signed-coordinate bore reader still rejects the oblique frame. The local
proof preserves original inward/outward bands, shared station rims, original
cap and vertex identities, and the actual annular closed material.

The corresponding `Source` writer re-proves the original standard graph.
`Write/ReadRationalWrappedAnisotropicStationBoreAssemblyJSON` use distinct
version-one tag `plainsolid-step-rational-anisotropic-station-bore-assembly`.
They retain both original weighted support families, top-level material names
and units, original source roles and exact occurrence matrices, and compare
all supplied claims with one complete original-source reproof. Neither stored
native material nor a pose edit overrides that proof. Bounds and atomic
failure follow the filled rational format.

The independent XCAF `bore-assembly-oblique` mode constructs the original
material with OCCT's cut operation, then creates shared identity and exact
3:4:5-frame occurrences. The retained AP242 source has 450 records. Original
and native-envelope re-export independently retain two solids, two shells and
shared local identity; their aggregate volume, area and centroid agree. Native
validation additionally checks two inward and two outward bands, exact shared
station rims, two annular cap wires and Euler characteristic zero. Negative
checks cover missing source faces, wrong coordinates/frames, altered inner rim
IDs and weights, forged source roles/poses, limits, cancellation and cross-mode
rejection. No arbitrary rational frame normalization, nested rational graph,
mixed units or approximate circular substitution is admitted.

### Generated AP242 rational occurrences

`WriteRationalWrappedAnisotropicStationAssembly` and the separate bore writer
accept two explicit product definitions (geometry-free root and one original
local material) plus two to sixteen `RationalAnisotropicAssemblyInstance` uses.
Each use supplies its identifier, name, parent/child identifiers, rational
origin and X/Z direction ratios. Inputs are copied. All raw and normalized
components must have terminating decimals and at most 256 numerator/denominator
bits. Exact perpendicularity and rational normalization are mandatory; the
right-handed Y direction is derived exactly. Thus raw `(3,4,0)` is admissible,
while `(1,1,0)` has irrational normalization and `(5,12,0)` has nonterminating
normalized coefficients and both reject. A translation of 1/3 also rejects.

The writer first stages the existing original-support material and standard
shared-product graph at neutral placements. It then writes each verified pose
into standard `AXIS2_PLACEMENT_3D` / `ITEM_DEFINED_TRANSFORMATION` occurrence
records and emits the explicit AP242 protocol/application/schema declaration.
The entire generated file is read by the corresponding strict rational reader,
and every resulting matrix is compared exactly with the caller's copied pose,
before output. Local geometry is shared, not baked; this is a bounded AP242
authoring subset, not arbitrary schema support. The original-source APIs and
existing signed writers remain unchanged. Generated files can use the existing
rational native envelopes for full source-bound persistence.

The pinned independent XCAF reader now reports actual component matrices as
well as shared identity and material measures. Tests qualify both filled and
through-bore generated output at the original 3:4:5 poses and at a new caller
translation, checking actual matrices, two solids/two shells, shared identity,
area, volume and the predicted centroid shift. Global OCCT spline quadrature
shows approximately 2.1e-7 volume variation under the tested rigid translation;
the comparison uses a 1e-6 absolute volume allowance (1e-7 area/centroid), while
all source/pose/material acceptance remains exact. Negative tests cover nil,
over-budget, nonterminating, irrational or nonorthogonal ratios, duplicate or
foreign occurrence IDs, cancellation and atomic pre-output failure. Canonical
bytes are stable under equivalent raw direction scales, and caller rationals
remain unchanged.

## Nested exact rational source assemblies

The separate `ReadNestedRationalWrappedAnisotropicStationAssembly` and bore
counterpart admit exactly the established three-product grammar: one root,
one reused geometry-free subgroup, one original local material, two to sixteen
root uses and one subgroup-to-leaf use. Both levels may have proper rational
orthonormal frames under the same common-unit and neutral-source-frame rules.
Original relationship/frame IDs remain available separately from expanded
`RationalHierarchyInstance` paths and composed poses. Every composition is
exact, with the same 256-bit and terminating-decimal gate, including the shared
64-character decimal serializer ceiling. No world control points are rounded.

Their `Source` writers re-prove and preserve the actual standard original graph.
The `JSON` APIs use distinct version-one tags
`plainsolid-step-nested-rational-anisotropic-station-assembly` and
`plainsolid-step-nested-rational-anisotropic-station-bore-assembly`. They retain
all original local material/support claims, subgroup ID, original relationships,
expanded two-use paths and exact composed matrices. Reload re-proves the whole
original source once and compares every stored claim; native bytes and stored
poses are not authorities. The existing flat rational and signed nested APIs
and tags remain strict. Caller-generated nested rational authoring is not part
of this source-backed milestone.

Independent XCAF `filled-nested-oblique` and `bore-nested-oblique` modes author
281- and 473-record AP242 sources with an oblique subgroup use and one oblique
root use. Both use exact original decimal ratios `(0.6,0.8,0)`; composition
produces `(-7/25,24/25,0)` exactly. The pinned reader reports actual leaf poses,
shared subgroup/leaf identity, topology and material measures. Tests compare
those poses and original/native/source-re-export measures, preserve local native
material proof, reject cyclic ownership and rounded frames, and test a composed
translation exceeding the decimal quota although its inputs each fit. Native
corruption tests cover subgroup/path/matrix/source/support/material claims,
limits, cancellation, cross-mode tags and atomic failure. Both through-bore
walls and the annular cap/rim topology remain independently re-proved.

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

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

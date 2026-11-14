# Registered analytic material dispatch

`ReadAnalyticMaterial(ctx, reader, limits, tolerance)` selects one of seven
existing, independently qualified strict Type186 material codecs:

| Family | Shells | Faces | Analytic supports |
|---|---:|---:|---|
| Sphere | 1 | 1 | one196 |
| Ring torus | 1 | 1 | one198 |
| Expanding 45° frustum | 1 | 3 | one194, two190 |
| Concentric sphere cavity | 2 | 2 | two196 |
| Concentric 45° frustum cavity | 2 | 6 | two194, four190 |
| Cylinder stock/sphere cavity | 2 | 4 | one192, one196, two190 |
| Cone stock/cylinder aperture | 1 | 4 | one194, one192, two190 |

A bounded directory inventory chooses a **candidate**, never accepts geometry.
Each signature requires exactly one Type186 root. The selected reindexed codec
then authenticates every original dependency, numeric value, directed loop,
shell orientation, transform and constructive material condition. Failure is
terminal; no other codec is tried. Unknown signatures, multiple roots, mixed
families and invalid selected graphs return a zero result. This is not a general
IGES importer and does not replace existing collection or individual APIs.

The inventory bounds source bytes to 32768, records to 512, entities to 40 and
values to 2048, intersected with caller limits. It performs no parameter geometry
conversion; the existing typed proof retains its limits, exact source gates,
proper signed-coordinate placement and original tolerance semantics. Sphere
native angular evaluation retains its previously documented bounded discrepancy;
dispatch does not strengthen that claim or infer rational sphere sewing.

`AnalyticMaterial` stores its selected payload privately, with no writable tag.
`Family()` identifies it; `Solid()` returns its native material and `LengthUnit()`
returns meters per model unit without rescaling. `SourceRoles()`
and `NativeFaceSourceIDs()` return caller-owned copies of source ancestry.
`WriteAnalyticMaterialSource` reauthenticates the original typed result and emits
its unchanged source bytes, including units, metadata and all original source
ordinals. It does not author a new shape or repair an external file. Existing
typed APIs remain available for detailed variant-specific inspection/persistence.
The zero result is invalid; even package-internal ambiguous payloads fail export.

## Qualification

Tests select all seven families in canonical, placed, reordered and reordered
placed forms (28 retained independently authored files), check positive native
mass and source identity, and verify copied ancestry cannot modify retained
state. Same-signature broken root ownership, mixed support signatures, duplicate
root declarations, parser limits, cancellation and empty/ambiguous results fail
atomically. Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 loads source
re-exports for all seven reordered placed families as valid material solids and
agrees with native area/volume. Use `PLAINSOLID_OCCT_IGES_SOLID` with the existing
`iges_topology_solid.cxx` comparator. Focused normal/live/race, Go1.23, vet and
100 bounded fuzz executions pass.

# Standalone exact cylinder supports

`ReadCylinderSupports` and `WriteCylinderSupports` exchange one generic
`REPRESENTATION` containing a `GEOMETRIC_SET` of up to 64 unbounded
`CYLINDRICAL_SURFACE` supports. This intentionally emits no bounded-surface
representation, face, product, shell or solid. A support is not a finite cylinder.

Each support has a uniquely owned explicit `AXIS2_PLACEMENT_3D`, location and
axis/reference directions. Only perpendicular signed coordinate unit directions
are supported; arbitrary normalized or projected frames are rejected. Original
surface/placement/datum names and representation/set names are retained. Units
are explicit meters per coordinate unit; coordinates are not rescaled. U is
radians about the source axis from the reference ray and V is signed distance
along the axis. `Geometry` returns the existing native analytic cylinder.

The reader requires exact decimal-to-binary geometry scalar conversion. The
writer stages a full strict read-back before caller I/O. Source frames are not
snapped. Shared dependencies, unused entities, implicit placement directions,
finite trims, assemblies, product graphs and unsupported contexts fail. Parsing
is bounded by ordinary STEP limits plus 512 total entities and 64 supports.
Explicit unit conversion contexts use the existing strict unit checker.

Preflight provides a declaration-level `ReadCylinderSupports` hint only; it
makes no geometry or ownership acceptance claim. Generic solid readers remain
strict. The independently authored retained OCCT fixture and optional
`PLAINSOLID_OCCT_CYLINDER_SUPPORT` helper qualify both axis signs with a
nondefault radial reference by comparing analytic positions and derivatives.
These tests use entity-to-surface transfer, not shape transfer or solid validity.

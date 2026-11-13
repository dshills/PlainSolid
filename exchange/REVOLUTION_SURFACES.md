# Native standalone revolution supports

`WriteRevolutionSurfacesJSON` and `ReadRevolutionSurfacesJSON` persist a
`step.RevolutionSurfaceModel` independently of mesh, B-rep and document schemas.
Version 1 has the distinct kind `standalone-analytic-revolution-surfaces` and
explicit `AngularUnit: "radian"`. `LengthUnit` is meters per model coordinate;
no coordinates are rescaled. Each ordered surface stores its name, origin,
signed-coordinate axis, angular limits, and original meridian profile name,
degree, poles, knots, normalized weights and logical metadata. Surface U remains
radians and V remains the original spline parameter. Empty logical fields remain
empty in native storage (the STEP writer separately emits its documented unknown
logical value).

The codec reconstructs every analytic support through
`geom.NewRevolutionSurface`. The polynomial, positive radial half-plane, strictly
monotone axial height, continuous clamped knot and conservative partial-angle
requirements therefore remain in force, including exact represented oblique
meridian phase. It creates no trimmed faces, shell, material, tensor substitute,
mesh or document asset. No full-turn topology or generic surface persistence is
implied. Represented float64 coefficients survive native reload exactly; native
JSON is not a claim that an external decimal represented those same exact reals.

The writer validates and stages the complete model before writing. The reader
returns an empty model on any error, including a later invalid support. Byte,
surface and cumulative-pole limits apply. Defaults are 4 MiB, 64 supports and
4096 poles, with hard maximums 64 MiB, 64 and 4096 respectively. Names are bounded
to 4096 UTF-8 bytes. The accepted native representation is canonical writer JSON
with optional insignificant whitespace; unknown/duplicate fields, changed field
order, trailing objects, noncanonical numeric spelling, other format kinds and
versions are rejected. Valid geometric edits are allowed: this is validation,
not authentication or a cryptographic tamper seal.

Focused tests cover multiple named supports, foot units, negative axes and
angular ranges, immutable evaluation snapshots, source-to-native-to-STEP exact
parameter evaluation, invalid geometry, metadata, limits and atomic failure.
An optional live OCCT test independently authors the oblique meridian revolution,
passes it through native storage and STEP export, and compares valid face count,
area, positions and partial derivatives. Set
`PLAINSOLID_OCCT_REVOLUTION_SURFACE` to the independently built revolution helper
with its `write-oblique` mode to run that test.

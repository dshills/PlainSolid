# Native standalone revolution supports

Package `plainsolid/exchange/revolution` provides
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

## Explicit rational version 2

`WriteRationalRevolutionSurfacesJSON` writes version 2 of the same standalone
kind. `ReadRationalRevolutionSurfacesJSON` accepts versions 1 and 2; version 1
still requires polynomial profiles, even through this new reader. The original
reader/writer remain version-1-only. Other versions are rejected.

Version 2 additionally accepts the bounded positive-weight rational profiles
proved by `geom.NewRevolutionSurface`: degree at most 16, at most 256 poles and
64 nonzero spans, exact per-span strict Bernstein sign of axial `H'W-HW'`,
600000 checked operations and 8192-bit coefficients per support. All previous
radial, knot, axis, angular and metadata requirements remain. Normalized weights
are persisted losslessly; no rational profile is converted to polynomial or to
a tensor representation. The same complete-read/complete-validation guarantee,
canonical representation and outer resource limits apply. Independent OCCT tests
now also exercise the rational source → native v2 → STEP export chain.

## Strict IGES/STEP support bridge

`FromIGES(iges.Model)` and `ToIGES(step.RevolutionSurfaceModel)` connect the
standalone type-120 codec with native v2 and the bounded STEP revolution codec.
They preserve the source geometry, source radian/profile parameters and units;
the OCCT IGES face-parameter convention remains separate as documented by that
codec. These operations remain in this leaf package to avoid a dependency cycle
between root mesh exchange and STEP modeling adapters.

The bridge does not silently drop metadata. IGES dependencies must occur as
axis/profile pairs in root order. Axis labels must be empty; all subscripts and
directory forms must be zero. Root/profile labels map exactly to surface/profile
names. The planar flag must be true, polynomial flag must match the represented
weights, and normal must equal the deterministic radial-cross-positive-coordinate-
axis unit normal. Its normalization must be rational and float64-exact; an
otherwise valid oblique meridian with an unrepresentable unit normal is rejected.
These fields can therefore be regenerated exactly without expanding native v2.

From IGES, the already-proven continuous strict monotonicity of generatrix axial
height establishes distinct endpoints and no self-intersections, so STEP curve
flags are explicitly F. To IGES, both STEP flags must already be F: unknown or
empty values are rejected rather than silently changed. Names must fit eight
printable ASCII bytes with no boundary spaces (IGES directory parsing trims them). Axis-line endpoint addition, scalar emission and standard
IGES units must also survive the strict IGES writer/read-back gate. Other valid
STEP models remain persistable natively but can be outside this bridge subset.

Unsupported conversions return `*UnsupportedConversionError` with `Field` and
`Reason`; `errors.Is(err, ErrUnsupportedConversion)` identifies the class. Failed
conversions return empty models. Valid results own independent profile geometry.
Changing unsupported source metadata is the caller's explicit decision; the
bridge never performs implicit truncation, normalization or flag replacement.

Independent tests exercise IGES → native v2 → STEP and STEP → native v2 → IGES
with live OCCT comparisons of face validity, area, positions and source-mapped
partials. Set both `PLAINSOLID_OCCT_IGES_REVOLUTION` and
`PLAINSOLID_OCCT_REVOLUTION_SURFACE` to the standalone helpers. A long OCCT-authored
STEP label is first rejected, then explicitly renamed by the test before the
reverse conversion; no lossless preservation claim is made for that caller edit.

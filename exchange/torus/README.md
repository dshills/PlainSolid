# Exact ring-torus support conversion

`FromIGES` / `ToIGES` bridge strict IGES198 form1 and STEP toroidal supports.
Center, both radii, exact signed-coordinate frame, names and explicit units
survive unchanged. Both major and tube parameters remain periodic radians;
no seam topology, material normal, finite trim or solid is inferred.

Only regular ring tori qualify. IGES subscripts must be zero; STEP representation,
set and placement names must be empty. Transferable labels must fit eight
printable ASCII bytes without boundary spaces. Incompatible metadata or exact
representation failures return an empty model with `UnsupportedConversionError`
(field/reason), matching `ErrUnsupportedConversion` via `errors.Is`.

Both strict codecs validate staged source and destination geometry. No frame
normalization, coordinate rescaling or persistent schema is introduced. Surface
slices are owned; geometry fields are values. Tests cover foot units through
both formats, signed axes, rejected metadata, retained independent fixtures and
optional live `PLAINSOLID_OCCT_IGES_TORUS` / `PLAINSOLID_OCCT_TORUS_SUPPORT`
comparisons of analytic positions and derivatives in both directions.

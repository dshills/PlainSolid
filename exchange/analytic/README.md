# Mixed standalone analytic support conversion

`FromIGES` and `ToIGES` convert the strict mixed support models from
`exchange/iges` and `exchange/step`. They preserve ordered planes, cylinders,
spheres and ring tori, exact represented frames and dimensions, and model units.
These are standalone support surfaces, not trimmed faces, assemblies or solids.
IGES type 194 cones are excluded.

The bridge delegates each support to the existing plane, cylinder, sphere or
torus converter and validates both complete output files. It accepts 1–64
independently owned supports. Results own their descriptor storage; a failure
returns an empty model, never a partially converted list.

Lossless metadata gates remain explicit: IGES subscripts must be zero; labels
must fit the shared printable eight-character domain; STEP representation,
geometric-set and placement names must be empty. Geometry and unit restrictions
of each strict codec still apply. General file headers, presentation attributes,
product identity and source entity numbers are outside these model contracts.
No implicit unit coercion or frame normalization occurs.

`errors.Is(err, ErrUnsupportedConversion)` classifies conversion failures.
`errors.As` to `*ConversionError` identifies the zero-based support index, or
`-1` for global metadata, count or complete-file validation. The underlying
specialized typed diagnostic remains available through error unwrapping.

Tests include retained independently authored OCCT files, foot-unit round trips,
metadata and ambiguous-variant rejection, and optional bidirectional OCCT
geometry/first-derivative checks for both axis signs. Set
`PLAINSOLID_OCCT_IGES_ANALYTIC_SUPPORTS` and
`PLAINSOLID_OCCT_ANALYTIC_SUPPORTS` to the dedicated helper executables built
from `tools/occtref/iges_analytic_supports.cxx` and `analytic_supports.cxx`.

# Exact standalone cylinder conversion

`FromIGES` and `ToIGES` bridge the strict IGES192 form1 and STEP cylindrical
support codecs. They preserve explicit length units, coordinates, radius,
signed coordinate axis/reference frame, and surface/location/direction names.
No normalization or unit rescaling occurs. The result remains an unbounded
analytic support, with no finite trim, face, shell, or solid claim.

Only metadata that survives both formats is accepted. IGES subscripts must be
zero. STEP representation, geometric-set and placement names must be empty.
All transferable names must fit eight printable ASCII bytes without boundary
spaces. Failures return an empty model and `UnsupportedConversionError`, with
`Field` and `Reason`; `errors.Is(err, ErrUnsupportedConversion)` is supported.
Both strict writers validate the staged source and destination representations,
including exact scalar/frame restrictions. The result owns its surface slice
and all geometry fields are values. No native persistence schema is added.

Tests pass explicit foot units through both strict codecs, retain negative and
positive axis directions, reject nonrepresentable metadata and use independent
OCCT source fixtures. Optional live tests set both
`PLAINSOLID_OCCT_IGES_CYLINDER` and `PLAINSOLID_OCCT_CYLINDER_SUPPORT` and compare
source/converted analytic points and derivatives in each direction. These are
support-level comparisons, not bounded-shape transfer tests.

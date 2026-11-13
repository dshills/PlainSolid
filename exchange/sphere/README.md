# Exact spherical support conversion

`FromIGES` and `ToIGES` bridge strict IGES196 form1 and STEP spherical support
models. They preserve center, positive radius, signed coordinate pole axis,
reference meridian, raw coordinates, explicit units and transferable names.
Longitude remains periodic around the authored axis; latitude keeps its natural
[-π/2,π/2] domain and pole singularities. No seam topology, material normal or
closed solid is inferred. No finite trimming parameters are invented.

IGES center metadata maps to STEP placement-location metadata. IGES subscripts
must be zero. STEP representation, set and placement names must be empty.
Transferable names must fit eight printable ASCII bytes without boundary spaces.
Unsupported metadata or exact representation failures return an empty model
and a typed `UnsupportedConversionError` with field/reason, matching
`ErrUnsupportedConversion` through `errors.Is`.

Both strict codecs validate staged source and output geometry; no normalization,
projection or unit rescaling is used. Results own their surface slices and all
geometry fields are values. There is no added native persistence schema.
Tests cover foot units through both formats, both axis signs, source metadata
rejection and independently authored fixtures. Optional live tests use
`PLAINSOLID_OCCT_IGES_SPHERE` and `PLAINSOLID_OCCT_SPHERE_SUPPORT` to compare
source/converted analytic positions and partials in both directions. This is
support-level interoperability, not trimmed-face or solid transfer.

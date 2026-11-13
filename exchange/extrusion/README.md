# Exact standalone extrusion conversion

This leaf package converts the supported IGES type 122 tabulated surface model
and STEP rational linear extrusion model. It adds no persistence format and
makes no trimmed-face, shell, or solid claim.

`FromIGES` and `ToIGES` return complete models or an error. Both pass their
staged output through the respective strict codec. Names and explicit length
units are preserved without rescaling coordinates. Native standalone surface
persistence remains in `exchange/revolution` and covers revolution surfaces only.

IGES evaluates `C(u) + v D` with `v` in `[0,1]`. STEP evaluates
`C(u) + t (D/|D|)` with `t` in `[0,|D|]`. Conversion preserves the original
profile parameter and uses `t = v |D|`; the V derivatives therefore have
different units. Negative displacement is retained. A nonzero STEP V start is
unsupported. The distance must be rational and exactly representable as
float64; STEP direction ratios must already be exactly unit length and satisfy
the existing finite-decimal codec limits. No direction is snapped.

Lossless metadata gates require short printable IGES-compatible names, zero
subscripts, canonical dependency order and profile form, and deterministically
recoverable plane/polynomial hints. STEP profile closed/self-intersection flags
must be explicit false. IGES conversion proves global injectivity of the entire
represented extrusion before assigning these flags; a source hint alone is
insufficient. The proof tries at most three coordinate projections with fixed
cell, depth, arithmetic-operation and coefficient-bit budgets. Failure to prove
injectivity is `ErrInconclusiveProof`, distinct from `ErrUnsupportedConversion`.
The latter exposes a field and reason through `UnsupportedConversionError`.
These conservative restrictions reject some valid surfaces.

Tests cover exact rational point and derivative correspondence, explicit foot
units through both strict codecs, signed displacement, metadata rejection and
inconclusive geometry. The retained STEP fixture was independently authored by
OCCT. Optional bidirectional live tests use
`PLAINSOLID_OCCT_IGES_TABULATED` and `PLAINSOLID_OCCT_EXTRUSION_BRIDGE`, built from
the dedicated helpers in `tools/occtref`. They compare independent geometry and
area while respecting the fraction-versus-distance parameter conversion.

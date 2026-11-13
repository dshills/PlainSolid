# Exact rooted cylinder asset persistence

This leaf package bridges one canonical native cylinder and the explicit rooted
half/quarter STEP cell dialects. It does not alter generic STEP import, native
BRep persistence or PlainCAD integration.

`FromSTEP(reader, Half|Quarter)` retains original bounded source bytes and the
proved neutral descriptor: part name, exact signed-coordinate pose, radius,
height and typed `SIUnit`. `MetresExponent` denotes exactly 10^exponent metres per
model unit; zero means metre. Only the supported SI prefix exponents are admitted.
The float scale used by existing STEP APIs is never presented as an exact rational
unit value. Coordinates are not numerically rescaled.

`FromNative(solid,name,unit,dialect)` uses the existing full canonical-cylinder
classifier. The result has no original source evidence. `Asset.Validate` returns
native material plus optional original `Provenance`; source face/edge IDs are
newly owned slices, not native chart indices. Source has four/six faces, while
native material has the canonical three charts.

`WriteNative`/`ReadNative` use format `plainsolid-cylinder-cells`, version1. The
neutral descriptor and optional original source are validated together on every
reload, geometry request and export. Stale name, unit, pose, dimensions or dialect
fail atomically. Original bytes remain exact, including alternate orientation
flags; `WriteSTEP` emits canonical rooted geometry, so subsequent STEP output may
normalize those flags. This evidence proves geometry/metadata binding, not a
cryptographic assertion of authorship.

Limits are 1MiB original STEP, 2MiB native JSON, eight JSON nesting levels and the
existing bounded topology codec limits. Unknown, duplicate, noncanonical-case
and Unicode case-alias fields, nulls, trailing data and unsupported versions are
rejected. The source unit audit retains strict rejection of vendor prefixes,
wrong angular dimensions and near-integer context dimensions. No unsupported
metadata is silently widened. A valid modified geometry with explicitly removed
source evidence is a new native-only asset, not a claim to the old provenance.

Tests exercise both STEP→native→reload→STEP chains, source/native role distinction,
malformed and forged valid JSON, stale evidence and exact unit tags. The optional
`PLAINSOLID_OCCT_SPLIT_CYLINDER` helper validates conventional TransferRoots,
face count, volume, area and centroid for both re-exported dialects.

## Original versus canonical STEP output

`WriteOriginalSTEP` requires retained, currently valid rooted source evidence.
It reproduces those bytes exactly: header, whitespace, entity IDs and equivalent
orientation flags. Missing source returns `ErrNoSource`; stale descriptor/source
binding fails without writing. It never invents a source file or implicitly
roots an unsupported bare model. The existing asset schema accepts only the
explicit rooted half/quarter dialects.

`WriteCanonicalSTEP` re-proves the asset and generates a canonical new rooted
product. `WriteSTEP` remains its compatibility entry point, with unchanged
behavior. Canonical output may normalize flags and IDs. Tests demonstrate the
distinction using reversed bound encodings, preserve original bytes through
native persistence, and independently verify both outputs with OCCT TransferRoots.
Short writes and ordinary I/O failures are reported by either output mode.

## Exact SI rescaling

`RescaleSI(asset,targetUnit)` multiplies origin coordinates, radius and height by
exact rational 10^(oldExponent-newExponent). Reference and axis directions are
unchanged. All five scalar results must be exactly binary64-representable before
either input or result geometry is constructed; otherwise
`errors.Is(err, ErrUnrepresentableScale)` identifies rounding/overflow/underflow
rejection and the returned asset is empty. Unsupported SI exponents also fail.
The converted geometry must still satisfy the strict exact STEP numeric domain.

The name and dialect are preserved. Identical unit requests validate and retain
original evidence. A changed unit produces a new native-only asset with
SourceSTEP empty, even when the conversion is exact. The old asset is untouched
and can still re-emit its original source separately; no false source IDs are
invented for converted geometry.

Tests compare exact rational physical coordinates/dimensions, round-trip
metre↔millimetre conversions, persist both dialects, and prove scalar-failure
ordering using deliberately invalid retained evidence. Independent OCCT root
loads of both unit encodings yield matching physical volume, area and centroid.

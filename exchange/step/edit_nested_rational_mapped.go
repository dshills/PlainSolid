package step

import "io"

// WriteNestedRationalMappedOuterPlacements edits every outer group use exactly
// once. It preserves inner uses, shared local geometry, contexts, names and units.
// Each edited use receives a new private operator. Its original item/map IDs stay
// stable; returned source ancestry names the new target ID. The input source is
// never mutated and may be written separately as the original evidence.
func WriteNestedRationalMappedOuterPlacements(w io.Writer, source *File, placements []RationalMappedPlacement, l Limits) error {
	return writeRationalMappedPlacements(w, source, placements, l, mappedNestedEdit)
}

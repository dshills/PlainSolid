package step

import "io"

// WriteDAGRationalMappedRootPlacements edits a nonempty selected subset of root
// uses. An item also referenced inside a group is rejected here: its wider effect
// requires the explicit shared API. Unselected item/operator records are retained.
func WriteDAGRationalMappedRootPlacements(w io.Writer, source *File, placements []RationalMappedPlacement, l Limits) error {
	return writeRationalMappedPlacements(w, source, placements, l, mappedDAGRootEdit)
}

// WriteDAGRationalMappedSharedPlacements explicitly edits selected internal use
// definitions. Every expanded path containing that item receives the new pose,
// including a root path if the same item is used at both levels. This is not an
// occurrence-specific copy: local geometry, membership and map identity remain.
// Complete exact graph proof is repeated before publication; source is immutable.
func WriteDAGRationalMappedSharedPlacements(w io.Writer, source *File, placements []RationalMappedPlacement, l Limits) error {
	return writeRationalMappedPlacements(w, source, placements, l, mappedDAGSharedEdit)
}

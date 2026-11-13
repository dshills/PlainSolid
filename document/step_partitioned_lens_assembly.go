package document

import (
	"io"
	"plainsolid/exchange/step"
)

// ReadPartitionedSplineLensAssemblySTEP imports shared local lens assets with
// per-product rational source seam ancestry and explicit rigid occurrences.
func ReadPartitionedSplineLensAssemblySTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	return readSplineLensAssemblySTEP(r, l, o, limits, historyLimit, true)
}

// WritePartitionedSplineLensAssemblySTEP checks shared product consistency,
// geometry/provenance bindings and exact seam lineage before staging output.
// Proper signed-axis rotations and finite translations remain occurrence data.
func (s State) WritePartitionedSplineLensAssemblySTEP(w io.Writer) error {
	return s.writeSplineLensAssemblySTEP(w, true)
}

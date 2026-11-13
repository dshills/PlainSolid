package document

import (
	"bytes"
	"io"
	"plainsolid/exchange/iges"
)

// ReadAffinePartitionedSplineLensIGES preserves the original affine conjugate
// basis, exact seam intervals, owner knot cells and raw source curve bases in a
// bounded native recipe. Source IDs remain distinct from canonical native IDs.
func ReadAffinePartitionedSplineLensIGES(r io.Reader, l iges.Limits, historyLimit int) (*Document, error) {
	return readPartitionedLensDocument(r, l, historyLimit, true)
}

// FromAffinePartitionedSplineLensIGES reauthors and verifies the supplied graph;
// retained provenance identifies that newly authored source graph.
func FromAffinePartitionedSplineLensIGES(m iges.PartitionedRationalSplineLens, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := iges.WriteAffinePartitionedSplineLensSolid(&b, m); e != nil {
		return nil, e
	}
	return ReadAffinePartitionedSplineLensIGES(&b, iges.Limits{}, historyLimit)
}

// WriteAffinePartitionedSplineLensIGES re-proves unchanged native geometry and
// the full original affine source recipe before output. Only one identity
// occurrence is supported; no placement or source basis is silently flattened.
func (s State) WriteAffinePartitionedSplineLensIGES(w io.Writer) error {
	return s.writePartitionedLensIGES(w, true)
}

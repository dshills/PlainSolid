package document

import (
	"io"
	"plainsolid/exchange/step"
)

// ReadRationalSplineLensAssemblySTEP retains raw-weight curve recipes and both
// support-cell ancestries per shared local asset, with explicit rigid instances.
func ReadRationalSplineLensAssemblySTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	return readSplineLensAssemblySTEP(r, l, o, limits, historyLimit, true, true, true)
}

// WriteRationalSplineLensAssemblySTEP re-proves native material, raw curve bases
// and source ownership before preserving shared rigid occurrences in STEP.
func (s State) WriteRationalSplineLensAssemblySTEP(w io.Writer) error {
	return s.writeSplineLensAssemblySTEP(w, true, true, true)
}

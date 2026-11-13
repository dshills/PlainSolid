package document

import (
	"io"
	"plainsolid/exchange/step"
)

// ReadRefinedSplineLensAssemblySTEP retains one lossless source-curve recipe per
// shared product asset, separate from its explicit proper rigid occurrences.
func ReadRefinedSplineLensAssemblySTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	return readSplineLensAssemblySTEP(r, l, o, limits, historyLimit, true, true)
}

// WriteRefinedSplineLensAssemblySTEP re-proves local source recipes and shared
// product ancestry, preserving original curve bases without flattening instances.
func (s State) WriteRefinedSplineLensAssemblySTEP(w io.Writer) error {
	return s.writeSplineLensAssemblySTEP(w, true, true)
}

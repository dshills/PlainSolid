package step

import "io"

// WriteRefinedSplineLensAssembly retains each product's authoritative SeamCurves
// and explicit rigid occurrences. The original curve bases are re-proved.
func WriteRefinedSplineLensAssembly(w io.Writer, m SplineLensAssemblyModel) error {
	return writeSplineLensAssembly(w, m, true, true)
}

// ReadRefinedSplineLensAssembly exposes ProductCurves separately from canonical
// native bodies, retaining distinct model and owner pcurve knot bases.
func ReadRefinedSplineLensAssembly(r io.Reader, l Limits, o CurvedOvalOptions) (SplineLensAssembly, error) {
	return readSplineLensAssembly(r, l, o, true, true)
}

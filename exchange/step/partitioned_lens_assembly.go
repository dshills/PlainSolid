package step

import "io"

// WritePartitionedSplineLensAssembly writes local partitioned lens definitions
// and explicit proper signed-axis rigid occurrences. Each product owns one lens
// or is an assembly group. No charts are transformed or duplicated per instance.
func WritePartitionedSplineLensAssembly(w io.Writer, m SplineLensAssemblyModel) error {
	return writeSplineLensAssembly(w, m, true)
}

// ReadPartitionedSplineLensAssembly retains source product identity and each
// product's exact seam ancestry in ProductPieces. ProductBodies contains the
// original supports and canonical native material; source/native edges differ.
func ReadPartitionedSplineLensAssembly(r io.Reader, l Limits, o CurvedOvalOptions) (SplineLensAssembly, error) {
	return readSplineLensAssembly(r, l, o, true)
}

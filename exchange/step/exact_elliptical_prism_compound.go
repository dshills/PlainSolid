package step

import (
	"context"
	"io"
)

// ExactEllipticalPrismCompound is a standard multi-root STEP material file.
// Its two independent products retain original model coordinates and source IDs.
// It does not infer assembly occurrences, Boolean union or spatial disjointness.
type ExactEllipticalPrismCompound struct {
	Members    [2]ExactEllipticalPrismFamily
	Source     *File
	LengthUnit float64
}

// ReadExactEllipticalPrismCompound accepts exactly two complete, dependency-
// disjoint rooted products. Only the exact neutral representation frame may alias across roots.
// Model geometry, contexts, units and product metadata remain independent. Each member passes the full
// existing exact family proof; unsupported inputs return an entirely empty result.
func ReadExactEllipticalPrismCompound(ctx context.Context, r io.Reader, l Limits, tol float64) (ExactEllipticalPrismCompound, error) {
	m, e := readExactEllipticalPrismCollection(ctx, r, l, tol, 2, 2)
	if e != nil {
		return ExactEllipticalPrismCompound{}, e
	}
	return ExactEllipticalPrismCompound{Members: [2]ExactEllipticalPrismFamily{m.Members[0], m.Members[1]}, Source: m.Source, LengthUnit: m.LengthUnit}, nil
}

// WriteExactEllipticalPrismCompound emits two independent standard product roots
// with deterministic ID rebasing. It preserves each original product's numeric
// geometry, metadata and unit declarations. Shared source objects are copied by
// value into separate graph records; no instance/placement is invented.
func WriteExactEllipticalPrismCompound(ctx context.Context, w io.Writer, sources [2]*File, l Limits, tol float64) error {
	return writeExactEllipticalPrismCollection(ctx, w, sources[:], l, tol, 2, 2)
}

package geom

import (
	"context"
	"math/big"
)

// NewAffineXYParallelogramReference retains a proved affine XY footprint in
// an independent unit chart with positive orientation. The chart starts at
// the lexicographically least physical corner. Its U then V edges run
// counterclockwise. Height is zero and model XY stays exact and unrounded.
func NewAffineXYParallelogramReference(ctx context.Context, source *SurfacePatch, o SurfacePatchOptions) (*SurfacePatch, error) {
	options, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if ctx == nil || source == nil || len(source.net) == 0 {
		return nil, ErrInvalid
	}
	if len(source.net) > options.MaxPoles {
		return nil, ErrConvergence
	}
	graph, work, e := newAffineXYGraphWithWork(ctx, source, options)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: work}
	ids := [4]int{0, (source.countU - 1) * source.countV, len(source.net) - 1, source.countV - 1}
	var corners [4][4]*big.Rat
	first := 0
	for i, id := range ids {
		h := source.net[id]
		c := [4]*big.Rat{new(big.Rat).Quo(h[0], h[3]), new(big.Rat).Quo(h[1], h[3]), new(big.Rat), big.NewRat(1, 1)}
		for _, x := range c[:2] {
			if e := budget.check(x); e != nil {
				return nil, e
			}
		}
		corners[i] = c
		if x := c[0].Cmp(corners[first][0]); x < 0 || x == 0 && c[1].Cmp(corners[first][1]) < 0 {
			first = i
		}
	}
	direction := graph.Orientation()
	order := [4]int{first, (first - direction + 4) % 4, (first + direction + 4) % 4, (first + 2) % 4}
	control := make([][4]*big.Rat, 4)
	for i, j := range order {
		control[i] = corners[j]
	}
	remaining := options
	remaining.MaxOperations -= budget.operations
	if remaining.MaxOperations < 1 {
		return nil, ErrConvergence
	}
	reference, e := NewExactHomogeneousBezierSurface(ctx, 1, 1, control, remaining)
	if e != nil {
		return nil, e
	}
	reference.operations += budget.operations
	reference.options = options
	return reference, nil
}

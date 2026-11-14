package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicGraphTubeWindowOptions bounds preparation of smaller, conservative
// original-chart tube boxes. SubdivisionDepth defaults to1 and is capped at6;
// MaxIntervals defaults to1024 and is capped at8192. Exact applies cumulatively
// to all interval contractions. MaxIterations defaults to2 and is capped at24.
// Decoded immutable recipes are additionally capped at8192 cells.
// If a contraction cannot prove an improvement, its original box is retained.
type AlgebraicGraphTubeWindowOptions struct {
	Exact                                         geom.ExactCurvePlaneEventOptions
	SubdivisionDepth, MaxIntervals, MaxIterations int
}

type AlgebraicGraphTubeWindowReport struct {
	Complete                                                                     bool // Every original segment has a covering partition, not convergence.
	Segments, Intervals, ContractedIntervals, FallbackIntervals, ExactOperations int
}

// WithRefinedWindowTubes returns a new immutable prepared query. Its covering
// boxes may improve BoundWindowDepth and RefineWindowDepth near rims, without
// changing material classification. Earlier acceptance can change the adaptive
// spatial partition, so aggregate lower bounds need not improve monotonically
// across preparations (identical spatial cells retain their old certificates).
// Each original full free interval is covered
// by adjacent represented cuts; no restricted branch or endpoint is dropped.
// Nonrepresentable cuts and inconclusive bounded contractions retain the whole
// original cell. Cancellation or resource exhaustion returns no prepared query.
func (q *AlgebraicGraphMaterialQuery) WithRefinedWindowTubes(ctx context.Context, o AlgebraicGraphTubeWindowOptions) (result *AlgebraicGraphMaterialQuery, out AlgebraicGraphTubeWindowReport, err error) {
	defer func() {
		if err != nil {
			result = nil
			out.Complete = false
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil {
		return nil, out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, out, e
	}
	o.Exact, err = graphMaterialLimits(o.Exact)
	if err != nil {
		return nil, out, err
	}
	if o.SubdivisionDepth == 0 {
		o.SubdivisionDepth = 1
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 1024
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 2
	}
	if o.SubdivisionDepth < 1 || o.SubdivisionDepth > 6 || o.MaxIntervals < 1 || o.MaxIntervals > 8192 || o.MaxIterations < 1 || o.MaxIterations > 24 {
		return nil, out, geom.ErrInvalid
	}
	boxes := make([][][]geom.ParameterRectangle, len(q.loops))
	cache := map[intersect.TubeRecipe]intersect.TubeRecipeData{}
	recipeCells := 0
	for i, loop := range q.loops {
		boxes[i] = make([][]geom.ParameterRectangle, len(loop))
		for j, piece := range loop {
			if e := ctx.Err(); e != nil {
				return nil, out, e
			}
			out.Segments++
			recipe, index, second := piece.Segment.TrimA().Recipe()
			if second {
				return nil, out, geom.ErrConvergence
			}
			data, ok := cache[recipe]
			if !ok {
				data = recipe.Data()
				recipeCells += len(data.Cells)
				if recipeCells > 8192 {
					return nil, out, geom.ErrConvergence
				}
				cache[recipe] = data
			}
			if index < 0 || index >= len(data.Cells) {
				return nil, out, geom.ErrConvergence
			}
			original := data.Cells[index].DomainA
			_, domain, _, e := piece.Segment.ExactFreeDomain()
			if e != nil {
				return nil, out, e
			}
			n := 1 << o.SubdivisionDepth
			cuts := make([]float64, n+1)
			represented := true
			for k := 0; k <= n; k++ {
				x := new(big.Rat).Add(domain.Lower, new(big.Rat).Mul(new(big.Rat).Sub(domain.Upper, domain.Lower), big.NewRat(int64(k), int64(n))))
				if x.Num().BitLen() > o.Exact.MaxCoefficientBits || x.Denom().BitLen() > o.Exact.MaxCoefficientBits {
					return nil, out, geom.ErrConvergence
				}
				f, exact := x.Float64()
				if !exact || !finite(f) {
					represented = false
					break
				}
				cuts[k] = f
			}
			if !represented {
				if out.Intervals >= o.MaxIntervals {
					return nil, out, geom.ErrConvergence
				}
				boxes[i][j] = []geom.ParameterRectangle{original}
				out.Intervals++
				out.FallbackIntervals++
				continue
			}
			if n > o.MaxIntervals-out.Intervals {
				return nil, out, geom.ErrConvergence
			}
			for k := 0; k < n; k++ {
				remaining := o.Exact.MaxOperations - out.ExactOperations
				if remaining < 1 {
					return nil, out, geom.ErrConvergence
				}
				enclosure, e := piece.Segment.EncloseFreeInterval(ctx, cuts[k], cuts[k+1], geom.SurfaceTubeIntervalOptions{MaxIterations: o.MaxIterations, MaxOperations: remaining, MaxCoefficientBits: o.Exact.MaxCoefficientBits})
				out.ExactOperations += enclosure.ExactOperations
				if out.ExactOperations > o.Exact.MaxOperations {
					return nil, out, geom.ErrConvergence
				}
				if e != nil && !errors.Is(e, geom.ErrConvergence) {
					return nil, out, e
				}
				if e := ctx.Err(); e != nil {
					return nil, out, e
				}
				b := original
				if enclosure.HasBounds {
					u := enclosure.ParametersA
					b = geom.ParameterRectangle{UMin: math.Max(original.UMin, u.Min.X), UMax: math.Min(original.UMax, u.Max.X), VMin: math.Max(original.VMin, u.Min.Y), VMax: math.Min(original.VMax, u.Max.Y)}
					if !finite(b.UMin) || !finite(b.UMax) || !finite(b.VMin) || !finite(b.VMax) || b.UMin > b.UMax || b.VMin > b.VMax {
						return nil, out, geom.ErrConvergence
					}
				}
				if b != original {
					out.ContractedIntervals++
				} else {
					out.FallbackIntervals++
				}
				out.Intervals++
				boxes[i][j] = append(boxes[i][j], b)
			}
		}
	}
	copy := *q
	copy.windowTubeBoxes = boxes
	out.Complete = true
	return &copy, out, nil
}

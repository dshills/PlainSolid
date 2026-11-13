package geom

import (
	"context"
	"math/big"
)

// AffineProjectedGraph retains an immutable affine projection proof and the
// original model surface. Inverse rows use the selected model coordinates;
// evaluation and comparison evidence always use original XYZ.
type AffineProjectedGraph struct {
	source     *SurfacePatch
	graph      *AffineXYGraph
	axes       [2]int
	order      [3]int
	operations int
}

func NewAffineProjectedGraph(ctx context.Context, p *SurfacePatch, axes [2]int, o SurfacePatchOptions) (*AffineProjectedGraph, error) {
	if ctx == nil || p == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return nil, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	projected, e := permuteGraphPatch(p, order, budget)
	if e != nil {
		return nil, e
	}
	child := options
	child.MaxOperations -= budget.operations
	if child.MaxOperations < 1 {
		return nil, ErrConvergence
	}
	graph, e := NewAffineXYGraph(ctx, projected, child)
	if e != nil {
		return nil, e
	}
	return &AffineProjectedGraph{source: p, graph: graph, axes: axes, order: order, operations: budget.operations + graph.ExactOperations()}, nil
}
func (g *AffineProjectedGraph) Axes() ([2]int, error) {
	if g == nil || g.graph == nil {
		return [2]int{}, ErrInvalid
	}
	return g.axes, nil
}
func (g *AffineProjectedGraph) ResidualAxis() (int, error) {
	if g == nil || g.graph == nil {
		return 0, ErrInvalid
	}
	return g.order[2], nil
}
func (g *AffineProjectedGraph) Orientation() int {
	if g == nil || g.graph == nil {
		return 0
	}
	return g.graph.Orientation()
}
func (g *AffineProjectedGraph) Domain() ParameterRectangle {
	if g == nil || g.source == nil {
		return ParameterRectangle{}
	}
	return g.source.Domain()
}
func (g *AffineProjectedGraph) ExactOperations() int {
	if g == nil {
		return 0
	}
	return g.operations
}

// InverseCoefficients returns owned U,V rows in selected model axes and constant.
func (g *AffineProjectedGraph) InverseCoefficients() ([2][3]*big.Rat, error) {
	if g == nil || g.graph == nil {
		return [2][3]*big.Rat{}, ErrInvalid
	}
	return g.graph.InverseCoefficients()
}

// InvertRational accepts projected points outside the original rectangle; Inside
// and BoundaryMask diagnose exact chart membership without rounded tests.
func (g *AffineProjectedGraph) InvertRational(ctx context.Context, projected [2]*big.Rat, o SurfacePatchEvaluationOptions) (AffineXYLocation, error) {
	if g == nil || g.graph == nil {
		return AffineXYLocation{}, ErrInvalid
	}
	return g.graph.InvertRational(ctx, projected, o)
}
func (g *AffineProjectedGraph) EvaluateRational(ctx context.Context, uv [2]*big.Rat, o SurfacePatchEvaluationOptions) (RationalSurfacePatchJet, error) {
	if g == nil || g.source == nil {
		return RationalSurfacePatchJet{}, ErrInvalid
	}
	return g.source.EvaluateRational(ctx, uv[0], uv[1], o)
}

// ProjectedGraphHeightComparison embeds original XYZ evidence. Difference is
// source[ResidualAxis]-target[ResidualAxis] at identical selected projection.
type ProjectedGraphHeightComparison struct {
	AffineGraphHeightComparison
	ProjectionAxes [2]int
	ResidualAxis   int
}

// CompareHeightAtRational requires the same ordered projection on both proofs.
// Outside target reports contain original source evidence but no residual sign.
func (g *AffineProjectedGraph) CompareHeightAtRational(ctx context.Context, target *AffineProjectedGraph, uv [2]*big.Rat, o SurfacePatchEvaluationOptions) (r ProjectedGraphHeightComparison, err error) {
	if g == nil || target == nil || g.graph == nil || target.graph == nil || g.axes != target.axes {
		return r, ErrInvalid
	}
	r.ProjectionAxes = g.axes
	r.ResidualAxis = g.order[2]
	r.AffineGraphHeightComparison, err = g.graph.CompareHeightAtRational(ctx, target.graph, uv, o)
	restorePoint := func(p [3]*big.Rat) (q [3]*big.Rat) {
		for i, j := range g.order {
			q[j] = p[i]
		}
		return q
	}
	restoreBox := func(p Box) Box {
		var lo, hi [3]float64
		for i, j := range g.order {
			lo[j] = coordinate(p.Min, i)
			hi[j] = coordinate(p.Max, i)
		}
		return Box{Min: Vec3{lo[0], lo[1], lo[2]}, Max: Vec3{hi[0], hi[1], hi[2]}}
	}
	r.SourcePoint = restorePoint(r.SourcePoint)
	r.TargetPoint = restorePoint(r.TargetPoint)
	r.SourceBounds = restoreBox(r.SourceBounds)
	r.TargetBounds = restoreBox(r.TargetBounds)
	return r, err
}

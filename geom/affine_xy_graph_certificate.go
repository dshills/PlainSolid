package geom

import (
	"context"
	"math/big"
)

// AffineXYGraph is an immutable proof that a rational spline chart has an
// invertible affine model-XY projection. The height may be nonlinear/rational.
// It retains the original chart and exact coefficients; callers cannot replace
// a proved map by modifying diagnostic copies.
type AffineXYGraph struct {
	source     *SurfacePatch
	inverse    [2][3]*big.Rat
	operations int
}

func NewAffineXYGraph(ctx context.Context, p *SurfacePatch, o SurfacePatchOptions) (*AffineXYGraph, error) {
	g, _, e := newAffineXYGraphWithWork(ctx, p, o)
	return g, e
}

func newAffineXYGraphWithWork(ctx context.Context, p *SurfacePatch, o SurfacePatchOptions) (_ *AffineXYGraph, work int, err error) {
	options, e := patchOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { work = min(b.operations, b.maxOperations) }()
	_, inverse, e := normalizeAffineXYGraph(ctx, p, b)
	if e != nil {
		return nil, 0, e
	}
	return &AffineXYGraph{source: p, inverse: inverse, operations: b.operations}, b.operations, nil
}

// Orientation is the exact sign of the model XY Jacobian, or zero for an invalid proof.
func (g *AffineXYGraph) Orientation() int {
	if g == nil || g.source == nil {
		return 0
	}
	return new(big.Rat).Sub(new(big.Rat).Mul(g.inverse[0][0], g.inverse[1][1]), new(big.Rat).Mul(g.inverse[0][1], g.inverse[1][0])).Sign()
}

func (g *AffineXYGraph) ExactOperations() int {
	if g == nil {
		return 0
	}
	return g.operations
}
func (g *AffineXYGraph) Domain() ParameterRectangle {
	if g == nil || g.source == nil {
		return ParameterRectangle{}
	}
	return g.source.Domain()
}

// InverseCoefficients returns owned rows giving U and V as functions of X,Y,1.
func (g *AffineXYGraph) InverseCoefficients() ([2][3]*big.Rat, error) {
	var result [2][3]*big.Rat
	if g == nil || g.source == nil {
		return result, ErrInvalid
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			result[i][j] = new(big.Rat).Set(g.inverse[i][j])
		}
	}
	return result, nil
}

type AffineXYLocation struct {
	UV           [2]*big.Rat
	Bounds       UVBox
	Inside       bool
	BoundaryMask uint8 // UMin, UMax, VMin, VMax in bits 0..3
	Operations   int
}

// InvertRational maps arbitrary rational model XY, including points beyond the
// chart rectangle. Inside and BoundaryMask are exact domain diagnostics.
func (g *AffineXYGraph) InvertRational(ctx context.Context, xy [2]*big.Rat, o SurfacePatchEvaluationOptions) (r AffineXYLocation, err error) {
	if ctx == nil || g == nil || g.source == nil || xy[0] == nil || xy[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	for _, x := range xy {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	d := g.source.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	r.Inside = true
	var lo, hi [2]float64
	for axis := 0; axis < 2; axis++ {
		value := new(big.Rat).Set(g.inverse[axis][2])
		for j := 0; j < 2; j++ {
			value.Add(value, new(big.Rat).Mul(g.inverse[axis][j], xy[j]))
			if e := b.check(value); e != nil {
				return r, e
			}
		}
		r.UV[axis] = value
		for side := 0; side < 2; side++ {
			order := value.Cmp(new(big.Rat).SetFloat64(ranges[axis][side]))
			if side == 0 && order < 0 || side == 1 && order > 0 {
				r.Inside = false
			}
			if order == 0 {
				r.BoundaryMask |= 1 << uint(2*axis+side)
			}
		}
		lo[axis], hi[axis] = rationalFloatBound(value, true), rationalFloatBound(value, false)
	}
	r.Bounds = UVBox{Min: Vec2{X: lo[0], Y: lo[1]}, Max: Vec2{X: hi[0], Y: hi[1]}}
	return r, nil
}

// EvaluateRational returns exact original model coordinates and derivatives.
func (g *AffineXYGraph) EvaluateRational(ctx context.Context, uv [2]*big.Rat, o SurfacePatchEvaluationOptions) (RationalSurfacePatchJet, error) {
	if g == nil || g.source == nil {
		return RationalSurfacePatchJet{}, ErrInvalid
	}
	return g.source.EvaluateRational(ctx, uv[0], uv[1], o)
}

type AffineGraphHeightComparison struct {
	Complete, TargetOutside    bool
	SourceUV, TargetUV         [2]*big.Rat
	SourcePoint, TargetPoint   [3]*big.Rat
	SourceBounds, TargetBounds Box
	Difference                 *big.Rat // source Z minus target Z, nil if outside
	Sign                       int
	Operations                 int
}

// CompareHeightAtRational proves height order at one source UV. Target inversion
// uses the exact shared model XY and rechecks it against exact target evaluation.
// An outside target is Complete with TargetOutside and no height sign claim.
func (source *AffineXYGraph) CompareHeightAtRational(ctx context.Context, target *AffineXYGraph, uv [2]*big.Rat, o SurfacePatchEvaluationOptions) (r AffineGraphHeightComparison, err error) {
	if ctx == nil || source == nil || source.source == nil || target == nil || target.source == nil || uv[0] == nil || uv[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	remaining := func() (SurfacePatchEvaluationOptions, error) {
		q := o
		q.MaxOperations -= r.Operations
		if q.MaxOperations < 1 {
			return q, ErrConvergence
		}
		return q, nil
	}
	q, e := remaining()
	if e != nil {
		return r, e
	}
	a, e := source.EvaluateRational(ctx, uv, q)
	r.Operations += a.ExactOperations
	if e != nil {
		return r, e
	}
	r.SourceUV = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.SourcePoint = a.Point
	r.SourceBounds = a.PointBounds
	q, e = remaining()
	if e != nil {
		return r, e
	}
	location, e := target.InvertRational(ctx, [2]*big.Rat{a.Point[0], a.Point[1]}, q)
	r.Operations += location.Operations
	if e != nil {
		return r, e
	}
	r.TargetUV = location.UV
	if !location.Inside {
		r.Complete = true
		r.TargetOutside = true
		return r, nil
	}
	q, e = remaining()
	if e != nil {
		return r, e
	}
	b, e := target.EvaluateRational(ctx, location.UV, q)
	r.Operations += b.ExactOperations
	if e != nil {
		return r, e
	}
	if a.Point[0].Cmp(b.Point[0]) != 0 || a.Point[1].Cmp(b.Point[1]) != 0 {
		return r, ErrConvergence
	}
	r.TargetPoint = b.Point
	r.TargetBounds = b.PointBounds
	r.Difference = new(big.Rat).Sub(a.Point[2], b.Point[2])
	budget := &polynomialBudget{maxOperations: o.MaxOperations - r.Operations, maxBits: o.MaxCoefficientBits}
	e = budget.check(r.Difference)
	r.Operations += budget.operations
	if e != nil {
		return r, e
	}
	r.Sign = r.Difference.Sign()
	r.Complete = true
	return r, nil
}

// CompareAffineXYGraphHeight constructs both immutable projection proofs and
// compares one original rational source UV with cumulative arithmetic limits.
func CompareAffineXYGraphHeight(ctx context.Context, source, target *SurfacePatch, uv [2]*big.Rat, o SurfacePatchOptions) (r AffineGraphHeightComparison, err error) {
	options, e := patchOptions(o)
	if e != nil {
		return r, e
	}
	used := 0
	defer func() { r.Operations += used }()
	var graphs [2]*AffineXYGraph
	for i, p := range []*SurfacePatch{source, target} {
		child := options
		child.MaxOperations -= used
		if child.MaxOperations < 1 {
			return r, ErrConvergence
		}
		graphs[i], e = NewAffineXYGraph(ctx, p, child)
		if e != nil {
			return r, e
		}
		used += graphs[i].ExactOperations()
	}
	remaining := options.MaxOperations - used
	if remaining < 1 {
		return r, ErrConvergence
	}
	return graphs[0].CompareHeightAtRational(ctx, graphs[1], uv, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits})
}

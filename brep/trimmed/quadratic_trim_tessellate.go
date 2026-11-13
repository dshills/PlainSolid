package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// QuadraticTrimTessellationOptions bounds an adaptive boundary-only proof.
// MaxSegments defaults to 4096 (maximum 65536), MaxChecks to 2*MaxSegments-1,
// and MaxDepth to 32 (maximum 64). Evaluation bounds each exact support sample.
type QuadraticTrimTessellationOptions struct {
	Deflection                       float64
	MaxSegments, MaxChecks, MaxDepth int
	Evaluation                       geom.SurfacePatchEvaluationOptions
}

// CertifiedQuadraticTrimPolyline preserves exact rational surface samples and
// original spline parameters, in traversal order. Points are their nearest
// binary64 display values; SquaredErrors includes that rounding. Every segment
// has a two-sided common-parameter correspondence certificate, not merely a
// sampled distance test. No loop topology or face-interior guarantee is made.
type CertifiedQuadraticTrimPolyline struct {
	CertifiedSplineTrimPolyline
	ExactPoints [][3]*big.Rat
	ExactUV     [][2]*big.Rat
	Checks      int
}

// TessellateQuadraticSplineTrimCertified adaptively subdivides one supported
// rational quadratic/linear Bézier UV trim. Exact endpoints are never silently
// moved; if an original-parameter midpoint is not representable it rejects.
// The exact curve/support composition is sampled only to choose chord endpoints;
// acceptance uses BoundQuadraticSplineTrimCorrespondence over every point.
// All failures return an empty result.
func TessellateQuadraticSplineTrimCertified(ctx context.Context, patch *geom.SurfacePatch, trim SplineUV, o QuadraticTrimTessellationOptions) (CertifiedQuadraticTrimPolyline, error) {
	empty := CertifiedQuadraticTrimPolyline{}
	if ctx == nil || patch == nil || trim.Curve == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(trim.Range.Min) || !finite(trim.Range.Max) || trim.Range.Min >= trim.Range.Max {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxChecks == 0 {
		o.MaxChecks = 2*o.MaxSegments - 1
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxSegments < 1 || o.MaxSegments > 65536 || o.MaxChecks < 1 || o.MaxDepth < 1 || o.MaxDepth > 64 {
		return empty, geom.ErrInvalid
	}
	degree := trim.Curve.Degree()
	poles, weights, knots := trim.Curve.Poles(), trim.Curve.Weights(), trim.Curve.Knots()
	if degree < 1 || degree > 2 || len(poles) != degree+1 || len(knots) != 2*(degree+1) {
		return empty, geom.ErrConvergence
	}
	type sample struct {
		point geom.Vec3
		exact [3]*big.Rat
		uv    [2]*big.Rat
	}
	evaluate := func(parameter float64) (sample, error) {
		var result sample
		t := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(parameter), ratInset(knots[0])), new(big.Rat).Sub(ratInset(knots[len(knots)-1]), ratInset(knots[0])))
		one := new(big.Rat).Sub(big.NewRat(1, 1), t)
		basis := []*big.Rat{one, t}
		if degree == 2 {
			basis = []*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, t)), new(big.Rat).Mul(t, t)}
		}
		w := new(big.Rat)
		u, v := new(big.Rat), new(big.Rat)
		for i, b := range basis {
			weight := big.NewRat(1, 1)
			if len(weights) > 0 {
				weight = ratInset(weights[i])
			}
			b = new(big.Rat).Mul(b, weight)
			w.Add(w, b)
			u.Add(u, new(big.Rat).Mul(b, ratInset(poles[i].X)))
			v.Add(v, new(big.Rat).Mul(b, ratInset(poles[i].Y)))
		}
		if w.Sign() <= 0 {
			return result, geom.ErrDomain
		}
		u.Quo(u, w)
		v.Quo(v, w)
		jet, e := patch.EvaluateRational(ctx, u, v, o.Evaluation)
		if e != nil {
			return result, e
		}
		result.uv = [2]*big.Rat{u, v}
		result.exact = jet.Point
		x, _ := jet.Point[0].Float64()
		y, _ := jet.Point[1].Float64()
		z, _ := jet.Point[2].Float64()
		result.point = geom.Vec3{X: x, Y: y, Z: z}
		if !result.point.IsFinite() {
			return sample{}, geom.ErrConvergence
		}
		return result, nil
	}
	a, err := evaluate(trim.Range.Min)
	if err != nil {
		return empty, err
	}
	b, err := evaluate(trim.Range.Max)
	if err != nil {
		return empty, err
	}
	type interval struct {
		lo, hi float64
		a, b   sample
		depth  int
	}
	stack := []interval{{trim.Range.Min, trim.Range.Max, a, b, 0}}
	result := CertifiedQuadraticTrimPolyline{}
	target := ratInset(o.Deflection)
	target.Mul(target, target)
	appendPoint := func(t float64, p sample) {
		result.Parameters = append(result.Parameters, t)
		result.Points = append(result.Points, p.point)
		result.ExactPoints = append(result.ExactPoints, p.exact)
		result.ExactUV = append(result.ExactUV, p.uv)
	}
	appendPoint(trim.Range.Min, a)
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if result.Checks == o.MaxChecks {
			return empty, geom.ErrConvergence
		}
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		part := trim
		part.Range = UVInterval{Min: node.lo, Max: node.hi}
		part.Reversed = false
		bound, e := BoundQuadraticSplineTrimCorrespondence(ctx, patch, part, node.a.point, node.b.point)
		result.Checks++
		if e != nil {
			return empty, e
		}
		if bound.Cmp(target) <= 0 {
			appendPoint(node.hi, node.b)
			result.SquaredErrors = append(result.SquaredErrors, bound)
			continue
		}
		if node.depth == o.MaxDepth || len(result.SquaredErrors)+len(stack)+2 > o.MaxSegments {
			return empty, geom.ErrConvergence
		}
		exactMid := new(big.Rat).Quo(new(big.Rat).Add(ratInset(node.lo), ratInset(node.hi)), big.NewRat(2, 1))
		mid, _ := exactMid.Float64()
		if mid <= node.lo || mid >= node.hi || ratInset(mid).Cmp(exactMid) != 0 {
			return empty, geom.ErrConvergence
		}
		center, e := evaluate(mid)
		if e != nil {
			return empty, e
		}
		stack = append(stack, interval{mid, node.hi, center, node.b, node.depth + 1}, interval{node.lo, mid, node.a, center, node.depth + 1})
	}
	if trim.Reversed {
		for i, j := 0, len(result.Points)-1; i < j; i, j = i+1, j-1 {
			result.Points[i], result.Points[j] = result.Points[j], result.Points[i]
			result.Parameters[i], result.Parameters[j] = result.Parameters[j], result.Parameters[i]
			result.ExactPoints[i], result.ExactPoints[j] = result.ExactPoints[j], result.ExactPoints[i]
			result.ExactUV[i], result.ExactUV[j] = result.ExactUV[j], result.ExactUV[i]
		}
		for i, j := 0, len(result.SquaredErrors)-1; i < j; i, j = i+1, j-1 {
			result.SquaredErrors[i], result.SquaredErrors[j] = result.SquaredErrors[j], result.SquaredErrors[i]
		}
	}
	return result, nil
}

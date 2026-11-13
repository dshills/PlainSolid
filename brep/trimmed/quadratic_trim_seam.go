package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// QuadraticTrimSeamOptions bounds the algebraic identity proof and shared chord
// refinement. MaxIdentityChecks defaults to17 and must allow all17 exact tests.
// Tessellation.MaxChecks counts both chart chord certificates (default
// 4*MaxSegments-2). Other limits retain QuadraticTrimTessellationOptions meaning.
type QuadraticTrimSeamOptions struct {
	Tessellation      QuadraticTrimTessellationOptions
	MaxIdentityChecks int
}

// ExactTrimParameterMap records originalParameter=Offset+Scale*q for the
// common normalized seam parameter q. A negative Scale proves reversal;
// Degree records the original represented trim degree, not a fitted degree.
// Coefficients are caller-owned exact rationals.
type ExactTrimParameterMap struct {
	Offset, Scale *big.Rat
	Degree        int
}

// CertifiedQuadraticTrimSeam is one shared model polyline with exact samples
// and separate directed source parameters/UV coordinates for its two charts.
// NormalizedParameters increases from0 to1 along BOTH supplied directed trims.
// SquaredErrors[chart][segment] includes rounding of the shared display chord.
// Complete continuous equality is proved algebraically before refinement. This
// is a seam primitive, not a stitched face or shell mesh certificate: adjacent
// face triangles must be re-certified if their boundary vertices are replaced.
type CertifiedQuadraticTrimSeam struct {
	ParameterMaps          [2]ExactTrimParameterMap
	Points                 []geom.Vec3
	ExactPoints            [][3]*big.Rat
	NormalizedParameters   []float64
	SourceParameters       [2][]float64
	ExactUV                [2][][2]*big.Rat
	SquaredErrors          [2][]*big.Rat
	IdentityChecks, Checks int
}

// CertifyQuadraticTrimSeam proves two composed rational trims agree at every
// normalized parameter and builds synchronized certified chords. Each composed
// homogeneous coordinate has degree<=8; cross-multiplication of two charts has
// degree<=16. Zero at17 distinct exact rational parameters therefore proves the
// polynomial identity, not a sampled approximation. Positive denominators and
// whole-domain UV control-hull containment are mandatory. Opposite original
// edge directions must be represented explicitly by SplineUV.Reversed.
// Failure returns no partial seam. Nonbinary source-parameter splits reject.
func CertifyQuadraticTrimSeam(ctx context.Context, a, b *geom.SurfacePatch, ta, tb SplineUV, o QuadraticTrimSeamOptions) (CertifiedQuadraticTrimSeam, error) {
	empty := CertifiedQuadraticTrimSeam{}
	if ctx == nil || a == nil || b == nil || !finite(o.Tessellation.Deflection) || o.Tessellation.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxIdentityChecks < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxIdentityChecks == 0 {
		o.MaxIdentityChecks = 17
	}
	if o.MaxIdentityChecks < 17 {
		return empty, geom.ErrConvergence
	}
	limits := o.Tessellation
	if limits.MaxSegments == 0 {
		limits.MaxSegments = 4096
	}
	if limits.MaxChecks == 0 {
		limits.MaxChecks = 4*limits.MaxSegments - 2
	}
	if limits.MaxDepth == 0 {
		limits.MaxDepth = 32
	}
	if limits.MaxSegments < 1 || limits.MaxSegments > 65536 || limits.MaxChecks < 1 || limits.MaxDepth < 1 || limits.MaxDepth > 64 {
		return empty, geom.ErrInvalid
	}
	patches := [2]*geom.SurfacePatch{a, b}
	trims := [2]SplineUV{ta, tb}
	var controls [2][3][3]*big.Rat
	for i, p := range patches {
		if _, _, ok := p.RationalBiquadraticCoefficients(); !ok {
			return empty, geom.ErrConvergence
		}
		h, e := quadraticTrimSourceControls(trims[i])
		if e != nil {
			return empty, e
		}
		controls[i] = h
		d := p.Domain()
		for _, pole := range trims[i].Curve.Poles() {
			if pole.Z != 0 || pole.X < d.UMin || pole.X > d.UMax || pole.Y < d.VMin || pole.Y > d.VMax {
				return empty, geom.ErrDomain
			}
		}
	}
	type sample struct {
		model [3]*big.Rat
		uv    [2][2]*big.Rat
		point geom.Vec3
	}
	evaluate := func(t *big.Rat) (sample, error) {
		var result sample
		one := new(big.Rat).Sub(big.NewRat(1, 1), t)
		basis := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, t)), new(big.Rat).Mul(t, t)}
		for chart, p := range patches {
			var homogeneous [3]*big.Rat
			for k := range homogeneous {
				homogeneous[k] = new(big.Rat)
				for j, w := range basis {
					homogeneous[k].Add(homogeneous[k], new(big.Rat).Mul(w, controls[chart][j][k]))
				}
			}
			if homogeneous[2].Sign() <= 0 {
				return sample{}, geom.ErrDomain
			}
			uv := [2]*big.Rat{new(big.Rat).Quo(homogeneous[0], homogeneous[2]), new(big.Rat).Quo(homogeneous[1], homogeneous[2])}
			jet, e := p.EvaluateRational(ctx, uv[0], uv[1], limits.Evaluation)
			if e != nil {
				return sample{}, e
			}
			result.uv[chart] = uv
			if chart == 0 {
				result.model = jet.Point
			} else {
				for k := range result.model {
					if result.model[k].Cmp(jet.Point[k]) != 0 {
						return sample{}, geom.ErrInvalid
					}
				}
			}
		}
		x, _ := result.model[0].Float64()
		y, _ := result.model[1].Float64()
		z, _ := result.model[2].Float64()
		result.point = geom.Vec3{X: x, Y: y, Z: z}
		if !result.point.IsFinite() {
			return sample{}, geom.ErrConvergence
		}
		return result, nil
	}
	var first, last sample
	for i := 0; i <= 16; i++ {
		s, e := evaluate(big.NewRat(int64(i), 16))
		if e != nil {
			return empty, e
		}
		if i == 0 {
			first = s
		}
		if i == 16 {
			last = s
		}
	}
	sourceParameter := func(chart int, t float64) (float64, error) {
		q := ratInset(t)
		if trims[chart].Reversed {
			q.Sub(big.NewRat(1, 1), q)
		}
		q.Mul(q, new(big.Rat).Sub(ratInset(trims[chart].Range.Max), ratInset(trims[chart].Range.Min)))
		q.Add(q, ratInset(trims[chart].Range.Min))
		f, _ := q.Float64()
		if !finite(f) || ratInset(f).Cmp(q) != 0 {
			return 0, geom.ErrConvergence
		}
		return f, nil
	}
	result := CertifiedQuadraticTrimSeam{IdentityChecks: 17}
	for i, trim := range trims {
		offset := ratInset(trim.Range.Min)
		scale := new(big.Rat).Sub(ratInset(trim.Range.Max), offset)
		if trim.Reversed {
			offset = ratInset(trim.Range.Max)
			scale.Neg(scale)
		}
		result.ParameterMaps[i] = ExactTrimParameterMap{Offset: offset, Scale: scale, Degree: trim.Curve.Degree()}
	}

	appendPoint := func(t float64, s sample) error {
		result.NormalizedParameters = append(result.NormalizedParameters, t)
		result.Points = append(result.Points, s.point)
		result.ExactPoints = append(result.ExactPoints, s.model)
		for i := range trims {
			parameter, e := sourceParameter(i, t)
			if e != nil {
				return e
			}
			result.SourceParameters[i] = append(result.SourceParameters[i], parameter)
			result.ExactUV[i] = append(result.ExactUV[i], s.uv[i])
		}
		return nil
	}
	if e := appendPoint(0, first); e != nil {
		return empty, e
	}
	type interval struct {
		lo, hi float64
		a, b   sample
		depth  int
	}
	stack := []interval{{0, 1, first, last, 0}}
	target := ratInset(limits.Deflection)
	target.Mul(target, target)
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if result.Checks > limits.MaxChecks-2 {
			return empty, geom.ErrConvergence
		}
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		var bounds [2]*big.Rat
		accepted := true
		for chart, p := range patches {
			lo, e := sourceParameter(chart, node.lo)
			if e != nil {
				return empty, e
			}
			hi, e := sourceParameter(chart, node.hi)
			if e != nil {
				return empty, e
			}
			part := trims[chart]
			part.Range = UVInterval{Min: lo, Max: hi}
			part.Reversed = lo > hi
			if part.Reversed {
				part.Range.Min, part.Range.Max = hi, lo
			}
			bounds[chart], e = BoundQuadraticSplineTrimCorrespondence(ctx, p, part, node.a.point, node.b.point)
			result.Checks++
			if e != nil {
				return empty, e
			}
			if bounds[chart].Cmp(target) > 0 {
				accepted = false
			}
		}
		if accepted {
			if e := appendPoint(node.hi, node.b); e != nil {
				return empty, e
			}
			for i := range bounds {
				result.SquaredErrors[i] = append(result.SquaredErrors[i], bounds[i])
			}
			continue
		}
		if node.depth == limits.MaxDepth || len(result.SquaredErrors[0])+len(stack)+2 > limits.MaxSegments {
			return empty, geom.ErrConvergence
		}
		exactMid := new(big.Rat).Quo(new(big.Rat).Add(ratInset(node.lo), ratInset(node.hi)), big.NewRat(2, 1))
		mid, _ := exactMid.Float64()
		if mid <= node.lo || mid >= node.hi || ratInset(mid).Cmp(exactMid) != 0 {
			return empty, geom.ErrConvergence
		}
		center, e := evaluate(exactMid)
		if e != nil {
			return empty, e
		}
		stack = append(stack, interval{mid, node.hi, center, node.b, node.depth + 1}, interval{node.lo, mid, node.a, center, node.depth + 1})
	}
	return result, nil
}

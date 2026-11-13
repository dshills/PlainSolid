package geom

import (
	"context"
	"math/big"
)

type surfaceTubeBoundaryAnchor struct {
	free       float64
	parameters [4]*big.Rat
	point      [3]*big.Rat
	operations int
}

// SurfaceTubeBoundaryOptions proposes one exact contact at a free-axis
// endpoint. In contrast to a uniform interior tube, a dependent coordinate
// may also reach its box boundary at this anchor.
type SurfaceTubeBoundaryOptions struct {
	SurfacePairTubeOptions
	AtUpper bool
	Anchor  [4]*big.Rat
}

// NewBoundaryAnchoredSurfaceTube certifies a regular branch starting at an
// exact boundary contact and continuing across the entire free interval.
// The patches must each be a single Bezier rectangle. An interval of
// Jacobian cofactors gives all dependent derivatives relative to the free
// coordinate. Integrating those bounds from the exact contact must keep
// the branch inside the box, strictly in the dependent interior away from
// the anchor. The same interval determinant proves injectivity of each
// fixed-free residual over its convex box. The proof is local to the boxes.
func NewBoundaryAnchoredSurfaceTube(ctx context.Context, a, c *SurfacePatch, o SurfaceTubeBoundaryOptions) (VerifiedSurfaceTube, error) {
	return newBoundaryAnchoredSurfaceTube(ctx, a, c, o, nil)
}

func newBoundaryAnchoredSurfaceTube(ctx context.Context, a, c *SurfacePatch, o SurfaceTubeBoundaryOptions, used *int) (VerifiedSurfaceTube, error) {
	var zero VerifiedSurfaceTube
	if ctx == nil || a == nil || c == nil || o.FreeAxis < 0 || o.FreeAxis > 3 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) == 0 {
			return zero, ErrDomain
		}
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return zero, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if used != nil {
		defer func() { *used = b.operations }()
	}
	ratFloat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	ranges := pairParameterRanges(SurfacePairParameterBox{a.Domain(), c.Domain()})
	h := &surfaceTubeBoundaryAnchor{}
	end := 0
	if o.AtUpper {
		end = 1
	}
	h.free = ranges[o.FreeAxis][end]
	for j, q := range o.Anchor {
		if q == nil {
			return zero, ErrInvalid
		}
		if e := b.check(q); e != nil {
			return zero, e
		}
		lo, hi := ratFloat(ranges[j][0]), ratFloat(ranges[j][1])
		if q.Cmp(lo) < 0 || q.Cmp(hi) > 0 || j == o.FreeAxis && q.Cmp(ratFloat(h.free)) != 0 {
			return zero, ErrDomain
		}
		h.parameters[j] = new(big.Rat).Set(q)
	}
	x, _, e := exactSurfaceJet(a, h.parameters[0], h.parameters[1], b)
	if e != nil {
		return zero, e
	}
	y, _, e := exactSurfaceJet(c, h.parameters[2], h.parameters[3], b)
	if e != nil {
		return zero, e
	}
	for i := range x {
		if x[i].Cmp(y[i]) != 0 {
			return zero, ErrConvergence
		}
	}
	h.point = x
	cofactors, e := surfaceTubeCofactorIntervals(ctx, a, c, b)
	if e != nil {
		return zero, e
	}
	denominator := cofactors[o.FreeAxis]
	sign := denominator.sign()
	if sign == 0 {
		return zero, ErrConvergence
	}
	width := new(big.Rat).Sub(ratFloat(ranges[o.FreeAxis][end^1]), ratFloat(h.free))
	for i, q := range h.parameters {
		if i == o.FreeAxis {
			continue
		}
		d, e := intervalDivide(cofactors[i], denominator, b)
		if e != nil {
			return zero, e
		}
		travel, e := intervalMultiply(d, exactInterval{width, width}, b)
		if e != nil {
			return zero, e
		}
		lo, hi := new(big.Rat).Add(q, travel.lo), new(big.Rat).Add(q, travel.hi)
		for _, x := range []*big.Rat{lo, hi} {
			if e := b.check(x); e != nil {
				return zero, e
			}
		}
		if lo.Cmp(ratFloat(ranges[i][0])) <= 0 || hi.Cmp(ratFloat(ranges[i][1])) >= 0 {
			return zero, ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	h.operations = b.operations
	return VerifiedSurfaceTube{a: a, b: c, axis: o.FreeAxis, tangentSign: sign, anchor: h}, nil
}

func surfaceTubeCofactorIntervals(ctx context.Context, a, c *SurfacePatch, b *polynomialBudget) (cofactors [4]exactInterval, err error) {
	_, ad, e := surfaceDifferentialIntervals(a, -1, 0, b)
	if e != nil {
		return cofactors, e
	}
	if e := ctx.Err(); e != nil {
		return cofactors, e
	}
	_, cd, e := surfaceDifferentialIntervals(c, -1, 0, b)
	if e != nil {
		return cofactors, e
	}
	neg := func(v exactInterval) exactInterval {
		return exactInterval{new(big.Rat).Neg(v.hi), new(big.Rat).Neg(v.lo)}
	}
	var jacobian [3][4]exactInterval
	for i := range jacobian {
		jacobian[i] = [4]exactInterval{ad[i][0], ad[i][1], neg(cd[i][0]), neg(cd[i][1])}
	}
	for i := range cofactors {
		var columns [3]int
		n := 0
		for j := range cofactors {
			if j != i {
				columns[n], n = j, n+1
			}
		}
		r, e := surfaceTubeIntervalDeterminant(jacobian, columns, b)
		if e != nil {
			return cofactors, e
		}
		if i%2 == 1 {
			r = neg(r)
		}
		cofactors[i] = r
	}
	return cofactors, nil
}

// CertifyBoundaryAnchoredSurfacePairTube reports the same local proof as
// NewBoundaryAnchoredSurfaceTube, with the usual tube range and orientation.
// Test.ExactOperations records proof work; other Krawczyk diagnostics are
// unset because this is a continuation proof.
func CertifyBoundaryAnchoredSurfacePairTube(ctx context.Context, a, c *SurfacePatch, o SurfaceTubeBoundaryOptions) (SurfaceTubeCertificate, error) {
	var work int
	t, e := newBoundaryAnchoredSurfaceTube(ctx, a, c, o, &work)
	if e != nil {
		return SurfaceTubeCertificate{Test: SurfaceSectionCertificate{ExactOperations: work}}, e
	}
	return t.boundaryCertificate(), nil
}

func (t VerifiedSurfaceTube) boundaryCertificate() SurfaceTubeCertificate {
	r := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})[t.axis]
	return SurfaceTubeCertificate{
		FreeTangentSign: t.tangentSign, FreeAxis: t.axis, FreeParameter: CertifiedScalarRange{Min: r[0], Max: r[1]},
		Test: SurfaceSectionCertificate{ExactOperations: t.anchor.operations}, ProvenBranch: true,
	}
}

func surfaceTubeIntervalDeterminant(m [3][4]exactInterval, c [3]int, b *polynomialBudget) (exactInterval, error) {
	d := exactInterval{new(big.Rat), new(big.Rat)}
	for i, a := range c {
		j, k := c[(i+1)%3], c[(i+2)%3]
		p, e := intervalMultiply(m[1][j], m[2][k], b)
		if e != nil {
			return exactInterval{}, e
		}
		q, e := intervalMultiply(m[1][k], m[2][j], b)
		if e != nil {
			return exactInterval{}, e
		}
		r, e := intervalSubtract(p, q, b)
		if e != nil {
			return exactInterval{}, e
		}
		t, e := intervalMultiply(m[0][a], r, b)
		if e != nil {
			return exactInterval{}, e
		}
		d, e = boundedExactInterval(new(big.Rat).Add(d.lo, t.lo), new(big.Rat).Add(d.hi, t.hi), b)
		if e != nil {
			return exactInterval{}, e
		}
	}
	return d, nil
}

func (a surfaceTubeBoundaryAnchor) evaluation(o SurfaceTubeEvaluationOptions) (SurfaceTubeEvaluation, error) {
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-8
	}
	if o.BoxTolerance == 0 {
		o.BoxTolerance = 1e-8
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 24
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || !finite(o.BoxTolerance) || o.BoxTolerance <= 0 ||
		o.MaxIterations < 1 || o.MaxIterations > 256 || o.MaxOperations < 0 || o.MaxCoefficientBits != 0 && o.MaxCoefficientBits < 64 {
		return SurfaceTubeEvaluation{}, ErrInvalid
	}
	q := func(x *big.Rat) CertifiedScalarRange {
		return certifiedRange(exactInterval{x, x})
	}
	x, y, z := q(a.point[0]), q(a.point[1]), q(a.point[2])
	uv := func(i int) UVBox {
		a, b := q(a.parameters[i]), q(a.parameters[i+1])
		return UVBox{Min: Vec2{a.Min, b.Min}, Max: Vec2{a.Max, b.Max}}
	}
	nearest := func(r *big.Rat) float64 { x, _ := r.Float64(); return x }
	r := SurfaceTubeEvaluation{ParametersA: uv(0), ParametersB: uv(2),
		Bounds: Box{Min: Vec3{x.Min, y.Min, z.Min}, Max: Vec3{x.Max, y.Max, z.Max}},
		Point:  Vec3{nearest(a.point[0]), nearest(a.point[1]), nearest(a.point[2])}, Enclosed: true}
	s := r.Bounds.Size()
	if max(s.X, s.Y, s.Z) > o.BoxTolerance {
		return r, ErrConvergence
	}
	for _, p := range []UVBox{r.ParametersA, r.ParametersB} {
		if max(p.Max.X-p.Min.X, p.Max.Y-p.Min.Y) > o.ParameterTolerance {
			return r, ErrConvergence
		}
	}
	r.Complete = true
	return r, nil
}

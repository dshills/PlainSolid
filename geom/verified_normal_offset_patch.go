package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetPatch is an immutable smooth offset surface. Its
// source and projection passed the full embedding proof for every fixed
// displacement between zero and distance. It does not own trimmed topology
// or certify collisions with other patches.
type VerifiedNormalOffsetPatch struct {
	source     *SurfacePatch
	distance   *big.Rat
	projection NormalOffsetProjection
}

// NewVerifiedNormalOffsetPatch builds a surface only after proving the full
// source domain. Inconclusive input returns the unresolved cover and no
// usable surface. The distance is copied.
func NewVerifiedNormalOffsetPatch(ctx context.Context, source *SurfacePatch, distance *big.Rat,
	projection NormalOffsetProjection, o NormalOffsetCoverageOptions) (*VerifiedNormalOffsetPatch, NormalOffsetCoverage, error) {
	if distance == nil {
		return nil, NormalOffsetCoverage{}, ErrInvalid
	}
	proof, err := source.CertifyProjectedNormalOffsetEmbedding(ctx, distance, projection, o)
	if err != nil {
		return nil, proof, err
	}
	return &VerifiedNormalOffsetPatch{source, new(big.Rat).Set(distance), projection}, proof, nil
}

func (p *VerifiedNormalOffsetPatch) Domain() ParameterRectangle {
	if p == nil {
		return ParameterRectangle{}
	}
	return p.source.Domain()
}

// Distance returns an owned copy of the signed distance in model units.
func (p *VerifiedNormalOffsetPatch) Distance() *big.Rat {
	if p == nil || p.distance == nil {
		return nil
	}
	return new(big.Rat).Set(p.distance)
}

// EvaluateRational encloses the true normal offset at original rational UV.
func (p *VerifiedNormalOffsetPatch) EvaluateRational(ctx context.Context, u, v *big.Rat,
	source SurfacePatchEvaluationOptions, offset SurfaceNormalOffsetEvaluationOptions) (RationalSurfaceOffsetPoint, error) {
	if p == nil || p.source == nil {
		return RationalSurfaceOffsetPoint{}, ErrInvalid
	}
	jet, err := p.source.EvaluateRational(ctx, u, v, source)
	if err != nil {
		return RationalSurfaceOffsetPoint{}, err
	}
	return jet.NormalOffsetRational(ctx, p.distance, offset)
}

// Eval implements Surface with a numerical representative chosen from the
// directed rational coordinate enclosure.
func (p *VerifiedNormalOffsetPatch) Eval(u, v float64) (Vec3, error) {
	if !finite(u) || !finite(v) {
		return Vec3{}, ErrDomain
	}
	bounds, err := p.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		return Vec3{}, err
	}
	return rationalOffsetVectorRepresentative(bounds.Lower, bounds.Upper)
}

// PartialsRational encloses the offset's analytic U and V partials in the
// source's physical parameter units, including the derivative of the unit
// normal. Ranges are returned as two independently owned rational vectors.
func (p *VerifiedNormalOffsetPatch) PartialsRational(ctx context.Context, u, v *big.Rat,
	source SurfacePatchEvaluationOptions, offset SurfaceNormalOffsetEvaluationOptions) (du, dv RationalSurfaceOffsetPoint, err error) {
	if p == nil || p.source == nil {
		return du, dv, ErrInvalid
	}
	jet, err := p.source.EvaluateSecondJetRational(ctx, u, v, source)
	if err != nil {
		return du, dv, err
	}
	return jet.offsetPartialsRational(ctx, p.distance, offset)
}

// Partials implements Surface using numerical representatives of the
// rigorous rational enclosures of the two original parameter derivatives.
func (p *VerifiedNormalOffsetPatch) Partials(u, v float64) (Vec3, Vec3, error) {
	if !finite(u) || !finite(v) {
		return Vec3{}, Vec3{}, ErrDomain
	}
	du, dv, err := p.PartialsRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	a, err := rationalOffsetVectorRepresentative(du.Lower, du.Upper)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	b, err := rationalOffsetVectorRepresentative(dv.Lower, dv.Upper)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	return a, b, nil
}

func rationalOffsetVectorRepresentative(lower, upper [3]*big.Rat) (Vec3, error) {
	var c [3]float64
	for i := range c {
		mid := new(big.Rat).Add(lower[i], upper[i])
		c[i], _ = mid.Quo(mid, big.NewRat(2, 1)).Float64()
	}
	v := Vec3{c[0], c[1], c[2]}
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}

func (s RationalSurfacePatchSecondJet) offsetPartialsRational(ctx context.Context, distance *big.Rat,
	o SurfaceNormalOffsetEvaluationOptions) (du, dv RationalSurfaceOffsetPoint, err error) {
	if o.SqrtBits == 0 {
		o.SqrtBits = 96
	}
	if o.SqrtBits < 1 || o.SqrtBits > 1024 {
		return du, dv, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return du, dv, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	if err := b.check(distance); err != nil {
		return du, dv, err
	}
	for _, v := range [5][3]*big.Rat{s.Du, s.Dv, s.Duu, s.Duv, s.Dvv} {
		for _, q := range v {
			if err := b.check(q); err != nil {
				return du, dv, err
			}
		}
	}
	a := boundedRatArithmetic{b: b}
	cross := func(x, y [3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			j, k := (i+1)%3, (i+2)%3
			r[i] = a.sub(a.mul(x[j], y[k]), a.mul(x[k], y[j]))
		}
		return r
	}
	dot := func(x, y [3]*big.Rat) *big.Rat {
		return a.add(a.add(a.mul(x[0], y[0]), a.mul(x[1], y[1])), a.mul(x[2], y[2]))
	}
	n := cross(s.Du, s.Dv)
	nu, nuOther := cross(s.Duu, s.Dv), cross(s.Du, s.Duv)
	nv, nvOther := cross(s.Duv, s.Dv), cross(s.Du, s.Dvv)
	scale := new(big.Rat)
	for _, q := range n {
		if v := new(big.Rat).Abs(q); v.Cmp(scale) > 0 {
			scale = v
		}
	}
	if a.err != nil {
		return du, dv, a.err
	}
	if scale.Sign() == 0 {
		return du, dv, ErrDegenerate
	}
	for i := range n {
		n[i] = a.quo(n[i], scale)
		nu[i] = a.quo(a.add(nu[i], nuOther[i]), scale)
		nv[i] = a.quo(a.add(nv[i], nvOther[i]), scale)
	}
	length2 := dot(n, n)
	if a.err != nil {
		return du, dv, a.err
	}
	lo, hi, err := rationalSqrtBounds(length2, o.SqrtBits, b)
	if err != nil {
		return du, dv, err
	}
	den, err := boundedExactInterval(a.mul(lo, length2), a.mul(hi, length2), b)
	if a.err != nil {
		return du, dv, a.err
	}
	if err != nil {
		return du, dv, err
	}
	var vectors [2]RationalSurfaceOffsetPoint
	for direction, derivative := range [2][3]*big.Rat{nu, nv} {
		base := [2][3]*big.Rat{s.Du, s.Dv}[direction]
		projection := dot(n, derivative)
		r := &vectors[direction]
		for i := range derivative {
			// The derivative of N/|N| is (N' |N|² - N(N·N'))/|N|³.
			num := a.mul(distance, a.sub(a.mul(derivative[i], length2), a.mul(n[i], projection)))
			if a.err != nil {
				return du, dv, a.err
			}
			change, err := intervalDivide(exactInterval{num, num}, den, b)
			if err != nil {
				return du, dv, err
			}
			r.Lower[i], r.Upper[i] = a.add(base[i], change.lo), a.add(base[i], change.hi)
		}
		r.Bounds = Box{Min: rationalSurfaceVectorBounds(r.Lower).Min, Max: rationalSurfaceVectorBounds(r.Upper).Max}
	}
	if a.err != nil {
		return du, dv, a.err
	}
	if err := ctx.Err(); err != nil {
		return du, dv, err
	}
	vectors[0].ExactOperations, vectors[1].ExactOperations = b.operations, b.operations
	return vectors[0], vectors[1], nil
}

var _ Surface = (*VerifiedNormalOffsetPatch)(nil)

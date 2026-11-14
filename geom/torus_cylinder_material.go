package geom

import (
	"context"
	"math/big"
)

// TorusCylinderOperation selects regularized material in the proved transverse domain.
type TorusCylinderOperation uint8

const (
	TorusCylinderCommon TorusCylinderOperation = iota + 1
	TorusCylinderDifference
	TorusCylinderUnion
)
const (
	TorusCylinderTorus    BoreBoundaryMask = 1
	TorusCylinderWall     BoreBoundaryMask = 2
	TorusCylinderLowerCap BoreBoundaryMask = 4
	TorusCylinderUpperCap BoreBoundaryMask = 8
)

// TorusCylinderMaterial is an immutable origin-centered Z-axis circular ring
// torus (major R, minor r) combined with a coaxial finite cylinder.
// It is an analytic query, not a native BRep or manifold certificate.
type TorusCylinderMaterial struct {
	R, r, c, lower, upper *big.Rat
	op                    TorusCylinderOperation
}

// NewTorusCylinderMaterial requires R>r>0, R-r<c<R+r and lower<-r<r<upper.
// These exact strict inequalities prove two transverse common circles and that
// neither cylinder cap meets the torus. Tangencies and cap cuts fail closed.
func NewTorusCylinderMaterial(R, r, c, lower, upper float64, op TorusCylinderOperation) (TorusCylinderMaterial, error) {
	var z TorusCylinderMaterial
	if !finite(R) || !finite(r) || !finite(c) || !finite(lower) || !finite(upper) || r <= 0 || R <= r || c <= 0 || (op != TorusCylinderCommon && op != TorusCylinderDifference && op != TorusCylinderUnion) {
		return z, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	s := TorusCylinderMaterial{q(R), q(r), q(c), q(lower), q(upper), op}
	if s.c.Cmp(new(big.Rat).Sub(s.R, s.r)) <= 0 || s.c.Cmp(new(big.Rat).Add(s.R, s.r)) >= 0 || s.lower.Cmp(new(big.Rat).Neg(s.r)) >= 0 || s.upper.Cmp(s.r) <= 0 {
		return z, ErrDomain
	}
	return s, nil
}
func (s TorusCylinderMaterial) location(v []int) (BorePointLocation, BoreBoundaryMask) {
	if s.op == TorusCylinderUnion {
		cylinderClosed := v[1] <= 0 && v[2] <= 0 && v[3] <= 0
		cylinderInside := v[1] < 0 && v[2] < 0 && v[3] < 0
		if v[0] < 0 || cylinderInside {
			return BoreInside, 0
		}
		if v[0] > 0 && !cylinderClosed {
			return BoreOutside, 0
		}
		var m BoreBoundaryMask
		if v[0] == 0 {
			m |= TorusCylinderTorus
		}
		if cylinderClosed {
			for i, mask := range []BoreBoundaryMask{TorusCylinderWall, TorusCylinderLowerCap, TorusCylinderUpperCap} {
				if v[i+1] == 0 {
					m |= mask
				}
			}
		}
		return BoreBoundary, m
	}
	wall := v[1]
	if s.op == TorusCylinderDifference {
		wall = -wall
	}
	if v[0] > 0 || wall > 0 {
		return BoreOutside, 0
	}
	var m BoreBoundaryMask
	if v[0] == 0 {
		m |= TorusCylinderTorus
	}
	if wall == 0 {
		m |= TorusCylinderWall
	}
	if m != 0 {
		return BoreBoundary, m
	}
	return BoreInside, 0
}
func (s TorusCylinderMaterial) polynomials(ctx context.Context, p, d [3]*big.Rat, b *polynomialBudget) ([]rationalPolynomial, error) {
	a := sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, v := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper} {
		a.check(v)
	}
	for i := range p {
		a.check(p[i])
		a.check(d[i])
	}
	if a.err != nil {
		return nil, a.err
	}
	var sq [3]rationalPolynomial
	for i := range sq {
		v, e := polynomialProduct(rationalPolynomial{p[i], d[i]}, rationalPolynomial{p[i], d[i]}, b)
		if e != nil {
			return nil, e
		}
		sq[i] = v
	}
	coefficient := func(v rationalPolynomial, k int) *big.Rat {
		if k >= len(v) {
			return new(big.Rat)
		}
		return v[k]
	}
	radial := make(rationalPolynomial, 3)
	sum := make(rationalPolynomial, 3)
	R2 := a.mul(s.R, s.R)
	r2 := a.mul(s.r, s.r)
	c2 := a.mul(s.c, s.c)
	for k := 0; k < 3; k++ {
		radial[k] = a.add(coefficient(sq[0], k), coefficient(sq[1], k))
		sum[k] = a.add(radial[k], coefficient(sq[2], k))
	}
	sum[0] = a.add(sum[0], a.sub(R2, r2))
	if a.err != nil {
		return nil, a.err
	}
	torus, e := polynomialProduct(sum, sum, b)
	if e != nil {
		return nil, e
	}
	factor := a.mul(big.NewRat(4, 1), R2)
	for k := range radial {
		for len(torus) <= k {
			torus = append(torus, new(big.Rat))
		}
		torus[k] = a.sub(torus[k], a.mul(factor, radial[k]))
	}
	radial[0] = a.sub(radial[0], c2)
	// sum = rho²+z²+R²-r² > 0 everywhere. Therefore the squared
	// implicit polynomial has exactly the ring torus's inside/outside sign.
	if a.err != nil {
		return nil, a.err
	}
	out := []rationalPolynomial{trimPolynomial(torus), trimPolynomial(radial)}
	if s.op == TorusCylinderUnion {
		out = append(out, trimPolynomial(rationalPolynomial{a.sub(s.lower, p[2]), a.sub(new(big.Rat), d[2])}), trimPolynomial(rationalPolynomial{a.sub(p[2], s.upper), a.add(new(big.Rat), d[2])}))
	}
	if a.err != nil {
		return nil, a.err
	}
	return out, nil
}

// ClassifyRational preserves sub-float probes and exact two-owner rim ties.
func (s TorusCylinderMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	if ctx == nil || s.R == nil {
		return BoreOutside, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return BoreOutside, 0, e
	}
	for _, v := range p {
		if v == nil {
			return BoreOutside, 0, ErrInvalid
		}
	}
	limits, e := patchOptions(o)
	if e != nil {
		return BoreOutside, 0, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	return s.classifyWithBudget(ctx, p, b)
}
func (s TorusCylinderMaterial) classifyWithBudget(ctx context.Context, p [3]*big.Rat, b *polynomialBudget) (BorePointLocation, BoreBoundaryMask, error) {
	ps, e := s.polynomials(ctx, p, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, b)
	if e != nil {
		return BoreOutside, 0, e
	}
	v := make([]int, len(ps))
	for i := range ps {
		q, e := polynomialValue(ps[i], new(big.Rat), b)
		if e != nil {
			return BoreOutside, 0, e
		}
		v[i] = q.Sign()
	}
	l, m := s.location(v)
	if e := ctx.Err(); e != nil {
		return BoreOutside, 0, e
	}
	return l, m, nil
}

// TorusCylinderLine contains complete closed material intervals and isolated
// contacts; interior clipping endpoints have owner mask zero.
type TorusCylinderLine struct {
	Intervals []BoreBoundaryInterval
	Points    []BoreLineHit
}

// ClipRationalLine preserves raw rational t and exact quartic/quadratic owners,
// including tangencies and continuous cylindrical generator contact. Arithmetic
// building/isolation/signs share Surface's work budget. Root ordering/codec and
// enclosure helpers have the same per-proof limits: at most ten distinct
// events, 45 insertion comparisons and 18*MaxSubdivisions enclosure calls
// for Union (eight/28/14 for Common and Difference).
// ParameterTolerance affects brackets only. Every failure returns no result.
func (s TorusCylinderMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	var z TorusCylinderLine
	if ctx == nil || s.R == nil || min == nil || max == nil || min.Cmp(max) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return z, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return z, e
	}
	nonzero := false
	for i := range p {
		if p[i] == nil || d[i] == nil {
			return z, ErrInvalid
		}
		nonzero = nonzero || d[i].Sign() != 0
	}
	if !nonzero {
		return z, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return z, ErrInvalid
	}
	limits, e := patchOptions(o.Surface)
	if e != nil {
		return z, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	if e = b.check(min); e != nil {
		return z, e
	}
	if e = b.check(max); e != nil {
		return z, e
	}
	ps, e := s.polynomials(ctx, p, d, b)
	if e != nil {
		return z, e
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	out, e := intersectMaterialBoundaryRational(ctx, ps, min, max, o, b, ro, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		l, m := s.location(v)
		if l == BoreInside {
			l = BoreBoundary
		}
		return l, m
	})
	if e != nil {
		return z, e
	}
	if e = ctx.Err(); e != nil {
		return z, e
	}
	return TorusCylinderLine{out.Intervals, out.Hits}, nil
}

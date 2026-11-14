package geom

import (
	"context"
	"math/big"
)

// ObliqueCylinderTorusCavityMaterial retains a proper exact rational torus
// frame independent of the original +Z cylinder. No float-normalized native
// support is implied. Strict containment uses the radiusR+r enclosing sphere.
type ObliqueCylinderTorusCavityMaterial struct {
	source TorusCylinderMaterial
	pose   [3][4]*big.Rat
}

// NewObliqueCylinderTorusCavityMaterial proves a proper orthonormal rational
// frame and conservative strict radial/axial separation in one bounded ledger.
// Pose columns are torus X,Y,Z and center; inputs are deeply owned.
func NewObliqueCylinderTorusCavityMaterial(ctx context.Context, R, r, c, lower, upper float64, pose [3][4]*big.Rat, o SurfacePatchOptions) (ObliqueCylinderTorusCavityMaterial, error) {
	var zero ObliqueCylinderTorusCavityMaterial
	if ctx == nil {
		return zero, ErrInvalid
	}
	for _, v := range []float64{R, r, c, lower, upper} {
		if !finite(v) {
			return zero, ErrInvalid
		}
	}
	if R <= r || r <= 0 || c <= 0 {
		return zero, ErrInvalid
	}
	limits, e := patchOptions(o)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: budget}
	for _, row := range pose {
		for _, v := range row {
			if v == nil {
				return zero, ErrInvalid
			}
			a.check(v)
		}
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	source := TorusCylinderMaterial{q(R), q(r), q(c), q(lower), q(upper), TorusCylinderUnion}
	for _, v := range []*big.Rat{source.R, source.r, source.c, source.lower, source.upper} {
		a.check(v)
	}
	if a.err != nil {
		return zero, a.err
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			dot := new(big.Rat)
			for k := 0; k < 3; k++ {
				dot = a.add(dot, a.mul(pose[k][i], pose[k][j]))
			}
			want := int64(0)
			if i == j {
				want = 1
			}
			if a.err != nil {
				return zero, a.err
			}
			if dot.Cmp(big.NewRat(want, 1)) != 0 {
				return zero, ErrDomain
			}
		}
	}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det = a.add(det, a.mul(pose[0][i], a.sub(a.mul(pose[1][j], pose[2][k]), a.mul(pose[1][k], pose[2][j]))))
	}
	if a.err != nil {
		return zero, a.err
	}
	if det.Cmp(big.NewRat(1, 1)) != 0 {
		return zero, ErrDomain
	}
	extent := a.add(source.R, source.r)
	gap := a.sub(source.c, extent)
	d2 := a.add(a.mul(pose[0][3], pose[0][3]), a.mul(pose[1][3], pose[1][3]))
	radial := a.sub(a.mul(gap, gap), d2)
	low := a.sub(a.sub(pose[2][3], extent), source.lower)
	high := a.sub(source.upper, a.add(pose[2][3], extent))
	if a.err != nil {
		return zero, a.err
	}
	if gap.Sign() <= 0 || radial.Sign() <= 0 || low.Sign() <= 0 || high.Sign() <= 0 {
		return zero, ErrDomain
	}
	var owned [3][4]*big.Rat
	for i := range owned {
		for j := range owned[i] {
			owned[i][j] = new(big.Rat).Set(pose[i][j])
		}
	}
	return ObliqueCylinderTorusCavityMaterial{source, owned}, nil
}
func (s ObliqueCylinderTorusCavityMaterial) TorusFrame() ([3][4]*big.Rat, error) {
	var out [3][4]*big.Rat
	if s.source.R == nil {
		return out, ErrInvalid
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(s.pose[i][j])
		}
	}
	return out, nil
}
func (s ObliqueCylinderTorusCavityMaterial) polynomials(ctx context.Context, p, d [3]*big.Rat, b *polynomialBudget) ([]rationalPolynomial, error) {
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	var local, dir [3]*big.Rat
	for i := 0; i < 3; i++ {
		local[i], dir[i] = new(big.Rat), new(big.Rat)
		for j := 0; j < 3; j++ {
			a.check(s.pose[j][i])
			a.check(s.pose[j][3])
			local[i] = a.add(local[i], a.mul(s.pose[j][i], a.sub(p[j], s.pose[j][3])))
			dir[i] = a.add(dir[i], a.mul(s.pose[j][i], d[j]))
		}
	}
	if a.err != nil {
		return nil, a.err
	}
	original, e := s.source.polynomials(ctx, p, d, b)
	if e != nil {
		return nil, e
	}
	torus, e := s.source.polynomials(ctx, local, dir, b)
	if e != nil {
		return nil, e
	}
	original[0] = torus[0]
	return original, nil
}
func (s ObliqueCylinderTorusCavityMaterial) location(v []int) (BorePointLocation, BoreBoundaryMask) {
	if v[0] < 0 || v[1] > 0 || v[2] > 0 || v[3] > 0 {
		return BoreOutside, 0
	}
	var mask BoreBoundaryMask
	for i, x := range v {
		if x == 0 {
			mask |= BoreBoundaryMask(1 << i)
		}
	}
	if mask != 0 {
		return BoreBoundary, mask
	}
	return BoreInside, 0
}
func (s ObliqueCylinderTorusCavityMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	if ctx == nil || s.source.R == nil {
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
func (s ObliqueCylinderTorusCavityMaterial) classifyWithBudget(ctx context.Context, p [3]*big.Rat, b *polynomialBudget) (BorePointLocation, BoreBoundaryMask, error) {
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

func (s ObliqueCylinderTorusCavityMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	var z TorusCylinderLine
	if ctx == nil || s.source.R == nil || min == nil || max == nil || min.Cmp(max) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
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

// BoundAffineRange certifies original material extrema under any nonsingular
// rational affine map. The strictly internal independently tilted torus cannot
// attain an extreme; the original cylinder support proof is reused unchanged.
func (s ObliqueCylinderTorusCavityMaterial) BoundAffineRange(ctx context.Context, m [3][4]*big.Rat, bits int, o SurfacePatchOptions) (CylinderTorusCavityBounds, error) {
	if s.source.R == nil {
		return CylinderTorusCavityBounds{}, ErrInvalid
	}
	return (CylinderTorusCavityMaterial{source: s.source}).BoundAffineRange(ctx, m, bits, o)
}

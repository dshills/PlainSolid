package geom

import (
	"context"
	"math"
	"math/big"
)

// ObliqueSkewCylinderMap transports the orthogonal section constraints
// b²+y²<=ru² and a²+(y-h)²<=rv² to two oblique cylinders. Coordinates of
// its affine map are ordered (a,b,y). Axial bounds are conservative, not sharp
// for skew cylinders. They are measured about each closest-axis foot.
type ObliqueSkewCylinderMap struct {
	Complete bool
	// ProjectionRelation is -1 for empty, 0 for isolated tangency, +1 for
	// positive-width radial overlap. It does not classify boundary graph nodes.
	ProjectionRelation   int
	ClosestFeet          [2][3]*big.Rat
	ClosestRawParameters [2]*big.Rat
	SeparationSquared    *big.Rat
	SeparationSign       int
	SeparationBounds     [2]*big.Rat
	RawAxialHalfExtents  [2]CylinderAxialExtent
	ExactOperations      int
	coefficients         [3][3]*big.Rat
	signs                [3][3]int
	foot                 [3]*big.Rat
	numerical            Affine
}

// NewObliqueSkewCylinderMap uses exact represented origin and axis data without
// assuming equal norms, orthogonality, or an already perpendicular displacement.
// The separation is signed in the direction u cross v. Numerical evaluation
// rounds algebraic coefficients; RepresentedAffine instead fails unless every
// coefficient is exactly representable in binary64.
func NewObliqueSkewCylinderMap(ctx context.Context, originU, u Vec3, ru float64, originV, v Vec3, rv float64, o SurfacePatchEvaluationOptions) (out ObliqueSkewCylinderMap, err error) {
	if !originU.IsFinite() || !originV.IsFinite() {
		return out, ErrInvalid
	}
	p, e := ClassifyCylinderCommonCaps(ctx, u, v, ru, rv, nil, nil, o)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: p.ExactOperations}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Complete = false
		}
	}()
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	quo := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
	U, V, d, W := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	N, M, C, du, dv := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		U[i] = rat(coordinate(u, i))
		V[i] = rat(coordinate(v, i))
		d[i] = sub(rat(coordinate(originV, i)), rat(coordinate(originU, i)))
		N.Add(N, mul(U[i], U[i]))
		M.Add(M, mul(V[i], V[i]))
		C.Add(C, mul(U[i], V[i]))
		du.Add(du, mul(d[i], U[i]))
		dv.Add(dv, mul(d[i], V[i]))
	}
	D := sub(mul(N, M), mul(C, C))
	s := quo(sub(mul(M, du), mul(C, dv)), D)
	t := quo(sub(mul(C, du), mul(N, dv)), D)
	triple := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		W[i] = sub(mul(U[j], V[k]), mul(U[k], V[j]))
		triple.Add(triple, mul(d[i], W[i]))
		out.ClosestFeet[0][i] = new(big.Rat).Add(rat(coordinate(originU, i)), mul(s, U[i]))
		out.ClosestFeet[1][i] = new(big.Rat).Add(rat(coordinate(originV, i)), mul(t, V[i]))
		out.foot[i] = new(big.Rat).Set(out.ClosestFeet[0][i])
		out.numerical[i][3], _ = out.foot[i].Float64()
	}
	out.ClosestRawParameters = [2]*big.Rat{s, t}
	out.SeparationSquared = quo(mul(triple, triple), D)
	out.SeparationSign = triple.Sign()
	sum := new(big.Rat).Add(rat(ru), rat(rv))
	out.ProjectionRelation = -out.SeparationSquared.Cmp(mul(sum, sum))
	out.RawAxialHalfExtents = p.Extents
	for _, q := range []*big.Rat{N, M, C, D, s, t, out.SeparationSquared} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	lo, hi, e := rationalSqrtBounds(out.SeparationSquared, 128, budget)
	if e != nil {
		return out, e
	}
	if triple.Sign() < 0 {
		lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
	}
	out.SeparationBounds = [2]*big.Rat{lo, hi}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			coefficient := U[i]
			scale := M
			if j == 1 {
				coefficient = V[i]
				scale = N
			} else if j == 2 {
				coefficient = W[i]
				scale = big.NewRat(1, 1)
			}
			sq := quo(mul(mul(coefficient, coefficient), scale), D)
			if e := budget.check(sq); e != nil {
				return out, e
			}
			out.coefficients[i][j] = sq
			out.signs[i][j] = coefficient.Sign()
			f, _ := sq.Float64()
			out.numerical[i][j] = float64(coefficient.Sign()) * math.Sqrt(f)
			if !finite(out.numerical[i][j]) || (sq.Sign() > 0 && out.numerical[i][j] == 0) {
				return out, ErrUnrepresentable
			}
		}
		if !finite(out.numerical[i][3]) {
			return out, ErrUnrepresentable
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	return out, nil
}

// Eval numerically maps section (a,b,y). It does not check material membership.
func (p ObliqueSkewCylinderMap) Eval(a, b, y float64) (Vec3, error) {
	if p.foot[0] == nil || !finite(a) || !finite(b) || !finite(y) {
		return Vec3{}, ErrInvalid
	}
	q := p.numerical.Apply(Vec3{X: a, Y: b, Z: y})
	if !q.IsFinite() {
		return Vec3{}, ErrConvergence
	}
	return q, nil
}

// RepresentedAffine returns a lossless exact map, or ErrUnrepresentable.
func (p ObliqueSkewCylinderMap) RepresentedAffine() (out Affine, err error) {
	if p.foot[0] == nil {
		return out, ErrInvalid
	}
	for i := 0; i < 3; i++ {
		out[i][3], err = RepresentedExactScalar(p.foot[i])
		if err != nil {
			return Affine{}, err
		}
		for j := 0; j < 3; j++ {
			q := p.coefficients[i][j]
			n := new(big.Int).Sqrt(q.Num())
			d := new(big.Int).Sqrt(q.Denom())
			if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
				return Affine{}, ErrUnrepresentable
			}
			r := new(big.Rat).SetFrac(n, d)
			if p.signs[i][j] < 0 {
				r.Neg(r)
			}
			out[i][j], err = RepresentedExactScalar(r)
			if err != nil {
				return Affine{}, err
			}
		}
	}
	return out, nil
}

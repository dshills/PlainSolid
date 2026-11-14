package geom

import (
	"context"
	"math/big"
)

// NewRationalCylindricalSpan constructs one exact quadratic/linear rational
// cylinder chart by axial extrusion of a rational quadratic circular arc.
// The frame must be an exact signed-coordinate frame. Arc controls use local
// homogeneous (w*x,w*y,w) coordinates and z0,z1 are local axial coordinates.
// The chart is normalized on [0,1]² and preserves original arc traversal.
// The complete circle equation and nonzero angular derivative are verified
// exactly. Binary64 control storage must be lossless; otherwise it rejects.
func NewRationalCylindricalSpan(ctx context.Context, frame Frame, center [2]*big.Rat, radius, z0, z1 *big.Rat, arc [3][3]*big.Rat) (*BSplineSurface, error) {
	if e := proveRationalCylindricalSpan(ctx, frame, center, radius, z0, z1, arc); e != nil {
		return nil, e
	}
	poles := make([]Vec3, 0, 6)
	weights := make([]float64, 0, 6)
	for _, p := range arc {
		w, e := RepresentedExactScalar(p[2])
		if e != nil {
			return nil, e
		}
		for _, z := range []*big.Rat{z0, z1} {
			local := [3]*big.Rat{new(big.Rat).Quo(p[0], p[2]), new(big.Rat).Quo(p[1], p[2]), z}
			t := [3]float64{}
			for i, a := range []float64{frame.origin.X, frame.origin.Y, frame.origin.Z} {
				x := new(big.Rat).SetFloat64(a)
				for j, f := range []Vec3{frame.x, frame.y, frame.z} {
					coef := []float64{f.X, f.Y, f.Z}[i]
					if coef != 0 {
						x.Add(x, new(big.Rat).Mul(local[j], new(big.Rat).SetFloat64(coef)))
					}
				}
				t[i], e = RepresentedExactScalar(x)
				if e != nil {
					return nil, e
				}
			}
			poles = append(poles, Vec3{t[0], t[1], t[2]})
			weights = append(weights, w)
		}
	}
	return NewBSplineSurfaceWithExactWeights(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, weights)
}

// NewExactRationalCylindricalSpanPatch proves the same circular extrusion as
// NewRationalCylindricalSpan but keeps all homogeneous coordinates rational.
// No Euclidean pole or positive weight must be representable as float64.
func NewExactRationalCylindricalSpanPatch(ctx context.Context, frame Frame, center [2]*big.Rat, radius, z0, z1 *big.Rat, arc [3][3]*big.Rat) (*SurfacePatch, error) {
	if e := proveRationalCylindricalSpan(ctx, frame, center, radius, z0, z1, arc); e != nil {
		return nil, e
	}
	var controls [6][4]*big.Rat
	for i, p := range arc {
		for level, z := range [2]*big.Rat{z0, z1} {
			local := [3]*big.Rat{p[0], p[1], new(big.Rat).Mul(z, p[2])}
			h := &controls[2*i+level]
			for axis, origin := range [3]float64{frame.origin.X, frame.origin.Y, frame.origin.Z} {
				x := new(big.Rat).Mul(new(big.Rat).SetFloat64(origin), p[2])
				for j, f := range [3]Vec3{frame.x, frame.y, frame.z} {
					coef := [3]float64{f.X, f.Y, f.Z}[axis]
					if coef != 0 {
						x.Add(x, new(big.Rat).Mul(local[j], new(big.Rat).SetFloat64(coef)))
					}
				}
				h[axis] = x
			}
			h[3] = p[2]
		}
	}
	return NewExactHomogeneousBezierSurface(ctx, 2, 1, controls[:], SurfacePatchOptions{MaxOperations: 4096, MaxCoefficientBits: 4096})
}

func proveRationalCylindricalSpan(ctx context.Context, frame Frame, center [2]*big.Rat, radius, z0, z1 *big.Rat, arc [3][3]*big.Rat) error {
	if ctx == nil || !frame.valid() || center[0] == nil || center[1] == nil || radius == nil || z0 == nil || z1 == nil || radius.Sign() <= 0 || z0.Cmp(z1) >= 0 {
		return ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	cardinal := func(v Vec3) bool {
		count := 0
		for _, x := range []float64{v.X, v.Y, v.Z} {
			if x != 0 {
				if x != 1 && x != -1 {
					return false
				}
				count++
			}
		}
		return count == 1
	}
	if !cardinal(frame.x) || !cardinal(frame.y) || !cardinal(frame.z) {
		return ErrDomain
	}
	b := &polynomialBudget{maxOperations: 4096, maxBits: 4096}
	for _, v := range []*big.Rat{center[0], center[1], radius, z0, z1} {
		if e := b.check(v); e != nil {
			return e
		}
	}
	var h [3]rationalPolynomial
	for j := range h {
		p := make([]*big.Rat, 3)
		for i := range p {
			if arc[i][j] == nil || arc[i][2] == nil || arc[i][2].Sign() <= 0 {
				return ErrInvalid
			}
			if e := b.check(arc[i][j]); e != nil {
				return e
			}
			p[i] = arc[i][j]
			if j < 2 {
				p[i] = new(big.Rat).Sub(p[i], new(big.Rat).Mul(center[j], arc[i][2]))
			}
		}
		x, e := bernsteinToPower(p, b)
		if e != nil {
			return e
		}
		h[j] = x
	}
	squared := [3]rationalPolynomial{}
	for i, v := range h {
		x, e := polynomialProduct(v, v, b)
		if e != nil {
			return e
		}
		squared[i] = x
	}
	r2 := new(big.Rat).Mul(radius, radius)
	for i := 0; i < 5; i++ {
		z := new(big.Rat)
		for j, x := range squared {
			if i >= len(x) {
				continue
			}
			if j == 2 {
				z.Sub(z, new(big.Rat).Mul(r2, x[i]))
			} else {
				z.Add(z, x[i])
			}
		}
		if z.Sign() != 0 {
			return ErrDomain
		}
	}
	a, e := polynomialProduct(h[0], polynomialDerivative(h[1]), b)
	if e != nil {
		return e
	}
	c, e := polynomialProduct(h[1], polynomialDerivative(h[0]), b)
	if e != nil {
		return e
	}
	cross := make(rationalPolynomial, max(len(a), len(c)))
	for i := range cross {
		cross[i] = new(big.Rat)
		if i < len(a) {
			cross[i].Set(a[i])
		}
		if i < len(c) {
			cross[i].Sub(cross[i], c[i])
		}
	}
	cross = trimPolynomial(cross)
	if len(cross) == 0 || len(cross) > 3 {
		return ErrConvergence
	}
	sign := 0
	for i := 0; i < 3; i++ {
		z := new(big.Rat)
		for j := 0; j <= i && j < len(cross); j++ {
			n, d := new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(2, int64(j))
			z.Add(z, new(big.Rat).Mul(cross[j], new(big.Rat).SetFrac(n, d)))
		}
		if z.Sign() == 0 || sign != 0 && z.Sign() != sign {
			return ErrConvergence
		}
		sign = z.Sign()
	}
	// Distinct endpoint directions in the same closed semicircle: the
	// control hull lies in the half-plane of their nonzero radial sum.
	axis := [2]*big.Rat{}
	for j := range axis {
		x := new(big.Rat).Quo(arc[0][j], arc[0][2])
		y := new(big.Rat).Quo(arc[2][j], arc[2][2])
		axis[j] = new(big.Rat).Sub(new(big.Rat).Add(x, y), new(big.Rat).Mul(big.NewRat(2, 1), center[j]))
	}
	for _, p := range arc {
		v := new(big.Rat)
		for j := range axis {
			v.Add(v, new(big.Rat).Mul(axis[j], new(big.Rat).Sub(p[j], new(big.Rat).Mul(center[j], p[2]))))
		}
		if v.Sign() <= 0 {
			return ErrConvergence
		}
	}
	return nil
}

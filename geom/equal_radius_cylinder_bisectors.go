package geom

import (
	"context"
	"math"
	"math/big"
)

// CylinderBisectorEllipse lies in PlaneNormal·(point-center)=0 and has
// point=center+s*BasisB+t*BasisW, CoefficientB*s²+CoefficientW*t²=RadiusSquared.
// Both coefficients are strictly positive exact rationals. Eval uses private
// numerical representatives; exported coefficient mutation cannot alter it.
type CylinderBisectorEllipse struct {
	PlaneNormal, BasisB, BasisW               [3]*big.Rat
	CoefficientB, CoefficientW, RadiusSquared *big.Rat
	center, b, w                              Vec3
	amplitudeB, amplitudeW                    float64
}

// EqualCylinderBisectorNode is center+RootSign*sqrt(Radicand)*Vector. Both
// ellipse curves pass through each node with distinct tangents, while the
// original cylinder surface normals agree there.
type EqualCylinderBisectorNode struct {
	Center, Vector [3]*big.Rat
	Radicand       *big.Rat
	RootSign       int
	Bounds         [3][2]*big.Rat
}
type EqualRadiusCylinderBisectors struct {
	Complete        bool
	Ellipses        [2]CylinderBisectorEllipse
	Nodes           [2]EqualCylinderBisectorNode
	ExactOperations int
}

// IntersectEqualRadiusCylinderBisectors factors two infinite equal-radius
// cylinders with a COMMON represented centerline crossing. Axes must be
// nonparallel and have equal exact squared norms; they need not be unit or
// orthogonal. The proof uses (u·p)²=(v·p)², retaining represented raw axes and
// rational bisector coefficients. Unequal norms are explicitly unsupported.
func IntersectEqualRadiusCylinderBisectors(ctx context.Context, center, u, v Vec3, radius float64, o SurfacePatchEvaluationOptions) (out EqualRadiusCylinderBisectors, err error) {
	if ctx == nil || !center.IsFinite() || !u.IsFinite() || !v.IsFinite() || u == (Vec3{}) || v == (Vec3{}) || !finite(radius) || radius <= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Complete = false
			out.Ellipses = [2]CylinderBisectorEllipse{}
			out.Nodes = [2]EqualCylinderBisectorNode{}
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	U, V, W := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	for i := 0; i < 3; i++ {
		U[i] = r(coordinate(u, i))
		V[i] = r(coordinate(v, i))
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, new(big.Rat).Mul(a[i], b[i]))
		}
		return q
	}
	N := dot(U, U)
	if N.Cmp(dot(V, V)) != 0 {
		return out, ErrDomain
	}
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		W[i] = new(big.Rat).Sub(new(big.Rat).Mul(U[j], V[k]), new(big.Rat).Mul(U[k], V[j]))
	}
	W2 := dot(W, W)
	if W2.Sign() == 0 {
		return out, ErrDegenerate
	}
	R2 := new(big.Rat).Mul(r(radius), r(radius))
	for _, q := range []*big.Rat{N, W2, R2} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	toVec := func(a [3]*big.Rat) (Vec3, error) {
		var x [3]float64
		for i := 0; i < 3; i++ {
			x[i], _ = a[i].Float64()
			if !finite(x[i]) {
				return Vec3{}, ErrUnrepresentable
			}
		}
		return Vec3{X: x[0], Y: x[1], Z: x[2]}, nil
	}
	for n, sign := range []int64{-1, 1} {
		q := CylinderBisectorEllipse{center: center, RadiusSquared: new(big.Rat).Set(R2), CoefficientW: new(big.Rat).Set(W2)}
		for i := 0; i < 3; i++ {
			q.PlaneNormal[i] = new(big.Rat).Add(U[i], new(big.Rat).Mul(big.NewRat(sign, 1), V[i]))
			q.BasisB[i] = new(big.Rat).Sub(U[i], new(big.Rat).Mul(big.NewRat(sign, 1), V[i]))
			q.BasisW[i] = new(big.Rat).Set(W[i])
		}
		UB := dot(U, q.BasisB)
		q.CoefficientB = new(big.Rat).Sub(dot(q.BasisB, q.BasisB), new(big.Rat).Quo(new(big.Rat).Mul(UB, UB), N))
		if q.CoefficientB.Sign() <= 0 {
			return out, ErrDegenerate
		}
		if e := budget.check(q.CoefficientB); e != nil {
			return out, e
		}
		var e error
		q.b, e = toVec(q.BasisB)
		if e != nil {
			return out, e
		}
		q.w, e = toVec(q.BasisW)
		if e != nil {
			return out, e
		}
		rb, _ := new(big.Rat).Quo(R2, q.CoefficientB).Float64()
		rw, _ := new(big.Rat).Quo(R2, W2).Float64()
		q.amplitudeB, q.amplitudeW = math.Sqrt(rb), math.Sqrt(rw)
		if !finite(q.amplitudeB) || !finite(q.amplitudeW) || q.amplitudeB <= 0 || q.amplitudeW <= 0 {
			return out, ErrUnrepresentable
		}
		out.Ellipses[n] = q
	}
	nodeRad := new(big.Rat).Quo(R2, W2)
	lo, hi, e := rationalSqrtBounds(nodeRad, 128, budget)
	if e != nil {
		return out, e
	}
	for n, sign := range []int{-1, 1} {
		q := EqualCylinderBisectorNode{Radicand: new(big.Rat).Set(nodeRad), RootSign: sign}
		for i := 0; i < 3; i++ {
			q.Center[i] = r(coordinate(center, i))
			q.Vector[i] = new(big.Rat).Set(W[i])
			factor := new(big.Rat).Mul(big.NewRat(int64(sign), 1), W[i])
			a, b := lo, hi
			if factor.Sign() < 0 {
				a, b = b, a
			}
			q.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(q.Center[i], new(big.Rat).Mul(factor, a)), new(big.Rat).Add(q.Center[i], new(big.Rat).Mul(factor, b))}
			for _, value := range q.Bounds[i] {
				if e := budget.check(value); e != nil {
					return out, e
				}
			}
		}
		out.Nodes[n] = q
	}
	out.Complete = true
	return out, nil
}

// Eval uses angular parameter radians; the shared positive/negative nodes
// occur at pi/2 and3pi/2 respectively. The entire ellipse is regular.
func (p CylinderBisectorEllipse) Eval(angle float64) (Vec3, error) {
	if p.amplitudeB <= 0 || !finite(angle) {
		return Vec3{}, ErrInvalid
	}
	s, c := math.Sincos(angle)
	q := p.center.Add(p.b.Scale(p.amplitudeB * c)).Add(p.w.Scale(p.amplitudeW * s))
	if !q.IsFinite() {
		return Vec3{}, ErrConvergence
	}
	return q, nil
}
func (p CylinderBisectorEllipse) Derivative(angle float64) (Vec3, error) {
	if p.amplitudeB <= 0 || !finite(angle) {
		return Vec3{}, ErrInvalid
	}
	s, c := math.Sincos(angle)
	q := p.b.Scale(-p.amplitudeB * s).Add(p.w.Scale(p.amplitudeW * c))
	if !q.IsFinite() {
		return Vec3{}, ErrConvergence
	}
	return q, nil
}

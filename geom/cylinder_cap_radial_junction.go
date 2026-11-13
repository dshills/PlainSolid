package geom

import (
	"context"
	"math/big"
)

// CylinderCapRadialJunctions reports all contacts of A's selected cap perimeter
// with B's infinite radial wall. Opposite axial cap restrictions are NOT applied.
// Curves are four positive-weight rational quarter charts, ordered CCW about
// A.Axis, on [0,1]. Shared cardinal endpoints appear in both incident charts;
// their exact model points provide identity. Coincident arcs remain explicit.
type CylinderCapRadialJunctions struct {
	Complete                 bool
	Curves                   [4]*ExactSplineCurve
	Contacts                 [4]ExactCurveQuadricIntersection
	Operations, Subdivisions int
}

// IntersectCylinderCapRadialJunctions requires the cap's canonical rational
// perpendicular basis to have rational lengths. Unsupported irrational frames
// return ErrUnrepresentable; no rounded circle or support equation is used.
func IntersectCylinderCapRadialJunctions(ctx context.Context, a, b RawCylinder, maxCap bool, o ExactCurveSegmentOptions) (out CylinderCapRadialJunctions, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return out, ErrInvalid
	}
	region, e := DescribeCylinderCapRegion(ctx, a, b, maxCap, SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	out.Operations = region.ExactOperations
	defer func() {
		if err != nil {
			out.Complete = false
		}
	}()
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: out.Operations}
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	basis := [2][3]*big.Rat{region.U, region.V}
	for k := 0; k < 2; k++ {
		sq := new(big.Rat)
		for _, v := range basis[k] {
			sq.Add(sq, mul(v, v))
		}
		n, d := new(big.Int).Sqrt(sq.Num()), new(big.Int).Sqrt(sq.Denom())
		if new(big.Int).Mul(n, n).Cmp(sq.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(sq.Denom()) != 0 {
			return out, ErrUnrepresentable
		}
		length := new(big.Rat).SetFrac(n, d)
		for i := 0; i < 3; i++ {
			basis[k][i] = new(big.Rat).Quo(mul(basis[k][i], rat(a.Radius)), length)
			if e := budget.check(basis[k][i]); e != nil {
				return out, e
			}
		}
	}
	pair, e := NewCylinderPairCommon(a, b)
	if e != nil {
		return out, e
	}
	B := pair.s[1]
	dot, originsq := new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		dot.Add(dot, mul(B.origin[i], B.axis[i]))
		originsq.Add(originsq, mul(B.origin[i], B.origin[i]))
	}
	var quad ExactQuadric
	for i := 0; i < 3; i++ {
		quad[i] = new(big.Rat).Sub(B.norm2, mul(B.axis[i], B.axis[i]))
		quad[6+i] = mul(big.NewRat(2, 1), new(big.Rat).Sub(mul(B.axis[i], dot), mul(B.norm2, B.origin[i])))
	}
	for k, ij := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		quad[3+k] = mul(big.NewRat(-2, 1), mul(B.axis[ij[0]], B.axis[ij[1]]))
	}
	quad[9] = new(big.Rat).Sub(mul(B.norm2, new(big.Rat).Sub(originsq, B.radius2)), mul(dot, dot))
	for _, q := range quad {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	out.Operations = budget.operations
	// Quarter chart homogeneous planar controls (1,0,1),(1,1,1),(0,2,2).
	for quarter := 0; quarter < 4; quarter++ {
		controls := make([][4]*big.Rat, 3)
		for k, local := range [][3]int64{{1, 0, 1}, {1, 1, 1}, {0, 2, 2}} {
			x, y := local[0], local[1]
			for j := 0; j < quarter; j++ {
				x, y = -y, x
			}
			w := big.NewRat(local[2], 1)
			for i := 0; i < 3; i++ {
				controls[k][i] = new(big.Rat).Add(mul(w, region.Origin[i]), new(big.Rat).Add(mul(big.NewRat(x, 1), basis[0][i]), mul(big.NewRat(y, 1), basis[1][i])))
			}
			controls[k][3] = w
		}
		remaining := o.MaxOperations - out.Operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		curve, e := NewExactHomogeneousBezier(ctx, controls, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		out.Operations += curve.operations
		remaining = o.MaxOperations - out.Operations
		subdiv := o.MaxSubdivisions - out.Subdivisions
		if remaining < 1 || subdiv < 1 {
			return out, ErrConvergence
		}
		local := o
		local.MaxOperations = remaining
		local.MaxSubdivisions = subdiv
		hits, e := IntersectExactCurveQuadric(ctx, curve, quad, ExactCurveQuadricOptions{ExactCurveSegmentOptions: local})
		out.Operations += hits.Operations
		out.Subdivisions += hits.Subdivisions
		if e != nil {
			return out, e
		}
		out.Curves[quarter] = curve
		out.Contacts[quarter] = hits
	}
	out.Complete = true
	return out, nil
}

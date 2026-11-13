package geom

import (
	"context"
	"math/big"
)

// SplineGraphAxisLine proves the entire contact set is one closed UV line
// Parameters[Axis]=Coordinate. LowerSide/UpperSide give strict A.Z-B.Z signs
// on either side, including the remaining original rectangle boundary.
// Positive homogeneous denominators preserve these signs. Graph excludes
// off-diagonal UV contacts; no shell topology is inferred.
type SplineGraphAxisLine struct {
	Proven               bool
	Axis                 int
	Coordinate           *big.Rat
	LowerSide, UpperSide int
	Domain               ParameterRectangle
	Endpoints            [2][2]*big.Rat
	OnKnot               bool
	Graph                SurfaceInjectivity
	CertifiedCells       []ParameterRectangle
	ExactOperations      int
}

// CertifySplineGraphAxisLine proves an exact rational contact coordinate from
// homogeneous Bernstein identities. On every union-knot cell the height
// numerator must be affine in Axis, with strictly same-signed Bernstein slope
// coefficients in the other axis; all coefficient lines must share one zero.
// This sufficient subset includes non-knot/nonbinary coordinates and variable
// positive denominators. Higher-degree or vanishing slope cases stay unresolved.
func CertifySplineGraphAxisLine(ctx context.Context, a, c *SurfacePatch, axis int, o GraphPointContactOptions) (r SplineGraphAxisLine, err error) {
	r.Axis = axis
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	r.Domain = difference.Domain()
	var root *big.Rat
	sign := 0
	for _, p := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		d := p.Domain()
		count, other := p.countU, p.countV
		lo, hi := d.UMin, d.UMax
		if axis == 1 {
			count, other = p.countV, p.countU
			lo, hi = d.VMin, d.VMax
		}
		if count < 2 {
			return r, ErrConvergence
		}
		at := func(i, j int) *big.Rat {
			if axis == 0 {
				return p.net[i*p.countV+j][2]
			}
			return p.net[j*p.countV+i][2]
		}
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))
		for j := 0; j < other; j++ {
			first, last := at(0, j), at(count-1, j)
			delta := new(big.Rat).Sub(last, first)
			if e := b.check(delta); e != nil {
				return r, e
			}
			if delta.Sign() == 0 {
				return r, ErrConvergence
			}
			if sign == 0 {
				sign = delta.Sign()
			} else if sign != delta.Sign() {
				return r, ErrConvergence
			}
			candidate := new(big.Rat).Sub(new(big.Rat).SetFloat64(lo), new(big.Rat).Quo(new(big.Rat).Mul(first, width), delta))
			if e := b.check(candidate); e != nil {
				return r, e
			}
			if root == nil {
				root = new(big.Rat).Set(candidate)
			} else if root.Cmp(candidate) != 0 {
				return r, ErrConvergence
			}
			for i := 1; i < count-1; i++ {
				expected := new(big.Rat).Add(first, new(big.Rat).Mul(big.NewRat(int64(i), int64(count-1)), delta))
				if e := b.check(expected); e != nil {
					return r, e
				}
				if expected.Cmp(at(i, j)) != 0 {
					return r, ErrConvergence
				}
			}
		}
		r.CertifiedCells = append(r.CertifiedCells, d)
	}
	if root == nil {
		return r, ErrConvergence
	}
	d := r.Domain
	bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	if root.Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) <= 0 || root.Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) >= 0 {
		return r, ErrConvergence
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	knots := difference.knotsU
	if axis == 1 {
		knots = difference.knotsV
	}
	for _, knot := range knots {
		r.OnKnot = r.OnKnot || root.Cmp(new(big.Rat).SetFloat64(knot)) == 0
	}
	r.Coordinate = new(big.Rat).Set(root)
	r.LowerSide, r.UpperSide = -sign, sign
	for endpoint := 0; endpoint < 2; endpoint++ {
		r.Endpoints[endpoint][axis] = new(big.Rat).Set(root)
		r.Endpoints[endpoint][1-axis] = new(big.Rat).SetFloat64(bounds[1-axis][endpoint])
	}
	r.Proven = true
	return r, nil
}

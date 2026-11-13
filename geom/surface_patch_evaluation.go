package geom

import "math/big"

// SurfacePatchJet contains nearest-rounded evaluations of exact rational point
// and first derivatives, together with outward component boxes. ExactOperations
// includes knot-piece extraction. Derivatives select the right-hand interior
// knot piece and left-hand final endpoint; continuity is not asserted.
type SurfacePatchJet struct {
	Point, Du, Dv                   Vec3
	PointBounds, DuBounds, DvBounds Box
	ExactOperations                 int
}

// EvaluateJet evaluates immutable exact patch coefficients at the exact binary
// rational UV values. No float control-net reconstruction is involved. Work uses
// the patch's operation/bit limits. Numerical values are nearest float64; their
// boxes retain outward bounds. Nonfinite numerical values return ErrInvalid.
func (p *SurfacePatch) EvaluateJet(u, v float64) (result SurfacePatchJet, err error) {
	if p == nil || len(p.net) == 0 {
		return result, ErrInvalid
	}
	d := p.Domain()
	if !finite(u) || !finite(v) || u < d.UMin || u > d.UMax || v < d.VMin || v > d.VMax {
		return result, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	current := p
	for {
		t, axis, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, axis, budget)
		if e != nil {
			return result, e
		}
		parameter := u
		if !axis {
			parameter = v
		}
		if parameter < t {
			current = left
		} else {
			current = right
		}
	}
	point, derivative, e := exactSurfaceJet(current, new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), budget)
	if e != nil {
		return result, e
	}
	for field := 0; field < 3; field++ {
		var vector Vec3
		var bounds Box
		for axis := 0; axis < 3; axis++ {
			exact := point[axis]
			if field > 0 {
				exact = derivative[axis][field-1]
			}
			nearest, _ := exact.Float64()
			lo, hi := rationalFloatBound(exact, true), rationalFloatBound(exact, false)
			switch axis {
			case 0:
				vector.X, bounds.Min.X, bounds.Max.X = nearest, lo, hi
			case 1:
				vector.Y, bounds.Min.Y, bounds.Max.Y = nearest, lo, hi
			case 2:
				vector.Z, bounds.Min.Z, bounds.Max.Z = nearest, lo, hi
			}
		}
		switch field {
		case 0:
			result.Point, result.PointBounds = vector, bounds
		case 1:
			result.Du, result.DuBounds = vector, bounds
		case 2:
			result.Dv, result.DvBounds = vector, bounds
		}
	}
	if !result.Point.IsFinite() || !result.Du.IsFinite() || !result.Dv.IsFinite() {
		return result, ErrInvalid
	}
	return result, nil
}

// Eval returns the nearest-rounded exact point at represented UV parameters.
func (p *SurfacePatch) Eval(u, v float64) (Vec3, error) {
	if p == nil || len(p.net) == 0 {
		return Vec3{}, ErrInvalid
	}
	d := p.Domain()
	if !finite(u) || !finite(v) || u < d.UMin || u > d.UMax || v < d.VMin || v > d.VMax {
		return Vec3{}, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	exact, e := exactPatchPointAt(p, u, v, budget)
	if e != nil {
		return Vec3{}, e
	}
	x, _ := exact[0].Float64()
	y, _ := exact[1].Float64()
	z, _ := exact[2].Float64()
	point := Vec3{x, y, z}
	if !point.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return point, nil
}

// Partials returns nearest-rounded exact one-sided first derivatives. Use
// EvaluateJet when outward derivative enclosures or work diagnostics are needed.
func (p *SurfacePatch) Partials(u, v float64) (Vec3, Vec3, error) {
	jet, e := p.EvaluateJet(u, v)
	return jet.Du, jet.Dv, e
}

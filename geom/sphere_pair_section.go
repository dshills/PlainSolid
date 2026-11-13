package geom

import (
	"context"
	"math/big"
)

// SphereSectionSupport retains original XY center and exact radius squared.
// Negative radius squared means no section; zero is a single support pole.
type SphereSectionSupport struct {
	Center        [2]*big.Rat
	RadiusSquared *big.Rat
	Source        BoreBoundaryMask
}

// SphereSectionContact is Base+Sign*sqrt(Radicand)*Vector. All rationals are
// caller-owned. Location/Boundary refer to the original 3D regularized material,
// not to a separately regularized planar set. Incidence retains hidden sources.
type SphereSectionContact struct {
	Base, Vector        [3]*big.Rat
	Radicand            *big.Rat
	Sign                int
	Bounds              [3]ExactParameterInterval
	Location            BorePointLocation
	Boundary, Incidence BoreBoundaryMask
}

// SpherePairSection describes complete original support-circle intersection
// events and isolated support poles; it does not assemble exposed planar arcs.
type SpherePairSection struct {
	Height             *big.Rat
	Supports           [2]SphereSectionSupport
	Contacts           []SphereSectionContact
	CoincidentCircle   bool
	CoincidentLocation BorePointLocation
	CoincidentBoundary BoreBoundaryMask
}

// HorizontalSection uses represented height and exact rational squared radii.
// No binary representability of circle radii or contact points is required.
// Empty/tangent regularized material remains distinguished from source contact.
func (s SpherePairMaterial) HorizontalSection(ctx context.Context, height float64, o SurfacePatchEvaluationOptions) (SpherePairSection, error) {
	var zero SpherePairSection
	if ctx == nil || s.radius2[0] == nil || !finite(height) {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return zero, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	out := SpherePairSection{Height: new(big.Rat).SetFloat64(height)}
	if e := budget.check(out.Height); e != nil {
		return zero, e
	}
	for i := 0; i < 2; i++ {
		for _, v := range s.center[i] {
			if e := budget.check(v); e != nil {
				return zero, e
			}
		}
		dz := new(big.Rat).Sub(out.Height, s.center[i][2])
		r2 := new(big.Rat).Sub(s.radius2[i], new(big.Rat).Mul(dz, dz))
		if e := budget.check(r2); e != nil {
			return zero, e
		}
		out.Supports[i] = SphereSectionSupport{Center: [2]*big.Rat{new(big.Rat).Set(s.center[i][0]), new(big.Rat).Set(s.center[i][1])}, RadiusSquared: r2, Source: 1 << uint(i)}
	}
	emit := func(base, vector [3]*big.Rat, rad *big.Rat, sign int, signs []int, inc BoreBoundaryMask) error {
		lo, hi, e := rationalSqrtBounds(rad, 96, budget)
		if e != nil {
			return e
		}
		p := SphereSectionContact{Radicand: new(big.Rat).Set(rad), Sign: sign, Incidence: inc}
		p.Location, p.Boundary = s.location(signs)
		for j := 0; j < 3; j++ {
			p.Base[j] = new(big.Rat).Set(base[j])
			p.Vector[j] = new(big.Rat).Set(vector[j])
			v := new(big.Rat).Mul(vector[j], big.NewRat(int64(sign), 1))
			a := new(big.Rat).Add(base[j], new(big.Rat).Mul(v, lo))
			b := new(big.Rat).Add(base[j], new(big.Rat).Mul(v, hi))
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			for _, q := range []*big.Rat{a, b} {
				if e := budget.check(q); e != nil {
					return e
				}
			}
			p.Bounds[j] = ExactParameterInterval{Lower: a, Upper: b}
		}
		out.Contacts = append(out.Contacts, p)
		return nil
	}
	// Handle isolated poles independently, including a pole lying inside the
	// other disk. Equal pole coordinates are emitted once with both identities.
	for i := 0; i < 2; i++ {
		if out.Supports[i].RadiusSquared.Sign() != 0 {
			continue
		}
		if i == 1 && out.Supports[0].RadiusSquared.Sign() == 0 && s.center[0][0].Cmp(s.center[1][0]) == 0 && s.center[0][1].Cmp(s.center[1][1]) == 0 {
			continue
		}
		base := [3]*big.Rat{s.center[i][0], s.center[i][1], out.Height}
		signs := make([]int, 2)
		inc := BoreBoundaryMask(0)
		for j := 0; j < 2; j++ {
			v := new(big.Rat).Neg(out.Supports[j].RadiusSquared)
			for k := 0; k < 2; k++ {
				d := new(big.Rat).Sub(base[k], s.center[j][k])
				v.Add(v, new(big.Rat).Mul(d, d))
			}
			signs[j] = v.Sign()
			if signs[j] == 0 {
				inc |= 1 << uint(j)
			}
		}
		if e := emit(base, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, new(big.Rat), 0, signs, inc); e != nil {
			return zero, e
		}
	}
	ra, rb := out.Supports[0].RadiusSquared, out.Supports[1].RadiusSquared
	if ra.Sign() > 0 && rb.Sign() > 0 {
		dx := new(big.Rat).Sub(s.center[1][0], s.center[0][0])
		dy := new(big.Rat).Sub(s.center[1][1], s.center[0][1])
		d2 := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
		if d2.Sign() == 0 {
			if ra.Cmp(rb) == 0 {
				out.CoincidentCircle = true
				out.CoincidentLocation, out.CoincidentBoundary = s.location([]int{0, 0})
			}
		} else {
			k := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Sub(ra, rb), d2), new(big.Rat).Mul(big.NewRat(2, 1), d2))
			rad := new(big.Rat).Sub(new(big.Rat).Quo(ra, d2), new(big.Rat).Mul(k, k))
			if e := budget.check(rad); e != nil {
				return zero, e
			}
			if rad.Sign() >= 0 {
				base := [3]*big.Rat{new(big.Rat).Add(s.center[0][0], new(big.Rat).Mul(k, dx)), new(big.Rat).Add(s.center[0][1], new(big.Rat).Mul(k, dy)), out.Height}
				vector := [3]*big.Rat{new(big.Rat).Neg(dy), dx, new(big.Rat)}
				signs := []int{-1, 1}
				if rad.Sign() == 0 {
					signs = []int{0}
				}
				for _, sign := range signs {
					if e := emit(base, vector, rad, sign, []int{0, 0}, SpherePairFirst|SpherePairSecond); e != nil {
						return zero, e
					}
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

package geom

import (
	"context"
	"math/big"
)

// SpherePlaneSectionContact retains world and Gram-chart coordinates of one
// event, both using the same Sign and Radicand. UV=UVBase+Sign*sqrt(Radicand)*
// UVVector. World bounds and global material marks are in SphereSectionContact.
type SpherePlaneSectionContact struct {
	SphereSectionContact
	UVBase, UVVector [2]*big.Rat
}

// SpherePairPlaneSection describes original support events, including isolated
// poles and coincident circles. It does not select or assemble boundary arcs.
type SpherePairPlaneSection struct {
	Chart              SpherePairPlaneSupports
	Contacts           []SpherePlaneSectionContact
	CoincidentCircle   bool
	CoincidentLocation BorePointLocation
	CoincidentBoundary BoreBoundaryMask
}

// PlaneSection uses a raw represented plane. Each of support extraction and
// contact construction has the supplied bounded exact budget. Empty results
// distinguish absent contacts from coincident circles; all failures are atomic.
func (s SpherePairMaterial) PlaneSection(ctx context.Context, origin, normal Vec3, o SurfacePatchEvaluationOptions) (SpherePairPlaneSection, error) {
	var zero SpherePairPlaneSection
	chart, err := s.PlaneSupports(ctx, origin, normal, o)
	if err != nil {
		return zero, err
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	var failure error
	check := func(q *big.Rat) *big.Rat {
		if failure == nil {
			failure = b.check(q)
		}
		return q
	}
	add := func(a, c *big.Rat) *big.Rat { return check(new(big.Rat).Add(a, c)) }
	sub := func(a, c *big.Rat) *big.Rat { return check(new(big.Rat).Sub(a, c)) }
	mul := func(a, c *big.Rat) *big.Rat { return check(new(big.Rat).Mul(a, c)) }
	div := func(a, c *big.Rat) *big.Rat { return check(new(big.Rat).Quo(a, c)) }
	out := SpherePairPlaneSection{Chart: chart}
	// This basis has identity entries in its two free rows, so exact coordinate
	// extraction is a left inverse, not a rounded least-squares projection.
	pivot := -1
	for j := 0; j < 3; j++ {
		if chart.Normal[j].Sign() != 0 {
			pivot = j
		}
	}
	var free [2]int
	k := 0
	for j := 0; j < 3; j++ {
		if j != pivot {
			free[k] = j
			k++
		}
	}
	emit := func(base, vector [3]*big.Rat, rad *big.Rat, sign int, signs []int, inc BoreBoundaryMask) error {
		if failure != nil {
			return failure
		}
		lo, hi, e := rationalSqrtBounds(rad, 96, b)
		if e != nil {
			return e
		}
		p := SpherePlaneSectionContact{SphereSectionContact: SphereSectionContact{Radicand: new(big.Rat).Set(rad), Sign: sign, Incidence: inc}}
		p.Location, p.Boundary = s.location(signs)
		for j := 0; j < 3; j++ {
			p.Base[j] = new(big.Rat).Set(base[j])
			p.Vector[j] = new(big.Rat).Set(vector[j])
			v := mul(vector[j], big.NewRat(int64(sign), 1))
			a, c := add(base[j], mul(v, lo)), add(base[j], mul(v, hi))
			if a.Cmp(c) > 0 {
				a, c = c, a
			}
			p.Bounds[j] = ExactParameterInterval{Lower: a, Upper: c}
		}
		for j := 0; j < 2; j++ {
			p.UVBase[j] = sub(base[free[j]], chart.Origin[free[j]])
			p.UVVector[j] = new(big.Rat).Set(vector[free[j]])
		}
		if failure != nil {
			return failure
		}
		out.Contacts = append(out.Contacts, p)
		return nil
	}
	ra, rb := chart.Supports[0].RadiusSquared, chart.Supports[1].RadiusSquared
	var delta [3]*big.Rat
	d2 := new(big.Rat)
	for j := 0; j < 3; j++ {
		delta[j] = sub(chart.Supports[1].WorldCenter[j], chart.Supports[0].WorldCenter[j])
		d2 = add(d2, mul(delta[j], delta[j]))
	}
	for i := 0; i < 2; i++ {
		if chart.Supports[i].Sign != 0 {
			continue
		}
		if i == 1 && ra.Sign() == 0 && d2.Sign() == 0 {
			continue
		}
		signs := []int{0, 0}
		inc := BoreBoundaryMask(0)
		for j := 0; j < 2; j++ {
			v := new(big.Rat).Neg(chart.Supports[j].RadiusSquared)
			if i != j {
				v = add(v, d2)
			}
			signs[j] = v.Sign()
			if signs[j] == 0 {
				inc |= 1 << uint(j)
			}
		}
		if e := emit(chart.Supports[i].WorldCenter, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, new(big.Rat), 0, signs, inc); e != nil {
			return zero, e
		}
	}
	if ra.Sign() > 0 && rb.Sign() > 0 {
		if d2.Sign() == 0 {
			if ra.Cmp(rb) == 0 {
				out.CoincidentCircle = true
				out.CoincidentLocation, out.CoincidentBoundary = s.location([]int{0, 0})
			}
		} else {
			t := div(add(sub(ra, rb), d2), mul(big.NewRat(2, 1), d2))
			rad := div(sub(div(ra, d2), mul(t, t)), chart.NormalSquared)
			if rad.Sign() >= 0 {
				var base, vec [3]*big.Rat
				for j := 0; j < 3; j++ {
					base[j] = add(chart.Supports[0].WorldCenter[j], mul(t, delta[j]))
					a, c := (j+1)%3, (j+2)%3
					vec[j] = sub(mul(chart.Normal[a], delta[c]), mul(chart.Normal[c], delta[a]))
				}
				signs := []int{-1, 1}
				if rad.Sign() == 0 {
					signs = []int{0}
				}
				for _, sign := range signs {
					if e := emit(base, vec, rad, sign, []int{0, 0}, SpherePairFirst|SpherePairSecond); e != nil {
						return zero, e
					}
				}
			}
		}
	}
	if failure != nil {
		return zero, failure
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

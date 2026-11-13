package geom

import (
	"context"
	"math/big"
)

// DiscoverCoupledQuarticGraphPointContact recovers the rational witness from
// pure-axis cubic/quartic coefficients in normalized chart coordinates, then
// independently calls the full global contact certificate on original inputs.
// A candidate formula alone never establishes a root or a contact type.
func DiscoverCoupledQuarticGraphPointContact(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r CoupledQuarticGraphContact, err error) {
	if ctx == nil || a == nil || c == nil {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 6 || p.degreeV > 6 {
		return r, ErrConvergence
	}
	var witness [2]*big.Rat
	domain := p.Domain()
	ranges := [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
	for axis := 0; axis < 2; axis++ {
		count := p.countU
		if axis == 1 {
			count = p.countV
		}
		values := make([]*big.Rat, count)
		for i := range values {
			index := i * p.countV
			if axis == 1 {
				index = i
			}
			values[i] = p.net[index][2]
		}
		power, e := bernsteinToPower(values, budget)
		if e != nil {
			return r, e
		}
		if len(power) < 5 || power[4].Sign() == 0 {
			return r, ErrConvergence
		}
		q := new(big.Rat).Quo(new(big.Rat).Neg(power[3]), new(big.Rat).Mul(big.NewRat(4, 1), power[4]))
		lo, hi := new(big.Rat).SetFloat64(ranges[axis][0]), new(big.Rat).SetFloat64(ranges[axis][1])
		witness[axis] = new(big.Rat).Add(lo, new(big.Rat).Mul(q, new(big.Rat).Sub(hi, lo)))
		if e := budget.check(witness[axis]); e != nil {
			return r, e
		}
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	return CertifyCoupledQuarticGraphPointContact(ctx, a, c, witness, o)
}

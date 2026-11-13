package geom

import "math/big"

// SurfaceSeparationOptions bounds exact projection work. Zero fields select
// 1M operations and 16384 coefficient bits. World axes and the two exact center
// tangent normals are tested. Normal projections require Bezier patches;
// non-Bezier patches still receive the world-axis tests.
type SurfaceSeparationOptions struct{ MaxOperations, MaxCoefficientBits int }

// SurfaceSeparation reports a strict separating control-hull projection.
// ProjectionA/B bound dot(Direction,point-Origin); Direction is an outward
// component enclosure of an exact rational, unnormalized direction. GapLower
// is in that direction's scale, not Euclidean distance. ExactStrictGap remains
// true even when GapLower underflows to zero. Separation does not follow from
// touching projection endpoints. An inconclusive result makes no contact claim.
type SurfaceSeparation struct {
	Separated, ExactStrictGap         bool
	Direction                         [3]CertifiedScalarRange
	Origin                            Vec3
	ProjectionA, ProjectionB          CertifiedScalarRange
	GapLower                          float64
	TestedDirections, ExactOperations int
}

func exactProjectionHull(p *SurfacePatch, origin Vec3, d [3]*big.Rat, b *polynomialBudget) (exactInterval, error) {
	var lo, hi *big.Rat
	for _, h := range p.net {
		value := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			centered := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(origin, axis)), h[3]))
			value.Add(value, new(big.Rat).Mul(d[axis], centered))
		}
		value.Quo(value, h[3])
		if e := b.check(value); e != nil {
			return exactInterval{}, e
		}
		if lo == nil || value.Cmp(lo) < 0 {
			lo = new(big.Rat).Set(value)
		}
		if hi == nil || value.Cmp(hi) > 0 {
			hi = new(big.Rat).Set(value)
		}
	}
	return exactInterval{lo, hi}, nil
}

// SeparateSurfacePatches proves disjointness using exact rational projection
// hulls of positive-weight control points. It cannot exclude a genuine contact,
// including tangencies or shared boundaries. Direction choice is merely a
// heuristic; the separation comparison itself is exact. Neither normalization
// nor a floating dot-product sign is used to decide separation.
func SeparateSurfacePatches(a, c *SurfacePatch, o SurfaceSeparationOptions) (result SurfaceSeparation, err error) {
	if a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return result, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	result.Origin = a.bounds.Min
	test := func(direction [3]*big.Rat) (bool, error) {
		nonzero := false
		for _, v := range direction {
			nonzero = nonzero || v.Sign() != 0
		}
		if !nonzero {
			return false, nil
		}
		result.TestedDirections++
		pa, e := exactProjectionHull(a, result.Origin, direction, b)
		if e != nil {
			return false, e
		}
		pc, e := exactProjectionHull(c, result.Origin, direction, b)
		if e != nil {
			return false, e
		}
		gap := new(big.Rat).Sub(pc.lo, pa.hi)
		other := new(big.Rat).Sub(pa.lo, pc.hi)
		if other.Cmp(gap) > 0 {
			gap = other
		}
		if gap.Sign() <= 0 {
			return false, nil
		}
		if e := b.check(gap); e != nil {
			return false, e
		}
		result.Separated, result.ExactStrictGap = true, true
		result.ProjectionA, result.ProjectionB = certifiedRange(pa), certifiedRange(pc)
		result.GapLower = rationalFloatBound(gap, true)
		for i, v := range direction {
			result.Direction[i] = certifiedRange(exactInterval{v, v})
		}
		return true, nil
	}
	for axis := 0; axis < 3; axis++ {
		direction := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		direction[axis].SetInt64(1)
		ok, e := test(direction)
		if e != nil || ok {
			return result, e
		}
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			continue
		}
		domain := p.Domain()
		mid := func(lo, hi float64) *big.Rat {
			return new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)), big.NewRat(2, 1))
		}
		_, derivative, e := exactSurfaceJet(p, mid(domain.UMin, domain.UMax), mid(domain.VMin, domain.VMax), b)
		if e != nil {
			return result, e
		}
		var normal [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			j, k := (axis+1)%3, (axis+2)%3
			normal[axis] = new(big.Rat).Sub(new(big.Rat).Mul(derivative[j][0], derivative[k][1]), new(big.Rat).Mul(derivative[k][0], derivative[j][1]))
			if e := b.check(normal[axis]); e != nil {
				return result, e
			}
		}
		ok, e := test(normal)
		if e != nil || ok {
			return result, e
		}
	}
	return result, nil
}

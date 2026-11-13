package geom

import (
	"context"
	"math/big"
)

// SurfaceSupportingPlaneOptions limits exact knot-aligned halfspace proof.
// Zero fields select depth24, 8192 cells, 4M operations and 16384 bits.
type SurfaceSupportingPlaneOptions struct{ MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int }

// SurfaceSupportingPlaneCertificate describes the zero set of the exact raw
// determinant ((b-a) cross (c-a)) dot (S-a). A noncoincident complete result
// proves that all remaining points have strict HalfspaceSign. SideMask bits
// 0..3 denote UMin,UMax,VMin,VMax. CornerMask bits 0..3 denote (UMin,VMin),
// (UMax,VMin),(UMax,VMax),(UMin,VMax), excluding corners already on marked sides.
// Coincident means the entire patch lies in the plane, with no finite contact
// mask description. On error the certificate is incomplete and masks/sign must
// not be used. No normalized or rounded plane normal participates in the proof.
type SurfaceSupportingPlaneCertificate struct {
	Complete, Coincident          bool
	HalfspaceSign                 int
	SideMask, CornerMask          uint8
	VisitedCells, ExactOperations int
}

// CertifySurfaceSupportingPlane proves only the stated boundary contact pattern.
// Interior contacts, partial boundary arcs, mixed signs or unproved bounds return
// ErrConvergence; this is not a general contact-set classifier.
func CertifySurfaceSupportingPlane(ctx context.Context, p *SurfacePatch, a, c, d Vec3, o SurfaceSupportingPlaneOptions) (r SurfaceSupportingPlaneCertificate, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || !a.IsFinite() || !c.IsFinite() || !d.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.ExactOperations = budget.operations
	}()
	var origin, x, y, normal [3]*big.Rat
	for axis := 0; axis < 3; axis++ {
		origin[axis] = new(big.Rat).SetFloat64(coordinate(a, axis))
		x[axis] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(c, axis)), origin[axis])
		y[axis] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(d, axis)), origin[axis])
	}
	nonzero := false
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		normal[axis] = new(big.Rat).Sub(new(big.Rat).Mul(x[j], y[k]), new(big.Rat).Mul(x[k], y[j]))
		if e := budget.check(normal[axis]); e != nil {
			return r, e
		}
		nonzero = nonzero || normal[axis].Sign() != 0
	}
	if !nonzero {
		return r, ErrDegenerate
	}
	return certifySurfaceSupportingEquation(ctx, p, o, budget, func(p *SurfacePatch) ([]*big.Rat, error) {
		return surfacePlaneHomogeneousCoefficients(p, origin, normal, budget)
	})
}

func surfacePlaneHomogeneousCoefficients(p *SurfacePatch, origin, normal [3]*big.Rat, budget *polynomialBudget) ([]*big.Rat, error) {
	values := make([]*big.Rat, len(p.net))
	for i, h := range p.net {
		value := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			delta := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(origin[axis], h[3]))
			value.Add(value, new(big.Rat).Mul(normal[axis], delta))
			if e := budget.check(value); e != nil {
				return nil, e
			}
		}
		values[i] = value
	}
	return values, nil
}

func certifySurfaceSupportingEquation(ctx context.Context, p *SurfacePatch, o SurfaceSupportingPlaneOptions, budget *polynomialBudget,
	coefficients func(*SurfacePatch) ([]*big.Rat, error)) (r SurfaceSupportingPlaneCertificate, err error) {
	initial, e := coefficients(p)
	if e != nil {
		return r, e
	}
	allZero := true
	for _, v := range initial {
		allZero = allZero && v.Sign() == 0
	}
	if allZero {
		r.Complete, r.Coincident = true, true
		return r, nil
	}
	sideMask := uint8(0)
	for side := 0; side < 4; side++ {
		zero := true
		count := p.countV
		if side >= 2 {
			count = p.countU
		}
		for i := 0; i < count; i++ {
			index := i
			if side == 1 {
				index = (p.countU-1)*p.countV + i
			} else if side == 2 {
				index = i * p.countV
			} else if side == 3 {
				index = i*p.countV + p.countV - 1
			}
			zero = zero && initial[index].Sign() == 0
		}
		if zero {
			sideMask |= 1 << side
		}
	}
	cornerIndices := [4]int{0, (p.countU - 1) * p.countV, len(p.net) - 1, p.countV - 1}
	adjacent := [4]uint8{1 | 4, 2 | 4, 2 | 8, 1 | 8}
	cornerMask := uint8(0)
	for corner, index := range cornerIndices {
		if initial[index].Sign() == 0 && sideMask&adjacent[corner] == 0 {
			cornerMask |= 1 << corner
		}
	}
	domain := p.Domain()
	allowedPoint := func(u, v float64) bool {
		if (u == domain.UMin && sideMask&1 != 0) || (u == domain.UMax && sideMask&2 != 0) || (v == domain.VMin && sideMask&4 != 0) || (v == domain.VMax && sideMask&8 != 0) {
			return true
		}
		return (u == domain.UMin && v == domain.VMin && cornerMask&1 != 0) || (u == domain.UMax && v == domain.VMin && cornerMask&2 != 0) || (u == domain.UMax && v == domain.VMax && cornerMask&4 != 0) || (u == domain.UMin && v == domain.VMax && cornerMask&8 != 0)
	}
	type node struct {
		p     *SurfacePatch
		depth int
	}
	stack := []node{{p, 0}}
	sign := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if r.VisitedCells >= o.MaxCells {
			return r, ErrConvergence
		}
		r.VisitedCells++
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t, u, knot := patchSplitChoice(n.p)
		if !knot {
			values, e := coefficients(n.p)
			if e != nil {
				return r, e
			}
			positive, negative := false, false
			for _, v := range values {
				positive = positive || v.Sign() > 0
				negative = negative || v.Sign() < 0
			}
			if !positive && !negative {
				return r, ErrConvergence
			}
			if !(positive && negative) {
				cellSign := 1
				if negative {
					cellSign = -1
				}
				if sign != 0 && sign != cellSign {
					return r, ErrConvergence
				}
				sign = cellSign
				rect := n.p.Domain()
				nu, nv := n.p.countU, n.p.countV
				for side := 0; side < 4; side++ {
					zero := true
					count := nv
					if side >= 2 {
						count = nu
					}
					for i := 0; i < count; i++ {
						index := i
						if side == 1 {
							index = (nu-1)*nv + i
						} else if side == 2 {
							index = i * nv
						} else if side == 3 {
							index = i*nv + nv - 1
						}
						zero = zero && values[index].Sign() == 0
					}
					if zero {
						allowed := (side == 0 && rect.UMin == domain.UMin && sideMask&1 != 0) || (side == 1 && rect.UMax == domain.UMax && sideMask&2 != 0) || (side == 2 && rect.VMin == domain.VMin && sideMask&4 != 0) || (side == 3 && rect.VMax == domain.VMax && sideMask&8 != 0)
						if !allowed {
							return r, ErrConvergence
						}
					}
				}
				indices := [4]int{0, (nu - 1) * nv, len(values) - 1, nv - 1}
				coordinates := [4]Vec2{{rect.UMin, rect.VMin}, {rect.UMax, rect.VMin}, {rect.UMax, rect.VMax}, {rect.UMin, rect.VMax}}
				for i, index := range indices {
					if values[index].Sign() == 0 && !allowedPoint(coordinates[i].X, coordinates[i].Y) {
						return r, ErrConvergence
					}
				}
				continue
			}
			if n.depth >= o.MaxDepth {
				return r, ErrConvergence
			}
		}
		left, right, e := n.p.split(t, u, budget)
		if e != nil {
			return r, e
		}
		depth := n.depth + 1
		if knot {
			depth = 0
		}
		stack = append(stack, node{right, depth}, node{left, depth})
	}
	r.Complete = true
	r.HalfspaceSign = sign
	r.SideMask = sideMask
	r.CornerMask = cornerMask
	return r, nil
}

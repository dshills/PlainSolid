package geom

import (
	"context"
	"math/big"
	"sort"
)

// IntersectWrappedCylinderLine intersects an arbitrary finite represented line
// with a canonical positive-weight rational cylinder and its planar disk caps.
// Exact rational Z clipping precedes side-conic isolation. Cap locations are
// evaluated in original rational t without binary64 conversion. Axial lines
// on the lateral boundary produce explicit coincident intervals. Horizontal
// lines retain the cap-plane overlap semantics of the horizontal API.
// The embedding/cap stage and side-isolation stage each have the supplied
// bounded arithmetic budget; side subdivisions are bounded per quadrant.
func IntersectWrappedCylinderLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	if !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	if direction.Z == 0 {
		return IntersectWrappedCylinderHorizontalLine(ctx, w, origin, direction, minT, maxT, o)
	}
	p, e := CertifyWrappedCylinderEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	options, _ := patchOptions(o.Surface)
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	lo, hi := new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT)
	z, dz := new(big.Rat).SetFloat64(origin.Z), new(big.Rat).SetFloat64(direction.Z)
	capT := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(p.LowerZ, z), dz), new(big.Rat).Quo(new(big.Rat).Sub(p.UpperZ, z), dz)}
	a, b := capT[0], capT[1]
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	sideLo, sideHi := new(big.Rat).Set(lo), new(big.Rat).Set(hi)
	if a.Cmp(sideLo) > 0 {
		sideLo.Set(a)
	}
	if b.Cmp(sideHi) < 0 {
		sideHi.Set(b)
	}
	for _, q := range []*big.Rat{lo, hi, z, dz, capT[0], capT[1], sideLo, sideHi} {
		if e := budget.check(q); e != nil {
			return empty, e
		}
	}
	if sideLo.Cmp(sideHi) > 0 {
		return empty, nil
	}
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	profile := func(t *big.Rat) (int, error) {
		x := new(big.Rat).Add(new(big.Rat).SetFloat64(origin.X), new(big.Rat).Mul(t, new(big.Rat).SetFloat64(direction.X)))
		y := new(big.Rat).Add(new(big.Rat).SetFloat64(origin.Y), new(big.Rat).Mul(t, new(big.Rat).SetFloat64(direction.Y)))
		u := new(big.Rat).Quo(new(big.Rat).Sub(x, p.CenterX), p.Radius)
		v := new(big.Rat).Quo(new(big.Rat).Sub(y, p.CenterY), p.Radius)
		for _, q := range []*big.Rat{x, y, u, v} {
			if e := budget.check(q); e != nil {
				return 0, e
			}
		}
		j := 0
		if u.Sign() < 0 {
			if v.Sign() >= 0 {
				j = 1
				u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
			} else {
				j = 2
				u.Neg(u)
				v.Neg(v)
			}
		} else if v.Sign() < 0 {
			j = 3
			u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
		}
		one := big.NewRat(1, 1)
		if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
			return 1, nil
		}
		sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
		if sum.Sign() < 0 {
			return -1, nil
		}
		wa, wb, wc := w.patch.net[4*j][3], w.patch.net[4*j+2][3], w.patch.net[4*j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(wa, wc), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(wb, wb))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, q := range []*big.Rat{sum, left, right} {
			if e := budget.check(q); e != nil {
				return 0, e
			}
		}
		return left.Cmp(right), nil
	}
	result := CylinderLineIntersections{}
	if direction.X == 0 && direction.Y == 0 {
		sign, e := profile(sideLo)
		if e != nil {
			return empty, e
		}
		if sign > 0 {
			return empty, nil
		}
		if sign == 0 {
			if sideLo.Cmp(sideHi) == 0 {
				result.Hits = append(result.Hits, CylinderLineHit{exact(sideLo), CylinderLateral})
			} else {
				result.Overlaps = append(result.Overlaps, CylinderLineOverlap{exact(sideLo), exact(sideHi), CylinderLateral})
			}
		}
	} else {
		result, e = intersectCylinderProfileLine(ctx, w, origin, direction, sideLo, sideHi, o, true)
		if e != nil {
			return empty, e
		}
	}
	for i, t := range capT {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 {
			continue
		}
		sign, e := profile(t)
		if e != nil {
			return empty, e
		}
		if sign > 0 {
			continue
		}
		part := CylinderLowerCap
		if i == 1 {
			part = CylinderUpperCap
		}
		merged := false
		if sign == 0 {
			for j := range result.Hits {
				h := &result.Hits[j]
				if h.Parameter.Min.Cmp(t) <= 0 && h.Parameter.Max.Cmp(t) >= 0 {
					m := h.Parameter.Multiplicity
					h.Parameter = exact(t)
					h.Parameter.Multiplicity = m
					h.Parts |= part
					merged = true
					break
				}
			}
			part |= CylinderLateral
		}
		if !merged {
			result.Hits = append(result.Hits, CylinderLineHit{exact(t), part})
		}
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	return result, nil
}

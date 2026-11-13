package geom

import (
	"context"
	"math/big"
	"sort"
)

type CylinderLocation uint8

const (
	CylinderUnknown CylinderLocation = iota
	CylinderOutside
	CylinderInside
	CylinderBoundary
)

// LocateWrappedCylinder classifies material bounded by the certified lateral
// patch and its two planar disks. Arbitrary positive rational quadrant weights
// are supported. No tolerance or circular approximation is used.
func LocateWrappedCylinder(ctx context.Context, w *WrappedSurfacePatch, point Vec3, o SurfacePatchOptions) (CylinderLocation, error) {
	if !point.IsFinite() {
		return CylinderUnknown, ErrInvalid
	}
	p, e := CertifyWrappedCylinderEmbedding(ctx, w, o)
	if e != nil {
		return CylinderUnknown, e
	}
	options, _ := patchOptions(o)
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	z := new(big.Rat).SetFloat64(point.Z)
	if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
		return CylinderOutside, nil
	}
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(point.X), p.CenterX), p.Radius)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Y), p.CenterY), p.Radius)
	for _, n := range []*big.Rat{z, u, v} {
		if e := budget.check(n); e != nil {
			return CylinderUnknown, e
		}
	}
	quadrant := 0
	if u.Sign() < 0 {
		if v.Sign() >= 0 {
			quadrant = 1
			u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
		} else {
			quadrant = 2
			u.Neg(u)
			v.Neg(v)
		}
	} else if v.Sign() < 0 {
		quadrant = 3
		u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
	}
	one := big.NewRat(1, 1)
	if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
		return CylinderOutside, nil
	}
	sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
	sign := -1
	if sum.Sign() >= 0 {
		j := 4 * quadrant
		a, b, c := w.patch.net[j][3], w.patch.net[j+2][3], w.patch.net[j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(a, c), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(b, b))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, n := range []*big.Rat{u, v, sum, left, right} {
			if e := budget.check(n); e != nil {
				return CylinderUnknown, e
			}
		}
		sign = left.Cmp(right)
	}
	if sign > 0 {
		return CylinderOutside, nil
	}
	if sign == 0 || z.Cmp(p.LowerZ) == 0 || z.Cmp(p.UpperZ) == 0 {
		return CylinderBoundary, nil
	}
	return CylinderInside, nil
}

// CylinderBoundaryPart identifies incident boundary patches as a bit mask.
type CylinderBoundaryPart uint8

const (
	CylinderLateral CylinderBoundaryPart = 1 << iota
	CylinderLowerCap
	CylinderUpperCap
	CylinderLowerBlend
	CylinderUpperBlend
)

type CylinderLineHit struct {
	Parameter ToroidLineRoot
	Parts     CylinderBoundaryPart
}

// CylinderLineOverlap contains an entire coincident boundary interval. Start
// and End bracket its algebraic endpoint parameters, not a rounded interval.
type CylinderLineOverlap struct {
	Start, End ToroidLineRoot
	Part       CylinderBoundaryPart
}
type CylinderLineIntersections struct {
	Hits     []CylinderLineHit
	Overlaps []CylinderLineOverlap
}

// IntersectWrappedCircularCylinderLine intersects a finite represented line
// with the complete circular cylinder boundary, including planar caps. Side
// tangencies retain multiplicity two. Coincident boundary intervals are
// explicit; their endpoints may be exact rational algebraic brackets.
// Options use the same original-parameter semantics as the toroid line query.
func IntersectWrappedCircularCylinderLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	if !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return empty, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	p, e := CertifyWrappedCylinderEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	if !p.Circular {
		return empty, ErrConvergence
	}
	options, _ := patchOptions(o.Surface)
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	x := new(big.Rat).Sub(new(big.Rat).SetFloat64(origin.X), p.CenterX)
	y := new(big.Rat).Sub(new(big.Rat).SetFloat64(origin.Y), p.CenterY)
	dx, dy, dz := new(big.Rat).SetFloat64(direction.X), new(big.Rat).SetFloat64(direction.Y), new(big.Rat).SetFloat64(direction.Z)
	z := new(big.Rat).SetFloat64(origin.Z)
	poly := rationalPolynomial{new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)), new(big.Rat).Mul(p.Radius, p.Radius)), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(x, dx), new(big.Rat).Mul(y, dy))), new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))}
	for _, n := range append([]*big.Rat{x, y, dx, dy, dz, z}, poly...) {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	poly = trimPolynomial(poly)
	lo, hi := new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT)
	tol := new(big.Rat).SetFloat64(o.ParameterTolerance)
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	eval := func(t *big.Rat) *big.Rat {
		r := new(big.Rat)
		for i := len(poly) - 1; i >= 0; i-- {
			r.Mul(r, t)
			r.Add(r, poly[i])
		}
		return r
	}
	isolate := func(a, b *big.Rat) ([]ToroidLineRoot, error) {
		var roots []ToroidLineRoot
		_, e := isolatePolynomialRoots(poly, a, b, budget, o.MaxSubdivisions, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(h, l).Cmp(tol) > 0 {
				return false, nil
			}
			roots = append(roots, ToroidLineRoot{new(big.Rat).Set(l), new(big.Rat).Set(h), m})
			return true, nil
		})
		sort.Slice(roots, func(i, j int) bool { return roots[i].Min.Cmp(roots[j].Min) < 0 })
		return roots, e
	}
	result := CylinderLineIntersections{}
	// Exact intersection of the parameter interval with the cylinder height.
	sideLo, sideHi := new(big.Rat).Set(lo), new(big.Rat).Set(hi)
	if dz.Sign() == 0 {
		if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
			sideLo.SetInt64(1)
			sideHi.SetInt64(0)
		}
	} else {
		a := new(big.Rat).Quo(new(big.Rat).Sub(p.LowerZ, z), dz)
		b := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperZ, z), dz)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if a.Cmp(sideLo) > 0 {
			sideLo.Set(a)
		}
		if b.Cmp(sideHi) < 0 {
			sideHi.Set(b)
		}
	}
	for _, n := range []*big.Rat{lo, hi, sideLo, sideHi} {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	if sideLo.Cmp(sideHi) <= 0 {
		if len(poly) == 0 {
			result.Overlaps = append(result.Overlaps, CylinderLineOverlap{exact(sideLo), exact(sideHi), CylinderLateral})
		} else {
			roots, e := isolate(sideLo, sideHi)
			if e != nil {
				return empty, e
			}
			for _, r := range roots {
				result.Hits = append(result.Hits, CylinderLineHit{r, CylinderLateral})
			}
		}
	}
	for i, height := range []*big.Rat{p.LowerZ, p.UpperZ} {
		part := CylinderLowerCap
		if i == 1 {
			part = CylinderUpperCap
		}
		if dz.Sign() != 0 {
			t := new(big.Rat).Quo(new(big.Rat).Sub(height, z), dz)
			if e := budget.check(t); e != nil {
				return empty, e
			}
			if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 || eval(t).Sign() > 0 {
				continue
			}
			merged := false
			if eval(t).Sign() == 0 {
				for j := range result.Hits {
					r := &result.Hits[j]
					if r.Parameter.Min.Cmp(t) <= 0 && r.Parameter.Max.Cmp(t) >= 0 {
						r.Parts |= part
						multiplicity := r.Parameter.Multiplicity
						r.Parameter = exact(t)
						r.Parameter.Multiplicity = multiplicity
						merged = true
						break
					}
				}
			}
			if !merged {
				if eval(t).Sign() == 0 {
					part |= CylinderLateral
				}
				result.Hits = append(result.Hits, CylinderLineHit{exact(t), part})
			}
			continue
		}
		if z.Cmp(height) != 0 {
			continue
		}
		for j := range result.Hits {
			result.Hits[j].Parts |= part
		}
		if len(poly) == 0 {
			result.Overlaps = append(result.Overlaps, CylinderLineOverlap{exact(lo), exact(hi), part})
			continue
		}
		roots, e := isolate(lo, hi)
		if e != nil {
			return empty, e
		}
		a, b := exact(lo), exact(hi)
		if eval(lo).Sign() > 0 {
			if len(roots) == 0 {
				continue
			}
			a = roots[0]
		}
		if eval(hi).Sign() > 0 {
			if len(roots) == 0 {
				continue
			}
			b = roots[len(roots)-1]
		}
		if len(roots) == 1 && roots[0].Multiplicity == 2 {
			for j := range result.Hits {
				result.Hits[j].Parts |= part
			}
			continue
		}
		result.Overlaps = append(result.Overlaps, CylinderLineOverlap{a, b, part})
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	return result, nil
}

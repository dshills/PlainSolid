package geom

import (
	"context"
	"math/big"
	"runtime"
	"sort"
)

// CircularRoundedCylinderProfile describes a right circular cylinder with
// inward circular rolling blends at its axial ends. A zero blend radius
// leaves its end sharp. All coordinates and dimensions are exact.
type CircularRoundedCylinderProfile struct {
	Center [2]*big.Rat
	Height [2]*big.Rat
	Radius *big.Rat
	Blend  [2]*big.Rat
}

// IntersectCircularRoundedCylinderLine returns exact original-parameter root
// enclosures and coincident intervals for this analytic profile. Slab clipping
// is rational. The toroidal quartic retains only its outer radial branch.
// Coplanar disk intervals and cylindrical generator intervals are explicit.
func IntersectCircularRoundedCylinderLine(ctx context.Context, p CircularRoundedCylinderProfile, origin, direction Vec3,
	minT, maxT float64, o ToroidLineOptions) (CylinderLineIntersections, error) {
	var empty CylinderLineIntersections
	if ctx == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) ||
		!finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	centers, e := validateCircularRoundedCylinderProfile(p, budget)
	if e != nil {
		return empty, e
	}
	z1, z2 := centers[0], centers[1]
	two := big.NewRat(2, 1)
	rat := func(f float64) *big.Rat { return new(big.Rat).SetFloat64(f) }
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	lo, hi, tol := rat(minT), rat(maxT), rat(o.ParameterTolerance)
	z, dz := rat(origin.Z), rat(direction.Z)
	x, y := new(big.Rat).Sub(rat(origin.X), p.Center[0]), new(big.Rat).Sub(rat(origin.Y), p.Center[1])
	dx, dy := rat(direction.X), rat(direction.Y)
	q := rationalPolynomial{
		new(big.Rat).Add(square(x), square(y)),
		new(big.Rat).Mul(two, new(big.Rat).Add(new(big.Rat).Mul(x, dx), new(big.Rat).Mul(y, dy))),
		new(big.Rat).Add(square(dx), square(dy)),
	}
	for _, n := range q {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	r2 := square(p.Radius)
	cap := [2]CylinderBoundaryPart{CylinderLowerCap, CylinderUpperCap}
	blend := [2]CylinderBoundaryPart{CylinderLowerBlend, CylinderUpperBlend}
	major := [2]*big.Rat{new(big.Rat).Sub(p.Radius, p.Blend[0]), new(big.Rat).Sub(p.Radius, p.Blend[1])}
	m2 := [2]*big.Rat{square(major[0]), square(major[1])}
	var result CylinderLineIntersections
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	hit := func(t ToroidLineRoot, part CylinderBoundaryPart) {
		result.Hits = append(result.Hits, CylinderLineHit{t, part})
	}
	overlap := func(a, b ToroidLineRoot, part CylinderBoundaryPart) {
		result.Overlaps = append(result.Overlaps, CylinderLineOverlap{a, b, part})
	}
	isolate := func(poly rationalPolynomial, a, b *big.Rat, part CylinderBoundaryPart, acceptBranch func(*big.Rat, *big.Rat) (int, error)) error {
		_, e := isolatePolynomialRoots(trimPolynomial(poly), a, b, budget, o.MaxSubdivisions, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if l.Cmp(h) != 0 && (l.Cmp(a) == 0 || h.Cmp(b) == 0 || new(big.Rat).Sub(h, l).Cmp(tol) > 0) {
				return false, nil
			}
			if acceptBranch != nil {
				branch, e := acceptBranch(l, h)
				if e != nil || branch == 0 {
					return false, e
				}
				if branch < 0 {
					return true, nil
				}
			}
			hit(ToroidLineRoot{new(big.Rat).Set(l), new(big.Rat).Set(h), m}, part)
			return true, nil
		})
		return e
	}
	circle := func(radius2, a, b *big.Rat, part CylinderBoundaryPart) error {
		f := clonePolynomial(q)
		f[0].Sub(f[0], radius2)
		if len(trimPolynomial(f)) == 0 {
			if a.Cmp(b) == 0 {
				hit(exact(a), part)
			} else {
				overlap(exact(a), exact(b), part)
			}
			return nil
		}
		return isolate(f, a, b, part, nil)
	}
	atHeight := func(h *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Sub(h, z), dz) }
	clip := func(low, high *big.Rat) (*big.Rat, *big.Rat, error) {
		if dz.Sign() == 0 {
			if z.Cmp(low) < 0 || z.Cmp(high) > 0 {
				return big.NewRat(1, 1), new(big.Rat), nil
			}
			return lo, hi, nil
		}
		a, b := atHeight(low), atHeight(high)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		for _, t := range []*big.Rat{a, b} {
			if e := budget.check(t); e != nil {
				return nil, nil, e
			}
		}
		if a.Cmp(lo) < 0 {
			a = lo
		}
		if b.Cmp(hi) > 0 {
			b = hi
		}
		return a, b, nil
	}
	// The unsquared equation A=2M sqrt(q) has A>0 because M>r.
	// On the outer branch q>=M²; equality is possible only in the
	// cap plane, where the same circle also belongs to the disk.
	toroid := func(i int, a, b *big.Rat) error {
		z0 := new(big.Rat).Sub(z, centers[i])
		A := rationalPolynomial{
			new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Add(q[0], square(z0)), m2[i]), square(p.Blend[i])),
			new(big.Rat).Add(q[1], new(big.Rat).Mul(two, new(big.Rat).Mul(z0, dz))),
			new(big.Rat).Add(q[2], square(dz)),
		}
		f, e := polynomialProduct(A, A, budget)
		if e != nil {
			return e
		}
		for len(f) < 3 {
			f = append(f, new(big.Rat))
		}
		fourM2 := new(big.Rat).Mul(big.NewRat(4, 1), m2[i])
		for k := range q {
			f[k].Sub(f[k], new(big.Rat).Mul(fourM2, q[k]))
			if e := budget.check(f[k]); e != nil {
				return e
			}
		}
		return isolate(f, a, b, blend[i], func(l, h *big.Rat) (int, error) {
			v, e := polynomialValue(q, l, budget)
			if e != nil {
				return 0, e
			}
			if l.Cmp(h) == 0 {
				if v.Cmp(m2[i]) >= 0 {
					return 1, nil
				}
				return -1, nil
			}
			w, e := polynomialValue(q, h, budget)
			if e != nil {
				return 0, e
			}
			if v.Cmp(m2[i]) < 0 && w.Cmp(m2[i]) < 0 {
				return -1, nil // convex quadratic: maximum at an endpoint
			}
			least := v
			if w.Cmp(v) < 0 {
				least = w
			}
			if q[2].Sign() != 0 {
				t := new(big.Rat).Quo(new(big.Rat).Neg(q[1]), new(big.Rat).Mul(two, q[2]))
				if l.Cmp(t) < 0 && t.Cmp(h) < 0 {
					least, e = polynomialValue(q, t, budget)
					if e != nil {
						return 0, e
					}
				}
			}
			if least.Cmp(m2[i]) > 0 {
				return 1, nil
			}
			return 0, nil
		})
	}
	if dz.Sign() == 0 {
		if z.Cmp(p.Height[0]) < 0 || z.Cmp(p.Height[1]) > 0 {
			return empty, nil
		}
		for i, end := range p.Height {
			if z.Cmp(end) != 0 {
				continue
			}
			part := CylinderLateral
			if p.Blend[i].Sign() != 0 {
				part = blend[i]
			}
			if e := circle(m2[i], lo, hi, cap[i]|part); e != nil {
				return empty, e
			}
			sort.Slice(result.Hits, func(i, j int) bool {
				return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0
			})
			at := [2]*big.Rat{lo, hi}
			ends := [2]ToroidLineRoot{exact(lo), exact(hi)}
			for j, t := range at {
				v, e := polynomialValue(q, t, budget)
				if e != nil {
					return empty, e
				}
				if v.Cmp(m2[i]) > 0 {
					if len(result.Hits) == 0 {
						return empty, nil
					}
					ends[j] = result.Hits[j*(len(result.Hits)-1)].Parameter
				}
			}
			runtime.KeepAlive(&m2) // golang/go#82026: keeps m2 in the stack map. Remove once the Go fix is in use.
			if len(result.Hits) == 1 && ends[0].Min.Cmp(ends[1].Min) == 0 && ends[0].Max.Cmp(ends[1].Max) == 0 {
				return result, nil // isolated tangent or clipped rim
			}
			if lo.Cmp(hi) == 0 {
				hit(exact(lo), cap[i])
			} else {
				overlap(ends[0], ends[1], cap[i])
			}
			return result, nil
		}
		part := CylinderLateral
		for i, c := range centers {
			if p.Blend[i].Sign() != 0 && z.Cmp(c) == 0 {
				part |= blend[i]
			}
		}
		switch {
		case z.Cmp(z1) < 0:
			e = toroid(0, lo, hi)
		case z.Cmp(z2) > 0:
			e = toroid(1, lo, hi)
		default:
			e = circle(r2, lo, hi, part)
		}
		if e != nil {
			return empty, e
		}
		sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
		return result, nil
	}
	for i, bounds := range [3][2]*big.Rat{{p.Height[0], z1}, {z1, z2}, {z2, p.Height[1]}} {
		if bounds[0].Cmp(bounds[1]) == 0 {
			continue
		}
		a, b, e := clip(bounds[0], bounds[1])
		if e != nil {
			return empty, e
		}
		if a.Cmp(b) > 0 {
			continue
		}
		if i == 1 {
			e = circle(r2, a, b, CylinderLateral)
		} else {
			e = toroid(i/2, a, b)
		}
		if e != nil {
			return empty, e
		}
	}
	for i, h := range p.Height {
		t := atHeight(h)
		if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 {
			continue
		}
		v, e := polynomialValue(q, t, budget)
		if e != nil {
			return empty, e
		}
		if v.Cmp(m2[i]) <= 0 {
			part := cap[i]
			if v.Cmp(m2[i]) == 0 {
				if p.Blend[i].Sign() == 0 {
					part |= CylinderLateral
				} else {
					part |= blend[i]
				}
			}
			hit(exact(t), part)
		}
	}
	runtime.KeepAlive(&m2)
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	unique := result.Hits[:0]
	for _, h := range result.Hits {
		if len(unique) > 0 {
			last := &unique[len(unique)-1]
			if last.Parameter.Max.Cmp(h.Parameter.Min) >= 0 {
				t := last.Parameter.Min
				if t.Cmp(last.Parameter.Max) != 0 || t.Cmp(h.Parameter.Min) != 0 || t.Cmp(h.Parameter.Max) != 0 {
					return empty, ErrConvergence
				}
				last.Parts |= h.Parts
				last.Parameter.Multiplicity = max(last.Parameter.Multiplicity, h.Parameter.Multiplicity)
				continue
			}
		}
		unique = append(unique, h)
	}
	result.Hits = unique
	// The generator overlap includes its endpoints. An isolated blend hit
	// at a seam additionally owns the lateral face.
	for i := range result.Hits {
		for _, span := range result.Overlaps {
			t := result.Hits[i].Parameter
			if t.Min.Cmp(t.Max) == 0 && (t.Min.Cmp(span.Start.Min) == 0 || t.Min.Cmp(span.End.Max) == 0) {
				result.Hits[i].Parts |= span.Part
			}
		}
	}
	return result, nil
}

func validateCircularRoundedCylinderProfile(p CircularRoundedCylinderProfile, budget *polynomialBudget) ([2]*big.Rat, error) {
	var zero [2]*big.Rat
	for _, n := range []*big.Rat{p.Center[0], p.Center[1], p.Height[0], p.Height[1], p.Radius, p.Blend[0], p.Blend[1]} {
		if n == nil {
			return zero, ErrInvalid
		}
		if e := budget.check(n); e != nil {
			return zero, e
		}
	}
	for _, r := range p.Blend {
		if r.Sign() < 0 || new(big.Rat).Mul(big.NewRat(2, 1), r).Cmp(p.Radius) >= 0 {
			return zero, ErrDomain
		}
	}
	z := [2]*big.Rat{new(big.Rat).Add(p.Height[0], p.Blend[0]), new(big.Rat).Sub(p.Height[1], p.Blend[1])}
	if z[0].Cmp(z[1]) >= 0 {
		return zero, ErrDomain
	}
	for _, q := range z {
		if e := budget.check(q); e != nil {
			return zero, e
		}
	}
	return z, nil
}

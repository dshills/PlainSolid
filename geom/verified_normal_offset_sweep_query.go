package geom

import (
	"context"
	"math/big"
)

// NormalOffsetSweepLocation is the outcome of an exact membership query.
type NormalOffsetSweepLocation uint8

const (
	NormalOffsetSweepUnresolved NormalOffsetSweepLocation = iota
	NormalOffsetSweepInterior
	NormalOffsetSweepExterior
	NormalOffsetSweepBoundary
)

// NormalOffsetSweepQueryOptions bounds inverse point certification.
// Zero allows 4096 visited cells, 30 partition levels and four million
// exact operations. The ordinary patch coefficient limits apply.
type NormalOffsetSweepQueryOptions struct {
	Patch              SurfacePatchOptions
	MaxDepth, MaxCells int
}

// NormalOffsetSweepQueryCell covers a possible inverse parameter point.
type NormalOffsetSweepQueryCell struct {
	UV           ParameterRectangle
	Displacement ExactParameterInterval
}

// NormalOffsetSweepQueryResult contains a directed U,V,offset enclosure
// for an interior or boundary certificate. It retains every unexcluded cell
// on ErrConvergence. Boundary points can remain unresolved.
type NormalOffsetSweepQueryResult struct {
	Location                      NormalOffsetSweepLocation
	Parameters                    [3]ExactParameterInterval
	Unresolved                    []NormalOffsetSweepQueryCell
	VisitedCells, ExactOperations int
}

// LocateRational classifies an exact model point in a verified normal
// sweep. It uses full cell enclosures to exclude candidates and an
// exact Krawczyk inclusion to prove existence. The original
// global sweep proof guarantees the inverse is unique.
func (s *VerifiedNormalOffsetSweep) LocateRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepQueryOptions) (r NormalOffsetSweepQueryResult, err error) {
	if ctx == nil || s == nil || s.source == nil || o.MaxDepth < 0 || o.MaxDepth > 64 ||
		o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return r, ErrInvalid
	}
	for _, q := range point {
		if q == nil {
			return r, ErrInvalid
		}
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 30
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	root := *s.source
	root.options.MaxPoles = min(root.options.MaxPoles, limits.MaxPoles)
	original := normalSweepParameterIntervals(root.Domain(), exactInterval{s.lower, s.upper})
	type node struct {
		patch        *SurfacePatch
		displacement exactInterval
		depth        int
	}
	stack := []node{{&root, exactInterval{s.lower, s.upper}, 0}}
	appendPending := func(n node) {
		r.Unresolved = append(r.Unresolved, NormalOffsetSweepQueryCell{n.patch.Domain(), copySweepInterval(n.displacement)})
	}
	defer func() {
		for i := len(stack) - 1; i >= 0; i-- {
			appendPending(stack[i])
		}
		r.ExactOperations = b.operations
	}()
	if len(s.source.net) > limits.MaxPoles {
		return r, ErrConvergence
	}
	for _, q := range [...]*big.Rat{point[0], point[1], point[2], s.lower, s.upper} {
		if err := b.check(q); err != nil {
			return r, err
		}
	}
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if r.VisitedCells >= o.MaxCells {
			return r, ErrConvergence
		}
		i := len(stack) - 1
		current := stack[i]
		r.VisitedCells++
		kind, bounds, err := normalSweepLocationCell(ctx, current.patch, current.displacement, original, point, b)
		if err != nil {
			return r, err
		}
		switch kind {
		case NormalOffsetSweepInterior, NormalOffsetSweepBoundary:
			r.Location = kind
			for i, x := range bounds {
				r.Parameters[i] = copySweepInterval(x)
			}
			// The verified sweep is globally one-to-one, so no other
			// cells can contain a second inverse.
			stack, r.Unresolved = nil, nil
			return r, nil
		case NormalOffsetSweepExterior:
			stack = stack[:i]
			continue
		}
		if current.depth == o.MaxDepth {
			appendPending(current)
			stack = stack[:i]
			continue
		}
		l, h := current, current
		l.depth, h.depth = current.depth+1, current.depth+1
		d := current.patch.Domain()
		axis := current.depth % 3
		t := parameterMidpoint(d.UMin, d.UMax)
		if axis == 0 && !(d.UMin < t && t < d.UMax) {
			axis = 1
		}
		if axis == 1 {
			t = parameterMidpoint(d.VMin, d.VMax)
			if !(d.VMin < t && t < d.VMax) {
				axis = 2
			}
		}
		if axis < 2 {
			lo, hi := d.UMin, d.UMax
			if axis == 1 {
				lo, hi = d.VMin, d.VMax
			}
			// Overlap the children around the central station so that an
			// interior inverse on a split can still have an interior box.
			left := parameterMidpoint(parameterMidpoint(lo, t), t)
			right := parameterMidpoint(t, parameterMidpoint(t, hi))
			if left <= lo || right >= hi || left == right {
				l.patch, h.patch, err = current.patch.split(t, axis == 0, b)
			} else {
				l.patch, _, err = current.patch.split(right, axis == 0, b)
				if err == nil {
					_, h.patch, err = current.patch.split(left, axis == 0, b)
				}
			}
			if err != nil {
				return r, err
			}
		} else {
			d := current.displacement
			m := exactSweepMidpoint(d)
			left := exactSweepMidpoint(exactInterval{exactSweepMidpoint(exactInterval{d.lo, m}), m})
			right := exactSweepMidpoint(exactInterval{m, exactSweepMidpoint(exactInterval{m, d.hi})})
			for _, q := range [2]*big.Rat{left, right} {
				if err := b.check(q); err != nil {
					return r, err
				}
			}
			l.displacement.hi, h.displacement.lo = right, left
		}
		stack = append(stack[:i], h, l)
	}
	if len(r.Unresolved) != 0 {
		return r, ErrConvergence
	}
	r.Location = NormalOffsetSweepExterior
	return r, nil
}

func copySweepInterval(x exactInterval) ExactParameterInterval {
	return ExactParameterInterval{Lower: new(big.Rat).Set(x.lo), Upper: new(big.Rat).Set(x.hi)}
}

func exactSweepMidpoint(x exactInterval) *big.Rat {
	return new(big.Rat).Quo(new(big.Rat).Add(x.lo, x.hi), big.NewRat(2, 1))
}

func normalSweepParameterIntervals(d ParameterRectangle, t exactInterval) [3]exactInterval {
	return [3]exactInterval{
		{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)},
		{new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)}, t,
	}
}

func normalSweepLocationCell(ctx context.Context, p *SurfacePatch, t exactInterval, original [3]exactInterval, query [3]*big.Rat,
	b *polynomialBudget) (NormalOffsetSweepLocation, [3]exactInterval, error) {
	var k [3]exactInterval
	d, err := offsetCellJacobianBounds(ctx, p, b)
	if d == nil || err != nil {
		return NormalOffsetSweepUnresolved, k, err
	}
	a := &d.arithmetic
	var jac [3][3]exactInterval
	for i := range jac {
		jac[i][2] = a.div(d.normal[i], d.length)
		box := a.add(d.point[i], a.mul(t, jac[i][2]))
		if a.err != nil {
			return NormalOffsetSweepUnresolved, k, a.err
		}
		if box.lo.Cmp(query[i]) > 0 || box.hi.Cmp(query[i]) < 0 {
			return NormalOffsetSweepExterior, k, nil
		}
	}
	var centerJac [3][3]*big.Rat
	for i := range jac {
		for j := 0; j < 2; j++ {
			jac[i][j] = a.add(d.tangent[j][i], a.mul(t, d.normalDerivative(i, j)))
		}
		if a.err != nil {
			return NormalOffsetSweepUnresolved, k, a.err
		}
		for j, x := range jac[i] {
			centerJac[i][j] = exactSweepMidpoint(x)
			if err := b.check(centerJac[i][j]); err != nil {
				return NormalOffsetSweepUnresolved, k, err
			}
		}
	}
	c, ok, err := inverseExact3(centerJac, b)
	if !ok || err != nil {
		return NormalOffsetSweepUnresolved, k, err
	}
	x := normalSweepParameterIntervals(p.Domain(), t)
	var m [3]*big.Rat
	var delta [3]exactInterval
	for i := range x {
		m[i] = exactSweepMidpoint(x[i])
		if err := b.check(m[i]); err != nil {
			return NormalOffsetSweepUnresolved, k, err
		}
		delta[i] = a.sub(x[i], exactInterval{m[i], m[i]})
	}
	if a.err != nil {
		return NormalOffsetSweepUnresolved, k, a.err
	}
	// G(x) = x − C(F(x) − query) shares its fixed points with F(x) = query.
	// Enclose G on this convex box with the interval mean value formula.
	// A disjoint image excludes a root; a self-map proves one by Brouwer.
	center, partials, err := exactSurfaceJet(p, m[0], m[1], b)
	if err != nil {
		return NormalOffsetSweepUnresolved, k, err
	}
	jet := RationalSurfacePatchJet{Point: center}
	for i := range partials {
		jet.Du[i], jet.Dv[i] = partials[i][0], partials[i][1]
	}
	if b.operations >= b.maxOperations {
		return NormalOffsetSweepUnresolved, k, ErrConvergence
	}
	f, err := jet.NormalOffsetRational(ctx, m[2], SurfaceNormalOffsetEvaluationOptions{
		MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
	b.operations += f.ExactOperations
	if err != nil {
		return NormalOffsetSweepUnresolved, k, err
	}
	var residual [3]exactInterval
	for i := range residual {
		residual[i] = a.sub(exactInterval{f.Lower[i], f.Upper[i]}, exactInterval{query[i], query[i]})
	}
	one := big.NewRat(1, 1)
	contained, interior, boundary := true, true, false
	for i := range k {
		k[i] = exactInterval{m[i], m[i]}
		for j := range residual {
			k[i] = a.sub(k[i], a.mul(exactInterval{c[i][j], c[i][j]}, residual[j]))
		}
		for j := range delta {
			e := offsetZeroInterval()
			if i == j {
				e = exactInterval{one, one}
			}
			for h := range jac {
				e = a.sub(e, a.mul(exactInterval{c[i][h], c[i][h]}, jac[h][j]))
			}
			k[i] = a.add(k[i], a.mul(e, delta[j]))
		}
		if a.err != nil {
			return NormalOffsetSweepUnresolved, k, a.err
		}
		if k[i].lo.Cmp(x[i].hi) > 0 || k[i].hi.Cmp(x[i].lo) < 0 {
			return NormalOffsetSweepExterior, k, nil
		}
		contained = contained && k[i].lo.Cmp(x[i].lo) >= 0 && k[i].hi.Cmp(x[i].hi) <= 0
		interior = interior && k[i].lo.Cmp(original[i].lo) > 0 && k[i].hi.Cmp(original[i].hi) < 0
		boundary = boundary || k[i].hi.Cmp(original[i].lo) == 0 || k[i].lo.Cmp(original[i].hi) == 0
	}
	if contained && interior {
		return NormalOffsetSweepInterior, k, nil
	}
	if contained && boundary {
		return NormalOffsetSweepBoundary, k, nil
	}
	return NormalOffsetSweepUnresolved, k, nil
}

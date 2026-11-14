package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// The five planar inequalities in the caller guard the original footprint.
// Below, internal XY knots select the local height polynomial only; they
// cannot introduce independent material faces or boundary events.
func (b *mixedRoofBox) piecewiseRoofStations(ctx context.Context, source int, origin, direction [3]*big.Rat, start, end *big.Rat, w *mixedRoofLineWork) (int, []mixedRoofLineStation, error) {
	u, v := b.roof.KnotBreaks()
	var cuts []*big.Rat
	for axis, knots := range [2][]float64{u, v} {
		if direction[axis].Sign() == 0 {
			continue
		}
		for _, knot := range knots {
			x, err := w.rat(new(big.Rat).Sub(ratInset(knot), origin[axis]))
			if err != nil {
				return 0, nil, err
			}
			if _, err = w.rat(x.Quo(x, direction[axis])); err != nil {
				return 0, nil, err
			}
			cuts = append(cuts, x)
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	coords := func(t *big.Rat) (xy [2]*big.Rat, err error) {
		for i := range xy {
			v, err := w.rat(new(big.Rat).Mul(direction[i], t))
			if err != nil {
				return xy, err
			}
			xy[i], err = w.rat(v.Add(v, origin[i]))
			if err != nil {
				return xy, err
			}
		}
		return xy, nil
	}
	cellAt := func(t *big.Rat) (*mixedRoofBox, error) {
		p, err := coords(t)
		if err != nil {
			return nil, err
		}
		for _, s := range b.roofSpans {
			if s.footprint(p) != parametric.Outside {
				return s, nil
			}
		}
		return nil, nil
	}
	mid := func(lo, hi *big.Rat) (*big.Rat, error) {
		a, err := w.rat(new(big.Rat).Add(lo, hi))
		if err != nil {
			return nil, err
		}
		return w.rat(a.Quo(a, big.NewRat(2, 1)))
	}
	near := func(t *big.Rat, left bool) (*mixedRoofBox, error) {
		for j, p := range cuts {
			if left {
				p = cuts[len(cuts)-1-j]
			}
			order := p.Cmp(t)
			if left && order < 0 || !left && order > 0 {
				m, err := mid(p, t)
				if err != nil {
					return nil, err
				}
				return cellAt(m)
			}
		}
		if len(cuts) == 0 { // constant XY
			return cellAt(t)
		}
		return nil, nil // before entry or after exit from the full footprint
	}
	type power struct {
		p      mixedRoofLinePoly
		degree int
	}
	cache := make(map[*mixedRoofBox]power)
	polynomial := func(s *mixedRoofBox) (power, error) {
		if p, ok := cache[s]; ok {
			return p, nil
		}
		all, err := s.linePolynomials(origin, direction, w)
		if err != nil {
			return power{}, err
		}
		p := power{all[5], len(all[5]) - 1}
		for p.degree >= 0 && p.p[p.degree].Sign() == 0 {
			p.degree--
		}
		cache[s] = p
		return p, nil
	}
	side := func(s *mixedRoofBox, t *big.Rat, left bool) (int, error) {
		if s == nil {
			return 1, nil // planar inequalities exclude this parameter sector
		}
		p, err := polynomial(s)
		if err != nil || p.degree < 0 {
			return 0, err
		}
		return w.sideSign(p.p, p.degree, t, left)
	}
	before, err := near(start, true)
	if err != nil {
		return 0, nil, err
	}
	initial, err := side(before, start, true)
	if err != nil {
		return 0, nil, err
	}
	parameters := []*big.Rat{start}
	for _, x := range cuts {
		if x.Cmp(start) > 0 && x.Cmp(end) < 0 && x.Cmp(parameters[len(parameters)-1]) != 0 {
			parameters = append(parameters, x)
		}
	}
	if start.Cmp(end) < 0 {
		parameters = append(parameters, end)
	}
	var events []mixedRoofLineStation
	for i, t := range parameters {
		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}
		left, err := near(t, true)
		if err != nil {
			return 0, nil, err
		}
		right, err := near(t, false)
		if err != nil {
			return 0, nil, err
		}
		l, err := side(left, t, true)
		if err != nil {
			return 0, nil, err
		}
		r, err := side(right, t, false)
		if err != nil {
			return 0, nil, err
		}
		s, err := cellAt(t)
		if err != nil {
			return 0, nil, err
		}
		at := 1
		if s != nil {
			p, err := polynomial(s)
			if err != nil {
				return 0, nil, err
			}
			at, err = w.value(p.p, t)
			if err != nil {
				return 0, nil, err
			}
		}
		if l != r || l != at {
			o, err := w.rootOptions()
			if err != nil {
				return 0, nil, err
			}
			root, n, err := geom.NewExactRationalRoot(ctx, t, o)
			if err != nil {
				return 0, nil, err
			}
			w.operations += n
			events = append(events, mixedRoofLineStation{root: root, where: []mixedRoofCrossing{{source: source, direction: r, at: at, event: at == 0 && (l != 0 || r != 0)}}})
		}
		if i+1 == len(parameters) || right == nil {
			continue
		}
		p, err := polynomial(right)
		if err != nil {
			return 0, nil, err
		}
		if p.degree < 1 {
			continue
		}
		o, err := w.rootOptions()
		if err != nil {
			return 0, nil, err
		}
		roots, err := geom.IsolateExactPolynomialRoots(ctx, p.p[:p.degree+1], t, parameters[i+1], o)
		if err != nil {
			return 0, nil, err
		}
		w.operations += roots.Operations
		w.subdivisions += roots.Subdivisions
		for _, root := range roots.Roots {
			v := root.Root.Interval()
			if v.Lower.Cmp(v.Upper) == 0 && (v.Lower.Cmp(t) == 0 || v.Lower.Cmp(parameters[i+1]) == 0) {
				continue // already certified using both neighboring pieces
			}
			if root.Multiplicity%2 != 0 {
				r = -r
			}
			events = append(events, mixedRoofLineStation{root: root.Root, where: []mixedRoofCrossing{{source: source, direction: r, event: true}}})
		}
	}
	return initial, events, nil
}

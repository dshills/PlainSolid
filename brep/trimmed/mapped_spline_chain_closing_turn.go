package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// mappedClosingAxisClock proves an exact affine or projective clock on an
// original isoparametric coedge. Its stored controls may be degree-elevated,
// but each homogeneous coordinate must be the same linear interpolation.
type mappedClosingAxisClock struct {
	curve      PCurve
	axis       int
	start, end [2]*big.Rat
	weight     [2]*big.Rat
}

func mappedClosingClock(trim PCurve) (mappedClosingAxisClock, bool) {
	h, err := mixedExactControls(trim)
	if err != nil || len(h) < 2 {
		return mappedClosingAxisClock{}, false
	}
	for _, p := range h {
		if p[0] == nil || p[1] == nil || p[2] == nil || p[2].Sign() <= 0 {
			return mappedClosingAxisClock{}, false
		}
	}
	d := len(h) - 1
	a, b := h[0], h[d]
	start, end := [2]*big.Rat{}, [2]*big.Rat{}
	for i := 0; i < 2; i++ {
		start[i], end[i] = new(big.Rat).Quo(a[i], a[2]), new(big.Rat).Quo(b[i], b[2])
	}
	axis := 0
	if start[0].Cmp(end[0]) == 0 {
		axis = 1
	}
	if start[axis].Cmp(end[axis]) == 0 || start[1-axis].Cmp(end[1-axis]) != 0 {
		return mappedClosingAxisClock{}, false
	}
	for i := 1; i < d; i++ {
		f := big.NewRat(int64(i), int64(d))
		for k := range a {
			expected := new(big.Rat).Add(a[k], new(big.Rat).Mul(f, new(big.Rat).Sub(b[k], a[k])))
			if expected.Cmp(h[i][k]) != 0 {
				return mappedClosingAxisClock{}, false
			}
		}
	}
	return mappedClosingAxisClock{curve: trim, axis: axis, start: start, end: end, weight: [2]*big.Rat{a[2], b[2]}}, true
}

func (c mappedClosingAxisClock) point(x *big.Rat) [2]*big.Rat {
	uv := c.start
	uv[c.axis] = x
	return uv
}

// parameter inverts the original positive clock at an exact physical chart
// coordinate and checks the complete original pcurve at the result.
func (c mappedClosingAxisClock) parameter(x *big.Rat) (*big.Rat, error) {
	if x == nil {
		return nil, geom.ErrInvalid
	}
	f := new(big.Rat).Quo(new(big.Rat).Sub(x, c.start[c.axis]), new(big.Rat).Sub(c.end[c.axis], c.start[c.axis]))
	if f.Sign() < 0 || f.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	a := new(big.Rat).Mul(f, c.weight[0])
	b := new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), f), c.weight[1])
	t := new(big.Rat).Quo(a, new(big.Rat).Add(a, b))
	uv, err := mixedExactEvaluate(c.curve, t)
	if err != nil {
		return nil, err
	}
	if !rationalGraphSameUV(uv, c.point(x)) {
		return nil, geom.ErrConvergence
	}
	return t, nil
}

type closingTurnProbe struct {
	ctx       context.Context
	patch     *geom.SurfacePatch
	knots     [2][]float64
	cells     map[[2]int]*geom.SurfacePatch
	remaining int
}

func (s *closingTurnProbe) cell(c mappedClosingAxisClock, a, b *big.Rat) (*geom.SurfacePatch, error) {
	a, b = minMaxRats(a, b)
	lo, hi := c.point(a), c.point(b)
	var id [2]int
	for axis, breaks := range s.knots {
		slot := -1
		for i := 0; i+1 < len(breaks); i++ {
			if lo[axis].Cmp(ratInset(breaks[i])) >= 0 && hi[axis].Cmp(ratInset(breaks[i+1])) <= 0 {
				if slot != -1 {
					return nil, nil
				}
				slot = i
			}
		}
		if slot < 0 {
			return nil, geom.ErrConvergence
		}
		id[axis] = slot
	}
	if p := s.cells[id]; p != nil {
		return p, nil
	}
	u, v := id[0], id[1]
	p, err := s.patch.Trim(geom.ParameterRectangle{
		UMin: s.knots[0][u], UMax: s.knots[0][u+1], VMin: s.knots[1][v], VMax: s.knots[1][v+1],
	})
	if err != nil {
		return nil, err
	}
	s.cells[id] = p
	return p, nil
}

func minMaxRats(a, b *big.Rat) (*big.Rat, *big.Rat) {
	if a.Cmp(b) > 0 {
		return b, a
	}
	return a, b
}

func (s *closingTurnProbe) normal(p *geom.SurfacePatch, c mappedClosingAxisClock, x *big.Rat) ([3]*big.Rat, error) {
	if err := s.ctx.Err(); err != nil {
		return [3]*big.Rat{}, err
	}
	if s.remaining == 0 {
		return [3]*big.Rat{}, geom.ErrConvergence
	}
	s.remaining--
	uv := c.point(x)
	j, err := p.EvaluateRational(s.ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		return [3]*big.Rat{}, err
	}
	return ddCross(j.Du, j.Dv), nil
}

// A conservative rational upper bound for tan(a), for 0<a<=1.
func mappedAngleTangentUpper(a float64) *big.Rat {
	x := ratInset(a)
	x2 := new(big.Rat).Mul(x, x)
	x3 := new(big.Rat).Mul(x2, x)
	x5 := new(big.Rat).Mul(x3, x2)
	sinUpper := new(big.Rat).Sub(x, new(big.Rat).Quo(x3, big.NewRat(6, 1)))
	sinUpper.Add(sinUpper, new(big.Rat).Quo(x5, big.NewRat(120, 1)))
	cosLower := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(x2, big.NewRat(2, 1)))
	return sinUpper.Quo(sinUpper, cosLower)
}

// obstructs is an exact necessary-condition test for one display edge. It
// only starts this extra refinement when two original normals provably exceed
// the full requested turn. The samples schedule work; the complete one-sided
// surface cone is still required for every display triangle.
func (s *closingTurnProbe) obstructs(p *geom.SurfacePatch, c mappedClosingAxisClock, a, b, squaredUpper *big.Rat) (bool, error) {
	var normals [17][3]*big.Rat
	width := new(big.Rat).Sub(b, a)
	for i := range normals {
		x := new(big.Rat).Add(a, new(big.Rat).Mul(width, big.NewRat(int64(i), 16)))
		n, err := s.normal(p, c, x)
		if err != nil {
			return false, err
		}
		for _, prior := range normals[:i] {
			if q, ok := ddTanSquared(prior, n); !ok || q.Cmp(squaredUpper) > 0 {
				return true, nil
			}
		}
		normals[i] = n
	}
	return false, nil
}

func (s *closingTurnProbe) split(p *geom.SurfacePatch, c mappedClosingAxisClock, a, b, limit *big.Rat, depth int, force bool, cuts *[]*big.Rat) error {
	mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	center, err := s.normal(p, c, mid)
	if err != nil {
		return err
	}
	straight := true
	q1 := new(big.Rat).Quo(new(big.Rat).Add(a, mid), big.NewRat(2, 1))
	q3 := new(big.Rat).Quo(new(big.Rat).Add(mid, b), big.NewRat(2, 1))
	for _, x := range []*big.Rat{a, q1, q3, b} {
		n, err := s.normal(p, c, x)
		if err != nil {
			return err
		}
		if q, ok := ddTanSquared(center, n); !ok || q.Cmp(limit) > 0 {
			straight = false
			break
		}
	}
	if straight && !force {
		return nil
	}
	if depth == 0 {
		return geom.ErrConvergence
	}
	t, err := c.parameter(mid)
	if err != nil {
		return err
	}
	*cuts = append(*cuts, t)
	if err := s.split(p, c, a, mid, limit, depth-1, false, cuts); err != nil {
		return err
	}
	return s.split(p, c, mid, b, limit, depth-1, false, cuts)
}

func mappedClosingTurnCuts(ctx context.Context, p *geom.SurfacePatch, trims []PCurve, prescribed [][]*big.Rat, angle float64) ([][]*big.Rat, error) {
	// There are at most sixteen original closing trims per roof. Normal
	// probes share a fixed budget across all of them, including both sides
	// of original varying-coordinate knots.
	u, v := p.KnotBreaks()
	probe := closingTurnProbe{ctx: ctx, patch: p, knots: [2][]float64{u, v}, cells: make(map[[2]int]*geom.SurfacePatch), remaining: 2048}
	upper := mappedAngleTangentUpper(angle)
	upper.Mul(upper, upper)
	limit := mappedAngleTangentLimit(angle / 4)
	out := slices.Clone(prescribed)
	for i, trim := range trims {
		clock, ok := mappedClosingClock(trim)
		if !ok {
			continue
		}
		a, b := minMaxRats(clock.start[clock.axis], clock.end[clock.axis])
		stations := []*big.Rat{a, b}
		for _, knot := range probe.knots[clock.axis] {
			q := ratInset(knot)
			if a.Cmp(q) < 0 && b.Cmp(q) > 0 {
				stations = append(stations, q)
			}
		}
		for _, t := range prescribed[i] {
			uv, err := mixedExactEvaluate(trim, t)
			if err != nil {
				return nil, err
			}
			stations = append(stations, uv[clock.axis])
		}
		slices.SortFunc(stations, (*big.Rat).Cmp)
		stations = slices.CompactFunc(stations, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
		var inserts []*big.Rat
		for j, start := range stations[:len(stations)-1] {
			end := stations[j+1]
			cell, err := probe.cell(clock, start, end)
			if err != nil {
				return nil, err
			}
			if cell == nil {
				continue
			}
			needs, err := probe.obstructs(cell, clock, start, end, upper)
			if err != nil {
				return nil, err
			}
			if needs {
				if err := probe.split(cell, clock, start, end, limit, 5, true, &inserts); err != nil {
					return nil, fmt.Errorf("closing coedge %d angular stations: %w", i, err)
				}
			}
		}
		if len(inserts) > 0 {
			cuts := append(slices.Clone(prescribed[i]), inserts...)
			cuts = append(cuts, new(big.Rat), big.NewRat(1, 1))
			slices.SortFunc(cuts, (*big.Rat).Cmp)
			out[i] = slices.CompactFunc(cuts, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
		}
	}
	return out, nil
}

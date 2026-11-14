package geom

import (
	"context"
	"math/big"
)

// WrappedCurveCellsOptions bounds aggregate arithmetic, isolated events and
// subdivisions. MaxStations bounds translated knot planes before enumeration.
type WrappedCurveCellsOptions struct {
	SurfacePatchOptions
	MaxEvents, MaxSubdivisions, MaxStations int
}

// WrappedCurveCell retains original global normalized t, original authored UV,
// selected fundamental cell and exact integer lift. Both immutable descriptors
// retain algebraic endpoints; neither represents a rounded native trim edge.
type WrappedCurveCell struct {
	UV, Model   *ExactComposedCurveCell
	PeriodShift [2]*big.Int
}

// WrappedCurveCells is an immutable complete continuous composition proof.
// Piece returns owned period integers and immutable source/model descriptors.
type WrappedCurveCells struct {
	pieces                   []WrappedCurveCell
	operations, subdivisions int
}

func (c *WrappedCurveCells) Len() int {
	if c == nil {
		return 0
	}
	return len(c.pieces)
}
func (c *WrappedCurveCells) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}
func (c *WrappedCurveCells) Subdivisions() int {
	if c == nil {
		return 0
	}
	return c.subdivisions
}
func (c *WrappedCurveCells) Piece(i int) (WrappedCurveCell, error) {
	if c == nil || i < 0 || i >= len(c.pieces) {
		return WrappedCurveCell{}, ErrInvalid
	}
	p := c.pieces[i]
	for a := range p.PeriodShift {
		p.PeriodShift[a] = new(big.Int).Set(p.PeriodShift[a])
	}
	return p, nil
}

// BuildParameterCurveCells composes a positive rational linear/quadratic UV
// Bezier across rational or irrational seams and knots. Exact enumeration of
// every translated knot plane makes a rational witness select the entire open
// cell. Continuity and independently checked endpoint identities extend this
// proof to each closed interval. Positive source and UV weights imply positive
// composed denominators on the selected domain. Original UV is never reduced.
// Coincidence with a knot plane uses one closed incident cell; no derivative
// continuity is asserted. All errors return nil, including work exhaustion.
func (w *WrappedSurfacePatch) BuildParameterCurveCells(ctx context.Context, uv *ExactSplineCurve, o WrappedCurveCellsOptions) (*WrappedCurveCells, error) {
	if ctx == nil || w == nil || w.patch == nil || uv == nil || uv.patch == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxStations == 0 {
		o.MaxStations = 4096
	}
	if o.MaxEvents < 2 || o.MaxEvents > 65536 || o.MaxSubdivisions < 1 || o.MaxSubdivisions > 1048576 || o.MaxStations < 1 || o.MaxStations > 65536 {
		return nil, ErrInvalid
	}
	p := uv.patch
	lo, hi := uv.Domain()
	if lo != 0 || hi != 1 || p.degreeU < 1 || p.degreeU > 2 || p.countU != p.degreeU+1 || len(p.net) > opts.MaxPoles || len(w.patch.net) > opts.MaxPoles {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	steps := 0
	var coord [3]rationalPolynomial
	var hull [2][2]*big.Rat
	for i := 0; i < p.countU; i++ {
		h := p.net[i*p.countV]
		if h[2].Sign() != 0 || h[3].Sign() <= 0 {
			return nil, ErrInvalid
		}
		for _, q := range h {
			if e := b.check(q); e != nil {
				return nil, e
			}
		}
		for axis := 0; axis < 2; axis++ {
			q := new(big.Rat).Quo(h[axis], h[3])
			if e := b.check(q); e != nil {
				return nil, e
			}
			if hull[axis][0] == nil || q.Cmp(hull[axis][0]) < 0 {
				hull[axis][0] = q
			}
			if hull[axis][1] == nil || q.Cmp(hull[axis][1]) > 0 {
				hull[axis][1] = q
			}
		}
	}
	for axis, component := range []int{0, 1, 3} {
		v := make([]*big.Rat, p.countU)
		for i := range v {
			v[i] = p.net[i*p.countV][component]
		}
		coord[axis], e = bernsteinToPower(v, b)
		if e != nil {
			return nil, e
		}
	}
	events := []*ExactGraphLineRoot{{polynomial: rationalPolynomial{new(big.Rat), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, {polynomial: rationalPolynomial{big.NewRat(-1, 1), big.NewRat(1, 1)}, bounds: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}}
	add := func(event *ExactGraphLineRoot) error {
		for i := len(events) - 1; i >= 0; i-- {
			if steps >= o.MaxSubdivisions {
				return ErrConvergence
			}
			c, e := compareRootBudget(ctx, event, events[i], b, o.MaxSubdivisions-steps)
			steps += c.Subdivisions
			if e != nil {
				return e
			}
			if !c.Proven {
				return ErrConvergence
			}
			event.bounds = c.Intervals[0]
			events[i].bounds = c.Intervals[1]
			if c.Order == 0 {
				return nil
			}
			if c.Order > 0 {
				if len(events) >= o.MaxEvents {
					return ErrConvergence
				}
				events = append(events, nil)
				copy(events[i+2:], events[i+1:])
				events[i+1] = event
				return nil
			}
		}
		if len(events) >= o.MaxEvents {
			return ErrConvergence
		}
		events = append(events, nil)
		copy(events[1:], events[:len(events)-1])
		events[0] = event
		return nil
	}
	floor := func(q *big.Rat) *big.Int {
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if r.Sign() < 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	d := w.Domain()
	mins := [2]float64{d.UMin, d.VMin}
	stations := 0
	for axis, knots := range [][]float64{w.patch.knotsU, w.patch.knotsV} {
		first, last := new(big.Int), new(big.Int)
		if period := w.period[axis]; period != nil {
			origin := new(big.Rat).SetFloat64(mins[axis])
			first = floor(new(big.Rat).Quo(new(big.Rat).Sub(hull[axis][0], origin), period))
			last = floor(new(big.Rat).Quo(new(big.Rat).Sub(hull[axis][1], origin), period))
			n := new(big.Int).Sub(last, first)
			if !n.IsInt64() || n.Int64() > int64(o.MaxStations) {
				return nil, ErrConvergence
			}
		}
		seen := map[string]bool{}
		for turn := new(big.Int).Set(first); turn.Cmp(last) <= 0; turn.Add(turn, big.NewInt(1)) {
			for j, knot := range knots {
				if j > 0 && knot == knots[j-1] {
					continue
				}
				stations++
				if stations > o.MaxStations {
					return nil, ErrConvergence
				}
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				station := new(big.Rat).SetFloat64(knot)
				if period := w.period[axis]; period != nil {
					station.Add(station, new(big.Rat).Mul(new(big.Rat).SetInt(turn), period))
				}
				if e := b.check(station); e != nil {
					return nil, e
				}
				if station.Cmp(hull[axis][0]) < 0 || station.Cmp(hull[axis][1]) > 0 || seen[station.RatString()] {
					continue
				}
				seen[station.RatString()] = true
				poly, e := subtractScaledPolynomial(coord[axis], coord[2], station, b)
				if e != nil {
					return nil, e
				}
				if len(poly) == 0 {
					continue
				}
				if steps >= o.MaxSubdivisions {
					return nil, ErrConvergence
				}
				var staged []*ExactGraphLineRoot
				n, e := isolatePolynomialRoots(poly, new(big.Rat), big.NewRat(1, 1), b, o.MaxSubdivisions-steps, func(lo, hi *big.Rat, _ int) (bool, error) {
					if e := ctx.Err(); e != nil {
						return false, e
					}
					if lo.Cmp(hi) != 0 {
						for _, bound := range []*big.Rat{lo, hi} {
							v, e := polynomialValue(poly, bound, b)
							if e != nil {
								return false, e
							}
							if v.Sign() == 0 {
								return false, nil
							}
						}
					}
					if len(staged) >= o.MaxEvents {
						return false, ErrConvergence
					}
					staged = append(staged, &ExactGraphLineRoot{polynomial: poly, bounds: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}})
					return true, nil
				})
				steps += n
				if e != nil {
					return nil, e
				}
				for _, event := range staged {
					if e := add(event); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	out := &WrappedCurveCells{}
	for i := 0; i+1 < len(events); i++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if events[i].bounds[1].Cmp(events[i+1].bounds[0]) >= 0 {
			return nil, ErrConvergence
		}
		witness := new(big.Rat).Quo(new(big.Rat).Add(events[i].bounds[1], events[i+1].bounds[0]), big.NewRat(2, 1))
		point, e := exactCurveRationalPoint(p, witness, b)
		if e != nil {
			return nil, e
		}
		local := coord
		piece := WrappedCurveCell{}
		var bounds [2][2]float64
		for axis, knots := range [][]float64{w.patch.knotsU, w.patch.knotsV} {
			shift := new(big.Int)
			if period := w.period[axis]; period != nil {
				shift = floor(new(big.Rat).Quo(new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(mins[axis])), period))
				offset := new(big.Rat).Mul(new(big.Rat).SetInt(shift), period)
				point[axis] = new(big.Rat).Sub(point[axis], offset)
				local[axis], e = subtractScaledPolynomial(coord[axis], coord[2], offset, b)
				if e != nil {
					return nil, e
				}
			}
			piece.PeriodShift[axis] = shift
			found := false
			for j := 0; j+1 < len(knots); j++ {
				l, h := knots[j], knots[j+1]
				if l >= h {
					continue
				}
				if point[axis].Cmp(new(big.Rat).SetFloat64(l)) >= 0 && point[axis].Cmp(new(big.Rat).SetFloat64(h)) <= 0 {
					bounds[axis] = [2]float64{l, h}
					found = true
					break
				}
			}
			if !found {
				return nil, ErrDomain
			}
		}
		domain := ParameterRectangle{UMin: bounds[0][0], UMax: bounds[0][1], VMin: bounds[1][0], VMax: bounds[1][1]}
		patch, e := trimPatchBudget(w.patch, domain, b)
		if e != nil {
			return nil, e
		}
		power, e := symbolicSurfaceCurvePower(ctx, patch, local, b)
		if e != nil {
			return nil, e
		}
		endpoints := [2]*ExactGraphLineRoot{copyExactRoot(events[i]), copyExactRoot(events[i+1])}
		piece.Model = &ExactComposedCurveCell{power: power, endpoints: endpoints, cell: domain}
		piece.UV = &ExactComposedCurveCell{power: [4]rationalPolynomial{coord[0], coord[1], nil, coord[2]}, endpoints: endpoints}
		// Prove each model join at the actual root by homogeneous cross products.
		if i > 0 {
			prior := out.pieces[i-1].Model
			for axis := 0; axis < 3; axis++ {
				a, e := polynomialProduct(prior.power[axis], power[3], b)
				if e != nil {
					return nil, e
				}
				c, e := polynomialProduct(power[axis], prior.power[3], b)
				if e != nil {
					return nil, e
				}
				delta, e := subtractScaledPolynomial(a, c, big.NewRat(1, 1), b)
				if e != nil {
					return nil, e
				}
				if steps >= o.MaxSubdivisions {
					return nil, ErrConvergence
				}
				sign, n, e := composedPolynomialRootSign(ctx, events[i], delta, b, o.MaxSubdivisions-steps)
				steps += n
				if e != nil {
					return nil, e
				}
				if sign != 0 {
					return nil, ErrConvergence
				}
			}
		}
		out.pieces = append(out.pieces, piece)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.operations = b.operations
	out.subdivisions = steps
	return out, nil
}

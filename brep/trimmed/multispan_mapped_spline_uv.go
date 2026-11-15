package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MultiSpanMappedSplineUV preserves three through eight original cubic/quartic
// source spans with independent positive Möbius speeds. Every source knot has
// degree-fold multiplicity and exact C0 ownership. All mapped pieces must prove
// the same complete model-normalized Bézier. Restricted descendants preserve the
// entire original source and at least two active spans.
type MultiSpanMappedSplineUV struct {
	source   SplineUV
	stations []*big.Rat
	parts    []MappedSplineUV
	curve    ExactBezierUV
}

// NewMultiSpanMappedSplineUV accepts a contiguous interval of an original clamped source domain.
// On each consecutive model interval, its normalized local t is mapped through
// the corresponding positive weight, then through the original span and reversal.
// There are at most eight bounded single-span mapped constructions and eight
// complete comparisons, retaining their existing independent o limits. Extension
// is bounded as NewPiecewiseMappedSplineUV. Station/weight coefficients have
// at most 4096 bits.
func NewMultiSpanMappedSplineUV(ctx context.Context, source SplineUV, stations []*big.Rat, weights []*big.Rat, o geom.SurfacePatchOptions) (MultiSpanMappedSplineUV, error) {
	var zero MultiSpanMappedSplineUV
	if ctx == nil || source.Curve == nil || len(weights) < 2 || len(weights) > 8 || len(stations) != len(weights)-1 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	prior := new(big.Rat)
	for _, s := range stations {
		if s == nil {
			return zero, geom.ErrInvalid
		}
		if s.Num().BitLen() > 4096 || s.Denom().BitLen() > 4096 {
			return zero, geom.ErrConvergence
		}
		if s.Cmp(prior) <= 0 || s.Cmp(big.NewRat(1, 1)) >= 0 {
			return zero, geom.ErrInvalid
		}
		prior = s
	}
	count, n := len(weights), source.Curve.Degree()
	if n < 3 || n > 4 {
		return zero, geom.ErrConvergence
	}
	total := (len(source.Curve.Poles()) - 1) / n
	knots := source.Curve.Knots()
	lo, hi := source.Curve.Domain()
	if n < 3 || n > 4 || total < 3 || total > 8 || !finite(source.Range.Min) || !finite(source.Range.Max) || source.Range.Min < lo || source.Range.Max > hi || source.Range.Min >= source.Range.Max || len(source.Curve.Poles()) != n*total+1 || len(knots) != n*(total+1)+2 {
		return zero, geom.ErrConvergence
	}
	raw := make([]float64, total+1)
	raw[0], raw[total] = lo, hi
	for i := 1; i < total; i++ {
		raw[i] = knots[n+1+(i-1)*n]
		if raw[i] <= raw[i-1] || raw[i] >= hi {
			return zero, geom.ErrConvergence
		}
	}
	cursor := 0
	for i, v := range raw {
		multiplicity := n
		if i == 0 || i == total {
			multiplicity++
		}
		for j := 0; j < multiplicity; j++ {
			if knots[cursor] != v {
				return zero, geom.ErrConvergence
			}
			cursor++
		}
	}
	active := []float64{source.Range.Min}
	for _, k := range raw {
		if k > source.Range.Min && k < source.Range.Max {
			active = append(active, k)
		}
	}
	active = append(active, source.Range.Max)
	if len(active) != count+1 {
		return zero, geom.ErrConvergence
	}
	raw = active
	parts := make([]MappedSplineUV, count)
	for i := range parts {
		index := i
		if source.Reversed {
			index = count - 1 - i
		}
		part := source
		part.Range = UVInterval{raw[index], raw[index+1]}
		q, e := NewMappedSplineUV(ctx, part, weights[i], o)
		if e != nil {
			return zero, e
		}
		parts[i] = q
		if i > 0 && parts[i-1].curve.ends[1] != q.curve.ends[0] {
			return zero, geom.ErrConvergence
		}
	}
	whole, e := extendMappedBezier(ctx, parts[0].curve, stations[0], o)
	if e != nil {
		return zero, e
	}
	a := new(big.Rat)
	for i, p := range parts {
		b := big.NewRat(1, 1)
		if i < len(stations) {
			b = stations[i]
		}
		expected, e := restrictMappedProofCurve(whole, a, b)
		if e != nil {
			return zero, e
		}
		if e = proveMappedRepairCurve(ctx, p.curve, expected, o); e != nil {
			return zero, e
		}
		a = b
	}
	out := MultiSpanMappedSplineUV{source: source, parts: parts, curve: whole}
	for _, s := range stations {
		out.stations = append(out.stations, new(big.Rat).Set(s))
	}
	return out, nil
}
func (q MultiSpanMappedSplineUV) Source() SplineUV { return q.source }
func (q MultiSpanMappedSplineUV) ModelStations() []*big.Rat {
	out := make([]*big.Rat, len(q.stations))
	for i, s := range q.stations {
		out[i] = new(big.Rat).Set(s)
	}
	return out
}
func (q MultiSpanMappedSplineUV) ParameterWeights() []*big.Rat {
	out := make([]*big.Rat, len(q.parts))
	for i, p := range q.parts {
		out[i] = p.ParameterWeight()
	}
	return out
}
func (q MultiSpanMappedSplineUV) AuthoredParameter(t *big.Rat) (*big.Rat, error) {
	if len(q.parts) < 2 || t == nil {
		return nil, geom.ErrDomain
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return nil, geom.ErrConvergence
	}
	if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	a := new(big.Rat)
	for i, p := range q.parts {
		b := big.NewRat(1, 1)
		if i < len(q.stations) {
			b = q.stations[i]
		}
		if t.Cmp(b) <= 0 {
			return p.AuthoredParameter(new(big.Rat).Quo(new(big.Rat).Sub(t, a), new(big.Rat).Sub(b, a)))
		}
		a = b
	}
	return nil, geom.ErrDomain
}
func (q MultiSpanMappedSplineUV) EvalUV(t float64) (geom.Vec2, error) { return q.curve.EvalUV(t) }
func (q MultiSpanMappedSplineUV) DerivativeUV(t float64) (geom.Vec2, error) {
	return q.curve.DerivativeUV(t)
}
func (q MultiSpanMappedSplineUV) BoundsUV() (geom.UVBox, error) { return q.curve.BoundsUV() }
func hasNativeMultiSpanMappedSpline(s Solid) bool {
	var has func(PCurve) bool
	has = func(c PCurve) bool {
		switch q := c.(type) {
		case MultiSpanMappedSplineUV:
			return true
		case subTrim:
			return has(q.base)
		case reverseTrim:
			return has(q.differentiableTrim)
		}
		return false
	}
	for _, e := range s.edges {
		if m, ok := e.Curve.(surfaceTrimCurve); ok && has(m.curve) {
			return true
		}
	}
	for _, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if has(u.Curve) {
					return true
				}
			}
		}
	}
	return false
}
func originalMappedStations(c PCurve) []*big.Rat {
	switch q := c.(type) {
	case PiecewiseMappedSplineUV:
		if q.station != nil {
			return []*big.Rat{q.ModelStation()}
		}
	case MultiSpanMappedSplineUV:
		return q.ModelStations()
	}
	return nil
}

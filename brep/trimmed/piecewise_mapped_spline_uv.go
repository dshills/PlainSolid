package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PiecewiseMappedSplineUV retains exactly two original cubic/quartic knot spans
// with independent positive Möbius source speeds. The entire mapped trace must
// equal one exact model-normalized Bézier; its source is not replaced. The
// original knot is a mandatory display station, not a new topological corner.
// On [0,s] the first source span uses phi_w0(t/s); on [s,1] the second
// uses phi_w1((t-s)/(1-s)), followed by its original range/reversal.
type PiecewiseMappedSplineUV struct {
	source  SplineUV
	station *big.Rat
	parts   [2]MappedSplineUV
	curve   ExactBezierUV
}

// NewPiecewiseMappedSplineUV requires the whole original clamped source domain,
// one interior knot of multiplicity degree, exact C0 agreement, and complete
// mapped identity on both sides of modelStation. Source restrictions/comparisons
// each retain o's work limits. Candidate extension adds at most200 rational
// operations with the supplied bit limit (at most16384); native coefficients
// remain bounded to4096 bits. All failure is atomic.
func NewPiecewiseMappedSplineUV(ctx context.Context, source SplineUV, modelStation *big.Rat, weights [2]*big.Rat, o geom.SurfacePatchOptions) (PiecewiseMappedSplineUV, error) {
	var zero PiecewiseMappedSplineUV
	if ctx == nil || source.Curve == nil || modelStation == nil || modelStation.Sign() <= 0 || modelStation.Cmp(big.NewRat(1, 1)) >= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if modelStation.Num().BitLen() > 4096 || modelStation.Denom().BitLen() > 4096 {
		return zero, geom.ErrConvergence
	}
	n := source.Curve.Degree()
	knots := source.Curve.Knots()
	lo, hi := source.Curve.Domain()
	if n < 3 || n > 4 || source.Range != (UVInterval{lo, hi}) || len(source.Curve.Poles()) != 2*n+1 || len(knots) != 3*n+2 {
		return zero, geom.ErrConvergence
	}
	cut := knots[n+1]
	if cut <= lo || cut >= hi {
		return zero, geom.ErrConvergence
	}
	for i, k := range knots {
		expected := lo
		if i > n {
			expected = cut
		}
		if i > 2*n {
			expected = hi
		}
		if k != expected {
			return zero, geom.ErrConvergence
		}
	}
	a, b := source, source
	if source.Reversed {
		a.Range.Min = cut
		b.Range.Max = cut
	} else {
		a.Range.Max = cut
		b.Range.Min = cut
	}
	first, e := NewMappedSplineUV(ctx, a, weights[0], o)
	if e != nil {
		return zero, e
	}
	second, e := NewMappedSplineUV(ctx, b, weights[1], o)
	if e != nil {
		return zero, e
	}
	if first.curve.ends[1] != second.curve.ends[0] {
		return zero, geom.ErrConvergence
	}
	whole, e := extendMappedBezier(ctx, first.curve, modelStation, o)
	if e != nil {
		return zero, e
	}
	x, y, e := whole.Split(modelStation)
	if e != nil {
		return zero, e
	}
	for i, p := range []MappedSplineUV{first, second} {
		if e := proveMappedRepairCurve(ctx, p.curve, []ExactBezierUV{x, y}[i], o); e != nil {
			return zero, e
		}
	}
	return PiecewiseMappedSplineUV{source: source, station: new(big.Rat).Set(modelStation), parts: [2]MappedSplineUV{first, second}, curve: whole}, nil
}

func extendMappedBezier(ctx context.Context, a ExactBezierUV, t *big.Rat, o geom.SurfacePatchOptions) (ExactBezierUV, error) {
	var zero ExactBezierUV
	maxOps, maxBits := o.MaxOperations, o.MaxCoefficientBits
	if maxOps == 0 {
		maxOps = 1000000
	}
	if maxBits == 0 || maxBits > 16384 {
		maxBits = 16384
	}
	if maxOps < 1 || maxBits < 1 {
		return zero, geom.ErrInvalid
	}
	count := 0
	check := func(x *big.Rat) error {
		count++
		if count > maxOps || x.Num().BitLen() > maxBits || x.Denom().BitLen() > maxBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	n := len(a.control)
	u := new(big.Rat).Sub(big.NewRat(1, 1), t)
	if e := check(u); e != nil {
		return zero, e
	}
	powT, powU := [5]*big.Rat{big.NewRat(1, 1)}, [5]*big.Rat{big.NewRat(1, 1)}
	for i := 1; i < n; i++ {
		powT[i] = new(big.Rat).Mul(powT[i-1], t)
		if e := check(powT[i]); e != nil {
			return zero, e
		}
		powU[i] = new(big.Rat).Mul(powU[i-1], u)
		if e := check(powU[i]); e != nil {
			return zero, e
		}
	}
	h := make([][3]*big.Rat, n)
	for i := range h {
		for k, c := range a.control[i] {
			x := new(big.Rat).Set(c)
			for j := 0; j < i; j++ {
				w := new(big.Rat).Mul(powT[j], powU[i-j])
				if e := check(w); e != nil {
					return zero, e
				}
				w.Mul(w, big.NewRat(polynomialChoose(i, j), 1))
				if e := check(w); e != nil {
					return zero, e
				}
				w.Mul(w, h[j][k])
				if e := check(w); e != nil {
					return zero, e
				}
				x.Sub(x, w)
				if e := check(x); e != nil {
					return zero, e
				}
			}
			h[i][k] = x.Quo(x, powT[i])
			if e := check(h[i][k]); e != nil {
				return zero, e
			}
		}
	}
	return NewExactBezierUV(h)
}
func (q PiecewiseMappedSplineUV) Source() SplineUV { return q.source }
func (q PiecewiseMappedSplineUV) ModelStation() *big.Rat {
	if q.station == nil {
		return nil
	}
	return new(big.Rat).Set(q.station)
}
func (q PiecewiseMappedSplineUV) ParameterWeights() [2]*big.Rat {
	return [2]*big.Rat{q.parts[0].ParameterWeight(), q.parts[1].ParameterWeight()}
}
func (q PiecewiseMappedSplineUV) AuthoredParameter(t *big.Rat) (*big.Rat, error) {
	if q.station == nil || t == nil || t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, geom.ErrDomain
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return nil, geom.ErrConvergence
	}
	if t.Cmp(q.station) <= 0 {
		return q.parts[0].AuthoredParameter(new(big.Rat).Quo(t, q.station))
	}
	return q.parts[1].AuthoredParameter(new(big.Rat).Quo(new(big.Rat).Sub(t, q.station), new(big.Rat).Sub(big.NewRat(1, 1), q.station)))
}
func (q PiecewiseMappedSplineUV) EvalUV(t float64) (geom.Vec2, error) { return q.curve.EvalUV(t) }
func (q PiecewiseMappedSplineUV) DerivativeUV(t float64) (geom.Vec2, error) {
	return q.curve.DerivativeUV(t)
}
func (q PiecewiseMappedSplineUV) BoundsUV() (geom.UVBox, error) { return q.curve.BoundsUV() }

func piecewiseMappedFaceCuts(uses [][]Use, cuts [][][]*big.Rat) ([][][]*big.Rat, error) {
	needed := false
	for _, loop := range uses {
		for _, u := range loop {
			if _, ok := u.Curve.(PiecewiseMappedSplineUV); ok {
				needed = true
			}
		}
	}
	if !needed {
		return cuts, nil
	}
	if cuts != nil && len(cuts) != len(uses) {
		return nil, geom.ErrInvalid
	}
	out := make([][][]*big.Rat, len(uses))
	total := 0
	for li, loop := range uses {
		if cuts != nil && len(cuts[li]) != len(loop) {
			return nil, geom.ErrInvalid
		}
		out[li] = make([][]*big.Rat, len(loop))
		for ui, u := range loop {
			var old []*big.Rat
			if cuts != nil {
				old = cuts[li][ui]
			}
			if len(old) > 2048-total {
				return nil, geom.ErrConvergence
			}
			if old != nil {
				if e := validateGraphCuts(old); e != nil {
					return nil, e
				}
				for _, t := range old {
					out[li][ui] = append(out[li][ui], new(big.Rat).Set(t))
				}
			}
			if q, ok := u.Curve.(PiecewiseMappedSplineUV); ok {
				if q.station == nil {
					return nil, geom.ErrInvalid
				}
				list := out[li][ui]
				if list == nil {
					list = []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
				}
				found := false
				for i, p := range list {
					if p.Cmp(q.station) == 0 {
						found = true
						break
					}
					if p.Cmp(q.station) > 0 {
						list = append(list, nil)
						copy(list[i+1:], list[i:])
						list[i] = q.ModelStation()
						found = true
						break
					}
				}
				if !found {
					return nil, geom.ErrInvalid
				}
				out[li][ui] = list
			}
			total += len(out[li][ui])
			if total > 2048 {
				return nil, geom.ErrConvergence
			}
		}
	}
	return out, nil
}

func hasNativePiecewiseMappedSpline(s Solid) bool {
	var has func(PCurve) bool
	has = func(c PCurve) bool {
		switch q := c.(type) {
		case PiecewiseMappedSplineUV:
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

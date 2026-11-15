package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// MappedSplineChain is an immutable proof-only, piecewise normalized trace.
// Unlike MultiSpanMappedSplineUV, its geometry need not be one global Bezier.
// It deliberately does not implement PCurve: native topology and face filling
// must not silently replace the original piecewise geometry by one polynomial.
type MappedSplineChain struct {
	parts       []MappedSplineUV
	stations    []*big.Rat
	independent bool
}

// NewMappedSplineChain re-proves two through eight contiguous restrictions of
// one whole original spline, independent positive maps, and exact C0 UV joins.
// Every map constructor retains o's independent work limit (at most eight).
// Model station coefficients are bounded to 4096 bits. No C1 promise is made.
func NewMappedSplineChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions) (MappedSplineChain, error) {
	return newMappedSplineChain(ctx, parts, stations, o, false)
}

// NewMappedSplineEdgeChain proves a chain of independently authored whole
// Bézier spline coedges. Their charts and endpoint positions remain exact.
// Each original basis and raw interval remains attached to its own coedge.
func NewMappedSplineEdgeChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions) (MappedSplineChain, error) {
	return newMappedSplineChain(ctx, parts, stations, o, true)
}

func newMappedSplineChain(ctx context.Context, parts []MappedSplineUV, stations []*big.Rat, o geom.SurfacePatchOptions, independent bool) (MappedSplineChain, error) {
	var zero MappedSplineChain
	if ctx == nil || len(parts) < 2 || len(parts) > 8 || len(stations) != len(parts)-1 {
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
	out := MappedSplineChain{independent: independent}
	for i, p := range parts {
		if p.source.Curve == nil {
			return zero, geom.ErrInvalid
		}
		for _, k := range p.source.Curve.Knots() {
			if k > p.source.Range.Min && k < p.source.Range.Max {
				return zero, geom.ErrConvergence
			}
		}
		q, e := NewMappedSplineUV(ctx, p.source, p.ParameterWeight(), o)
		if e != nil {
			return zero, e
		}
		if independent {
			lo, hi := q.source.Curve.Domain()
			if q.source.Range != (UVInterval{lo, hi}) {
				return zero, geom.ErrConvergence
			}
		}
		if i > 0 {
			a := out.parts[i-1]
			if a.curve.ends[1] != q.curve.ends[0] {
				return zero, geom.ErrConvergence
			}
			if !independent {
				if !sameAuthoredBezierSpline(a.source.Curve, q.source.Curve) || a.source.Reversed != q.source.Reversed || a.source.ExactEndpoints != q.source.ExactEndpoints {
					return zero, geom.ErrConvergence
				}
				if (!q.source.Reversed && a.source.Range.Max != q.source.Range.Min) || (q.source.Reversed && a.source.Range.Min != q.source.Range.Max) {
					return zero, geom.ErrConvergence
				}
			}
		}
		out.parts = append(out.parts, q)
	}
	if !independent {
		first, last := out.parts[0].source, out.parts[len(parts)-1].source
		lo, hi := first.Curve.Domain()
		if (!first.Reversed && (first.Range.Min != lo || last.Range.Max != hi)) || (first.Reversed && (first.Range.Max != hi || last.Range.Min != lo)) {
			return zero, geom.ErrConvergence
		}
	}
	for _, s := range stations {
		out.stations = append(out.stations, new(big.Rat).Set(s))
	}
	return out, nil
}
func (c MappedSplineChain) Sources() []SplineUV {
	out := make([]SplineUV, len(c.parts))
	for i, p := range c.parts {
		out[i] = p.Source()
	}
	return out
}
func (c MappedSplineChain) ModelStations() []*big.Rat {
	out := make([]*big.Rat, len(c.stations))
	for i, p := range c.stations {
		out[i] = new(big.Rat).Set(p)
	}
	return out
}
func (c MappedSplineChain) locate(t *big.Rat) (int, *big.Rat, error) {
	if len(c.parts) < 2 || t == nil {
		return 0, nil, geom.ErrInvalid
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return 0, nil, geom.ErrConvergence
	}
	if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return 0, nil, geom.ErrDomain
	}
	lo := new(big.Rat)
	for i := range c.parts {
		hi := big.NewRat(1, 1)
		if i < len(c.stations) {
			hi = c.stations[i]
		}
		if t.Cmp(hi) <= 0 {
			return i, new(big.Rat).Quo(new(big.Rat).Sub(t, lo), new(big.Rat).Sub(hi, lo)), nil
		}
		lo = hi
	}
	return 0, nil, geom.ErrDomain
}
func (c MappedSplineChain) AuthoredParameter(t *big.Rat) (*big.Rat, error) {
	i, u, e := c.locate(t)
	if e != nil {
		return nil, e
	}
	return c.parts[i].AuthoredParameter(u)
}
func (c MappedSplineChain) cell(a, b *big.Rat) (ExactBezierUV, error) {
	i, lo, hi, e := c.cellParameters(a, b)
	if e != nil {
		return ExactBezierUV{}, e
	}
	return restrictMappedProofCurve(c.parts[i].curve, lo, hi)
}

func (c MappedSplineChain) cellParameters(a, b *big.Rat) (int, *big.Rat, *big.Rat, error) {
	mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	i, _, e := c.locate(mid)
	if e != nil {
		return 0, nil, nil, e
	}
	lo := new(big.Rat)
	if i > 0 {
		lo = c.stations[i-1]
	}
	hi := big.NewRat(1, 1)
	if i < len(c.stations) {
		hi = c.stations[i]
	}
	if a.Cmp(lo) < 0 || b.Cmp(hi) > 0 {
		return 0, nil, nil, geom.ErrConvergence
	}
	w := new(big.Rat).Sub(hi, lo)
	return i, new(big.Rat).Quo(new(big.Rat).Sub(a, lo), w), new(big.Rat).Quo(new(big.Rat).Sub(b, lo), w), nil
}

// MappedSplineChainAgreement proves continuous complete model correspondence;
// it is not an embedding, closed shell, native persistence or face-fill proof.
// Source ownership is the ordered pair of immutable input chains/supports.
type MappedSplineChainAgreement struct {
	cuts     []*big.Rat
	points   [][3]*big.Rat
	sources  [2]MappedSplineChain
	supports [2]geom.Surface
}

func (r MappedSplineChainAgreement) ModelCuts() []*big.Rat {
	out := make([]*big.Rat, len(r.cuts))
	for i, p := range r.cuts {
		out[i] = new(big.Rat).Set(p)
	}
	return out
}
func (r MappedSplineChainAgreement) ModelPoints() [][3]*big.Rat {
	out := make([][3]*big.Rat, len(r.points))
	for i, p := range r.points {
		for k := range p {
			out[i][k] = new(big.Rat).Set(p[k])
		}
	}
	return out
}
func (r MappedSplineChainAgreement) AuthoredParameters(owner int) ([]*big.Rat, error) {
	if owner < 0 || owner > 1 || len(r.cuts) == 0 {
		return nil, geom.ErrInvalid
	}
	out := make([]*big.Rat, len(r.cuts))
	for i, t := range r.cuts {
		p, e := r.sources[owner].AuthoredParameter(t)
		if e != nil {
			return nil, e
		}
		out[i] = p
	}
	return out, nil
}

// CertifyMappedSplineChainAgreement compares every cell of the common exact
// station union (at most 15 cells), using full homogeneous composition equality.
// Each cell retains o's proof budget; total work is bounded by 15 such proofs
// plus at most 16 exact endpoint evaluations. Internal restriction endpoints
// must be representable under the existing exact Bezier descriptor contract.
// Both supports must be directly supported exact spline charts (no placement).
func CertifyMappedSplineChainAgreement(ctx context.Context, a geom.Surface, ca MappedSplineChain, b geom.Surface, cb MappedSplineChain, o edgecheck.Options) (MappedSplineChainAgreement, error) {
	var zero MappedSplineChainAgreement
	if ctx == nil || len(ca.parts) < 2 || len(cb.parts) < 2 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for _, c := range []MappedSplineChain{ca, cb} {
		for _, s := range c.stations {
			for i, t := range cuts {
				if s.Cmp(t) == 0 {
					break
				}
				if s.Cmp(t) < 0 {
					cuts = append(cuts, nil)
					copy(cuts[i+1:], cuts[i:])
					cuts[i] = new(big.Rat).Set(s)
					break
				}
			}
		}
	}
	if o.MaxIntervals > 0 && len(cuts)-1 > o.MaxIntervals {
		return zero, geom.ErrConvergence
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	pa, e := exactSplineSupportPatch(a, limits)
	if e != nil {
		return zero, e
	}
	pb, e := exactSplineSupportPatch(b, limits)
	if e != nil {
		return zero, e
	}
	out := MappedSplineChainAgreement{cuts: cuts, sources: [2]MappedSplineChain{ca, cb}, supports: [2]geom.Surface{a, b}}
	for i := 0; i+1 < len(cuts); i++ {
		qa, e := ca.cell(cuts[i], cuts[i+1])
		if e != nil {
			return zero, e
		}
		qb, e := cb.cell(cuts[i], cuts[i+1])
		if e != nil {
			return zero, e
		}
		da, e := qa.descriptor(ctx, limits)
		if e != nil {
			return zero, e
		}
		db, e := qb.descriptor(ctx, limits)
		if e != nil {
			return zero, e
		}
		equal, e := compareTrimCompositionsOnPlacedSplineSupports(ctx, a, da, b, db, o)
		if e != nil {
			return zero, e
		}
		if !equal {
			return zero, geom.ErrConvergence
		}
		for endpoint := 0; endpoint < 2; endpoint++ {
			if i > 0 && endpoint == 0 {
				continue
			}
			ua, ub := qa.ends[endpoint], qb.ends[endpoint]
			xa, e := pa.EvaluateRational(ctx, ratInset(ua.X), ratInset(ua.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
			if e != nil {
				return zero, e
			}
			xb, e := pb.EvaluateRational(ctx, ratInset(ub.X), ratInset(ub.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
			if e != nil {
				return zero, e
			}
			var point [3]*big.Rat
			for k := range point {
				if xa.Point[k].Cmp(xb.Point[k]) != 0 {
					return zero, geom.ErrConvergence
				}
				point[k] = new(big.Rat).Set(xa.Point[k])
			}
			out.points = append(out.points, point)
		}
	}
	return out, nil
}

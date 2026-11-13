package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// proposeTerminalFromBoundaryContact derives hints from independently
// isolated edge/face contacts. It never uses a numerical representative
// as a split vertex: both exact contact and the original tube are checked.
func proposeTerminalFromBoundaryContact(ctx context.Context, segment intersect.TubeSegment, end int, a, b intersect.Patch,
	o SplineShellOpenOptions) (intersect.SplineTubeTerminal, error) {
	var empty intersect.SplineTubeTerminal
	if ctx == nil || end < 0 || end > 1 {
		return empty, geom.ErrInvalid
	}
	q, e := segment.Evaluate(float64(end))
	if e != nil {
		return empty, e
	}
	boxes := [2]geom.UVBox{q.ParametersA, q.ParametersB}
	var patches [2]*geom.SurfacePatch
	var domains [2]geom.ParameterRectangle
	for i, x := range [2]intersect.Patch{a, b} {
		s, ok := x.Surface.(*geom.BSplineSurface)
		if !ok {
			return empty, geom.ErrConvergence
		}
		p, e := geom.NewSurfacePatch(s, o.Trace.Seeds.Patches)
		if e != nil {
			return empty, e
		}
		domains[i] = geom.ParameterRectangle{UMin: x.Min.X, UMax: x.Max.X, VMin: x.Min.Y, VMax: x.Max.Y}
		patches[i], e = p.Trim(domains[i])
		if e != nil {
			return empty, e
		}
	}
	limit := o.Endpoints.MaxOperations
	if limit == 0 {
		limit = 1000000
	}
	if limit < 1 {
		return empty, geom.ErrInvalid
	}
	for owner, p := range patches {
		d := domains[owner]
		for i, fixed := range [4]float64{d.UMin, d.UMax, d.VMin, d.VMax} {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			axis := i / 2
			lo, hi := boxes[owner].Min.X, boxes[owner].Max.X
			if axis == 1 {
				lo, hi = boxes[owner].Min.Y, boxes[owner].Max.Y
			}
			if fixed < lo || fixed > hi {
				continue
			}
			curve, e := p.IsoCurve(axis, fixed, o.Trace.Seeds.Patches)
			if e != nil {
				return empty, e
			}
			r, e := geom.IntersectExactSplineCurveSurface(ctx, curve, patches[owner^1], o.BoundaryContacts)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return empty, e
			}
			for _, h := range r.Hits {
				tlo, thi := boxes[owner].Min.Y, boxes[owner].Max.Y
				if axis == 1 {
					tlo, thi = boxes[owner].Min.X, boxes[owner].Max.X
				}
				other := boxes[owner^1]
				options := [3][]*big.Rat{
					terminalRationalCandidates(max(tlo, h.CurveParameter.Min), min(thi, h.CurveParameter.Max)),
					terminalRationalCandidates(max(other.Min.X, h.UV.Min.X), min(other.Max.X, h.UV.Max.X)),
					terminalRationalCandidates(max(other.Min.Y, h.UV.Min.Y), min(other.Max.Y, h.UV.Max.Y)),
				}
				for _, t := range options[0] {
					for _, u := range options[1] {
						for _, v := range options[2] {
							if e := ctx.Err(); e != nil {
								return empty, e
							}
							if limit < 1 {
								return empty, geom.ErrConvergence
							}
							p, e := geom.CertifyCurveSurfaceRationalHit(ctx, curve, patches[owner^1], h, t, u, v,
								geom.SurfacePatchEvaluationOptions{MaxOperations: limit, MaxCoefficientBits: o.Endpoints.MaxCoefficientBits})
							limit -= p.ExactOperations
							if e != nil {
								return empty, e
							}
							if !p.Contact || !p.Regular {
								continue
							}
							hint := [4]*big.Rat{}
							hint[2*owner+axis], hint[2*owner+(axis^1)] = ratInset(fixed), t
							hint[2*(owner^1)], hint[2*(owner^1)+1] = u, v
							if limit < 1 {
								return empty, geom.ErrConvergence
							}
							certificate, e := segment.CertifyEndpointOnEdge(ctx, end, owner, d, geom.SurfaceEdge(i), hint,
								geom.TubeEndpointEdgeOptions{MaxOperations: limit, MaxCoefficientBits: o.Endpoints.MaxCoefficientBits})
							limit -= certificate.ExactOperations
							if e == nil && certificate.Proven {
								return intersect.SplineTubeTerminal{Operand: owner, Side: geom.SurfaceEdge(i), Proof: certificate}, nil
							}
							if e != nil && !isUnresolvedOpenSpline(e) && !errors.Is(e, geom.ErrDomain) {
								return empty, e
							}
						}
					}
				}
			}
		}
	}
	return empty, geom.ErrConvergence
}

func terminalRationalCandidates(lo, hi float64) []*big.Rat {
	if !finite(lo) || !finite(hi) || lo > hi {
		return nil
	}
	a, b := ratInset(lo), ratInset(hi)
	mid := lo/2 + hi/2
	out := make([]*big.Rat, 0, 8)
	add := func(x *big.Rat) {
		if len(out) >= 8 || x.Cmp(a) < 0 || x.Cmp(b) > 0 {
			return
		}
		for _, y := range out {
			if x.Cmp(y) == 0 {
				return
			}
		}
		out = append(out, x)
	}
	if math.Abs(mid) < 1<<42 {
		for d := int64(1); d <= 1024; d++ {
			add(big.NewRat(int64(math.Round(mid*float64(d))), d))
		}
	}
	add(a)
	add(b)
	add(ratInset(mid))
	return out
}

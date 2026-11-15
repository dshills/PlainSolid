package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// EdgeUseProof identifies the face loop and edge use that was checked.
// Loop zero is the outer boundary.
type EdgeUseProof struct {
	Face, Loop, Use, Edge int
	Report                edgecheck.Report
}

// EdgeAgreementReport describes continuous same-parameter agreement between
// model curves and their supported trim uses. Complete requires a
// successful certificate for every use. Unsupported cases stay unresolved.
type EdgeAgreementReport struct {
	Uses     []EdgeUseProof
	Complete bool
}

// ValidateEdgeAgreement proves agreement over the entire normalized range
// for supported affine planar charts, Bézier edges, analytic seams and poles,
// and certified implicit sections. Bounds describe ideal evaluators, excluding float roundoff. Unresolved
// uses are reported separately from actual tolerance violations.
func (s Solid) ValidateEdgeAgreement(ctx context.Context, o edgecheck.Options) (EdgeAgreementReport, error) {
	r := EdgeAgreementReport{Complete: true}
	if ctx == nil {
		return EdgeAgreementReport{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return EdgeAgreementReport{}, e
	}
	affineCharts := affineEdgeCharts{}
	for i, f := range s.faces {
		chart, ok := edgeChart(f.Surface)
		for j, l := range append([][]Use{f.Outer}, f.Holes...) {
			for k, u := range l {
				if e := ctx.Err(); e != nil {
					r.Complete = false
					return r, e
				}
				p := EdgeUseProof{Face: i, Loop: j, Use: k, Edge: u.Edge, Report: edgecheck.Report{Tolerance: s.tolerance}}
				c, supported := edgeBezier(u)
				model := s.edges[u.Edge]
				m, modelSupported := modelBezier(model)
				identical := sameCertifiedSection(model, f.Surface, u) || sameSplineTrim(model, f.Surface, u) || sameAnalyticEdgeUse(model, f.Surface, u)
				if !identical {
					var e error
					identical, e = compareWrappedEdgeUse(ctx, model, f.Surface, u, o)
					if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrInvalid) && !errors.Is(e, geom.ErrConvergence) {
						r.Complete = false
						return r, e
					}
				}
				var poleError *big.Rat
				if model.Curve == nil {
					poleError = analyticPoleSquaredError(f.Surface, u, s.vertices[model.Start])
				}
				if !identical {
					var e error
					identical, e = compareSplineEdgeUse(ctx, model, f.Surface, u, o)
					if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrInvalid) && !errors.Is(e, geom.ErrConvergence) {
						r.Complete = false
						return r, e
					}
				}
				if !identical {
					var e error
					identical, e = affineCharts.matchUse(ctx, model, f.Surface, u, o)
					if e != nil {
						r.Complete = false
						return r, e
					}
				}
				switch {
				case poleError != nil:
					tol := ratInset(s.tolerance)
					p.Report.SquaredUpperBound = poleError
					p.Report.Reason = "constant analytic pole and exact vertex distance"
					if poleError.Cmp(tol.Mul(tol, tol)) <= 0 {
						p.Report.Status = edgecheck.Certified
					} else {
						p.Report.Status = edgecheck.Exceeded
					}
				case identical:
					p.Report.Status = edgecheck.Certified
					p.Report.SquaredUpperBound = new(big.Rat)
					p.Report.Reason = "matching continuous evaluators and parameter directions"
				case !ok || !supported:
					p.Report.Reason = "unsupported surface or edge parameterization"
				default:
					c.Reversed = u.Reversed
					var x edgecheck.Report
					var e error
					switch y := model.Curve.(type) {
					case geom.Line:
						x, e = edgecheck.CheckPlanarLine(ctx, c, chart, y, model.Range.Min, model.Range.Max, s.tolerance, o)
					case surfaceTrimCurve:
						frame, ok := edgeChart(y.surface)
						q, good := edgeBezier(Use{UVStart: y.a, UVEnd: y.b, Curve: y.curve})
						if ok && good && model.Range.Min == 0 && model.Range.Max == 1 {
							x, e = edgecheck.CheckPlanarPair(ctx, c, chart, q, frame, s.tolerance, o)
						} else {
							x = edgecheck.Report{Tolerance: s.tolerance, Reason: "unsupported model trim chart"}
						}
					default:
						if modelSupported {
							x, e = edgecheck.CheckPlanar(ctx, c, chart, m, s.tolerance, o)
						} else {
							x = edgecheck.Report{Tolerance: s.tolerance, Reason: "unsupported model edge"}
						}
					}
					if e != nil {
						r.Complete = false
						return r, fmt.Errorf("trimmed: continuous check of face %d edge %d: %w", i, u.Edge, e)
					}
					p.Report = x
				}
				r.Complete = r.Complete && p.Report.Status == edgecheck.Certified
				r.Uses = append(r.Uses, p)
			}
		}
	}
	return r, nil
}

func sameSplineTrim(e parametric.Edge, surface geom.Surface, use Use) bool {
	m, ok := e.Curve.(surfaceTrimCurve)
	if !ok || m.curve != nil || use.Curve != nil || e.Range.Min < 0 || e.Range.Max > 1 {
		return false
	}
	x, px := parametric.SurfaceMapping(m.surface)
	y, py := parametric.SurfaceMapping(surface)
	a, ok := x.(*geom.BSplineSurface)
	b, has := y.(*geom.BSplineSurface)
	if !ok || !has || px != py {
		return false
	}
	p, q := e.Range.Min, e.Range.Max
	if use.Reversed {
		p, q = q, p
	}
	// Split edges can retain an affine subrange. Compare the exact
	// endpoint parameters before certifying the entire affine trim.
	coord := func(a, b, t float64) *big.Rat {
		x, y, z := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b), new(big.Rat).SetFloat64(t)
		return new(big.Rat).Add(x, new(big.Rat).Mul(new(big.Rat).Sub(y, x), z))
	}
	for _, pair := range []struct {
		t float64
		p geom.Vec2
	}{{p, use.UVStart}, {q, use.UVEnd}} {
		if coord(m.a.X, m.b.X, pair.t).Cmp(new(big.Rat).SetFloat64(pair.p.X)) != 0 ||
			coord(m.a.Y, m.b.Y, pair.t).Cmp(new(big.Rat).SetFloat64(pair.p.Y)) != 0 {
			return false
		}
	}
	return geom.SameSplineSurface(a, b)
}

func sameCertifiedSection(e parametric.Edge, surface geom.Surface, use Use) bool {
	m, ok := e.Curve.(surfaceTrimCurve)
	if !ok || e.Range.Min != 0 || e.Range.Max != 1 {
		return false
	}
	unwrap := func(c PCurve) (PCurve, bool) {
		reversed := false
		for {
			r, ok := c.(reverseTrim)
			if !ok {
				break
			}
			reversed, c = !reversed, r.differentiableTrim
		}
		return c, reversed
	}
	a, revA := unwrap(m.curve)
	b, revB := unwrap(use.Curve)
	if revA != (revB != use.Reversed) {
		return false
	}
	x, px := parametric.SurfaceMapping(m.surface)
	y, py := parametric.SurfaceMapping(surface)
	if px != py {
		return false
	}
	switch p := a.(type) {
	case intersect.CertifiedTrim:
		sa, hasA := x.(*geom.BSplineSurface)
		sb, hasB := y.(*geom.BSplineSurface)
		q, ok := b.(intersect.CertifiedTrim)
		return ok && hasA && hasB && p.CertifiesSharedChartSection(q, sa, sb)
	case intersect.TubeTrim:
		q, ok := b.(intersect.TubeTrim)
		return ok && p.CertifiesPairedTubeOnSurfaces(q, x, y)
	}
	return false
}

func edgeChart(s geom.Surface) (edgecheck.Chart, bool) {
	if wall, ok := s.(splineGraphAffineWall); ok {
		return wall.chart(), true
	}
	base, m := parametric.SurfaceMapping(s)
	p, ok := base.(geom.PlaneSurface)
	if !ok {
		return edgecheck.Chart{}, false
	}
	f := p.Frame()
	if m != geom.Identity() {
		return edgecheck.Chart{}, false
	}
	return edgecheck.Chart{Origin: f.Origin(), U: f.XAxis(), V: f.YAxis()}, true
}

func edgeBezier(u Use) (edgecheck.PlanarCurve, bool) {
	p, ok := TrimValidationSegment(u)
	if !ok {
		return edgecheck.PlanarCurve{}, false
	}
	switch p.Kind {
	case trimvalidate.Line:
		return edgecheck.PlanarCurve{Poles: []geom.Vec2{p.Start, p.End}}, true
	case trimvalidate.Quadratic:
		return edgecheck.PlanarCurve{Poles: []geom.Vec2{p.Start, p.Control, p.End}}, true
	case trimvalidate.RationalQuadratic:
		return edgecheck.PlanarCurve{Poles: []geom.Vec2{p.Start, p.Control, p.End}, Weights: p.Weights[:]}, true
	}
	return edgecheck.PlanarCurve{}, false
}

func modelBezier(e parametric.Edge) (edgecheck.Curve, bool) {
	switch x := e.Curve.(type) {
	case *geom.BSpline:
		if x == nil {
			return edgecheck.Curve{}, false
		}
		a, b := x.Domain()
		p, k, w := x.Poles(), x.Knots(), x.Weights()
		if len(p) != x.Degree()+1 || len(k) != 2*len(p) || e.Range.Min != a || e.Range.Max != b {
			return edgecheck.Curve{}, false
		}
		for i, t := range k {
			if i < len(p) && t != a || i >= len(p) && t != b {
				return edgecheck.Curve{}, false
			}
		}
		return edgecheck.Curve{Poles: p, Weights: w}, true
	}
	return edgecheck.Curve{}, false
}

package intersect

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/geom"
)

// SplinePlaneOptions controls branch certification and exact section-root
// evaluation. ResidualTolerance defaults to 1e-8 in model units.
type SplinePlaneOptions struct {
	Patch geom.SurfacePatchOptions
	// Domain selects a strict subdomain. Its zero value uses the full
	// active knot domain of the supplied surface.
	Domain            geom.ParameterRectangle
	Trace             geom.SurfaceTraceOptions
	Evaluation        geom.ArcEvaluationOptions
	ResidualTolerance float64
}

// CertifiedContour is an ordered list of paired trim curves. Each pair has
// the same normalized [0,1] parameter on the spline and plane. Closed
// contours repeat no segment. Only certified arc chains are included.
type CertifiedContour struct {
	Segments []CertifiedSegment
	Closed   bool
}

// CertifiedSegment retains one exact implicit intersection segment and
// returns p-curves for the original spline and plane charts.
type CertifiedSegment struct {
	surface        *geom.BSplineSurface
	second         *geom.BSplineSurface // exact common-chart height difference when present
	difference     *geom.SurfacePatch
	plane          geom.Frame
	arc            geom.SurfacePlaneArc
	ends           [2]geom.SurfaceArcEvaluation
	start, end     float64
	evaluation     geom.ArcEvaluationOptions
	residual       float64
	options        SplinePlaneOptions
	contour, index int
}

// SplinePlaneTrimData describes the recipe for a certified p-curve. Its
// segment indices follow TraceSplinePlane's deterministic contour traversal.
type SplinePlaneTrimData struct {
	Surface          *geom.BSplineSurface
	Plane            geom.Frame
	Options          SplinePlaneOptions
	Contour, Segment int
	OnPlane          bool
	Second           *geom.BSplineSurface
}

// CertifiedTrim implements the trimmed.PCurve evaluation and derivative
// contracts. Floating-point representatives are checked against the
// intersection and the paired support at each call.
type CertifiedTrim struct {
	segment CertifiedSegment
	plane   bool
}

func (s CertifiedSegment) SplineTrim() CertifiedTrim { return CertifiedTrim{segment: s} }
func (s CertifiedSegment) PlaneTrim() CertifiedTrim  { return CertifiedTrim{segment: s, plane: true} }

// PlaneSignOnRight returns the certified plane numerator sign on the right
// side of the directed trim in the original spline UV chart. A positive
// sign means the original surface lies toward the plane normal there.
// The sign is local to the regular contour; zero means no certificate.
func (s CertifiedSegment) PlaneSignOnRight() int {
	if s.surface == nil || s.start == s.end {
		return 0
	}
	n := s.arc.Certificate.PlaneNumeratorDerivative.Sign
	if s.end < s.start {
		n = -n
	}
	if s.arc.Certificate.FixedParameterAxis == 1 {
		n = -n
	}
	return n
}

// Data returns the source recipe without replacing the intersection by samples.
func (p CertifiedTrim) Data() SplinePlaneTrimData {
	s := p.segment
	return SplinePlaneTrimData{s.surface, s.plane, s.options, s.contour, s.index, p.plane, s.second}
}

// CertifiesSharedChartSection reports whether p and q describe the same
// exact common-chart curve on the supplied original graph supports. It
// compares spline data so the proof also applies after native round trips.
func (p CertifiedTrim) CertifiesSharedChartSection(q CertifiedTrim, a, b *geom.BSplineSurface) bool {
	x, y := p.Data(), q.Data()
	if a == nil || b == nil || x.Surface == nil || x.Second == nil || x.OnPlane || y.OnPlane ||
		x.Plane != y.Plane || x.Options != y.Options || x.Contour != y.Contour || x.Segment != y.Segment {
		return false
	}
	if !geom.SameSplineSurface(x.Surface, y.Surface) || !geom.SameSplineSurface(x.Second, y.Second) {
		return false
	}
	on := func(a *geom.BSplineSurface) bool {
		return geom.SameSplineSurface(a, x.Surface) || geom.SameSplineSurface(a, x.Second)
	}
	return on(a) && on(b)
}

// BoundsUV returns an outward enclosure from the certified source cell. For
// the planar chart it projects the cell's spatial hull.
func (p CertifiedTrim) BoundsUV() (geom.UVBox, error) {
	s := p.segment
	if s.surface == nil {
		return geom.UVBox{}, geom.ErrInvalid
	}
	if !p.plane {
		return s.arc.ParameterBounds()
	}
	b := s.arc.Bounds
	out := geom.UVBox{Min: geom.Vec2{X: math.Inf(1), Y: math.Inf(1)}, Max: geom.Vec2{X: math.Inf(-1), Y: math.Inf(-1)}}
	for _, x := range []float64{b.Min.X, b.Max.X} {
		for _, y := range []float64{b.Min.Y, b.Max.Y} {
			for _, z := range []float64{b.Min.Z, b.Max.Z} {
				p := geom.Vec3{X: x, Y: y, Z: z}
				v := s.plane.ToLocal(p)
				e := 64 * 0x1p-52 * p.Sub(s.plane.Origin()).Length()
				out.Min.X, out.Min.Y = min(out.Min.X, math.Nextafter(v.X-e, math.Inf(-1))), min(out.Min.Y, math.Nextafter(v.Y-e, math.Inf(-1)))
				out.Max.X, out.Max.Y = max(out.Max.X, math.Nextafter(v.X+e, math.Inf(1))), max(out.Max.Y, math.Nextafter(v.Y+e, math.Inf(1)))
			}
		}
	}
	if !finite(out.Min.X) || !finite(out.Min.Y) || !finite(out.Max.X) || !finite(out.Max.Y) {
		return geom.UVBox{}, geom.ErrInvalid
	}
	return out, nil
}

// TraceSplinePlane exposes a certified spline–plane trace as paired p-curves.
// The supplied frame selects the plane chart. The report retains unresolved
// tangencies, coincidences and budget limits from geom.TraceSurfacePlane.
// With tangent isolation enabled, proved point contacts are available in
// the report only. P-curves require a complete noncoincident trace.
func TraceSplinePlane(s *geom.BSplineSurface, frame geom.Frame, o SplinePlaneOptions) ([]CertifiedContour, geom.SurfacePlaneTrace, error) {
	return TraceSplinePlaneContext(context.Background(), s, frame, o)
}

// TraceSplinePlaneContext checks cancellation during exact arc isolation and joins.
func TraceSplinePlaneContext(ctx context.Context, s *geom.BSplineSurface, frame geom.Frame, o SplinePlaneOptions) ([]CertifiedContour, geom.SurfacePlaneTrace, error) {
	if ctx == nil {
		return nil, geom.SurfacePlaneTrace{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	p, e := geom.NewSurfacePatch(s, o.Patch)
	if e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	return traceCertifiedPlane(ctx, s, nil, p, frame, o)
}

// TraceSplineHeightDifference certifies contours where two rational spline
// graphs meet at a common parameter. It requires identical projected charts;
// degrees, knot breaks and positive rational weights may differ. SplineTrim on
// the returned segments evaluates the p-curve common to both original faces.
// PlaneTrim refers to the auxiliary height-difference chart.
func TraceSplineHeightDifference(a, b *geom.BSplineSurface, o SplinePlaneOptions) ([]CertifiedContour, geom.SurfacePlaneTrace, error) {
	return TraceSplineHeightDifferenceContext(context.Background(), a, b, o)
}

// TraceSplineHeightDifferenceContext checks cancellation during exact tracing.
func TraceSplineHeightDifferenceContext(ctx context.Context, a, b *geom.BSplineSurface, o SplinePlaneOptions) ([]CertifiedContour, geom.SurfacePlaneTrace, error) {
	if ctx == nil {
		return nil, geom.SurfacePlaneTrace{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	pa, e := geom.NewSurfacePatch(a, o.Patch)
	if e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	pb, e := geom.NewSurfacePatch(b, o.Patch)
	if e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	p, e := geom.NewSurfaceHeightDifferencePatch(pa, pb, geom.SurfaceHeightPatchOptions{
		MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
	})
	if errors.Is(e, geom.ErrInvalid) {
		p, e = geom.NewGeneralSurfaceHeightDifferencePatch(ctx, pa, pb, geom.GeneralSurfaceHeightOptions{
			MaxPoles: o.Patch.MaxPoles, MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
		})
	}
	if e != nil {
		return nil, geom.SurfacePlaneTrace{}, e
	}
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	return traceCertifiedPlane(ctx, a, b, p, frame, o)
}

func traceCertifiedPlane(ctx context.Context, s, second *geom.BSplineSurface, p *geom.SurfacePatch, frame geom.Frame, o SplinePlaneOptions) ([]CertifiedContour, geom.SurfacePlaneTrace, error) {
	var empty geom.SurfacePlaneTrace
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = 1e-8
	}
	if !finite(o.ResidualTolerance) || o.ResidualTolerance <= 0 {
		return nil, empty, geom.ErrInvalid
	}
	plane, e := geom.NewPlane(frame.Origin(), frame.ZAxis())
	if e != nil {
		return nil, empty, e
	}
	if o.Domain != (geom.ParameterRectangle{}) {
		p, e = p.Trim(o.Domain)
		if e != nil {
			return nil, empty, e
		}
	}
	r, e := geom.TraceSurfacePlaneContext(ctx, p, plane, o.Trace)
	if e != nil {
		return nil, r, e
	}
	if !r.Complete || len(r.Isolation.Coplanar) > 0 {
		return nil, r, ErrUnresolved
	}
	contours := make([]CertifiedContour, len(r.Contours))
	for i, c := range r.Contours {
		contours[i].Closed = c.Closed
		for j, ref := range c.Arcs {
			if e := ctx.Err(); e != nil {
				return nil, r, e
			}
			a := r.Isolation.Arcs[ref.Index]
			x, y := 0, 1
			if ref.Reversed {
				x, y = y, x
			}
			first, e := a.EndpointEvaluation(x)
			if e != nil {
				return nil, r, e
			}
			last, e := a.EndpointEvaluation(y)
			if e != nil {
				return nil, r, e
			}
			b := CertifiedSegment{surface: s, second: second, plane: frame, arc: a, ends: [2]geom.SurfaceArcEvaluation{first, last},
				evaluation: o.Evaluation, residual: o.ResidualTolerance, options: o, contour: i, index: j}
			if second != nil {
				b.difference = p
			}
			b.start, b.end = b.free(midUV(first.Parameters)), b.free(midUV(last.Parameters))
			if b.start == b.end || !finite(b.end-b.start) {
				return nil, r, fmt.Errorf("%w: nonrepresentable contour parameter interval", ErrUnresolved)
			}
			for _, t := range []float64{0, 1} {
				if _, e = b.eval(t); e != nil {
					return nil, r, e
				}
			}
			contours[i].Segments = append(contours[i].Segments, b)
		}
	}
	return contours, r, nil
}

func midUV(b geom.UVBox) geom.Vec2 {
	return geom.Vec2{X: max(b.Min.X, min(b.Max.X, b.Min.X/2+b.Max.X/2)), Y: max(b.Min.Y, min(b.Max.Y, b.Min.Y/2+b.Max.Y/2))}
}

func (s CertifiedSegment) free(uv geom.Vec2) float64 {
	if s.arc.Certificate.FixedParameterAxis == 0 {
		return uv.Y
	}
	return uv.X
}

type certifiedEval struct{ uv, plane geom.Vec2 }

func (s CertifiedSegment) world(p geom.Vec2) (geom.Vec3, error) {
	a, e := s.surface.Eval(p.X, p.Y)
	if e != nil || s.second == nil {
		return a, e
	}
	b, e := s.second.Eval(p.X, p.Y)
	scale := math.Max(math.Abs(a.Z), math.Abs(b.Z))
	if e == nil && scale*128*0x1p-52 > s.residual*.001 {
		return s.difference.Eval(p.X, p.Y)
	}
	a.Z -= b.Z
	return a, e
}

func (s CertifiedSegment) partials(p geom.Vec2) (geom.Vec3, geom.Vec3, error) {
	a, b, e := s.surface.Partials(p.X, p.Y)
	if e != nil || s.second == nil {
		return a, b, e
	}
	c, d, e := s.second.Partials(p.X, p.Y)
	a.Z, b.Z = a.Z-c.Z, b.Z-d.Z
	return a, b, e
}

func (s CertifiedSegment) implicitPartials(p geom.Vec2, a, b geom.Vec3) (float64, float64, error) {
	da, db := a.Dot(s.plane.ZAxis()), b.Dot(s.plane.ZAxis())
	if s.difference == nil {
		return da, db, nil
	}
	// Only the solved coordinate is required to stay nonzero. A cancellation
	// in the free partial is normal at an extremum and its tiny absolute error
	// does not destabilize the derivative ratio.
	d := da
	if s.arc.Certificate.FixedParameterAxis == 1 {
		d = db
	}
	sign := s.arc.Certificate.PlaneNumeratorDerivative.Sign
	if finite(d) && d != 0 && (d > 0) == (sign > 0) {
		return da, db, nil
	}
	u, v, e := s.difference.Partials(p.X, p.Y)
	return u.Z, v.Z, e
}

func (s CertifiedSegment) eval(t float64) (certifiedEval, error) {
	if s.surface == nil || !finite(t) || t < 0 || t > 1 {
		return certifiedEval{}, geom.ErrDomain
	}
	var q geom.SurfaceArcEvaluation
	var e error
	endpoint := t == 0 || t == 1
	if endpoint {
		q = s.ends[int(t)]
	} else {
		f := (1-t)*s.start + t*s.end
		if fast, ok := s.fastEval(t, f); ok {
			return fast, nil
		}
		q, e = s.arc.Evaluate(f, s.evaluation)
		if e != nil {
			// An endpoint's midpoint need not equal its exact algebraic
			// coordinate. Rounding a nearby t into this tiny interval is
			// explicit when it cannot be evaluated with the endpoint bound.
			i := 0
			if t > .5 {
				i = 1
			}
			b := s.ends[i].Parameters
			if e != geom.ErrDomain || f < s.free(b.Min) || f > s.free(b.Max) {
				return certifiedEval{}, e
			}
			q = s.ends[i]
		}
	}
	p := midUV(q.Parameters)
	world, e := s.world(p)
	if e != nil {
		return certifiedEval{}, e
	}
	v := s.plane.ToLocal(world)
	w := s.plane.ToWorld(geom.Vec3{X: v.X, Y: v.Y})
	if !v.IsFinite() || !w.IsFinite() || world.Distance(w) > s.residual ||
		world.Distance(q.Point)+q.Bounds.Size().Length()/2 > s.residual {
		return certifiedEval{}, fmt.Errorf("%w: intersection enclosure or residual exceeds tolerance", ErrUnresolved)
	}
	return certifiedEval{p, geom.Vec2{X: v.X, Y: v.Y}}, nil
}

// The public arc evaluation in geom always returns exact enclosures. Trim
// evaluators can take a faster numerical path because their contract is a
// model-space residual, using the proven single-branch cell as a guard.
// Any failure falls back to the exact section solver.
func (s CertifiedSegment) fastEval(t, f float64) (certifiedEval, bool) {
	d := s.arc.Domain
	lo, hi := d.UMin, d.UMax
	x := (1-t)*midUV(s.ends[0].Parameters).X + t*midUV(s.ends[1].Parameters).X
	fixed := s.arc.Certificate.FixedParameterAxis
	if fixed == 1 {
		lo, hi = d.VMin, d.VMax
		x = (1-t)*midUV(s.ends[0].Parameters).Y + t*midUV(s.ends[1].Parameters).Y
	}
	if x < lo || x > hi {
		return certifiedEval{}, false
	}
	bracketed := false
	for range 14 {
		p := geom.Vec2{X: x, Y: f}
		if fixed == 1 {
			p = geom.Vec2{X: f, Y: x}
		}
		w, e := s.world(p)
		if e != nil {
			return certifiedEval{}, false
		}
		q := s.plane.ToLocal(w)
		if !q.IsFinite() {
			return certifiedEval{}, false
		}
		// Do not accept a numerically near point on a section for which the
		// exact arc may end just before this free coordinate.
		a, b := s.ends[0].Parameters, s.ends[1].Parameters
		inside := (f > s.free(a.Max) && f < s.free(b.Min)) || (f > s.free(b.Max) && f < s.free(a.Min))
		if !inside {
			return certifiedEval{}, false
		}
		u, v, e := s.partials(p)
		if e != nil {
			return certifiedEval{}, false
		}
		du := u
		if fixed == 1 {
			du = v
		}
		da, db, e := s.implicitPartials(p, u, v)
		if e != nil {
			return certifiedEval{}, false
		}
		df := da
		if fixed == 1 {
			df = db
		}
		step := q.Z / df
		if finite(step) && math.Abs(step)*du.Length() <= s.residual*.01 &&
			w.Distance(s.plane.ToWorld(geom.Vec3{X: q.X, Y: q.Y})) <= s.residual*.01 {
			return certifiedEval{p, geom.Vec2{X: q.X, Y: q.Y}}, true
		}
		if (q.Z > 0) == (s.arc.Certificate.PlaneNumeratorDerivative.Sign > 0) {
			hi = x
		} else {
			lo = x
		}
		next := x - step
		if !finite(next) || next <= lo || next >= hi || next == x {
			next = lo + (hi-lo)/2
			bracketed = true
		}
		if next == x || next == lo || next == hi && bracketed {
			return certifiedEval{}, false
		}
		x = next
	}
	return certifiedEval{}, false
}

func (p CertifiedTrim) EvalUV(t float64) (geom.Vec2, error) {
	q, e := p.segment.eval(t)
	if p.plane {
		return q.plane, e
	}
	return q.uv, e
}

// DerivativeUV evaluates the implicit tangent by differentiating the plane
// equation on the original spline. The result is numerical; the arc
// certificate guarantees a nonzero derivative in the solved coordinate.
func (p CertifiedTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	s := p.segment
	q, e := s.eval(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	a, b, e := s.partials(q.uv)
	if e != nil {
		return geom.Vec2{}, e
	}
	da, db, e := s.implicitPartials(q.uv, a, b)
	if e != nil {
		return geom.Vec2{}, e
	}
	w := s.end - s.start
	d := geom.Vec2{X: w, Y: -w * da / db}
	if s.arc.Certificate.FixedParameterAxis == 0 {
		d = geom.Vec2{X: -w * db / da, Y: w}
	}
	if p.plane {
		v := a.Scale(d.X).Add(b.Scale(d.Y))
		d = geom.Vec2{X: v.Dot(s.plane.XAxis()), Y: v.Dot(s.plane.YAxis())}
	}
	if !finite(d.X) || !finite(d.Y) {
		return geom.Vec2{}, ErrSingular
	}
	return d, nil
}

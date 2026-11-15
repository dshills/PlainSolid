package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type SplineTubeFaceTrimOptions struct {
	Loops trimvalidate.Options
	geom.TubeTrimIntersectionOptions
	// Projection selects model axes for graph comparison; nil is XY.
	// UV parameters and model events always remain in original units.
	Projection *[2]int
}

// SplineTubeTrimSource identifies one original face coedge and active spline
// span. Span is indexed in increasing original knot order. ParameterStart
// and ParameterEnd follow loop traversal, in original spline knot units for
// SplineUV and [0,1] for the other uses.
type SplineTubeTrimSource struct {
	Loop, Use, Edge, Span        int
	Reversed                     bool
	ParameterStart, ParameterEnd float64
}

type SplineTubeTrimContact struct {
	Source    SplineTubeTrimSource
	Parameter *geom.ExactAlgebraicRoot
	geom.TubeTrimContact
}

type SplineTubeTrimPending struct {
	Source     SplineTubeTrimSource
	Events     []*geom.ExactComposedCurveEvent
	Coincident []geom.ComposedCurvePlaneIntersection
}

type SplineTubeFaceTrimReport struct {
	Loops           trimvalidate.Report
	Contacts        []SplineTubeTrimContact
	Pending         []SplineTubeTrimPending
	ExactOperations int
	Complete        bool
	ProjectionAxes  [2]int
	ResidualAxis    int
}

// IntersectSplineTubeFaceTrims exhausts the selected verified tube segment
// against a certified outer loop and all holes, using the original face
// support. Positive-weight rational Bézier coedges through degree eight qualify.
// Coincidences and contacts on a dependent tube wall are unresolved.
// A contact can have multiple owners at a knot or face vertex; these records
// are kept separately. The report does not classify the intervening branch.
func IntersectSplineTubeFaceTrims(ctx context.Context, segment intersect.TubeSegment, chart int, f Face,
	o SplineTubeFaceTrimOptions) (r SplineTubeFaceTrimReport, err error) {
	if ctx == nil || chart < 0 || chart > 1 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.ProjectionAxes = [2]int{0, 1}
	if o.Projection != nil {
		r.ProjectionAxes = *o.Projection
	}
	a, b := r.ProjectionAxes[0], r.ProjectionAxes[1]
	if a < 0 || a > 2 || b < 0 || b > 2 || a == b {
		return r, geom.ErrInvalid
	}
	r.ResidualAxis = 3 - a - b
	trim := segment.TrimA()
	if chart == 1 {
		trim = segment.TrimB()
	}
	if !trim.HasSourceSurface(f.Surface) {
		return r, geom.ErrDomain
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxEvents < 1 || o.MaxSubdivisions < 0 ||
		!finite(o.Tolerance) || o.Tolerance < 0 {
		return r, geom.ErrInvalid
	}
	r.Loops, err = ValidateFaceTrimLoops(ctx, f, o.Loops)
	if err != nil {
		return r, err
	}
	if r.Loops.Status != trimvalidate.Certified {
		return r, geom.ErrConvergence
	}
	for i, loop := range append([][]Use{f.Outer}, f.Holes...) {
		for j, u := range loop {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			p := o.SurfacePatchOptions
			if p.MaxOperations == 0 {
				p.MaxOperations = o.MaxOperations
			}
			p.MaxOperations = min(p.MaxOperations, o.MaxOperations) - r.ExactOperations
			if p.MaxOperations < 1 {
				return r, geom.ErrConvergence
			}
			spans, used, e := tubeFaceTrimSpans(ctx, u, SplineTubeTrimSource{Loop: i, Use: j, Edge: u.Edge, Reversed: u.Reversed}, p)
			r.ExactOperations += used
			if e != nil {
				return r, e
			}
			for _, span := range spans {
				p.MaxOperations = o.MaxOperations - r.ExactOperations
				if p.MaxOperations < 1 {
					return r, geom.ErrConvergence
				}
				uv, e := exactBezierTrimParameterCurve(ctx, span.controls, p)
				if e != nil {
					return r, e
				}
				r.ExactOperations += uv.ExactOperations()
				child := o.TubeTrimIntersectionOptions
				child.MaxOperations -= r.ExactOperations
				child.MaxEvents -= len(r.Contacts) + len(r.Pending)
				if child.MaxOperations < 1 || child.MaxEvents < 1 {
					return r, geom.ErrConvergence
				}
				var q geom.TubeTrimIntersection
				if o.Projection == nil || r.ProjectionAxes == [2]int{0, 1} {
					q, e = segment.IntersectRationalBezierTrim(ctx, chart, uv, child)
				} else {
					p, e2 := segment.IntersectRationalBezierTrimProjected(ctx, chart, uv, r.ProjectionAxes, child)
					q, e = p.TubeTrimIntersection, e2
				}
				r.ExactOperations += q.ExactOperations
				if e != nil && (!errors.Is(e, geom.ErrConvergence) || len(q.Pending)+len(q.Coincident) == 0) {
					return r, e
				}
				if len(q.Pending)+len(q.Coincident) > 0 {
					r.Pending = append(r.Pending, SplineTubeTrimPending{span.source, q.Pending, q.Coincident})
					// An unresolved span can leave later events unexamined.
					return r, geom.ErrConvergence
				}
				if !q.Complete || len(q.Hits) > child.MaxEvents {
					return r, geom.ErrConvergence
				}
				for _, h := range q.Hits {
					x := h.TrimEvent.Root()
					if span.source.ParameterStart != 0 || span.source.ParameterEnd != 1 {
						p.MaxOperations = o.MaxOperations - r.ExactOperations
						if p.MaxOperations < 1 {
							return r, geom.ErrConvergence
						}
						a, z := ratInset(span.source.ParameterStart), ratInset(span.source.ParameterEnd)
						x, used, e = x.AffineMap(ctx, a, new(big.Rat).Sub(z, a), geom.ExactCurvePlaneEventOptions{
							MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
						r.ExactOperations += used
						if e != nil {
							return r, e
						}
					}
					r.Contacts = append(r.Contacts, SplineTubeTrimContact{span.source, x, h})
				}
			}
		}
	}
	r.Complete = true
	return r, nil
}

type splineTubeTrimSpan struct {
	source   SplineTubeTrimSource
	controls [][3]*big.Rat
}

func tubeFaceTrimSpans(ctx context.Context, u Use, source SplineTubeTrimSource, p geom.SurfacePatchOptions) ([]splineTubeTrimSpan, int, error) {
	if q, ok := shortSplineTrim(u.Curve); ok {
		if q.Curve.Degree() > 8 {
			return nil, 0, geom.ErrConvergence
		}
		cells, used, e := q.Curve.ExactBezierSpans(ctx, q.Range.Min, q.Range.Max, geom.ExactBezierSpanOptions{SurfacePatchOptions: p})
		if e != nil {
			return nil, used, e
		}
		spans := make([]splineTubeTrimSpan, len(cells))
		for i := range spans {
			k := i
			if q.Reversed {
				k = len(cells) - 1 - i
			}
			c := cells[k]
			s := source
			s.Span, s.ParameterStart, s.ParameterEnd = k, c.Start, c.End
			if q.Reversed {
				s.ParameterStart, s.ParameterEnd = c.End, c.Start
			}
			spans[i].source = s
			spans[i].controls = make([][3]*big.Rat, max(3, len(c.Control)))
			for j, pole := range c.Control {
				if pole[2].Sign() != 0 {
					return nil, used, geom.ErrDomain
				}
				m := j
				if q.Reversed {
					m = len(c.Control) - 1 - j
				}
				if len(c.Control) == 2 {
					m *= 2
				}
				spans[i].controls[m] = [3]*big.Rat{pole[0], pole[1], pole[3]}
			}
			if len(c.Control) == 2 {
				h := spans[i].controls
				for j := range h[1] {
					h[1][j] = new(big.Rat).Quo(new(big.Rat).Add(h[0][j], h[2][j]), big.NewRat(2, 1))
				}
			}
		}
		return spans, used, nil
	}
	source.ParameterEnd = 1
	if q, ok := u.Curve.(ExactBezierUV); ok {
		return []splineTubeTrimSpan{{source, q.HomogeneousControls()}}, 0, nil
	}
	if q, ok := u.Curve.(*QuadraticUV); ok {
		if q == nil {
			return nil, 0, geom.ErrInvalid
		}
		u.Curve = *q
	}
	h, e := partitionQuadraticControls(u)
	if e != nil {
		return nil, 0, e
	}
	return []splineTubeTrimSpan{{source, h[:]}}, 0, nil
}

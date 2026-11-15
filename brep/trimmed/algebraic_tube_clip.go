package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineTubeClipOptions bounds the entire two-face proof. Intersection
// options supply the total exact algebra budget. Loops bounds the aggregate
// UV proof, including location of every interval witness.
type SplineTubeClipOptions struct {
	SplineTubeFaceTrimOptions
	MaxIntervals int
}

// SplineTubeClipEvent groups all proved coedge contacts at one branch point.
// TubeEnd is -1 for an interior event, or 0/1 in segment traversal order.
type SplineTubeClipEvent struct {
	Free                 *geom.ExactAlgebraicRoot
	TubeEnd              int
	Contacts             [2][]SplineTubeTrimContact
	spanStarts, spanEnds [2][]int
}

// SplineTubeClipInterval is an open interval in segment traversal order.
// Locations apply throughout it. Retained is true exactly when both
// original faces contain its interior. Endpoint incidence is in Events.
type SplineTubeClipInterval struct {
	Start, End int
	Witness    *big.Rat
	Locations  [2]trimvalidate.Location
	// CoincidentTrim indexes FaceTrims[chart].Coincidences, or is -1 when
	// this interval does not lie on an original trim coedge in that chart.
	CoincidentTrim [2]int
	// WitnessPoint is an exact point of both original supports.
	WitnessPoint geom.TubeAlgebraicGraphPointCertificate
	Retained     bool
}

type SplineTubeClipReport struct {
	FreeAxis                                    int
	Reversed                                    bool
	FaceTrims                                   [2]SplineTubeFaceTrimReport
	Events                                      []SplineTubeClipEvent
	Intervals                                   []SplineTubeClipInterval
	ExactOperations, LoopWork, LoopSubdivisions int
	Complete                                    bool
}

// ClipSplineTubeByFaceTrims proves which open parts of an already verified
// branch lie in both original faces. It exhausts all supported trim contacts,
// orders and groups exact roots, and proves one interior witness per interval.
// Continuity and absence of further boundary contacts make the witness
// classification constant over the entire open interval. This does not claim
// that one segment exhausts all contacts between the two surfaces.
func ClipSplineTubeByFaceTrims(ctx context.Context, segment intersect.TubeSegment, faces [2]Face,
	o SplineTubeClipOptions) (r SplineTubeClipReport, err error) {
	if ctx == nil {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 32000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 1024
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 || o.MaxIntervals < 1 || o.MaxIntervals > 65536 {
		return r, geom.ErrInvalid
	}
	o.Loops, err = o.Loops.WithDefaults()
	if err != nil {
		return r, err
	}
	axis, domain, reversed, e := segment.ExactFreeDomain()
	if e != nil {
		return r, e
	}
	r.FreeAxis, r.Reversed = axis, reversed
	exact := func() geom.ExactCurvePlaneEventOptions {
		return geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations - r.ExactOperations,
			MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	}
	loopBudget := func() trimvalidate.Options {
		q := o.Loops
		q.MaxWork -= r.LoopWork
		q.MaxSubdivisions -= r.LoopSubdivisions
		return q
	}
	charge := func(q trimvalidate.Report) {
		r.LoopWork += q.Work
		r.LoopSubdivisions += q.Subdivisions
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, b, p)
		r.ExactOperations += q.Operations
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q, e
	}
	start, used, e := geom.NewExactRationalRoot(ctx, domain.Lower, exact())
	r.ExactOperations += used
	if e != nil {
		return r, e
	}
	if exact().MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	end, used, e := geom.NewExactRationalRoot(ctx, domain.Upper, exact())
	r.ExactOperations += used
	if e != nil {
		return r, e
	}
	ends := [2]int{0, 1}
	if reversed {
		ends = [2]int{1, 0}
	}
	events := []SplineTubeClipEvent{{Free: start, TubeEnd: ends[0]}, {Free: end, TubeEnd: ends[1]}}
	insert := func(chart int, c SplineTubeTrimContact) (int, error) {
		x := c.Parameters[axis]
		lo, hi := 0, len(events)
		for lo < hi {
			m := (lo + hi) / 2
			order, e := compare(events[m].Free, x)
			if e != nil {
				return 0, e
			}
			if order.Order < 0 {
				lo = m + 1
			} else {
				hi = m
			}
		}
		if lo == len(events) {
			return 0, geom.ErrConvergence
		}
		order, e := compare(events[lo].Free, x)
		if e != nil {
			return 0, e
		}
		if order.Order != 0 {
			if lo == 0 || len(events)-1 >= o.MaxIntervals {
				return 0, geom.ErrConvergence
			}
			events = append(events, SplineTubeClipEvent{})
			copy(events[lo+1:], events[lo:])
			events[lo] = SplineTubeClipEvent{Free: x, TubeEnd: -1}
		}
		events[lo].Contacts[chart] = append(events[lo].Contacts[chart], c)
		return lo, nil
	}
	for chart, face := range faces {
		child := o.SplineTubeFaceTrimOptions
		child.MaxOperations -= r.ExactOperations
		child.Loops = loopBudget()
		if child.MaxOperations < 1 || child.Loops.MaxWork < 1 || child.Loops.MaxSubdivisions < 1 {
			return r, geom.ErrConvergence
		}
		q, e := IntersectSplineTubeFaceTrims(ctx, segment, chart, face, child)
		r.FaceTrims[chart] = q
		r.ExactOperations += q.ExactOperations
		charge(q.Loops)
		if e != nil {
			return r, e
		}
		if !q.Complete {
			return r, geom.ErrConvergence
		}
		for _, c := range q.Contacts {
			if _, e := insert(chart, c); e != nil {
				return r, e
			}
		}
		for i, span := range q.Coincidences {
			starts := 0
			if (span.FreeOrder < 0) != reversed {
				starts = 1
			}
			first, e := insert(chart, span.Ends[starts])
			if e != nil {
				return r, e
			}
			events[first].spanStarts[chart] = append(events[first].spanStarts[chart], i)
			last, e := insert(chart, span.Ends[1-starts])
			if e != nil {
				return r, e
			}
			events[last].spanEnds[chart] = append(events[last].spanEnds[chart], i)
		}
	}
	if reversed {
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}
	}
	r.Events = events
	active := [2]int{-1, -1}
	for i, a := range events {
		for chart := range faces {
			for _, span := range a.spanEnds[chart] {
				if active[chart] != span {
					return r, geom.ErrConvergence
				}
				active[chart] = -1
			}
			for _, span := range a.spanStarts[chart] {
				if active[chart] != -1 {
					return r, geom.ErrConvergence
				}
				active[chart] = span
			}
		}
		if i == len(events)-1 {
			if active != [2]int{-1, -1} {
				return r, geom.ErrConvergence
			}
			break
		}
		z := events[i+1]
		if reversed {
			a, z = z, a
		}
		q, e := compare(a.Free, z.Free)
		if e != nil {
			return r, e
		}
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 {
			return r, geom.ErrConvergence
		}
		w := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		child := o.SurfaceCurveAgreementOptions
		child.MaxOperations -= r.ExactOperations
		if child.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		pointOpts := geom.TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: child, Tolerance: o.Tolerance}
		var point geom.TubeAlgebraicGraphPointCertificate
		if o.Projection == nil || *o.Projection == [2]int{0, 1} {
			point, e = segment.EvaluateRationalGraph(ctx, w, pointOpts)
		} else {
			point, e = segment.EvaluateRationalProjectedGraph(ctx, w, *o.Projection, pointOpts)
		}
		r.ExactOperations += point.ExactOperations
		if e != nil {
			return r, e
		}
		if !point.Proven {
			return r, geom.ErrConvergence
		}
		iv := SplineTubeClipInterval{Start: i, End: i + 1, Witness: w, WitnessPoint: point, CoincidentTrim: active}
		for chart, face := range faces {
			if active[chart] != -1 {
				iv.Locations[chart] = trimvalidate.Boundary
				continue
			}
			for bit := 16; bit <= min(512, o.MaxCoefficientBits/2); bit *= 2 {
				width := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(bit)))
				var lo, hi [2]*big.Rat
				for j := range lo {
					root := point.Parameters[2*chart+j]
					p := exact()
					if p.MaxOperations < 1 {
						return r, geom.ErrConvergence
					}
					b, e := root.Enclose(ctx, width, p)
					r.ExactOperations += b.Operations
					if e != nil {
						return r, e
					}
					if !b.Converged {
						return r, geom.ErrConvergence
					}
					lo[j], hi[j] = b.Interval.Lower, b.Interval.Upper
				}
				l := loopBudget()
				if l.MaxWork < 1 || l.MaxSubdivisions < 1 {
					return r, geom.ErrConvergence
				}
				x, proof, e := LocateRationalBoxInFaceTrim(ctx, face, lo, hi, l)
				charge(proof)
				if e != nil {
					return r, e
				}
				if proof.Status == trimvalidate.Certified && (x == trimvalidate.Inside || x == trimvalidate.Outside) {
					iv.Locations[chart] = x
					break
				}
			}
			if iv.Locations[chart] == trimvalidate.Unknown {
				return r, geom.ErrConvergence
			}
		}
		iv.Retained = closedFaceLocation(iv.Locations[0]) && closedFaceLocation(iv.Locations[1])
		r.Intervals = append(r.Intervals, iv)
	}
	r.Complete = true
	return r, nil
}

func closedFaceLocation(x trimvalidate.Location) bool {
	return x == trimvalidate.Inside || x == trimvalidate.Boundary
}

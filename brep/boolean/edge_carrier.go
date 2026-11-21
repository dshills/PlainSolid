package boolean

import (
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// edgeSamples is the number of evenly spaced samples of I5, ends included.
const edgeSamples = 17

// edgeRefinements bounds how often one step between two samples of a
// circle or ellipse is halved (I5 step 6).
const edgeRefinements = 6

// failAt adds the place of an error raised below intake.
func failAt(err error, format string, args ...any) error {
	return fmt.Errorf("%w (%s)", err, fmt.Sprintf(format, args...))
}

// edgeCarrier derives the carrier and interval of an edge from the
// carriers s1 and s2 of the faces of its two uses (I5). The stored curve is
// only evaluated; its Go type is never inspected (S3).
func edgeCarrier(e parametric.Edge, s1, s2 surface, t tolerances) (edge, error) {
	at := func(k, of int) float64 {
		if k == of {
			return e.Range.Max
		}
		return e.Range.Min + (e.Range.Max-e.Range.Min)*float64(k)/float64(of)
	}
	points := make([]geom.Vec3, edgeSamples)
	for k := range points {
		p, err := e.Curve.Eval(at(k, edgeSamples-1))
		if err != nil || !p.IsFinite() {
			return edge{}, fail(ErrInvalid, "edge carrier", "sample %d cannot be evaluated", k)
		}
		if d := math.Max(math.Abs(s1.offset(p)), math.Abs(s2.offset(p))); !(d <= t.eps) {
			return edge{}, fail(ErrInvalid, "edge carrier", "sample %d is %g from a face of the edge", k, d)
		}
		points[k] = p
	}
	candidates, err := edgeCandidates(points, s1, s2, t)
	if err != nil {
		return edge{}, err
	}
	var chosen curve
	found, least := 0, math.Inf(1)
	for _, c := range candidates {
		worst := 0.
		for _, p := range points {
			worst = math.Max(worst, off(c, p))
		}
		if least = math.Min(least, worst); worst <= t.eps {
			chosen = c
			found++
		}
	}
	switch {
	case found == 0:
		return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve lies on no line, circle or ellipse of its faces (deviation %g)", least)
	case found > 1:
		return edge{}, fail(ErrTolerance, "edge carrier", "two carriers fit the stored curve")
	}
	// Interval and direction (step 6). The advance from one sample to the
	// next is the difference of their carrier parameters, the shorter way
	// round on a closed carrier.
	period := chosen.period()
	advance := func(p, q geom.Vec3) float64 {
		d := chosen.param(q) - chosen.param(p)
		if period > 0 {
			d -= period * math.Round(d/period)
		}
		return d
	}
	out := edge{start: e.Start, end: e.End, box: around(points...)}
	if period == 0 {
		// On a line: oriented from the first sample to the last, and
		// every advance positive. No added sample can mend a step back.
		if chosen.param(points[edgeSamples-1]) < chosen.param(points[0]) {
			chosen = chosen.flipped()
		}
		for k := 0; k+1 < edgeSamples; k++ {
			if !(advance(points[k], points[k+1]) > 0) {
				return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve runs back along its carrier after sample %d", k)
			}
		}
		out.c, out.t0, out.t1 = chosen, chosen.param(points[0]), chosen.param(points[edgeSamples-1])
		return out, nil
	}
	// On a circle or ellipse the direction is the one most of the sixteen
	// first advances have: only a step of more than half a period is
	// measured the wrong way, and there is at most one.
	with, against := 0, 0
	for k := 0; k+1 < edgeSamples; k++ {
		switch d := advance(points[k], points[k+1]); {
		case d > 0:
			with++
		case d < 0:
			against++
		}
	}
	if with == against {
		return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve has no direction along its carrier")
	}
	if against > with {
		chosen = chosen.flipped()
	}
	// A step is halved, at most six times, while it advances more than an
	// eighth of a period or against the edge. Every added sample must lie
	// on the faces and on the carrier like the first ones.
	total := 0.
	var walk func(u0, u1 float64, p, q geom.Vec3, depth int) error
	walk = func(u0, u1 float64, p, q geom.Vec3, depth int) error {
		d := advance(p, q)
		if d >= 0 && d <= period/8 {
			total += d
			return nil
		}
		if depth == 0 {
			return fail(ErrUnsupported, "edge carrier", "the stored curve does not advance along its carrier")
		}
		mid := (u0 + u1) / 2
		m, err := e.Curve.Eval(mid)
		if err != nil || !m.IsFinite() {
			return fail(ErrInvalid, "edge carrier", "an added sample cannot be evaluated")
		}
		if gap := math.Max(math.Abs(s1.offset(m)), math.Abs(s2.offset(m))); !(gap <= t.eps) {
			return fail(ErrInvalid, "edge carrier", "an added sample is %g from a face of the edge", gap)
		}
		if gap := off(chosen, m); !(gap <= t.eps) {
			return fail(ErrUnsupported, "edge carrier", "an added sample is %g from the carrier", gap)
		}
		if err := walk(u0, mid, p, m, depth-1); err != nil {
			return err
		}
		return walk(mid, u1, m, q, depth-1)
	}
	for k := 0; k+1 < edgeSamples; k++ {
		if err := walk(at(k, edgeSamples-1), at(k+1, edgeSamples-1), points[k], points[k+1], edgeRefinements); err != nil {
			return edge{}, err
		}
	}
	// A closed edge has the whole period as its interval. The bounds of
	// an arc reach past its samples by its sagitta.
	closed := total > period/2 && points[0].Distance(points[edgeSamples-1]) <= 2*t.eps
	if (e.Start == e.End) != closed || total > period*(1+1e-9) {
		return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve runs %g of a turn between its ends", total/period)
	}
	out.c, out.t0 = chosen, chosen.param(points[0])
	if out.t1 = out.t0 + total; closed {
		out.t1 = out.t0 + period
	}
	if f, ok := chosen.(termed); ok {
		_, _, along, across := f.terms()
		out.box = out.box.grown(math.Max(along.Length(), across.Length()) * (1 - math.Cos(total/(2*float64(edgeSamples-1)))))
	} else {
		out.box = out.box.with(leg{out.c, out.t0, out.t1}.box())
	}
	return out, nil
}

// edgeCandidates lists the carriers an edge may lie on (I5 steps 3 and 4).
// For two different surfaces they are the carrier curves of the row of
// section 7.4, the table that the face-with-face pass uses. For one
// surface the faces fix no curve and the samples do: a line through the
// two samples farthest apart, or the curve in which the plane through
// three samples cuts the surface, or, on a plane, the circle or ellipse
// through samples.
func edgeCandidates(points []geom.Vec3, s1, s2 surface, t tolerances) ([]curve, error) {
	pair, err := surfacePair(s1, s2, t)
	if err != nil {
		return nil, err
	}
	if pair.open {
		return nil, fail(ErrUnsupported, "edge carrier", "the faces of the edge are a surface pair with no row")
	}
	if !pair.alike {
		return pair.curves, nil
	}
	// One surface. Where it is a plane the samples are taken in it: the
	// carrier of an edge lies in the carriers of its faces, and the
	// stored curve of a face that was seated (S7) is as far from its
	// plane as the face was.
	if own, ok := s1.(plane); ok {
		flat := make([]geom.Vec3, len(points))
		for i, p := range points {
			flat[i] = p.Sub(own.frame.ZAxis().Scale(own.offset(p)))
		}
		points = flat
	}
	a, b := 0, 0
	for i := range points {
		for j := i + 1; j < len(points); j++ {
			if points[i].Distance(points[j]) > points[a].Distance(points[b]) {
				a, b = i, j
			}
		}
	}
	dir, err := points[b].Sub(points[a]).Unit()
	if err != nil {
		return nil, fail(ErrInvalid, "edge carrier", "the stored curve is a single point")
	}
	straight := line{points[a], dir}
	fits := func(c curve) bool {
		for _, p := range points {
			if off(c, p) > t.eps {
				return false
			}
		}
		return true
	}
	if fits(straight) {
		return []curve{straight}, nil
	}
	// Three samples: the first, the one farthest from it, and the one
	// farthest from the line through those two.
	second, third := 0, 0
	for i, p := range points {
		if p.Distance(points[0]) > points[second].Distance(points[0]) {
			second = i
		}
	}
	chord := line{points[0], points[second].Sub(points[0]).Scale(1 / points[second].Distance(points[0]))}
	for i, p := range points {
		if off(chord, p) > off(chord, points[third]) {
			third = i
		}
	}
	frame, err := geom.NewFrame(points[0], chord.dir, chord.dir.Cross(points[third].Sub(points[0])))
	if err != nil {
		return nil, fail(ErrInvalid, "edge carrier", "the samples span no plane")
	}
	cutting, err := surfacePair(plane{frame}, s1, t)
	if err != nil {
		return nil, err
	}
	if !cutting.alike {
		return cutting.curves, nil
	}
	// The surface is that plane: the circle through the three samples
	// or, failing that, the conic through five of them.
	if c, ok := conicThrough([5]geom.Vec3{points[0], points[second], points[third], points[0], points[second]}, t.eps); ok && fits(c) {
		return []curve{c}, nil
	}
	if c, ok := conicThrough([5]geom.Vec3{points[0], points[3], points[6], points[9], points[12]}, t.eps); ok {
		return []curve{c}, nil
	}
	return nil, nil
}

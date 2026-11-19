package boolean

import (
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// edgeSamples is the number of evenly spaced samples of I5, ends included.
const edgeSamples = 17

// edgeRefinements bounds how often one step between two samples is halved
// when it does not advance along the carrier (I5 step 6).
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
	// Interval and direction (step 6). A step between two samples is
	// halved, up to six times, until it is short enough to be read
	// without doubt: on a closed carrier at most an eighth of a period.
	// The carrier is then turned along the edge, and every step must
	// advance; one that does not is halved again before the edge is
	// refused.
	type sample struct {
		s float64
		p geom.Vec3
	}
	period := chosen.period()
	step := func(a, b sample) float64 {
		d := chosen.param(b.p) - chosen.param(a.p)
		if period > 0 {
			d -= period * math.Round(d/period)
		}
		return d
	}
	var cut func(a, b sample, depth int, bad func(float64) bool) ([]sample, bool)
	cut = func(a, b sample, depth int, bad func(float64) bool) ([]sample, bool) {
		if !bad(step(a, b)) {
			return []sample{b}, true
		}
		p, err := e.Curve.Eval((a.s + b.s) / 2)
		if depth == 0 || err != nil || !p.IsFinite() {
			return nil, false
		}
		mid := sample{(a.s + b.s) / 2, p}
		first, ok := cut(a, mid, depth-1, bad)
		if !ok {
			return nil, false
		}
		second, ok := cut(mid, b, depth-1, bad)
		return append(first, second...), ok
	}
	refine := func(list []sample, bad func(float64) bool) ([]sample, int) {
		out := list[:1:1]
		for k := 1; k < len(list); k++ {
			more, ok := cut(list[k-1], list[k], edgeRefinements, bad)
			if !ok {
				return nil, k - 1
			}
			out = append(out, more...)
		}
		return out, -1
	}
	list := make([]sample, edgeSamples)
	for k, p := range points {
		list[k] = sample{at(k, edgeSamples-1), p}
	}
	list, failed := refine(list, func(d float64) bool { return period > 0 && math.Abs(d) > period/8 })
	total := 0.
	for k := 1; failed < 0 && k < len(list); k++ {
		total += step(list[k-1], list[k])
	}
	if failed < 0 && total < 0 {
		chosen, total = chosen.flipped(), -total
	}
	if failed < 0 {
		_, failed = refine(list, func(d float64) bool { return d <= 0 || period > 0 && d > period/8 })
	}
	if failed >= 0 {
		return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve does not advance along its carrier after sample %d", failed)
	}
	out := edge{start: e.Start, end: e.End, c: chosen, t0: chosen.param(points[0]), box: around(points...)}
	out.t1 = out.t0 + total
	if period > 0 {
		// A closed edge has the whole period as its interval. The bounds
		// of an arc reach past its samples by its sagitta.
		closed := total > period/2 && points[0].Distance(points[edgeSamples-1]) <= 2*t.eps
		if (e.Start == e.End) != closed || total > period*(1+1e-9) {
			return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve runs %g of a turn between its ends", total/period)
		}
		if closed {
			out.t1 = out.t0 + period
		}
		_, _, along, across := chosen.terms()
		out.box = out.box.grown(math.Max(along.Length(), across.Length()) * (1 - math.Cos(total/(2*float64(edgeSamples-1)))))
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

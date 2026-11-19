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
		return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve lies on no line of its faces (deviation %g); curved edges are not accepted before Part 2.1", least)
	case found > 1:
		return edge{}, fail(ErrTolerance, "edge carrier", "two carriers fit the stored curve")
	}
	// Interval and direction (step 6): orient the carrier along the edge
	// and require every step between samples to advance.
	if chosen.param(points[edgeSamples-1]) < chosen.param(points[0]) {
		chosen = chosen.flipped()
	}
	var advance func(s0, s1, u0, u1 float64, depth int) bool
	advance = func(s0, s1, u0, u1 float64, depth int) bool {
		if u1 > u0 {
			return true
		}
		if depth == 0 {
			return false
		}
		mid := (s0 + s1) / 2
		p, err := e.Curve.Eval(mid)
		if err != nil || !p.IsFinite() {
			return false
		}
		um := chosen.param(p)
		return advance(s0, mid, u0, um, depth-1) && advance(mid, s1, um, u1, depth-1)
	}
	for k := 0; k+1 < edgeSamples; k++ {
		u0, u1 := chosen.param(points[k]), chosen.param(points[k+1])
		if !advance(at(k, edgeSamples-1), at(k+1, edgeSamples-1), u0, u1, edgeRefinements) {
			return edge{}, fail(ErrUnsupported, "edge carrier", "the stored curve runs back along its carrier after sample %d", k)
		}
	}
	return edge{start: e.Start, end: e.End, c: chosen, t0: chosen.param(points[0]), t1: chosen.param(points[edgeSamples-1]), box: around(points...)}, nil
}

// edgeCandidates lists the carriers an edge may lie on (I5 steps 3 and 4).
// For two different surfaces they are the carrier curves of the row of
// section 7.4, the table that the face-with-face pass uses. For one
// surface the faces fix no curve and the samples do.
func edgeCandidates(points []geom.Vec3, s1, s2 surface, t tolerances) ([]curve, error) {
	pair, err := surfacePair(s1, s2, t)
	if err != nil {
		return nil, err
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
	return []curve{line{points[a], dir}}, nil
}

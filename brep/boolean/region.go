package boolean

import (
	"math"

	"plainsolid/geom"
)

// where is the answer of the tolerant point-in-loops test of X9.
type where uint8

const (
	without where = iota
	onRim
	within
)

// nearest is the point of the leg closest to p.
func (l leg) nearest(p geom.Vec3) geom.Vec3 {
	t := math.Max(math.Min(l.t0, l.t1), math.Min(math.Max(l.t0, l.t1), l.c.param(p)))
	return l.c.at(t)
}

// locate decides X9 for a point of radius r on a carrier, against the
// region that loops bound in the carrier's chart. A point within its
// radius of a loop is on the boundary; otherwise it is inside or outside
// by the number of times a chart ray from it crosses the loops.
func locate(ch chart, loops [][]leg, p geom.Vec3, r float64) where {
	for _, loop := range loops {
		for _, l := range loop {
			if p.Distance(l.nearest(p)) <= r {
				return onRim
			}
		}
	}
	q, crossings := ch.locate(p), 0
	for _, loop := range loops {
		points := ring(ch, loop)
		for i, a := range points {
			b := points[(i+1)%len(points)]
			if (a.Y > q.Y) != (b.Y > q.Y) && a.X+(q.Y-a.Y)/(b.Y-a.Y)*(b.X-a.X) > q.X {
				crossings++
			}
		}
	}
	if crossings%2 == 1 {
		return within
	}
	return without
}

// inFace decides X9 for a point against the trimmed region of an input
// face.
func (a *arrangement) inFace(op, f int, p geom.Vec3, r float64) where {
	return locate(a.ops[op].faces[f].s.chart(), a.borders[op][f], p, r)
}

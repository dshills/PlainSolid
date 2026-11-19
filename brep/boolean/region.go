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

// locate decides X9 for a point of radius r on a carrier, against the
// region that loops bound in the carrier's chart. A point within its
// radius of a loop is on the boundary. Otherwise it is inside or outside
// by the number of times the chart line from it along the second chart
// direction crosses the loops. That direction does not repeat, so the
// count is the same on a chart whose first direction does. The line is
// moved sideways, by less than r, when it would graze the end of a leg:
// the point is farther than r from every leg, so the moved line starts
// in the same region.
func locate(ch chart, loops [][]leg, p geom.Vec3, r float64) where {
	var paths []path
	for _, loop := range loops {
		for _, l := range loop {
			if p.Distance(l.nearest(p)) <= r {
				return onRim
			}
			paths = append(paths, ch.image(l))
		}
	}
	q := ch.locate(p)
	count := 0
	for _, shift := range [...]float64{0, r / 4, -r / 4, r / 2, -r / 2, r / 8, -r / 8} {
		n, clean := crossings(paths, q.X+shift, q.Y, ch.period(), r/32)
		if count = n; clean {
			break
		}
	}
	if count%2 == 1 {
		return within
	}
	return without
}

// crossings counts the points of the paths on the chart line of first
// coordinate x, modulo the period, that lie beyond y in the second. It
// also reports whether the line stays clear, by more than guard, of every
// end of a path and of every place where a path turns back in the first
// coordinate; if not, the count is not to be trusted.
func crossings(paths []path, x, y, period, guard float64) (int, bool) {
	count, clean := 0, true
	for _, p := range paths {
		// Cut the path where its first coordinate turns back, so that
		// each part is monotone in it.
		lo, hi := math.Min(p.t0, p.t1), math.Max(p.t0, p.t1)
		cuts := []float64{lo}
		if p.lin.X == 0 && (p.cos.X != 0 || p.sin.X != 0) {
			turn := math.Atan2(p.sin.X, p.cos.X)
			for t := turn + math.Pi*math.Ceil((lo-turn)/math.Pi); t < hi; t += math.Pi {
				if t > lo {
					cuts = append(cuts, t)
				}
			}
		}
		cuts = append(cuts, hi)
		for i := 0; i+1 < len(cuts); i++ {
			ta, tb := cuts[i], cuts[i+1]
			xa, xb := p.at(ta).X, p.at(tb).X
			first, last := 0., 0.
			if period > 0 {
				first, last = math.Ceil((math.Min(xa, xb)-guard-x)/period), math.Floor((math.Max(xa, xb)+guard-x)/period)
			}
			for k := first; k <= last; k++ {
				at := x + k*period
				if math.Abs(at-xa) <= guard || math.Abs(at-xb) <= guard {
					clean = false
				}
				if (xa > at) == (xb > at) {
					continue
				}
				// Bisect for the parameter where the path is on the line.
				a, b := ta, tb
				for n := 0; n < 60; n++ {
					mid := (a + b) / 2
					if (p.at(mid).X > at) == (xa > at) {
						a = mid
					} else {
						b = mid
					}
				}
				if p.at((a+b)/2).Y > y {
					count++
				}
			}
		}
	}
	return count, clean
}

// inFace decides X9 for a point against the trimmed region of an input
// face.
func (a *arrangement) inFace(op, f int, p geom.Vec3, r float64) where {
	return locate(a.ops[op].faces[f].s.chart(), a.borders[op][f], p, r)
}

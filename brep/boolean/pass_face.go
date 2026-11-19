package boolean

import (
	"context"
	"math"

	"plainsolid/geom"
)

// arrange runs the intersection stage: the passes of section 7.3 in
// their fixed order, then seal. Its result is the only data the building
// stage reads (AR8).
func arrange(ctx context.Context, ops [2]*operand, t tolerances, o Options) (*arrangement, error) {
	a, err := newArrangement(ctx, ops, t, o)
	if err != nil {
		return nil, err
	}
	for _, pass := range []func() error{a.lower, a.faceWithFace, a.tracesCross, a.seal} {
		if err := pass(); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// faceWithFace is X6. A pair of faces is resolved by the table of
// section 7.4 into no contact, a same-surface pair, or carrier curves
// that are laid into both faces as traces.
func (a *arrangement) faceWithFace() error {
	for fa, x := range a.ops[0].faces {
		for fb, y := range a.ops[1].faces {
			if x.box.clear(y.box, a.t.margin) {
				continue
			}
			if err := a.tick(); err != nil {
				return err
			}
			pair, err := surfacePair(x.s, y.s, a.t)
			if err != nil {
				return failAt(err, "face %d of A with face %d of B", fa, fb)
			}
			if pair.alike {
				same, err := sameSurface([2]surface{x.s, y.s}, [2][]point{a.rimNodes(0, fa), a.rimNodes(1, fb)})
				if err != nil {
					return failAt(err, "face %d of A with face %d of B", fa, fb)
				}
				if same {
					at := a.ops[0].verts[a.ops[0].edges[x.loops[0][0].edge].start]
					a.pairs = append(a.pairs, samePair{[2]int{fa, fb}, (x.s.normal(at).Dot(y.s.normal(at)) < 0) != (x.reversed != y.reversed)})
					continue
				}
			}
			if pair.open && !a.apart(fa, fb) {
				// SP3: no closed form, and SP2 does not prove them separate.
				return fail(ErrUnsupported, "X6", "face %d of A and face %d of B are cylinders with axes that are not parallel", fa, fb)
			}
			for i, c := range pair.curves {
				if err := a.lay(c, fa, fb, pair.touching[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// extent is the part of a cylinder's axis that a face reaches, as two
// points, grown by ε at each end.
func (a *arrangement) extent(op, f int) (geom.Vec3, geom.Vec3) {
	face := a.ops[op].faces[f]
	axis := face.s.(cylinder).axis()
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, x := range [2]float64{face.box.lo.X, face.box.hi.X} {
		for _, y := range [2]float64{face.box.lo.Y, face.box.hi.Y} {
			for _, z := range [2]float64{face.box.lo.Z, face.box.hi.Z} {
				u := axis.param(geom.Vec3{X: x, Y: y, Z: z})
				lo, hi = math.Min(lo, u), math.Max(hi, u)
			}
		}
	}
	return axis.at(lo - a.t.eps), axis.at(hi + a.t.eps)
}

// apart is the proof of separation of SP2 for two cylinder faces whose
// axes are not parallel: their bounds grown by ε do not overlap, or their
// axis segments are farther apart than the two radii and ε.
func (a *arrangement) apart(fa, fb int) bool {
	x, y := a.ops[0].faces[fa], a.ops[1].faces[fb]
	if x.box.clear(y.box, a.t.eps) {
		return true
	}
	p0, p1 := a.extent(0, fa)
	q0, q1 := a.extent(1, fb)
	return segmentDistance(p0, p1, q0, q1) > x.s.(cylinder).radius+y.s.(cylinder).radius+a.t.eps
}

// rimNodes lists the nodes on the boundary of a face, each once.
func (a *arrangement) rimNodes(op, f int) []point {
	var out []point
	seen := filled(a.nodes, false)
	for _, loop := range a.ops[op].faces[f].loops {
		for _, u := range loop {
			for _, s := range a.stops[op][u.edge] {
				if n := a.find(s.node); !seen[n] {
					seen[n] = true
					out = append(out, point{a.nodes[n].pos, a.nodes[n].radius})
				}
			}
		}
	}
	return out
}

// tracesCross is X8: two traces in one face that cross at a point that
// is not a node contradict the passes that should have found it.
func (a *arrangement) tracesCross() error {
	for i, x := range a.traces {
		for j := i + 1; j < len(a.traces); j++ {
			y := a.traces[j]
			if x.faces[0] != y.faces[0] && x.faces[1] != y.faces[1] {
				continue
			}
			if err := a.ctx.Err(); err != nil {
				return err
			}
			m, err := curveMeets(x.c, y.c, a.t)
			if err != nil {
				continue // the table has no row to check this pair with
			}
			for _, h := range m.hits {
				if h.gap > 2*a.t.eps || h.p.Distance(leg{x.c, x.t0, x.t1}.nearest(h.p)) > a.t.eps || h.p.Distance(leg{y.c, y.t0, y.t1}.nearest(h.p)) > a.t.eps {
					continue
				}
				if a.covers(x.ends[0], h.p) || a.covers(x.ends[1], h.p) || a.covers(y.ends[0], h.p) || a.covers(y.ends[1], h.p) {
					continue
				}
				return fail(ErrInconsistent, "X8", "two traces cross at %v, which is not a node", h.p)
			}
		}
	}
	return nil
}

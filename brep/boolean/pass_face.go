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
				if pair.narrow {
					return fail(ErrUnsupported, "X6", "face %d of A and face %d of B are cylinders of equal radius whose axes meet and are too nearly parallel", fa, fb)
				}
				return fail(ErrUnsupported, "X6", "face %d of A and face %d of B are cylinders with axes that are not parallel", fa, fb)
			}
			if err := a.meetings(pair.curves, fa, fb); err != nil {
				return err
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

// meetings is X7: where two carrier curves of one surface pair meet each
// other inside both faces, a node is made before X6 cuts the curves, so
// that both are cut there. No pair of the trial's table has two curves
// that meet, two lines along a cylinder being parallel; the rule is here
// because section 7.3 states it for any pair.
func (a *arrangement) meetings(curves []curve, fa, fb int) error {
	for i, c := range curves {
		for _, other := range curves[:i] {
			m, err := curveMeets(c, other, a.t)
			if err != nil {
				return failAt(err, "face %d of A with face %d of B", fa, fb)
			}
			for _, h := range m.hits {
				if a.inFace(0, fa, h.p, a.t.eps) != within || a.inFace(1, fb, h.p, a.t.eps) != within {
					continue
				}
				n, err := a.add(h.p)
				if err != nil {
					return err
				}
				a.loose[0][fa], a.loose[1][fb] = append(a.loose[0][fa], n), append(a.loose[1][fb], n)
			}
		}
	}
	return nil
}

// extent is the part of a cylinder's axis that a face reaches, as two
// points, grown by the filter margin at each end.
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
	return axis.at(lo - a.t.margin), axis.at(hi + a.t.margin)
}

// apart is the proof of separation of SP2 for two cylinder faces whose
// axes are not parallel: their bounds grown by the filter margin do not
// overlap, or their axis segments are farther apart than the two radii
// and twice the margin (T4), or one face is clear of the whole of the
// other's cylinder, or subdivision separates them.
func (a *arrangement) apart(fa, fb int) bool {
	x, y := a.ops[0].faces[fa], a.ops[1].faces[fb]
	if x.box.clear(y.box, a.t.margin) {
		return true
	}
	p0, p1 := a.extent(0, fa)
	q0, q1 := a.extent(1, fb)
	if segmentDistance(p0, p1, q0, q1) > x.s.(cylinder).radius+y.s.(cylinder).radius+2*a.t.margin {
		return true
	}
	if a.clearOf(0, fa, y.s.(cylinder)) || a.clearOf(1, fb, x.s.(cylinder)) {
		return true
	}
	p, q := patchOf(x.s.(cylinder), p0, p1), patchOf(y.s.(cylinder), q0, q1)
	tests := separationTests
	return a.separate(p, q, x.box, y.box, 2*separationDepth, &tests)
}

// patch is a rectangle of a cylinder: an interval of the angle about the
// axis and an interval along it.
type patch struct {
	s              cylinder
	a0, a1, v0, v1 float64
}

// patchOf is the whole turn of a cylinder between two points of its axis.
func patchOf(s cylinder, from, to geom.Vec3) patch {
	return patch{s, 0, 2 * math.Pi, s.axis().param(from), s.axis().param(to)}
}

// extent holds the patch. Every point of an arc is within its sagitta of
// its chord, and the patch is straight along the axis, so the box of the
// four corners grown by the sagitta holds every point of it.
func (p patch) extent() bounds {
	corner := func(angle, v float64) geom.Vec3 {
		return p.s.axis().at(v).Add(p.s.frame.XAxis().Scale(p.s.radius * math.Cos(angle))).Add(p.s.frame.YAxis().Scale(p.s.radius * math.Sin(angle)))
	}
	return around(corner(p.a0, p.v0), corner(p.a1, p.v0), corner(p.a0, p.v1), corner(p.a1, p.v1)).grown(p.s.radius * (1 - math.Cos((p.a1-p.a0)/2)))
}

// The subdivision of SP2 halves each of the two patches at most
// separationDepth times each way, the larger of the two at each step, and
// makes at most separationTests comparisons of two boxes.
const (
	separationDepth = 12
	separationTests = 20000
)

// separate proves that two cylinder faces do not come within twice the
// filter margin of each other. Each face lies in a patch of its cylinder
// and in its own bounds, so its points in a part of the patch lie in the
// box of that part cut down to those bounds. Two parts whose boxes are
// clear by the margin hold no pair of near points; otherwise the larger
// part is halved both ways and its four quarters are tried. When the
// depth or the number of tests is used up nothing is proved, and the
// pair stays unsupported (SP3). No property of the faces is used but
// their carriers and bounds.
func (a *arrangement) separate(p, q patch, bp, bq bounds, depth int, tests *int) bool {
	*tests--
	x, y := p.extent(), q.extent()
	x = bounds{geom.Vec3{X: math.Max(x.lo.X, bp.lo.X), Y: math.Max(x.lo.Y, bp.lo.Y), Z: math.Max(x.lo.Z, bp.lo.Z)}, geom.Vec3{X: math.Min(x.hi.X, bp.hi.X), Y: math.Min(x.hi.Y, bp.hi.Y), Z: math.Min(x.hi.Z, bp.hi.Z)}}
	y = bounds{geom.Vec3{X: math.Max(y.lo.X, bq.lo.X), Y: math.Max(y.lo.Y, bq.lo.Y), Z: math.Max(y.lo.Z, bq.lo.Z)}, geom.Vec3{X: math.Min(y.hi.X, bq.hi.X), Y: math.Min(y.hi.Y, bq.hi.Y), Z: math.Min(y.hi.Z, bq.hi.Z)}}
	empty := func(b bounds) bool {
		// The bounds of a face are those of its points, so a part whose
		// box misses them by the margin holds no point of the face.
		return b.lo.X-b.hi.X > 2*a.t.margin || b.lo.Y-b.hi.Y > 2*a.t.margin || b.lo.Z-b.hi.Z > 2*a.t.margin
	}
	if empty(x) || empty(y) || x.clear(y, a.t.margin) {
		return true
	}
	if depth == 0 || *tests <= 0 {
		return false
	}
	size := func(p patch) float64 { return math.Max(p.s.radius*(p.a1-p.a0), math.Abs(p.v1-p.v0)) }
	if size(p) < size(q) {
		p, q, bp, bq = q, p, bq, bp
	}
	am, vm := (p.a0+p.a1)/2, (p.v0+p.v1)/2
	for _, part := range [4]patch{{p.s, p.a0, am, p.v0, vm}, {p.s, am, p.a1, p.v0, vm}, {p.s, p.a0, am, vm, p.v1}, {p.s, am, p.a1, vm, p.v1}} {
		if !a.separate(part, q, bp, bq, depth-1, tests) {
			return false
		}
	}
	return true
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

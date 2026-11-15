package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ConvexArcExtrusionBody describes a positive extrusion of a convex CCW
// line/circle profile in a proper signed-coordinate frame. Original angles
// and anchored endpoints retain ExtrudeArcLoop's bounded trig semantics;
// this is not an exact algebraic circle-incidence certificate.
type ConvexArcExtrusionBody struct {
	Name     string
	Frame    geom.Frame
	Height   float64
	Segments []trimmed.ExtrusionSegment
}
type ConvexArcExtrusionModel struct {
	Body       ConvexArcExtrusionBody
	LengthUnit float64
}

func BuildConvexArcExtrusion(p ConvexArcExtrusionBody) (trimmed.ArcLoopExtrusion, error) {
	fail := func() (trimmed.ArcLoopExtrusion, error) { return trimmed.ArcLoopExtrusion{}, geom.ErrDomain }
	if len(p.Segments) < 3 || len(p.Segments) > 32 || !finiteOblique(p.Height) || p.Height <= 0 {
		return fail()
	}
	for _, v := range []geom.Vec3{p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis()} {
		n := 0
		for _, c := range []float64{v.X, v.Y, v.Z} {
			if c != 0 {
				if c != 1 && c != -1 {
					return fail()
				}
				n++
			}
		}
		if n != 1 {
			return fail()
		}
	}
	tangent := func(s trimmed.ExtrusionSegment, end bool) geom.Vec2 {
		if s.Radius == 0 {
			return geom.Vec2{X: s.End.X - s.Start.X, Y: s.End.Y - s.Start.Y}
		}
		q := s.Start
		if end {
			q = s.End
		}
		return geom.Vec2{X: s.Center.Y - q.Y, Y: q.X - s.Center.X}
	}
	arcs, lines := 0, 0
	scale := max(1, p.Height)
	for i, s := range p.Segments {
		if s.Radius > 0 {
			arcs++
			if !finiteOblique(s.Sweep) || s.Sweep <= 0 || s.Sweep >= math.Pi {
				return fail()
			}
			scale = max(scale, s.Radius)
		} else {
			lines++
		}
		a, b := tangent(s, true), tangent(p.Segments[(i+1)%len(p.Segments)], false)
		turn, e := geom.Orient2D(geom.Vec2{}, a, b)
		if e != nil || turn < 0 {
			return fail()
		}
		if turn == 0 {
			ax, ay, bx, by := new(big.Rat).SetFloat64(a.X), new(big.Rat).SetFloat64(a.Y), new(big.Rat).SetFloat64(b.X), new(big.Rat).SetFloat64(b.Y)
			if ax == nil || ay == nil || bx == nil || by == nil || new(big.Rat).Add(new(big.Rat).Mul(ax, bx), new(big.Rat).Mul(ay, by)).Sign() <= 0 {
				return fail()
			}
		}
		for j, q := range p.Segments {
			if i != j && q.Start == s.Start {
				return fail()
			}
			turn, e := geom.Orient2D(s.Start, s.End, q.Start)
			if e != nil || turn < 0 {
				return fail()
			}
		}

	}
	if arcs < 1 || arcs > 4 || lines < 1 {
		return fail()
	}
	return trimmed.ExtrudeArcLoop(context.Background(), p.Frame, p.Segments, p.Height, scale*1e-10)
}
func writeConvexArcExtrusion(w io.Writer, m ConvexArcExtrusionModel) error {
	s, e := BuildConvexArcExtrusion(m.Body)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s.Solid, nativeLongEdgeLimits{50, 144, 96, 1, 48})
}
func WriteConvexArcExtrusion(w io.Writer, m ConvexArcExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeConvexArcExtrusion(&b, m); e != nil {
		return e
	}
	if _, e := ReadConvexArcExtrusion(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}

// ReadConvexArcExtrusion accepts only a complete regenerated canonical graph.
// At most four circular runs and 81 represented sweep combinations are tried;
// source owner pcurves determine every chart angle, not atan2 fitting.
func ReadConvexArcExtrusion(r io.Reader, l Limits) (ConvexArcExtrusionModel, error) {
	fail := func() (ConvexArcExtrusionModel, error) { return ConvexArcExtrusionModel{}, circularSourceFailure() }
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 8192)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	l.MaxFaces = min(l.MaxFaces, 50)
	l.MaxVertices = min(l.MaxVertices, 96)
	l.MaxEdgeUses = min(l.MaxEdgeUses, 288)
	l.MaxSolids = min(l.MaxSolids, 1)
	f, _, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil || len(ids) != 1 {
		return fail()
	}
	body, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	ids, e = listRefs(shell.Args[1], 5)
	if e != nil || len(ids) > 50 {
		return fail()
	}
	faces := make([]circularSourceFace, len(ids))
	for i, id := range ids {
		faces[i], e = readCircularSourceFaceBounded(f, id, 1, 48)
		if e != nil || len(faces[i].loops) != 1 {
			return fail()
		}
	}
	low, high := faces[0], faces[1]
	if low.kind != "PLANE" || high.kind != "PLANE" || low.sense || !high.sense || len(low.loops[0]) != len(ids)-2 || len(high.loops[0]) != len(ids)-2 {
		return fail()
	}
	frame := low.frame
	delta := frame.ToLocal(high.frame.Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 || high.frame.ZAxis() != frame.ZAxis() {
		return fail()
	}
	type run struct {
		segment trimmed.ExtrusionSegment
		angles  []float64
	}
	var runs []run
	for _, wall := range faces[2:] {
		if !wall.sense || len(wall.loops[0]) != 4 {
			return fail()
		}
		if wall.kind == "PLANE" {
			found := false
			var s trimmed.ExtrusionSegment
			for _, use := range wall.loops[0] {
				ed, err := readCircularEdge(f, use.id, true)
				if err != nil || ed.kind != "LINE" {
					return fail()
				}
				a, err := analyticVertex(f, ed.a)
				if err != nil {
					return fail()
				}
				b, err := analyticVertex(f, ed.b)
				if err != nil {
					return fail()
				}
				a, b = frame.ToLocal(a), frame.ToLocal(b)
				if a.Z == 0 && b.Z == 0 {
					if found {
						return fail()
					}
					found = true
					if !use.sense {
						a, b = b, a
					}
					s.Start = geom.Vec2{X: a.X, Y: a.Y}
					s.End = geom.Vec2{X: b.X, Y: b.Y}
				}
			}
			if !found {
				return fail()
			}
			runs = append(runs, run{segment: s})
			continue
		}
		if wall.kind != "CYLINDER" || wall.frame.ZAxis() != frame.ZAxis() || wall.frame.XAxis() != frame.XAxis() {
			return fail()
		}
		center := frame.ToLocal(wall.frame.Origin())
		if center.Z != 0 {
			return fail()
		}
		type endpoint struct {
			u float64
			p geom.Vec2
		}
		var ends []endpoint
		for _, use := range wall.loops[0] {
			ed, err := readCircularEdge(f, use.id, true)
			if err != nil {
				return fail()
			}
			if ed.kind != "LINE" {
				if ed.kind != "CIRCLE" {
					return fail()
				}
				continue
			}
			found := false
			for _, pc := range ed.pcs {
				owner, g, err := circularPCurveGeometry(f, pc)
				if err != nil {
					return fail()
				}
				if owner != wall.surface {
					continue
				}
				if found {
					return fail()
				}
				found = true
				uv, d, err := circularUnitLineGeometry(f, g, 2)
				if err != nil || d.X != 0 || math.Abs(d.Y) != 1 || (uv.Y != 0 && uv.Y != delta.Z) {
					return fail()
				}
				a, err := analyticVertex(f, ed.a)
				if err != nil {
					return fail()
				}
				b, err := analyticVertex(f, ed.b)
				if err != nil {
					return fail()
				}
				a, b = frame.ToLocal(a), frame.ToLocal(b)
				if a.Z != 0 {
					a, b = b, a
				}
				if a.Z != 0 || b.Z != delta.Z || a.X != b.X || a.Y != b.Y {
					return fail()
				}
				ends = append(ends, endpoint{uv.X, geom.Vec2{X: a.X, Y: a.Y}})
			}
			if !found {
				return fail()
			}
		}
		if len(ends) != 2 {
			return fail()
		}
		if ends[0].u > ends[1].u {
			ends[0], ends[1] = ends[1], ends[0]
		}
		if ends[0].u >= ends[1].u {
			return fail()
		}
		c := geom.Vec2{X: center.X, Y: center.Y}
		if len(runs) == 0 || runs[len(runs)-1].segment.Radius != wall.radius || runs[len(runs)-1].segment.Center != c {
			runs = append(runs, run{segment: trimmed.ExtrusionSegment{Start: ends[0].p, End: ends[0].p, Center: c, Radius: wall.radius}, angles: []float64{ends[0].u}})
		}
		last := &runs[len(runs)-1]
		if last.segment.End != ends[0].p || last.angles[len(last.angles)-1] != ends[0].u {
			return fail()
		}
		last.segment.End = ends[1].p
		last.angles = append(last.angles, ends[1].u)
	}
	if len(runs) < 3 || len(runs) > 32 {
		return fail()
	}
	candidates := make([][]trimmed.ExtrusionSegment, len(runs))
	arcs := 0
	for i, g := range runs {
		if g.segment.Radius == 0 {
			candidates[i] = []trimmed.ExtrusionSegment{g.segment}
			continue
		}
		arcs++
		if arcs > 4 {
			return fail()
		}
		start, base := g.angles[0], g.angles[len(g.angles)-1]-g.angles[0]
		for _, sweep := range []float64{base, math.Nextafter(base, math.Inf(-1)), math.Nextafter(base, math.Inf(1))} {
			n := int(math.Ceil(sweep / (math.Pi / 4)))
			if sweep <= 0 || sweep >= math.Pi {
				continue
			}
			if sweep == math.Pi/2 && start == math.Round(start/(math.Pi/2))*(math.Pi/2) && len(g.angles) == 2 {
				n = 1
			}
			if n != len(g.angles)-1 {
				continue
			}
			match := true
			for j, u := range g.angles {
				if u != start+sweep*float64(j)/float64(n) {
					match = false
					break
				}
			}
			if match {
				s := g.segment
				s.StartAngle = start
				s.Sweep = sweep
				candidates[i] = append(candidates[i], s)
			}
		}
		if len(candidates[i]) == 0 {
			return fail()
		}
	}
	m := ConvexArcExtrusionModel{Body: ConvexArcExtrusionBody{Name: name, Frame: frame, Height: delta.Z, Segments: make([]trimmed.ExtrusionSegment, len(runs))}, LengthUnit: unit}
	var search func(int) bool
	search = func(i int) bool {
		if i == len(candidates) {
			var b bytes.Buffer
			return writeConvexArcExtrusion(&b, m) == nil && verifyConvexArcGraph(f, &b) == nil
		}
		for _, s := range candidates[i] {
			m.Body.Segments[i] = s
			if search(i + 1) {
				return true
			}
		}
		return false
	}
	if search(0) {
		return m, nil
	}
	return fail()
}

// Compare both registered metadata roots in one bijection, so every entity,
// including context sharing, belongs to the regenerated canonical graph.
func verifyConvexArcGraph(source *File, generated io.Reader) error {
	expected, e := Parse(generated, Limits{})
	if e != nil {
		return e
	}
	root := func(f *File) (*File, error) {
		copyFile := *f
		copyFile.Entities = make(map[int64]Entity, len(f.Entities)+1)
		roots := make([]Value, 2)
		for id, en := range f.Entities {
			copyFile.Entities[id] = en
			if len(en.Records) != 1 {
				continue
			}
			for i, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION"} {
				if en.Records[0].Type == k {
					if roots[i].Kind == Reference {
						return nil, geom.ErrInvalid
					}
					roots[i] = Value{Kind: Reference, Ref: id}
				}
			}
		}
		for _, v := range roots {
			if v.Kind != Reference {
				return nil, geom.ErrInvalid
			}
		}
		if _, ok := copyFile.Entities[-1]; ok {
			return nil, geom.ErrInvalid
		}
		copyFile.Entities[-1] = Entity{ID: -1, Records: []Record{{Type: "CANONICAL_ROOTS", Args: roots}}}
		return &copyFile, nil
	}
	a, e := root(source)
	if e != nil {
		return e
	}
	b, e := root(expected)
	if e != nil {
		return e
	}
	used, e := equalCircularExtrusionGraph(a, -1, b, -1)
	if e != nil {
		return e
	}
	if len(used) != len(a.Entities) || len(a.Entities) != len(b.Entities) {
		return geom.ErrInvalid
	}
	return nil
}

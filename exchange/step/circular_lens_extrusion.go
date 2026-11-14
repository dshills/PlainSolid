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

// CircularLensExtrusionBody retains a convex intersection bounded by two
// positive minor circular arcs and a positive extrusion height.
type CircularLensExtrusionBody struct {
	Name   string
	Frame  geom.Frame
	Height float64
	Arcs   [2]trimmed.ExtrusionSegment
}
type CircularLensExtrusionModel struct {
	Body       CircularLensExtrusionBody
	LengthUnit float64
}

func BuildCircularLensExtrusion(p CircularLensExtrusionBody) (trimmed.ArcLoopExtrusion, error) {
	fail := func() (trimmed.ArcLoopExtrusion, error) { return trimmed.ArcLoopExtrusion{}, geom.ErrDomain }
	if !finiteOblique(p.Height) || p.Height <= 0 {
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
	scale := max(1, p.Height)
	for i, a := range p.Arcs {
		b := p.Arcs[1-i]
		if !finiteOblique(a.Center.X) || !finiteOblique(a.Center.Y) {
			return fail()
		}
		if a.Radius <= 0 || !finiteOblique(a.Radius) || a.Sweep <= 0 || a.Sweep >= math.Pi || !finiteOblique(a.StartAngle) || !finiteOblique(a.Sweep) || a.End != b.Start || a.Start == a.End || a.Center == b.Center {
			return fail()
		}
		incoming := geom.Vec2{X: a.Center.Y - a.End.Y, Y: a.End.X - a.Center.X}
		outgoing := geom.Vec2{X: b.Center.Y - b.Start.Y, Y: b.Start.X - b.Center.X}
		turn, e := geom.Orient2D(geom.Vec2{}, incoming, outgoing)
		if e != nil || turn <= 0 {
			return fail()
		}
		scale = max(scale, a.Radius, math.Hypot(a.End.X-a.Start.X, a.End.Y-a.Start.Y))
	}
	// Strict transversality uses the represented input coefficients exactly.
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	a, b := p.Arcs[0], p.Arcs[1]
	dx := new(big.Rat).Sub(rat(a.Center.X), rat(b.Center.X))
	dy := new(big.Rat).Sub(rat(a.Center.Y), rat(b.Center.Y))
	d2 := new(big.Rat).Add(square(dx), square(dy))
	sum := new(big.Rat).Add(rat(a.Radius), rat(b.Radius))
	difference := new(big.Rat).Sub(rat(a.Radius), rat(b.Radius))
	if d2.Cmp(square(difference)) <= 0 || d2.Cmp(square(sum)) >= 0 {
		return fail()
	}
	return trimmed.ExtrudeArcLoop(context.Background(), p.Frame, p.Arcs[:], p.Height, scale*1e-10)
}
func writeCircularLensExtrusion(w io.Writer, m CircularLensExtrusionModel) error {
	s, e := BuildCircularLensExtrusion(m.Body)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s.Solid, nativeLongEdgeLimits{10, 24, 16, 1, 8})
}
func WriteCircularLensExtrusion(w io.Writer, m CircularLensExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeCircularLensExtrusion(&b, m); e != nil {
		return e
	}
	if _, e := ReadCircularLensExtrusion(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}

// ReadCircularLensExtrusion recovers anchored endpoints from shared vertices
// and represented angles from axial pcurves. It accepts only exact native
// partition tables and a complete regenerated canonical graph.
func ReadCircularLensExtrusion(r io.Reader, l Limits) (CircularLensExtrusionModel, error) {
	fail := func() (CircularLensExtrusionModel, error) {
		return CircularLensExtrusionModel{}, circularSourceFailure()
	}
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
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
	ids, e = listRefs(shell.Args[1], 4)
	if e != nil || len(ids) > 10 {
		return fail()
	}
	faces := make([]circularSourceFace, len(ids))
	for i, id := range ids {
		faces[i], e = readCircularSourceFaceBounded(f, id, 1, 8)
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
		center     geom.Vec2
		radius     float64
		angles     []float64
		start, end geom.Vec2
	}
	var runs []run
	for _, wall := range faces[2:] {
		if wall.kind != "CYLINDER" || !wall.sense || len(wall.loops[0]) != 4 || wall.frame.ZAxis() != frame.ZAxis() || wall.frame.XAxis() != frame.XAxis() {
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
			edge, err := readCircularEdge(f, use.id, true)
			if err != nil {
				return fail()
			}
			if edge.kind != "LINE" {
				if edge.kind != "CIRCLE" {
					return fail()
				}
				continue
			}
			found := false
			for _, pc := range edge.pcs {
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
				if err != nil || d.X != 0 || math.Abs(d.Y) != 1 || uv.Y != 0 && uv.Y != delta.Z {
					return fail()
				}
				a, err := analyticVertex(f, edge.a)
				if err != nil {
					return fail()
				}
				b, err := analyticVertex(f, edge.b)
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
		if len(runs) == 0 || runs[len(runs)-1].center != c || runs[len(runs)-1].radius != wall.radius {
			runs = append(runs, run{center: c, radius: wall.radius, angles: []float64{ends[0].u}, start: ends[0].p, end: ends[0].p})
		}
		if len(runs) > 2 {
			return fail()
		}
		last := &runs[len(runs)-1]
		if last.angles[len(last.angles)-1] != ends[0].u || last.end != ends[0].p {
			return fail()
		}
		last.angles = append(last.angles, ends[1].u)
		last.end = ends[1].p
	}
	if len(runs) != 2 {
		return fail()
	}
	m := CircularLensExtrusionModel{LengthUnit: unit, Body: CircularLensExtrusionBody{Name: name, Frame: frame, Height: delta.Z}}

	// Subtracting represented endpoint angles may lose one ULP of the
	// authored sweep. Consider only the adjacent represented values; every
	// native subdivision angle and the complete output graph must match exactly.
	var candidates [2][]trimmed.ExtrusionSegment
	for i, g := range runs {
		start, base := g.angles[0], g.angles[len(g.angles)-1]-g.angles[0]
		for _, sweep := range []float64{base, math.Nextafter(base, math.Inf(-1)), math.Nextafter(base, math.Inf(1))} {
			n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 4)))
			if sweep <= 0 || sweep >= math.Pi || n != len(g.angles)-1 {
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
				candidates[i] = append(candidates[i], trimmed.ExtrusionSegment{Start: g.start, End: g.end, Center: g.center, Radius: g.radius, StartAngle: start, Sweep: sweep})
			}
		}
		if len(candidates[i]) == 0 {
			return fail()
		}
	}
	for _, a := range candidates[0] {
		for _, b := range candidates[1] {
			m.Body.Arcs = [2]trimmed.ExtrusionSegment{a, b}
			var out bytes.Buffer
			if e = writeCircularLensExtrusion(&out, m); e != nil {
				continue
			}
			if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e == nil {
				return m, nil
			}
		}
	}
	return fail()
}

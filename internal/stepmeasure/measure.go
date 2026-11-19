// Package stepmeasure measures the solid that a STEP file describes, from
// the entities of the file alone. It is a check for tests of a STEP writer:
// it shares no code with a writer or with the readers of exchange/step
// beyond the parser, and it builds no solid.
//
// It takes one MANIFOLD_SOLID_BREP whose faces lie on planes and circular
// cylinders and whose edges are lines, circles and ellipses, with any
// arrangement of faces, loops and edges.
package stepmeasure

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// Measures are the volume, the surface area and the box of a solid, in the
// length unit of its file.
type Measures struct {
	Volume, Area float64
	Box          geom.Box
	Faces, Edges int
}

// Boundary reads the one solid of a STEP file as its entities state it and
// measures it. The volume is the flux of x/3 through the faces and the area
// is the sum of the face areas, both taken as integrals around the loops of
// each face, so a face that is misoriented, a loop that runs the wrong way
// or an arc that takes the wrong side of its circle changes the result. It
// also requires what a closed shell must have: every loop closes vertex to
// vertex, every edge is used twice in opposite directions, every vertex of
// an edge lies on the edge's curve and on the surfaces of the faces that use
// it, and every face has positive area.
func Boundary(data []byte) (Measures, error) {
	file, err := step.Parse(bytes.NewReader(data), step.Limits{})
	if err != nil {
		return Measures{}, err
	}
	r := reader{file: file, points: map[int64]geom.Vec3{}, paths: map[int64]path{}, uses: map[int64][]bool{}}
	var solids []int64
	for _, id := range file.Order {
		if e := file.Entities[id]; len(e.Records) == 1 && e.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			solids = append(solids, id)
		}
	}
	if len(solids) != 1 {
		return Measures{}, fmt.Errorf("stepmeasure: %d solids, want one", len(solids))
	}
	shell := r.record(r.record(solids[0], "MANIFOLD_SOLID_BREP", 2).Args[1].Ref, "CLOSED_SHELL", 2)
	var out Measures
	for _, face := range shell.Args[1].Items {
		flux, area := r.face(face.Ref)
		if r.err == nil && !(area > 0) {
			r.fail("face #%d has area %g", face.Ref, area)
		}
		out.Volume += flux / 3
		out.Area += area
		out.Faces++
	}
	for id, senses := range r.uses {
		if len(senses) != 2 || senses[0] == senses[1] {
			r.fail("edge #%d is used %d times, or twice in one direction", id, len(senses))
		}
	}
	if r.err != nil {
		return Measures{}, r.err
	}
	if !(out.Volume > 0) {
		return Measures{}, fmt.Errorf("stepmeasure: volume %g", out.Volume)
	}
	out.Edges = len(r.paths)
	out.Box, err = geom.Bounds(r.extent)
	return out, err
}

type reader struct {
	file   *step.File
	err    error
	scale  float64 // the largest coordinate of a vertex seen so far
	points map[int64]geom.Vec3
	paths  map[int64]path
	uses   map[int64][]bool // by edge, the direction of each use
	extent []geom.Vec3      // points whose box is the box of the edges
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = errors.New("stepmeasure: " + fmt.Sprintf(format, args...))
	}
}

// near is the bound within which a point lies on a curve or a surface.
func (r *reader) near(d float64) bool { return d <= 1e-7*math.Max(1, r.scale) }

var none = step.Record{Args: make([]step.Value, 8)}

// record gives a simple entity of a type, or a blank one after a failure.
func (r *reader) record(id int64, kind string, args int) step.Record {
	e := r.file.Entities[id]
	if len(e.Records) != 1 || e.Records[0].Type != kind || len(e.Records[0].Args) != args {
		r.fail("#%d is not a %s of %d attributes", id, kind, args)
		return none
	}
	return e.Records[0]
}

func (r *reader) kind(id int64) string {
	if e := r.file.Entities[id]; len(e.Records) == 1 {
		return e.Records[0].Type
	}
	return ""
}

func (r *reader) number(v step.Value) float64 {
	x, err := strconv.ParseFloat(v.Text, 64)
	if v.Kind != step.Number || err != nil {
		r.fail("%q is not a number", v.Text)
	}
	return x
}

func (r *reader) flag(v step.Value) bool {
	if v.Kind != step.Enumeration || (v.Text != "T" && v.Text != "F") {
		r.fail("%q is not a logical", v.Text)
	}
	return v.Text == "T"
}

func (r *reader) triple(id int64, kind string) geom.Vec3 {
	c := r.record(id, kind, 2).Args[1].Items
	if len(c) != 3 {
		r.fail("#%d has %d coordinates", id, len(c))
		return geom.Vec3{}
	}
	return geom.Vec3{X: r.number(c[0]), Y: r.number(c[1]), Z: r.number(c[2])}
}

func (r *reader) frame(id int64) geom.Frame {
	a := r.record(id, "AXIS2_PLACEMENT_3D", 4).Args
	f, err := geom.NewFrame(r.triple(a[1].Ref, "CARTESIAN_POINT"), r.triple(a[3].Ref, "DIRECTION"), r.triple(a[2].Ref, "DIRECTION"))
	if err != nil && r.err == nil {
		r.fail("placement #%d: %v", id, err)
	}
	return f
}

func (r *reader) vertex(id int64) geom.Vec3 {
	if p, ok := r.points[id]; ok {
		return p
	}
	p := r.triple(r.record(id, "VERTEX_POINT", 2).Args[1].Ref, "CARTESIAN_POINT")
	r.points[id] = p
	r.scale = math.Max(r.scale, math.Max(math.Abs(p.X), math.Max(math.Abs(p.Y), math.Abs(p.Z))))
	return p
}

// path is an edge as a curve in space: its point and its velocity at a
// parameter that runs from t0 at the start vertex to t1 at the end vertex.
type path struct {
	from, to int64
	t0, t1   float64
	at       func(t float64) (p, d geom.Vec3)
	panels   int
}

// edge reads an EDGE_CURVE. An edge runs from its first vertex to its
// second along its curve, in the direction of the curve when same_sense is
// true and against it otherwise; on a closed curve that fixes which of the
// two arcs between the vertices is meant.
func (r *reader) edge(id int64) path {
	if p, ok := r.paths[id]; ok {
		return p
	}
	a := r.record(id, "EDGE_CURVE", 5).Args
	out := path{from: a[1].Ref, to: a[2].Ref, panels: 1}
	start, end, same := r.vertex(out.from), r.vertex(out.to), r.flag(a[4])
	r.extent = append(r.extent, start, end)
	// A curve may come wrapped with its p-curves; the curve in space is
	// what is measured.
	curve := a[3].Ref
	if kind := r.kind(curve); kind == "SURFACE_CURVE" || kind == "SEAM_CURVE" {
		curve = r.record(curve, kind, 4).Args[1].Ref
	}
	switch kind := r.kind(curve); kind {
	case "LINE":
		l := r.record(curve, "LINE", 3).Args
		v := r.record(l[2].Ref, "VECTOR", 3).Args
		origin, along := r.triple(l[1].Ref, "CARTESIAN_POINT"), r.triple(v[1].Ref, "DIRECTION")
		span := end.Sub(start)
		for _, p := range [2]geom.Vec3{start, end} {
			if !r.near(p.Sub(origin).Cross(along).Length() / math.Max(along.Length(), 1e-300)) {
				r.fail("edge #%d: a vertex is off its line", id)
			}
		}
		if (span.Dot(along) > 0) != same || out.from == out.to {
			r.fail("edge #%d runs against its line", id)
		}
		out.t1 = 1
		out.at = func(t float64) (geom.Vec3, geom.Vec3) { return start.Add(span.Scale(t)), span }
	case "CIRCLE", "ELLIPSE":
		var f geom.Frame
		var major, minor float64
		if kind == "CIRCLE" {
			c := r.record(curve, "CIRCLE", 3).Args
			f, major = r.frame(c[1].Ref), r.number(c[2])
			minor = major
		} else {
			c := r.record(curve, "ELLIPSE", 4).Args
			f, major, minor = r.frame(c[1].Ref), r.number(c[2]), r.number(c[3])
		}
		if !(major > 0 && minor > 0) {
			r.fail("edge #%d: radii %g and %g", id, major, minor)
			break
		}
		at := func(t float64) (geom.Vec3, geom.Vec3) {
			s, c := math.Sincos(t)
			return f.ToWorld(geom.Vec3{X: major * c, Y: minor * s}), f.XAxis().Scale(-major * s).Add(f.YAxis().Scale(minor * c))
		}
		angle := func(p geom.Vec3) float64 {
			q := f.ToLocal(p)
			t := math.Atan2(q.Y/minor, q.X/major)
			if on, _ := at(t); !r.near(on.Distance(p)) {
				r.fail("edge #%d: a vertex is off its conic", id)
			}
			return t
		}
		out.t0, out.t1 = angle(start), angle(end)
		turn := 2 * math.Pi
		if !same {
			turn = -turn
		}
		if out.from == out.to {
			out.t1 = out.t0 + turn
		} else if (out.t1-out.t0)*turn <= 0 {
			out.t1 += turn
		}
		out.at, out.panels = at, 64
		// The box of an arc reaches past its ends where a coordinate turns.
		for _, axis := range [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
			t := math.Atan2(minor*f.YAxis().Dot(axis), major*f.XAxis().Dot(axis))
			lo, hi := math.Min(out.t0, out.t1), math.Max(out.t0, out.t1)
			for t += math.Pi * math.Ceil((lo-t)/math.Pi); t <= hi; t += math.Pi {
				p, _ := at(t)
				r.extent = append(r.extent, p)
			}
		}
	default:
		r.fail("edge #%d has a curve of kind %q", id, kind)
	}
	if out.at == nil {
		out.at = func(float64) (geom.Vec3, geom.Vec3) { return geom.Vec3{}, geom.Vec3{} }
	}
	r.paths[id] = out
	return out
}

// Five-point Gauss-Legendre nodes and weights on [-1, 1].
var gauss = [5][2]float64{
	{0, 128. / 225},
	{-math.Sqrt(5-2*math.Sqrt(10./7)) / 3, (322 + 13*math.Sqrt(70)) / 900},
	{math.Sqrt(5-2*math.Sqrt(10./7)) / 3, (322 + 13*math.Sqrt(70)) / 900},
	{-math.Sqrt(5+2*math.Sqrt(10./7)) / 3, (322 - 13*math.Sqrt(70)) / 900},
	{math.Sqrt(5+2*math.Sqrt(10./7)) / 3, (322 - 13*math.Sqrt(70)) / 900},
}

// along integrates g(point, velocity) over an edge from its start to its end.
func (p path) along(g func(p, d geom.Vec3) float64) float64 {
	sum, h := 0., (p.t1-p.t0)/float64(p.panels)
	for k := 0; k < p.panels; k++ {
		mid := p.t0 + (float64(k)+.5)*h
		for _, n := range gauss {
			q, d := p.at(mid + n[0]*h/2)
			sum += n[1] * g(q, d) * h / 2
		}
	}
	return sum
}

// face gives the flux of x through an ADVANCED_FACE along its normal, and
// its area. The normal of a face is the normal of its surface when
// same_sense is true and the opposite otherwise, and its bounds, each taken
// in the sense its own flag gives, run counterclockwise about that normal
// with the face on their left.
func (r *reader) face(id int64) (flux, area float64) {
	a := r.record(id, "ADVANCED_FACE", 4).Args
	same := r.flag(a[3])
	sign := 1.
	if !same {
		sign = -1
	}
	var f geom.Frame
	var radius float64
	cylinder := false
	switch kind := r.kind(a[2].Ref); kind {
	case "PLANE":
		f = r.frame(r.record(a[2].Ref, "PLANE", 2).Args[1].Ref)
	case "CYLINDRICAL_SURFACE":
		s := r.record(a[2].Ref, "CYLINDRICAL_SURFACE", 3).Args
		f, radius, cylinder = r.frame(s[1].Ref), r.number(s[2]), true
	default:
		r.fail("face #%d has a surface of kind %q", id, kind)
		return 0, 0
	}
	// The integrands, as functions of a point and a velocity in the frame
	// of the surface. On a plane the area is half the circulation of
	// p × dp about the face normal, and x·n is the height of the plane
	// along that normal, so the two signs cancel in the flux. On a
	// cylinder, with the angle θ about the axis and the height z, the area
	// of a region is −r∮z dθ counterclockwise about the outward normal, and
	// x·n at angle θ is r plus the part of the frame origin along the
	// normal there; the flux is −r∮z(x·n) dθ around the bounds as the face
	// uses them, whichever way the face looks.
	origin := f.Origin()
	ox, oy, oz := origin.Dot(f.XAxis()), origin.Dot(f.YAxis()), origin.Dot(f.ZAxis())
	areaOf := func(p, d geom.Vec3) float64 { return sign * (p.X*d.Y - p.Y*d.X) / 2 }
	fluxOf := func(p, d geom.Vec3) float64 { return oz * (p.X*d.Y - p.Y*d.X) / 2 }
	if cylinder {
		turn := func(p, d geom.Vec3) float64 { return (p.X*d.Y - p.Y*d.X) / (p.X*p.X + p.Y*p.Y) }
		areaOf = func(p, d geom.Vec3) float64 { return -sign * radius * p.Z * turn(p, d) }
		fluxOf = func(p, d geom.Vec3) float64 {
			rho := math.Hypot(p.X, p.Y)
			return -radius * p.Z * (radius + ox*p.X/rho + oy*p.Y/rho) * turn(p, d)
		}
	}
	on := func(p geom.Vec3) bool {
		q := f.ToLocal(p)
		if cylinder {
			return r.near(math.Abs(math.Hypot(q.X, q.Y) - radius))
		}
		return r.near(math.Abs(q.Z))
	}
	for _, bound := range a[1].Items {
		kind := r.kind(bound.Ref)
		if kind != "FACE_BOUND" && kind != "FACE_OUTER_BOUND" {
			r.fail("face #%d has a bound of kind %q", id, kind)
			return 0, 0
		}
		b := r.record(bound.Ref, kind, 3).Args
		forward := r.flag(b[2])
		loop := r.record(b[1].Ref, "EDGE_LOOP", 2).Args[1].Items
		var ends [][2]int64
		for _, item := range loop {
			o := r.record(item.Ref, "ORIENTED_EDGE", 5).Args
			e, with := r.edge(o[3].Ref), r.flag(o[4])
			if !on(r.vertex(e.from)) || !on(r.vertex(e.to)) {
				r.fail("face #%d: edge #%d ends off the surface", id, o[3].Ref)
			}
			mid, _ := e.at((e.t0 + e.t1) / 2)
			if !on(mid) {
				r.fail("face #%d: edge #%d leaves the surface between its ends", id, o[3].Ref)
			}
			direction := 1.
			if with != forward {
				direction = -1
			}
			local := func(g func(p, d geom.Vec3) float64) func(p, d geom.Vec3) float64 {
				return func(p, d geom.Vec3) float64 {
					return g(f.ToLocal(p), geom.Vec3{X: d.Dot(f.XAxis()), Y: d.Dot(f.YAxis()), Z: d.Dot(f.ZAxis())})
				}
			}
			area += direction * e.along(local(areaOf))
			flux += direction * e.along(local(fluxOf))
			r.uses[o[3].Ref] = append(r.uses[o[3].Ref], with == forward)
			if with {
				ends = append(ends, [2]int64{e.from, e.to})
			} else {
				ends = append(ends, [2]int64{e.to, e.from})
			}
		}
		for i, e := range ends {
			if e[1] != ends[(i+1)%len(ends)][0] {
				r.fail("face #%d: a loop does not close", id)
			}
		}
	}
	return flux, area
}

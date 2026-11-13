package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func steppedMeridian(frame geom.Frame, profile []geom.Vec2) (trimmed.SteppedShaft, bool) {
	if len(profile) != 6 {
		return trimmed.SteppedShaft{}, false
	}
	p := append([]geom.Vec2(nil), profile...)
	var radius, shoulder float64
	lo, hi := p[0].Y, p[0].Y
	for _, q := range p {
		lo, hi = min(lo, q.Y), max(hi, q.Y)
		radius = max(radius, q.X)
	}
	shaft := 0.
	for _, q := range p {
		if q.Y == lo {
			shaft = max(shaft, q.X)
		}
		if q.X == radius {
			shoulder = q.Y
		}
	}
	for _, q := range p {
		if q.X == radius {
			shoulder = min(shoulder, q.Y)
		}
	}
	if shaft <= 0 || shaft >= radius || shoulder <= lo || shoulder >= hi {
		return trimmed.SteppedShaft{}, false
	}
	want := [...]geom.Vec2{{Y: lo}, {X: shaft, Y: lo}, {X: shaft, Y: shoulder},
		{X: radius, Y: shoulder}, {X: radius, Y: hi}, {Y: hi}}
	for start, q := range p {
		if q != want[0] {
			continue
		}
		for _, direction := range []int{1, -1} {
			match := true
			for i, q := range want {
				match = match && p[(start+len(p)+direction*i)%len(p)] == q
			}
			if match {
				f, err := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: shoulder}), frame.XAxis(), frame.ZAxis())
				return trimmed.SteppedShaft{Frame: f, ShaftRadius: shaft, HeadRadius: radius, ShaftLength: shoulder - lo, HeadHeight: hi - shoulder}, err == nil
			}
		}
	}
	return trimmed.SteppedShaft{}, false
}

func (b *Bridge) putSteppedShaft(p trimmed.SteppedShaft) (Shape, error) {
	s, err := p.Solid(context.Background())
	if err != nil {
		return Shape{}, err
	}
	a, r, l, h := p.ShaftRadius, p.HeadRadius, p.ShaftLength, p.HeadHeight
	volume := math.Pi * (a*a*l + r*r*h)
	area := 2 * math.Pi * (a*l + r*h + r*r)
	if len(p.Socket) > 0 {
		depth := h - p.SocketFloor
		var section, perimeter float64
		for i, u := range p.Socket {
			v := p.Socket[(i+1)%len(p.Socket)]
			section += u.X*v.Y - u.Y*v.X
			perimeter += math.Hypot(v.X-u.X, v.Y-u.Y)
		}
		volume -= math.Abs(section) * depth / 2
		area += perimeter * depth
	}
	return b.putAnalyticBody(&analyticBody{solid: s, shaft: &p, volume: volume, area: area})
}

func (b *Bridge) cutShaftSocket(p trimmed.SteppedShaft, s brep.Solid) (Shape, error) {
	reject := func() (Shape, error) {
		return Shape{}, &failureError{"UNSUPPORTED", "shaft socket requires a simple axial prism entering only the top of the head"}
	}
	if len(p.Socket) != 0 || !cardinalFrame(p.Frame) {
		return reject()
	}
	vertices, faces, edges := s.Vertices(), s.Faces(), s.Edges()
	if len(vertices) < 6 || len(vertices) > 128 {
		return reject()
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	local := make([]geom.Vec3, len(vertices))
	for i, v := range vertices {
		q := p.Frame.ToLocal(v)
		if p.Frame.ToWorld(q) != v {
			return reject()
		}
		local[i] = q
		lo, hi = min(lo, q.Z), max(hi, q.Z)
	}
	if lo <= 0 || lo >= p.HeadHeight || hi < p.HeadHeight {
		return reject()
	}
	for _, f := range faces {
		if len(f.Holes) != 0 {
			return reject()
		}
		ring := make([]geom.Vec2, len(f.Boundary))
		cap := true
		for i, u := range f.Boundary {
			e := edges[u.Edge]
			id := e.Start
			if u.Reversed {
				id = e.End
			}
			q := local[id]
			cap = cap && q.Z == lo
			ring[i] = geom.Vec2{X: q.X, Y: q.Y}
		}
		if cap {
			if p.Socket != nil {
				return reject()
			}
			p.Socket = ring
		}
	}
	if len(p.Socket) < 3 || 2*len(p.Socket) != len(vertices) || len(faces) != len(p.Socket)+2 {
		return reject()
	}
	// Reconstruct the expected prism and compare every vertex and directed
	// face wire. A matching bounding box alone cannot qualify the tool.
	points := make([]geom.Vec3, len(p.Socket))
	for i, v := range p.Socket {
		points[i] = p.Frame.ToWorld(geom.Vec3{X: v.X, Y: v.Y, Z: lo})
	}
	want, err := brep.Prism(points, p.Frame.ZAxis().Scale(hi-lo))
	if err != nil || !samePrismFaces(s, want) {
		return reject()
	}
	p.SocketFloor = lo
	return b.putSteppedShaft(p)
}

func samePrismFaces(a, b brep.Solid) bool {
	wires := func(s brep.Solid) [][]geom.Vec3 {
		faces, vertices, edges := s.Faces(), s.Vertices(), s.Edges()
		r := make([][]geom.Vec3, len(faces))
		for i, f := range faces {
			r[i] = make([]geom.Vec3, len(f.Boundary))
			for j, u := range f.Boundary {
				e := edges[u.Edge]
				v := e.Start
				if u.Reversed {
					v = e.End
				}
				r[i][j] = vertices[v]
			}
		}
		return r
	}
	x, y := wires(a), wires(b)
	if len(x) != len(y) {
		return false
	}
	seen := make([]bool, len(y))
	for _, w := range x {
		found := false
		for j, v := range y {
			if seen[j] || len(w) != len(v) {
				continue
			}
			for start, p := range v {
				if w[0] != p {
					continue
				}
				match := true
				for i, q := range w {
					match = match && q == v[(start+i)%len(v)]
				}
				if match {
					seen[j], found = true, true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (p *analyticBody) shaftRecipe() (trimmed.SteppedShaft, error) {
	if p == nil || p.shaft == nil {
		return trimmed.SteppedShaft{}, errors.New("operation requires retained stepped shaft")
	}
	return *p.shaft, nil
}

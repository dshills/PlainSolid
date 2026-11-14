package plaincadbridge

import (
	"context"
	"fmt"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// finishBoredEdges preserves the qualified cavity graph while replacing only
// its six rectangular exterior faces. Strict footprint clearance proves the
// treatments and every cavity are disjoint, including blind floors.
func (b *Bridge) finishBoredEdges(original *boreBody, body *edgeBody) (Shape, error) {
	fail := func() (Shape, error) {
		return Shape{}, unsupportedEdge("retained cavities must clear both treated cap strips")
	}
	if !cardinalFrame(body.frame) {
		return fail()
	}
	base, err := brep.Box(geom.Vec3{}, geom.Vec3{X: body.length, Y: body.width, Z: body.height})
	if err != nil {
		return Shape{}, err
	}
	var tools []*cylinderBody
	for _, c := range original.cylinders {
		o := body.frame.ToLocal(c.frame.Origin())
		if body.frame.ToWorld(o) != c.frame.Origin() {
			return fail()
		}
		localVector := func(v geom.Vec3) geom.Vec3 {
			return geom.Vec3{X: v.Dot(body.frame.XAxis()), Y: v.Dot(body.frame.YAxis()), Z: v.Dot(body.frame.ZAxis())}
		}
		z := localVector(c.frame.ZAxis())
		if z != (geom.Vec3{Z: 1}) && z != (geom.Vec3{Z: -1}) {
			return fail()
		}
		st := body.chain.Stock
		if o.X-c.radius <= 0 || o.X+c.radius >= body.length || o.Y-c.radius <= st.TopMinY || o.Y+c.radius >= st.TopMaxY || o.Y-c.radius <= st.BottomMinY || o.Y+c.radius >= st.BottomMaxY {
			return fail()
		}
		f, e := geom.NewFrame(o, localVector(c.frame.XAxis()), z)
		if e != nil {
			return Shape{}, e
		}
		tool, e := newCylinder(f, c.radius, c.height)
		if e != nil {
			return Shape{}, e
		}
		tools = append(tools, tool)
	}
	donor, err := newBore(base, tools)
	if err != nil {
		return Shape{}, err
	}
	vertices := body.chain.Solid.Vertices()
	edges := body.chain.Solid.Edges()
	faces := body.chain.Solid.Faces()
	dv, de, df := donor.solid.Vertices(), donor.solid.Edges(), donor.solid.Faces()
	if len(df) < 6 {
		return fail()
	}
	vo, eo := len(vertices), len(edges)
	vertices = append(vertices, dv...)
	for _, e := range de {
		e.Start += vo
		e.End += vo
		edges = append(edges, e)
	}
	remap := func(loop []trimmed.Use) []trimmed.Use {
		out := append([]trimmed.Use(nil), loop...)
		for i := range out {
			out[i].Edge += eo
		}
		return out
	}
	for cap, id := range []int{body.chain.Stock.BottomFace, body.chain.Stock.TopFace} {
		plane, ok := df[cap].Surface.(geom.PlaneSurface)
		if !ok {
			return fail()
		}
		z := float64(cap) * body.height
		if plane.Frame().Origin().Z != z {
			return fail()
		}
		f := faces[id]
		f.Surface = df[cap].Surface
		f.Reversed = df[cap].Reversed
		for i := range f.Outer {
			u := &f.Outer[i]
			e := edges[u.Edge]
			a, c := e.Start, e.End
			if u.Reversed {
				a, c = c, a
			}
			p, q := plane.Frame().ToLocal(vertices[a]), plane.Frame().ToLocal(vertices[c])
			if p.Z != 0 || q.Z != 0 {
				return fail()
			}
			u.UVStart = geom.Vec2{X: p.X, Y: p.Y}
			u.UVEnd = geom.Vec2{X: q.X, Y: q.Y}
			u.Curve = nil
		}
		// Orient the outer loop to this plane's positive chart, retaining its
		// physical shell direction through the donor face's Reversed flag.
		area := 0.0
		for _, u := range f.Outer {
			area += u.UVStart.X*u.UVEnd.Y - u.UVStart.Y*u.UVEnd.X
		}
		if area < 0 {
			for i, j := 0, len(f.Outer)-1; i < j; i, j = i+1, j-1 {
				f.Outer[i], f.Outer[j] = f.Outer[j], f.Outer[i]
			}
			for i := range f.Outer {
				u := &f.Outer[i]
				u.Reversed = !u.Reversed
				u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			}
		}
		f.Holes = nil
		for _, h := range df[cap].Holes {
			f.Holes = append(f.Holes, remap(h))
		}
		faces[id] = f
	}
	for _, f := range df[6:] {
		f.Outer = remap(f.Outer)
		for i := range f.Holes {
			f.Holes[i] = remap(f.Holes[i])
		}
		faces = append(faces, f)
	}
	em, vm := map[int]int{}, map[int]int{}
	var vs []geom.Vec3
	var es []parametric.Edge
	vertex := func(id int) int {
		if n, ok := vm[id]; ok {
			return n
		}
		n := len(vs)
		vm[id] = n
		vs = append(vs, vertices[id])
		return n
	}
	for i := range faces {
		for _, loop := range append([][]trimmed.Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range loop {
				id := loop[j].Edge
				n, ok := em[id]
				if !ok {
					n = len(es)
					em[id] = n
					e := edges[id]
					e.Start = vertex(e.Start)
					e.End = vertex(e.End)
					es = append(es, e)
				}
				loop[j].Edge = n
			}
		}
	}
	solid, err := trimmed.NewSolidContext(context.Background(), vs, es, faces, 1e-7)
	if err != nil {
		return Shape{}, fmt.Errorf("retained cavity graft: %w", err)
	}
	solid, err = solid.Transformed(frameMap(body.frame))
	if err != nil {
		return Shape{}, err
	}
	v, a, err := original.measures()
	if err != nil {
		return Shape{}, err
	}
	v += body.chain.NominalVolume - body.length*body.width*body.height
	a += body.chain.NominalSurfaceArea - 2*(body.length*body.width+body.length*body.height+body.width*body.height)
	bounds, err := frameMap(body.frame).ApplyBox(geom.Box{Max: geom.Vec3{X: body.length, Y: body.width, Z: body.height}})
	if err != nil {
		return Shape{}, err
	}
	return b.publish(stored{analytic: &analyticBody{solid: solid, volume: v, area: a}, result: Shape{GeometryAssertions: Assertions{true, v, a, 1}, Bounds: Bounds{coordinates(bounds.Min), coordinates(bounds.Max)}}})
}

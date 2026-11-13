package prism

import (
	"context"
	"fmt"

	"plainsolid/brep"
	"plainsolid/geom"
)

// Solids converts two planar vertical prisms to profiles, evaluates their
// Boolean, and places the results at the shared base elevation. The operands
// must have matching minimum/maximum Z, one holed or simple polygonal cap at
// each end, and one vertical side face per cap edge. The source graph is
// checked against a canonical extrusion before Boolean evaluation.
func Solids(ctx context.Context, a, b brep.Solid, op Operation, options Options) ([]brep.Solid, error) {
	p, lo, hi, e := extract(a)
	if e != nil {
		return nil, e
	}
	q, lo2, hi2, e := extract(b)
	if e != nil {
		return nil, e
	}
	if lo != lo2 || hi != hi2 {
		return nil, fmt.Errorf("prism: mismatched cap planes")
	}
	out, e := Boolean(ctx, p, q, hi-lo, op, options)
	if e != nil {
		return nil, e
	}
	if lo == 0 {
		return out, nil
	}
	for i, v := range out {
		r, e := v.Transformed(geom.Translation(geom.Vec3{Z: lo}))
		if e != nil {
			return nil, e
		}
		out[i] = r
	}
	return out, nil
}

type pointEdge struct{ a, b geom.Vec3 }

func edgeKey(a, b geom.Vec3) pointEdge {
	if a.X > b.X || a.X == b.X && (a.Y > b.Y || a.Y == b.Y && a.Z > b.Z) {
		a, b = b, a
	}
	return pointEdge{a, b}
}

func extract(s brep.Solid) (Profile, float64, float64, error) {
	var p Profile
	b, e := s.Bounds()
	if e != nil {
		return p, 0, 0, e
	}
	lo, hi := b.Min.Z, b.Max.Z
	if lo >= hi || !finite(hi-lo) {
		return p, 0, 0, fmt.Errorf("prism: invalid extrusion height")
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	caps := [2]int{-1, -1}
	for i, f := range faces {
		at := v[edges[f.Boundary[0].Edge].Start].Z
		flat := true
		for _, u := range f.Boundary {
			x := edges[u.Edge]
			if v[x.Start].Z != at || v[x.End].Z != at {
				flat = false
				break
			}
		}
		if flat {
			k := -1
			if at == lo {
				k = 0
			} else if at == hi {
				k = 1
			}
			if k < 0 || caps[k] >= 0 {
				return p, 0, 0, fmt.Errorf("prism: expected a single face per cap")
			}
			caps[k] = i
		} else if len(f.Boundary) != 4 || len(f.Holes) != 0 {
			return p, 0, 0, fmt.Errorf("prism: expected quadrilateral side faces")
		}
	}
	if caps[0] < 0 || caps[1] < 0 {
		return p, 0, 0, fmt.Errorf("prism: missing cap")
	}
	loop := func(uses []brep.EdgeUse) []geom.Vec2 {
		r := make([]geom.Vec2, len(uses))
		for i, u := range uses {
			e := edges[u.Edge]
			j := e.Start
			if u.Reversed {
				j = e.End
			}
			r[i] = geom.Vec2{X: v[j].X, Y: v[j].Y}
		}
		return r
	}
	bottom := faces[caps[0]]
	p.Outer = loop(bottom.Boundary)
	for _, u := range bottom.Holes {
		p.Holes = append(p.Holes, loop(u))
	}
	var x [][]geom.Vec3
	for _, u := range append([][]geom.Vec2{p.Outer}, p.Holes...) {
		var r []geom.Vec3
		for _, w := range u {
			r = append(r, geom.Vec3{X: w.X, Y: w.Y, Z: lo})
		}
		x = append(x, r)
	}
	expected, e := brep.PrismWithHoles(x[0], x[1:], geom.Vec3{Z: hi - lo})
	if e != nil {
		return Profile{}, 0, 0, e
	}
	if len(v) != len(expected.Vertices()) || len(edges) != len(expected.Edges()) || len(faces) != len(expected.Faces()) {
		return Profile{}, 0, 0, fmt.Errorf("prism: source topology differs from vertical extrusion")
	}
	// Compare each oriented face by its edge set and normal. This rules out
	// displaced top vertices, twisted side walls and inverted shells.
	// Vertex Z of the generated top may round on lo+(hi-lo); use the two
	// original elevation symbols when comparing the edge sets.
	expectedVertices := expected.Vertices()
	top := len(expectedVertices) / 2
	for i := top; i < len(expectedVertices); i++ {
		expectedVertices[i].Z = hi
	}
	keyFace := func(v []geom.Vec3, e []brep.Edge, f brep.Face) map[pointEdge]int {
		r := map[pointEdge]int{}
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				x := e[u.Edge]
				a, b := v[x.Start], v[x.End]
				if u.Reversed {
					a, b = b, a
				}
				k := edgeKey(a, b)
				d := 1
				if k.a != a {
					d = -1
				}
				r[k] = d
			}
		}
		return r
	}
	ef, ee := expected.Faces(), expected.Edges()
	keys := make([]map[pointEdge]int, len(ef))
	for i, f := range ef {
		keys[i] = keyFace(expectedVertices, ee, f)
	}
	for _, f := range faces {
		k := keyFace(v, edges, f)
		id := -1
		for i, z := range keys {
			if len(z) != len(k) {
				continue
			}
			match := true
			for edge, d := range k {
				if z[edge] != d {
					match = false
					break
				}
			}
			if match {
				id = i
				break
			}
		}
		if id < 0 {
			return Profile{}, 0, 0, fmt.Errorf("prism: source faces do not match oriented extrusion")
		}
		keys[id] = nil
	}
	return p, lo, hi, nil
}

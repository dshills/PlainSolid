package faceclip

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

type ExtrusionOptions struct {
	Sheet      SheetOptions
	Validation collision.LocationOptions
}
type ExtrusionRole uint8

const (
	StartCap ExtrusionRole = iota
	EndCap
	LoopWall
)

// ExtrusionSource aligns with solid faces. Cap faces name a source sheet
// triangle; wall faces name an original loop and its segment i->(i+1)%n.
// Inapplicable indices are -1. Every output face is triangular.
type ExtrusionSource struct {
	Role                         ExtrusionRole
	SheetTriangle, Loop, Segment int
}
type Extrusion struct {
	Solid       brep.Solid
	Sheet       Sheet
	FaceSources []ExtrusionSource
	ExactVolume *big.Rat
}

// ExtrudeRegion constructs a material prism from an exact planar region. Every
// original rational vertex AND translated endpoint must be exactly representable
// as binary64. Unlike TriangulateSheet it never permits coordinate displacement.
// Holes and disconnected components are retained. Offset may be oblique but must
// have a nonzero exact component normal to the support. Input loop order sets
// ancestry; outward material orientation is constructed for either offset sign.
func ExtrudeRegion(ctx context.Context, b Boundary, offset geom.Vec3, o ExtrusionOptions) (Extrusion, error) {
	empty := Extrusion{}
	if !offset.IsFinite() {
		return empty, ErrInvalid
	}
	sheet, e := TriangulateSheet(ctx, b, o.Sheet)
	if e != nil {
		return empty, e
	}
	result := Extrusion{Sheet: sheet, ExactVolume: new(big.Rat)}
	if len(sheet.Mesh.Vertices) == 0 {
		return result, nil
	}
	if sheet.MaxSquaredDisplacement.Sign() != 0 {
		return empty, ErrConversion
	}
	vertices := append([]geom.Vec3(nil), sheet.Mesh.Vertices...)
	delta := vec(offset)
	count := len(vertices)
	for _, p := range sheet.ExactVertices {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		var exactEnd Point
		var xyz [3]float64
		for k := range p {
			exactEnd[k] = add(p[k], delta[k])
			xyz[k], _ = exactEnd[k].Float64()
		}
		v := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		if !v.IsFinite() || !same(vec(v), exactEnd) {
			return empty, ErrConversion
		}
		vertices = append(vertices, v)
	}
	t := sheet.Mesh.Triangles[0]
	normal := cross(diff(sheet.ExactVertices[t[1]], sheet.ExactVertices[t[0]]), diff(sheet.ExactVertices[t[2]], sheet.ExactVertices[t[0]]))
	advance := dot(normal, delta)
	if advance.Sign() == 0 {
		return empty, ErrInvalid
	}
	positive := advance.Sign() > 0
	limit := o.Sheet.MaxTriangles
	if limit == 0 {
		limit = 16384
	}
	var triangles [][3]int
	appendFace := func(t [3]int, src ExtrusionSource) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(triangles) >= limit {
			return ErrLimit
		}
		triangles = append(triangles, t)
		result.FaceSources = append(result.FaceSources, src)
		return nil
	}
	for i, t := range sheet.Mesh.Triangles {
		bottom, top := t, [3]int{t[0] + count, t[1] + count, t[2] + count}
		if positive {
			bottom[1], bottom[2] = bottom[2], bottom[1]
		} else {
			top[1], top[2] = top[2], top[1]
		}
		if e := appendFace(bottom, ExtrusionSource{StartCap, i, -1, -1}); e != nil {
			return empty, e
		}
		if e := appendFace(top, ExtrusionSource{EndCap, i, -1, -1}); e != nil {
			return empty, e
		}
	}
	for i, loop := range sheet.LoopVertices {
		for j, a := range loop {
			c := loop[(j+1)%len(loop)]
			pair := [2][3]int{{a, c, c + count}, {a, c + count, a + count}}
			for _, t := range pair {
				if !positive {
					t[1], t[2] = t[2], t[1]
				}
				if e := appendFace(t, ExtrusionSource{LoopWall, -1, i, j}); e != nil {
					return empty, e
				}
			}
		}
	}
	var edges []brep.Edge
	faces := make([]brep.Face, len(triangles))
	lookup := map[[2]int]int{}
	for i, t := range triangles {
		for j, a := range t {
			c := t[(j+1)%3]
			reverse := a > c
			if reverse {
				a, c = c, a
			}
			key := [2]int{a, c}
			id, ok := lookup[key]
			if !ok {
				id = len(edges)
				lookup[key] = id
				edges = append(edges, brep.Edge{Start: a, End: c})
			}
			faces[i].Boundary = append(faces[i].Boundary, brep.EdgeUse{Edge: id, Reversed: reverse})
		}
	}
	solid, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, ErrConversion
	}
	if _, e = collision.NewPointLocator(ctx, solid, o.Validation); e != nil {
		return empty, e
	}
	result.Solid = solid
	result.ExactVolume.Mul(b.ProjectedArea, new(big.Rat).Abs(div(advance, normal[b.Axis])))
	return result, nil
}

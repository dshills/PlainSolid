package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PlacedSolid is an immutable trimmed boundary and its affine placement.
type PlacedSolid struct {
	Solid     Solid
	Transform geom.Affine
}

// AssembleShells combines independently placed boundaries without welding.
// A caller using this as a material solid must establish their embedding,
// orientation and nesting. MaxFaces bounds the aggregate face count.
func AssembleShells(ctx context.Context, parts []PlacedSolid, maxFaces int) (Solid, error) {
	if ctx == nil || maxFaces < 1 || len(parts) > 10000 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	var v []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	tol := 0.
	for _, x := range parts {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if len(x.Solid.faces) == 0 {
			return Solid{}, fmt.Errorf("trimmed: empty component")
		}
		if len(x.Solid.faces) > maxFaces-len(faces) {
			return Solid{}, parametric.ErrBudget
		}
		t := x.Solid
		if x.Transform != geom.Identity() {
			var e error
			t, e = x.Solid.Transformed(x.Transform)
			if e != nil {
				return Solid{}, e
			}
		}
		nv, ne := len(v), len(edges)
		v = append(v, t.vertices...)
		for _, e := range t.edges {
			e.Start += nv
			e.End += nv
			edges = append(edges, e)
		}
		for _, f := range t.Faces() {
			for _, l := range append([][]Use{f.Outer}, f.Holes...) {
				for i := range l {
					l[i].Edge += ne
				}
			}
			faces = append(faces, f)
		}
		tol = max(tol, t.tolerance)
	}
	return NewSolidContext(ctx, v, edges, faces, tol)
}

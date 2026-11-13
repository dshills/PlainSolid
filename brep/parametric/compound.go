package parametric

import (
	"context"
	"fmt"

	"plainsolid/geom"
)

// Compound groups independently placed exact solids without fusing them or
// asserting that their volumes are disjoint. Child IDs are unique and stable.
type Compound struct{ children []Component }

// Component instances one solid with a local-to-compound transform.
type Component struct {
	ID        string
	Solid     Solid
	Transform geom.Affine
}

func NewCompound(children []Component) (Compound, error) {
	if len(children) > 10000 {
		return Compound{}, fmt.Errorf("parametric: compound component limit exceeded")
	}
	seen := map[string]bool{}
	for i, c := range children {
		if c.ID == "" || seen[c.ID] || len(c.Solid.faces) == 0 {
			return Compound{}, fmt.Errorf("parametric: invalid compound component %d", i)
		}
		if _, e := c.Transform.Inverse(); e != nil {
			return Compound{}, e
		}
		seen[c.ID] = true
	}
	return Compound{append([]Component(nil), children...)}, nil
}

func (c Compound) Components() []Component { return append([]Component(nil), c.children...) }

// Bounds encloses each placed solid. Empty compounds have no bounds.
func (c Compound) Bounds() (geom.Box, error) {
	if len(c.children) == 0 {
		return geom.Box{}, geom.ErrDegenerate
	}
	var result geom.Box
	for i, x := range c.children {
		p, e := x.Solid.Transformed(x.Transform)
		if e != nil {
			return geom.Box{}, e
		}
		b, e := p.Bounds()
		if e != nil {
			return geom.Box{}, e
		}
		if i == 0 {
			result = b
		} else {
			result, e = result.Union(b)
			if e != nil {
				return geom.Box{}, e
			}
		}
	}
	return result, nil
}

// Boundary assembles the placed child faces as independent shells with
// remapped topology. It does not check inter-component contact/overlap.
// It may only be used as a material solid when the caller has established
// that components form a valid non-overlapping oriented boundary.
func (c Compound) Boundary(ctx context.Context, maxFaces int) (Solid, error) {
	if ctx == nil || maxFaces < 1 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	var v []geom.Vec3
	var edges []Edge
	var faces []Face
	tolerance := 0.
	for _, x := range c.children {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if len(x.Solid.faces) > maxFaces-len(faces) {
			return Solid{}, ErrBudget
		}
		p, e := x.Solid.Transformed(x.Transform)
		if e != nil {
			return Solid{}, e
		}
		nv, ne := len(v), len(edges)
		v = append(v, p.vertices...)
		for _, x := range p.edges {
			x.Start += nv
			x.End += nv
			edges = append(edges, x)
		}
		for _, f := range p.faces {
			for i := range f.Boundary {
				f.Boundary[i].Edge += ne
			}
			faces = append(faces, f)
		}
		tolerance = max(tolerance, p.tolerance)
	}
	return NewSolid(v, edges, faces, tolerance)
}

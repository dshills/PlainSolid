package document

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s State) wrappedRimFilletSTEPBody(id string) (step.WrappedRimFilletBody, error) {
	node := s.Nodes[id]
	solid, ok := s.Trimmed[node.Asset]
	if !ok {
		return step.WrappedRimFilletBody{}, fmt.Errorf("document: rim fillet requires trimmed asset")
	}
	name := node.Name
	if name == "" {
		name = id
	}
	m, e := step.WrappedRimFilletFromSolid(context.Background(), solid, name, s.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		return m, e
	}
	label := s.Labels["step:"+node.Asset]
	get := func(k string) string {
		a := label.Attributes[k]
		if a.Kind == "string" {
			return a.Text
		}
		return ""
	}
	m.CapNames = [2]string{get("step.cap.lower.name"), get("step.cap.upper.name")}
	for i := range m.Walls {
		m.Walls[i].Name = get(fmt.Sprintf("step.wall.%d.name", i))
	}
	for current, depth := id, 0; current != ""; depth++ {
		if depth >= 256 {
			return step.WrappedRimFilletBody{}, geom.ErrInvalid
		}
		n := s.Nodes[current]
		a := n.Transform
		identity := geom.Identity()
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				if a[r][c] != identity[r][c] {
					return step.WrappedRimFilletBody{}, fmt.Errorf("document: standalone fillet placement must be translation")
				}
			}
		}
		v := [3]float64{m.Center.X, m.Center.Y, m.Center.Z}
		for i := range v {
			v[i], e = geom.RepresentedExactScalar(new(big.Rat).Add(new(big.Rat).SetFloat64(v[i]), new(big.Rat).SetFloat64(a[i][3])))
			if e != nil {
				return step.WrappedRimFilletBody{}, e
			}
		}
		m.Center = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
		for i := range m.Walls {
			m.Walls[i].Geometry, _, e = m.Walls[i].Geometry.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
			if e != nil {
				return step.WrappedRimFilletBody{}, e
			}
		}
		current = n.Parent
	}
	return m, nil
}

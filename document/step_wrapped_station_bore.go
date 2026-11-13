package document

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s State) wrappedStationBoreSTEPBody(id string) (step.WrappedStationBoreBody, error) {
	return s.wrappedAxialBoreSTEPBody(id, false)
}
func (s State) wrappedAnnularFrustumSTEPBody(id string) (step.WrappedAnnularFrustumBody, error) {
	m, e := s.wrappedAxialBoreSTEPBody(id, true)
	return step.WrappedAnnularFrustumBody(m), e
}
func (s State) wrappedAxialBoreSTEPBody(id string, single bool) (step.WrappedStationBoreBody, error) {
	node := s.Nodes[id]
	solid, ok := s.Trimmed[node.Asset]
	if !ok {
		return step.WrappedStationBoreBody{}, geom.ErrInvalid
	}
	name := node.Name
	if name == "" {
		name = id
	}
	var m step.WrappedStationBoreBody
	var e error
	if single {
		var q step.WrappedAnnularFrustumBody
		q, e = step.WrappedAnnularFrustumFromSolid(context.Background(), solid, name, s.LengthUnit, geom.SurfacePatchOptions{})
		m = step.WrappedStationBoreBody(q)
	} else {
		m, e = step.WrappedStationBoreFromSolid(context.Background(), solid, name, s.LengthUnit, geom.SurfacePatchOptions{})
	}
	if e != nil {
		return m, e
	}
	attrs := s.Labels["step:"+node.Asset].Attributes
	get := func(k string) string {
		v := attrs[k]
		if v.Kind == "string" {
			return v.Text
		}
		return ""
	}
	m.CapNames = [2]string{get("step.cap.lower.name"), get("step.cap.upper.name")}
	for side, body := range []*step.WrappedStationLoftBody{&m.Outer, &m.Inner} {
		tag := "outer"
		if side == 1 {
			tag = "inner"
		}
		for i := range body.Walls {
			body.Walls[i].Name = get(fmt.Sprintf("step.%s.wall.%d.name", tag, i))
		}
	}
	add := func(a, b float64) (float64, error) {
		return geom.RepresentedExactScalar(new(big.Rat).Add(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)))
	}
	for current, depth := id, 0; current != ""; depth++ {
		if depth >= 256 {
			return m, geom.ErrInvalid
		}
		n := s.Nodes[current]
		a := n.Transform
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				if a[r][c] != geom.Identity()[r][c] {
					return m, fmt.Errorf("document: standalone station bore placement must be translation")
				}
			}
		}
		for _, body := range []*step.WrappedStationLoftBody{&m.Outer, &m.Inner} {
			body.Center.X, e = add(body.Center.X, a[0][3])
			if e != nil {
				return m, e
			}
			body.Center.Y, e = add(body.Center.Y, a[1][3])
			if e != nil {
				return m, e
			}
			for i := range body.Stations {
				body.Stations[i].Z, e = add(body.Stations[i].Z, a[2][3])
				if e != nil {
					return m, e
				}
			}
			for i := range body.Walls {
				body.Walls[i].Geometry, _, e = body.Walls[i].Geometry.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
				if e != nil {
					return m, e
				}
			}
		}
		current = n.Parent
	}
	return m, nil
}

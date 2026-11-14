package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func (b *Bridge) treatCylinderRims(s stored, q request) (Shape, error) {
	size := float64(q.Radius)
	chamfer := q.Op == "chamferCylinderRims"
	if chamfer {
		size = float64(q.Distance)
	}
	if s.cylinder == nil || len(q.RimCenters) == 0 || len(q.RimCenters) > 2 || q.Normal == nil || size <= 0 {
		return Shape{}, unsupportedEdge("circular rim treatment requires one or two original cylinder caps")
	}
	c := s.cylinder
	n, err := q.Normal.vec().Unit()
	if err != nil || math.Abs(math.Abs(n.Dot(c.frame.ZAxis()))-1) > 1e-9 {
		return Shape{}, errors.New("circular rim axis no longer matches its original cylinder")
	}
	var lower, upper float64
	start := c.frame.Origin()
	end := start.Add(c.frame.ZAxis().Scale(c.height))
	for _, center := range q.RimCenters {
		switch {
		case nearPoint(center.vec(), start):
			lower = size
		case nearPoint(center.vec(), end):
			upper = size
		default:
			return Shape{}, unsupportedEdge("circular rim was lost or is ambiguous")
		}
	}
	var p parametric.Solid
	if chamfer {
		p, err = chamferedCylinder(c, lower, upper)
	} else {
		p, err = parametric.RoundedCylinder(c.frame, c.radius, c.height, lower, upper)
	}
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(p)
	if err != nil {
		return Shape{}, err
	}
	body.rimPatches = true
	body.rimTreatment = &cylinderRimTreatment{frame: c.frame, radius: c.radius, height: c.height, lower: lower, upper: upper, chamfer: chamfer}
	return b.putAnalyticBody(body)
}

func chamferedCylinder(c *cylinderBody, lower, upper float64) (parametric.Solid, error) {
	if lower+upper >= c.height || max(lower, upper) >= c.radius {
		return parametric.Solid{}, geom.ErrDomain
	}
	profile := []geom.Vec2{{}, {X: c.radius - lower}}
	if lower > 0 {
		profile = append(profile, geom.Vec2{X: c.radius, Y: lower})
	}
	profile = append(profile, geom.Vec2{X: c.radius, Y: c.height - upper})
	if upper > 0 {
		profile = append(profile, geom.Vec2{X: c.radius - upper, Y: c.height})
	}
	return parametric.Revolve(c.frame, append(profile, geom.Vec2{Y: c.height}))
}

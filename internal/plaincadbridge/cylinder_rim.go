package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep/parametric"
)

func (b *Bridge) filletCylinderRims(s stored, q request) (Shape, error) {
	if s.cylinder == nil || len(q.RimCenters) == 0 || len(q.RimCenters) > 2 || q.Normal == nil || q.Radius <= 0 {
		return Shape{}, unsupportedEdge("circular rim fillet requires one or two original cylinder caps")
	}
	c := s.cylinder
	n, err := q.Normal.vec().Unit()
	if err != nil || math.Abs(math.Abs(n.Dot(c.frame.ZAxis()))-1) > 1e-9 {
		return Shape{}, errors.New("circular rim axis no longer matches its original cylinder")
	}
	radius := float64(q.Radius)
	var lower, upper float64
	start := c.frame.Origin()
	end := start.Add(c.frame.ZAxis().Scale(c.height))
	for _, center := range q.RimCenters {
		switch {
		case nearPoint(center.vec(), start):
			lower = radius
		case nearPoint(center.vec(), end):
			upper = radius
		default:
			return Shape{}, unsupportedEdge("circular rim was lost or is ambiguous")
		}
	}
	p, err := parametric.RoundedCylinder(c.frame, c.radius, c.height, lower, upper)
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(p)
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalyticBody(body)
}

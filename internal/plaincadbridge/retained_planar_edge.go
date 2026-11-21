package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) treatRetainedPlanarEdge(s stored, q request, normal geom.Vec3, size float64, pair [2]geom.Vec3) (Shape, error) {
	if !s.planar() {
		return Shape{}, unsupportedEdge("retained edge treatment requires planar stock")
	}
	if pair[1].Sub(pair[0]).Dot(normal) != 0 {
		return Shape{}, unsupportedEdge("retained edge is not in the selected cap")
	}
	// The extrusion normal must be one of the two actual adjacent supports.
	support := false
	for i, f := range s.solid.Faces() {
		p, e := s.solid.FacePlane(i)
		if e != nil {
			return Shape{}, e
		}
		if p.Normal() != normal && p.Normal() != normal.Scale(-1) {
			continue
		}
		for _, u := range f.Boundary {
			ed := s.solid.Edges()[u.Edge]
			v := s.solid.Vertices()
			if v[ed.Start].Sub(pair[0]).Dot(normal) == 0 && v[ed.End].Sub(pair[0]).Dot(normal) == 0 {
				support = true
			}
		}
	}
	if !support {
		return Shape{}, unsupportedEdge("retained edge cap normal is unsupported")
	}
	bevel := q.Op == "chamfer"
	solid, e := trimmed.TreatPlanarOrthogonalEdge(context.Background(), s.solid, pair[0], pair[1], size, bevel, 1e-9)
	if e != nil {
		return Shape{}, unsupportedEdge("retained local edge treatment: " + e.Error())
	}
	length := pair[1].Sub(pair[0]).Length()
	section := size * size * (1 - math.Pi/4)
	wall := size * math.Pi / 2
	if bevel {
		section = size * size / 2
		wall = size * math.Sqrt2
	}
	volume := s.result.GeometryAssertions.Volume - length*section
	area := s.result.GeometryAssertions.SurfaceArea + length*(wall-2*size) - 2*section
	// Every extremum of the stock survives a treatment of one edge.
	body := &analyticBody{solid: solid, volume: volume, area: area}
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 || len(solid.Topology().Shells) != 1 {
		return Shape{}, unsupportedEdge("invalid treated stock measures")
	}
	return b.publish(stored{analytic: body, result: Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: s.result.Bounds}})
}

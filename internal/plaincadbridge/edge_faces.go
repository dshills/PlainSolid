package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (body *edgeBody) planeArea(id int, f trimmed.Face, n geom.Vec3) (float64, error) {
	stock := body.chain.Stock
	if id == stock.EndFaces[0] || id == stock.EndFaces[1] {
		return stock.NominalVolume / body.length, nil
	}
	if id == stock.TopFace || id == stock.BottomFace {
		width := body.width
		if id == stock.TopFace {
			width -= body.radius
			if !body.bottomFront {
				width -= body.chamfer
			}
		} else if body.bottomFront {
			width -= body.chamfer
		}
		area := body.length * width
		if body.bore != nil {
			area -= math.Pi * body.bore.Radius * body.bore.Radius
		}
		if body.pocket != nil {
			area -= (body.pocket.Max.X - body.pocket.Min.X) * (body.pocket.Max.Y - body.pocket.Min.Y)
		}
		if id == stock.TopFace && body.boss != nil {
			r := body.boss.Footprint
			area -= (r.Max.X - r.Min.X) * (r.Max.Y - r.Min.Y)
		}
		return area, nil
	}
	if len(f.Holes) != 0 {
		return 0, unsupportedEdge("unproved planar face area")
	}
	v, e := body.chain.Solid.Vertices(), body.chain.Solid.Edges()
	area := 0.
	for _, u := range f.Outer {
		if u.Curve != nil {
			return 0, unsupportedEdge("unproved curved planar trim area")
		}
		a, b := e[u.Edge].Start, e[u.Edge].End
		if u.Reversed {
			a, b = b, a
		}
		area += v[a].Cross(v[b]).Dot(n) / 2
	}
	return math.Abs(area), nil
}
func (body *edgeBody) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	origin = body.frame.ToLocal(origin)
	n = geom.Vec3{X: n.Dot(body.frame.XAxis()), Y: n.Dot(body.frame.YAxis()), Z: n.Dot(body.frame.ZAxis())}
	found := -1
	var result PlanarFace
	for id, f := range body.chain.Solid.Faces() {
		surface, m := parametric.SurfaceMapping(f.Surface)
		p, ok := surface.(geom.PlaneSurface)
		if !ok {
			continue
		}
		o := m.Apply(p.Frame().Origin())
		normal, err := m.ApplyVector(p.Frame().ZAxis()).Unit()
		if err != nil {
			return PlanarFace{}, err
		}
		if math.Abs(math.Abs(normal.Dot(n))-1) > 1e-9 || math.Abs(o.Sub(origin).Dot(normal)) > 1e-7 {
			continue
		}
		if found >= 0 {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		found = id
		area, err := body.planeArea(id, f, normal)
		if err != nil {
			return PlanarFace{}, err
		}
		if area <= 1e-14 {
			return PlanarFace{}, errors.New("planar face has no measurable area")
		}
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		result = PlanarFace{id, coordinates(body.frame.ToWorld(o)), coordinates(frameMap(body.frame).ApplyVector(normal)), area}
	}
	if found < 0 {
		return PlanarFace{}, errors.New("planar face reference no longer exists")
	}
	return result, nil
}

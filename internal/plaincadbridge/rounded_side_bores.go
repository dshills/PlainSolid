package plaincadbridge

import (
	"context"
	"math"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func roundedSideBoreStock(a stored, c *cylinderBody) bool {
	if a.analytic == nil || a.analytic.rounded == nil || c == nil {
		return false
	}
	p := a.analytic.rounded
	return p.isPlainExtrusion() && cardinalFrame(p.frame) && cardinalFrame(c.frame) &&
		p.frame.ZAxis().Dot(c.frame.ZAxis()) == 0
}

// A finite cutter can be described in either direction. Its physical
// endpoints select the original exterior plane and the interior floor.
func roundedSideBoreSite(stock trimmed.Solid, c *cylinderBody) (trimmed.PlanarSteppedBore, error) {
	axis, start := c.frame.ZAxis(), c.frame.Origin()
	end := c.frame.ToWorld(geom.Vec3{Z: c.height})
	if !exactAxisDistance(end.Dot(axis), start.Dot(axis), c.height) {
		return trimmed.PlanarSteppedBore{}, geom.ErrUnrepresentable
	}
	for i, face := range stock.Faces() {
		plane, ok := face.Surface.(geom.PlaneSurface)
		if !ok || face.Reversed {
			continue
		}
		frame := plane.Frame()
		opening := start
		switch frame.ZAxis() {
		case axis:
			opening = end
		case axis.Scale(-1):
		default:
			continue
		}
		local := frame.ToLocal(opening)
		if local.Z != 0 || exactDotDifference(opening, frame.Origin(), frame.ZAxis()) != 0 {
			continue
		}
		if !exactAxisDistance(opening.Dot(frame.XAxis()), frame.Origin().Dot(frame.XAxis()), local.X) ||
			!exactAxisDistance(opening.Dot(frame.YAxis()), frame.Origin().Dot(frame.YAxis()), local.Y) {
			return trimmed.PlanarSteppedBore{}, geom.ErrUnrepresentable
		}
		return trimmed.PlanarSteppedBore{
			LowerCap: -1, UpperCap: i, Center: geom.Vec2{X: local.X, Y: local.Y},
			Steps: []trimmed.AxialBoreStep{{Radius: c.radius, Depth: c.height}},
		}, nil
	}
	return trimmed.PlanarSteppedBore{}, geom.ErrDomain
}

func overlapRoundedSideBore(a, tool stored) (float64, bool, error) {
	c := tool.cylinder
	if !roundedSideBoreStock(a, c) {
		return 0, false, nil
	}
	stock := a.analytic.solid
	site, err := roundedSideBoreSite(stock, c)
	if err != nil {
		return 0, true, err
	}
	if err := trimmed.ProvePlanarSteppedBore(context.Background(), stock, site.LowerCap, site.UpperCap, site.Center, site.Steps); err != nil {
		return 0, true, err
	}
	return tool.result.GeometryAssertions.Volume, true, nil
}

func (b *Bridge) cutRoundedSideBores(a stored, tools []stored) (Shape, bool, error) {
	for _, tool := range tools {
		if !roundedSideBoreStock(a, tool.cylinder) {
			return Shape{}, false, nil
		}
	}
	result := *a.analytic
	sites := make([]trimmed.PlanarSteppedBore, 0, len(tools))
	for _, tool := range tools {
		c := tool.cylinder
		site, err := roundedSideBoreSite(result.solid, c)
		if err != nil {
			return Shape{}, true, err
		}
		if slices.ContainsFunc(sites, func(prior trimmed.PlanarSteppedBore) bool {
			return prior.UpperCap == site.UpperCap && prior.Center == site.Center && prior.Steps[0] == site.Steps[0]
		}) {
			continue
		}
		sites = append(sites, site)
		result.volume -= tool.result.GeometryAssertions.Volume
		// The circular opening and planar floor have equal area.
		result.area += 2 * math.Pi * c.radius * c.height
	}
	if !removesMaterial(a.analytic.volume, result.volume) {
		return Shape{}, true, unsupportedEdge("rounded side bores removed no volume")
	}
	solid, err := trimmed.SewPlanarSteppedBores(context.Background(), result.solid, sites)
	if err != nil {
		return Shape{}, true, err
	}
	recipe := *result.rounded
	recipe.cut = true
	result.solid, result.rounded = solid, &recipe
	out, err := b.putAnalyticBody(&result)
	return out, true, err
}

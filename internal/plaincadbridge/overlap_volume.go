package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// overlapVolume is a measure-only query. In particular it does not publish a
// fictitious intersection shape when only a qualified material-volume proof is
// available. The ordinary common operation retains its geometry contract.
func (b *Bridge) overlapVolume(aID, cID string) (any, error) {
	a, err := b.get(aID)
	if err != nil {
		return nil, err
	}
	c, err := b.get(cID)
	if err != nil {
		return nil, err
	}
	result := func(volume float64) (any, error) { return map[string]float64{"volume": volume}, nil }
	if boundsSeparated(a.result.Bounds, c.result.Bounds) {
		return result(0)
	}
	// Capture may test a complete planar target against a containing circular
	// tool. This is the inverse of the contained-bore overlap route below.
	if a.cylinder != nil && c.planar() || c.cylinder != nil && a.planar() {
		cylinder, planar := a.cylinder, c.solid
		if cylinder == nil {
			cylinder, planar = c.cylinder, a.solid
		}
		volume, proven, err := containedPlanarCylinderVolume(cylinder, planar)
		if err != nil {
			return nil, err
		}
		if proven {
			return result(volume)
		}
	}
	// Test a through-profile overlap without reordering operands for later
	// paths when neither side is supported as through-profile stock.
	profileStock, profileTool := a, c
	if profileStock.cylinder != nil && profileTool.analytic != nil {
		profileStock, profileTool = profileTool, profileStock
	}
	if _, volume, handled, err := cutThroughProfiles(profileStock, []stored{profileTool}); handled {
		if err != nil {
			return nil, err
		}
		return result(volume)
	}
	if a.cylinder != nil && c.cylinder != nil {
		n, m := a.cylinder.frame.ZAxis(), c.cylinder.frame.ZAxis()
		if cardinalFrame(a.cylinder.frame) && (m == n || m == n.Scale(-1)) {
			volume, err := parallelCylinderOverlap(a.cylinder, c.cylinder)
			if err != nil {
				return nil, err
			}
			return result(volume)
		}
	}
	if a.planar() && c.analytic != nil && c.analytic.pockets != nil {
		a, c = c, a
	}
	if c.planar() {
		stock, supported, err := a.planarCutBoundary()
		if err != nil {
			return nil, err
		}
		if supported {
			s, err := poly.Combine(context.Background(), stock, c.solid, poly.Intersection, poly.Options{})
			if err != nil {
				return nil, err
			}
			if len(s.Faces()) == 0 {
				return result(0)
			}
			p, err := s.MassProperties()
			if err != nil {
				return nil, err
			}
			return result(p.Volume)
		}
	}
	stock, tool := a, c
	if stock.cylinder != nil && tool.analytic != nil || stock.cylinder != nil && tool.cylinder != nil && stock.cylinder.radius < tool.cylinder.radius {
		stock, tool = tool, stock
	}
	if sector, ok := coaxialThrough(stock, tool.cylinder); ok {
		return result(sector.angle * (tool.cylinder.radius*tool.cylinder.radius - sector.inner*sector.inner) * sector.height / 2)
	}
	if a.cylinder != nil && c.analytic != nil {
		a, c = c, a
	}
	if a.analytic != nil && a.analytic.pockets != nil && c.cylinder != nil {
		p, err := a.analytic.pockets.throughBore(c.cylinder)
		if err != nil {
			return nil, err
		}
		if _, err := trimmed.ExtrudeBoxWithPocketsAndBores(context.Background(), p.frame, p.size, p.pockets, p.bores); err != nil {
			return nil, err
		}
		return result(math.Pi * c.cylinder.radius * c.cylinder.radius * p.size.Z)
	}
	if a.analytic != nil || c.analytic != nil {
		return nil, &failureError{"UNSUPPORTED", "overlap volume is outside the supported analytic stock domain"}
	}
	if a.cylinder != nil && c.planar() {
		a, c = c, a
	}
	if a.planar() && c.cylinder != nil {
		if _, _, ok := cornerCylinderRecipe(a.solid, c.cylinder); ok {
			r := c.cylinder.radius
			return result(math.Pi * r * r * c.cylinder.height / 4)
		}
	}
	if a.treated != nil || c.treated != nil {
		volume, err := treatedOverlap(a, c)
		if err != nil {
			return nil, err
		}
		return result(volume)
	}
	if a.cylinder != nil && c.cylinder == nil {
		a, c = c, a
	}
	if c.cylinder == nil || a.cylinder != nil {
		return nil, &failureError{"UNSUPPORTED", "overlap volume requires planar stock or qualified bored stock and a cylindrical tool"}
	}
	base := a.solid
	if a.bore != nil {
		base = a.bore.base
	}
	cut, start, end, err := boreSpan(base, c.cylinder, false)
	if err != nil {
		return nil, err
	}
	lo, hi := math.Max(0, start), math.Min(c.cylinder.height, end)
	if a.bore == nil {
		if _, _, err := trimmed.BoreCylinderAlong(base, cut.Entry, cut.Exit, cut.AxisPoint, cut.Direction, cut.Radius, 1e-9); err != nil {
			return nil, &failureError{"UNSUPPORTED", "overlap volume requires a strictly contained radial footprint: " + err.Error()}
		}
		return result(math.Pi * c.cylinder.radius * c.cylinder.radius * math.Max(0, hi-lo))
	}
	if hi <= lo {
		return result(0)
	}
	tools := append(append([]*cylinderBody(nil), a.bore.cylinders...), c.cylinder)
	body, err := newBore(base, tools)
	if err != nil {
		return nil, err
	}
	volume, _, err := body.measures()
	if err != nil {
		return nil, err
	}
	delta := a.result.GeometryAssertions.Volume - volume
	tolerance := math.Max(1e-9, math.Abs(a.result.GeometryAssertions.Volume)*1e-10)
	if !finite(delta) || delta < -tolerance || delta > math.Min(a.result.GeometryAssertions.Volume, c.result.GeometryAssertions.Volume)+tolerance {
		return nil, errors.New("inconsistent overlap volume")
	}
	return result(math.Max(0, delta))
}

func parallelCylinderOverlap(a, b *cylinderBody) (float64, error) {
	m := b.frame.ZAxis()
	origin := b.frame.Origin()
	end := origin.Add(m.Scale(b.height))
	p, q := a.frame.ToLocal(origin), a.frame.ToLocal(end)
	if a.frame.ToWorld(p) != origin || a.frame.ToWorld(q) != end || p.X != q.X || p.Y != q.Y {
		return 0, geom.ErrUnrepresentable
	}
	height := min(a.height, max(p.Z, q.Z)) - max(0, min(p.Z, q.Z))
	if height <= 0 {
		return 0, nil
	}
	solid, err := parametric.CombineParallelCylinders(a.frame, parametric.DiskProfile{Radius: a.radius},
		parametric.DiskProfile{Center: geom.Vec2{X: p.X, Y: p.Y}, Radius: b.radius}, height, parametric.Intersection, 1e-9)
	if err != nil || len(solid.Faces()) == 0 {
		return 0, err
	}
	mass, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	return mass.Volume, err
}

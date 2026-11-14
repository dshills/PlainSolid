package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func crossedBoreFromSTEP(frame geom.Frame, dimensions, crossing geom.Vec3, radii [2]float64, unit float64) (*boreBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 || !cardinalFrame(frame) {
		return nil, geom.ErrInvalid
	}
	origin, end := frame.Origin().Scale(scale), frame.ToWorld(dimensions).Scale(scale)
	bounds, err := geom.Bounds([]geom.Vec3{origin, end})
	if err != nil {
		return nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return nil, err
	}
	tools := make([]*cylinderBody, 0, 2)
	for i, site := range []struct {
		origin, end, axis geom.Vec3
	}{
		{geom.Vec3{Y: crossing.Y, Z: crossing.Z}, geom.Vec3{X: dimensions.X, Y: crossing.Y, Z: crossing.Z}, frame.XAxis()},
		{geom.Vec3{X: crossing.X, Y: crossing.Y}, geom.Vec3{X: crossing.X, Y: crossing.Y, Z: dimensions.Z}, frame.ZAxis()},
	} {
		start, finish := frame.ToWorld(site.origin).Scale(scale), frame.ToWorld(site.end).Scale(scale)
		length := finish.Sub(start).Dot(site.axis)
		if start.Add(site.axis.Scale(length)) != finish {
			return nil, geom.ErrUnrepresentable
		}
		f, err := geom.FrameFromNormal(start, site.axis)
		if err != nil {
			return nil, err
		}
		tool, err := newCylinder(f, radii[i]*scale, length)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return newBore(base, tools)
}

func crossedBoreRecipe(base brep.Solid, tools []*cylinderBody, blind []bool) (geom.Frame, geom.Vec3, geom.Vec3, [2]float64, bool) {
	fail := func() (geom.Frame, geom.Vec3, geom.Vec3, [2]float64, bool) {
		return geom.Frame{}, geom.Vec3{}, geom.Vec3{}, [2]float64{}, false
	}
	vertices := base.Vertices()
	if len(tools) != 2 || blind[0] || blind[1] || len(vertices) != 8 ||
		len(base.Edges()) != 12 || len(base.Faces()) != 6 {
		return fail()
	}
	a, b := tools[0], tools[1]
	if !cardinalFrame(a.frame) || !cardinalFrame(b.frame) ||
		a.frame.ZAxis().Dot(b.frame.ZAxis()) != 0 {
		return fail()
	}
	// Sort the two unoriented cardinal axes so grouped and successive cuts
	// use the same chart regardless of tool order or signed through extent.
	positive := func(v geom.Vec3) geom.Vec3 {
		if v.X+v.Y+v.Z < 0 {
			return v.Scale(-1)
		}
		return v
	}
	x, z := positive(a.frame.ZAxis()), positive(b.frame.ZAxis())
	if x.X < z.X || x.X == z.X && x.Y < z.Y {
		x, z, a, b = z, x, b, a
	}
	orientation, err := geom.NewFrame(geom.Vec3{}, x, z)
	if err != nil {
		return fail()
	}
	for i, v := range vertices {
		vertices[i] = orientation.ToLocal(v)
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil || !rectangularFaces(base, orientation, bounds.Min, bounds.Max) {
		return fail()
	}
	origin := orientation.ToWorld(bounds.Min)
	frame, err := geom.NewFrame(origin, x, z)
	size := bounds.Max.Sub(bounds.Min)
	if err != nil || frame.ToWorld(size) != orientation.ToWorld(bounds.Max) {
		return fail()
	}
	p, q := frame.ToLocal(a.frame.Origin()), frame.ToLocal(b.frame.Origin())
	if p.Y != q.Y || frame.ToWorld(p) != a.frame.Origin() || frame.ToWorld(q) != b.frame.Origin() {
		return fail()
	}
	crossing := geom.Vec3{X: q.X, Y: p.Y, Z: p.Z}
	return frame, size, crossing, [2]float64{a.radius, b.radius}, true
}

func newCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool, error) {
	if len(tools) > 2 {
		var sites [2]int
		count := 0
		for i, tool := range tools {
			if blind[i] || !cardinalFrame(tool.frame) {
				return nil, false, nil
			}
			matched := false
			for _, j := range sites[:count] {
				old := tools[j]
				if tool.radius == old.radius && sameCardinalBoreLine(old, tool) {
					matched = true
					break
				}
			}
			if !matched {
				if count == len(sites) {
					return nil, false, nil
				}
				sites[count] = i
				count++
			}
		}
		if count != len(sites) {
			return nil, false, nil
		}
		i, j := sites[0], sites[1]
		tools = []*cylinderBody{tools[i], tools[j]}
		cuts = []trimmed.CylinderCut{cuts[i], cuts[j]}
		spans = []float64{spans[i], spans[j]}
		blind = []bool{false, false}
	}
	frame, size, crossing, radii, ok := crossedBoreRecipe(base, tools, blind)
	if !ok {
		return nil, false, nil
	}
	ctx := context.Background()
	var result trimmed.BoxCrossedBoresResult
	var err error
	if radii[0] == radii[1] {
		result, err = trimmed.BoxCrossedBores(ctx, frame, size, crossing, radii[0], 1e-9)
	} else {
		result, err = trimmed.BoxUnequalCrossedBores(ctx, frame, size, crossing, radii[0], radii[1], 1e-9)
	}
	if err != nil {
		return nil, true, err
	}
	return &boreBody{solid: result.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, crossed: &result}, true, nil
}

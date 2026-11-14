package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func crossedBoreSTEPRecipe(body *boreBody) (step.CrossedBoreBody, error) {
	frame, size, crossing, ok := crossedBoreRecipe(body.base, body.cylinders, body.blind)
	if !ok {
		return step.CrossedBoreBody{}, geom.ErrUnrepresentable
	}
	return step.CrossedBoreBody{Name: "PlainSolid", Frame: frame, Dimensions: size, Crossing: crossing, Radius: body.cylinders[0].radius}, nil
}

func crossedBoreFromSTEP(p step.CrossedBoreBody) (*boreBody, error) {
	origin, end := p.Frame.Origin(), p.Frame.ToWorld(p.Dimensions)
	bounds, err := geom.Bounds([]geom.Vec3{origin, end})
	if err != nil {
		return nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return nil, err
	}
	var tools []*cylinderBody
	for _, site := range []struct {
		origin, axis geom.Vec3
		length       float64
	}{
		{geom.Vec3{Y: p.Crossing.Y, Z: p.Crossing.Z}, p.Frame.XAxis(), p.Dimensions.X},
		{geom.Vec3{X: p.Crossing.X, Y: p.Crossing.Y}, p.Frame.ZAxis(), p.Dimensions.Z},
	} {
		frame, err := geom.FrameFromNormal(p.Frame.ToWorld(site.origin), site.axis)
		if err != nil {
			return nil, err
		}
		tool, err := newCylinder(frame, p.Radius, site.length)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return newBore(base, tools)
}

func crossedBoreRecipe(base brep.Solid, tools []*cylinderBody, blind []bool) (geom.Frame, geom.Vec3, geom.Vec3, bool) {
	fail := func() (geom.Frame, geom.Vec3, geom.Vec3, bool) {
		return geom.Frame{}, geom.Vec3{}, geom.Vec3{}, false
	}
	vertices := base.Vertices()
	if len(tools) != 2 || blind[0] || blind[1] || len(vertices) != 8 ||
		len(base.Edges()) != 12 || len(base.Faces()) != 6 {
		return fail()
	}
	a, b := tools[0], tools[1]
	if a.radius != b.radius || !cardinalFrame(a.frame) || !cardinalFrame(b.frame) ||
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
	return frame, size, crossing, true
}

func newCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool, error) {
	if len(tools) > 2 {
		var sites [2]int
		count := 0
		for i, tool := range tools {
			if blind[i] || !cardinalFrame(tool.frame) {
				return nil, false, nil
			}
			axis, origin := tool.frame.ZAxis(), tool.frame.Origin()
			matched := false
			for _, j := range sites[:count] {
				old := tools[j]
				delta := old.frame.Origin().Sub(origin)
				if tool.radius == old.radius && axis.Cross(old.frame.ZAxis()) == (geom.Vec3{}) &&
					axis.Cross(delta) == (geom.Vec3{}) {
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
	frame, size, crossing, ok := crossedBoreRecipe(base, tools, blind)
	if !ok {
		return nil, false, nil
	}
	result, err := trimmed.BoxCrossedBores(context.Background(), frame, size, crossing, tools[0].radius, 1e-9)
	if err != nil {
		return nil, true, err
	}
	return &boreBody{solid: result.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, crossed: &result}, true, nil
}

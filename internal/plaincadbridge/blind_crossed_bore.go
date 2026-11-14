package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type blindCrossedGeometry struct {
	crossedBoreGeometry
	depth       float64
	oppositeCap bool
}

func blindCrossedRecipe(base brep.Solid, tools []*cylinderBody, spans []float64, blind []bool) (blindCrossedGeometry, bool) {
	var p blindCrossedGeometry
	if len(tools) != 2 || blind[0] == blind[1] || tools[0].radius != tools[1].radius {
		return p, false
	}
	crossed, ok := crossedBoreRecipe(base, tools, []bool{false, false})
	if !ok || crossed.offset != 0 {
		return p, false
	}
	index := 0
	if blind[1] {
		index = 1
	}
	axis := tools[index].frame.ZAxis()
	// The native recipe stops local Z. Swap local X/Z with the required
	// transverse reflection if the authored blind hole runs along local X.
	if axis.Dot(crossed.frame.XAxis()) != 0 {
		old := crossed.frame
		f, err := geom.NewFrame(old.ToWorld(geom.Vec3{Y: crossed.dimensions.Y}), old.ZAxis(), old.XAxis())
		if err != nil {
			return p, false
		}
		crossed.frame = f
		x, y, z := crossed.dimensions.X, crossed.dimensions.Y, crossed.dimensions.Z
		crossed.dimensions = geom.Vec3{X: z, Y: y, Z: x}
		crossed.crossing = f.ToLocal(old.ToWorld(crossed.crossing))
	}
	direction := axis.Dot(crossed.frame.ZAxis())
	if direction != 1 && direction != -1 || !finite(spans[index]) || spans[index] <= 0 {
		return p, false
	}
	p = blindCrossedGeometry{crossedBoreGeometry: crossed, depth: spans[index], oppositeCap: direction == -1}
	start, finish := 0., spans[index]
	if p.oppositeCap {
		start, finish = crossed.dimensions.Z, crossed.dimensions.Z-spans[index]
	}
	c := crossed.crossing
	toolFinish := tools[index].frame.Origin().Add(axis.Scale(tools[index].height))
	expected := p.frame.ToWorld(geom.Vec3{X: c.X, Y: c.Y, Z: finish})
	if toolFinish != expected || start == finish {
		return blindCrossedGeometry{}, false
	}
	return p, true
}

func newBlindCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool, error) {
	p, ok := blindCrossedRecipe(base, tools, spans, blind)
	if !ok {
		return nil, false, nil
	}
	result, err := trimmed.BoxBlindCrossedBores(context.Background(), p.frame, p.dimensions, p.crossing, p.radii[0], p.depth, p.oppositeCap, 1e-9)
	if err != nil {
		return nil, true, err
	}
	floors := make([]int, 2)
	for i := range blind {
		if blind[i] {
			floors[i] = trimmed.BlindCrossedBoreFloorFace
		}
	}
	return &boreBody{
		solid: result.Solid, base: base, cylinders: tools, cuts: cuts,
		spans: spans, blind: blind, floors: floors, crossed: &result, blindCrossed: &p,
	}, true, nil
}

func blindCrossedFromSTEP(p step.CrossedBoreBody, unit float64) (*boreBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 || !cardinalFrame(p.Frame) {
		return nil, geom.ErrInvalid
	}
	bounds, err := geom.Bounds([]geom.Vec3{p.Frame.Origin().Scale(scale), p.Frame.ToWorld(p.Dimensions).Scale(scale)})
	if err != nil {
		return nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return nil, err
	}
	t := p.Crossing
	xStart := p.Frame.ToWorld(geom.Vec3{Y: t.Y, Z: t.Z}).Scale(scale)
	xEnd := p.Frame.ToWorld(geom.Vec3{X: p.Dimensions.X, Y: t.Y, Z: t.Z}).Scale(scale)
	xf, err := geom.FrameFromNormal(xStart, p.Frame.XAxis())
	if err != nil {
		return nil, err
	}
	x, err := newCylinder(xf, p.Radius*scale, xEnd.Sub(xStart).Dot(xf.ZAxis()))
	if err != nil {
		return nil, err
	}
	zFrom, zTo, normal := 0., p.BlindDepth, p.Frame.ZAxis()
	if p.OppositeCap {
		zFrom, zTo, normal = p.Dimensions.Z, p.Dimensions.Z-p.BlindDepth, normal.Scale(-1)
	}
	zStart := p.Frame.ToWorld(geom.Vec3{X: t.X, Y: t.Y, Z: zFrom}).Scale(scale)
	zEnd := p.Frame.ToWorld(geom.Vec3{X: t.X, Y: t.Y, Z: zTo}).Scale(scale)
	zf, err := geom.FrameFromNormal(zStart, normal)
	if err != nil {
		return nil, err
	}
	z, err := newCylinder(zf, p.Radius*scale, zEnd.Sub(zStart).Dot(normal))
	if err != nil {
		return nil, err
	}
	if zStart.Add(normal.Scale(z.height)) != zEnd {
		return nil, geom.ErrUnrepresentable
	}
	return newBore(base, []*cylinderBody{x, z})
}

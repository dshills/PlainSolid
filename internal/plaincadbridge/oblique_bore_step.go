package plaincadbridge

import (
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func obliqueBoreSTEPRecipe(b *boreBody) (step.ObliqueBoreBody, error) {
	cut := b.cuts[0]
	bounds, err := b.base.Bounds()
	if err != nil {
		return step.ObliqueBoreBody{}, err
	}
	box, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return step.ObliqueBoreBody{}, err
	}
	out := step.ObliqueBoreBody{
		Name: "PlainSolid", Stock: bounds,
		AxisPoint: cut.AxisPoint, Direction: cut.Direction, Radius: cut.Radius,
	}
	// STEP uses the canonical box's face ordering. The retained CAD stock's
	// face IDs can be different when authored as a prism.
	for _, target := range []struct {
		source int
		dest   *int
	}{{cut.Entry, &out.Entry}, {cut.Exit, &out.Exit}} {
		plane, err := b.base.FacePlane(target.source)
		if err != nil {
			return step.ObliqueBoreBody{}, err
		}
		*target.dest = -1
		for i := range box.Faces() {
			f, err := box.FacePlane(i)
			if err != nil {
				return step.ObliqueBoreBody{}, err
			}
			if f.Normal() == plane.Normal() && f.SignedDistance(plane.Origin()) == 0 {
				*target.dest = i
				break
			}
		}
		if *target.dest < 0 {
			return step.ObliqueBoreBody{}, geom.ErrUnrepresentable
		}
	}
	return out, nil
}

func obliqueBoreFromSTEP(p step.ObliqueBoreBody, unit float64) (*boreBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return nil, geom.ErrInvalid
	}
	base, err := brep.Box(p.Stock.Min.Scale(scale), p.Stock.Max.Scale(scale))
	if err != nil {
		return nil, err
	}
	cut := trimmed.CylinderCut{
		Entry: p.Entry, Exit: p.Exit, AxisPoint: p.AxisPoint.Scale(scale),
		Direction: p.Direction, Radius: p.Radius * scale,
	}
	entry, err := base.FacePlane(cut.Entry)
	if err != nil {
		return nil, err
	}
	exit, err := base.FacePlane(cut.Exit)
	if err != nil {
		return nil, err
	}
	axis, err := cut.Direction.Unit()
	if err != nil {
		return nil, err
	}
	length := -exit.SignedDistance(cut.AxisPoint) / exit.Normal().Dot(axis)
	// The STEP boundary retains the infinite cylinder support. A replacement
	// finite tool, when needed for subsequent modeling, must span every
	// generator. Offset by more than the largest axial opening extent.
	denominator := min(math.Abs(entry.Normal().Dot(axis)), math.Abs(exit.Normal().Dot(axis)))
	padding := 2 * cut.Radius / denominator
	frame, err := geom.FrameFromNormal(cut.AxisPoint.Sub(axis.Scale(padding)), cut.Direction)
	if err != nil {
		return nil, err
	}
	tool, err := newCylinder(frame, cut.Radius, length+2*padding)
	if err != nil {
		return nil, err
	}
	if !toolCapClearsOpening(tool, entry, 0) || !toolCapClearsOpening(tool, exit, tool.height) {
		return nil, geom.ErrUnrepresentable
	}
	return buildObliqueThroughBore(base, tool, cut, axis, entry, exit, length)
}

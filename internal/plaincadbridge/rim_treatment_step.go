package plaincadbridge

import (
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type cylinderRimTreatment struct {
	frame          geom.Frame
	radius, height float64
	lower, upper   float64
	chamfer        bool
}

func (p cylinderRimTreatment) writeSTEP(w io.Writer) error {
	if !cardinalFrame(p.frame) {
		return geom.ErrDomain
	}
	z := p.frame.ZAxis()
	center := p.frame.Origin()
	sizes := [2]float64{p.lower, p.upper}
	if z.X < 0 || z.Y < 0 || z.Z < 0 {
		center = p.frame.ToWorld(geom.Vec3{Z: p.height})
		z = z.Scale(-1)
		sizes[0], sizes[1] = sizes[1], sizes[0]
	}
	if p.chamfer {
		model := step.WrappedRimChamferBody{Name: "PlainSolid", Center: center, Axis: z,
			Radius: p.radius, Height: p.height, Setbacks: sizes, LengthUnit: .001}
		return step.WritePlacedWrappedRimChamfer(w, model)
	}
	body := step.WrappedRimFilletBody{Name: "PlainSolid", Center: center, Radius: p.radius,
		Height: p.height, Fillets: sizes, LengthUnit: .001}
	return step.WritePlacedWrappedRimFillet(w, body, z)
}

func (p cylinderRimTreatment) transformed(m geom.Affine) (*cylinderRimTreatment, error) {
	f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
	if err != nil {
		return nil, err
	}
	p.frame = f
	return &p, nil
}

func (b *Bridge) putRimFilletSTEP(p step.WrappedRimFilletBody, axis geom.Vec3) (Shape, error) {
	return b.putRimTreatmentSTEP(p.Center, axis, p.Radius, p.Height, p.LengthUnit, p.Fillets, false)
}

func (b *Bridge) putRimChamferSTEP(p step.WrappedRimChamferBody) (Shape, error) {
	return b.putRimTreatmentSTEP(p.Center, p.Axis, p.Radius, p.Height, p.LengthUnit, p.Setbacks, true)
}

func (b *Bridge) putRimTreatmentSTEP(center, axis geom.Vec3, radius, height, unit float64, sizes [2]float64, chamfer bool) (Shape, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.FrameFromNormal(center.Scale(scale), axis)
	if err != nil {
		return Shape{}, err
	}
	recipe := cylinderRimTreatment{frame: frame, radius: radius * scale, height: height * scale,
		lower: sizes[0] * scale, upper: sizes[1] * scale, chamfer: chamfer}
	var s parametric.Solid
	if chamfer {
		c := cylinderBody{frame: frame, radius: recipe.radius, height: recipe.height}
		s, err = chamferedCylinder(&c, recipe.lower, recipe.upper)
	} else {
		s, err = parametric.RoundedCylinder(frame, recipe.radius, recipe.height, recipe.lower, recipe.upper)
	}
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(s)
	if err != nil {
		return Shape{}, err
	}
	body.rimPatches, body.rimTreatment = true, &recipe
	return b.putAnalyticBody(body)
}

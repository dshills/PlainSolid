package plaincadbridge

import (
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type cylinderRimFillet struct {
	frame          geom.Frame
	radius, height float64
	lower, upper   float64
}

func (p cylinderRimFillet) writeSTEP(w io.Writer) error {
	if !cardinalFrame(p.frame) {
		return geom.ErrDomain
	}
	z := p.frame.ZAxis()
	center := p.frame.Origin()
	fillets := [2]float64{p.lower, p.upper}
	if z.X < 0 || z.Y < 0 || z.Z < 0 {
		center = p.frame.ToWorld(geom.Vec3{Z: p.height})
		z = z.Scale(-1)
		fillets[0], fillets[1] = fillets[1], fillets[0]
	}
	body := step.WrappedRimFilletBody{Name: "PlainSolid", Center: center, Radius: p.radius,
		Height: p.height, Fillets: fillets, LengthUnit: .001}
	return step.WritePlacedWrappedRimFillet(w, body, z)
}

func (p cylinderRimFillet) transformed(m geom.Affine) (*cylinderRimFillet, error) {
	f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
	if err != nil {
		return nil, err
	}
	p.frame = f
	return &p, nil
}

func (b *Bridge) putRimFilletSTEP(p step.WrappedRimFilletBody, axis geom.Vec3) (Shape, error) {
	scale := p.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.FrameFromNormal(p.Center.Scale(scale), axis)
	if err != nil {
		return Shape{}, err
	}
	recipe := cylinderRimFillet{frame: frame, radius: p.Radius * scale, height: p.Height * scale,
		lower: p.Fillets[0] * scale, upper: p.Fillets[1] * scale}
	s, err := parametric.RoundedCylinder(frame, recipe.radius, recipe.height, recipe.lower, recipe.upper)
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(s)
	if err != nil {
		return Shape{}, err
	}
	body.rimPatches, body.rimFillet = true, &recipe
	return b.putAnalyticBody(body)
}

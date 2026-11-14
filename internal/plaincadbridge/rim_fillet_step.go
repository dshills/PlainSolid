package plaincadbridge

import (
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type cylinderRimFillet struct {
	frame          geom.Frame
	radius, height float64
	lower, upper   float64
}

func (p cylinderRimFillet) stepBody() (step.WrappedRimFilletBody, bool) {
	z := p.frame.ZAxis()
	if math.Abs(z.Z) != 1 || z.X != 0 || z.Y != 0 {
		return step.WrappedRimFilletBody{}, false
	}
	center := p.frame.Origin()
	fillets := [2]float64{p.lower, p.upper}
	if z.Z < 0 {
		center.Z -= p.height
		fillets[0], fillets[1] = fillets[1], fillets[0]
	}
	return step.WrappedRimFilletBody{Name: "PlainSolid", Center: center, Radius: p.radius,
		Height: p.height, Fillets: fillets, LengthUnit: .001}, true
}

func (p cylinderRimFillet) transformed(m geom.Affine) (*cylinderRimFillet, error) {
	f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
	if err != nil {
		return nil, err
	}
	p.frame = f
	return &p, nil
}

func (b *Bridge) putRimFilletSTEP(p step.WrappedRimFilletBody) (Shape, error) {
	scale := p.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.FrameFromNormal(p.Center.Scale(scale), geom.Vec3{Z: 1})
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

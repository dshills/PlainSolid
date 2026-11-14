package plaincadbridge

import (
	"math"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (p radialSector) quarterAnnularSTEP() (step.QuarterAnnularExtrusionBody, bool) {
	if p.angle != math.Pi/2 || p.inner <= 0 || !cardinalFrame(p.frame) {
		return step.QuarterAnnularExtrusionBody{}, false
	}
	return step.QuarterAnnularExtrusionBody{
		Name: "PlainSolid", Frame: p.frame, Inner: p.inner, Radius: p.radius, Height: p.height,
	}, true
}

func (b *Bridge) putQuarterAnnularSTEP(m step.QuarterAnnularExtrusionModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	return b.putRadialSector(radialSector{
		frame: frame, inner: p.Inner * scale, radius: p.Radius * scale, height: p.Height * scale, angle: math.Pi / 2,
	})
}

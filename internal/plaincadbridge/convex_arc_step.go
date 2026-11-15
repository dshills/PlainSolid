package plaincadbridge

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (p *arcLoopSupport) convexArcSTEP() (step.ConvexArcExtrusionBody, bool) {
	if !cardinalFrame(p.frame) {
		return step.ConvexArcExtrusionBody{}, false
	}
	frame := p.frame
	if p.height < 0 {
		var err error
		frame, err = geom.NewFrame(frame.ToWorld(geom.Vec3{Z: p.height}), frame.XAxis(), frame.ZAxis())
		if err != nil {
			return step.ConvexArcExtrusionBody{}, false
		}
	}
	return step.ConvexArcExtrusionBody{Name: "PlainSolid", Frame: frame, Height: math.Abs(p.height), Segments: p.segments}, true
}

func (b *Bridge) putConvexArcExtrusionSTEP(m step.ConvexArcExtrusionModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	if scale != 1 {
		frame, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
		if err != nil {
			return Shape{}, err
		}
		p.Frame, p.Height = frame, p.Height*scale
		p.Segments = append([]trimmed.ExtrusionSegment(nil), p.Segments...)
		mul := func(v geom.Vec2) geom.Vec2 { return geom.Vec2{X: v.X * scale, Y: v.Y * scale} }
		for i := range p.Segments {
			s := &p.Segments[i]
			s.Start, s.End, s.Center = mul(s.Start), mul(s.End), mul(s.Center)
			s.Radius *= scale
		}
	}
	solid, err := step.BuildConvexArcExtrusion(p)
	if err != nil {
		return Shape{}, err
	}
	return b.putArcLoop(solid, p.Frame, p.Segments, p.Height)
}

package plaincadbridge

import (
	"math"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s circularSegmentSweep) semicircleSTEP() (step.SemicircleExtrusionBody, bool) {
	if math.Abs(s.sweep) != math.Pi || s.start != 0 {
		return step.SemicircleExtrusionBody{}, false
	}
	origin := s.origin.Add(s.z.Scale(min(0, s.height)))
	x := s.x
	if s.sweep < 0 {
		x = x.Scale(-1)
	}
	frame, err := geom.NewFrame(origin, x, s.z)
	if err != nil || frame.XAxis() != x || frame.ZAxis() != s.z || !cardinalFrame(frame) {
		return step.SemicircleExtrusionBody{}, false
	}
	return step.SemicircleExtrusionBody{Name: "PlainSolid", Frame: frame, Radius: s.radius, Height: math.Abs(s.height)}, true
}

func (b *Bridge) putSemicircleSTEP(m step.SemicircleExtrusionModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	f, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	p.Frame = f
	p.Center = geom.Vec2{X: p.Center.X * scale, Y: p.Center.Y * scale}
	p.Radius, p.Height = p.Radius*scale, p.Height*scale
	s, err := step.BuildSemicircleExtrusion(p)
	if err != nil {
		return Shape{}, err
	}
	r, h := p.Radius, p.Height
	segment := circularSegmentSweep{
		origin: f.ToWorld(geom.Vec3{X: p.Center.X, Y: p.Center.Y}), x: f.XAxis(), y: f.YAxis(), z: f.ZAxis(),
		radius: r, height: h, sweep: math.Pi,
	}
	return b.putAnalyticBody(&analyticBody{solid: s, segment: &segment,
		volume: math.Pi * r * r * h / 2, area: math.Pi*r*r + (math.Pi+2)*r*h})
}

package plaincadbridge

import (
	"math"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s circularSegmentSweep) circularSegmentSTEP() (step.CircularSegmentExtrusionBody, bool) {
	start, sweep := s.start, s.sweep
	if sweep < 0 {
		start, sweep = start+sweep, -sweep
	}
	if math.Remainder(start, 2*math.Pi) != start {
		return step.CircularSegmentExtrusionBody{}, false
	}
	origin := s.origin.Add(s.z.Scale(min(0, s.height)))
	f, err := geom.NewFrame(origin, s.x, s.z)
	if err != nil || f.XAxis() != s.x || f.ZAxis() != s.z || !cardinalFrame(f) {
		return step.CircularSegmentExtrusionBody{}, false
	}
	return step.CircularSegmentExtrusionBody{Name: "PlainSolid", Frame: f,
		Radius: s.radius, Height: math.Abs(s.height), StartAngle: start, Sweep: sweep}, true
}

func (b *Bridge) putCircularSegmentSTEP(m step.CircularSegmentExtrusionModel) (Shape, error) {
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
	p.Radius, p.Height = p.Radius*scale, p.Height*scale
	s, err := step.BuildCircularSegmentExtrusion(p)
	if err != nil {
		return Shape{}, err
	}
	r, h, sweep := p.Radius, p.Height, p.Sweep
	section := r * r * (sweep - math.Sin(sweep)) / 2
	perimeter := r * (sweep + 2*math.Sin(sweep/2))
	segment := circularSegmentSweep{
		origin: f.Origin(), x: f.XAxis(), y: f.YAxis(), z: f.ZAxis(),
		radius: r, height: h, start: p.StartAngle, sweep: sweep,
	}
	return b.putAnalyticBody(&analyticBody{solid: s, segment: &segment, volume: section * h, area: 2*section + perimeter*h})
}

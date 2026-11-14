package plaincadbridge

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (p *arcLoopSupport) quarterSectorSTEP() (step.QuarterSectorExtrusionBody, bool) {
	if len(p.segments) != 3 || !cardinalFrame(p.frame) {
		return step.QuarterSectorExtrusionBody{}, false
	}
	frame := p.frame
	if p.height < 0 {
		var err error
		frame, err = geom.NewFrame(frame.ToWorld(geom.Vec3{Z: p.height}), frame.XAxis(), frame.ZAxis())
		if err != nil {
			return step.QuarterSectorExtrusionBody{}, false
		}
	}
	for i, arc := range p.segments {
		if math.Abs(arc.Sweep) != math.Pi/2 || arc.Radius <= 0 {
			continue
		}
		previous, next := p.segments[(i+2)%3], p.segments[(i+1)%3]
		if previous != (trimmed.ExtrusionSegment{Start: arc.Center, End: arc.Start}) ||
			next != (trimmed.ExtrusionSegment{Start: arc.End, End: arc.Center}) {
			continue
		}
		ratio := arc.StartAngle / (math.Pi / 2)
		start := math.Round(ratio)
		if arc.StartAngle != start*(math.Pi/2) {
			continue
		}
		a, b := arc.Start, arc.End
		if arc.Sweep < 0 {
			start--
			a, b = b, a
		}
		q := (int(start)%4 + 4) % 4
		body := step.QuarterSectorExtrusionBody{Name: "PlainSolid", Frame: frame, Center: arc.Center,
			Radius: arc.Radius, Height: math.Abs(p.height), Quarter: q}
		segments, err := body.Segments()
		if err == nil && segments[1].Start == a && segments[1].End == b {
			return body, true
		}
	}
	return step.QuarterSectorExtrusionBody{}, false
}

func (b *Bridge) putQuarterSectorSTEP(m step.QuarterSectorExtrusionModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	p.Frame = frame
	p.Center = geom.Vec2{X: p.Center.X * scale, Y: p.Center.Y * scale}
	p.Radius, p.Height = p.Radius*scale, p.Height*scale
	segments, err := p.Segments()
	if err != nil {
		return Shape{}, err
	}
	solid, err := step.BuildQuarterSectorExtrusion(p)
	if err != nil {
		return Shape{}, err
	}
	return b.putArcLoop(solid, frame, segments, p.Height)
}

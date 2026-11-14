package plaincadbridge

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s *arcLoopSupport) circularLensSTEP() (step.CircularLensExtrusionBody, bool) {
	if len(s.segments) != 2 || !cardinalFrame(s.frame) {
		return step.CircularLensExtrusionBody{}, false
	}
	a, b := s.segments[0], s.segments[1]
	if a.Radius <= 0 || b.Radius <= 0 || a.Sweep*b.Sweep <= 0 ||
		math.Abs(a.Sweep) >= math.Pi || math.Abs(b.Sweep) >= math.Pi {
		return step.CircularLensExtrusionBody{}, false
	}
	if a.Sweep < 0 {
		reverse := func(s trimmed.ExtrusionSegment) trimmed.ExtrusionSegment {
			s.Start, s.End = s.End, s.Start
			s.StartAngle += s.Sweep
			s.Sweep = -s.Sweep
			return s
		}
		a, b = reverse(b), reverse(a)
	}
	f := s.frame
	if s.height < 0 {
		var err error
		f, err = geom.NewFrame(f.ToWorld(geom.Vec3{Z: s.height}), f.XAxis(), f.ZAxis())
		if err != nil {
			return step.CircularLensExtrusionBody{}, false
		}
	}
	return step.CircularLensExtrusionBody{Name: "PlainSolid", Frame: f, Height: math.Abs(s.height),
		Arcs: [2]trimmed.ExtrusionSegment{a, b}}, true
}

func (b *Bridge) putCircularLensSTEP(m step.CircularLensExtrusionModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	f, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	p.Frame, p.Height = f, p.Height*scale
	for i, arc := range p.Arcs {
		mul := func(v geom.Vec2) geom.Vec2 { return geom.Vec2{X: v.X * scale, Y: v.Y * scale} }
		arc.Start, arc.End, arc.Center = mul(arc.Start), mul(arc.End), mul(arc.Center)
		arc.Radius *= scale
		p.Arcs[i] = arc
	}
	s, err := step.BuildCircularLensExtrusion(p)
	if err != nil {
		return Shape{}, err
	}
	return b.putArcLoop(s, f, p.Arcs[:], p.Height)
}

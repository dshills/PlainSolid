package plaincadbridge

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s *arcLoopSupport) circularLensSTEP() (step.CircularLensExtrusionBody, bool) {
	if !cardinalFrame(s.frame) {
		return step.CircularLensExtrusionBody{}, false
	}
	arcs, ok := lensSupportArcs(s.segments)
	if !ok {
		return step.CircularLensExtrusionBody{}, false
	}
	a, b := arcs[0], arcs[1]
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

// lensSupportArcs joins adjacent authored seams on the same circular
// support for exchange. Original sketch entities and feature IDs are retained.
func lensSupportArcs(segments []trimmed.ExtrusionSegment) ([2]trimmed.ExtrusionSegment, bool) {
	var arcs [2]trimmed.ExtrusionSegment
	if len(segments) < 2 {
		return arcs, false
	}
	if len(segments) == 2 {
		return [2]trimmed.ExtrusionSegment{segments[0], segments[1]}, true
	}
	same := func(a, b trimmed.ExtrusionSegment) bool {
		return a.Center == b.Center && a.Radius == b.Radius
	}
	first := -1
	count := 0
	for i, a := range segments {
		if a.Radius <= 0 || a.Sweep == 0 {
			return arcs, false
		}
		if !same(a, segments[(i+len(segments)-1)%len(segments)]) {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	if count != len(arcs) {
		return arcs, false
	}
	index := 0
	arcs[0] = segments[first]
	for i := 1; i < len(segments); i++ {
		next := segments[(first+i)%len(segments)]
		run := &arcs[index]
		if !same(*run, next) {
			index++
			arcs[index] = next
			continue
		}
		end := run.StartAngle + run.Sweep
		// The native input has already checked both evaluations against
		// their shared anchored vertex. Allow only angular roundoff at
		// the represented seam and preserve the sum of the input sweeps.
		scale := max(1, math.Abs(end), math.Abs(next.StartAngle))
		budget := 64 * (math.Nextafter(scale, math.Inf(1)) - scale)
		if run.End != next.Start || (run.Sweep > 0) != (next.Sweep > 0) ||
			math.Abs(math.Remainder(end-next.StartAngle, 2*math.Pi)) > budget {
			return arcs, false
		}
		run.End = next.End
		run.Sweep += next.Sweep
	}
	return arcs, true
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

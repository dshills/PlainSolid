package plaincadbridge

import (
	"math"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type coaxialSpan struct{ lo, hi, radius float64 }
type coaxialBody struct {
	frame geom.Frame
	spans []coaxialSpan
	inner float64
}

func coaxialRecipe(s stored) (*coaxialBody, bool) {
	if c := s.cylinder; c != nil {
		return &coaxialBody{frame: c.frame, spans: []coaxialSpan{{0, c.height, c.radius}}}, cardinalFrame(c.frame)
	}
	if a := s.analytic; a != nil {
		if a.coaxial != nil {
			return a.coaxial, true
		}
		if p := a.shaft; p != nil && len(p.Socket) == 0 {
			return &coaxialBody{frame: p.Frame, spans: []coaxialSpan{{-p.ShaftLength, 0, p.ShaftRadius}, {0, p.HeadHeight, p.HeadRadius}}}, cardinalFrame(p.Frame)
		}
		if r := a.sector; r != nil && r.angle == 2*math.Pi {
			return &coaxialBody{frame: r.frame, spans: []coaxialSpan{{0, r.height, r.radius}}, inner: r.inner}, cardinalFrame(r.frame)
		}
	}
	return nil, false
}

// fuseCoaxial takes the outer radius envelope of connected coaxial stock
// with a common inner radius. The original axial spans determine the
// steps and, for annular stock, the continuous bore.
func (b *Bridge) fuseCoaxial(a, c stored) (Shape, bool, error) {
	x, ok := coaxialRecipe(a)
	if !ok {
		return Shape{}, false, nil
	}
	y, ok := coaxialRecipe(c)
	if !ok || x.inner != y.inner {
		return Shape{}, false, nil
	}
	axis := x.frame.ZAxis()
	other := y.frame.ZAxis()
	if other != axis && other != axis.Scale(-1) {
		return Shape{}, false, nil
	}
	center := x.frame.ToLocal(y.frame.Origin())
	if center.X != 0 || center.Y != 0 {
		return Shape{}, false, nil
	}
	reject := func(message string) (Shape, bool, error) { return Shape{}, true, &failureError{"UNSUPPORTED", message} }
	spans := append([]coaxialSpan(nil), x.spans...)
	for _, p := range y.spans {
		ends := [2]float64{}
		for i, z := range []float64{p.lo, p.hi} {
			world := y.frame.ToWorld(geom.Vec3{Z: z})
			local := x.frame.ToLocal(world)
			if !world.IsFinite() || local.X != 0 || local.Y != 0 || x.frame.ToWorld(local) != world {
				return reject("coaxial join endpoints cannot be represented in one exact frame")
			}
			ends[i] = local.Z
		}
		lo, hi := min(ends[0], ends[1]), max(ends[0], ends[1])
		if !(lo < hi) {
			return reject("coaxial join has collapsed axial extent")
		}
		spans = append(spans, coaxialSpan{lo, hi, p.radius})
	}
	if len(spans) > 64 {
		return Shape{}, true, &failureError{"LIMIT", "coaxial join exceeds 64 source intervals"}
	}
	levels := make([]float64, 0, 2*len(spans))
	for _, p := range spans {
		levels = append(levels, p.lo, p.hi)
	}
	sort.Float64s(levels)
	unique := levels[:0]
	for _, z := range levels {
		if len(unique) == 0 || z != unique[len(unique)-1] {
			unique = append(unique, z)
		}
	}
	result := &coaxialBody{frame: x.frame, inner: x.inner}
	for i := 0; i+1 < len(unique); i++ {
		lo, hi := unique[i], unique[i+1]
		radius := 0.
		for _, p := range spans {
			if p.lo <= lo && p.hi >= hi {
				radius = max(radius, p.radius)
			}
		}
		if radius == 0 {
			return reject("coaxial join requires connected axial intervals")
		}
		n := len(result.spans)
		if n > 0 && result.spans[n-1].radius == radius {
			result.spans[n-1].hi = hi
		} else {
			result.spans = append(result.spans, coaxialSpan{lo, hi, radius})
		}
	}
	if len(result.spans) > 32 {
		return Shape{}, true, &failureError{"LIMIT", "coaxial join exceeds 32 retained steps"}
	}
	shape, err := b.putCoaxial(result)
	return shape, true, err
}

func (p *coaxialBody) profile() []geom.Vec2 {
	first, last := p.spans[0], p.spans[len(p.spans)-1]
	profile := []geom.Vec2{{X: p.inner, Y: first.lo}, {X: first.radius, Y: first.lo}}
	for i, span := range p.spans {
		profile = append(profile, geom.Vec2{X: span.radius, Y: span.hi})
		if i+1 < len(p.spans) {
			profile = append(profile, geom.Vec2{X: p.spans[i+1].radius, Y: span.hi})
		}
	}
	return append(profile, geom.Vec2{X: p.inner, Y: last.hi})
}

func (b *Bridge) putCoaxial(p *coaxialBody) (Shape, error) {
	first, last := p.spans[0], p.spans[len(p.spans)-1]
	ringArea := func(r float64) float64 { return (r - p.inner) * (r + p.inner) }
	volume := 0.0
	area := math.Pi * (ringArea(first.radius) + ringArea(last.radius))
	area += 2 * math.Pi * p.inner * (last.hi - first.lo)
	for i, span := range p.spans {
		length := span.hi - span.lo
		volume += math.Pi * (span.radius - p.inner) * (span.radius + p.inner) * length
		area += 2 * math.Pi * span.radius * length
		if i+1 < len(p.spans) {
			r := p.spans[i+1].radius
			area += math.Pi * math.Abs(r*r-span.radius*span.radius)
		}
	}
	patches, err := parametric.Revolve(p.frame, p.profile())
	if err != nil {
		return Shape{}, err
	}
	solid, err := trimmed.FromParametric(patches)
	if err != nil {
		return Shape{}, err
	}
	if _, err = p.bounds(); err != nil {
		return Shape{}, err
	}
	return b.putAnalyticBody(&analyticBody{solid: solid, patches: &patches, coaxial: p, volume: volume, area: area})
}

func (p *coaxialBody) bounds() (geom.Box, error) {
	if !cardinalFrame(p.frame) || len(p.spans) == 0 {
		return geom.Box{}, geom.ErrDomain
	}
	radius := 0.
	for _, s := range p.spans {
		radius = max(radius, s.radius)
	}
	lo, hi := p.spans[0].lo, p.spans[len(p.spans)-1].hi
	box := geom.Box{Min: geom.Vec3{X: -radius, Y: -radius, Z: lo}, Max: geom.Vec3{X: radius, Y: radius, Z: hi}}
	out, err := frameMap(p.frame).ApplyBox(box)
	if err != nil {
		return geom.Box{}, err
	}
	if out.Min.X >= out.Max.X || out.Min.Y >= out.Max.Y || out.Min.Z >= out.Max.Z {
		return geom.Box{}, geom.ErrUnrepresentable
	}
	return out, nil
}

func (p *coaxialBody) transformed(m geom.Affine) (*coaxialBody, error) {
	f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
	if err != nil || !cardinalFrame(f) || f.YAxis() != m.ApplyVector(p.frame.YAxis()) {
		return nil, geom.ErrUnrepresentable
	}
	out := &coaxialBody{frame: f, spans: append([]coaxialSpan(nil), p.spans...), inner: p.inner}
	_, err = out.bounds()
	return out, err
}

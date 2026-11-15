package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
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

type stepArcLoopSupport struct {
	descriptor     step.ExactCircularLoopExtrusion
	bounds         geom.Box
	exactPlacement bool
}

// putExactSTEPArcLoop accepts an already authenticated product-local solid.
// Its native world boundary and bridge geometry assertions use millimetres.
func (b *Bridge) putExactSTEPArcLoop(ctx context.Context, solid trimmed.Solid, d step.ExactCircularLoopExtrusion) (Shape, error) {
	scale := d.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	p, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if err != nil {
		return Shape{}, err
	}
	bounds, err := stepArcLoopBounds(d, scale)
	if err != nil {
		return Shape{}, err
	}
	placement := geom.Scaling(geom.Vec3{X: scale, Y: scale, Z: scale}).Mul(frameMap(d.Frame))
	if placement != geom.Identity() {
		solid, err = solid.TransformedContext(ctx, placement)
		if err != nil {
			return Shape{}, err
		}
	}
	return b.putAnalyticBody(&analyticBody{
		solid: solid, volume: p.Volume * scale * scale * scale, area: p.Area * scale * scale,
		stepArcLoop: &stepArcLoopSupport{descriptor: d, bounds: bounds, exactPlacement: d.LengthUnit == .001 && cardinalFrame(d.Frame)},
	})
}

func (p stepArcLoopSupport) transformed(m geom.Affine) (*stepArcLoopSupport, error) {
	d := p.descriptor
	scale := d.LengthUnit / .001
	origin := m.Apply(d.Frame.Origin().Scale(scale)).Scale(1 / scale)
	x, y, z := m.ApplyVector(d.Frame.XAxis()), m.ApplyVector(d.Frame.YAxis()), m.ApplyVector(d.Frame.ZAxis())
	// Reversing local Z preserves the loop's two axes through a reflection.
	if m.Determinant() < 0 {
		z, d.Height = z.Scale(-1), -d.Height
	}
	var err error
	d.Frame, err = geom.NewFrame(origin, x, z)
	if err != nil {
		return nil, err
	}
	bounds, err := stepArcLoopBounds(d, scale)
	if err != nil {
		return nil, err
	}
	// The bridge's general rigid check permits numerical error. Check all
	// original axes before this recipe can authorize an exact operation.
	exact := p.exactPlacement && cardinalFrame(d.Frame) &&
		d.Frame.XAxis() == x && d.Frame.YAxis() == y && d.Frame.ZAxis() == z
	return &stepArcLoopSupport{descriptor: d, bounds: bounds, exactPlacement: exact}, nil
}

func stepArcLoopBounds(d step.ExactCircularLoopExtrusion, scale float64) (geom.Box, error) {
	frame, err := geom.NewFrame(d.Frame.Origin().Scale(scale), d.Frame.XAxis(), d.Frame.ZAxis())
	if err != nil {
		return geom.Box{}, err
	}
	// Angles are used only to locate world-axis extrema. The boundary and
	// STEP recipe retain the exact endpoints and proved rational owners.
	mul := func(v geom.Vec2) geom.Vec2 { return geom.Vec2{X: v.X * scale, Y: v.Y * scale} }
	segments := make([]trimmed.ExtrusionSegment, len(d.Segments))
	for i, in := range d.Segments {
		s := &segments[i]
		s.Start, s.End, s.Center = mul(in.Start), mul(in.End), mul(in.Center)
		s.Radius = in.Radius * scale
		if in.Radius > 0 {
			start := geom.Vec2{X: in.Start.X - in.Center.X, Y: in.Start.Y - in.Center.Y}
			end := geom.Vec2{X: in.End.X - in.Center.X, Y: in.End.Y - in.Center.Y}
			s.StartAngle = math.Atan2(start.Y, start.X)
			s.Sweep = math.Atan2(start.X*end.Y-start.Y*end.X, start.X*end.X+start.Y*end.Y)
		}
	}
	return trimmed.ArcLoopBounds(frame, segments, d.Height*scale)
}

package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type throughProfileStock struct {
	frame              geom.Frame
	low, high          float64
	lowerCap, upperCap int
}

func (a *analyticBody) throughProfile() (throughProfileStock, bool) {
	if a != nil && a.rounded != nil {
		p := a.rounded
		return throughProfileStock{p.frame, min(0, p.height), max(0, p.height), 0, 1}, cardinalFrame(p.frame)
	}
	if a != nil && a.annular != nil {
		p := a.annular
		return throughProfileStock{p.Frame, 0, p.Height, p.lowerFace, p.upperFace}, cardinalFrame(p.Frame)
	}
	return throughProfileStock{}, false
}

func (p throughProfileStock) clipTool(ctx context.Context, tool stored) (trimmed.Solid, float64, float64, error) {
	n := p.frame.ZAxis()
	project := func(origin, axis geom.Vec3, height float64) (geom.Vec3, bool) {
		if axis != n && axis != n.Scale(-1) {
			return geom.Vec3{}, false
		}
		start := p.frame.ToLocal(origin)
		end := start.Z + height*axis.Dot(n)
		return start, p.frame.ToWorld(start) == origin && min(start.Z, end) <= p.low && max(start.Z, end) >= p.high
	}
	h := p.high - p.low
	frame, err := geom.NewFrame(p.frame.ToWorld(geom.Vec3{Z: p.low}), p.frame.XAxis(), n)
	if err != nil {
		return trimmed.Solid{}, 0, 0, err
	}
	if c := tool.cylinder; c != nil {
		start, ok := project(c.frame.Origin(), c.frame.ZAxis(), c.height)
		if !ok {
			return trimmed.Solid{}, 0, 0, geom.ErrDomain
		}
		center := p.frame.ToWorld(geom.Vec3{X: start.X, Y: start.Y, Z: p.low})
		frame, err := geom.NewFrame(center, p.frame.XAxis(), n)
		if err != nil {
			return trimmed.Solid{}, 0, 0, err
		}
		area, perimeter := math.Pi*c.radius*c.radius, 2*math.Pi*c.radius
		geometry, err := trimmed.ExtrudeDiskWithHoles(ctx, frame,
			trimmed.ExtrusionCircle{Radius: c.radius}, nil, nil, h)
		return geometry, area, perimeter, err
	}
	if tool.analytic != nil && tool.analytic.rounded != nil {
		c := tool.analytic.rounded
		start, ok := project(c.frame.Origin(), c.frame.ZAxis(), c.height)
		if !ok || c.cut || c.fillet != 0 || c.frame.XAxis() != frame.XAxis() || c.frame.YAxis() != frame.YAxis() {
			return trimmed.Solid{}, 0, 0, geom.ErrDomain
		}
		profile := c.profile
		profile.Min.X += start.X
		profile.Min.Y += start.Y
		profile.Max.X += start.X
		profile.Max.Y += start.Y
		w, d := profile.Max.X-profile.Min.X, profile.Max.Y-profile.Min.Y
		area, perimeter := w*d, 2*(w+d)
		for _, radius := range profile.Radii {
			area -= (1 - math.Pi/4) * radius * radius
			perimeter += (math.Pi/2 - 2) * radius
		}
		geometry, err := trimmed.ExtrudeRoundedRectangle(ctx, frame, profile, nil, nil, h)
		return geometry, area, perimeter, err
	}
	return trimmed.Solid{}, 0, 0, geom.ErrDomain
}

func cutThroughProfiles(a stored, tools []stored) (*analyticBody, float64, bool, error) {
	p, ok := a.analytic.throughProfile()
	if !ok {
		return nil, 0, false, nil
	}
	result := *a.analytic
	ctx := context.Background()
	removed := 0.
	for _, tool := range tools {
		upperCap := p.upperCap
		if result.rounded != nil && result.rounded.pedestal != nil {
			var err error
			upperCap, err = result.rounded.pedestal.cutCap(p, tool)
			if err != nil {
				return nil, 0, true, err
			}
		}
		clipped, area, perimeter, err := p.clipTool(ctx, tool)
		if err != nil {
			return nil, 0, true, err
		}
		result.solid, err = trimmed.SewPlanarThroughPrism(ctx, result.solid, clipped, p.lowerCap, upperCap, 0, 1)
		if err != nil {
			return nil, 0, true, err
		}
		height := p.high - p.low
		areaDelta := perimeter*height - 2*area
		if result.rounded != nil && result.rounded.stack != nil && result.rounded.stack.bores != nil {
			recipe, stack := *result.rounded, *result.rounded.stack
			before, err := trimmed.SewPlanarThroughPrism(ctx, stack.bores.base.solid, clipped, p.lowerCap, upperCap, 0, 1)
			if err != nil {
				return nil, 0, true, err
			}
			stack.bores = stack.bores.withBase(before, -area*height, areaDelta)
			recipe.stack = &stack
			result.rounded = &recipe
		}
		removed += area * height
		result.area += areaDelta
	}
	result.volume -= removed
	if result.rounded != nil {
		recipe := *result.rounded
		recipe.cut = true
		result.rounded = &recipe
	}
	if result.annular != nil {
		recipe := *result.annular
		recipe.cut = true
		result.annular = &recipe
	}
	return &result, removed, true, nil
}

func (a pedestalStock) cutCap(p throughProfileStock, tool stored) (int, error) {
	c := tool.cylinder
	if c == nil {
		return 0, geom.ErrDomain
	}
	center := p.frame.ToLocal(c.frame.Origin())
	if p.frame.ToWorld(center) != c.frame.Origin() {
		return 0, geom.ErrUnrepresentable
	}
	exact := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dx := new(big.Rat).Sub(exact(center.X), exact(a.center.X))
	dy := new(big.Rat).Sub(exact(center.Y), exact(a.center.Y))
	distance2 := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
	gap := new(big.Rat).Sub(exact(a.inner), exact(c.radius))
	if gap.Sign() > 0 && distance2.Cmp(gap.Mul(gap, gap)) < 0 {
		return a.centerFace, nil
	}
	reach := new(big.Rat).Add(exact(a.outer), exact(c.radius))
	if distance2.Cmp(reach.Mul(reach, reach)) > 0 {
		return p.upperCap, nil
	}
	return 0, geom.ErrDomain
}

package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type roundedStack struct {
	top, floor       float64
	upper, pocketCap int
	bores            *roundedBores
}

func roundedToolExtent(base, tool *roundedStock) (trimmed.RoundedRectangle, float64, float64, bool) {
	if base.frame.XAxis() != tool.frame.XAxis() || base.frame.YAxis() != tool.frame.YAxis() ||
		!cardinalFrame(base.frame) || tool.cut || tool.stack != nil || tool.pedestal != nil || tool.chamfer != 0 || tool.fillet != 0 {
		return trimmed.RoundedRectangle{}, 0, 0, false
	}
	origin := base.frame.ToLocal(tool.frame.Origin())
	if base.frame.ToWorld(origin) != tool.frame.Origin() {
		return trimmed.RoundedRectangle{}, 0, 0, false
	}
	profile := tool.profile
	profile.Min.X += origin.X
	profile.Max.X += origin.X
	profile.Min.Y += origin.Y
	profile.Max.Y += origin.Y
	end := origin.Z + tool.height
	if !finite(end) {
		return trimmed.RoundedRectangle{}, 0, 0, false
	}
	return profile, min(origin.Z, end), max(origin.Z, end), true
}

func roundedMeasures(p trimmed.RoundedRectangle) (float64, float64) {
	w, d := p.Max.X-p.Min.X, p.Max.Y-p.Min.Y
	area, perimeter := w*d, 2*(w+d)
	for _, radius := range p.Radii {
		area -= (1 - math.Pi/4) * radius * radius
		perimeter += (math.Pi/2 - 2) * radius
	}
	return area, perimeter
}

func buildRoundedLayer(ctx context.Context, frame geom.Frame, p trimmed.RoundedRectangle, lo, hi float64) (trimmed.Solid, error) {
	f, err := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: lo}), frame.XAxis(), frame.ZAxis())
	if err != nil {
		return trimmed.Solid{}, err
	}
	return trimmed.ExtrudeRoundedRectangle(ctx, f, p, nil, nil, hi-lo)
}

func (b *Bridge) fuseRoundedStack(a, tool stored) (Shape, bool, error) {
	if a.analytic == nil || a.analytic.rounded == nil || tool.analytic == nil || tool.analytic.rounded == nil {
		return Shape{}, false, nil
	}
	base := *a.analytic.rounded
	if base.height <= 0 || base.cut || base.stack != nil || base.pedestal != nil {
		return Shape{}, false, nil
	}
	profile, lo, hi, ok := roundedToolExtent(&base, tool.analytic.rounded)
	if !ok || lo > base.height || lo < base.chamfer || hi <= base.height {
		return Shape{}, false, nil
	}
	ctx := context.Background()
	layer, err := buildRoundedLayer(ctx, base.frame, profile, base.height, hi)
	if err != nil {
		return Shape{}, true, err
	}
	solid, upper, err := trimmed.SewPlanarCapPrism(ctx, a.analytic.solid, layer, 1, 0, 1, false)
	if err != nil {
		return Shape{}, true, err
	}
	base.stack = &roundedStack{top: hi, upper: upper, pocketCap: -1}
	area, perimeter := roundedMeasures(profile)
	h := hi - base.height
	out, err := b.putAnalyticBody(&analyticBody{solid: solid, rounded: &base,
		volume: a.analytic.volume + area*h, area: a.analytic.area + perimeter*h})
	return out, true, err
}

func (b *Bridge) cutRoundedStack(a, tool stored) (Shape, bool, error) {
	if a.analytic == nil || a.analytic.rounded == nil || a.analytic.rounded.stack == nil ||
		tool.analytic == nil || tool.analytic.rounded == nil {
		return Shape{}, false, nil
	}
	base := *a.analytic.rounded
	stack := *base.stack
	profile, lo, hi, ok := roundedToolExtent(&base, tool.analytic.rounded)
	if !ok || lo <= base.height || lo >= stack.top || hi < stack.top {
		return Shape{}, false, nil
	}
	ctx := context.Background()
	layer, err := buildRoundedLayer(ctx, base.frame, profile, lo, stack.top)
	if err != nil {
		return Shape{}, true, err
	}
	solid, floor, err := trimmed.SewPlanarCapPrism(ctx, a.analytic.solid, layer, stack.upper, 0, 1, true)
	if err != nil {
		return Shape{}, true, err
	}
	area, perimeter := roundedMeasures(profile)
	h := stack.top - lo
	if stack.bores != nil {
		before, _, err := trimmed.SewPlanarCapPrism(ctx, stack.bores.base.solid, layer, stack.upper, 0, 1, true)
		if err != nil {
			return Shape{}, true, err
		}
		stack.bores = stack.bores.withBase(before, -area*h, perimeter*h)
	}
	if stack.pocketCap < 0 {
		stack.floor, stack.pocketCap = lo, floor
	}
	base.stack = &stack
	out, err := b.putAnalyticBody(&analyticBody{solid: solid, rounded: &base,
		volume: a.analytic.volume - area*h, area: a.analytic.area + perimeter*h})
	return out, true, err
}

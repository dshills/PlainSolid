package plaincadbridge

import (
	"context"
	"errors"
	"slices"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const sideOpeningTolerance = 1e-9

func sideOpeningStockFromNotch(p step.SideNotchBody) (step.SideOpeningCutsBody, bool) {
	if p.Left < 0 || len(p.Through)+len(p.Blind) != 0 {
		return step.SideOpeningCutsBody{}, false
	}
	f, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{X: -p.Left, Y: -p.Inset}), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return step.SideOpeningCutsBody{}, false
	}
	return step.SideOpeningCutsBody{
		Name: p.Name, Frame: f, Width: p.Left + p.Right, Back: p.Back + p.Inset, Length: p.Length,
		Cuts: []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{
			Center: geom.Vec2{X: p.Left, Y: p.Inset}, Radius: p.Radius,
		}, Depth: p.Depth}},
	}, true
}

func sideOpeningCylinderSpan(p step.SideOpeningCutsBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func cornerOpeningStock(stock brep.Solid, tool *cylinderBody) (step.SideOpeningCutsBody, bool) {
	var p step.SideOpeningCutsBody
	vertices := stock.Vertices()
	if tool == nil || !cardinalFrame(tool.frame) || len(vertices) != 8 ||
		len(stock.Edges()) != 12 || len(stock.Faces()) != 6 {
		return p, false
	}
	for i, v := range vertices {
		vertices[i] = tool.frame.ToLocal(v)
		if tool.frame.ToWorld(vertices[i]) != v {
			return p, false
		}
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil || !rectangularFaces(stock, tool.frame, bounds.Min, bounds.Max) {
		return p, false
	}
	size := bounds.Max.Sub(bounds.Min)
	origin := tool.frame.ToWorld(bounds.Min)
	frame, err := geom.NewFrame(origin, tool.frame.XAxis(), tool.frame.ZAxis())
	if err != nil || tool.frame.ToLocal(origin) != bounds.Min || frame.YAxis() != tool.frame.YAxis() ||
		frame.ToWorld(size) != tool.frame.ToWorld(bounds.Max) {
		return p, false
	}
	p = step.SideOpeningCutsBody{Name: "PlainSolid", Frame: frame, Width: size.X, Back: size.Y, Length: size.Z}
	circle, low, high, ok := sideOpeningCylinderSpan(p, tool)
	if !ok || high <= low || trimmed.BoxSideOpeningCount(p.Width, p.Back, circle, sideOpeningTolerance) != 2 ||
		!exactAxisDistance(bounds.Max.X, bounds.Min.X, size.X) || !exactAxisDistance(bounds.Max.Y, bounds.Min.Y, size.Y) ||
		!exactAxisDistance(bounds.Max.Z, bounds.Min.Z, size.Z) {
		return step.SideOpeningCutsBody{}, false
	}
	return p, true
}

func exactStockCorner(p step.SideOpeningCutsBody, circle trimmed.ExtrusionCircle) bool {
	x, y := circle.Center.X, circle.Center.Y
	return (x == 0 || x == p.Width) && (y == 0 || y == p.Back)
}

// sideOpeningStockFromDualDisk recovers the original analytic stock and
// cutters after a strict equal-depth pair. A later group is rebuilt from
// those descriptors just as it would be if all tools were supplied together.
func sideOpeningStockFromDualDisk(p *sideDualDiskBody) (step.SideOpeningCutsBody, bool) {
	f, e := geom.NewFrame(p.frame.ToWorld(geom.Vec3{X: p.min.X, Y: p.min.Y}), p.frame.XAxis(), p.frame.ZAxis())
	if e != nil {
		return step.SideOpeningCutsBody{}, false
	}
	cuts := make([]trimmed.SideOpeningCut, len(p.discs))
	for i, disc := range p.discs {
		disc.Center.X -= p.min.X
		disc.Center.Y -= p.min.Y
		cuts[i] = trimmed.SideOpeningCut{ExtrusionCircle: disc, Depth: p.depth}
	}
	return step.SideOpeningCutsBody{Name: "PlainSolid", Frame: f, Width: p.max.X - p.min.X, Back: p.max.Y - p.min.Y, Length: p.height, Cuts: cuts}, true
}

// Keep the precise equal-depth family when it can represent the exact
// section. Its narrower representability gate can reject an irrational
// circle intersection which the analytic two-opening path proves directly.
// Retry only that exact representability failure and publish at most one body.
func (b *Bridge) cutSideOpeningFamilies(a stored, tools []stored) (Shape, bool, error) {
	first, handled, err := b.cutSideDualDisk(a, tools)
	if handled && !errors.Is(err, geom.ErrUnrepresentable) {
		return first, true, err
	}
	if result, ok, second := b.cutSideOpeningCuts(a, tools); ok {
		return result, true, second
	}
	return first, handled, err
}

func (b *Bridge) cutSideOpeningCuts(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideOpeningCutsBody
	var skip *cylinderBody
	switch {
	case a.analytic != nil && a.analytic.sideOpeningCuts != nil:
		p = *a.analytic.sideOpeningCuts
		p.Cuts = slices.Clone(p.Cuts)
	case a.analytic != nil && a.analytic.sideDualDisk != nil:
		var ok bool
		p, ok = sideOpeningStockFromDualDisk(a.analytic.sideDualDisk)
		if !ok {
			return Shape{}, false, nil
		}
	case a.analytic != nil && a.analytic.sideNotch != nil:
		var ok bool
		p, ok = sideOpeningStockFromNotch(*a.analytic.sideNotch)
		if !ok {
			return Shape{}, false, nil
		}
	case a.planar():
		if len(tools) >= 2 {
			for _, input := range tools {
				c := input.cylinder
				if f, box, ok := sideCylinderBlindRecipe(a.solid, c); ok {
					p, _ = sideOpeningStockFromNotch(sideNotchRecipe(f, box, c.radius, c.height))
					skip = c
					break
				}
				if f, box, ok := sideCylinderThroughRecipe(a.solid, c); ok {
					p, _ = sideOpeningStockFromNotch(sideNotchRecipe(f, box, c.radius, box.Max.Z))
					skip = c
					break
				}
			}
		}
		if p.Length == 0 {
			for _, input := range tools {
				if candidate, ok := cornerOpeningStock(a.solid, input.cylinder); ok {
					p = candidate
					break
				}
			}
		}
		if p.Length == 0 {
			return Shape{}, false, nil
		}
	default:
		return Shape{}, false, nil
	}
	spans := make([][][2]float64, len(p.Cuts))
	for i, cut := range p.Cuts {
		if cut.OppositeCap {
			spans[i] = append(spans[i], [2]float64{p.Length - cut.Depth, p.Length})
		} else {
			spans[i] = append(spans[i], [2]float64{0, cut.Depth})
		}
	}
	for _, input := range tools {
		c := input.cylinder
		if c == nil {
			return Shape{}, false, nil
		}
		if c == skip || boundsSeparated(a.result.Bounds, input.result.Bounds) {
			continue
		}
		circle, low, high, ok := sideOpeningCylinderSpan(p, c)
		if !ok || high <= low || trimmed.BoxSideOpeningCount(p.Width, p.Back, circle, sideOpeningTolerance) == 0 {
			return Shape{}, false, nil
		}
		matched := false
		for i := range p.Cuts {
			if p.Cuts[i].ExtrusionCircle == circle {
				spans[i] = append(spans[i], [2]float64{low, high})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if len(p.Cuts) == trimmed.MaxSideOpeningCuts {
			return Shape{}, true, &failureError{"LIMIT", "Too many distinct side-opening Hole cutters"}
		}
		p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: circle})
		spans = append(spans, [][2]float64{{low, high}})
	}
	if len(p.Cuts) == 0 {
		return Shape{}, false, nil
	}
	if len(p.Cuts) == 1 {
		circle := p.Cuts[0].ExtrusionCircle
		if a.analytic != nil && a.analytic.sideNotch != nil ||
			a.planar() && (skip != nil || exactStockCorner(p, circle)) {
			return Shape{}, false, nil
		}
	}
	for i, site := range spans {
		sort.Slice(site, func(a, b int) bool { return site[a][0] < site[b][0] })
		low, high := site[0][0], site[0][1]
		for _, span := range site[1:] {
			if span[0] > high {
				return Shape{}, true, unsupportedEdge("opposed cuts at a side-opening site must connect")
			}
			high = max(high, span[1])
		}
		if low != 0 && high != p.Length {
			return Shape{}, true, unsupportedEdge("side-opening cuts must open on an exterior cap")
		}
		if !exactAxisDistance(high, low, high-low) {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		p.Cuts[i].Depth, p.Cuts[i].OppositeCap = high-low, low != 0
	}
	body, err := sideOpeningCutsBody(p)
	if err != nil {
		return Shape{}, true, err
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, body.volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, err := b.putAnalyticBody(body)
	return out, true, err
}

func sideOpeningCutsBody(p step.SideOpeningCutsBody) (*analyticBody, error) {
	result, err := trimmed.BoxSideOpeningCuts(context.Background(), p.Frame, p.Width, p.Back, p.Length, p.Cuts, sideOpeningTolerance)
	if err != nil {
		return nil, err
	}
	p.Cuts = result.RetainedCuts
	var corners []geom.Vec3
	for _, x := range []float64{0, p.Width} {
		for _, y := range []float64{0, p.Back} {
			for _, z := range []float64{0, p.Length} {
				corners = append(corners, p.Frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z}))
			}
		}
	}
	return &analyticBody{solid: result.Solid, sideOpeningCuts: &p, polygonExtrema: corners,
		volume: p.Width*p.Back*p.Length - result.RemovedVolume,
		area:   2*(p.Width*p.Back+(p.Width+p.Back)*p.Length) + result.SurfaceAreaDelta}, nil
}

func overlapSideOpeningCuts(a, tool stored) (float64, bool, error) {
	if a.analytic == nil || tool.cylinder == nil {
		return 0, false, nil
	}
	var p step.SideOpeningCutsBody
	if a.analytic.sideOpeningCuts != nil {
		p = *a.analytic.sideOpeningCuts
	} else if a.analytic.sideDualDisk != nil {
		var ok bool
		p, ok = sideOpeningStockFromDualDisk(a.analytic.sideDualDisk)
		if !ok {
			return 0, false, nil
		}
	} else if a.analytic.sideNotch != nil {
		var ok bool
		p, ok = sideOpeningStockFromNotch(*a.analytic.sideNotch)
		if !ok {
			return 0, false, nil
		}
	} else {
		return 0, false, nil
	}
	c, low, high, ok := sideOpeningCylinderSpan(p, tool.cylinder)
	if !ok || high <= low || trimmed.BoxSideOpeningCount(p.Width, p.Back, c, sideOpeningTolerance) == 0 {
		return 0, false, nil
	}
	if a.analytic.sideOpeningCuts == nil && a.analytic.sideDualDisk == nil && c == p.Cuts[0].ExtrusionCircle {
		return 0, false, nil
	}
	volume, err := trimmed.BoxSideOpeningCutsOverlap(p.Frame, p.Width, p.Back, p.Length, p.Cuts, c, low, high, sideOpeningTolerance)
	return volume, true, err
}

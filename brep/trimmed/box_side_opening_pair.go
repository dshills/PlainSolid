package trimmed

import (
	"context"
	"math"
	"slices"
	"sort"

	"plainsolid/geom"
)

// SideOpeningCut removes a cylindrical footprint from the selected end cap.
type SideOpeningCut struct {
	ExtrusionCircle
	Depth       float64
	OppositeCap bool
}

type SideOpeningPairResult struct {
	Solid                           Solid
	RemovedVolume, SurfaceAreaDelta float64
}

// BoxSideOpeningPair removes two transversely intersecting disks. Each disk
// crosses one stock side and clears the other three. Stock is [0,width] ×
// [0,back] × [0,length] in frame. Depth equal to length cuts through.
func BoxSideOpeningPair(ctx context.Context, frame geom.Frame, width, back, length float64, cuts [2]SideOpeningCut, tolerance float64) (SideOpeningPairResult, error) {
	var out SideOpeningPairResult
	if ctx == nil || !finite(length) || !finite(tolerance) || tolerance <= 0 || length <= 8*tolerance {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	for _, cut := range cuts {
		if !finite(cut.Depth) || cut.Depth <= 8*tolerance || cut.Depth > length || cut.Depth < length && length-cut.Depth <= 8*tolerance {
			return out, geom.ErrDomain
		}
	}
	if sideCutLess(cuts[1].ExtrusionCircle, cuts[0].ExtrusionCircle) {
		cuts[0], cuts[1] = cuts[1], cuts[0]
	}
	points, edges, err := sidePairArrangement(frame, width, back, [2]ExtrusionCircle{cuts[0].ExtrusionCircle, cuts[1].ExtrusionCircle}, tolerance)
	if err != nil {
		return out, err
	}
	builder := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	material := func(mask, active uint8) bool { return mask&sidePairStock != 0 && mask&active == 0 }
	cap := func(z float64, keep func(uint8) bool, reversed bool) error {
		return buildSidePairCap(&builder, points, edges, z, keep, reversed)
	}
	var spans [2][2]float64
	levels := []float64{0, length}
	for i, cut := range cuts {
		spans[i] = sideOpeningSpan(length, cut)
		levels = append(levels, spans[i][0], spans[i][1])
	}
	// A common opposed floor puts both transverse wall junctions at the
	// same axial level, where the current slab proof cannot separate them.
	if spans[0][1] == spans[1][0] || spans[1][1] == spans[0][0] {
		return out, geom.ErrUnrepresentable
	}
	sort.Float64s(levels)
	levels = slices.Compact(levels)
	activeIn := func(bottom, top float64) uint8 {
		var active uint8
		for i, span := range spans {
			if span[0] <= bottom && top <= span[1] {
				active |= 1 << i
			}
		}
		return active
	}
	bottomActive := activeIn(0, levels[1])
	if err := cap(0, func(mask uint8) bool { return material(mask, bottomActive) }, true); err != nil {
		return out, err
	}
	topActive := activeIn(levels[len(levels)-2], length)
	if err := cap(length, func(mask uint8) bool { return material(mask, topActive) }, false); err != nil {
		return out, err
	}
	out.SurfaceAreaDelta = -sidePairRemovedArea(points, edges, bottomActive) - sidePairRemovedArea(points, edges, topActive)
	for i, top := range levels[1:] {
		bottom := levels[i]
		active := activeIn(bottom, top)
		thickness := top - bottom
		if thickness <= 8*tolerance {
			return SideOpeningPairResult{}, geom.ErrDomain
		}
		out.RemovedVolume += sidePairRemovedArea(points, edges, active) * thickness
		out.SurfaceAreaDelta -= 2 * (width + back) * thickness
		for _, edge := range edges {
			left, right := material(edge.left, active), material(edge.right, active)
			if left == right {
				continue
			}
			if !left {
				edge = edge.reversed()
			}
			if err := buildSidePairWall(&builder, points, edge, bottom, top); err != nil {
				return SideOpeningPairResult{}, err
			}
			out.SurfaceAreaDelta += edge.length(points) * thickness
		}
		if top < length {
			next := activeIn(top, levels[i+2])
			floors := func(mask uint8) bool { return material(mask, next) && !material(mask, active) }
			ceilings := func(mask uint8) bool { return material(mask, active) && !material(mask, next) }
			if err := cap(top, floors, true); err != nil {
				return SideOpeningPairResult{}, err
			}
			if err := cap(top, ceilings, false); err != nil {
				return SideOpeningPairResult{}, err
			}
			out.SurfaceAreaDelta += 2*sidePairRemovedArea(points, edges, active|next) -
				sidePairRemovedArea(points, edges, active) - sidePairRemovedArea(points, edges, next)
		}
	}
	out.Solid, err = NewSolidContext(ctx, builder.vertices, builder.edges, builder.faces, tolerance)
	if err != nil {
		return SideOpeningPairResult{}, err
	}
	if len(out.Solid.Topology().Shells) != 1 {
		return SideOpeningPairResult{}, geom.ErrUnrepresentable
	}
	return out, nil
}

func sideOpeningSpan(length float64, cut SideOpeningCut) [2]float64 {
	if cut.OppositeCap {
		return [2]float64{length - cut.Depth, length}
	}
	return [2]float64{0, cut.Depth}
}

const sidePairStock = 4

type sidePairEdge struct {
	start, end  int
	left, right uint8
	circle      ExtrusionCircle
	from, to    float64
}

func (e sidePairEdge) reversed() sidePairEdge {
	e.start, e.end = e.end, e.start
	e.from, e.to = e.to, e.from
	e.left, e.right = e.right, e.left
	return e
}

func (e sidePairEdge) length(points []geom.Vec2) float64 {
	if e.circle.Radius != 0 {
		return e.circle.Radius * math.Abs(e.to-e.from)
	}
	p, q := points[e.start], points[e.end]
	return math.Hypot(q.X-p.X, q.Y-p.Y)
}

func sidePairRemovedArea(points []geom.Vec2, edges []sidePairEdge, active uint8) float64 {
	removed := func(mask uint8) bool { return mask&sidePairStock != 0 && mask&active != 0 }
	area := 0.
	for _, edge := range edges {
		left, right := removed(edge.left), removed(edge.right)
		if left == right {
			continue
		}
		if !left {
			edge = edge.reversed()
		}
		p, q := points[edge.start], points[edge.end]
		if c := edge.circle; c.Radius != 0 {
			area += (c.Center.X*(q.Y-p.Y) - c.Center.Y*(q.X-p.X) + c.Radius*c.Radius*(edge.to-edge.from)) / 2
		} else {
			area += (p.X*q.Y - p.Y*q.X) / 2
		}
	}
	return area
}

func sideCutLess(a, b ExtrusionCircle) bool {
	if a.Center.X != b.Center.X {
		return a.Center.X < b.Center.X
	}
	if a.Center.Y != b.Center.Y {
		return a.Center.Y < b.Center.Y
	}
	return a.Radius < b.Radius
}

func sidePairArrangement(frame geom.Frame, width, back float64, circles [2]ExtrusionCircle, tol float64) ([]geom.Vec2, []sidePairEdge, error) {
	if !finite(width) || !finite(back) || width <= 0 || back <= 0 {
		return nil, nil, geom.ErrInvalid
	}
	fail := func() ([]geom.Vec2, []sidePairEdge, error) { return nil, nil, geom.ErrDomain }
	pad := 8 * tol
	distances := func(p geom.Vec2) [4]float64 {
		return [4]float64{p.Y, width - p.X, back - p.Y, p.X}
	}
	var opening [2]int
	var sideOffsets [2]float64
	for i, c := range circles {
		if !uvFinite(c.Center) || !finite(c.Radius) || c.Radius <= pad {
			return fail()
		}
		count := 0
		for j, distance := range distances(c.Center) {
			if distance-c.Radius > pad {
				continue
			}
			if c.Radius-math.Abs(distance) <= pad {
				return fail()
			}
			opening[i], sideOffsets[i] = j, distance
			count++
		}
		if count != 1 {
			return fail()
		}
	}
	first, second := circles[0], circles[1]
	a := unionDisk{center: first.Center, radius: first.Radius}
	b := unionDisk{center: second.Center, radius: second.Radius}
	if circleRelation(a, b, true) >= 0 || circleRelation(a, b, false) <= 0 {
		return fail()
	}
	dx, dy := second.Center.X-first.Center.X, second.Center.Y-first.Center.Y
	d := math.Hypot(dx, dy)
	r, s := first.Radius, second.Radius
	if min(r+s-d, d-math.Abs(r-s)) <= pad {
		return fail()
	}
	cosine := (d + (r-s)*(r+s)/d) / (2 * r)
	along, cross := r*cosine, r*math.Sqrt((1-cosine)*(1+cosine))
	if !finite(cross) || cross <= pad {
		return fail()
	}
	points := []geom.Vec2{{}, {X: width}, {X: width, Y: back}, {Y: back}}
	type event struct {
		id    int
		angle float64
	}
	var events [2][]event
	add := func(i, id int) {
		c, p := circles[i], points[id]
		events[i] = append(events[i], event{id, math.Atan2(p.Y-c.Center.Y, p.X-c.Center.X)})
	}
	for _, sign := range []float64{-1, 1} {
		p := geom.Vec2{X: first.Center.X + (along*dx-sign*cross*dy)/d, Y: first.Center.Y + (along*dy+sign*cross*dx)/d}
		if !uvFinite(p) {
			return fail()
		}
		for _, distance := range distances(p) {
			if math.Abs(distance) <= pad {
				return fail()
			}
		}
		id := len(points)
		points = append(points, p)
		add(0, id)
		add(1, id)
	}
	sides := [4][]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	for i, c := range circles {
		j, offset := opening[i], sideOffsets[i]
		root := math.Sqrt((c.Radius - offset) * (c.Radius + offset))
		for _, delta := range []float64{-root, root} {
			var p geom.Vec2
			switch j {
			case 0:
				p.X = c.Center.X + delta
			case 1:
				p = geom.Vec2{X: width, Y: c.Center.Y + delta}
			case 2:
				p = geom.Vec2{X: c.Center.X + delta, Y: back}
			case 3:
				p.Y = c.Center.Y + delta
			}
			id := len(points)
			points = append(points, p)
			sides[j] = append(sides[j], id)
			add(i, id)
		}
	}
	classify := func(p geom.Vec2, circle ExtrusionCircle) (bool, bool) {
		distance := math.Hypot(p.X-circle.Center.X, p.Y-circle.Center.Y)
		return distance < circle.Radius, math.Abs(distance-circle.Radius) > tol
	}
	var edges []sidePairEdge
	for j, side := range sides {
		if len(side) == 2 {
			edges = append(edges, sidePairEdge{start: side[0], end: side[1], left: sidePairStock})
			continue
		}
		origin, finish := points[side[0]], points[side[1]]
		coordinate := func(p geom.Vec2) float64 {
			switch j {
			case 0:
				return p.X
			case 1:
				return p.Y
			case 2:
				return -p.X
			default:
				return -p.Y
			}
		}
		sort.Slice(side, func(a, b int) bool { return coordinate(points[side[a]]) < coordinate(points[side[b]]) })
		if points[side[0]] != origin || points[side[len(side)-1]] != finish {
			return fail()
		}
		for i, start := range side[:len(side)-1] {
			end := side[i+1]
			p, q := points[start], points[end]
			if coordinate(q)-coordinate(p) <= pad {
				return fail()
			}
			mid := geom.Vec2{X: p.X + (q.X-p.X)/2, Y: p.Y + (q.Y-p.Y)/2}
			mask := uint8(sidePairStock)
			for k, circle := range circles {
				inside, clear := classify(mid, circle)
				if !clear {
					return fail()
				}
				if inside {
					mask |= 1 << k
				}
			}
			edges = append(edges, sidePairEdge{start: start, end: end, left: mask})
		}
	}
	for i, ev := range events {
		sort.Slice(ev, func(j, k int) bool { return ev[j].angle < ev[k].angle })
		c := circles[i]
		for j, start := range ev {
			end := ev[(j+1)%len(ev)]
			if j == len(ev)-1 {
				end.angle += 2 * math.Pi
			}
			sweep := end.angle - start.angle
			if sweep*c.Radius <= pad {
				return fail()
			}
			mid := arcLoopPoint(ExtrusionSegment{Center: c.Center, Radius: c.Radius}, start.angle+sweep/2)
			inward := distances(mid)
			distance := min(inward[0], inward[1], inward[2], inward[3])
			if math.Abs(distance) <= tol {
				return fail()
			}
			if distance < 0 {
				continue
			}
			mask := uint8(sidePairStock)
			inside, clear := classify(mid, circles[1-i])
			if !clear {
				return fail()
			}
			if inside {
				mask |= 1 << (1 - i)
			}
			n := int(math.Ceil(sweep / (math.Pi / 4)))
			current := start.id
			for k := 0; k < n; k++ {
				from, to := start.angle+sweep*float64(k)/float64(n), start.angle+sweep*float64(k+1)/float64(n)
				next := end.id
				if k+1 < n {
					next = len(points)
					points = append(points, arcLoopPoint(ExtrusionSegment{Center: c.Center, Radius: c.Radius}, to))
				}
				edges = append(edges, sidePairEdge{current, next, mask | 1<<i, mask, c, from, to})
				current = next
			}
		}
	}
	for i, p := range points {
		q := frame.ToLocal(frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y}))
		if q.Z != 0 || math.Hypot(q.X-p.X, q.Y-p.Y) > tol/8 {
			return nil, nil, geom.ErrUnrepresentable
		}
		points[i] = geom.Vec2{X: q.X, Y: q.Y}
	}
	return points, edges, nil
}

func buildSidePairCap(b *cornerCylinderBuilder, points []geom.Vec2, edges []sidePairEdge, z float64, keep func(uint8) bool, reversed bool) error {
	next := make(map[int]sidePairEdge)
	var starts []int
	for _, edge := range edges {
		left, right := keep(edge.left), keep(edge.right)
		if left == right {
			continue
		}
		if !left {
			edge = edge.reversed()
		}
		if _, exists := next[edge.start]; exists {
			return geom.ErrUnrepresentable
		}
		next[edge.start] = edge
		starts = append(starts, edge.start)
	}
	f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{Z: z}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	surface, err := geom.NewPlaneSurface(f)
	if err != nil {
		return err
	}
	for _, start := range starts {
		if _, exists := next[start]; !exists {
			continue
		}
		var world []geom.Vec3
		var uv []geom.Vec2
		curves := make(map[int]PCurve)
		id := start
		for {
			edge, exists := next[id]
			if !exists {
				return geom.ErrUnrepresentable
			}
			delete(next, id)
			p, q := points[edge.start], points[edge.end]
			if c := edge.circle; c.Radius != 0 {
				curves[len(uv)] = unionArcUV{
					EllipseUV: EllipseUV{Center: c.Center, Cos: geom.Vec2{X: c.Radius}, Sin: geom.Vec2{Y: c.Radius}, From: edge.from, To: edge.to},
					Start:     p, End: q,
				}
			}
			uv = append(uv, p)
			world = append(world, geom.Vec3{X: p.X, Y: p.Y, Z: z})
			id = edge.end
			if id == start {
				break
			}
		}
		if len(uv) < 3 {
			return geom.ErrUnrepresentable
		}
		if err := b.face(surface, world, uv, curves, reversed); err != nil {
			return err
		}
	}
	return nil
}

func buildSidePairWall(b *cornerCylinderBuilder, points []geom.Vec2, edge sidePairEdge, bottom, top float64) error {
	p, q := points[edge.start], points[edge.end]
	circle := edge.circle
	if circle.Radius == 0 {
		return b.wall(p, q, bottom, top)
	}
	f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{X: circle.Center.X, Y: circle.Center.Y}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	surface, err := geom.NewCylinderSurface(f, circle.Radius)
	if err != nil {
		return err
	}
	reversed := edge.to < edge.from
	if reversed {
		p, q = q, p
		edge.from, edge.to = edge.to, edge.from
	}
	world := []geom.Vec3{{X: p.X, Y: p.Y, Z: bottom}, {X: q.X, Y: q.Y, Z: bottom}, {X: q.X, Y: q.Y, Z: top}, {X: p.X, Y: p.Y, Z: top}}
	uv := []geom.Vec2{{X: edge.from, Y: bottom}, {X: edge.to, Y: bottom}, {X: edge.to, Y: top}, {X: edge.from, Y: top}}
	return b.face(surface, world, uv, nil, reversed)
}

// BoxSideOpeningPairOverlap measures the remaining stock in a tool's
// footprint and finite axial interval after a previous side-opening cut.
func BoxSideOpeningPairOverlap(frame geom.Frame, width, back, length float64, old SideOpeningCut, tool ExtrusionCircle, low, high, tolerance float64) (float64, error) {
	if !finite(tolerance) || tolerance <= 0 || !finite(length) || !finite(old.Depth) || old.Depth <= 0 || old.Depth > length ||
		!finite(low) || !finite(high) || low < 0 || high <= low || high > length {
		return 0, geom.ErrInvalid
	}
	points, edges, err := sidePairArrangement(frame, width, back, [2]ExtrusionCircle{old.ExtrusionCircle, tool}, tolerance)
	if err != nil {
		return 0, err
	}
	oldArea := sidePairRemovedArea(points, edges, 1)
	toolArea := sidePairRemovedArea(points, edges, 2)
	unionArea := sidePairRemovedArea(points, edges, 3)
	span := sideOpeningSpan(length, old)
	shared := max(0, min(span[1], high)-max(span[0], low))
	return max(0, toolArea*(high-low)-(oldArea+toolArea-unionArea)*shared), nil
}

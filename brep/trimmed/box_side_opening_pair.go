package trimmed

import (
	"context"
	"math"
	"slices"
	"sort"

	"plainsolid/geom"
)

// SideOpeningCut removes a cylindrical footprint from the local Z=0 cap.
type SideOpeningCut struct {
	ExtrusionCircle
	Depth float64
}

type SideOpeningPairResult struct {
	Solid                           Solid
	RemovedVolume, SurfaceAreaDelta float64
}

// BoxSideOpeningPair removes two transversely intersecting disks that both
// cross Y=0 and clear the other three sides. Stock is [0,width] × [0,back] ×
// [0,length] in frame. Both tools enter Z=0. Depth equal to length cuts through.
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
	if cuts[0].Depth != cuts[1].Depth && math.Abs(cuts[0].Depth-cuts[1].Depth) <= 8*tolerance {
		return out, geom.ErrDomain
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
	const both = 3
	if err := cap(0, func(mask uint8) bool { return material(mask, both) }, true); err != nil {
		return out, err
	}
	levels := []float64{0, cuts[0].Depth, cuts[1].Depth, length}
	sort.Float64s(levels)
	levels = slices.Compact(levels)
	activeAt := func(top float64) uint8 {
		var active uint8
		for i, cut := range cuts {
			if top <= cut.Depth {
				active |= 1 << i
			}
		}
		return active
	}
	topActive := activeAt(length)
	if err := cap(length, func(mask uint8) bool { return material(mask, topActive) }, false); err != nil {
		return out, err
	}
	out.SurfaceAreaDelta = -sidePairRemovedArea(points, edges, both) - sidePairRemovedArea(points, edges, topActive)
	for i, top := range levels[1:] {
		bottom := levels[i]
		active := activeAt(top)
		thickness := top - bottom
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
			next := activeAt(levels[i+2])
			if err := cap(top, func(mask uint8) bool { return material(mask, next) && !material(mask, active) }, true); err != nil {
				return SideOpeningPairResult{}, err
			}
			out.SurfaceAreaDelta += sidePairRemovedArea(points, edges, active) - sidePairRemovedArea(points, edges, next)
		}
	}
	out.Solid, err = NewSolidContext(ctx, builder.vertices, builder.edges, builder.faces, tolerance)
	if err != nil {
		return SideOpeningPairResult{}, err
	}
	return out, nil
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
	for _, c := range circles {
		if !uvFinite(c.Center) || !finite(c.Radius) || c.Radius-math.Abs(c.Center.Y) <= pad ||
			c.Center.X-c.Radius <= pad || width-c.Center.X-c.Radius <= pad || back-c.Center.Y-c.Radius <= pad {
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
		if !uvFinite(p) || math.Abs(p.Y) <= pad {
			return fail()
		}
		id := len(points)
		points = append(points, p)
		add(0, id)
		add(1, id)
	}
	side := []int{0, 1}
	for i, c := range circles {
		root := math.Sqrt((c.Radius - c.Center.Y) * (c.Radius + c.Center.Y))
		for _, x := range []float64{c.Center.X - root, c.Center.X + root} {
			id := len(points)
			points = append(points, geom.Vec2{X: x})
			side = append(side, id)
			add(i, id)
		}
	}
	sort.Slice(side, func(i, j int) bool { return points[side[i]].X < points[side[j]].X })
	classify := func(p geom.Vec2, circle ExtrusionCircle) (bool, bool) {
		distance := math.Hypot(p.X-circle.Center.X, p.Y-circle.Center.Y)
		return distance < circle.Radius, math.Abs(distance-circle.Radius) > tol
	}
	var edges []sidePairEdge
	for i, start := range side[:len(side)-1] {
		end := side[i+1]
		x, y := points[start].X, points[end].X
		if y-x <= pad {
			return fail()
		}
		mask := uint8(sidePairStock)
		for j, circle := range circles {
			inside, clear := classify(geom.Vec2{X: x + (y-x)/2}, circle)
			if !clear {
				return fail()
			}
			if inside {
				mask |= 1 << j
			}
		}
		edges = append(edges, sidePairEdge{start: start, end: end, left: mask})
	}
	for i := 1; i <= 3; i++ {
		edges = append(edges, sidePairEdge{start: i, end: (i + 1) % 4, left: sidePairStock})
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
			if math.Abs(mid.Y) <= tol {
				return fail()
			}
			if mid.Y < 0 {
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
// footprint and finite axial interval. The other footprint is the previously
// removed circle at the same side, measured without constructing a result.
func BoxSideOpeningPairOverlap(frame geom.Frame, width, back float64, old, tool ExtrusionCircle, oldDepth, low, high, tolerance float64) (float64, error) {
	if !finite(tolerance) || tolerance <= 0 || !finite(oldDepth) || oldDepth <= 0 || !finite(low) || !finite(high) || low < 0 || high <= low {
		return 0, geom.ErrInvalid
	}
	points, edges, err := sidePairArrangement(frame, width, back, [2]ExtrusionCircle{old, tool}, tolerance)
	if err != nil {
		return 0, err
	}
	oldArea := sidePairRemovedArea(points, edges, 1)
	toolArea := sidePairRemovedArea(points, edges, 2)
	unionArea := sidePairRemovedArea(points, edges, 3)
	shared := max(0, min(oldDepth, high)-low)
	return max(0, toolArea*(high-low)-(oldArea+toolArea-unionArea)*shared), nil
}

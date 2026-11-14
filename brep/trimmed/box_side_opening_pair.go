package trimmed

import (
	"context"
	"math"
	"math/bits"
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

type SideOpeningCutsResult struct {
	Solid                           Solid
	RemovedVolume, SurfaceAreaDelta float64
	RetainedCuts                    []SideOpeningCut
}

type SideOpeningPairResult = SideOpeningCutsResult

// BoxSideOpeningPair removes two circular cutters. Each disk
// crosses one stock side or two adjacent sides. Stock is [0,width] ×
// [0,back] × [0,length] in frame. Depth equal to length cuts through.
func BoxSideOpeningPair(ctx context.Context, frame geom.Frame, width, back, length float64, cuts [2]SideOpeningCut, tolerance float64) (SideOpeningPairResult, error) {
	return BoxSideOpeningCuts(ctx, frame, width, back, length, cuts[:], tolerance)
}

// MaxSideOpeningCuts bounds the size of the circle arrangement and B-rep.
const MaxSideOpeningCuts = 16

// BoxSideOpeningCuts removes finite circular cutters with parallel axes.
// Each footprint crosses one rectangular side or two adjacent sides, and
// each interval opens at an end cap. Fully concealed cutters do not introduce
// extra wall or floor partitions.
func BoxSideOpeningCuts(ctx context.Context, frame geom.Frame, width, back, length float64, cuts []SideOpeningCut, tolerance float64) (SideOpeningCutsResult, error) {
	var out SideOpeningCutsResult
	if ctx == nil || len(cuts) == 0 || len(cuts) > MaxSideOpeningCuts || !finite(width) || !finite(back) ||
		width <= 0 || back <= 0 || !finite(length) || !finite(tolerance) || tolerance <= 0 || length <= 8*tolerance {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	for _, cut := range cuts {
		if sideOpeningSides(width, back, cut.ExtrusionCircle, tolerance) == 0 ||
			!finite(cut.Depth) || cut.Depth <= 8*tolerance || cut.Depth > length || cut.Depth < length && length-cut.Depth <= 8*tolerance {
			return out, geom.ErrDomain
		}
	}
	cuts = removeContainedSideOpeningCuts(cuts, length, tolerance)
	sort.Slice(cuts, func(i, j int) bool { return sideCutLess(cuts[i].ExtrusionCircle, cuts[j].ExtrusionCircle) })
	circles := make([]ExtrusionCircle, len(cuts))
	for i, cut := range cuts {
		circles[i] = cut.ExtrusionCircle
	}
	points, edges, err := sideOpeningArrangement(frame, width, back, circles, tolerance)
	if err != nil {
		return out, err
	}
	spans, levels := sideOpeningLevels(cuts, length)
	if retained := exposedSideOpeningCuts(edges, spans, levels); bits.OnesCount32(retained) != len(cuts) {
		var visible []SideOpeningCut
		circles = circles[:0]
		for i, cut := range cuts {
			if retained&(1<<i) != 0 {
				visible = append(visible, cut)
				circles = append(circles, cut.ExtrusionCircle)
			}
		}
		if len(visible) == 0 {
			return out, geom.ErrDomain
		}
		cuts = visible
		spans, levels = sideOpeningLevels(cuts, length)
		points, edges, err = sideOpeningArrangement(frame, width, back, circles, tolerance)
		if err != nil {
			return out, err
		}
	}
	builder := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	material := func(mask, active uint32) bool { return mask&sideOpeningStock != 0 && mask&active == 0 }
	cap := func(z float64, keep func(uint32) bool, reversed bool) error {
		return buildSideOpeningCap(&builder, points, edges, z, keep, reversed, tolerance)
	}
	for i, span := range spans {
		// A common opposed floor puts two junctions at the same axial
		// level. A contact where both footprints overlap is unresolved.
		for j, prior := range spans[:i] {
			if span[0] == prior[1] || span[1] == prior[0] {
				a, b := cuts[i].ExtrusionCircle, cuts[j].ExtrusionCircle
				if circleRelation(unionDisk{center: a.Center, radius: a.Radius}, unionDisk{center: b.Center, radius: b.Radius}, true) < 0 {
					return out, geom.ErrUnrepresentable
				}
			}
		}
	}
	activeIn := func(bottom, top float64) uint32 { return sideOpeningActive(spans, bottom, top) }
	bottomActive := activeIn(0, levels[1])
	if err := cap(0, func(mask uint32) bool { return material(mask, bottomActive) }, true); err != nil {
		return out, err
	}
	topActive := activeIn(levels[len(levels)-2], length)
	if err := cap(length, func(mask uint32) bool { return material(mask, topActive) }, false); err != nil {
		return out, err
	}
	out.SurfaceAreaDelta = -sideOpeningRemovedArea(points, edges, bottomActive) - sideOpeningRemovedArea(points, edges, topActive)
	for i, top := range levels[1:] {
		bottom := levels[i]
		active := activeIn(bottom, top)
		thickness := top - bottom
		if thickness <= 8*tolerance {
			return SideOpeningCutsResult{}, geom.ErrDomain
		}
		out.RemovedVolume += sideOpeningRemovedArea(points, edges, active) * thickness
		out.SurfaceAreaDelta -= 2 * (width + back) * thickness
		for _, edge := range edges {
			left, right := material(edge.left, active), material(edge.right, active)
			if left == right {
				continue
			}
			if !left {
				edge = edge.reversed()
			}
			if err := buildSideOpeningWall(&builder, points, edge, bottom, top); err != nil {
				return SideOpeningCutsResult{}, err
			}
			out.SurfaceAreaDelta += edge.length(points) * thickness
		}
		if top < length {
			next := activeIn(top, levels[i+2])
			floors := func(mask uint32) bool { return material(mask, next) && !material(mask, active) }
			ceilings := func(mask uint32) bool { return material(mask, active) && !material(mask, next) }
			if err := cap(top, floors, true); err != nil {
				return SideOpeningCutsResult{}, err
			}
			if err := cap(top, ceilings, false); err != nil {
				return SideOpeningCutsResult{}, err
			}
			out.SurfaceAreaDelta += 2*sideOpeningRemovedArea(points, edges, active|next) -
				sideOpeningRemovedArea(points, edges, active) - sideOpeningRemovedArea(points, edges, next)
		}
	}
	if out.RemovedVolume <= 0 {
		return SideOpeningCutsResult{}, geom.ErrDomain
	}
	// Retained stock bounds and STEP recovery require evidence for each
	// rectangular extent. A chain of through cuts can erase a whole side.
	local := make([]geom.Vec3, len(builder.vertices))
	for i, point := range builder.vertices {
		local[i] = frame.ToLocal(point)
	}
	bounds, err := geom.Bounds(local)
	if err != nil || bounds.Min != (geom.Vec3{}) || bounds.Max != (geom.Vec3{X: width, Y: back, Z: length}) {
		return SideOpeningCutsResult{}, geom.ErrUnrepresentable
	}
	out.Solid, err = NewSolidContext(ctx, builder.vertices, builder.edges, builder.faces, tolerance)
	if err != nil {
		return SideOpeningCutsResult{}, err
	}
	if len(out.Solid.Topology().Shells) != 1 {
		return SideOpeningCutsResult{}, geom.ErrUnrepresentable
	}
	out.RetainedCuts = cuts
	return out, nil
}

func sideOpeningLevels(cuts []SideOpeningCut, length float64) ([][2]float64, []float64) {
	spans := make([][2]float64, len(cuts))
	levels := []float64{0, length}
	for i, cut := range cuts {
		spans[i] = sideOpeningSpan(length, cut)
		levels = append(levels, spans[i][0], spans[i][1])
	}
	sort.Float64s(levels)
	return spans, slices.Compact(levels)
}

// Every open cell in the clipped section meets an arrangement edge on its
// boundary. Its coverage mask records which circles contain that cell. A
// cutter is redundant exactly when no cell in any slab depends on its bit.
func exposedSideOpeningCuts(edges []sideOpeningEdge, spans [][2]float64, levels []float64) uint32 {
	retained := uint32(1)<<len(spans) - 1
	if len(spans) == 1 {
		return retained
	}
	masks := make(map[uint32]struct{})
	for _, edge := range edges {
		for _, mask := range [...]uint32{edge.left, edge.right} {
			if mask&sideOpeningStock != 0 {
				masks[mask&^sideOpeningStock] = struct{}{}
			}
		}
	}
	active := make([]uint32, len(levels)-1)
	for i, top := range levels[1:] {
		active[i] = sideOpeningActive(spans, levels[i], top)
	}
	// Remove one cutter at a time so mutually redundant cutters cannot
	// all be discarded. The incoming cutter order is canonical.
	for i := range spans {
		bit := uint32(1) << i
		needed := false
		for _, slab := range active {
			slab &= retained
			if slab&bit == 0 {
				continue
			}
			for mask := range masks {
				if mask&slab == bit {
					needed = true
					break
				}
			}
			if needed {
				break
			}
		}
		if !needed {
			retained &^= bit
		}
	}
	return retained
}

func sideOpeningSpan(length float64, cut SideOpeningCut) [2]float64 {
	if cut.OppositeCap {
		return [2]float64{length - cut.Depth, length}
	}
	return [2]float64{0, cut.Depth}
}

func removeContainedSideOpeningCuts(cuts []SideOpeningCut, length, tolerance float64) []SideOpeningCut {
	filtered := make([]SideOpeningCut, 0, len(cuts))
	for i, cut := range cuts {
		span := sideOpeningSpan(length, cut)
		covered := false
		for j, other := range cuts {
			if i == j || other.Radius <= cut.Radius {
				continue
			}
			otherSpan := sideOpeningSpan(length, other)
			if span[0] < otherSpan[0] || span[1] > otherSpan[1] {
				continue
			}
			distance := math.Hypot(other.Center.X-cut.Center.X, other.Center.Y-cut.Center.Y)
			if other.Radius-cut.Radius-distance > 8*tolerance &&
				circleRelation(unionDisk{center: cut.Center, radius: cut.Radius}, unionDisk{center: other.Center, radius: other.Radius}, false) < 0 {
				covered = true
				break
			}
		}
		if !covered {
			filtered = append(filtered, cut)
		}
	}
	return filtered
}

func sideOpeningActive(spans [][2]float64, bottom, top float64) uint32 {
	var active uint32
	for i, span := range spans {
		if span[0] <= bottom && top <= span[1] {
			active |= 1 << i
		}
	}
	return active
}

const sideOpeningStock uint32 = 1 << 31

type sideOpeningEdge struct {
	start, end  int
	left, right uint32
	circle      ExtrusionCircle
	from, to    float64
}

func (e sideOpeningEdge) reversed() sideOpeningEdge {
	e.start, e.end = e.end, e.start
	e.from, e.to = e.to, e.from
	e.left, e.right = e.right, e.left
	return e
}

func (e sideOpeningEdge) length(points []geom.Vec2) float64 {
	if e.circle.Radius != 0 {
		return e.circle.Radius * math.Abs(e.to-e.from)
	}
	p, q := points[e.start], points[e.end]
	return math.Hypot(q.X-p.X, q.Y-p.Y)
}

func sideOpeningRemovedArea(points []geom.Vec2, edges []sideOpeningEdge, active uint32) float64 {
	return sideOpeningArea(points, edges, func(mask uint32) bool { return mask&sideOpeningStock != 0 && mask&active != 0 })
}

func sideOpeningArea(points []geom.Vec2, edges []sideOpeningEdge, keep func(uint32) bool) float64 {
	area := 0.
	for _, edge := range edges {
		left, right := keep(edge.left), keep(edge.right)
		if left == right {
			continue
		}
		if !left {
			edge = edge.reversed()
		}
		area += edge.area(points, geom.Vec2{})
	}
	return area
}

func (e sideOpeningEdge) area(points []geom.Vec2, origin geom.Vec2) float64 {
	p, q := sub2(points[e.start], origin), sub2(points[e.end], origin)
	if c := e.circle; c.Radius != 0 {
		center := sub2(c.Center, origin)
		return (center.X*(q.Y-p.Y) - center.Y*(q.X-p.X) + c.Radius*c.Radius*(e.to-e.from)) / 2
	}
	return (p.X*q.Y - p.Y*q.X) / 2
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

func sideOpeningDistances(width, back float64, p geom.Vec2) [4]float64 {
	return [4]float64{p.Y, width - p.X, back - p.Y, p.X}
}

// BoxSideOpeningCount returns the number of eligible stock side contacts
// for a footprint, or zero outside the side-opening domain.
func BoxSideOpeningCount(width, back float64, c ExtrusionCircle, tolerance float64) int {
	return bits.OnesCount8(sideOpeningSides(width, back, c, tolerance))
}

func sideOpeningSides(width, back float64, c ExtrusionCircle, tol float64) uint8 {
	pad := 8 * tol
	if !finite(width) || !finite(back) || width <= 0 || back <= 0 || !finite(tol) || tol < 0 ||
		!uvFinite(c.Center) || !finite(c.Radius) || c.Radius <= pad {
		return 0
	}
	var openings uint8
	for side, distance := range sideOpeningDistances(width, back, c.Center) {
		if distance-c.Radius > pad {
			continue
		}
		if c.Radius-math.Abs(distance) <= pad {
			along, limit := c.Center.X, width
			if side%2 == 1 {
				along, limit = c.Center.Y, back
			}
			// Tangency to a supporting line is eligible only when the
			// disk lies inside that half-plane and touches its endpoint.
			if distance != c.Radius || along != 0 && along != limit {
				return 0
			}
		}
		openings |= 1 << side
	}
	switch openings {
	case 1, 2, 4, 8, 3, 6, 12, 9:
		return openings
	default:
		return 0
	}
}

func sideOpeningArrangement(frame geom.Frame, width, back float64, circles []ExtrusionCircle, tol float64) ([]geom.Vec2, []sideOpeningEdge, error) {
	if !finite(width) || !finite(back) || width <= 0 || back <= 0 {
		return nil, nil, geom.ErrInvalid
	}
	fail := func() ([]geom.Vec2, []sideOpeningEdge, error) { return nil, nil, geom.ErrDomain }
	pad := 8 * tol
	openings := make([]uint8, len(circles))
	for i, c := range circles {
		openings[i] = sideOpeningSides(width, back, c, tol)
		if openings[i] == 0 {
			return fail()
		}
	}
	points := []geom.Vec2{{}, {X: width}, {X: width, Y: back}, {Y: back}}
	type event struct {
		id    int
		angle float64
	}
	events := make([][]event, len(circles))
	cornerEvents := make([][4]bool, len(circles))
	add := func(i, id int) {
		if id < 4 {
			if cornerEvents[i][id] {
				return
			}
			cornerEvents[i][id] = true
		}
		c, p := circles[i], points[id]
		events[i] = append(events[i], event{id, math.Atan2(p.Y-c.Center.Y, p.X-c.Center.X)})
	}
	onCircle := func(p geom.Vec2, c ExtrusionCircle) bool {
		return circleRelation(unionDisk{center: p}, unionDisk{center: c.Center, radius: c.Radius}, true) == 0
	}
	for i, first := range circles {
		a := unionDisk{center: first.Center, radius: first.Radius}
		for j := i + 1; j < len(circles); j++ {
			second := circles[j]
			b := unionDisk{center: second.Center, radius: second.Radius}
			outer, inner := circleRelation(a, b, true), circleRelation(a, b, false)
			dx, dy := second.Center.X-first.Center.X, second.Center.Y-first.Center.Y
			d := math.Hypot(dx, dy)
			r, s := first.Radius, second.Radius
			if !finite(d) || math.Abs(r+s-d) <= pad || math.Abs(d-math.Abs(r-s)) <= pad {
				return fail()
			}
			if outer > 0 || inner < 0 {
				continue
			}
			if outer == 0 || inner == 0 {
				return fail()
			}
			cosine := (d + (r-s)*(r+s)/d) / (2 * r)
			along, cross := r*cosine, r*math.Sqrt((1-cosine)*(1+cosine))
			if !finite(cross) || cross <= pad {
				return fail()
			}
			for _, sign := range []float64{-1, 1} {
				p := geom.Vec2{X: first.Center.X + (along*dx-sign*cross*dy)/d, Y: first.Center.Y + (along*dy+sign*cross*dx)/d}
				if !uvFinite(p) {
					return fail()
				}
				corner := -1
				for id, vertex := range points[:4] {
					if math.Hypot(p.X-vertex.X, p.Y-vertex.Y) <= pad && onCircle(vertex, first) && onCircle(vertex, second) {
						corner = id
						break
					}
				}
				if corner >= 0 {
					add(i, corner)
					add(j, corner)
					continue
				}
				for _, distance := range sideOpeningDistances(width, back, p) {
					if math.Abs(distance) <= pad {
						return fail()
					}
				}
				for k, circle := range circles {
					if k != i && k != j && math.Abs(math.Hypot(p.X-circle.Center.X, p.Y-circle.Center.Y)-circle.Radius) <= pad {
						return fail()
					}
				}
				id := len(points)
				points = append(points, p)
				add(i, id)
				add(j, id)
			}
		}
	}
	sides := [4][]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	for i, c := range circles {
		for j, offset := range sideOpeningDistances(width, back, c.Center) {
			if openings[i]&(1<<j) == 0 {
				continue
			}
			root := math.Sqrt((c.Radius - offset) * (c.Radius + offset))
			hitsSide := false
			for _, delta := range []float64{-root, root} {
				var p geom.Vec2
				var along, limit float64
				if j%2 == 0 {
					along, limit = c.Center.X+delta, width
					p = geom.Vec2{X: along, Y: float64(j/2) * back}
				} else {
					along, limit = c.Center.Y+delta, back
					p = geom.Vec2{X: float64(1-j/2) * width, Y: along}
				}
				if math.Abs(along) <= pad || math.Abs(limit-along) <= pad {
					first, last := sides[j][0], sides[j][1]
					if j >= 2 {
						first, last = last, first
					}
					id := first
					if math.Abs(limit-along) < math.Abs(along) {
						id = last
					}
					// Both support lines meet at this existing vertex. Prove
					// equality before sharing its ID with the circular trim.
					if !onCircle(points[id], c) {
						return fail()
					}
					add(i, id)
					hitsSide = true
					continue
				}
				if along < 0 || along > limit {
					continue
				}
				id := len(points)
				points = append(points, p)
				sides[j] = append(sides[j], id)
				add(i, id)
				hitsSide = true
			}
			if !hitsSide {
				return fail()
			}
		}
	}
	classify := func(p geom.Vec2, circle ExtrusionCircle) (bool, bool) {
		distance := math.Hypot(p.X-circle.Center.X, p.Y-circle.Center.Y)
		return distance < circle.Radius, math.Abs(distance-circle.Radius) > tol
	}
	var edges []sideOpeningEdge
	for j, side := range sides {
		if len(side) == 2 {
			edges = append(edges, sideOpeningEdge{start: side[0], end: side[1], left: sideOpeningStock})
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
			mask := uint32(sideOpeningStock)
			for k, circle := range circles {
				inside, clear := classify(mid, circle)
				if !clear {
					return fail()
				}
				if inside {
					mask |= 1 << k
				}
			}
			edges = append(edges, sideOpeningEdge{start: start, end: end, left: mask})
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
			inward := sideOpeningDistances(width, back, mid)
			distance := min(inward[0], inward[1], inward[2], inward[3])
			if math.Abs(distance) <= tol {
				return fail()
			}
			if distance < 0 {
				continue
			}
			mask := uint32(sideOpeningStock)
			for k, other := range circles {
				if i == k {
					continue
				}
				inside, clear := classify(mid, other)
				if !clear {
					return fail()
				}
				if inside {
					mask |= 1 << k
				}
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
				edges = append(edges, sideOpeningEdge{current, next, mask | 1<<i, mask, c, from, to})
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

func buildSideOpeningCap(b *cornerCylinderBuilder, points []geom.Vec2, edges []sideOpeningEdge, z float64, keep func(uint32) bool, reversed bool, tolerance float64) error {
	next := make(map[int]sideOpeningEdge)
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
	type loop struct {
		use   []Use
		area  float64
		holes [][]Use
	}
	var outer, inner []loop
	for _, start := range starts {
		if _, exists := next[start]; !exists {
			continue
		}
		var world []geom.Vec3
		var uv []geom.Vec2
		curves := make(map[int]PCurve)
		area := 0.
		id := start
		for {
			edge, exists := next[id]
			if !exists {
				return geom.ErrUnrepresentable
			}
			delete(next, id)
			p, q := points[edge.start], points[edge.end]
			area += edge.area(points, points[start])
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
		if len(uv) < 3 || !finite(area) || math.Abs(area) <= tolerance*tolerance {
			return geom.ErrUnrepresentable
		}
		wire, err := b.wire(surface, world, uv, curves)
		if err != nil {
			return err
		}
		if area > 0 {
			outer = append(outer, loop{use: wire, area: area})
		} else {
			inner = append(inner, loop{use: wire, area: area})
		}
	}
	for _, hole := range inner {
		owner := -1
		for i, region := range outer {
			if owner >= 0 && region.area >= outer[owner].area {
				continue
			}
			inside, err := sideOpeningLoopContains(region.use, hole.use[0].UVStart, tolerance)
			if err != nil {
				return err
			}
			if inside {
				owner = i
			}
		}
		if owner < 0 {
			return geom.ErrUnrepresentable
		}
		outer[owner].holes = append(outer[owner].holes, hole.use)
	}
	for _, region := range outer {
		b.faces = append(b.faces, Face{Surface: surface, Reversed: reversed, Outer: region.use, Holes: region.holes})
	}
	return nil
}

func sideOpeningLoopContains(loop []Use, point geom.Vec2, tolerance float64) (bool, error) {
	winding := 0
	for _, use := range loop {
		if arc, ok := use.Curve.(unionArcUV); ok {
			use.Curve = arc.EllipseUV
		}
		crossings, near, err := trimCrossing(use, point, 1, 1, tolerance)
		if err != nil {
			return false, err
		}
		if near {
			return false, geom.ErrUnrepresentable
		}
		winding += crossings
	}
	return winding == 1, nil
}

func buildSideOpeningWall(b *cornerCylinderBuilder, points []geom.Vec2, edge sideOpeningEdge, bottom, top float64) error {
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
	return BoxSideOpeningCutsOverlap(frame, width, back, length, []SideOpeningCut{old}, tool, low, high, tolerance)
}

// BoxSideOpeningCutsOverlap measures material still present in the tool
// footprint over [low, high]. The tool may extend a previously cut footprint.
func BoxSideOpeningCutsOverlap(frame geom.Frame, width, back, length float64, old []SideOpeningCut, tool ExtrusionCircle, low, high, tolerance float64) (float64, error) {
	if !finite(tolerance) || tolerance <= 0 || !finite(width) || !finite(back) || width <= 0 || back <= 0 ||
		!finite(length) || len(old) > MaxSideOpeningCuts ||
		!finite(low) || !finite(high) || low < 0 || high <= low || high > length {
		return 0, geom.ErrInvalid
	}
	for _, cut := range old {
		if sideOpeningSides(width, back, cut.ExtrusionCircle, tolerance) == 0 ||
			!finite(cut.Depth) || cut.Depth <= 0 || cut.Depth > length {
			return 0, geom.ErrInvalid
		}
	}
	old = removeContainedSideOpeningCuts(old, length, tolerance)
	circles := make([]ExtrusionCircle, 0, len(old)+1)
	spans := make([][2]float64, 0, len(old))
	levels := []float64{low, high}
	var toolMask uint32
	for i, cut := range old {
		circles = append(circles, cut.ExtrusionCircle)
		if cut.ExtrusionCircle == tool {
			toolMask = 1 << i
		}
		span := sideOpeningSpan(length, cut)
		spans = append(spans, span)
		for _, level := range span {
			if low < level && level < high {
				levels = append(levels, level)
			}
		}
	}
	if toolMask == 0 {
		toolMask = 1 << len(circles)
		circles = append(circles, tool)
	}
	points, edges, err := sideOpeningArrangement(frame, width, back, circles, tolerance)
	if err != nil {
		return 0, err
	}
	sort.Float64s(levels)
	levels = slices.Compact(levels)
	volume := 0.
	for i, top := range levels[1:] {
		bottom := levels[i]
		active := sideOpeningActive(spans, bottom, top)
		area := sideOpeningArea(points, edges, func(mask uint32) bool {
			return mask&sideOpeningStock != 0 && mask&toolMask != 0 && mask&active == 0
		})
		volume += area * (top - bottom)
	}
	return max(0, volume), nil
}

package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
)

// ExtrusionSegment is an authored line or circular arc. Radius zero selects
// a line; an arc uses its signed sweep and retains the authored endpoints.
type ExtrusionSegment struct {
	Start, End, Center        geom.Vec2
	Radius, StartAngle, Sweep float64
}

type ArcLoopExtrusion struct {
	Solid           Solid
	Area, Perimeter float64
	Bounds          geom.Box
}

// ExtrudeArcLoop sweeps a proved simple line/circle loop. Its bounded domain
// includes convex loops, axis-aligned line/quarter-circle loops with either
// signed curvature, transverse two-circle boundaries, and circular segments
// with one transverse circular notch whose disk is strictly clear of the
// closing chord. Unresolved nonconvex arrangements are rejected rather than
// replaced by a polygon. Positive and negative heights are supported.
//
// General trigonometric arcs use tolerant endpoint anchoring, not an exact
// algebraic incidence certificate. Authored endpoints are retained verbatim
// when their distance from the evaluated circle is at most both tolerance and
// 1024 ULPs of max(abs(center.X), abs(center.Y), radius). Interior evaluation
// remains the analytic circle; thus endpoint continuity is only within that
// allowance. Cardinal quarters retain the stricter existing construction.
// Area, perimeter and bounds are floating-point analytic estimates, not outward
// certified enclosures. Callers requiring exact incidence must use exact-curve
// proof APIs rather than infer it from this tolerant solid constructor.
func ExtrudeArcLoop(ctx context.Context, frame geom.Frame, segments []ExtrusionSegment, height, tolerance float64) (ArcLoopExtrusion, error) {
	var result ArcLoopExtrusion
	if ctx == nil || len(segments) < 2 || len(segments) > 64 || !finite(height) || height == 0 || !finite(tolerance) || tolerance <= 0 {
		return result, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	pieces := make([]ExtrusionSegment, 0, len(segments)*2)
	origin := segments[0].Start
	signedArea, perimeter := 0., 0.
	for i, s := range segments {
		if !uvFinite(s.Start) || !uvFinite(s.End) || s.Start == s.End || s.End != segments[(i+1)%len(segments)].Start || !finite(s.Radius) || s.Radius < 0 {
			return result, geom.ErrDomain
		}
		p, q := sub2(s.Start, origin), sub2(s.End, origin)
		if s.Radius == 0 {
			if s.Sweep != 0 || s.StartAngle != 0 || s.Center != (geom.Vec2{}) {
				return result, geom.ErrInvalid
			}
			signedArea += (p.X*q.Y - p.Y*q.X) / 2
			perimeter += math.Hypot(q.X-p.X, q.Y-p.Y)
			pieces = append(pieces, s)
			continue
		}
		if !uvFinite(s.Center) || !finite(s.StartAngle) || math.Abs(s.StartAngle) > 8*math.Pi || !finite(s.Sweep) || s.Sweep == 0 || math.Abs(s.Sweep) >= 2*math.Pi || s.Radius <= 8*tolerance {
			return result, geom.ErrDomain
		}
		for j, point := range []geom.Vec2{s.Start, s.End} {
			expected := arcLoopPoint(s, s.StartAngle+float64(j)*s.Sweep)
			if math.Hypot(expected.X-point.X, expected.Y-point.Y) > min(tolerance, 1024*(math.Nextafter(max(math.Abs(s.Center.X), math.Abs(s.Center.Y), s.Radius), math.Inf(1))-max(math.Abs(s.Center.X), math.Abs(s.Center.Y), s.Radius))) {
				return result, geom.ErrDomain
			}
		}
		center := sub2(s.Center, origin)
		signedArea += (center.X*(s.End.Y-s.Start.Y) - center.Y*(s.End.X-s.Start.X) + s.Radius*s.Radius*s.Sweep) / 2
		perimeter += s.Radius * math.Abs(s.Sweep)
		if len(segments) >= 3 && arcLoopCardinalQuarter(s) {
			pieces = append(pieces, s)
			continue
		}
		count := max(1, int(math.Ceil(math.Abs(s.Sweep)/(math.Pi/4))))
		// Native face loops require at least three coedges. A circular segment
		// bounded by one arc and one chord needs an interior analytic arc split.
		if len(segments) == 2 && segments[(i+1)%2].Radius == 0 {
			count = max(2, count)
		}
		previous := s.Start
		for j := 0; j < count; j++ {
			start := s.StartAngle + s.Sweep*float64(j)/float64(count)
			end := s.StartAngle + s.Sweep*float64(j+1)/float64(count)
			next := arcLoopPoint(s, end)
			if j == count-1 {
				next = s.End
			}
			pieces = append(pieces, ExtrusionSegment{Start: previous, End: next, Center: s.Center, Radius: s.Radius, StartAngle: start, Sweep: end - start})
			previous = next
		}
	}
	if len(pieces) > 128 {
		return result, geom.ErrDomain
	}
	if !finite(signedArea) || !finite(perimeter) || math.Abs(signedArea) <= tolerance*perimeter || math.Abs(height) <= 8*tolerance {
		return result, geom.ErrDomain
	}
	if signedArea < 0 {
		for i := range pieces {
			p := &pieces[i]
			p.Start, p.End = p.End, p.Start
			if p.Radius > 0 {
				p.StartAngle += p.Sweep
				p.Sweep = -p.Sweep
			}
		}
		for i, j := 0, len(pieces)-1; i < j; i, j = i+1, j-1 {
			pieces[i], pieces[j] = pieces[j], pieces[i]
		}
		signedArea = -signedArea
	}
	if !arcLoopConvex(pieces) && !arcLoopSeparatedQuarters(pieces) && !arcLoopTwoCircles(segments, tolerance) && !arcLoopCircleNotch(segments, tolerance) {
		return result, geom.ErrDomain
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	lo, hi := min(0, height), max(0, height)
	curves := map[int]PCurve{}
	uv := make([]geom.Vec2, len(pieces))
	for i, s := range pieces {
		uv[i] = s.Start
		if s.Radius > 0 {
			arc, e := newBoundedAnchoredCircularUV(EllipseUV{Center: s.Center, Cos: geom.Vec2{X: s.Radius}, Sin: geom.Vec2{Y: s.Radius}, From: s.StartAngle, To: s.StartAngle + s.Sweep}, s.Start, s.End)
			if e != nil {
				return result, e
			}
			curves[i] = arc
			if arcLoopCardinalQuarter(s) {
				k := int(math.Round(s.StartAngle / (math.Pi / 2)))
				clockwise := s.Sweep < 0
				if clockwise {
					k = -k
				}
				k = (k%4 + 4) % 4
				arc, e := extrusionArc(ExtrusionCircle{Center: s.Center, Radius: s.Radius}, k, s.Start, s.End, clockwise)
				if e != nil {
					return result, e
				}
				curves[i] = arc
			}
		}
	}
	for i, z := range []float64{lo, hi} {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return result, e
		}
		surface, e := geom.NewPlaneSurface(f)
		if e != nil {
			return result, e
		}
		points := make([]geom.Vec3, len(uv))
		for j, p := range uv {
			points[j] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
		}
		if e = b.face(surface, points, uv, curves, i == 0); e != nil {
			return result, e
		}
	}
	for _, s := range pieces {
		if s.Radius == 0 {
			if e := b.wall(s.Start, s.End, lo, hi); e != nil {
				return result, e
			}
			continue
		}
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: s.Center.X, Y: s.Center.Y}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return result, e
		}
		surface, e := geom.NewCylinderSurface(f, s.Radius)
		if e != nil {
			return result, e
		}
		a, z := s.StartAngle, s.StartAngle+s.Sweep
		p, q := s.Start, s.End
		reversed := s.Sweep < 0
		if reversed {
			a, z = z, a
			p, q = q, p
		}
		points := []geom.Vec3{{X: p.X, Y: p.Y, Z: lo}, {X: q.X, Y: q.Y, Z: lo}, {X: q.X, Y: q.Y, Z: hi}, {X: p.X, Y: p.Y, Z: hi}}
		chart := []geom.Vec2{{X: a, Y: lo}, {X: z, Y: lo}, {X: z, Y: hi}, {X: a, Y: hi}}
		if e = b.face(surface, points, chart, nil, reversed); e != nil {
			return result, e
		}
	}
	solid, e := NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return result, e
	}
	bounds, e := arcLoopBounds(frame, pieces, lo, hi)
	if e != nil {
		return result, e
	}
	return ArcLoopExtrusion{Solid: solid, Area: signedArea, Perimeter: perimeter, Bounds: bounds}, nil
}

func arcLoopPoint(s ExtrusionSegment, angle float64) geom.Vec2 {
	quadrant := math.Round(angle / (math.Pi / 2))
	if angle == quadrant*(math.Pi/2) {
		switch (int(quadrant)%4 + 4) % 4 {
		case 0:
			return geom.Vec2{X: s.Center.X + s.Radius, Y: s.Center.Y}
		case 1:
			return geom.Vec2{X: s.Center.X, Y: s.Center.Y + s.Radius}
		case 2:
			return geom.Vec2{X: s.Center.X - s.Radius, Y: s.Center.Y}
		case 3:
			return geom.Vec2{X: s.Center.X, Y: s.Center.Y - s.Radius}
		}
	}
	sin, cos := math.Sincos(math.Remainder(angle, 2*math.Pi))
	return geom.Vec2{X: s.Center.X + s.Radius*cos, Y: s.Center.Y + s.Radius*sin}
}
func arcLoopCardinalQuarter(s ExtrusionSegment) bool {
	if s.Radius <= 0 || math.Abs(s.Sweep) != math.Pi/2 {
		return false
	}
	k := math.Round(s.StartAngle / (math.Pi / 2))
	return s.StartAngle == k*(math.Pi/2) && s.Start == arcLoopPoint(s, s.StartAngle) && s.End == arcLoopPoint(s, s.StartAngle+s.Sweep)
}
func arcLoopConvex(p []ExtrusionSegment) bool {
	tangent := func(s ExtrusionSegment, end bool) geom.Vec2 {
		if s.Radius == 0 {
			return sub2(s.End, s.Start)
		}
		q := s.Start
		if end {
			q = s.End
		}
		q = sub2(q, s.Center)
		return geom.Vec2{X: -q.Y, Y: q.X}
	}
	for i, s := range p {
		if s.Radius > 0 && s.Sweep < 0 {
			return false
		}
		for j, t := range p {
			if j != i && t.Start == s.Start {
				return false
			}
			sign, e := geom.Orient2D(s.Start, s.End, t.Start)
			if e != nil || sign < 0 {
				return false
			}
		}
		a, c := tangent(s, true), tangent(p[(i+1)%len(p)], false)
		sign, e := geom.Orient2D(geom.Vec2{}, a, c)
		if e != nil || sign < 0 || sign == 0 && a.X*c.X+a.Y*c.Y <= 0 {
			return false
		}
	}
	return true
}
func arcLoopSeparatedQuarters(p []ExtrusionSegment) bool {
	for _, s := range p {
		if s.Radius > 0 {
			if !arcLoopCardinalQuarter(s) {
				return false
			}
		} else if s.Start.X != s.End.X && s.Start.Y != s.End.Y {
			return false
		}
	}
	for i, a := range p {
		for j := i + 1; j < len(p); j++ {
			b := p[j]
			lo := geom.Vec2{X: max(min(a.Start.X, a.End.X), min(b.Start.X, b.End.X)), Y: max(min(a.Start.Y, a.End.Y), min(b.Start.Y, b.End.Y))}
			hi := geom.Vec2{X: min(max(a.Start.X, a.End.X), max(b.Start.X, b.End.X)), Y: min(max(a.Start.Y, a.End.Y), max(b.Start.Y, b.End.Y))}
			if lo.X > hi.X || lo.Y > hi.Y {
				continue
			}
			adjacent := j == i+1 || i == 0 && j == len(p)-1
			if adjacent {
				// An axis line through a cardinal circle endpoint is radial or
				// tangent. The opposite radial intersection lies outside this quarter.
				if (a.Radius == 0) != (b.Radius == 0) {
					continue
				}
				if a.Radius > 0 && a.Center == b.Center && a.Radius == b.Radius && !(a.Start == b.End && a.End == b.Start) {
					continue
				}
			}
			if j == i+1 && lo == a.End && hi == a.End {
				continue
			}
			if i == 0 && j == len(p)-1 && lo == a.Start && hi == a.Start {
				continue
			}
			return false
		}
	}
	return true
}
func arcLoopBounds(frame geom.Frame, pieces []ExtrusionSegment, lo, hi float64) (geom.Box, error) {
	var points []geom.Vec3
	add := func(p geom.Vec2) {
		for _, z := range []float64{lo, hi} {
			points = append(points, frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y, Z: z}))
		}
	}
	for _, s := range pieces {
		add(s.Start)
		add(s.End)
		if s.Radius == 0 {
			continue
		}
		add(arcLoopPoint(s, s.StartAngle))
		add(arcLoopPoint(s, s.StartAngle+s.Sweep))
		a, b := min(s.StartAngle, s.StartAngle+s.Sweep), max(s.StartAngle, s.StartAngle+s.Sweep)
		x, y := frame.XAxis(), frame.YAxis()
		for _, pair := range [][2]float64{{x.X, y.X}, {x.Y, y.Y}, {x.Z, y.Z}} {
			if pair == ([2]float64{}) {
				continue
			}
			angle := math.Atan2(pair[1], pair[0])
			k := math.Ceil((a - angle) / math.Pi)
			for v := angle + k*math.Pi; v <= b; v += math.Pi {
				if v >= a {
					add(arcLoopPoint(s, v))
				}
			}
		}
	}
	return geom.Bounds(points)
}

// ArcLoopBounds evaluates the analytic line/circle extrema in a rigid frame.
// It is suitable for a retained, already-qualified loop after placement; it
// checks finite curve data but does not independently certify loop simplicity.
func ArcLoopBounds(frame geom.Frame, segments []ExtrusionSegment, height float64) (geom.Box, error) {
	if len(segments) == 0 || len(segments) > 128 || !finite(height) || height == 0 || frame.XAxis().Length() == 0 || frame.ZAxis().Length() == 0 {
		return geom.Box{}, geom.ErrInvalid
	}
	for _, s := range segments {
		if !uvFinite(s.Start) || !uvFinite(s.End) || !uvFinite(s.Center) || !finite(s.Radius) || s.Radius < 0 || !finite(s.StartAngle) || math.Abs(s.StartAngle) > 8*math.Pi || !finite(s.Sweep) || math.Abs(s.Sweep) >= 2*math.Pi {
			return geom.Box{}, geom.ErrInvalid
		}
		if s.Radius > 0 && s.Sweep == 0 || s.Radius == 0 && (s.Sweep != 0 || s.StartAngle != 0) {
			return geom.Box{}, geom.ErrInvalid
		}
	}
	return arcLoopBounds(frame, segments, min(0, height), max(0, height))
}

// General circular anchors retain authored endpoints only within a bounded
// floating-point evaluation envelope; they never introduce a chord substitute.
func newBoundedAnchoredCircularUV(c EllipseUV, start, end geom.Vec2) (anchoredCircularUV, error) {
	r := c.Cos.X
	if !finite(r) || r <= 0 || c.Cos.Y != 0 || c.Sin.X != 0 || math.Abs(c.Sin.Y) != r || c.Linear != (geom.Vec2{}) || !uvFinite(c.Center) || !uvFinite(start) || !uvFinite(end) || !finite(c.From) || !finite(c.To) || math.Abs(c.From) > 10*math.Pi || math.Abs(c.To) > 10*math.Pi || c.From == c.To || math.Abs(c.To-c.From) > math.Pi/2 {
		return anchoredCircularUV{}, geom.ErrDomain
	}
	scale := max(math.Abs(c.Center.X), math.Abs(c.Center.Y), r, math.Abs(start.X), math.Abs(start.Y), math.Abs(end.X), math.Abs(end.Y))
	budget := 1024 * (math.Nextafter(scale, math.Inf(1)) - scale)
	if !finite(budget) || budget <= 0 {
		return anchoredCircularUV{}, geom.ErrDomain
	}
	for i, p := range []geom.Vec2{start, end} {
		q, e := c.EvalUV(float64(i))
		if e != nil || math.Hypot(p.X-q.X, p.Y-q.Y) > budget {
			return anchoredCircularUV{}, geom.ErrDomain
		}
	}
	return anchoredCircularUV{EllipseUV: c, Start: start, End: end}, nil
}

// Two transverse circles have exactly two intersections. Contiguous arcs
// joining the two opposite intersection branches therefore form a simple
// lens, crescent, or union boundary; no additional crossing can be hidden
// between samples. Signed arc choices retain the authored region.
func arcLoopTwoCircles(segments []ExtrusionSegment, tolerance float64) bool {
	same := func(a, b ExtrusionSegment) bool { return a.Center == b.Center && a.Radius == b.Radius }
	start := -1
	for i, s := range segments {
		if s.Radius <= 0 {
			return false
		}
		if !same(s, segments[(i+len(segments)-1)%len(segments)]) {
			start = i
		}
	}
	if start < 0 {
		return false
	}
	var groups []ExtrusionSegment
	for i := range segments {
		s := segments[(start+i)%len(segments)]
		if len(groups) > 0 && same(groups[len(groups)-1], s) {
			g := &groups[len(groups)-1]
			if (g.Sweep > 0) != (s.Sweep > 0) {
				return false
			}
			g.Sweep += s.Sweep
			g.End = s.End
		} else {
			groups = append(groups, s)
		}
	}
	if len(groups) != 2 {
		return false
	}
	a, b := groups[0], groups[1]
	if a.Start != b.End || a.End != b.Start || math.Abs(a.Sweep) >= 2*math.Pi || math.Abs(b.Sweep) >= 2*math.Pi || math.Hypot(a.Start.X-a.End.X, a.Start.Y-a.End.Y) <= 16*tolerance {
		return false
	}
	da, db := unionDisk{center: a.Center, radius: a.Radius}, unionDisk{center: b.Center, radius: b.Radius}
	if circleRelation(da, db, true) >= 0 || circleRelation(da, db, false) <= 0 {
		return false
	}
	x, e := geom.Orient2D(a.Center, b.Center, a.Start)
	if e != nil || x == 0 {
		return false
	}
	y, e := geom.Orient2D(a.Center, b.Center, a.End)
	return e == nil && x*y < 0
}

// A circular segment with one transverse circular notch remains simple when
// the notch disk is strictly clear of its closing chord. The two outer arc
// runs leave one angular gap, and the clockwise notch joins its two branches
// through the outer disk. Full-circle intersection counts prove that no
// additional crossing is hidden between the declared shared endpoints.
func arcLoopCircleNotch(input []ExtrusionSegment, tolerance float64) bool {
	segments := append([]ExtrusionSegment(nil), input...)
	lineIndex := -1
	for i, s := range segments {
		if s.Radius == 0 {
			if lineIndex >= 0 {
				return false
			}
			lineIndex = i
		}
	}
	if lineIndex < 0 {
		return false
	}
	if segments[(lineIndex+1)%len(segments)].Sweep < 0 {
		for i := range segments {
			s := &segments[i]
			s.Start, s.End = s.End, s.Start
			if s.Radius > 0 {
				s.StartAngle += s.Sweep
				s.Sweep = -s.Sweep
			}
		}
		for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
			segments[i], segments[j] = segments[j], segments[i]
		}
		lineIndex = len(segments) - 1 - lineIndex
	}
	line := segments[lineIndex]
	same := func(a, b ExtrusionSegment) bool { return a.Center == b.Center && a.Radius == b.Radius }
	var groups []ExtrusionSegment
	for i := 1; i < len(segments); i++ {
		s := segments[(lineIndex+i)%len(segments)]
		if s.Radius <= 0 {
			return false
		}
		if len(groups) > 0 && same(groups[len(groups)-1], s) {
			g := &groups[len(groups)-1]
			if (g.Sweep > 0) != (s.Sweep > 0) {
				return false
			}
			g.Sweep += s.Sweep
			g.End = s.End
		} else {
			groups = append(groups, s)
		}
	}
	if len(groups) != 3 || !same(groups[0], groups[2]) || same(groups[0], groups[1]) || groups[0].Sweep <= 0 || groups[2].Sweep <= 0 || groups[1].Sweep >= 0 || math.Abs(groups[1].Sweep) >= 2*math.Pi {
		return false
	}
	outer, notch := groups[0], groups[1]
	from := math.Atan2(line.End.Y-outer.Center.Y, line.End.X-outer.Center.X)
	to := math.Atan2(line.Start.Y-outer.Center.Y, line.Start.X-outer.Center.X)
	span := math.Mod(to-from+2*math.Pi, 2*math.Pi)
	if span <= 0 || groups[0].Sweep+groups[2].Sweep >= span-16*tolerance/outer.Radius {
		return false
	}
	a, b := unionDisk{center: outer.Center, radius: outer.Radius}, unionDisk{center: notch.Center, radius: notch.Radius}
	if circleRelation(a, b, true) >= 0 || circleRelation(a, b, false) <= 0 {
		return false
	}
	x, e := geom.Orient2D(a.center, b.center, notch.Start)
	if e != nil || x == 0 {
		return false
	}
	y, e := geom.Orient2D(a.center, b.center, notch.End)
	if e != nil || x*y >= 0 {
		return false
	}
	disk := prismCircle{center: notch.Center, radius: notch.Radius}
	if !disk.leftOf(line.Start, line.End, 8*tolerance) {
		return false
	}
	midpoint := arcLoopPoint(notch, notch.StartAngle+notch.Sweep/2)
	return (prismCircle{center: midpoint}).inside(outer.Center, ratInset(outer.Radius), 8*tolerance)
}

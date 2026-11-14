package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// BoxSideCylinderThrough removes a finite axial circular notch crossing just
// the lower Y side. Frame's axis passes through the circle center; stock spans
// X=[-left,right], Y=[-inset,back], Z=[0,length].
func BoxSideCylinderThrough(ctx context.Context, frame geom.Frame, left, right, back, inset, length, radius, tolerance float64) (Solid, error) {
	return BoxSideCylinderWithBores(ctx, frame, left, right, back, inset, length, radius, length, nil, nil, tolerance)
}

// BoxSideCylinderBlind cuts a circular notch from Z=0 to depth. The footprint
// crosses the lower Y side and clears the other three sides. The far cap and
// the material behind the analytic circular-segment floor remain intact.
func BoxSideCylinderBlind(ctx context.Context, frame geom.Frame, left, right, back, inset, length, radius, depth, tolerance float64) (Solid, error) {
	return BoxSideCylinderBlindWithBores(ctx, frame, left, right, back, inset, length, radius, depth, nil, nil, tolerance)
}

// BoxSideCylinderBlindWithBores adds through holes and blind bores from
// either cap whose footprints clear the side-opening tool and other bores.
func BoxSideCylinderBlindWithBores(ctx context.Context, frame geom.Frame, left, right, back, inset, length, radius, depth float64, through []ExtrusionCircle, blind []ExtrusionBlindHole, tolerance float64) (Solid, error) {
	if !finite(depth) || length-depth <= 8*tolerance {
		return Solid{}, geom.ErrDomain
	}
	return BoxSideCylinderWithBores(ctx, frame, left, right, back, inset, length, radius, depth, through, blind, tolerance)
}

// BoxSideCylinderWithBores cuts a side notch from Z=0 to depth and adds
// disjoint interior bores. A depth equal to length opens both caps.
func BoxSideCylinderWithBores(ctx context.Context, frame geom.Frame, left, right, back, inset, length, radius, depth float64, through []ExtrusionCircle, blind []ExtrusionBlindHole, tolerance float64) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	for _, v := range []float64{left, right, back, inset, length, radius, depth, tolerance} {
		if !finite(v) {
			return Solid{}, geom.ErrInvalid
		}
	}
	pad := 8 * tolerance
	if tolerance <= 0 || !finite(pad) || radius <= pad || depth <= pad || depth > length || radius-math.Abs(inset) <= pad || left-radius <= pad || right-radius <= pad || back-radius <= pad {
		return Solid{}, geom.ErrDomain
	}
	if depth < length && length-depth <= pad {
		return Solid{}, geom.ErrDomain
	}
	if e := validateNotchedBoxBores(ctx, frame, -left, right, -inset, back, length, radius, pad, through, blind); e != nil {
		return Solid{}, e
	}
	root := math.Sqrt((radius - inset) * (radius + inset))
	start, end := math.Pi+math.Asin(inset/radius), -math.Asin(inset/radius)
	n := int(math.Ceil((start - end) / (math.Pi / 2)))
	if depth < length {
		// The floor's chord and circular rim need at least three coedges.
		// A shallow opening still gets two arcs even when one would bound it.
		n = max(n, 2)
	}
	p := []geom.Vec2{{X: root, Y: -inset}, {X: right, Y: -inset}, {X: right, Y: back}, {X: -left, Y: back}, {X: -left, Y: -inset}, {X: -root, Y: -inset}}
	arcs := map[int][2]float64{}
	for i := 0; i < n; i++ {
		a := start + (end-start)*float64(i)/float64(n)
		z := start + (end-start)*float64(i+1)/float64(n)
		arcs[5+i] = [2]float64{a, z}
		if i+1 < n {
			p = append(p, geom.Vec2{X: radius * math.Cos(z), Y: radius * math.Sin(z)})
		}
	}
	// Shared world representatives must survive cardinal placement. Only the
	// already-qualified model tolerance covers trig/intersection roundoff.
	for i, v := range p {
		q := frame.ToLocal(frame.ToWorld(geom.Vec3{X: v.X, Y: v.Y}))
		if math.Hypot(q.X-v.X, q.Y-v.Y) > tolerance/8 || q.Z != 0 {
			return Solid{}, geom.ErrUnrepresentable
		}
		p[i] = geom.Vec2{X: q.X, Y: q.Y}
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}

	if e := b.sideCylinderCap(p, arcs, 0, radius, tolerance, true); e != nil {
		return Solid{}, e
	}
	upper := p
	upperArcs := arcs
	if depth < length {
		upper, upperArcs = p[:6], nil
		// The floor runs from the left to the right contact along the
		// stock side, then back along the circular arc.
		floor := []geom.Vec2{p[5], p[0]}
		for i := len(p) - 1; i > 5; i-- {
			floor = append(floor, p[i])
		}
		floorArcs := make(map[int][2]float64, n)
		for i := 0; i < n; i++ {
			a := arcs[5+n-1-i]
			floorArcs[1+i] = [2]float64{a[1], a[0]}
		}
		if e := b.sideCylinderCap(floor, floorArcs, depth, radius, tolerance, true); e != nil {
			return Solid{}, e
		}
		for i, v := range upper {
			if e := b.wall(v, upper[(i+1)%len(upper)], depth, length); e != nil {
				return Solid{}, e
			}
		}
	}
	upperCap := len(b.faces)
	if e := b.sideCylinderCap(upper, upperArcs, length, radius, tolerance, false); e != nil {
		return Solid{}, e
	}
	for i, v := range p {
		q := p[(i+1)%len(p)]
		var e error
		if a, ok := arcs[i]; ok {
			e = b.cylinder(a[1], a[0], depth, radius, true, v, q)
		} else {
			e = b.wall(v, q, 0, depth)
		}
		if e != nil {
			return Solid{}, e
		}
	}
	if e := b.addNotchedBoxBores(length, upperCap, through, blind); e != nil {
		return Solid{}, e
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}

func (b *cornerCylinderBuilder) sideCylinderCap(p []geom.Vec2, arcs map[int][2]float64, z, radius, tolerance float64, reversed bool) error {
	frame := b.frame
	cf, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return e
	}
	surface, e := geom.NewPlaneSurface(cf)
	if e != nil {
		return e
	}
	points := make([]geom.Vec3, len(p))
	curves := map[int]PCurve{}
	for i, v := range p {
		points[i] = geom.Vec3{X: v.X, Y: v.Y, Z: z}
		if a, ok := arcs[i]; ok {
			curve := EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: radius}, From: a[0], To: a[1]}
			lo, e0 := curve.EvalUV(0)
			hi, e1 := curve.EvalUV(1)
			q := p[(i+1)%len(p)]
			if e0 != nil || e1 != nil || math.Hypot(lo.X-v.X, lo.Y-v.Y) > tolerance || math.Hypot(hi.X-q.X, hi.Y-q.Y) > tolerance {
				return geom.ErrUnrepresentable
			}
			curves[i] = unionArcUV{EllipseUV: curve, Start: v, End: q}
		}
	}
	return b.face(surface, points, p, curves, reversed)
}

func validateNotchedBoxBores(ctx context.Context, frame geom.Frame, minX, maxX, minY, maxY, length, radius, pad float64, through []ExtrusionCircle, blind []ExtrusionBlindHole) error {
	if len(through)+len(blind) == 0 {
		return nil
	}
	holes, e := validateAxialExtrusionHoles(ctx, frame, through, blind, length, pad)
	if e != nil {
		return e
	}
	for _, h := range blind {
		if h.Depth <= pad || length-h.Depth <= pad {
			return geom.ErrDomain
		}
	}
	for _, h := range holes {
		for _, pair := range [][2]float64{{h.Center.X, minX}, {maxX, h.Center.X}, {h.Center.Y, minY}, {maxY, h.Center.Y}} {
			gap := new(big.Rat).Sub(ratInset(pair[0]), ratInset(pair[1]))
			gap.Sub(gap, ratInset(h.Radius))
			if gap.Cmp(ratInset(pad)) <= 0 {
				return geom.ErrDomain
			}
		}
		gap := new(big.Rat).Add(ratInset(radius), ratInset(h.Radius))
		gap.Add(gap, ratInset(pad))
		if extrusionCenterDistanceSquared(h, ExtrusionCircle{}).Cmp(gap.Mul(gap, gap)) <= 0 {
			return geom.ErrDomain
		}
	}
	return nil
}

func (b *cornerCylinderBuilder) addNotchedBoxBores(length float64, upperCap int, through []ExtrusionCircle, blind []ExtrusionBlindHole) error {
	for _, h := range through {
		if e := b.addBoxBoreAtCaps(h, length, [2]int{0, upperCap}); e != nil {
			return e
		}
	}
	for _, h := range blind {
		opening, floor, cap := 0., h.Depth, 0
		if h.OppositeCap {
			opening, floor, cap = length, length-h.Depth, upperCap
			if new(big.Rat).Sub(ratInset(length), ratInset(h.Depth)).Cmp(ratInset(floor)) != 0 {
				return geom.ErrUnrepresentable
			}
		}
		if e := b.addBoxBlindBore(h.ExtrusionCircle, opening, floor, cap); e != nil {
			return e
		}
	}
	return nil
}

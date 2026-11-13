package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SewPlanarThroughPrism subtracts a convex analytic prism whose two caps exactly
// coincide with selected parallel stock caps. Cap frames must share cardinal XY
// axes. The constructor proves cap containment, separation from existing holes,
// and separation from every other stock face before sewing reversed tool walls.
// Conservative bounds deliberately reject unproved near/tangent or curved-side
// intersections. Neither input is mutated; original stock face IDs are retained.
func SewPlanarThroughPrism(ctx context.Context, stock, tool Solid, stockLow, stockHigh, toolLow, toolHigh int) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if stockLow == stockHigh || toolLow == toolHigh || stockLow < 0 || stockHigh < 0 || toolLow < 0 || toolHigh < 0 || stockLow >= len(stock.faces) || stockHigh >= len(stock.faces) || toolLow >= len(tool.faces) || toolHigh >= len(tool.faces) {
		return Solid{}, geom.ErrInvalid
	}
	if len(stock.Topology().Shells) != 1 || len(tool.Topology().Shells) != 1 {
		return Solid{}, geom.ErrDomain
	}
	caps := []Face{stock.faces[stockLow], stock.faces[stockHigh], tool.faces[toolLow], tool.faces[toolHigh]}
	frames := make([]geom.Frame, 4)
	for i, f := range caps {
		p, ok := f.Surface.(geom.PlaneSurface)
		if !ok {
			return Solid{}, geom.ErrDomain
		}
		frames[i] = p.Frame()
		if !prismCardinal(frames[i]) || f.Reversed != (i%2 == 0) {
			return Solid{}, geom.ErrDomain
		}
		if i > 0 && (frames[i].XAxis() != frames[0].XAxis() || frames[i].YAxis() != frames[0].YAxis() || frames[i].ZAxis() != frames[0].ZAxis()) {
			return Solid{}, geom.ErrDomain
		}
	}
	offsets := make([]geom.Vec3, 4)
	for i, f := range frames {
		p, e := prismExactLocal(frames[0], f.Origin())
		if e != nil {
			return Solid{}, e
		}
		offsets[i] = p
	}
	height := offsets[1].Z
	if height <= 0 || offsets[2].Z != 0 || offsets[3].Z != height || offsets[2].X != offsets[3].X || offsets[2].Y != offsets[3].Y || len(caps[2].Holes) > 0 || len(caps[3].Holes) > 0 {
		return Solid{}, geom.ErrDomain
	}
	if len(caps[2].Outer) != len(caps[3].Outer) {
		return Solid{}, geom.ErrDomain
	}
	for i, u := range caps[2].Outer {
		v := caps[3].Outer[i]
		if u.UVStart != v.UVStart || u.UVEnd != v.UVEnd || !reflect.DeepEqual(u.Curve, v.Curve) {
			return Solid{}, geom.ErrDomain
		}
	}
	if e := prismConvexWire(caps[2].Outer); e != nil {
		return Solid{}, e
	}
	pad := 8 * max(stock.tolerance, tool.tolerance)
	if !finite(pad) {
		return Solid{}, geom.ErrInvalid
	}
	toolMin, toolMax, e := prismWireBounds(caps[2].Outer)
	if e != nil {
		return Solid{}, e
	}
	// The complete tool footprint and caps are bounded by this box in the
	// common sketch frame; support-separation proofs below use all eight corners.
	rawMin, rawMax := toolMin, toolMax
	var disk *prismDisk
	if circle, ok := prismWholeCircle(caps[2].Outer); ok {
		center := frames[2].ToWorld(geom.Vec3{X: circle.Center.X, Y: circle.Center.Y})
		if p, err := prismExactLocal(frames[2], center); err == nil && p == (geom.Vec3{X: circle.Center.X, Y: circle.Center.Y}) {
			disk = &prismDisk{center: center, axis: frames[2].ZAxis(), radius: circle.Cos.X}
		}
	}
	toolMin.X = math.Nextafter(toolMin.X+offsets[2].X, math.Inf(-1))
	toolMin.Y = math.Nextafter(toolMin.Y+offsets[2].Y, math.Inf(-1))
	toolMax.X = math.Nextafter(toolMax.X+offsets[2].X, math.Inf(1))
	toolMax.Y = math.Nextafter(toolMax.Y+offsets[2].Y, math.Inf(1))
	for i := 0; i < 2; i++ {
		lo := geom.Vec2{X: math.Nextafter(toolMin.X-offsets[i].X, math.Inf(-1)), Y: math.Nextafter(toolMin.Y-offsets[i].Y, math.Inf(-1))}
		hi := geom.Vec2{X: math.Nextafter(toolMax.X-offsets[i].X, math.Inf(1)), Y: math.Nextafter(toolMax.Y-offsets[i].Y, math.Inf(1))}
		if e := prismCapContains(caps[i], lo, hi, disk.inFrame(frames[i]), pad); e != nil {
			return Solid{}, fmt.Errorf("through prism cap %d: %w", i, e)
		}
	}
	capBox, e := parametric.SurfaceBounds(caps[2].Surface, parametric.Interval{Min: rawMin.X, Max: rawMax.X}, parametric.Interval{Min: rawMin.Y, Max: rawMax.Y})
	if e != nil {
		return Solid{}, e
	}
	upperBox, e := parametric.SurfaceBounds(caps[3].Surface, parametric.Interval{Min: rawMin.X, Max: rawMax.X}, parametric.Interval{Min: rawMin.Y, Max: rawMax.Y})
	if e != nil {
		return Solid{}, e
	}
	worldBox, e := capBox.Union(upperBox)
	if e != nil {
		return Solid{}, e
	}
	world := make([]geom.Vec3, 0, 8)
	for _, x := range []float64{worldBox.Min.X, worldBox.Max.X} {
		for _, y := range []float64{worldBox.Min.Y, worldBox.Max.Y} {
			for _, z := range []float64{worldBox.Min.Z, worldBox.Max.Z} {
				world = append(world, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
	}

	for i, f := range stock.faces {
		if i == stockLow || i == stockHigh {
			continue
		}
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if !prismMissesFace(f, world, worldBox, disk, pad) {
			return Solid{}, fmt.Errorf("through prism stock face %d (%T): %w", i, f.Surface, geom.ErrDomain)
		}
	}
	for i, f := range tool.faces {
		if i == toolLow || i == toolHigh {
			continue
		}
		if len(f.Holes) != 0 || len(f.Outer) != 4 {
			return Solid{}, geom.ErrDomain
		}
		switch s := f.Surface.(type) {
		case geom.PlaneSurface:
			if prismExactDot(s.Frame().ZAxis(), frames[0].ZAxis()).Sign() != 0 {
				return Solid{}, geom.ErrDomain
			}
		case geom.CylinderSurface:
			sf, _ := s.Parameters()
			if sf.ZAxis() != frames[0].ZAxis() && sf.ZAxis() != frames[0].ZAxis().Scale(-1) {
				return Solid{}, geom.ErrDomain
			}
		default:
			return Solid{}, geom.ErrDomain
		}
		low, high := 0, 0
		for _, u := range f.Outer {
			if u.Curve != nil {
				return Solid{}, geom.ErrDomain
			}
			edge := tool.edges[u.Edge]
			vertex := edge.Start
			if u.Reversed {
				vertex = edge.End
			}
			p, e := prismExactLocal(frames[0], tool.vertices[vertex])
			if e != nil {
				return Solid{}, e
			}
			if p.Z == 0 {
				low++
			} else if p.Z == height {
				high++
			} else {
				return Solid{}, geom.ErrDomain
			}
		}
		if low != 2 || high != 2 {
			return Solid{}, geom.ErrDomain
		}
	}

	for k, id := range []int{stockLow, stockHigh} {
		lo := geom.Vec2{X: math.Nextafter(toolMin.X-offsets[k].X, math.Inf(-1)), Y: math.Nextafter(toolMin.Y-offsets[k].Y, math.Inf(-1))}
		hi := geom.Vec2{X: math.Nextafter(toolMax.X-offsets[k].X, math.Inf(1)), Y: math.Nextafter(toolMax.Y-offsets[k].Y, math.Inf(1))}
		stock, e = prismRefineCapSeed(ctx, stock, id, lo, hi, disk.inFrame(frames[k]), pad)
		if e != nil {
			return Solid{}, e
		}
	}
	vertices := append(stock.Vertices(), tool.Vertices()...)
	edges := stock.Edges()
	edgeOffset := len(edges)
	vertexOffset := len(stock.vertices)
	for _, edge := range tool.edges {
		edge.Start += vertexOffset
		edge.End += vertexOffset
		edges = append(edges, edge)
	}
	faces := stock.Faces()
	for k, ids := range [][2]int{{stockLow, toolLow}, {stockHigh, toolHigh}} {
		shift := geom.Vec2{X: offsets[k+2].X - offsets[k].X, Y: offsets[k+2].Y - offsets[k].Y}
		wire, e := prismShiftWire(tool.faces[ids[1]].Outer, shift, edgeOffset)
		if e != nil {
			return Solid{}, e
		}
		hole := make([]Use, len(wire))
		for i, u := range wire {
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			u.Reversed = !u.Reversed
			u.Curve = reversePCurve(u.Curve)
			hole[len(wire)-1-i] = u
		}
		faces[ids[0]].Holes = append(faces[ids[0]].Holes, hole)
	}
	for i, f := range tool.Faces() {
		if i == toolLow || i == toolHigh {
			continue
		}
		f.Reversed = !f.Reversed
		for j := range f.Outer {
			f.Outer[j].Edge += edgeOffset
		}
		faces = append(faces, f)
	}
	return NewSolidContext(ctx, vertices, edges, faces, max(stock.tolerance, tool.tolerance))
}
func prismCardinal(f geom.Frame) bool {
	for _, v := range []geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()} {
		if math.Abs(v.X)+math.Abs(v.Y)+math.Abs(v.Z) != 1 || v.X != math.Trunc(v.X) || v.Y != math.Trunc(v.Y) || v.Z != math.Trunc(v.Z) {
			return false
		}
	}
	return true
}
func prismExactDot(a, b geom.Vec3) *big.Rat {
	v := new(big.Rat)
	for _, p := range [][2]float64{{a.X, b.X}, {a.Y, b.Y}, {a.Z, b.Z}} {
		v.Add(v, new(big.Rat).Mul(ratInset(p[0]), ratInset(p[1])))
	}
	return v
}
func prismExactLocal(f geom.Frame, p geom.Vec3) (geom.Vec3, error) {
	o := f.Origin()
	coords := [3]float64{}
	for i, axis := range []geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()} {
		sum := new(big.Rat)
		for _, v := range [][3]float64{{p.X, o.X, axis.X}, {p.Y, o.Y, axis.Y}, {p.Z, o.Z, axis.Z}} {
			sum.Add(sum, new(big.Rat).Mul(new(big.Rat).Sub(ratInset(v[0]), ratInset(v[1])), ratInset(v[2])))
		}
		x, e := geom.RepresentedExactScalar(sum)
		if e != nil {
			return geom.Vec3{}, e
		}
		coords[i] = x
	}
	return geom.Vec3{X: coords[0], Y: coords[1], Z: coords[2]}, nil
}
func prismCircleCurve(c PCurve) (EllipseUV, bool) {
	switch v := c.(type) {
	case EllipseUV:
		return v, true
	case anchoredCircularUV:
		return v.EllipseUV, true
	case reverseTrim:
		c, ok := prismCircleCurve(v.differentiableTrim)
		c.From, c.To = c.To, c.From
		return c, ok
	case subTrim:
		c, ok := prismCircleCurve(v.base)
		if !ok {
			return EllipseUV{}, false
		}
		a, b := c.From, c.To
		c.From = (1-v.min)*a + v.min*b
		c.To = (1-v.max)*a + v.max*b
		return c, true
	}
	return EllipseUV{}, false
}
func prismConvexWire(w []Use) error {
	if len(w) < 3 || len(w) > 64 {
		return geom.ErrDomain
	}
	for _, u := range w {
		if u.Curve != nil {
			c, ok := prismCircleCurve(u.Curve)
			if !ok || c.Linear != (geom.Vec2{}) || c.Cos.X <= 0 || c.Cos.Y != 0 || c.Sin.X != 0 || c.Sin.Y != c.Cos.X || c.To <= c.From || c.To-c.From > math.Pi/2 {
				return geom.ErrDomain
			}
		}
		for _, v := range w {
			turn, e := geom.Orient2D(u.UVStart, u.UVEnd, v.UVStart)
			if e != nil || turn < 0 {
				return geom.ErrDomain
			}
		}
	}
	return nil
}
func prismWireBounds(w []Use) (geom.Vec2, geom.Vec2, error) {
	lo, hi := geom.Vec2{X: math.Inf(1), Y: math.Inf(1)}, geom.Vec2{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, u := range w {
		a, b, e := trimBounds(u)
		if e != nil {
			return lo, hi, e
		}
		lo.X = min(lo.X, a.X)
		lo.Y = min(lo.Y, a.Y)
		hi.X = max(hi.X, b.X)
		hi.Y = max(hi.Y, b.Y)
	}
	return lo, hi, nil
}
func prismCapContains(f Face, lo, hi geom.Vec2, disk *prismCircle, pad float64) error {
	if e := prismConvexWire(f.Outer); e != nil {
		return e
	}
	corners := []geom.Vec2{{X: lo.X - pad, Y: lo.Y - pad}, {X: hi.X + pad, Y: lo.Y - pad}, {X: hi.X + pad, Y: hi.Y + pad}, {X: lo.X - pad, Y: hi.Y + pad}}
	first, circle := prismWholeCircle(f.Outer)
	if circle {
		if disk != nil {
			if !disk.inside(first.Center, ratInset(first.Cos.X), pad) {
				return geom.ErrDomain
			}
		} else {
			r2 := new(big.Rat).Mul(ratInset(first.Cos.X), ratInset(first.Cos.X))
			for _, p := range corners {
				if prismDistanceSquared(p, first.Center).Cmp(r2) >= 0 {
					return geom.ErrDomain
				}
			}
		}
	} else {
		for _, u := range f.Outer {
			for _, p := range corners {
				sign, e := geom.Orient2D(u.UVStart, u.UVEnd, p)
				if e != nil || sign <= 0 {
					return geom.ErrDomain
				}
			}
		}
	}
	for _, hole := range f.Holes {
		if disk != nil {
			circle, ok := prismWholeCircle(hole)
			if ok && disk.outside(circle.Center, ratInset(circle.Cos.X), pad) {
				continue
			}
		}
		a, b, e := prismWireBounds(hole)
		if e != nil {
			return e
		}
		if !(hi.X+pad < a.X || lo.X-pad > b.X || hi.Y+pad < a.Y || lo.Y-pad > b.Y) {
			return geom.ErrDomain
		}
	}
	return nil
}
func prismShiftWire(w []Use, d geom.Vec2, offset int) ([]Use, error) {
	out := append([]Use(nil), w...)
	for i, u := range out {
		u.Edge += offset
		var e error
		if c, ok := u.Curve.(anchoredCircularUV); ok {
			c.Center, e = prismExactAdd(c.Center, d)
			if e != nil {
				return nil, e
			}
			// Keep the exact analytic center and radius. The cardinal UV
			// representatives use the same rounded sums as the model edge.
			c.Start = geom.Vec2{X: c.Start.X + d.X, Y: c.Start.Y + d.Y}
			c.End = geom.Vec2{X: c.End.X + d.X, Y: c.End.Y + d.Y}
			u.UVStart, u.UVEnd = c.Start, c.End
			u.Curve = c
			out[i] = u
			continue
		}
		u.UVStart, e = prismExactAdd(u.UVStart, d)
		if e != nil {
			return nil, e
		}
		u.UVEnd, e = prismExactAdd(u.UVEnd, d)
		if e != nil {
			return nil, e
		}
		if u.Curve != nil {
			switch c := u.Curve.(type) {
			case EllipseUV:
				c.Center, e = prismExactAdd(c.Center, d)
				if e != nil {
					return nil, e
				}
				u.Curve = c
			default:
				return nil, geom.ErrDomain
			}
		}
		out[i] = u
	}
	return out, nil
}
func prismMissesFace(f Face, world []geom.Vec3, toolBox geom.Box, disk *prismDisk, pad float64) bool {
	d, e := faceUVBounds(f)
	if e != nil {
		return false
	}
	box, e := parametric.SurfaceBounds(f.Surface, parametric.Interval{Min: d.UMin, Max: d.UMax}, parametric.Interval{Min: d.VMin, Max: d.VMax})
	if e == nil && (box.Max.X+pad < toolBox.Min.X || toolBox.Max.X+pad < box.Min.X || box.Max.Y+pad < toolBox.Min.Y || toolBox.Max.Y+pad < box.Min.Y || box.Max.Z+pad < toolBox.Min.Z || toolBox.Max.Z+pad < box.Min.Z) {
		return true
	}
	if p, ok := f.Surface.(geom.PlaneSurface); ok {
		normal, origin := p.Frame().ZAxis(), p.Frame().Origin()
		positive, negative := true, true
		for _, v := range world {
			value := new(big.Rat)
			for _, c := range [][3]float64{{v.X, origin.X, normal.X}, {v.Y, origin.Y, normal.Y}, {v.Z, origin.Z, normal.Z}} {
				value.Add(value, new(big.Rat).Mul(new(big.Rat).Sub(ratInset(c[0]), ratInset(c[1])), ratInset(c[2])))
			}
			positive = positive && value.Cmp(ratInset(pad)) > 0
			negative = negative && value.Cmp(ratInset(-pad)) < 0
		}
		if positive || negative {
			return true
		}
		if profile := disk.inFrame(p.Frame()); profile != nil {
			for _, hole := range f.Holes {
				c, ok := prismWholeCircle(hole)
				if ok && profile.inside(c.Center, ratInset(c.Cos.X), pad) {
					return true
				}
			}
			c, ok := prismWholeCircle(f.Outer)
			return ok && profile.outside(c.Center, ratInset(c.Cos.X), pad)
		}
		return false
	}
	var frame geom.Frame
	var minimum, maximum *big.Rat
	switch s := f.Surface.(type) {
	case geom.CylinderSurface:
		f, r := s.Parameters()
		frame = f
		minimum, maximum = ratInset(r), ratInset(r)
	case geom.TorusSurface:
		f, major, minor := s.Parameters()
		frame = f
		minimum = new(big.Rat).Sub(ratInset(major), ratInset(minor))
		maximum = new(big.Rat).Add(ratInset(major), ratInset(minor))
	case geom.ConeSurface:
		f, slope := s.Parameters()
		frame = f
		minimum = new(big.Rat).Mul(ratInset(max(0, d.VMin)), ratInset(slope))
		maximum = new(big.Rat).Mul(ratInset(max(0, d.VMax)), ratInset(slope))
	default:
		f, a, b, ok := parametric.RevolutionSideParameters(f.Surface)
		if !ok || d.VMin < 0 || d.VMax > 1 {
			return false
		}
		frame = f
		ra := new(big.Rat).Add(ratInset(a.X), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(b.X), ratInset(a.X)), ratInset(d.VMin)))
		rb := new(big.Rat).Add(ratInset(a.X), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(b.X), ratInset(a.X)), ratInset(d.VMax)))
		minimum, maximum = ra, rb
		if ra.Cmp(rb) > 0 {
			minimum, maximum = rb, ra
		}
	}
	if !prismCardinal(frame) {
		return false
	}
	if profile := disk.inFrame(frame); profile != nil &&
		(profile.inside(geom.Vec2{}, minimum, pad) || profile.outside(geom.Vec2{}, maximum, pad)) {
		return true
	}
	lo, hi := geom.Vec2{X: math.Inf(1), Y: math.Inf(1)}, geom.Vec2{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, p := range world {
		v, e := prismExactLocal(frame, p)
		if e != nil {
			return false
		}
		lo.X = min(lo.X, v.X)
		lo.Y = min(lo.Y, v.Y)
		hi.X = max(hi.X, v.X)
		hi.Y = max(hi.Y, v.Y)
	}
	nearest := func(a, b float64) float64 {
		if a <= 0 && b >= 0 {
			return 0
		}
		return min(math.Abs(a), math.Abs(b))
	}
	squared := func(x, y float64) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Mul(ratInset(x), ratInset(x)), new(big.Rat).Mul(ratInset(y), ratInset(y)))
	}
	low2 := squared(nearest(lo.X, hi.X), nearest(lo.Y, hi.Y))
	high2 := squared(max(math.Abs(lo.X), math.Abs(hi.X)), max(math.Abs(lo.Y), math.Abs(hi.Y)))
	minimum.Sub(minimum, ratInset(pad))
	maximum.Add(maximum, ratInset(pad))
	if minimum.Sign() > 0 && high2.Cmp(new(big.Rat).Mul(minimum, minimum)) < 0 {
		return true
	}
	return low2.Cmp(new(big.Rat).Mul(maximum, maximum)) > 0
}

func prismExactAdd(a, b geom.Vec2) (geom.Vec2, error) {
	x, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(a.X), ratInset(b.X)))
	if e != nil {
		return geom.Vec2{}, e
	}
	y, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(a.Y), ratInset(b.Y)))
	return geom.Vec2{X: x, Y: y}, e
}

// Refining a circular cap's analytic coedges keeps its coarse chord seed from
// excluding a hole proved inside the actual circle. Model edges remain shared;
// at most four bisections of the initial cap are attempted.
func prismRefineCapSeed(ctx context.Context, s Solid, id int, lo, hi geom.Vec2, disk *prismCircle, pad float64) (Solid, error) {
	if _, ok := prismWholeCircle(s.faces[id].Outer); !ok {
		return s, nil
	}
	for round := 0; round <= 4; round++ {
		inside := true
		for _, u := range s.faces[id].Outer {
			if disk != nil {
				inside = inside && disk.leftOf(u.UVStart, u.UVEnd, pad)
				continue
			}
			for _, p := range []geom.Vec2{{X: lo.X - pad, Y: lo.Y - pad}, {X: hi.X + pad, Y: lo.Y - pad}, {X: hi.X + pad, Y: hi.Y + pad}, {X: lo.X - pad, Y: hi.Y + pad}} {
				turn, e := geom.Orient2D(u.UVStart, u.UVEnd, p)
				inside = inside && e == nil && turn > 0
			}
		}
		if inside {
			return s, nil
		}
		if round == 4 || len(s.faces[id].Outer) >= 64 {
			return Solid{}, parametric.ErrBudget
		}
		uses := append([]Use(nil), s.faces[id].Outer...)
		for _, u := range uses {
			if e := ctx.Err(); e != nil {
				return Solid{}, e
			}
			edge := s.edges[u.Edge]
			var e error
			s, _, e = s.splitEdgeDraft(u.Edge, (edge.Range.Min+edge.Range.Max)/2)
			if e != nil {
				return Solid{}, e
			}
		}
	}
	return Solid{}, parametric.ErrBudget
}
func prismWholeCircle(w []Use) (EllipseUV, bool) {
	if len(w) < 4 || len(w) > 64 {
		return EllipseUV{}, false
	}
	curves := make([]EllipseUV, 0, len(w))
	var first EllipseUV
	for i, u := range w {
		c, ok := prismCircleCurve(u.Curve)
		if !ok {
			return EllipseUV{}, false
		}
		if i == 0 {
			first = c
		}
		if c.From > c.To {
			c.From, c.To = c.To, c.From
		}
		if c.Center != first.Center || c.Cos != first.Cos || c.Sin != first.Sin || c.Linear != (geom.Vec2{}) ||
			c.Cos.X <= 0 || c.Cos.Y != 0 || c.Sin.X != 0 || math.Abs(c.Sin.Y) != c.Cos.X ||
			c.To <= c.From || c.To-c.From > math.Pi/2 {
			return EllipseUV{}, false
		}
		curves = append(curves, c)
	}
	sort.Slice(curves, func(i, j int) bool { return curves[i].From < curves[j].From })
	if curves[0].From != 0 || curves[len(curves)-1].To != 2*math.Pi {
		return EllipseUV{}, false
	}
	for i := 1; i < len(curves); i++ {
		if curves[i-1].To != curves[i].From {
			return EllipseUV{}, false
		}
	}
	return first, true
}

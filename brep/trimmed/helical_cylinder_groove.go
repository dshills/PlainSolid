package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// GrooveCylinderWithHelicalPolygon subtracts a blind-ended constant-lead
// groove from cylindrical stock. The convex meridian must meet the stock
// radius in precisely one side, with all remaining vertices strictly inside.
// The open groove must stay inside one cylinder chart period and clear both
// original caps. The result retains the original cylinder, its cap supports,
// both helical rims and the exact analytic cutter faces inside the stock.
// Seam-crossing grooves and lead-in or runout features remain unresolved.
func GrooveCylinderWithHelicalPolygon(ctx context.Context, frame geom.Frame, stockRadius, height float64,
	profile []geom.Vec2, lead, start, end float64, hand geom.HelixHandedness) (Solid, error) {
	if ctx == nil || !finite(stockRadius) || stockRadius <= 0 || !finite(height) || height <= 0 ||
		!finite(lead) || lead <= 0 || !finite(start) || !finite(end) || end <= start || end-start >= 1 ||
		hand != geom.LeftHanded && hand != geom.RightHanded || len(profile) > 32 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	n := len(p)
	outer := -1
	for i, a := range p {
		b, c := p[(i+1)%n], p[(i+2)%n]
		sign, e := geom.Orient2D(a, b, c)
		if e != nil || sign <= 0 || a.X <= 0 || a.X > stockRadius {
			return Solid{}, geom.ErrDomain
		}
		if a.X == stockRadius && b.X == stockRadius {
			if outer >= 0 {
				return Solid{}, geom.ErrDomain
			}
			outer = i
		}
		z0 := new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(start)))
		z1 := new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(end)))
		if z0.Sign() <= 0 || z1.Cmp(ratInset(height)) >= 0 {
			return Solid{}, geom.ErrConvergence
		}
	}
	if outer < 0 {
		return Solid{}, geom.ErrDomain
	}
	for i, a := range p {
		if i != outer && i != (outer+1)%n && a.X == stockRadius {
			return Solid{}, geom.ErrDomain
		}
	}
	phaseStart, phaseEnd := float64(hand)*start, float64(hand)*end
	period := math.Floor(min(phaseStart, phaseEnd))
	if math.Abs(period) > 1<<20 || min(phaseStart, phaseEnd) <= period || max(phaseStart, phaseEnd) >= period+1 {
		return Solid{}, geom.ErrConvergence
	}
	stock, e := parametric.Cylinder(frame, stockRadius, height)
	if e != nil {
		return Solid{}, e
	}
	base, e := FromParametric(stock)
	if e != nil {
		return Solid{}, e
	}
	tool, e := SweepHelicalPolygon(ctx, frame, p, lead, start, end, hand)
	if e != nil {
		return Solid{}, e
	}
	project := func(uv geom.Vec2) (geom.Vec2, error) {
		var z float64
		if uv.Y == 0 {
			z = p[outer].Y
		} else {
			z = p[(outer+1)%n].Y
		}
		p := geom.Vec2{X: 2 * math.Pi * (float64(hand)*uv.X - period), Y: lead*uv.X + z}
		if p.X <= 0 || p.X >= 2*math.Pi || p.Y <= 0 || p.Y >= height {
			return geom.Vec2{}, geom.ErrUnrepresentable
		}
		return p, nil
	}
	return sewHelicalGroove(ctx, base, tool, outer, hand, project)
}

// GrooveCylinderWithHelicalSeam extends the blind-ended cylindrical groove
// to multiple turns. A shear of the stock's angular chart follows the
// helical cut, allowing the complete opening to be an interior trim in one
// chart. Lead must have an exactly representable reciprocal so the stock
// chart and helical supports have the same analytic pitch. The original
// material is one capped cylinder; no separate seam material is introduced.
func GrooveCylinderWithHelicalSeam(ctx context.Context, frame geom.Frame, stockRadius, height float64,
	profile []geom.Vec2, lead, start, end float64, hand geom.HelixHandedness) (Solid, error) {
	return GrooveCylinderWithHelicalStarts(ctx, frame, stockRadius, height, profile, lead, start, end, hand, 1)
}

// GrooveCylinderWithHelicalStarts places evenly phased grooves by axial
// offsets of lead/starts over the same physical turn window. It proves that
// each profile's full axial width is strictly smaller than the start spacing.
// Native chart trims require exactly representable offsets and coordinates.
// This operation does not open any groove through a stock cap.
func GrooveCylinderWithHelicalStarts(ctx context.Context, frame geom.Frame, stockRadius, height float64,
	profile []geom.Vec2, lead, start, end float64, hand geom.HelixHandedness, starts int) (Solid, error) {
	if ctx == nil || !finite(stockRadius) || stockRadius <= 0 || !finite(height) || height <= 0 ||
		!finite(lead) || lead <= 0 || !finite(start) || !finite(end) || end <= start || end-start > 16 ||
		hand != geom.LeftHanded && hand != geom.RightHanded || len(profile) > 32 || starts < 1 || starts > 8 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	n := len(p)
	if starts*(n+1)+3 > 34 {
		return Solid{}, parametric.ErrBudget
	}
	spacing := new(big.Rat).Quo(ratInset(lead), big.NewRat(int64(starts), 1))
	lastOffset := new(big.Rat).Mul(spacing, big.NewRat(int64(starts-1), 1))
	low, high := p[0].Y, p[0].Y
	outer := -1
	for i, a := range p {
		low, high = min(low, a.Y), max(high, a.Y)
		if a.X <= 0 || a.X > stockRadius {
			return Solid{}, geom.ErrDomain
		}
		b := p[(i+1)%n]
		for j, c := range p {
			if j == i || j == (i+1)%n {
				continue
			}
			q, e := geom.Orient2D(a, b, c)
			if e != nil || q <= 0 {
				return Solid{}, geom.ErrDomain
			}
		}
		if a.X == stockRadius && b.X == stockRadius {
			if outer >= 0 {
				return Solid{}, geom.ErrDomain
			}
			outer = i
		}
		z0 := new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(start)))
		z1 := new(big.Rat).Add(lastOffset, new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(end))))
		if z0.Sign() <= 0 || z1.Cmp(ratInset(height)) >= 0 {
			return Solid{}, geom.ErrConvergence
		}
	}
	if new(big.Rat).Sub(ratInset(high), ratInset(low)).Cmp(spacing) >= 0 {
		return Solid{}, geom.ErrDomain
	}
	if outer < 0 {
		return Solid{}, geom.ErrDomain
	}
	for i, q := range p {
		if i != outer && i != (outer+1)%n && q.X == stockRadius {
			return Solid{}, geom.ErrDomain
		}
	}
	tw := new(big.Rat).Quo(big.NewRat(int64(hand), 1), ratInset(lead))
	twist, e := geom.RepresentedExactScalar(tw)
	if e != nil {
		return Solid{}, e
	}
	// Place the seam strictly between two starts. The same gap separates
	// their analytic openings throughout the wall chart.
	center := new(big.Rat).Quo(new(big.Rat).Add(ratInset(low), ratInset(high)), big.NewRat(2, 1))
	halfStep := new(big.Rat).Quo(big.NewRat(1, 2), big.NewRat(int64(starts), 1))
	phaseExact := new(big.Rat).Neg(new(big.Rat).Add(halfStep, new(big.Rat).Mul(tw, center)))
	phase, e := geom.RepresentedExactScalar(phaseExact)
	if e != nil {
		return Solid{}, e
	}
	stock, e := helicalSeamCylinder(ctx, frame, stockRadius, height, twist, phase)
	if e != nil {
		return Solid{}, e
	}
	for j := range starts {
		shift := new(big.Rat).Mul(spacing, big.NewRat(int64(j), 1))
		q := append([]geom.Vec2(nil), p...)
		for i := range q {
			q[i].Y, e = geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(q[i].Y), shift))
			if e != nil {
				return Solid{}, e
			}
		}
		zEnds := [2]*big.Rat{ratInset(q[outer].Y), ratInset(q[(outer+1)%n].Y)}
		var sideU [2]float64
		for i, z := range zEnds {
			u := new(big.Rat).Neg(new(big.Rat).Add(phaseExact, new(big.Rat).Mul(tw, z)))
			if u.Sign() <= 0 {
				u.Add(u, big.NewRat(1, 1))
			}
			if u.Sign() <= 0 || u.Cmp(big.NewRat(1, 1)) >= 0 {
				return Solid{}, geom.ErrDomain
			}
			sideU[i], e = geom.RepresentedExactScalar(u)
			if e != nil {
				return Solid{}, e
			}
		}
		tool, e := SweepHelicalPolygon(ctx, frame, q, lead, start, end, hand)
		if e != nil {
			return Solid{}, e
		}
		project := func(uv geom.Vec2) (geom.Vec2, error) {
			k := int(uv.Y)
			if k < 0 || k > 1 || float64(k) != uv.Y {
				return geom.Vec2{}, geom.ErrDomain
			}
			z := new(big.Rat).Add(zEnds[k], new(big.Rat).Mul(ratInset(lead), ratInset(uv.X)))
			v, e := geom.RepresentedExactScalar(z)
			return geom.Vec2{X: sideU[k], Y: v}, e
		}
		stock, e = sewHelicalGroove(ctx, stock, tool, outer, hand, project)
		if e != nil {
			return Solid{}, e
		}
	}
	return stock, nil
}

func helicalSeamCylinder(ctx context.Context, frame geom.Frame, radius, height, twist, phase float64) (Solid, error) {
	wall, e := geom.NewTwistedCylinderSurface(frame, radius, twist, phase)
	if e != nil {
		return Solid{}, e
	}
	vertices := make([]geom.Vec3, 2)
	for i, z := range []float64{0, height} {
		vertices[i], e = wall.Eval(0, z)
		if e != nil {
			return Solid{}, e
		}
	}
	uv := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: height}, {Y: height}}
	edges := []parametric.Edge{
		{Start: 0, End: 0, Curve: surfaceTrimCurve{wall, uv[0], uv[1], nil}, Range: parametric.Interval{Max: 1}},
		{Start: 0, End: 1, Curve: surfaceTrimCurve{wall, uv[1], uv[2], nil}, Range: parametric.Interval{Max: 1}},
		{Start: 1, End: 1, Curve: surfaceTrimCurve{wall, uv[3], uv[2], nil}, Range: parametric.Interval{Max: 1}},
	}
	f := Face{Surface: wall}
	for i, id := range [4]int{0, 1, 2, 1} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: uv[i], UVEnd: uv[(i+1)%4]})
	}
	faces := []Face{f}
	for i, z := range []float64{0, height} {
		theta := 2 * math.Pi * math.Remainder(twist*z+phase, 1)
		x := frame.XAxis().Scale(math.Cos(theta)).Add(frame.YAxis().Scale(math.Sin(theta)))
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), x, frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		cap, e := parametric.Cylinder(f, radius, 1)
		if e != nil {
			return Solid{}, e
		}
		face := cap.Faces()[1] // the lower polar disk has the desired origin
		p := [4]geom.Vec2{{}, {X: 2 * math.Pi}, {X: 2 * math.Pi, Y: radius}, {Y: radius}}
		center := len(vertices)
		vertices = append(vertices, f.Origin())
		pole := len(edges)
		edges = append(edges, parametric.Edge{Start: center, End: center})
		seam := len(edges)
		edges = append(edges, parametric.Edge{Start: center, End: i, Range: parametric.Interval{Max: 1}, Curve: surfaceTrimCurve{face.Surface, p[1], p[2], nil}})
		f2 := Face{Surface: face.Surface, Reversed: i == 1}
		for j, id := range [4]int{pole, seam, 2 * i, seam} {
			f2.Outer = append(f2.Outer, Use{Edge: id, Reversed: j >= 2, UVStart: p[j], UVEnd: p[(j+1)%4]})
		}
		faces = append(faces, f2)
	}
	return NewSolidContext(ctx, vertices, edges, faces, max(radius, height)*1e-11)
}

func sewHelicalGroove(ctx context.Context, stock, tool Solid, outer int, hand geom.HelixHandedness,
	project func(geom.Vec2) (geom.Vec2, error)) (Solid, error) {
	v, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	vertexBase, edgeBase := len(v), len(edges)
	v = append(v, tool.vertices...)
	for _, e := range tool.edges {
		e.Start += vertexBase
		e.End += vertexBase
		edges = append(edges, e)
	}
	opening := tool.faces[outer+2]
	var hole []Use
	for i := range opening.Outer {
		k := i
		if hand == geom.RightHanded {
			k = len(opening.Outer) - 1 - i
		}
		u := opening.Outer[k]
		if hand == geom.RightHanded {
			u.UVStart, u.UVEnd, u.Reversed = u.UVEnd, u.UVStart, !u.Reversed
		}
		u.Edge += edgeBase
		var e error
		u.UVStart, e = project(u.UVStart)
		if e != nil {
			return Solid{}, e
		}
		u.UVEnd, e = project(u.UVEnd)
		if e != nil {
			return Solid{}, e
		}
		hole = append(hole, u)
	}
	faces[0].Holes = append(faces[0].Holes, hole)
	for i, f := range tool.faces {
		if i == outer+2 {
			continue
		}
		f = cloneFace(f)
		f.Reversed = !f.Reversed
		for i := range f.Outer {
			f.Outer[i].Edge += edgeBase
		}
		faces = append(faces, f)
	}
	return NewSolidContext(ctx, v, edges, faces, max(stock.tolerance, tool.tolerance))
}

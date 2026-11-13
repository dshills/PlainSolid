package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalCylinderCommon intersects a constant-lead polygon sweep with a
// coaxial cylindrical band in the same frame, with axial bounds in that
// frame. When the cylindrical caps do not cut the radial overlap, radial
// clipping commutes with the sweep. The returned solid retains the exact
// analytic helical supports. A cap-crossing result remains unresolved.
func HelicalCylinderCommon(ctx context.Context, frame geom.Frame, profile []geom.Vec2,
	lead, start, end float64, hand geom.HelixHandedness, inner, outer, bottom, top float64) (Solid, error) {
	if ctx == nil || !finite(inner) || !finite(outer) || inner < 0 || outer <= inner ||
		!finite(bottom) || !finite(top) || bottom >= top || !finite(lead) || lead <= 0 ||
		!finite(start) || !finite(end) || end <= start || end-start > 16 || len(profile) > 32 ||
		hand != geom.RightHanded && hand != geom.LeftHanded {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	lo, hi := p[0].Y, p[0].Y
	for _, q := range p {
		if q.X <= 0 {
			return Solid{}, geom.ErrDomain
		}
		lo, hi = min(lo, q.Y), max(hi, q.Y)
	}
	if new(big.Rat).Sub(ratInset(hi), ratInset(lo)).Cmp(ratInset(lead)) >= 0 {
		return Solid{}, geom.ErrDomain
	}
	// A convex meridian clips to a single convex component. Disconnected
	// intersections of nonconvex meridians are outside this entry point.
	for i, a := range p {
		b, c := p[(i+1)%len(p)], p[(i+2)%len(p)]
		s, e := geom.Orient2D(a, b, c)
		if e != nil || s <= 0 {
			return Solid{}, geom.ErrConvergence
		}
	}
	var poly [][2]*big.Rat
	for _, x := range p {
		poly = append(poly, [2]*big.Rat{ratInset(x.X), ratInset(x.Y)})
	}
	for i, r := range []*big.Rat{ratInset(inner), ratInset(outer)} {
		var next [][2]*big.Rat
		add := func(x [2]*big.Rat) {
			if len(next) == 0 || next[len(next)-1][0].Cmp(x[0]) != 0 || next[len(next)-1][1].Cmp(x[1]) != 0 {
				next = append(next, x)
			}
		}
		for j, a := range poly {
			b := poly[(j+1)%len(poly)]
			x, y := a[0].Cmp(r)*(1-2*i), b[0].Cmp(r)*(1-2*i)
			if x >= 0 {
				add(a)
			}
			if x*y < 0 {
				t := new(big.Rat).Quo(new(big.Rat).Sub(r, a[0]), new(big.Rat).Sub(b[0], a[0]))
				v := new(big.Rat).Add(a[1], new(big.Rat).Mul(t, new(big.Rat).Sub(b[1], a[1])))
				add([2]*big.Rat{r, v})
			}
		}
		if len(next) < 3 {
			return Solid{}, nil
		}
		poly = next
	}
	a, b := new(big.Rat).Mul(ratInset(lead), ratInset(start)), new(big.Rat).Mul(ratInset(lead), ratInset(end))
	zMin, zMax := new(big.Rat).Add(poly[0][1], a), new(big.Rat).Add(poly[0][1], b)
	for _, q := range poly[1:] {
		zMin = minRat(zMin, new(big.Rat).Add(q[1], a))
		zMax = maxRat(zMax, new(big.Rat).Add(q[1], b))
	}
	if zMax.Cmp(ratInset(bottom)) <= 0 || zMin.Cmp(ratInset(top)) >= 0 {
		return Solid{}, nil
	}
	if zMin.Cmp(ratInset(bottom)) < 0 || zMax.Cmp(ratInset(top)) > 0 {
		return Solid{}, geom.ErrConvergence
	}
	clipped := make([]geom.Vec2, len(poly))
	for i, q := range poly {
		r, e := geom.RepresentedExactScalar(q[0])
		if e != nil {
			return Solid{}, e
		}
		z, e := geom.RepresentedExactScalar(q[1])
		if e != nil {
			return Solid{}, e
		}
		clipped[i] = geom.Vec2{X: r, Y: z}
	}
	return SweepHelicalPolygon(ctx, frame, clipped, lead, start, end, hand)
}

// SweepHelicalPolygon builds the boundary of a constant-lead, finite sweep of
// a simple meridian polygon. X is radius; Y is axial offset. Every radius is
// positive. The exact axial width of the complete profile must be less than
// one lead, so equal angular/radial coordinates at different turns cannot
// identify interior material or boundary points. Planar ends and the original
// helical profile sides share authoritative model curves. This builds the
// sweep itself, rather than cutting it from another body.
func SweepHelicalPolygon(ctx context.Context, frame geom.Frame, profile []geom.Vec2, lead, start, end float64,
	hand geom.HelixHandedness) (Solid, error) {
	return sweepHelicalPolygon(ctx, frame, profile, lead, start, end, hand, true)
}

func sweepHelicalPolygon(ctx context.Context, frame geom.Frame, profile []geom.Vec2, lead, start, end float64,
	hand geom.HelixHandedness, validateDisplay bool) (Solid, error) {
	if ctx == nil || !finite(lead) || lead <= 0 || !finite(start) || !finite(end) || end <= start ||
		end-start > 16 || len(profile) > 32 || !finite(lead*start) || !finite(lead*end) || hand != geom.RightHanded && hand != geom.LeftHanded {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p, scale, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	lo, hi := p[0].Y, p[0].Y
	for _, x := range p {
		if x.X <= 0 {
			return Solid{}, geom.ErrDomain
		}
		lo, hi = min(lo, x.Y), max(hi, x.Y)
	}
	width := new(big.Rat).Sub(ratInset(hi), ratInset(lo))
	if width.Cmp(ratInset(lead)) >= 0 {
		return Solid{}, geom.ErrDomain
	}
	tol := max(scale, lead*(end-start)) * 1e-11
	if !finite(tol) || tol <= 0 || lead*(end-start) <= 8*tol {
		return Solid{}, geom.ErrUnrepresentable
	}
	var caps [2]geom.PlaneSurface
	for i, u := range [2]float64{start, end} {
		a := 2 * math.Pi * float64(hand) * math.Remainder(u, 1)
		x := frame.XAxis().Scale(math.Cos(a)).Add(frame.YAxis().Scale(math.Sin(a)))
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: lead * u}), x, x.Cross(frame.ZAxis()))
		if e != nil {
			return Solid{}, e
		}
		caps[i], e = geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
	}
	n := len(p)
	vertices := make([]geom.Vec3, 0, 2*n)
	edges := make([]parametric.Edge, 0, 3*n)
	faces := make([]Face, 0, n+2)
	add := func(i, j int, s geom.Surface, a, b geom.Vec2) {
		edges = append(edges, parametric.Edge{Start: i, End: j, Range: parametric.Interval{Max: 1}, Curve: surfaceTrimCurve{s, a, b, nil}})
	}
	for op, cap := range caps {
		f := Face{Surface: cap, Reversed: (op == 1) != (hand == geom.LeftHanded)}
		for i, a := range p {
			j := (i + 1) % n
			v, e := cap.Eval(a.X, a.Y)
			if e != nil {
				return Solid{}, e
			}
			vertices = append(vertices, v)
			f.Outer = append(f.Outer, Use{Edge: len(edges), UVStart: a, UVEnd: p[j]})
			add(op*n+i, op*n+j, cap, a, p[j])
		}
		faces = append(faces, f)
	}
	uv := [4]geom.Vec2{{X: start}, {X: end}, {X: end, Y: 1}, {X: start, Y: 1}}
	for i, a := range p {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		j := (i + 1) % n
		wall, e := geom.NewHelicalProfileSurface(frame, a, p[j], lead, hand, start, end)
		if e != nil {
			return Solid{}, e
		}
		add(i, n+i, wall, uv[0], uv[1])
		f := Face{Surface: wall, Reversed: hand == geom.LeftHanded}
		for k, id := range [4]int{2*n + i, n + i, 2*n + j, i} {
			f.Outer = append(f.Outer, Use{Edge: id, Reversed: k >= 2, UVStart: uv[k], UVEnd: uv[(k+1)%4]})
		}
		faces = append(faces, f)
	}
	return newSolidContext(ctx, vertices, edges, faces, tol, validateDisplay)
}

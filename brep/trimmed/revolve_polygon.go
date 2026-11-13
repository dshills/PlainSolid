package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RevolvePolygon sweeps a simple meridian polygon about frame Z, retaining
// analytic surfaces. Radius is meridian X and height is Y. A signed angle
// selects a sweep from frame X in either direction; a full turn omits end
// faces. A partial turn caps the two original meridian loops, which may be
// concave. One contiguous axis side is supported and uses shared pole IDs.
func RevolvePolygon(ctx context.Context, frame geom.Frame, profile []geom.Vec2, angle float64) (Solid, error) {
	if ctx == nil || !finite(angle) || angle == 0 || math.Abs(angle) > 2*math.Pi {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if math.Abs(angle) == 2*math.Pi {
		s, e := parametric.Revolve(frame, profile)
		if e != nil {
			return Solid{}, e
		}
		return FromParametric(s)
	}
	p, scale, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	lo, hi := min(0, angle), max(0, angle)
	tol := scale * 1e-12
	for _, q := range p {
		if q.X > 0 && q.X*min(hi-lo, 2*math.Pi-(hi-lo)) <= 8*tol {
			return Solid{}, geom.ErrUnrepresentable
		}
	}
	var caps [2]geom.PlaneSurface
	for i, u := range [2]float64{lo, hi} {
		x := frame.XAxis().Scale(math.Cos(u)).Add(frame.YAxis().Scale(math.Sin(u)))
		f, e := geom.NewFrame(frame.Origin(), x, x.Cross(frame.ZAxis()))
		if e != nil {
			return Solid{}, e
		}
		caps[i], e = geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
	}
	n := len(p)
	ids := [2][]int{make([]int, n), make([]int, n)}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	add := func(a, b int, s geom.Surface, u, v geom.Vec2) int {
		i := len(edges)
		x := parametric.Edge{Start: a, End: b}
		if a != b {
			x.Curve, x.Range = surfaceTrimCurve{s, u, v, nil}, parametric.Interval{Max: 1}
		}
		edges = append(edges, x)
		return i
	}
	for j, u := range [2]float64{lo, hi} {
		for i, q := range p {
			if j == 1 && q.X == 0 {
				ids[j][i] = ids[0][i]
				continue
			}
			ids[j][i] = len(vertices)
			vertices = append(vertices, frame.ToWorld(geom.Vec3{X: q.X * math.Cos(u), Y: q.X * math.Sin(u), Z: q.Y}))
		}
	}
	meridian := [2][]int{make([]int, n), make([]int, n)}
	var faces []Face
	for j, cap := range caps {
		f := Face{Surface: cap, Reversed: j == 1}
		for i, a := range p {
			k := (i + 1) % n
			b := p[k]
			if j == 1 && a.X == 0 && b.X == 0 {
				meridian[j][i] = meridian[0][i]
			} else {
				meridian[j][i] = add(ids[j][i], ids[j][k], cap, a, b)
			}
			f.Outer = append(f.Outer, Use{Edge: meridian[j][i], UVStart: a, UVEnd: b})
		}
		faces = append(faces, f)
	}
	angular := make([]int, n)
	for i := range angular {
		angular[i] = -1
	}
	for i, a := range p {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		j := (i + 1) % n
		b := p[j]
		if a.X == 0 && b.X == 0 {
			continue
		}
		wall, e := parametric.RevolutionSide(frame, a, b)
		if e != nil {
			return Solid{}, e
		}
		for k, v := range [2]int{i, j} {
			if angular[v] < 0 {
				angular[v] = add(ids[0][v], ids[1][v], wall, geom.Vec2{X: lo, Y: float64(k)}, geom.Vec2{X: hi, Y: float64(k)})
			}
		}
		uv := [4]geom.Vec2{{X: lo}, {X: hi}, {X: hi, Y: 1}, {X: lo, Y: 1}}
		f := Face{Surface: wall}
		for k, id := range [4]int{angular[i], meridian[1][i], angular[j], meridian[0][i]} {
			f.Outer = append(f.Outer, Use{Edge: id, Reversed: k >= 2, UVStart: uv[k], UVEnd: uv[(k+1)%4]})
		}
		faces = append(faces, f)
	}
	return NewSolidContext(ctx, vertices, edges, faces, tol)
}

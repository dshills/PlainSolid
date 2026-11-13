package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalReliefSpec is a coaxial enlargement of a tube end. Depth is measured
// inward from that cap. A zero Radius and Depth leaves that end unchanged.
type HelicalReliefSpec struct{ Radius, Depth float64 }

func (s HelicalReliefSpec) valid() bool {
	return s == (HelicalReliefSpec{}) || finite(s.Radius) && finite(s.Depth) && s.Radius > 0 && s.Depth > 0
}

// HelicalEndReliefs preserves all helical groove faces and the original
// twisted bore support. Each present end appends a cylindrical wall and an
// annular shoulder; absent face pairs are {-1,-1}. The volume coefficient
// uses the ideal orthonormal-frame convention of HelicalEntryRelief.
type HelicalEndReliefs struct {
	Solid, Original            Solid
	LowerFaces, UpperFaces     [2]int
	RemovedVolumePiCoefficient *big.Rat
}

// GrooveTubeWithEndReliefs widens one or both axial ends of a blind-ended
// helical tube. The reliefs must be strictly disjoint from each other and
// every groove, with native sewing clearance. The original groove curves
// are retained. The result does not include overlapping lead-ins.
func GrooveTubeWithEndReliefs(ctx context.Context, r HelicalGrooveRecipe, lower, upper HelicalReliefSpec) (HelicalEndReliefs, error) {
	var empty HelicalEndReliefs
	if ctx == nil || !lower.valid() || !upper.valid() || lower.Depth == 0 && upper.Depth == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if !finite(r.Height) || r.Height <= 0 || new(big.Rat).Add(ratInset(lower.Depth), ratInset(upper.Depth)).Cmp(ratInset(r.Height)) >= 0 {
		return empty, geom.ErrDomain
	}
	var original, current Solid
	var e error
	out := HelicalEndReliefs{
		LowerFaces: [2]int{-1, -1}, UpperFaces: [2]int{-1, -1},
		RemovedVolumePiCoefficient: new(big.Rat),
	}
	if lower.Depth > 0 {
		base, e := GrooveTubeWithEntryRelief(ctx, r, lower.Radius, lower.Depth)
		if e != nil {
			return empty, e
		}
		original, current = base.Original, base.Solid
		out.LowerFaces = [2]int{base.ReliefWall, base.Shoulder}
		out.RemovedVolumePiCoefficient.Set(base.RemovedVolumePiCoefficient)
	} else {
		original, e = GrooveTubeWithHelicalStarts(ctx, r.Frame, r.OuterRadius, r.BoreRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts)
		if e != nil {
			return empty, e
		}
		current = original
	}
	out.Original = original
	if upper.Depth > 0 {
		upperResult, e := applyUpperHelicalRelief(ctx, r, current, lower.Depth, upper)
		if e != nil {
			return empty, e
		}
		current = upperResult.Solid
		out.UpperFaces = [2]int{upperResult.ReliefWall, upperResult.Shoulder}
		out.RemovedVolumePiCoefficient.Add(out.RemovedVolumePiCoefficient, upperResult.RemovedVolumePiCoefficient)
	}
	out.Solid = current
	return out, nil
}

func applyUpperHelicalRelief(ctx context.Context, r HelicalGrooveRecipe, stock Solid, lowerDepth float64, spec HelicalReliefSpec) (HelicalEntryRelief, error) {
	var empty HelicalEntryRelief
	if spec.Radius <= r.BoreRadius || spec.Radius >= r.OuterRadius {
		return empty, geom.ErrDomain
	}
	pad := new(big.Rat).Mul(ratInset(stock.tolerance), big.NewRat(8, 1))
	clear := func(a, b float64) bool { return new(big.Rat).Sub(ratInset(a), ratInset(b)).Cmp(pad) > 0 }
	if !clear(spec.Radius, r.BoreRadius) || !clear(r.OuterRadius, spec.Radius) || !clear(spec.Depth, 0) {
		return empty, geom.ErrConvergence
	}
	cutZ, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ratInset(r.Height), ratInset(spec.Depth)))
	if e != nil {
		return empty, e
	}
	if !clear(cutZ, lowerDepth) {
		return empty, geom.ErrDomain
	}
	advance := new(big.Rat).Mul(ratInset(r.Lead), ratInset(r.End))
	lastStart := new(big.Rat).Mul(ratInset(r.Lead), big.NewRat(int64(r.Starts-1), int64(r.Starts)))
	advance.Add(advance, lastStart)
	for _, p := range r.Profile {
		last := new(big.Rat).Add(advance, ratInset(p.Y))
		if new(big.Rat).Sub(ratInset(cutZ), last).Cmp(pad) <= 0 {
			return empty, geom.ErrConvergence
		}
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	wall, ok := faces[1].Surface.(geom.TwistedCylinderSurface)
	if !ok || len(edges) < 8 || len(vertices) < 4 {
		return empty, geom.ErrInvalid
	}
	frame, _, twist, phase := wall.Parameters()
	angle, e := geom.RepresentedExactScalar(new(big.Rat).Add(new(big.Rat).Mul(ratInset(twist), ratInset(cutZ)), ratInset(phase)))
	if e != nil {
		return empty, e
	}
	vertices[3], e = wall.Eval(0, cutZ)
	if e != nil {
		return empty, e
	}
	edges[4].Curve = surfaceTrimCurve{wall, geom.Vec2{X: 1, Y: lowerDepth}, geom.Vec2{X: 1, Y: cutZ}, nil}
	edges[5].Curve = surfaceTrimCurve{wall, geom.Vec2{Y: cutZ}, geom.Vec2{X: 1, Y: cutZ}, nil}
	for i := range faces[1].Outer {
		u := &faces[1].Outer[i]
		if u.UVStart.Y == r.Height {
			u.UVStart.Y = cutZ
		}
		if u.UVEnd.Y == r.Height {
			u.UVEnd.Y = cutZ
		}
	}
	cylinder, e := geom.NewTwistedCylinderSurface(frame, spec.Radius, 0, 0)
	if e != nil {
		return empty, e
	}
	cylinder = cylinder.ReversedAngular()
	v0, v1 := len(vertices), len(vertices)+1
	for _, z := range []float64{cutZ, r.Height} {
		v, e := cylinder.Eval(0, z)
		if e != nil {
			return empty, e
		}
		vertices = append(vertices, v)
	}
	add := func(a, b int, s geom.Surface, x, y geom.Vec2) int {
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: a, End: b, Curve: surfaceTrimCurve{s, x, y, nil}, Range: parametric.Interval{Max: 1}})
		return id
	}
	p := [4]geom.Vec2{{Y: cutZ}, {X: 1, Y: cutZ}, {X: 1, Y: r.Height}, {Y: r.Height}}
	low := add(v0, v0, cylinder, p[0], p[1])
	seam := add(v0, v1, cylinder, p[1], p[2])
	high := add(v1, v1, cylinder, p[3], p[2])
	relief := Face{Surface: cylinder}
	for i, id := range [4]int{low, seam, high, seam} {
		relief.Outer = append(relief.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: p[i], UVEnd: p[(i+1)%4]})
	}
	out := HelicalEntryRelief{ReliefWall: len(faces), Shoulder: len(faces) + 1}
	faces = append(faces, relief)
	top, e := geom.NewTwistedAnnularSurface(frame, spec.Radius, r.OuterRadius, r.Height, 0)
	if e != nil {
		return empty, e
	}
	edges[7].Start = v1
	edges[7].Curve = surfaceTrimCurve{top, geom.Vec2{X: 1, Y: spec.Radius}, geom.Vec2{X: 1, Y: r.OuterRadius}, nil}
	topFace := Face{Surface: top, Reversed: true}
	q := [4]geom.Vec2{{Y: spec.Radius}, {X: 1, Y: spec.Radius}, {X: 1, Y: r.OuterRadius}, {Y: r.OuterRadius}}
	for i, id := range [4]int{high, 7, 2, 7} {
		topFace.Outer = append(topFace.Outer, Use{Edge: id, Reversed: i == 0 || i >= 2, UVStart: q[i], UVEnd: q[(i+1)%4]})
	}
	faces[3] = topFace
	shoulder, e := geom.NewTwistedAnnularSurface(frame, r.BoreRadius, spec.Radius, cutZ, math.Remainder(angle, 1))
	if e != nil {
		return empty, e
	}
	q = [4]geom.Vec2{{Y: r.BoreRadius}, {X: 1, Y: r.BoreRadius}, {X: 1, Y: spec.Radius}, {Y: spec.Radius}}
	radial := add(3, v0, shoulder, q[1], q[2])
	f := Face{Surface: shoulder, Reversed: true}
	for i, id := range [4]int{5, radial, low, radial} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i == 0 || i == 3, UVStart: q[i], UVEnd: q[(i+1)%4]})
	}
	faces = append(faces, f)
	out.Solid, e = NewSolidContext(ctx, vertices, edges, faces, stock.tolerance)
	if e != nil {
		return empty, e
	}
	radius, bore := ratInset(spec.Radius), ratInset(r.BoreRadius)
	out.RemovedVolumePiCoefficient = new(big.Rat).Mul(
		new(big.Rat).Sub(new(big.Rat).Mul(radius, radius), new(big.Rat).Mul(bore, bore)), ratInset(spec.Depth))
	return out, nil
}

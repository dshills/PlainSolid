package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalEntryRelief retains the original blind internal groove. Its lower
// bore rim and seam are shortened, the lower cap is recharted, and two new
// faces provide a widened entry cylinder and annular shoulder. Groove tool
// faces and the original twisted inner support/holes remain unchanged.
type HelicalEntryRelief struct {
	Solid                      Solid
	Original                   Solid
	ReliefWall, Shoulder       int
	Radius, Depth              float64
	RemovedVolumePiCoefficient *big.Rat
}

// GrooveTubeWithEntryRelief opens the lower axial cap into a stepped bore.
// The widened radius stays strictly inside stock and depth is strictly below
// the first point of every helical cut, with eight native sewing tolerances of
// clearance. This is a disjoint entry relief, not an overlapping lead-in Boolean.
// Inputs retain the original groove's exact representability/finite limits.
func GrooveTubeWithEntryRelief(ctx context.Context, r HelicalGrooveRecipe, reliefRadius, depth float64) (HelicalEntryRelief, error) {
	fail := func(e error) (HelicalEntryRelief, error) { return HelicalEntryRelief{}, e }
	if ctx == nil || !finite(reliefRadius) || !finite(depth) || r.BoreRadius <= 0 || reliefRadius <= r.BoreRadius || reliefRadius >= r.OuterRadius || depth <= 0 {
		return fail(geom.ErrInvalid)
	}
	original, e := GrooveTubeWithHelicalStarts(ctx, r.Frame, r.OuterRadius, r.BoreRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts)
	if e != nil {
		return fail(e)
	}
	pad := new(big.Rat).Mul(ratInset(original.tolerance), big.NewRat(8, 1))
	if new(big.Rat).Sub(ratInset(reliefRadius), ratInset(r.BoreRadius)).Cmp(pad) <= 0 || new(big.Rat).Sub(ratInset(r.OuterRadius), ratInset(reliefRadius)).Cmp(pad) <= 0 || ratInset(depth).Cmp(pad) <= 0 {
		return fail(geom.ErrConvergence)
	}
	for _, p := range r.Profile {
		first := new(big.Rat).Add(ratInset(p.Y), new(big.Rat).Mul(ratInset(r.Lead), ratInset(r.Start)))
		if new(big.Rat).Sub(first, ratInset(depth)).Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
	}
	v, edges, faces := original.Vertices(), original.Edges(), original.Faces()
	wall, ok := faces[1].Surface.(geom.TwistedCylinderSurface)
	if !ok || len(edges) < 8 || len(v) < 4 {
		return fail(geom.ErrInvalid)
	}
	frame, _, twist, phase := wall.Parameters()
	atDepth, e := geom.RepresentedExactScalar(new(big.Rat).Add(new(big.Rat).Mul(ratInset(twist), ratInset(depth)), ratInset(phase)))
	if e != nil {
		return fail(e)
	}
	v[2], e = wall.Eval(0, depth)
	if e != nil {
		return fail(e)
	}
	edges[3].Curve = surfaceTrimCurve{wall, geom.Vec2{Y: depth}, geom.Vec2{X: 1, Y: depth}, nil}
	edges[4].Curve = surfaceTrimCurve{wall, geom.Vec2{X: 1, Y: depth}, geom.Vec2{X: 1, Y: r.Height}, nil}
	for i := range faces[1].Outer {
		u := &faces[1].Outer[i]
		if u.UVStart.Y == 0 {
			u.UVStart.Y = depth
		}
		if u.UVEnd.Y == 0 {
			u.UVEnd.Y = depth
		}
	}
	relief, e := geom.NewTwistedCylinderSurface(frame, reliefRadius, 0, 0)
	if e != nil {
		return fail(e)
	}
	relief = relief.ReversedAngular()
	lowID, highID := len(v), len(v)+1
	for _, z := range []float64{0, depth} {
		p, e := relief.Eval(0, z)
		if e != nil {
			return fail(e)
		}
		v = append(v, p)
	}
	add := func(a, b int, s geom.Surface, x, y geom.Vec2) int {
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: a, End: b, Curve: surfaceTrimCurve{s, x, y, nil}, Range: parametric.Interval{Max: 1}})
		return id
	}
	low := add(lowID, lowID, relief, geom.Vec2{}, geom.Vec2{X: 1})
	seam := add(lowID, highID, relief, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: depth})
	high := add(highID, highID, relief, geom.Vec2{Y: depth}, geom.Vec2{X: 1, Y: depth})
	result := HelicalEntryRelief{Original: original, Radius: reliefRadius, Depth: depth, ReliefWall: len(faces), Shoulder: len(faces) + 1}
	p := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: depth}, {Y: depth}}
	f := Face{Surface: relief}
	for i, id := range [4]int{low, seam, high, seam} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: p[i], UVEnd: p[(i+1)%4]})
	}
	faces = append(faces, f)
	bottom, e := geom.NewTwistedAnnularSurface(frame, reliefRadius, r.OuterRadius, 0, 0)
	if e != nil {
		return fail(e)
	}
	edges[6].Start = lowID
	edges[6].Curve = surfaceTrimCurve{bottom, geom.Vec2{X: 1, Y: reliefRadius}, geom.Vec2{X: 1, Y: r.OuterRadius}, nil}
	cap := Face{Surface: bottom}
	bp := [4]geom.Vec2{{Y: reliefRadius}, {X: 1, Y: reliefRadius}, {X: 1, Y: r.OuterRadius}, {Y: r.OuterRadius}}
	for i, id := range [4]int{low, 6, 0, 6} {
		cap.Outer = append(cap.Outer, Use{Edge: id, Reversed: i == 0 || i >= 2, UVStart: bp[i], UVEnd: bp[(i+1)%4]})
	}
	faces[2] = cap
	shoulder, e := geom.NewTwistedAnnularSurface(frame, r.BoreRadius, reliefRadius, depth, math.Remainder(atDepth, 1))
	if e != nil {
		return fail(e)
	}
	radial := add(2, highID, shoulder, geom.Vec2{X: 1, Y: r.BoreRadius}, geom.Vec2{X: 1, Y: reliefRadius})
	sp := [4]geom.Vec2{{Y: r.BoreRadius}, {X: 1, Y: r.BoreRadius}, {X: 1, Y: reliefRadius}, {Y: reliefRadius}}
	f = Face{Surface: shoulder}
	for i, id := range [4]int{3, radial, high, radial} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i == 0 || i == 3, UVStart: sp[i], UVEnd: sp[(i+1)%4]})
	}
	faces = append(faces, f)
	result.Solid, e = NewSolidContext(ctx, v, edges, faces, original.tolerance)
	if e != nil {
		return fail(e)
	}
	// This coefficient describes the ideal orthonormal-frame coaxial removal;
	// the original frame and native numerical chart conventions are retained.
	result.RemovedVolumePiCoefficient = new(big.Rat).Mul(new(big.Rat).Sub(new(big.Rat).Mul(ratInset(reliefRadius), ratInset(reliefRadius)), new(big.Rat).Mul(ratInset(r.BoreRadius), ratInset(r.BoreRadius))), ratInset(depth))
	return result, nil
}

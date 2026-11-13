package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalTipRelief keeps the original external groove support and hole wires.
// Only its lower boundary/seam and lower disk change. The new tip cylinder and
// downward-facing shoulder join the unmodified grooved shaft above Depth.
type HelicalTipRelief struct {
	Solid, Original            Solid
	TipWall, Shoulder          int
	Radius, Depth              float64
	RemovedVolumePiCoefficient *big.Rat
}

// GrooveShaftWithTipRelief reduces the lower end radius without entering any
// blind helical cut. The exact axial gap to every start is at least eight native
// sewing tolerances. BoreRadius must be zero. This is not a relief overlapping
// the thread or a general helical Boolean. RemovedVolumePiCoefficient targets
// the ideal represented-affine shaft and includes its exact frame determinant.
func GrooveShaftWithTipRelief(ctx context.Context, r HelicalGrooveRecipe, tipRadius, depth float64) (HelicalTipRelief, error) {
	fail := func(e error) (HelicalTipRelief, error) { return HelicalTipRelief{}, e }
	if ctx == nil || r.BoreRadius != 0 || !finite(tipRadius) || !finite(depth) || tipRadius <= 0 || tipRadius >= r.OuterRadius || depth <= 0 {
		return fail(geom.ErrInvalid)
	}
	original, e := GrooveCylinderWithHelicalStarts(ctx, r.Frame, r.OuterRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts)
	if e != nil {
		return fail(e)
	}
	pad := new(big.Rat).Mul(ratInset(original.tolerance), big.NewRat(8, 1))
	if ratInset(tipRadius).Cmp(pad) <= 0 || new(big.Rat).Sub(ratInset(r.OuterRadius), ratInset(tipRadius)).Cmp(pad) <= 0 || ratInset(depth).Cmp(pad) <= 0 {
		return fail(geom.ErrConvergence)
	}
	for _, p := range r.Profile {
		first := new(big.Rat).Add(ratInset(p.Y), new(big.Rat).Mul(ratInset(r.Lead), ratInset(r.Start)))
		if new(big.Rat).Sub(first, ratInset(depth)).Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
	}
	v, edges, faces := original.Vertices(), original.Edges(), original.Faces()
	wall, ok := faces[0].Surface.(geom.TwistedCylinderSurface)
	if !ok || len(v) < 4 || len(edges) < 7 {
		return fail(geom.ErrInvalid)
	}
	frame, _, twist, phase := wall.Parameters()
	phaseDepth, e := geom.RepresentedExactScalar(new(big.Rat).Add(new(big.Rat).Mul(ratInset(twist), ratInset(depth)), ratInset(phase)))
	if e != nil {
		return fail(e)
	}
	v[0], e = wall.Eval(0, depth)
	if e != nil {
		return fail(e)
	}
	edges[0].Curve = surfaceTrimCurve{wall, geom.Vec2{Y: depth}, geom.Vec2{X: 1, Y: depth}, nil}
	edges[1].Curve = surfaceTrimCurve{wall, geom.Vec2{X: 1, Y: depth}, geom.Vec2{X: 1, Y: r.Height}, nil}
	for i := range faces[0].Outer {
		u := &faces[0].Outer[i]
		if u.UVStart.Y == 0 {
			u.UVStart.Y = depth
		}
		if u.UVEnd.Y == 0 {
			u.UVEnd.Y = depth
		}
	}
	tip, e := geom.NewTwistedCylinderSurface(frame, tipRadius, twist, phase)
	if e != nil {
		return fail(e)
	}
	lowID, highID := len(v), len(v)+1
	for _, z := range []float64{0, depth} {
		p, e := tip.Eval(0, z)
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
	low := add(lowID, lowID, tip, geom.Vec2{}, geom.Vec2{X: 1})
	seam := add(lowID, highID, tip, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: depth})
	high := add(highID, highID, tip, geom.Vec2{Y: depth}, geom.Vec2{X: 1, Y: depth})
	out := HelicalTipRelief{Original: original, Radius: tipRadius, Depth: depth, TipWall: len(faces), Shoulder: len(faces) + 1}
	p := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: depth}, {Y: depth}}
	f := Face{Surface: tip}
	for i, id := range [4]int{low, seam, high, seam} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: p[i], UVEnd: p[(i+1)%4]})
	}
	faces = append(faces, f)
	oriented := func(z, turn float64) (geom.Frame, error) {
		theta := 2 * math.Pi * math.Remainder(turn, 1)
		x := frame.XAxis().Scale(math.Cos(theta)).Add(frame.YAxis().Scale(math.Sin(theta)))
		return geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), x, frame.ZAxis())
	}
	bottomFrame, e := oriented(0, phase)
	if e != nil {
		return fail(e)
	}
	cylinder, e := parametric.Cylinder(bottomFrame, tipRadius, 1)
	if e != nil {
		return fail(e)
	}
	disk := cylinder.Faces()[1].Surface
	bp := [4]geom.Vec2{{}, {X: 2 * math.Pi}, {X: 2 * math.Pi, Y: tipRadius}, {Y: tipRadius}}
	edges[4].End = lowID
	edges[4].Curve = surfaceTrimCurve{disk, bp[1], bp[2], nil}
	f = Face{Surface: disk}
	for i, id := range [4]int{3, 4, low, 4} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: bp[i], UVEnd: bp[(i+1)%4]})
	}
	faces[1] = f
	shoulderFrame, e := oriented(0, phaseDepth)
	if e != nil {
		return fail(e)
	}
	shoulder, e := geom.NewTwistedAnnularSurface(shoulderFrame, tipRadius, r.OuterRadius, depth, 0)
	if e != nil {
		return fail(e)
	}
	radial := add(highID, 0, shoulder, geom.Vec2{X: 1, Y: tipRadius}, geom.Vec2{X: 1, Y: r.OuterRadius})
	sp := [4]geom.Vec2{{Y: tipRadius}, {X: 1, Y: tipRadius}, {X: 1, Y: r.OuterRadius}, {Y: r.OuterRadius}}
	f = Face{Surface: shoulder}
	for i, id := range [4]int{high, radial, 0, radial} {
		f.Outer = append(f.Outer, Use{Edge: id, Reversed: i >= 2, UVStart: sp[i], UVEnd: sp[(i+1)%4]})
	}
	faces = append(faces, f)
	out.Solid, e = NewSolidContext(ctx, v, edges, faces, original.tolerance)
	if e != nil {
		return fail(e)
	}
	x, y, z := exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis()), exactBoundPoint(frame.ZAxis())
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, new(big.Rat).Mul(x[i], new(big.Rat).Sub(new(big.Rat).Mul(y[j], z[k]), new(big.Rat).Mul(y[k], z[j]))))
	}
	det.Abs(det)
	out.RemovedVolumePiCoefficient = new(big.Rat).Mul(det, new(big.Rat).Mul(new(big.Rat).Sub(new(big.Rat).Mul(ratInset(r.OuterRadius), ratInset(r.OuterRadius)), new(big.Rat).Mul(ratInset(tipRadius), ratInset(tipRadius))), ratInset(depth)))
	return out, nil
}

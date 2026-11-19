//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type RoundedFaceRecognition struct {
	Face            int
	Outer           geom.PlanarRoundedRectangle
	Holes           []geom.PlanarCircularProfile
	ExactOperations int
}

// RecognizeRoundedPlanarFace proves a complete canonical mixed line/quarter-arc
// wire with one common corner radius (or a complete sharp rectangle), and
// circular inner wires. All actual model edges and represented anchors are
// checked. Separate corner radii are unsupported, never replaced by an average.
func RecognizeRoundedPlanarFace(ctx context.Context, target Solid, index int, o geom.SurfacePatchEvaluationOptions) (out RoundedFaceRecognition, err error) {
	out.Face = index
	if ctx == nil || index < 0 || index >= len(target.faces) {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, geom.ErrInvalid
	}
	face := target.faces[index]
	plane, ok := face.Surface.(geom.PlaneSurface)
	if !ok {
		return out, geom.ErrConvergence
	}
	frame := plane.Frame()
	if _, e := geom.NewPlanarRoundedRectangle(frame, 1, 1, 0); e != nil {
		return out, e
	}
	if len(face.Outer) != 4 && len(face.Outer) != 8 {
		return out, geom.ErrConvergence
	}
	if len(face.Holes) > 16 {
		return out, geom.ErrConvergence
	}
	check := func(q *big.Rat) error {
		out.ExactOperations++
		if out.ExactOperations > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	world := func(f geom.Frame, uv geom.Vec3) (geom.Vec3, error) {
		origin, x, y, z := f.Origin(), f.XAxis(), f.YAxis(), f.ZAxis()
		rows := [3][4]float64{{origin.X, x.X, y.X, z.X}, {origin.Y, x.Y, y.Y, z.Y}, {origin.Z, x.Z, y.Z, z.Z}}
		local := [3]float64{uv.X, uv.Y, uv.Z}
		var result [3]float64
		for i, row := range rows {
			q := new(big.Rat).SetFloat64(row[0])
			for j, v := range local {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(row[j+1]), new(big.Rat).SetFloat64(v)))
			}
			if e := check(q); e != nil {
				return geom.Vec3{}, e
			}
			f, e := geom.RepresentedExactScalar(q)
			if e != nil {
				return geom.Vec3{}, e
			}
			result[i] = f
		}
		return geom.Vec3{X: result[0], Y: result[1], Z: result[2]}, nil
	}
	verify := func(use Use, start, end geom.Vec2, arc *anchoredCircularUV) error {
		if use.Reversed || use.UVStart != start || use.UVEnd != end || use.Edge < 0 || use.Edge >= len(target.edges) {
			return geom.ErrConvergence
		}
		if arc == nil {
			if use.Curve != nil {
				return geom.ErrConvergence
			}
		} else {
			actual, ok := use.Curve.(anchoredCircularUV)
			if !ok || actual != *arc {
				return geom.ErrConvergence
			}
		}
		edge := target.edges[use.Edge]
		if edge.Start < 0 || edge.End < 0 || edge.Start >= len(target.vertices) || edge.End >= len(target.vertices) || edge.Range.Min != 0 || edge.Range.Max != 1 {
			return geom.ErrConvergence
		}
		a, e := world(frame, geom.Vec3{X: start.X, Y: start.Y})
		if e != nil {
			return e
		}
		b, e := world(frame, geom.Vec3{X: end.X, Y: end.Y})
		if e != nil {
			return e
		}
		if a != target.vertices[edge.Start] || b != target.vertices[edge.End] {
			return geom.ErrConvergence
		}
		model, ok := edge.Curve.(surfaceTrimCurve)
		if !ok || model.curve != nil {
			return geom.ErrConvergence
		}
		if arc == nil {
			surface, ok := model.surface.(geom.PlaneSurface)
			if !ok {
				return geom.ErrConvergence
			}
			ma, e := world(surface.Frame(), geom.Vec3{X: model.a.X, Y: model.a.Y})
			if e != nil {
				return e
			}
			mb, e := world(surface.Frame(), geom.Vec3{X: model.b.X, Y: model.b.Y})
			if e != nil {
				return e
			}
			if ma != a || mb != b {
				return geom.ErrConvergence
			}
			return nil
		}
		cylinder, ok := model.surface.(geom.CylinderSurface)
		if !ok || model.a.X != arc.From || model.b.X != arc.To || model.a.Y != model.b.Y {
			return geom.ErrConvergence
		}
		cf, r := cylinder.Parameters()
		sign := 1.
		if arc.Sin.Y < 0 {
			sign = -1
		}
		if r != arc.Cos.X || cf.XAxis() != frame.XAxis() || cf.YAxis() != frame.YAxis().Scale(sign) || cf.ZAxis() != frame.ZAxis().Scale(sign) {
			return geom.ErrConvergence
		}
		center, e := world(frame, geom.Vec3{X: arc.Center.X, Y: arc.Center.Y})
		if e != nil {
			return e
		}
		mc, e := world(cf, geom.Vec3{Z: model.a.Y})
		if e != nil {
			return e
		}
		if center != mc {
			return geom.ErrConvergence
		}
		return nil
	}
	chain := func(wire []Use) error {
		ids := map[int]bool{}
		for i, use := range wire {
			if use.Edge < 0 || use.Edge >= len(target.edges) || ids[use.Edge] {
				return geom.ErrConvergence
			}
			ids[use.Edge] = true
			next := wire[(i+1)%len(wire)]
			if next.Edge < 0 || next.Edge >= len(target.edges) || target.edges[use.Edge].End != target.edges[next.Edge].Start {
				return geom.ErrConvergence
			}
		}
		return nil
	}
	min, max := face.Outer[0].UVStart, face.Outer[0].UVStart
	radius := -1.
	arcCount := 0
	for _, use := range face.Outer {
		if !uvFinite(use.UVStart) {
			return out, geom.ErrInvalid
		}
		p := use.UVStart
		if p.X < min.X {
			min.X = p.X
		}
		if p.X > max.X {
			max.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
		if use.Curve != nil {
			arc, ok := use.Curve.(anchoredCircularUV)
			if !ok {
				return out, geom.ErrConvergence
			}
			if radius < 0 {
				radius = arc.Cos.X
			} else if radius != arc.Cos.X {
				return out, geom.ErrConvergence
			}
			arcCount++
		}
	}
	if arcCount == 0 {
		radius = 0
		if len(face.Outer) != 4 {
			return out, geom.ErrConvergence
		}
	} else if arcCount != 4 || len(face.Outer) != 8 {
		return out, geom.ErrConvergence
	}
	points, arcs, e := roundedExtrusionProfile(RoundedRectangle{Min: min, Max: max, Radii: [4]float64{radius, radius, radius, radius}})
	if e != nil {
		return out, e
	}
	if len(points) != len(face.Outer) {
		return out, geom.ErrConvergence
	}
	if e := chain(face.Outer); e != nil {
		return out, e
	}
	for i, use := range face.Outer {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		var arc *anchoredCircularUV
		if arcs[i].circle.Radius > 0 {
			x, e := extrusionArc(arcs[i].circle, arcs[i].quadrant, points[i], points[(i+1)%len(points)], false)
			if e != nil {
				return out, e
			}
			arc = &x
		}
		if e := verify(use, points[i], points[(i+1)%len(points)], arc); e != nil {
			return out, e
		}
	}
	centerUV := geom.Vec2{}
	half := geom.Vec2{}
	for i, ends := range [][2]float64{{min.X, max.X}, {min.Y, max.Y}} {
		c := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(ends[0]), new(big.Rat).SetFloat64(ends[1])), big.NewRat(2, 1))
		h := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(ends[1]), new(big.Rat).SetFloat64(ends[0])), big.NewRat(2, 1))
		if e := check(c); e != nil {
			return out, e
		}
		if e := check(h); e != nil {
			return out, e
		}
		cv, e := geom.RepresentedExactScalar(c)
		if e != nil {
			return out, e
		}
		hv, e := geom.RepresentedExactScalar(h)
		if e != nil {
			return out, e
		}
		if i == 0 {
			centerUV.X = cv
			half.X = hv
		} else {
			centerUV.Y = cv
			half.Y = hv
		}
	}
	center, e := world(frame, geom.Vec3{X: centerUV.X, Y: centerUV.Y})
	if e != nil {
		return out, e
	}
	centerFrame, e := geom.NewFrame(center, frame.XAxis(), frame.ZAxis())
	if e != nil {
		return out, e
	}
	out.Outer, e = geom.NewPlanarRoundedRectangle(centerFrame, half.X, half.Y, radius)
	if e != nil {
		return out, e
	}
	for _, wire := range face.Holes {
		if len(wire) != 4 {
			return out, geom.ErrConvergence
		}
		if e := chain(wire); e != nil {
			return out, e
		}
		first, ok := wire[0].Curve.(anchoredCircularUV)
		if !ok {
			return out, geom.ErrConvergence
		}
		h := ExtrusionCircle{Center: first.Center, Radius: first.Cos.X}
		for i, use := range wire {
			c := [4]int64{1, 0, -1, 0}
			s := [4]int64{0, -1, 0, 1}
			var p [2]geom.Vec2
			for end := 0; end < 2; end++ {
				q := (i + end) % 4
				x := new(big.Rat).Add(new(big.Rat).SetFloat64(h.Center.X), new(big.Rat).Mul(new(big.Rat).SetFloat64(h.Radius), big.NewRat(c[q], 1)))
				y := new(big.Rat).Add(new(big.Rat).SetFloat64(h.Center.Y), new(big.Rat).Mul(new(big.Rat).SetFloat64(h.Radius), big.NewRat(s[q], 1)))

				if e := check(x); e != nil {
					return out, e
				}
				if e := check(y); e != nil {
					return out, e
				}
				xf, e := geom.RepresentedExactScalar(x)
				if e != nil {
					return out, e
				}
				yf, e := geom.RepresentedExactScalar(y)
				if e != nil {
					return out, e
				}
				p[end] = geom.Vec2{X: xf, Y: yf}
			}
			arc, e := extrusionArc(h, i, p[0], p[1], true)
			if e != nil {
				return out, e
			}
			if e := verify(use, p[0], p[1], &arc); e != nil {
				return out, e
			}
		}
		center, e := world(frame, geom.Vec3{X: h.Center.X, Y: h.Center.Y})
		if e != nil {
			return out, e
		}
		disk, e := geom.NewPlanarCircularProfile(center, frame.ZAxis(), h.Radius)
		if e != nil {
			return out, e
		}
		remaining := o.MaxOperations - out.ExactOperations
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		r, e := geom.CompareRoundedRectangleDisk(ctx, out.Outer, disk, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		out.ExactOperations += r.ExactOperations
		if e != nil {
			return out, e
		}
		if r.Relation != geom.RoundedDiskStrictlyCovered {
			return out, geom.ErrConvergence
		}
		for _, other := range out.Holes {
			remaining = o.MaxOperations - out.ExactOperations
			if remaining < 1 {
				return out, geom.ErrConvergence
			}
			r, e := geom.ComparePlanarCircularProfiles(ctx, disk, other, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			out.ExactOperations += r.ExactOperations
			if e != nil {
				return out, e
			}
			if r.Relation != geom.CircularProfilesDisjoint {
				return out, geom.ErrConvergence
			}
		}
		out.Holes = append(out.Holes, disk)
	}
	return out, nil
}

type CylinderToRoundedFaceProof struct {
	Target          RoundedFaceRecognition
	Coverage        geom.RoundedRectangleDiskComparison
	HoleRelations   []geom.CircularProfileComparison
	Height          float64
	Endpoint        geom.Vec3
	ExactOperations int
}

// CylinderToRoundedFace re-recognizes the actual target cap before exact
// coverage testing. Axis-normal positive represented height and the actual
// constructor endpoint are checked. This is selected-face termination, not a
// first-hit or Boolean proof; analytic topology retains numerical tolerance.
func CylinderToRoundedFace(ctx context.Context, center, normal geom.Vec3, radius float64, target Solid, index int, o geom.SurfacePatchEvaluationOptions) (Solid, CylinderToRoundedFaceProof, error) {
	var proof CylinderToRoundedFaceProof
	recognition, e := RecognizeRoundedPlanarFace(ctx, target, index, o)
	proof.Target = recognition
	proof.ExactOperations = recognition.ExactOperations
	if e != nil {
		return Solid{}, proof, e
	}
	if !center.IsFinite() || !normal.IsFinite() {
		return Solid{}, proof, geom.ErrInvalid
	}
	axis := -1
	sign := 1.
	for i, q := range []float64{normal.X, normal.Y, normal.Z} {
		if q != 0 {
			if axis >= 0 {
				return Solid{}, proof, geom.ErrDomain
			}
			axis = i
			if q < 0 {
				sign = -1
			}
		}
	}
	if axis < 0 {
		return Solid{}, proof, geom.ErrInvalid
	}
	frame, _, _, _ := recognition.Outer.Parameters()
	z := frame.ZAxis()
	if [3]float64{z.X, z.Y, z.Z}[axis] == 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	cap := frame.Origin()
	value := func(p geom.Vec3) float64 { return [3]float64{p.X, p.Y, p.Z}[axis] }
	h := new(big.Rat).Sub(new(big.Rat).SetFloat64(value(cap)), new(big.Rat).SetFloat64(value(center)))
	if sign < 0 {
		h.Neg(h)
	}
	if h.Sign() <= 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	height, e := geom.RepresentedExactScalar(h)
	if e != nil {
		return Solid{}, proof, e
	}
	endpoint := center
	unit := geom.Vec3{}
	switch axis {
	case 0:
		endpoint.X = cap.X
		unit.X = sign
	case 1:
		endpoint.Y = cap.Y
		unit.Y = sign
	case 2:
		endpoint.Z = cap.Z
		unit.Z = sign
	}
	disk, e := geom.NewPlanarCircularProfile(endpoint, unit, radius)
	if e != nil {
		return Solid{}, proof, e
	}
	max := o.MaxOperations
	if max == 0 {
		max = 1000000
	}
	remaining := max - proof.ExactOperations
	if remaining < 1 {
		return Solid{}, proof, geom.ErrConvergence
	}
	proof.Coverage, e = geom.CompareRoundedRectangleDisk(ctx, recognition.Outer, disk, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	proof.ExactOperations += proof.Coverage.ExactOperations
	if e != nil {
		return Solid{}, proof, e
	}
	if proof.Coverage.Relation != geom.RoundedDiskStrictlyCovered && proof.Coverage.Relation != geom.RoundedDiskCoveredTouching {
		return Solid{}, proof, brep.ErrTerminationUncovered
	}
	for _, hole := range recognition.Holes {
		remaining = max - proof.ExactOperations
		if remaining < 1 {
			return Solid{}, proof, geom.ErrConvergence
		}
		r, e := geom.ComparePlanarCircularProfiles(ctx, disk, hole, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		proof.ExactOperations += r.ExactOperations
		if e != nil {
			return Solid{}, proof, e
		}
		proof.HoleRelations = append(proof.HoleRelations, r)
		if r.Relation != geom.CircularProfilesDisjoint && r.Relation != geom.CircularProfilesExternalTangent {
			return Solid{}, proof, brep.ErrTerminationUncovered
		}
	}
	f, e := geom.FrameFromNormal(center, unit)
	if e != nil {
		return Solid{}, proof, e
	}
	if f.ToWorld(geom.Vec3{Z: height}) != endpoint {
		return Solid{}, proof, geom.ErrUnrepresentable
	}
	native, e := parametric.Cylinder(f, radius, height)
	if e != nil {
		return Solid{}, proof, e
	}
	solid, e := FromParametric(native)
	if e != nil {
		return Solid{}, proof, e
	}
	proof.Height = height
	proof.Endpoint = endpoint
	return solid, proof, nil
}

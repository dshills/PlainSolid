package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CircularFaceRecognition is reconstructed from one actual plane face's
// anchored quarter-arc wires, shared vertices and cylindrical model edges.
// It is diagnostic data, never trusted by CylinderToRoundFace as input proof.
type CircularFaceRecognition struct {
	Face            int
	Outer           geom.PlanarCircularProfile
	Holes           []geom.PlanarCircularProfile
	ExactOperations int
}

// RecognizeCircularPlanarFace supports coordinate-axis plane frames and four
// canonical anchored quarter arcs per wire. All model-edge parameterizations,
// frame axes, radius and exact cardinal vertices are independently checked.
// Eccentric holes must be strictly nested/disjoint. Numerical topology tolerance
// is not used to establish the exact circle relations.
func RecognizeCircularPlanarFace(ctx context.Context, target Solid, faceIndex int, o geom.SurfacePatchEvaluationOptions) (out CircularFaceRecognition, err error) {
	out.Face = faceIndex
	if ctx == nil || faceIndex < 0 || faceIndex >= len(target.faces) {
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
	face := target.faces[faceIndex]
	plane, ok := face.Surface.(geom.PlaneSurface)
	if !ok {
		return out, geom.ErrConvergence
	}
	frame := plane.Frame()
	axisVector := func(v geom.Vec3) bool {
		nonzero := 0
		for _, q := range []float64{v.X, v.Y, v.Z} {
			if q != 0 {
				if q != 1 && q != -1 {
					return false
				}
				nonzero++
			}
		}
		return nonzero == 1
	}
	if !axisVector(frame.XAxis()) || !axisVector(frame.YAxis()) || !axisVector(frame.ZAxis()) {
		return out, geom.ErrConvergence
	}
	exactWorld := func(f geom.Frame, p geom.Vec3) (geom.Vec3, error) {
		origin := f.Origin()
		x, y, z := f.XAxis(), f.YAxis(), f.ZAxis()
		rows := [3][4]float64{{origin.X, x.X, y.X, z.X}, {origin.Y, x.Y, y.Y, z.Y}, {origin.Z, x.Z, y.Z, z.Z}}
		local := [3]float64{p.X, p.Y, p.Z}
		var values [3]float64
		for i, row := range rows {
			q := new(big.Rat).SetFloat64(row[0])
			for j := 0; j < 3; j++ {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(row[j+1]), new(big.Rat).SetFloat64(local[j])))
			}
			out.ExactOperations++
			if out.ExactOperations > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
				return geom.Vec3{}, geom.ErrConvergence
			}
			v, e := geom.RepresentedExactScalar(q)
			if e != nil {
				return geom.Vec3{}, e
			}
			values[i] = v
		}
		return geom.Vec3{X: values[0], Y: values[1], Z: values[2]}, nil
	}
	recognize := func(wire []Use, hole bool) (geom.PlanarCircularProfile, error) {
		if len(wire) != 4 {
			return geom.PlanarCircularProfile{}, geom.ErrConvergence
		}
		var common anchoredCircularUV
		var center geom.Vec3
		ids := map[int]bool{}
		for i, use := range wire {
			if e := ctx.Err(); e != nil {
				return geom.PlanarCircularProfile{}, e
			}
			arc, ok := use.Curve.(anchoredCircularUV)
			if !ok || use.Reversed || arc.Start != use.UVStart || arc.End != use.UVEnd {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			if _, e := newAnchoredCircularUV(arc.EllipseUV, arc.Start, arc.End); e != nil {
				return geom.PlanarCircularProfile{}, e
			}
			sign := 1.
			if hole {
				sign = -1
			}
			if arc.From != float64(i)*math.Pi/2 || arc.To != float64(i+1)*math.Pi/2 || arc.Sin.Y != sign*arc.Cos.X {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}

			for end, uv := range []geom.Vec2{arc.Start, arc.End} {
				q := (i + end) % 4
				cosines := [4]int64{1, 0, -1, 0}
				sines := [4]int64{0, 1, 0, -1}
				x := new(big.Rat).Add(new(big.Rat).SetFloat64(arc.Center.X), new(big.Rat).Mul(new(big.Rat).SetFloat64(arc.Cos.X), big.NewRat(cosines[q], 1)))
				y := new(big.Rat).Add(new(big.Rat).SetFloat64(arc.Center.Y), new(big.Rat).Mul(new(big.Rat).SetFloat64(arc.Sin.Y), big.NewRat(sines[q], 1)))

				out.ExactOperations += 2
				if out.ExactOperations > o.MaxOperations || x.Num().BitLen() > o.MaxCoefficientBits || x.Denom().BitLen() > o.MaxCoefficientBits || y.Num().BitLen() > o.MaxCoefficientBits || y.Denom().BitLen() > o.MaxCoefficientBits {
					return geom.PlanarCircularProfile{}, geom.ErrConvergence
				}
				xf, e := geom.RepresentedExactScalar(x)
				if e != nil {
					return geom.PlanarCircularProfile{}, e
				}
				yf, e := geom.RepresentedExactScalar(y)
				if e != nil {
					return geom.PlanarCircularProfile{}, e
				}
				if uv != (geom.Vec2{X: xf, Y: yf}) {
					return geom.PlanarCircularProfile{}, geom.ErrConvergence
				}
			}
			if i == 0 {
				common = arc
				var e error
				center, e = exactWorld(frame, geom.Vec3{X: arc.Center.X, Y: arc.Center.Y})
				if e != nil {
					return geom.PlanarCircularProfile{}, e
				}
			} else if arc.Center != common.Center || arc.Cos != common.Cos || arc.Sin != common.Sin {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			if use.Edge < 0 || use.Edge >= len(target.edges) || ids[use.Edge] {
				return geom.PlanarCircularProfile{}, geom.ErrInvalid
			}
			ids[use.Edge] = true
			edge := target.edges[use.Edge]
			if edge.Start < 0 || edge.End < 0 || edge.Start >= len(target.vertices) || edge.End >= len(target.vertices) || edge.Range.Min != 0 || edge.Range.Max != 1 {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			next := wire[(i+1)%4]
			if next.Edge < 0 || next.Edge >= len(target.edges) || edge.End != target.edges[next.Edge].Start {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			curve, ok := edge.Curve.(surfaceTrimCurve)
			if !ok || curve.curve != nil || curve.a.X != arc.From || curve.b.X != arc.To || curve.a.Y != curve.b.Y {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			cyl, ok := curve.surface.(geom.CylinderSurface)
			if !ok {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			cf, r := cyl.Parameters()
			if r != arc.Cos.X || cf.XAxis() != frame.XAxis() || cf.YAxis() != frame.YAxis().Scale(sign) || cf.ZAxis() != frame.ZAxis().Scale(sign) {
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			mc, e := exactWorld(cf, geom.Vec3{Z: curve.a.Y})
			if e != nil || mc != center {
				if e != nil {
					return geom.PlanarCircularProfile{}, e
				}
				return geom.PlanarCircularProfile{}, geom.ErrConvergence
			}
			for j, uv := range []geom.Vec2{arc.Start, arc.End} {
				point, e := exactWorld(frame, geom.Vec3{X: uv.X, Y: uv.Y})
				if e != nil {
					return geom.PlanarCircularProfile{}, e
				}
				index := edge.Start
				if j == 1 {
					index = edge.End
				}
				if target.vertices[index] != point {
					return geom.PlanarCircularProfile{}, geom.ErrConvergence
				}
			}
		}
		return geom.NewPlanarCircularProfile(center, frame.ZAxis(), common.Cos.X)
	}
	var e error
	out.Outer, e = recognize(face.Outer, false)
	if e != nil {
		return out, e
	}
	if len(face.Holes) > 16 {
		return out, geom.ErrConvergence
	}
	for _, wire := range face.Holes {
		hole, e := recognize(wire, true)
		if e != nil {
			return out, e
		}
		out.Holes = append(out.Holes, hole)
	}
	compare := func(a, b geom.PlanarCircularProfile) (geom.CircularProfileComparison, error) {
		remaining := o.MaxOperations - out.ExactOperations
		if remaining < 1 {
			return geom.CircularProfileComparison{}, geom.ErrConvergence
		}
		r, e := geom.ComparePlanarCircularProfiles(ctx, a, b, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		out.ExactOperations += r.ExactOperations
		return r, e
	}
	for i, hole := range out.Holes {
		r, e := compare(out.Outer, hole)
		if e != nil {
			return out, e
		}
		if r.Relation != geom.CircularProfileAStrictlyContainsB {
			return out, geom.ErrConvergence
		}
		for j := 0; j < i; j++ {
			r, e := compare(hole, out.Holes[j])
			if e != nil {
				return out, e
			}
			if r.Relation != geom.CircularProfilesDisjoint {
				return out, geom.ErrConvergence
			}
		}
	}
	return out, nil
}

type CylinderToRoundFaceProof struct {
	Target          CircularFaceRecognition
	Height          float64
	Endpoint        geom.Vec3
	Outer           geom.CircularProfileComparison
	Holes           []geom.CircularProfileComparison
	ExactOperations int
}

// CylinderToRoundFace builds a native analytic cylinder ending on a recognized
// actual circular cap with optional eccentric circular holes. Normal must be a
// coordinate-axis direction parallel to the cap; magnitude is ignored. Exact
// height/endpoint and disk coverage are proved; first contact with other faces
// and Boolean removal remain outside scope. Analytic topology has its separate
// numerical sewing tolerance, independent of the exact coverage predicates.
func CylinderToRoundFace(ctx context.Context, center, normal geom.Vec3, radius float64, target Solid, faceIndex int, o geom.SurfacePatchEvaluationOptions) (Solid, CylinderToRoundFaceProof, error) {
	var proof CylinderToRoundFaceProof
	if !center.IsFinite() || !normal.IsFinite() {
		return Solid{}, proof, geom.ErrInvalid
	}
	recognition, e := RecognizeCircularPlanarFace(ctx, target, faceIndex, o)
	proof.Target = recognition
	proof.ExactOperations = recognition.ExactOperations
	if e != nil {
		return Solid{}, proof, e
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
	cap, capNormal, _ := recognition.Outer.Parameters()
	normals := [3]float64{capNormal.X, capNormal.Y, capNormal.Z}
	if normals[axis] == 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	for i, q := range normals {
		if i != axis && q != 0 {
			return Solid{}, proof, geom.ErrDomain
		}
	}
	coordinate := func(v geom.Vec3) float64 { return [3]float64{v.X, v.Y, v.Z}[axis] }
	h := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(cap)), new(big.Rat).SetFloat64(coordinate(center)))
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
	compare := func(other geom.PlanarCircularProfile) (geom.CircularProfileComparison, error) {
		remaining := max - proof.ExactOperations
		if remaining < 1 {
			return geom.CircularProfileComparison{}, geom.ErrConvergence
		}
		r, e := geom.ComparePlanarCircularProfiles(ctx, disk, other, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		proof.ExactOperations += r.ExactOperations
		return r, e
	}
	proof.Outer, e = compare(recognition.Outer)
	if e != nil {
		return Solid{}, proof, e
	}
	if proof.Outer.Relation != geom.CircularProfileBStrictlyContainsA && proof.Outer.Relation != geom.CircularProfileBContainsATangent && proof.Outer.Relation != geom.CircularProfilesCoincident {
		return Solid{}, proof, brep.ErrTerminationUncovered
	}
	for _, hole := range recognition.Holes {
		r, e := compare(hole)
		if e != nil {
			return Solid{}, proof, e
		}
		proof.Holes = append(proof.Holes, r)
		if r.Relation != geom.CircularProfilesDisjoint && r.Relation != geom.CircularProfilesExternalTangent {
			return Solid{}, proof, brep.ErrTerminationUncovered
		}
	}
	frame, e := geom.FrameFromNormal(center, unit)
	if e != nil {
		return Solid{}, proof, e
	}
	if frame.ToWorld(geom.Vec3{Z: height}) != endpoint {
		return Solid{}, proof, geom.ErrUnrepresentable
	}
	native, e := parametric.Cylinder(frame, radius, height)
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

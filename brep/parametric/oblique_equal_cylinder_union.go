package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ObliqueEqualCylinderUnionResult retains analytic faces and the exact
// infinite-support common volume used in the finite union identity.
type ObliqueEqualCylinderUnionResult struct {
	Solid                                   Solid
	WallFaces, LowerCapFaces, UpperCapFaces [2][]int
	CommonVolumeSquared                     *big.Rat
}

// ObliqueEqualCylinderUnion fuses two equal-radius cylinders with crossing,
// equal-norm nonparallel raw axes. Both caps of each cylinder must strictly
// clear the infinite-support common, so their four disks remain exposed.
// The original model-space cylinder walls and caps are evaluated directly.
func ObliqueEqualCylinderUnion(ctx context.Context, center, u, v geom.Vec3, r float64, uCaps, vCaps [2]float64, tolerance float64) (ObliqueEqualCylinderUnionResult, error) {
	var empty ObliqueEqualCylinderUnionResult
	if ctx == nil || !center.IsFinite() || !finite(r) || r <= 0 || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	s, e := newObliqueCylinderUnionChart(ctx, center, u, v, r, uCaps, vCaps)
	if e != nil {
		return empty, e
	}
	if r/8 <= tolerance {
		return empty, geom.ErrConvergence
	}
	var out ObliqueEqualCylinderUnionResult
	var faces []Face
	for owner := range 2 {
		for _, side := range []int{-1, 1} {
			for _, half := range []int{-1, 1} {
				for cap := range 2 {
					s.owner, s.side, s.half, s.cap = owner, side, half, cap == 1
					id := len(faces)
					if !s.cap {
						out.WallFaces[owner] = append(out.WallFaces[owner], id)
					} else if side < 0 {
						out.LowerCapFaces[owner] = append(out.LowerCapFaces[owner], id)
					} else {
						out.UpperCapFaces[owner] = append(out.UpperCapFaces[owner], id)
					}
					reverse := owner == 0 && side*half < 0 || owner == 1 && side*half > 0
					if s.cap {
						reverse = !reverse
					}
					faces = append(faces, Face{Surface: s, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{0, 1}, Reversed: reverse})
				}
			}
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	out.CommonVolumeSquared = new(big.Rat).Set(s.measure.VolumeSquared)
	return out, nil
}

type obliqueCylinderUnionChart struct {
	center, u, v      geom.Vec3
	radius            float64
	caps              [2][2]float64
	A, B, W, U, V     geom.Vec3
	cot, invSin       float64
	measure           geom.EqualCylinderCommonMeasure
	owner, side, half int
	cap               bool
}

func newObliqueCylinderUnionChart(ctx context.Context, center, u, v geom.Vec3, r float64, uCaps, vCaps [2]float64) (obliqueCylinderUnionChart, error) {
	var empty obliqueCylinderUnionChart
	if !center.IsFinite() {
		return empty, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	c := [2][2]float64{uCaps, vCaps}
	var windows [2][2]*big.Rat
	for i, a := range c {
		if !finite(a[0]) || !finite(a[1]) || a[0] >= a[1] {
			return empty, geom.ErrInvalid
		}
		windows[i] = [2]*big.Rat{q(a[0]), q(a[1])}
	}
	proof, e := geom.MeasureEqualCylinderCommon(ctx, u, v, r, &windows[0], &windows[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	if !proof.CapsCover || proof.CapTouchMask != 0 {
		return empty, geom.ErrConvergence
	}
	// Obtain only the linear columns about zero. A large translation
	// must not erase a small component of the basis.
	m, e := geom.NewObliqueSkewCylinderMap(ctx, geom.Vec3{}, u, r, geom.Vec3{}, v, r, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	x, e := m.Eval(1, 0, 0)
	if e != nil {
		return empty, e
	}
	y, e := m.Eval(0, 1, 0)
	if e != nil {
		return empty, e
	}
	z, e := m.Eval(0, 0, 1)
	if e != nil {
		return empty, e
	}
	uu, e := u.Unit()
	if e != nil {
		return empty, e
	}
	vv, e := v.Unit()
	if e != nil {
		return empty, e
	}
	ss, _ := proof.SinSquared.Float64()
	sin := math.Sqrt(ss)
	cos := uu.Dot(vv)
	if !finite(sin) || sin <= 0 {
		return empty, geom.ErrUnrepresentable
	}
	return obliqueCylinderUnionChart{center: center, u: u, v: v, radius: r, caps: c, A: x, B: y, W: z, U: uu, V: vv, cot: cos / sin, invSin: 1 / sin, measure: proof}, nil
}

func (s obliqueCylinderUnionChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	// The ideal section basis has both column norms 1/sin(theta).
	// Since 0<sin²(theta)<=sin(theta), the reciprocal squared sine
	// is a rational upper bound on either basis length.
	n := upperBoreRat(new(big.Rat).Inv(s.measure.SinSquared))
	if s.cap {
		return upMul(2, s.radius), s.radius, 0, nil
	}
	return upMul(s.radius, upAdd(1, upMul(2, n))), upMul(s.radius, upAdd(1, upMul(2, n))), 0, nil
}
func (s obliqueCylinderUnionChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s obliqueCylinderUnionChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s obliqueCylinderUnionChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	w, y := s.radius*cs, s.radius*sn
	sg, half := float64(s.side), float64(s.half)
	primary, secondary, axis, n := s.A, s.B, s.u, s.U
	if s.owner == 1 {
		primary, secondary, axis, n = s.B, s.A, s.v, s.V
	}
	end := s.caps[s.owner][0]
	if s.side > 0 {
		end = s.caps[s.owner][1]
	}
	capAxis := axis.Scale(end)
	if s.cap {
		// Remove the longitudinal projection from the section basis;
		// the remaining vector is unit and orthogonal to W.
		radial := secondary.Sub(n.Scale(s.cot)).Scale(half)
		p = capAxis.Add(radial.Scale(w * v)).Add(s.W.Scale(y))
		du = radial.Scale(-y * v).Add(s.W.Scale(w))
		dv = radial.Scale(w)
	} else {
		edge := primary.Scale(sg).Add(secondary.Scale(half))
		radial := secondary.Sub(n.Scale(s.cot)).Scale(half)
		atCap := capAxis.Add(radial.Scale(w)).Add(s.W.Scale(y))
		atRim := edge.Scale(w).Add(s.W.Scale(y))
		p = atRim.Scale(1 - v).Add(atCap.Scale(v))
		du = edge.Scale(-y * (1 - v)).Add(radial.Scale(-y * v)).Add(s.W.Scale(w))
		dv = atCap.Sub(atRim)
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

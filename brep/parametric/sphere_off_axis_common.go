package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereOffAxisCommonResult retains the native wall and whichever spherical
// or planar end faces bound the material.
type SphereOffAxisCommonResult struct {
	Solid                                          Solid
	CylindricalFaces, LowerEndFaces, UpperEndFaces []int
}

// SphereOffAxisCylinderCommon constructs the intersection of a sphere and a
// finite cylinder whose projected disk is strictly inside the sphere's
// equatorial disk. Each cylinder end must either leave the entire corresponding
// spherical cap intact or cut through the full cylindrical disk. A plane
// crossing only part of a cap or rim is outside this shell construction.
func SphereOffAxisCylinderCommon(ctx context.Context, frame geom.Frame, sphereRadius, cylinderRadius, offset, lower, upper, tolerance float64) (SphereOffAxisCommonResult, error) {
	var empty SphereOffAxisCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	s, e := newOffsetCommonChart(ctx, frame, sphereRadius, cylinderRadius, offset, lower, upper, tolerance)
	if e != nil {
		return empty, e
	}
	var out SphereOffAxisCommonResult
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		v := Interval{0, 1}
		s.kind = offsetCommonWall
		out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
		faces = append(faces, Face{Surface: s, U: u, V: v})
		s.kind = offsetCommonLower
		out.LowerEndFaces = append(out.LowerEndFaces, len(faces))
		faces = append(faces, Face{Surface: s, U: u, V: v})
		s.kind = offsetCommonUpper
		out.UpperEndFaces = append(out.UpperEndFaces, len(faces))
		faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: true})
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

const (
	offsetCommonWall = iota
	offsetCommonLower
	offsetCommonUpper
)

type offsetCommonChart struct {
	frame                          geom.Frame
	R, r, offset, lower, upper     float64
	kind                           int
	lowerSpherical, upperSpherical bool
	cap                            geom.SphereToolCapPatch
}

func newOffsetCommonChart(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64) (offsetCommonChart, error) {
	var out offsetCommonChart
	if !finite(R) || !finite(r) || !finite(offset) || !finite(lower) || !finite(upper) ||
		R <= 0 || r <= 0 || offset < 0 || lower >= upper || frame.ZAxis().Length() == 0 {
		return out, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	far := add(q(offset), q(r))
	if sub(q(upper), q(lower)).Cmp(pad) <= 0 || q(r).Cmp(pad) <= 0 || sub(q(R), far).Cmp(pad) <= 0 {
		return out, geom.ErrConvergence
	}
	near := sub(q(offset), q(r))
	if near.Sign() < 0 {
		near.SetInt64(0)
	}
	minH2, maxH2 := sub(sq(q(R)), sq(far)), sub(sq(q(R)), sq(near))
	classify := func(outward float64) (bool, error) {
		// At or beyond the entire polar cap, an axial plane changes
		// no positive material. Inside the central band it cuts a full
		// disk. Remaining cases require a partial cap arrangement.
		if outward > 0 && sq(q(outward)).Cmp(maxH2) >= 0 {
			return true, nil
		}
		if sq(add(q(math.Abs(outward)), pad)).Cmp(minH2) < 0 {
			return false, nil
		}
		return false, geom.ErrConvergence
	}
	bottom, e := classify(-lower)
	if e != nil {
		return out, e
	}
	top, e := classify(upper)
	if e != nil {
		return out, e
	}
	cap, e := geom.NewSphereToolCapPatch(ctx, R, r, offset, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return out, e
	}
	return offsetCommonChart{frame, R, r, offset, lower, upper, offsetCommonWall, bottom, top, cap}, nil
}

func (s offsetCommonChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s offsetCommonChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s offsetCommonChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	if u == 2*math.Pi {
		u = 0
	}
	sn, cs := math.Sincos(u)
	radial, tangent := geom.Vec3{X: cs, Y: sn}, geom.Vec3{X: -sn, Y: cs}
	o := geom.Vec3{X: s.offset}
	switch s.kind {
	case offsetCommonWall:
		lo, hi, a, b := s.lower, s.upper, 0., 0.
		if s.lowerSpherical || s.upperSpherical {
			rim, e := s.cap.Eval(u, 1)
			if e != nil {
				return p, du, dv, e
			}
			d, _, e := s.cap.Partials(u, 1)
			if e != nil {
				return p, du, dv, e
			}
			if s.lowerSpherical {
				lo, a = -rim.Z, -d.Z
			}
			if s.upperSpherical {
				hi, b = rim.Z, d.Z
			}
		}
		height := (1-v)*lo + v*hi
		p = o.Add(radial.Scale(s.r)).Add(geom.Vec3{Z: height})
		du = tangent.Scale(s.r).Add(geom.Vec3{Z: (1-v)*a + v*b})
		dv = geom.Vec3{Z: hi - lo}
	case offsetCommonLower, offsetCommonUpper:
		hi := s.kind == offsetCommonUpper
		if hi && s.upperSpherical || !hi && s.lowerSpherical {
			p, err = s.cap.Eval(u, v)
			if err == nil {
				du, dv, err = s.cap.Partials(u, v)
			}
			if !hi {
				p.Z, du.Z, dv.Z = -p.Z, -du.Z, -dv.Z
			}
		} else {
			h := s.lower
			if hi {
				h = s.upper
			}
			p = o.Add(radial.Scale(s.r * v)).Add(geom.Vec3{Z: h})
			du, dv = tangent.Scale(s.r*v), radial.Scale(s.r)
		}
	default:
		err = geom.ErrInvalid
	}
	if err != nil {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, err
	}
	p, du, dv = s.frame.ToWorld(p), frameVector(s.frame, du), frameVector(s.frame, dv)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s offsetCommonChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	uu, uv, vv := s.r, s.r, 0.
	if s.kind == offsetCommonWall {
		uv = 0
		if s.lowerSpherical || s.upperSpherical {
			b, e := geom.BoundSphereCylinderRimHeight(ctx, geom.Vec3{}, s.R, geom.Vec3{X: s.offset}, geom.Vec3{Z: 1}, s.r, geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				return 0, 0, 0, e
			}
			uu = upAdd(uu, upperBoreRat(b.SecondDerivativeAbsolute))
			uv = upperBoreRat(b.FirstDerivativeAbsolute)
			if s.lowerSpherical && s.upperSpherical {
				uv = upMul(uv, 2)
			}
		}
	} else if s.kind == offsetCommonLower && s.lowerSpherical || s.kind == offsetCommonUpper && s.upperSpherical {
		b, e := s.cap.HessianBounds()
		if e != nil {
			return 0, 0, 0, e
		}
		uu, uv, vv = upperBoreRat(b.UU), upperBoreRat(b.UV), upperBoreRat(b.VV)
	}
	n := boreFrameNorm(s.frame)
	return upMul(uu, n), upMul(uv, n), upMul(vv, n), nil
}

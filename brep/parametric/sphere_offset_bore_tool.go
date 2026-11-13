package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

const (
	toolBoreLower = offsetBoreFloor + 1 + iota
	toolBoreUpper
	toolBoreWall
	toolBoreFloor
)

// toolBoreChart uses the same tool-centered angle across the two sphere
// patches, the cylinder wall, and the floor. The lower sphere chart collapses
// only at the south pole. The upper one ends at the equator without a
// singular square-root derivative.
type toolBoreChart struct {
	offsetBoreChart
	upper   geom.SphereToolUpperPatch
	equator geom.SphereToolEquator
}

func newToolBoreChart(ctx context.Context, s offsetBoreChart, tolerance float64) (toolBoreChart, error) {
	var result toolBoreChart
	if !finite(s.radius) || !finite(s.bore) || !finite(s.offset) || !finite(s.floor) ||
		s.offset < 0 || s.bore <= 0 || s.radius <= 0 || s.frame.ZAxis().Length() == 0 ||
		s.kind < toolBoreLower || s.kind > toolBoreFloor {
		return result, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	outer := new(big.Rat).Add(q(s.offset), q(s.bore))
	near := new(big.Rat).Add(q(math.Abs(s.floor)), pad)
	if q(s.bore).Cmp(pad) <= 0 ||
		new(big.Rat).Sub(q(s.radius), outer).Cmp(pad) <= 0 ||
		new(big.Rat).Add(sq(outer), sq(near)).Cmp(sq(q(s.radius))) >= 0 {
		return result, geom.ErrConvergence
	}
	upper, e := geom.NewSphereToolUpperPatch(ctx, s.radius, s.bore, s.offset, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return result, e
	}
	equator, e := geom.NewSphereToolEquator(ctx, s.radius, s.offset, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return result, e
	}
	return toolBoreChart{s, upper, equator}, nil
}

func makeToolCenteredSphereBore(ctx context.Context, s offsetBoreChart, tolerance float64) (SphereOffAxisBlindBoreResult, error) {
	var empty SphereOffAxisBlindBoreResult
	s.kind = toolBoreLower
	chart, e := newToolBoreChart(ctx, s, tolerance)
	if e != nil {
		return empty, e
	}
	var out SphereOffAxisBlindBoreResult
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		for kind := toolBoreLower; kind <= toolBoreFloor; kind++ {
			chart.kind = kind
			switch kind {
			case toolBoreLower, toolBoreUpper:
				out.SphericalFaces = append(out.SphericalFaces, len(faces))
			case toolBoreWall:
				out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
			case toolBoreFloor:
				out.FloorFaces = append(out.FloorFaces, len(faces))
			}
			faces = append(faces, Face{Surface: chart, U: u, V: Interval{0, 1}, Reversed: kind != toolBoreLower})
		}
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

func (s toolBoreChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.toolEval(u, v)
	return p, e
}
func (s toolBoreChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.toolEval(u, v)
	return a, b, e
}
func (s toolBoreChart) toolEval(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	if u == 2*math.Pi {
		u = 0
	}
	sin, cos := math.Sincos(u)
	e := geom.Vec3{X: cos, Y: sin}
	n := geom.Vec3{X: -sin, Y: cos}
	z := geom.Vec3{Z: 1}
	o := geom.Vec3{X: s.offset}
	switch s.kind {
	case toolBoreUpper:
		p, err = s.upper.Eval(u, v)
		if err == nil {
			du, dv, err = s.upper.Partials(u, v)
		}
	case toolBoreLower:
		var xy geom.Vec2
		var phiFirst float64
		xy, _, phiFirst, _, err = s.equator.Evaluate(u)
		if err == nil {
			r := geom.Vec3{X: xy.X, Y: xy.Y}
			t := geom.Vec3{X: -xy.Y, Y: xy.X}
			sl, cl := math.Sincos((v - 1) * (math.Pi / 2))
			if v == 0 {
				sl, cl = -1, 0
			}
			p = r.Scale(cl).Add(z.Scale(s.radius * sl))
			du = t.Scale(cl * phiFirst)
			dv = r.Scale(-sl * math.Pi / 2).Add(z.Scale(s.radius * cl * math.Pi / 2))
		}
	case toolBoreWall:
		var rim geom.Vec3
		var dRim geom.Vec3
		rim, err = s.upper.Eval(u, 0)
		if err == nil {
			dRim, _, err = s.upper.Partials(u, 0)
		}
		if err == nil {
			p = o.Add(e.Scale(s.bore)).Add(z.Scale(s.floor + v*(rim.Z-s.floor)))
			if v == 1 {
				p = rim
			}
			du = n.Scale(s.bore).Add(z.Scale(v * dRim.Z))
			dv = z.Scale(rim.Z - s.floor)
		}
	case toolBoreFloor:
		p = o.Add(e.Scale(v * s.bore)).Add(z.Scale(s.floor))
		du, dv = n.Scale(v*s.bore), e.Scale(s.bore)
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

func (s toolBoreChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	var uu, uv, vv float64
	switch s.kind {
	case toolBoreUpper:
		b, e := s.upper.HessianBounds()
		if e != nil {
			return 0, 0, 0, e
		}
		uu, uv, vv = upperBoreRat(b.UU), upperBoreRat(b.UV), upperBoreRat(b.VV)
	case toolBoreLower:
		p := upperBoreRat(s.equator.SphereAngleFirst[1])
		q := upperBoreRat(s.equator.SphereAngleSecondAbsolute)
		k := up(math.Pi / 2)
		uu, uv, vv = upMul(s.radius, upAdd(upMul(p, p), q)), upMul(s.radius, upMul(p, k)), upMul(s.radius, upMul(k, k))
	case toolBoreWall:
		b, e := geom.BoundSphereCylinderRimHeight(ctx, geom.Vec3{}, s.radius, geom.Vec3{X: s.offset}, geom.Vec3{Z: 1}, s.bore, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return 0, 0, 0, e
		}
		uu, uv = upAdd(s.bore, upperBoreRat(b.SecondDerivativeAbsolute)), upperBoreRat(b.FirstDerivativeAbsolute)
	case toolBoreFloor:
		uu, uv = s.bore, s.bore
	default:
		return 0, 0, 0, geom.ErrInvalid
	}
	n := boreFrameNorm(s.frame)
	return upMul(uu, n), upMul(uv, n), upMul(vv, n), nil
}

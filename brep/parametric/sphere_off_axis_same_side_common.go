package parametric

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// SphereOffAxisSameSideCommonResult carries the planar lenses and spherical
// zone of a finite cylinder clipped by two caps on one side of the equator.
type SphereOffAxisSameSideCommonResult struct {
	Solid                            Solid
	CylindricalFaces, SphericalFaces []int
	PlanarEndFaces                   [2][]int
	Incidence                        geom.SphereCylinderSameSideSlab
}

// SphereOffAxisCylinderSameSideCommon requires two strict transverse lens
// caps on the same open side of the equator and a contained tool footprint.
// A common pole at the far cap's exact radical chord lies strictly inside
// both nested lens disks. The cylinder owns only the angular intervals where
// its wall reaches the nearer cap.
func SphereOffAxisCylinderSameSideCommon(ctx context.Context, frame geom.Frame, R, r, d, lower, upper, tolerance float64) (SphereOffAxisSameSideCommonResult, error) {
	var empty SphereOffAxisSameSideCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	s, inc, e := newOffsetSameSideCommon(ctx, frame, R, r, d, lower, upper, tolerance)
	if e != nil {
		return empty, e
	}
	n, f := s.end[s.near], s.end[s.far]
	cuts := slices.Compact([]float64{0, n.angle, f.angle, math.Pi, f.finish, n.finish, 2 * math.Pi})
	out := SphereOffAxisSameSideCommonResult{Incidence: inc}
	var faces []Face
	for i := 0; i+1 < len(cuts); i++ {
		u := Interval{cuts[i], cuts[i+1]}
		if !u.valid() || (u.Max-u.Min)*r <= 8*tolerance {
			return empty, geom.ErrConvergence
		}
		for j, x := range s.end {
			s.clipped[j] = u.Max <= x.angle || u.Min >= x.finish
		}
		v := Interval{0, 1}
		for j := range 2 {
			s.kind = sameSideLower + j
			out.PlanarEndFaces[j] = append(out.PlanarEndFaces[j], len(faces))
			faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: j == 1})
		}
		if !s.clipped[s.near] {
			s.kind = sameSideWall
			out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
			faces = append(faces, Face{Surface: s, U: u, V: v})
		}
		if s.clipped[s.far] {
			s.kind = sameSideStrip
			out.SphericalFaces = append(out.SphericalFaces, len(faces))
			faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: s.far == 1})
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

const (
	sameSideWall = iota
	sameSideLower
	sameSideUpper
	sameSideStrip
)

type offsetSameSideCommon struct {
	offsetDoubleCommon
	near, far int
}

func newOffsetSameSideCommon(ctx context.Context, frame geom.Frame, R, r, d, lo, hi, tol float64) (offsetSameSideCommon, geom.SphereCylinderSameSideSlab, error) {
	var empty offsetSameSideCommon
	var no geom.SphereCylinderSameSideSlab
	if lo == 0 || hi == 0 || lo < 0 && hi > 0 {
		return empty, no, geom.ErrDomain
	}
	b, _, e := newOffsetDoubleCommon(ctx, frame, R, r, d, lo, hi, tol)
	if e != nil {
		return empty, no, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	inc, e := geom.ClassifySameSideSphereCylinderSlab(ctx, geom.Vec3{}, R, geom.Vec3{X: d}, geom.Vec3{Z: 1}, r, [2]*big.Rat{q(lo), q(hi)}, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, no, e
	}
	near, far := inc.NearCap, inc.FarCap
	if b.end[near].angle >= b.end[far].angle || b.end[near].finish <= b.end[far].finish {
		return empty, no, geom.ErrUnrepresentable
	}
	// The far chord is interior to the far section disk. The near exit
	// has the same pole and the extra squared radius hFar²−hNear².
	hub := inc.Arrangement.Caps[far].Events.Points[0].Base[0]
	h := [2]float64{lo, hi}
	sq := func(a *big.Rat) *big.Rat { return new(big.Rat).Mul(a, a) }
	nearMargin := new(big.Rat).Sub(new(big.Rat).Sub(sq(q(R)), sq(q(h[near]))), sq(hub))
	f, _ := new(big.Rat).Quo(nearMargin, sq(q(R))).Float64()
	if !finite(f) || f <= b.end[far].margin {
		return empty, no, geom.ErrUnrepresentable
	}
	b.end[near].margin, b.end[near].hub = f, b.end[far].hub
	return offsetSameSideCommon{b, near, far}, inc, nil
}
func (s offsetSameSideCommon) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s offsetSameSideCommon) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s offsetSameSideCommon) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	var event [2]bool
	for j, a := range s.end {
		event[j] = u == a.angle || u == a.finish
		if !event[j] && s.clipped[j] != (u < a.angle || u > a.finish) {
			return p, du, dv, geom.ErrDomain
		}
	}
	if u == 2*math.Pi {
		u = 0
	}
	sn, cs := math.Sincos(u)
	r, d := s.r/s.R, s.d/s.R
	b, t := geom.Vec3{X: d + r*cs, Y: r * sn}, geom.Vec3{X: -r * sn, Y: r * cs}
	z := geom.Vec3{Z: 1}
	if s.kind == sameSideWall {
		if s.clipped[s.near] {
			return p, du, dv, geom.ErrDomain
		}
		h := [2]float64{s.end[0].cap, s.end[1].cap}
		dh := 0.
		if s.clipped[s.far] {
			n := b.Length()
			hf := math.Copysign(math.Sqrt((1-n)*(1+n)), h[s.far])
			dh = -b.Dot(t) / hf
			if event[s.near] {
				h[s.far] = h[s.near]
			} else if !event[s.far] {
				h[s.far] = hf
			}
		}
		a, bv := 0., 0.
		if s.far == 0 {
			a = dh
		} else {
			bv = dh
		}
		p, du, dv = b.Add(z.Scale((1-v)*h[0]+v*h[1])), t.Add(z.Scale((1-v)*a+v*bv)), z.Scale(h[1]-h[0])
	} else {
		hub := geom.Vec3{X: s.end[s.far].hub}
		w := b.Sub(hub)
		c := w.Length()
		if c <= 0 || !finite(c) {
			return p, du, dv, geom.ErrConvergence
		}
		e := w.Scale(1 / c)
		dc := e.Dot(t)
		de := t.Sub(e.Scale(dc)).Scale(1 / c)
		exit := func(j int) (rho, deriv float64) {
			if !s.clipped[j] {
				return c, dc
			}
			x := -hub.X * e.X
			root := math.Sqrt(s.end[j].margin + x*x)
			rho = x + root
			if x < 0 {
				rho = s.end[j].margin / (root - x)
			}
			if event[j] {
				rho = c
			}
			return rho, -hub.X * de.X * rho / root
		}
		switch s.kind {
		case sameSideLower, sameSideUpper:
			j := s.kind - sameSideLower
			rho, drho := exit(j)
			p = hub.Add(e.Scale(v * rho)).Add(z.Scale(s.end[j].cap))
			du = de.Scale(v * rho).Add(e.Scale(v * drho))
			dv = e.Scale(rho)
		case sameSideStrip:
			if !s.clipped[s.far] {
				return p, du, dv, geom.ErrDomain
			}
			a, da := exit(s.far)
			b, db := exit(s.near)
			x := hub.Add(e.Scale((1-v)*a + v*b))
			xu := de.Scale((1-v)*a + v*b).Add(e.Scale((1-v)*da + v*db))
			xv := e.Scale(b - a)
			n := x.Length()
			h := math.Copysign(math.Sqrt((1-n)*(1+n)), s.end[s.far].cap)
			if v == 0 || event[s.far] {
				h = s.end[s.far].cap
			} else if v == 1 && (s.clipped[s.near] || event[s.near]) {
				h = s.end[s.near].cap
			}
			p, du, dv = x.Add(z.Scale(h)), xu.Add(z.Scale(-x.Dot(xu)/h)), xv.Add(z.Scale(-x.Dot(xv)/h))
		default:
			return p, du, dv, geom.ErrDomain
		}
	}
	p, du, dv = s.frame.ToWorld(p.Scale(s.R)), frameVector(s.frame, du.Scale(s.R)), frameVector(s.frame, dv.Scale(s.R))
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

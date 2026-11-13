package parametric

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// SphereOffAxisDoubleCommonResult retains the common cylinder wall and each
// planar lens, with algebraic cap events and adjacent spherical strips.
type SphereOffAxisDoubleCommonResult struct {
	Solid                             Solid
	CylindricalFaces                  []int
	PlanarEndFaces, SphericalEndFaces [2][]int
	CapMaterial                       [2]geom.SphereCylinderCapMaterial
	AngularEvents                     geom.SphereCylinderDoubleCapEvents
}

// SphereOffAxisCylinderDoubleCommon assembles a contained offset cylinder
// clipped by two transverse sphere-section lenses. The planes straddle the
// equator. Each end uses its own exact radical-chord midpoint and both share
// the same tool azimuth along the wall, even when their cap radii differ.
func SphereOffAxisCylinderDoubleCommon(ctx context.Context, frame geom.Frame, R, r, d, lower, upper, tolerance float64) (SphereOffAxisDoubleCommonResult, error) {
	var empty SphereOffAxisDoubleCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	s, caps, e := newOffsetDoubleCommon(ctx, frame, R, r, d, lower, upper, tolerance)
	if e != nil {
		return empty, e
	}
	// Sort by the exact cosine events first. Only identical section radii
	// can coincide; distinct events must remain distinct when represented.
	order := [2]int{0, 1}
	if s.end[0].cosine.Cmp(s.end[1].cosine) < 0 {
		order = [2]int{1, 0}
	}
	if s.end[0].cosine.Cmp(s.end[1].cosine) != 0 && !(s.end[order[0]].angle < s.end[order[1]].angle) {
		return empty, geom.ErrConvergence
	}
	cuts := []float64{0, s.end[order[0]].angle, s.end[order[1]].angle, math.Pi, s.end[order[1]].finish, s.end[order[0]].finish, 2 * math.Pi}
	cuts = slices.Compact(cuts)
	var faces []Face
	out := SphereOffAxisDoubleCommonResult{CapMaterial: caps.Caps, AngularEvents: caps}
	for i := 0; i+1 < len(cuts); i++ {
		u := Interval{cuts[i], cuts[i+1]}
		if !u.valid() || (u.Max-u.Min)*r <= 8*tolerance {
			return empty, geom.ErrConvergence
		}
		v := Interval{0, 1}
		for j, p := range s.end {
			s.clipped[j] = u.Max <= p.angle || u.Min >= p.finish
		}
		s.kind = doubleWall
		out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
		faces = append(faces, Face{Surface: s, U: u, V: v})
		for j := range 2 {
			s.kind = doubleLowerPlanar + 2*j
			out.PlanarEndFaces[j] = append(out.PlanarEndFaces[j], len(faces))
			faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: j == 1})
			if s.clipped[j] {
				s.kind++
				out.SphericalEndFaces[j] = append(out.SphericalEndFaces[j], len(faces))
				faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: j == 1})
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
	return out, nil
}

const (
	doubleWall = iota
	doubleLowerPlanar
	doubleLowerStrip
	doubleUpperPlanar
	doubleUpperStrip
)

type doubleCommonEnd struct {
	cosine                          *big.Rat
	angle, finish, cap, hub, margin float64
}
type offsetDoubleCommon struct {
	frame                 geom.Frame
	R, r, d, lower, upper float64
	end                   [2]doubleCommonEnd
	clipped               [2]bool
	kind                  int
}

func newOffsetDoubleCommon(ctx context.Context, frame geom.Frame, R, r, d, lower, upper, tolerance float64) (offsetDoubleCommon, geom.SphereCylinderDoubleCapEvents, error) {
	var empty offsetDoubleCommon
	var caps geom.SphereCylinderDoubleCapEvents
	if !finite(R) || !finite(r) || !finite(d) || !finite(lower) || !finite(upper) || R <= 0 || r <= 0 || d <= 0 || lower >= 0 || upper <= 0 || frame.ZAxis().Length() == 0 {
		return empty, caps, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, caps, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	r2, R2 := sq(q(r)), sq(q(R))
	pad := mul(q(tolerance), big.NewRat(8, 1))
	if q(r).Cmp(pad) <= 0 || sub(q(R), add(q(r), q(d))).Cmp(pad) <= 0 {
		return empty, caps, geom.ErrConvergence
	}
	caps, e := geom.OrderSphereCylinderDoubleCapEvents(ctx, geom.Vec3{}, R, geom.Vec3{X: d}, geom.Vec3{Z: 1}, r, [2]*big.Rat{q(lower), q(upper)}, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, geom.SphereCylinderDoubleCapEvents{}, e
	}
	s := offsetDoubleCommon{frame: frame, R: R, r: r, d: d, lower: lower, upper: upper}
	for i, h := range [2]float64{lower, upper} {
		m := caps.Caps[i]
		hub := m.Events.Points[0].Base[0]
		a2 := sub(R2, sq(q(h)))
		margin := sub(a2, sq(hub))
		if margin.Cmp(sq(pad)) <= 0 || sq(sub(q(r), pad)).Cmp(sq(sub(hub, q(d)))) <= 0 ||
			a2.Cmp(sq(add(new(big.Rat).Abs(hub), pad))) <= 0 {
			return empty, caps, geom.ErrConvergence
		}
		cos := new(big.Rat).Quo(sub(sub(a2, sq(q(d))), r2), mul(big.NewRat(2, 1), mul(q(d), q(r))))
		c, _ := cos.Float64()
		angle := math.Acos(c)
		if !finite(angle) || angle <= 0 || angle >= math.Pi {
			return empty, caps, geom.ErrUnrepresentable
		}
		hf, _ := new(big.Rat).Quo(hub, q(R)).Float64()
		mf, _ := new(big.Rat).Quo(margin, R2).Float64()
		if mf <= 0 || !finite(mf) {
			return empty, caps, geom.ErrUnrepresentable
		}
		s.end[i] = doubleCommonEnd{cos, angle, 2*math.Pi - angle, h / R, hf, mf}
	}
	return s, caps, nil
}

func (s offsetDoubleCommon) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s offsetDoubleCommon) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}

func (s offsetDoubleCommon) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	event := [2]bool{}
	for j, b := range s.end {
		event[j] = u == b.angle || u == b.finish
		if !event[j] && s.clipped[j] != (u < b.angle || u > b.finish) {
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
	hyp := b.Length()
	h := math.Sqrt((1 - hyp) * (1 + hyp))
	dh := -b.Dot(t) / h
	if s.kind == doubleWall {
		lo, hi := s.end[0].cap, s.end[1].cap
		dlo, dhi := 0., 0.
		if s.clipped[0] {
			if !event[0] {
				lo = -h
			}
			dlo = -dh
		}
		if s.clipped[1] {
			if !event[1] {
				hi = h
			}
			dhi = dh
		}
		p, du, dv = b.Add(z.Scale((1-v)*lo+v*hi)), t.Add(z.Scale((1-v)*dlo+v*dhi)), z.Scale(hi-lo)
	} else if s.kind >= doubleLowerPlanar && s.kind <= doubleUpperStrip {
		j := (s.kind - doubleLowerPlanar) / 2
		strip := (s.kind-doubleLowerPlanar)%2 == 1
		a := s.end[j]
		o := geom.Vec3{X: a.hub}
		w := b.Sub(o)
		c := w.Length()
		if c <= 0 || !finite(c) {
			return p, du, dv, geom.ErrConvergence
		}
		e := w.Scale(1 / c)
		dc := e.Dot(t)
		de := t.Sub(e.Scale(dc)).Scale(1 / c)
		rho, drho := c, dc
		if s.clipped[j] || event[j] {
			// Positive exit of the sphere-section circle along the same
			// ray. Numerically rationalize the root on the far side.
			x := -a.hub * e.X
			root := math.Sqrt(a.margin + x*x)
			rho = x + root
			if x < 0 {
				rho = a.margin / (root - x)
			}
			if event[j] {
				rho = c
			}
			drho = -a.hub * de.X * rho / root
		}
		if strip {
			if !s.clipped[j] && !event[j] {
				return p, du, dv, geom.ErrDomain
			}
			x := o.Add(e.Scale((1-v)*rho + v*c))
			xu := de.Scale((1-v)*rho + v*c).Add(e.Scale((1-v)*drho + v*dc))
			xv := e.Scale(c - rho)
			n := x.Length()
			zh := math.Copysign(math.Sqrt((1-n)*(1+n)), a.cap)
			if v == 0 || event[j] {
				zh = a.cap
			}
			p, du, dv = x.Add(z.Scale(zh)), xu.Add(z.Scale(-x.Dot(xu)/zh)), xv.Add(z.Scale(-x.Dot(xv)/zh))
		} else {
			p = o.Add(e.Scale(v * rho)).Add(z.Scale(a.cap))
			du = de.Scale(v * rho).Add(e.Scale(v * drho))
			dv = e.Scale(rho)
		}
	} else {
		return p, du, dv, geom.ErrDomain
	}
	p, du, dv = s.frame.ToWorld(p.Scale(s.R)), frameVector(s.frame, du.Scale(s.R)), frameVector(s.frame, dv.Scale(s.R))
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

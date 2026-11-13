package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereOffAxisPartialCommonResult keeps the wall and both planar ends, plus
// the spherical strips that meet the upper cap along a circular arc.
type SphereOffAxisPartialCommonResult struct {
	Solid                                                Solid
	CylindricalFaces, LowerCap, UpperCap, SphericalFaces []int
	UpperCapMaterial                                     geom.SphereCylinderCapMaterial
}

// SphereOffAxisCylinderPartialCommon intersects a sphere with a contained
// offset cylinder whose lower plane cuts the full disk and whose upper plane
// cuts a transverse lens containing the tool axis. All supports are analytic.
// Upper cap events are retained exactly in UpperCapMaterial; the native
// azimuth chart uses a represented evaluation of the exact junction angle.
// The input frame expresses the local sphere center, axis and offset.
func SphereOffAxisCylinderPartialCommon(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64) (SphereOffAxisPartialCommonResult, error) {
	var empty SphereOffAxisPartialCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	s, material, e := newOffsetPartialCommon(ctx, frame, R, r, offset, lower, upper, tolerance)
	if e != nil {
		return empty, e
	}
	return sewOffsetPartialCommon(ctx, s, material, tolerance)
}

// SphereOffAxisCylinderChordCommon assembles the same one-lens operation
// around the exact radical-chord midpoint. The lens need not contain either
// circle center. The lower cap must still cut the entire cylindrical disk.
func SphereOffAxisCylinderChordCommon(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64) (SphereOffAxisPartialCommonResult, error) {
	var empty SphereOffAxisPartialCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	s, material, e := newOffsetPartialCommonMode(ctx, frame, R, r, offset, lower, upper, tolerance, true)
	if e != nil {
		return empty, e
	}
	return sewOffsetPartialCommon(ctx, s, material, tolerance)
}

func sewOffsetPartialCommon(ctx context.Context, s offsetPartialCommon, material geom.SphereCylinderCapMaterial, tolerance float64) (SphereOffAxisPartialCommonResult, error) {
	var empty SphereOffAxisPartialCommonResult
	// These cuts subdivide the full angular range into bounded patches
	// and distinguish both endpoints of the lens arcs. No patch crosses
	// the switch between the two upper-support formulae.
	cuts := []float64{0, s.angle, math.Pi, s.endAngle, 2 * math.Pi}
	out := SphereOffAxisPartialCommonResult{UpperCapMaterial: material}
	var faces []Face
	for i := 0; i+1 < len(cuts); i++ {
		u := Interval{cuts[i], cuts[i+1]}
		v := Interval{0, 1}
		for k := partialCommonWall; k <= partialCommonStrip; k++ {
			if k == partialCommonStrip && (i == 1 || i == 2) {
				continue
			}
			s.kind = k
			s.clipped = i == 0 || i == 3
			switch k {
			case partialCommonWall:
				out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
			case partialCommonLower:
				out.LowerCap = append(out.LowerCap, len(faces))
			case partialCommonUpper:
				out.UpperCap = append(out.UpperCap, len(faces))
			case partialCommonStrip:
				out.SphericalFaces = append(out.SphericalFaces, len(faces))
			}
			faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: k >= partialCommonUpper})
		}
	}
	var e error
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
	partialCommonWall = iota
	partialCommonLower
	partialCommonUpper
	partialCommonStrip
)

type offsetPartialCommon struct {
	frame                      geom.Frame
	R, r, offset, lower, upper float64
	angle, endAngle, a2        float64
	hub, chord, chordMargin    float64
	kind                       int
	clipped, chordMode         bool
}

func newOffsetPartialCommon(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64) (offsetPartialCommon, geom.SphereCylinderCapMaterial, error) {
	return newOffsetPartialCommonMode(ctx, frame, R, r, offset, lower, upper, tolerance, false)
}

func newOffsetPartialCommonMode(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64, chord bool) (offsetPartialCommon, geom.SphereCylinderCapMaterial, error) {
	var s offsetPartialCommon
	var no geom.SphereCylinderCapMaterial
	if !finite(R) || !finite(r) || !finite(offset) || !finite(lower) || !finite(upper) ||
		R <= 0 || r <= 0 || offset <= 0 || lower >= upper || upper <= 0 || frame.ZAxis().Length() == 0 {
		return s, no, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sq := func(x *big.Rat) *big.Rat { return mul(x, x) }
	pad := mul(q(tolerance), big.NewRat(8, 1))
	far := add(q(offset), q(r))
	a2 := sub(sq(q(R)), sq(q(upper)))
	lo := add(q(math.Abs(lower)), pad)
	if sub(q(R), far).Cmp(pad) <= 0 || sub(q(upper), q(lower)).Cmp(pad) <= 0 || q(r).Cmp(pad) <= 0 ||
		add(sq(far), sq(lo)).Cmp(sq(q(R))) >= 0 || !chord && a2.Cmp(sq(add(q(offset), pad))) <= 0 {
		return s, no, geom.ErrConvergence
	}
	m, e := geom.ClassifySphereCylinderCapMaterial(ctx, geom.Vec3{}, R, geom.Vec3{X: offset}, geom.Vec3{Z: 1}, r, q(upper), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return s, no, e
	}
	if m.Kind != geom.SphereCylinderCapLens || m.Tangent {
		return s, no, geom.ErrConvergence
	}
	// The cap circle hits the cylinder when cos θ = (a²-d²-r²)/(2dr).
	cosine := new(big.Rat).Quo(sub(sub(a2, sq(q(offset))), sq(q(r))), mul(big.NewRat(2, 1), mul(q(offset), q(r))))
	c, _ := cosine.Float64()
	angle := math.Acos(c)
	if !finite(angle) || angle <= 0 || angle >= math.Pi || math.Sin(angle)*r <= 8*tolerance {
		return s, no, geom.ErrConvergence
	}
	f, _ := new(big.Rat).Quo(a2, sq(q(R))).Float64()
	if !finite(f) || f <= 0 {
		return s, no, geom.ErrUnrepresentable
	}
	s = offsetPartialCommon{frame: frame, R: R, r: r, offset: offset, lower: lower, upper: upper, angle: angle, endAngle: 2*math.Pi - angle, a2: f}
	if chord {
		hub := m.Events.Points[0].Base[0]
		// At the radical chord both squared radial margins are equal.
		chord2 := sub(a2, sq(hub))
		if chord2.Cmp(sq(pad)) <= 0 || sq(sub(q(r), pad)).Cmp(sq(sub(hub, q(offset)))) <= 0 ||
			a2.Cmp(sq(add(new(big.Rat).Abs(hub), pad))) <= 0 {
			return offsetPartialCommon{}, no, geom.ErrConvergence
		}
		s.angle, s.endAngle = math.Pi/2, 3*math.Pi/2
		s.hub, _ = new(big.Rat).Quo(hub, q(R)).Float64()
		s.chordMargin, _ = new(big.Rat).Quo(chord2, sq(q(R))).Float64()
		s.chord, s.chordMode = math.Sqrt(s.chordMargin), true
		if s.chord <= 0 || !finite(s.chord) {
			return offsetPartialCommon{}, no, geom.ErrUnrepresentable
		}
	}
	return s, m, nil
}

func (s offsetPartialCommon) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}

func (s offsetPartialCommon) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}

func (s offsetPartialCommon) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	switchAngle := u == s.angle || u == s.endAngle
	clipped := s.clipped
	if !switchAngle && clipped != (u < s.angle || u > s.endAngle) {
		return p, du, dv, geom.ErrDomain
	}
	if u == 2*math.Pi {
		u = 0
	}
	sn, cs := math.Sincos(u)
	if s.chordMode && switchAngle {
		cs, sn = 0, 1
		if u == s.endAngle {
			sn = -1
		}
	}
	d, r := s.offset/s.R, s.r/s.R
	if s.chordMode {
		return s.evaluateChord(sn, cs, v, switchAngle)
	}
	o := geom.Vec3{X: d}
	e, t := geom.Vec3{X: cs, Y: sn}, geom.Vec3{X: -sn, Y: cs}
	z := geom.Vec3{Z: 1}
	var a, a1 float64
	if (clipped || switchAngle) && (s.kind == partialCommonUpper || s.kind == partialCommonStrip) {
		m := s.a2 - d*d
		root := math.Sqrt(m + (d*cs)*(d*cs))
		a = root - d*cs
		if cs > 0 {
			a = m / (root + d*cs)
		}
		if switchAngle {
			a = r
		}
		a1 = d * sn * a / root
		if !finite(a) || !finite(a1) || a <= 0 || a > r || root <= 0 {
			return p, du, dv, geom.ErrConvergence
		}
	}
	switch s.kind {
	case partialCommonWall:
		lo, hi, dHi := s.lower/s.R, s.upper/s.R, 0.
		if clipped {
			b := o.Add(e.Scale(r))
			h := math.Sqrt((1 - b.Length()) * (1 + b.Length()))
			if !switchAngle {
				hi = h
			}
			dHi = d * r * sn / h
		}
		p = o.Add(e.Scale(r)).Add(z.Scale((1-v)*lo + v*hi))
		du = t.Scale(r).Add(z.Scale(v * dHi))
		dv = z.Scale(hi - lo)
	case partialCommonLower, partialCommonUpper:
		rho, drho, h := r, 0., s.lower/s.R
		if s.kind == partialCommonUpper {
			h = s.upper / s.R
			if clipped {
				rho, drho = a, a1
			}
		}
		p = o.Add(e.Scale(v * rho)).Add(z.Scale(h))
		du = t.Scale(v * rho).Add(e.Scale(v * drho))
		dv = e.Scale(rho)
	case partialCommonStrip:
		if !clipped && !switchAngle {
			return p, du, dv, geom.ErrDomain
		}
		rho := (1-v)*a + v*r
		rhoU, rhoV := (1-v)*a1, r-a
		b := o.Add(e.Scale(rho))
		bu, bv := t.Scale(rho).Add(e.Scale(rhoU)), e.Scale(rhoV)
		norm := b.Length()
		h := math.Sqrt((1 - norm) * (1 + norm))
		if v == 0 || switchAngle {
			h = s.upper / s.R
		}
		p = b.Add(z.Scale(h))
		du, dv = bu.Add(z.Scale(-b.Dot(bu)/h)), bv.Add(z.Scale(-b.Dot(bv)/h))
	default:
		return p, du, dv, geom.ErrInvalid
	}
	p = s.frame.ToWorld(p.Scale(s.R))
	du, dv = frameVector(s.frame, du.Scale(s.R)), frameVector(s.frame, dv.Scale(s.R))
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s offsetPartialCommon) evaluateChord(sn, cs, v float64, switchAngle bool) (p, du, dv geom.Vec3, err error) {
	d := s.offset / s.R
	o := geom.Vec3{X: s.hub}
	e, t := geom.Vec3{X: cs, Y: sn}, geom.Vec3{X: -sn, Y: cs}
	z := geom.Vec3{Z: 1}
	exit := func(center, margin float64) (rho, first float64) {
		x := center * cs
		root := math.Sqrt(margin + x*x)
		rho = x + root
		if x < 0 {
			rho = margin / (root - x)
		}
		if switchAngle {
			rho = s.chord
		}
		return rho, -center * sn * rho / root
	}
	c, dc := exit(d-s.hub, s.chordMargin)
	a, da := exit(-s.hub, s.chordMargin)
	lo, hi := s.lower/s.R, s.upper/s.R
	switch s.kind {
	case partialCommonWall:
		b, db := o.Add(e.Scale(c)), e.Scale(dc).Add(t.Scale(c))
		dh := 0.
		if s.clipped {
			n := b.Length()
			h := math.Sqrt((1 - n) * (1 + n))
			dh = -b.Dot(db) / h
			if !switchAngle {
				hi = h
			}
		}
		p, du, dv = b.Add(z.Scale((1-v)*lo+v*hi)), db.Add(z.Scale(v*dh)), z.Scale(hi-lo)
	case partialCommonLower, partialCommonUpper:
		rho, drho, h := c, dc, lo
		if s.kind == partialCommonUpper {
			h = hi
			if s.clipped {
				rho, drho = a, da
			}
		}
		p, du, dv = o.Add(e.Scale(v*rho)).Add(z.Scale(h)), t.Scale(v*rho).Add(e.Scale(v*drho)), e.Scale(rho)
	case partialCommonStrip:
		if !s.clipped {
			return p, du, dv, geom.ErrDomain
		}
		rho := (1-v)*a + v*c
		b := o.Add(e.Scale(rho))
		bu, bv := t.Scale(rho).Add(e.Scale((1-v)*da+v*dc)), e.Scale(c-a)
		n := b.Length()
		h := math.Sqrt((1 - n) * (1 + n))
		if v == 0 || switchAngle {
			h = hi
		}
		p, du, dv = b.Add(z.Scale(h)), bu.Add(z.Scale(-b.Dot(bu)/h)), bv.Add(z.Scale(-b.Dot(bv)/h))
	default:
		return p, du, dv, geom.ErrInvalid
	}
	p = s.frame.ToWorld(p.Scale(s.R))
	du, dv = frameVector(s.frame, du.Scale(s.R)), frameVector(s.frame, dv.Scale(s.R))
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() || c <= 0 || a <= 0 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

package parametric

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// SphereFiniteCylinderUnionResult contains native material components in
// increasing axial order. The ideal represented-affine volumes are exactly
// π times the corresponding VolumePiCoefficients.
type SphereFiniteCylinderUnionResult struct {
	Solids               []Solid
	VolumePiCoefficients []*big.Rat
}

// SphereFiniteCylinderUnion unites a sphere centered at frame.Origin with
// a coaxial circular cylinder at local axial heights [lower,upper]. The
// exterior keeps analytic spherical and cylindrical surfaces and any exposed
// disk or annular cap. All used curved junction heights and cap radii must
// be exactly representable. Separated inputs remain separate solids. A
// pole-only join would pinch the union and is rejected as nonmanifold.
func SphereFiniteCylinderUnion(ctx context.Context, frame geom.Frame, sphereRadius, cylinderRadius, lower, upper, tolerance float64) (SphereFiniteCylinderUnionResult, error) {
	var empty SphereFiniteCylinderUnionResult
	if ctx == nil || !finite(sphereRadius) || !finite(cylinderRadius) || !finite(lower) || !finite(upper) || !finite(tolerance) ||
		sphereRadius <= 0 || cylinderRadius <= 0 || lower >= upper || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if lower == sphereRadius || upper == -sphereRadius {
		return empty, geom.ErrConvergence
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	rs, cs := sq(q(sphereRadius)), sq(q(cylinderRadius))
	sqrt := func(r *big.Rat) (float64, error) {
		n, d := new(big.Int).Sqrt(r.Num()), new(big.Int).Sqrt(r.Denom())
		x := new(big.Rat).SetFrac(n, d)
		if sq(x).Cmp(r) != 0 {
			return 0, geom.ErrUnrepresentable
		}
		return geom.RepresentedExactScalar(x)
	}
	x, y, z := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	axes := [3][3]float64{{x.X, x.Y, x.Z}, {y.X, y.Y, y.Z}, {z.X, z.Y, z.Z}}
	det := new(big.Rat)
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, mul(q(axes[0][i]), sub(mul(q(axes[1][j]), q(axes[2][k])), mul(q(axes[1][k]), q(axes[2][j])))))
	}
	if det.Sign() <= 0 {
		return empty, geom.ErrDegenerate
	}
	sphere, e := geom.NewSphereSurface(frame, sphereRadius)
	if e != nil {
		return empty, e
	}
	cyl, e := geom.NewCylinderSurface(frame, cylinderRadius)
	if e != nil {
		return empty, e
	}
	at := func(z float64) *big.Rat { return sub(rs, sq(q(z))) }
	lo, hi := max(lower, -sphereRadius), min(upper, sphereRadius)
	cuts := []float64{-sphereRadius, sphereRadius, lower, upper}
	if lo < hi && cylinderRadius < sphereRadius {
		for _, sign := range []float64{-1, 1} {
			near, far := lo, hi
			if sign < 0 {
				near, far = hi, lo
			}
			if sign*far <= 0 || at(far).Cmp(cs) >= 0 || sign*near > 0 && at(near).Cmp(cs) <= 0 {
				continue
			}
			h, e := sqrt(sub(rs, cs))
			if e != nil {
				return empty, e
			}
			cuts = append(cuts, sign*h)
		}
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	const (
		absent = iota
		spherical
		cylindrical
	)
	type band struct {
		lo, hi float64
		kind   int
	}
	var bands []band
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		r2 := sub(rs, sq(mul(add(q(a), q(b)), big.NewRat(1, 2))))
		kind := absent
		if r2.Sign() > 0 {
			kind = spherical
		}
		if a >= lower && b <= upper && r2.Cmp(cs) <= 0 {
			kind = cylindrical
		}
		if n := len(bands); n > 0 && bands[n-1].kind == kind {
			bands[n-1].hi = b
		} else {
			bands = append(bands, band{a, b, kind})
		}
	}
	// A tangency at a pole was rejected above. Do not sew separate
	// components whose gap cannot be distinguished at the requested scale.
	pad := mul(q(tolerance), big.NewRat(8, 1))
	for _, b := range bands {
		if sub(q(b.hi), q(b.lo)).Cmp(pad) <= 0 {
			return empty, geom.ErrConvergence
		}
	}
	radiusAt := func(kind int, z float64) (float64, error) {
		switch kind {
		case spherical:
			return sqrt(at(z))
		case cylindrical:
			return cylinderRadius, nil
		default:
			return 0, nil
		}
	}
	primitive := func(z float64) *big.Rat {
		a := q(z)
		return sub(mul(rs, a), mul(big.NewRat(1, 3), mul(sq(a), a)))
	}
	out := SphereFiniteCylinderUnionResult{}
	for i := 0; i < len(bands); {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if bands[i].kind == absent {
			i++
			continue
		}
		first := i
		for i < len(bands) && bands[i].kind != absent {
			i++
		}
		part := bands[first:i]
		volume := new(big.Rat)
		for _, b := range part {
			if b.kind == cylindrical {
				volume.Add(volume, mul(cs, sub(q(b.hi), q(b.lo))))
			} else {
				volume.Add(volume, sub(primitive(b.hi), primitive(b.lo)))
			}
		}
		type shoulder struct {
			disk           disk
			inner, outer   float64
			normalPositive bool
		}
		var caps []shoulder
		for j := 0; j <= len(part); j++ {
			before, after, z := absent, absent, part[0].lo
			if j > 0 {
				before, z = part[j-1].kind, part[j-1].hi
			}
			if j < len(part) {
				after = part[j].kind
			}
			a, e := radiusAt(before, z)
			if e != nil {
				return empty, e
			}
			b, e := radiusAt(after, z)
			if e != nil {
				return empty, e
			}
			if a == b {
				continue
			}
			inner, outer := min(a, b), max(a, b)
			if sub(q(outer), q(inner)).Cmp(pad) <= 0 {
				return empty, geom.ErrConvergence
			}
			f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
			if e != nil {
				return empty, e
			}
			caps = append(caps, shoulder{disk{f}, inner, outer, a > b})
		}
		var faces []Face
		latitude := func(z float64) float64 { return math.Asin(z / sphereRadius) }
		for j := range 4 {
			u := Interval{float64(j) * math.Pi / 2, float64(j+1) * math.Pi / 2}
			for _, b := range part {
				if b.kind == cylindrical {
					faces = append(faces, Face{Surface: cyl, U: u, V: Interval{b.lo, b.hi}})
				} else {
					faces = append(faces, Face{Surface: sphere, U: u, V: Interval{latitude(b.lo), latitude(b.hi)}})
				}
			}
			for _, c := range caps {
				faces = append(faces, Face{Surface: c.disk, U: u, V: Interval{c.inner, c.outer}, Reversed: c.normalPositive})
			}
		}
		s, e := joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return empty, e
		}
		out.Solids = append(out.Solids, s)
		out.VolumePiCoefficients = append(out.VolumePiCoefficients, volume.Mul(volume, det))
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

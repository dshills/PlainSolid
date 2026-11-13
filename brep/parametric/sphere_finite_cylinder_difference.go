package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereFiniteCylinderDifferenceResult contains one or more disjoint native
// material bodies. Volumes are exact ideal represented-affine coefficients
// times π, indexed by component in increasing axial order.
type SphereFiniteCylinderDifferenceResult struct {
	Solids               []Solid
	VolumePiCoefficients []*big.Rat
}

// SphereFiniteCylinderDifference subtracts a finite coaxial cylinder from
// the sphere. It retains planar floors, cylindrical hole walls and spherical
// exteriors. An equatorial cylindrical cut whose radius covers the sphere at
// its relevant heights can separate the sphere into two caps. Disjoint and
// pole-only contacts leave the sphere unchanged. All used curved junction
// heights and cap radii must be exactly representable. A tool floor that
// pinches the shell at a transverse rim is rejected as nonmanifold.
func SphereFiniteCylinderDifference(ctx context.Context, frame geom.Frame, sphereRadius, cylinderRadius, lower, upper, tolerance float64) (SphereFiniteCylinderDifferenceResult, error) {
	var empty SphereFiniteCylinderDifferenceResult
	if ctx == nil || !finite(sphereRadius) || !finite(cylinderRadius) || !finite(lower) || !finite(upper) || !finite(tolerance) ||
		sphereRadius <= 0 || cylinderRadius <= 0 || lower >= upper || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	rs, cs := sq(q(sphereRadius)), sq(q(cylinderRadius))
	sqrt := func(r *big.Rat) (float64, error) {
		if r.Sign() < 0 {
			return 0, geom.ErrDomain
		}
		n, d := new(big.Int).Sqrt(r.Num()), new(big.Int).Sqrt(r.Denom())
		x := new(big.Rat).SetFrac(n, d)
		if sq(x).Cmp(r) != 0 {
			return 0, geom.ErrUnrepresentable
		}
		return geom.RepresentedExactScalar(x)
	}
	x, y, z := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	m := [3][3]float64{{x.X, x.Y, x.Z}, {y.X, y.Y, y.Z}, {z.X, z.Y, z.Z}}
	det := new(big.Rat)
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, mul(q(m[0][i]), sub(mul(q(m[1][j]), q(m[2][k])), mul(q(m[1][k]), q(m[2][j])))))
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
	active := lo < hi
	cuts := []float64{-sphereRadius, sphereRadius}
	if active {
		if lo > -sphereRadius {
			cuts = append(cuts, lo)
		}
		if hi < sphereRadius {
			cuts = append(cuts, hi)
		}
		if cylinderRadius < sphereRadius {
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
	}
	for i := 1; i < len(cuts); i++ {
		for j := i; j > 0 && cuts[j] < cuts[j-1]; j-- {
			cuts[j], cuts[j-1] = cuts[j-1], cuts[j]
		}
	}
	const (
		absent = iota
		full
		annular
	)
	type layer struct {
		lo, hi float64
		kind   int
	}
	var layers []layer
	pad := mul(q(tolerance), big.NewRat(8, 1))
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if sub(q(b), q(a)).Cmp(pad) <= 0 {
			return empty, geom.ErrConvergence
		}
		kind := full
		if active && a >= lo && b <= hi {
			mid := mul(add(q(a), q(b)), big.NewRat(1, 2))
			kind = absent
			if sub(rs, sq(mid)).Cmp(cs) > 0 {
				kind = annular
			}
		}
		layers = append(layers, layer{a, b, kind})
	}
	for i := 1; i < len(layers); i++ {
		before, after := layers[i-1], layers[i]
		if before.kind != absent && after.kind != absent && before.kind != after.kind && at(after.lo).Cmp(cs) == 0 {
			// The tool floor and wall and both sides of the sphere meet
			// on one circle. There is no manifold shell at this pinch.
			return empty, geom.ErrConvergence
		}
	}
	radiusAt := func(z float64) (float64, error) {
		if at(z).Cmp(cs) == 0 {
			return cylinderRadius, nil
		}
		return sqrt(at(z))
	}
	primitive := func(z float64) *big.Rat {
		a := q(z)
		return sub(mul(rs, a), mul(big.NewRat(1, 3), mul(sq(a), a)))
	}
	out := SphereFiniteCylinderDifferenceResult{}
	for i := 0; i < len(layers); {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if layers[i].kind == absent {
			i++
			continue
		}
		first := i
		for i < len(layers) && layers[i].kind != absent {
			i++
		}
		l := layers[first:i]
		volume := new(big.Rat)
		var faces []Face
		for j := range 4 {
			u := Interval{float64(j) * math.Pi / 2, float64(j+1) * math.Pi / 2}
			lat := func(z float64) float64 { return math.Asin(z / sphereRadius) }
			// All nonempty consecutive layers share the original sphere
			// exterior. Split only where another surface joins its rim.
			for k, b := range l {
				faces = append(faces, Face{Surface: sphere, U: u, V: Interval{lat(b.lo), lat(b.hi)}})
				if b.kind == annular {
					faces = append(faces, Face{Surface: cyl, U: u, V: Interval{b.lo, b.hi}, Reversed: true})
				}
				if k > 0 && l[k-1].kind != b.kind {
					z := b.lo
					// At a finite tool cap, only the removed center disk
					// changes occupancy. Its normal points into the void.
					f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
					if e != nil {
						return empty, e
					}
					faces = append(faces, Face{Surface: disk{f}, U: u, V: Interval{0, cylinderRadius}, Reversed: b.kind == annular})
				}
				if j == 0 {
					volume.Add(volume, sub(primitive(b.hi), primitive(b.lo)))
					if b.kind == annular {
						volume.Sub(volume, mul(cs, sub(q(b.hi), q(b.lo))))
					}
				}
			}
			for end, atZ := range []float64{l[0].lo, l[len(l)-1].hi} {
				if math.Abs(atZ) == sphereRadius {
					continue
				}
				outer, e := radiusAt(atZ)
				if e != nil {
					return empty, e
				}
				inner := 0.
				if end == 0 && l[0].kind == annular || end == 1 && l[len(l)-1].kind == annular {
					inner = cylinderRadius
				}
				if outer == inner {
					continue
				}
				if sub(q(outer), q(inner)).Cmp(pad) <= 0 {
					return empty, geom.ErrConvergence
				}
				f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: atZ}), frame.XAxis(), frame.ZAxis())
				if e != nil {
					return empty, e
				}
				faces = append(faces, Face{Surface: disk{f}, U: u, V: Interval{inner, outer}, Reversed: end == 1})
			}
		}
		s, e := joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return empty, e
		}
		out.Solids = append(out.Solids, s)
		out.VolumePiCoefficients = append(out.VolumePiCoefficients, volume.Mul(volume, det))
	}
	return out, nil
}

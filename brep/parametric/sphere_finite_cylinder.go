package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereFiniteCylinderCommonResult describes the intersection of a sphere
// with a finite coaxial cylinder. Face groups contain original result indices;
// an absent boundary group is nil. The exact ideal represented-affine volume
// is π times VolumePiCoefficient. Native seams use the supplied tolerance.
type SphereFiniteCylinderCommonResult struct {
	Solid               Solid
	SphericalFaces      []int
	CylindricalFaces    []int
	LowerCap, UpperCap  []int
	VolumePiCoefficient *big.Rat
}

// SphereFiniteCylinderCommon intersects a sphere of radius sphereRadius
// centered on frame.Origin with the cylinder of radius cylinderRadius and
// local axial bounds [lower, upper]. It retains analytic spherical zones,
// cylindrical side patches and planar clipping caps. Radius and height
// comparisons are exact on represented inputs. Curved rim heights and cap
// radii used by the result must be exactly representable. A disjoint or
// point-tangent intersection returns an empty regularized solid.
// A clip exactly at a transverse rim is supported with one shared cap rim.
func SphereFiniteCylinderCommon(ctx context.Context, frame geom.Frame, sphereRadius, cylinderRadius, lower, upper, tolerance float64) (SphereFiniteCylinderCommonResult, error) {
	var empty SphereFiniteCylinderCommonResult
	if ctx == nil || !finite(sphereRadius) || !finite(cylinderRadius) || !finite(lower) || !finite(upper) || !finite(tolerance) ||
		sphereRadius <= 0 || cylinderRadius <= 0 || lower >= upper || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if upper <= -sphereRadius || lower >= sphereRadius {
		empty.VolumePiCoefficient = new(big.Rat)
		return empty, nil
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	radiusSquared, cylinderSquared := sq(q(sphereRadius)), sq(q(cylinderRadius))
	sqrtExact := func(r *big.Rat) (float64, error) {
		if r.Sign() < 0 {
			return 0, geom.ErrInvalid
		}
		n, d := new(big.Int).Sqrt(r.Num()), new(big.Int).Sqrt(r.Denom())
		x := new(big.Rat).SetFrac(n, d)
		if sq(x).Cmp(r) != 0 {
			return 0, geom.ErrUnrepresentable
		}
		return geom.RepresentedExactScalar(x)
	}
	lo, hi := max(lower, -sphereRadius), min(upper, sphereRadius)
	pad := mul(q(tolerance), big.NewRat(8, 1))
	clear := func(a, b float64) bool { return sub(q(a), q(b)).Cmp(pad) > 0 }
	if !clear(hi, lo) || !clear(min(sphereRadius, cylinderRadius), 0) {
		return empty, geom.ErrConvergence
	}
	// Classify in radius-squared space first. An irrational intersection
	// height is irrelevant if the selected band never crosses that rim.
	at := func(z float64) int {
		return sub(radiusSquared, sq(q(z))).Cmp(cylinderSquared)
	}
	cuts := []float64{lo, hi}
	if cylinderRadius < sphereRadius {
		for _, sign := range []float64{-1, 1} {
			near, far := lo, hi
			if sign < 0 {
				near, far = hi, lo
			}
			if sign*far <= 0 || at(far) >= 0 || sign*near > 0 && at(near) <= 0 {
				continue
			}
			h, e := sqrtExact(sub(radiusSquared, cylinderSquared))
			if e != nil {
				return empty, e
			}
			cuts = append(cuts, sign*h)
		}
	}
	if len(cuts) > 2 {
		// There can be at most two transverse cuts.
		for i := 1; i < len(cuts); i++ {
			for j := i; j > 0 && cuts[j] < cuts[j-1]; j-- {
				cuts[j], cuts[j-1] = cuts[j-1], cuts[j]
			}
		}
	}
	type band struct {
		a, b     float64
		cylinder bool
	}
	var bands []band
	volume := new(big.Rat)
	primitive := func(z *big.Rat) *big.Rat {
		return sub(mul(radiusSquared, z), mul(big.NewRat(1, 3), mul(z, sq(z))))
	}
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if !clear(b, a) {
			return empty, geom.ErrConvergence
		}
		mid := mul(add(q(a), q(b)), big.NewRat(1, 2))
		cyl := sub(radiusSquared, sq(mid)).Cmp(cylinderSquared) > 0
		bands = append(bands, band{a, b, cyl})
		if cyl {
			volume.Add(volume, mul(cylinderSquared, sub(q(b), q(a))))
		} else {
			volume.Add(volume, sub(primitive(q(b)), primitive(q(a))))
		}
	}
	sphere, e := geom.NewSphereSurface(frame, sphereRadius)
	if e != nil {
		return empty, e
	}
	cylinder, e := geom.NewCylinderSurface(frame, cylinderRadius)
	if e != nil {
		return empty, e
	}
	var caps [2]geom.Surface
	var capRadii [2]float64
	for i, z := range []float64{lo, hi} {
		if z == -sphereRadius || z == sphereRadius {
			continue
		}
		capRadii[i] = cylinderRadius
		if at(z) < 0 {
			capRadii[i], e = sqrtExact(sub(radiusSquared, sq(q(z))))
			if e != nil {
				return empty, e
			}
		}
		if !clear(capRadii[i], 0) {
			return empty, geom.ErrConvergence
		}
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return empty, e
		}
		caps[i] = disk{f}
	}
	out := SphereFiniteCylinderCommonResult{}
	var faces []Face
	for j := range 4 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		u := Interval{float64(j) * math.Pi / 2, float64(j+1) * math.Pi / 2}
		for _, b := range bands {
			if b.cylinder {
				out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
				faces = append(faces, Face{Surface: cylinder, U: u, V: Interval{b.a, b.b}})
			} else {
				latitude := func(z float64) float64 { return math.Asin(z / sphereRadius) }
				out.SphericalFaces = append(out.SphericalFaces, len(faces))
				faces = append(faces, Face{Surface: sphere, U: u, V: Interval{latitude(b.a), latitude(b.b)}})
			}
		}
		for i, cap := range caps {
			if cap == nil {
				continue
			}
			if i == 0 {
				out.LowerCap = append(out.LowerCap, len(faces))
			} else {
				out.UpperCap = append(out.UpperCap, len(faces))
			}
			faces = append(faces, Face{Surface: cap, U: u, V: Interval{0, capRadii[i]}, Reversed: i == 1})
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	x, y, z := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	axes := [3][3]float64{{x.X, x.Y, x.Z}, {y.X, y.Y, y.Z}, {z.X, z.Y, z.Z}}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, mul(q(axes[0][i]), sub(mul(q(axes[1][j]), q(axes[2][k])), mul(q(axes[1][k]), q(axes[2][j])))))
	}
	if det.Sign() <= 0 {
		return empty, geom.ErrDegenerate
	}
	out.VolumePiCoefficient = volume.Mul(volume, det)
	return out, nil
}

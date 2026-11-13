package parametric

import (
	"context"
	"math"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// AlgebraicCommonRim records exact axial/radial provenance. A nil root means
// the associated interval is an exact represented singleton. SeamSquaredBound
// bounds mathematical sphere and circle charts in the original affine frame;
// native independently normalized cap frames/evaluator rounding retain the
// supplied sewing tolerance, not an exact chart-identity claim.
type AlgebraicCommonRim struct {
	HeightRoot, RadiusRoot     *geom.ExactAlgebraicRoot
	Height, Radius             geom.ExactParameterInterval
	NativeHeight, NativeRadius float64
	Latitude                   float64
	SeamSquaredBound           *big.Rat
}
type SphereFiniteCylinderAlgebraicResult struct {
	Solid                                                Solid
	Rims                                                 []AlgebraicCommonRim
	SphericalFaces, CylindricalFaces, LowerCap, UpperCap []int
	VolumePiInterval                                     [2]*big.Rat
}

// SphereFiniteCylinderCommonAlgebraic handles finite axial cap planes and
// uncut sphere poles. Sphere-controlled cap radii and internal crossing
// heights may be irrational. Root isolation obeys options; each rim additionally
// uses a fixed 49-term rational Taylor enclosure. Ambiguous cap/crossing order,
// tangency and inadequate sewing bounds fail atomically. The volume
// interval bounds ideal represented-affine material, not numerical quadrature.
func SphereFiniteCylinderCommonAlgebraic(ctx context.Context, frame geom.Frame, R, r, lower, upper, tolerance float64, o geom.ExactCurvePlaneEventOptions) (SphereFiniteCylinderAlgebraicResult, error) {
	fail := func(e error) (SphereFiniteCylinderAlgebraicResult, error) {
		return SphereFiniteCylinderAlgebraicResult{}, e
	}
	if ctx == nil || !finite(R) || !finite(r) || !finite(lower) || !finite(upper) || !finite(tolerance) || R <= 0 || r <= 0 || lower >= upper || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if lower >= R || upper <= -R {
		return SphereFiniteCylinderAlgebraicResult{VolumePiInterval: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, nil
	}
	lower, upper = max(lower, -R), min(upper, R)
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	div := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	R2, r2 := sq(q(R)), sq(q(r))
	pad := mul(q(tolerance), big.NewRat(8, 1))
	for _, gap := range []*big.Rat{sub(q(upper), q(lower)), q(r)} {
		if gap.Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
	}
	axes := [3]geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}
	a := [3][3]float64{}
	norm, det := new(big.Rat), new(big.Rat)
	for i, p := range axes {
		a[i] = [3]float64{p.X, p.Y, p.Z}
		for _, v := range a[i] {
			if !finite(v) {
				return fail(geom.ErrInvalid)
			}
			norm.Add(norm, sq(q(v)))
		}
	}
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, mul(q(a[0][i]), sub(mul(q(a[1][j]), q(a[2][k])), mul(q(a[1][k]), q(a[2][j])))))
	}
	if det.Sign() <= 0 {
		return fail(geom.ErrInvalid)
	}
	rootOf := func(value *big.Rat) (*geom.ExactAlgebraicRoot, geom.ExactParameterInterval, error) {
		root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(value).RatString(), "0", "1"}, Lower: "0", Upper: q(R).RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o})
		if e != nil {
			return nil, geom.ExactParameterInterval{}, e
		}
		enc, e := root.Enclose(ctx, div(q(tolerance), big.NewRat(128, 1)), o)
		if e != nil {
			return nil, geom.ExactParameterInterval{}, e
		}
		if !enc.Converged {
			return nil, geom.ExactParameterInterval{}, geom.ErrConvergence
		}
		return root, enc.Interval, nil
	}
	singleton := func(v float64) geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: q(v), Upper: q(v)}
	}
	rounded := func(v geom.ExactParameterInterval) float64 {
		x, _ := div(add(v.Lower, v.Upper), big.NewRat(2, 1)).Float64()
		return x
	}
	rims := []AlgebraicCommonRim{{Height: singleton(lower), NativeHeight: lower}, {Height: singleton(upper), NativeHeight: upper}}
	if r < R {
		root, iv, e := rootOf(sub(R2, r2))
		if e != nil {
			return fail(e)
		}
		for _, sign := range []int{-1, 1} {
			v := iv
			rr := root
			if sign < 0 {
				v = geom.ExactParameterInterval{Lower: new(big.Rat).Neg(iv.Upper), Upper: new(big.Rat).Neg(iv.Lower)}
				rr, _, e = root.AffineMap(ctx, new(big.Rat), big.NewRat(-1, 1), o)
				if e != nil {
					return fail(e)
				}
			}
			for _, endpoint := range []float64{lower, upper} {
				if sub(q(endpoint), v.Upper).Cmp(pad) <= 0 && sub(v.Lower, q(endpoint)).Cmp(pad) <= 0 {
					return fail(geom.ErrConvergence)
				}
			}
			if v.Lower.Cmp(q(lower)) > 0 && v.Upper.Cmp(q(upper)) < 0 {
				rims = append(rims, AlgebraicCommonRim{HeightRoot: rr, Height: v, NativeHeight: rounded(v), Radius: singleton(r), NativeRadius: r})
			}
		}
	}
	sort.Slice(rims, func(i, j int) bool { return rims[i].NativeHeight < rims[j].NativeHeight })
	for i := range rims {
		rim := &rims[i]
		if math.Abs(rim.NativeHeight) == R && rim.HeightRoot == nil {
			rim.Radius = singleton(0)
			rim.Latitude = math.Copysign(math.Pi/2, rim.NativeHeight)
			rim.SeamSquaredBound = new(big.Rat)
			continue
		}
		if rim.HeightRoot == nil {
			rad2 := sub(R2, sq(q(rim.NativeHeight)))
			if rad2.Cmp(r2) < 0 {
				root, iv, e := rootOf(rad2)
				if e != nil {
					return fail(e)
				}
				rim.RadiusRoot = root
				rim.Radius = iv
				rim.NativeRadius = rounded(iv)
			} else {
				rim.Radius = singleton(r)
				rim.NativeRadius = r
			}
		}
		if rim.Radius.Lower.Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
		rim.Latitude = math.Atan2(rim.NativeHeight, rim.NativeRadius)
		// Only sphere-active rims need a spherical seam proof. Internal crossing
		// roots and cap-radius roots identify these; exact equality also qualifies.
		active := rim.HeightRoot != nil || rim.RadiusRoot != nil || sub(R2, sq(q(rim.NativeHeight))).Cmp(r2) == 0
		rim.SeamSquaredBound = new(big.Rat)
		if !active {
			continue
		}
		x := q(rim.Latitude)
		if math.Abs(rim.Latitude) >= 2 {
			return fail(geom.ErrConvergence)
		}
		sin, cos, term := new(big.Rat), new(big.Rat), big.NewRat(1, 1)
		for n := 0; n <= 48; n++ {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			switch n % 4 {
			case 0:
				cos.Add(cos, term)
			case 1:
				sin.Add(sin, term)
			case 2:
				cos.Sub(cos, term)
			case 3:
				sin.Sub(sin, term)
			}
			term = div(mul(term, x), big.NewRat(int64(n+1), 1))
		}
		remainder := mul(q(R), new(big.Rat).Abs(term))
		dx := add(new(big.Rat).Abs(sub(mul(q(R), cos), q(rim.NativeRadius))), remainder)
		dz := add(new(big.Rat).Abs(sub(mul(q(R), sin), q(rim.NativeHeight))), remainder)
		rim.SeamSquaredBound = mul(norm, add(sq(dx), sq(dz)))
		if rim.SeamSquaredBound.Cmp(div(sq(q(tolerance)), big.NewRat(16, 1))) > 0 {
			return fail(geom.ErrConvergence)
		}
	}
	sphere, e := geom.NewSphereSurface(frame, R)
	if e != nil {
		return fail(e)
	}
	cylinder, e := geom.NewCylinderSurface(frame, r)
	if e != nil {
		return fail(e)
	}
	primitive := func(z *big.Rat) *big.Rat { return sub(mul(R2, z), div(mul(z, sq(z)), big.NewRat(3, 1))) }
	cylinderBand := make([]bool, len(rims)-1)
	volume, errorBound := new(big.Rat), new(big.Rat)
	for i := 0; i+1 < len(rims); i++ {
		a, b := rims[i], rims[i+1]
		if sub(b.Height.Lower, a.Height.Upper).Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
		mid := div(add(a.Height.Upper, b.Height.Lower), big.NewRat(2, 1))
		cyl := sub(R2, sq(mid)).Cmp(r2) > 0
		cylinderBand[i] = cyl
		lo, hi := q(a.NativeHeight), q(b.NativeHeight)
		if cyl {
			volume.Add(volume, mul(r2, sub(hi, lo)))
		} else {
			volume.Add(volume, sub(primitive(hi), primitive(lo)))
		}
		// On the bracketed sphere interval |(R²z-z³/3)'|<=2R²;
		// cylinder derivative=r². Summed endpoint errors conservatively cover
		// every band, independently of cancellation at the true crossing.
		for _, rim := range []AlgebraicCommonRim{a, b} {
			err := new(big.Rat).Abs(sub(q(rim.NativeHeight), rim.Height.Lower))
			other := new(big.Rat).Abs(sub(q(rim.NativeHeight), rim.Height.Upper))
			if other.Cmp(err) > 0 {
				err = other
			}
			errorBound.Add(errorBound, mul(add(mul(big.NewRat(2, 1), R2), r2), err))
		}
	}
	result := SphereFiniteCylinderAlgebraicResult{Rims: rims}
	var faces []Face
	for j := 0; j < 4; j++ {
		u := Interval{float64(j) * math.Pi / 2, float64(j+1) * math.Pi / 2}
		for i, cyl := range cylinderBand {
			a, b := rims[i], rims[i+1]
			if cyl {
				result.CylindricalFaces = append(result.CylindricalFaces, len(faces))
				faces = append(faces, Face{Surface: cylinder, U: u, V: Interval{a.NativeHeight, b.NativeHeight}})
			} else {
				result.SphericalFaces = append(result.SphericalFaces, len(faces))
				faces = append(faces, Face{Surface: sphere, U: u, V: Interval{a.Latitude, b.Latitude}})
			}
		}
		for cap, index := range []int{0, len(rims) - 1} {
			rim := rims[index]
			if rim.NativeRadius == 0 {
				continue
			}
			f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: rim.NativeHeight}), frame.XAxis(), frame.ZAxis())
			if e != nil {
				return fail(e)
			}
			if cap == 0 {
				result.LowerCap = append(result.LowerCap, len(faces))
			} else {
				result.UpperCap = append(result.UpperCap, len(faces))
			}
			faces = append(faces, Face{Surface: disk{f}, U: u, V: Interval{0, rim.NativeRadius}, Reversed: cap == 1})
		}
	}
	result.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	result.VolumePiInterval = [2]*big.Rat{mul(det, sub(volume, errorBound)), mul(det, add(volume, errorBound))}
	if result.VolumePiInterval[0].Sign() <= 0 {
		return fail(geom.ErrConvergence)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return result, nil
}

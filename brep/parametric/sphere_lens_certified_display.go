package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// SphericalLensDisplayProof binds the exact original rim to an analytical
// display shell. ModelDeviation bounds the distance between corresponding
// real-arithmetic chart points under normalized patch parameters. It does not
// include evaluation roundoff or mesh deflection. All rationals are caller-owned.
type SphericalLensDisplayProof struct {
	OriginalCenter           [3]*big.Rat
	OriginalRadiusSquared    *big.Rat
	OriginalLatitudes        [2]geom.ExactParameterInterval
	ModelDeviation           *big.Rat
	SourceFaces, DisplayCaps [2]int
	DisplayBand              int
}

// CertifiedSphericalLensDilation owns the ideal source material and a separate
// display shell whose chart approximation has a proved bound. Use the source's
// Classify/Clip/MeasureDilatedSphericalLens methods for exact queries.
type CertifiedSphericalLensDilation struct {
	source   SpherePairSet
	display  Solid
	proof    SphericalLensDisplayProof
	distance float64
	bits     int
}

// ProveSphericalLensDilationDisplay builds a bounded analytical display of a
// transverse spherical lens dilation. It preserves a nonrepresentable original
// rim as an exact rational plane center and squared radius; display charts may
// round radius, center and joins only within the proved allowance. The rounded
// charts are not used as authority for exact material queries. The display
// model deviation may consume at most one quarter of the caller's tolerance.
func (s SpherePairSet) ProveSphericalLensDilationDisplay(ctx context.Context, distance, tolerance float64, o geom.SphereLensMeasureOptions) (CertifiedSphericalLensDilation, error) {
	var zero CertifiedSphericalLensDilation
	if ctx == nil || !s.valid || !finite(distance) || distance <= 0 || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	balls, owners, err := s.sphereOffsetOwners(Intersection, false)
	if err != nil {
		return zero, err
	}
	bound, err := s.query.EncloseTransverseLensRim(ctx, o)
	if err != nil {
		return zero, err
	}
	q := lensDisplayRat
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	var center [3]float64
	centerError := new(big.Rat)
	for i, exact := range bound.Rim.Center {
		var delta *big.Rat
		center[i], delta, err = lensDisplayRound(geom.ExactParameterInterval{Lower: exact, Upper: exact})
		if err != nil {
			return zero, err
		}
		centerError.Add(centerError, delta) // L1 conservatively bounds Euclidean error.
	}
	rim, rimError, err := lensDisplayRound(bound.Radius)
	if err != nil || rim <= 0 {
		return zero, geom.ErrUnrepresentable
	}
	var latitude [2]float64
	angleError := new(big.Rat)
	for i, exact := range bound.Latitude {
		var delta *big.Rat
		latitude[i], delta, err = lensDisplayRound(exact)
		if err != nil {
			return zero, err
		}
		angleError = lensDisplayMax(angleError, delta)
	}
	if latitude[1] <= -math.Pi/2 || latitude[0] >= math.Pi/2 || latitude[0] <= latitude[1] {
		return zero, geom.ErrUnrepresentable
	}
	piError := lensDisplayError(q(math.Pi), bound.Pi)
	longitudeError := mul(big.NewRat(2, 1), piError)
	latitudeError := lensDisplayMax(angleError, mul(big.NewRat(1, 2), piError))
	// The outer torus's U and V derivative norms are bounded analytically by
	// h+d and d. Interpolation within the two endpoint latitude enclosures
	// keeps the same uniform angular error throughout the original band.
	d := q(distance)
	deviation := add(add(centerError, rimError), add(mul(add(bound.Radius.Upper, d), longitudeError), mul(d, angleError)))
	var caps [2]geom.SphereSurface
	var firstFrame geom.Frame
	for i, id := range owners {
		f, _ := s.solid.faces[id].Surface.(geom.SphereSurface).Parameters()
		if i == 0 {
			firstFrame = f
		}
		exact := add(q(balls[i].Radius), d)
		radius, radiusError, e := lensDisplayRound(geom.ExactParameterInterval{Lower: exact, Upper: exact})
		if e != nil {
			return zero, e
		}
		caps[i], e = geom.NewSphereSurface(f, radius)
		if e != nil {
			return zero, e
		}
		// For a sphere of the exact expanded radius both partial speeds
		// are bounded by that radius, including at the two poles.
		deviation = lensDisplayMax(deviation, add(radiusError, mul(exact, add(longitudeError, latitudeError))))
	}
	if deviation.Cmp(mul(q(tolerance), big.NewRat(1, 4))) > 0 {
		return zero, geom.ErrUnrepresentable
	}
	f, err := geom.NewFrame(geom.Vec3{X: center[0], Y: center[1], Z: center[2]}, firstFrame.XAxis(), firstFrame.ZAxis())
	if err != nil {
		return zero, err
	}
	rolling, err := geom.NewOuterTorusSurface(f, rim, distance)
	if err != nil {
		return zero, err
	}
	display, err := stitchSphericalLensDilation(caps, rolling, latitude, tolerance)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	p := SphericalLensDisplayProof{bound.Rim.Center, bound.Rim.RadiusSquared, bound.Latitude, deviation, owners, [2]int{2, 0}, 1}
	return CertifiedSphericalLensDilation{s, display, p, distance, o.PrecisionBits}, nil
}

// DisplaySolid returns the immutable bounded chart representation.
func (r CertifiedSphericalLensDilation) DisplaySolid() Solid { return r.display }

// Source returns the immutable original lens and the requested displacement.
func (r CertifiedSphericalLensDilation) Source() (SpherePairSet, float64) {
	return r.source, r.distance
}

func (r CertifiedSphericalLensDilation) Proof() SphericalLensDisplayProof {
	if r.proof.ModelDeviation == nil {
		return SphericalLensDisplayProof{}
	}
	p := r.proof
	clone := func(v *big.Rat) *big.Rat { return new(big.Rat).Set(v) }
	for i, x := range p.OriginalCenter {
		p.OriginalCenter[i] = clone(x)
	}
	p.OriginalRadiusSquared, p.ModelDeviation = clone(p.OriginalRadiusSquared), clone(p.ModelDeviation)
	for i, v := range p.OriginalLatitudes {
		p.OriginalLatitudes[i] = geom.ExactParameterInterval{Lower: clone(v.Lower), Upper: clone(v.Upper)}
	}
	return p
}

// Tessellate reserves the proved model deviation from Deflection, then runs
// the analytical display mesh bound. Angular tolerance is not supported by
// this combined bound. As with Solid's analytical tessellation, numerical
// evaluation roundoff is separate from the model and interpolation bounds.
func (r CertifiedSphericalLensDilation) Tessellate(ctx context.Context, o TessellationOptions) (mesh.Mesh, error) {
	if r.proof.ModelDeviation == nil || !finite(o.Deflection) || o.Deflection <= 0 || o.Angle != 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	remaining := new(big.Rat).Sub(lensDisplayRat(o.Deflection), r.proof.ModelDeviation)
	if remaining.Sign() <= 0 {
		return mesh.Mesh{}, ErrBudget
	}
	var err error
	o.Deflection, err = lensDisplayLower(remaining)
	if err != nil {
		return mesh.Mesh{}, err
	}
	o.AnalyticBound = true
	return r.display.Tessellate(ctx, o)
}

func lensDisplayRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

func lensDisplayMax(x, y *big.Rat) *big.Rat {
	if x.Cmp(y) >= 0 {
		return x
	}
	return y
}

func lensDisplayError(x *big.Rat, bound geom.ExactParameterInterval) *big.Rat {
	lo := new(big.Rat).Abs(new(big.Rat).Sub(x, bound.Lower))
	hi := new(big.Rat).Abs(new(big.Rat).Sub(x, bound.Upper))
	return lensDisplayMax(lo, hi)
}

func lensDisplayRound(bound geom.ExactParameterInterval) (float64, *big.Rat, error) {
	mid := new(big.Rat).Mul(new(big.Rat).Add(bound.Lower, bound.Upper), big.NewRat(1, 2))
	v, _ := mid.Float64()
	if !finite(v) {
		return 0, nil, geom.ErrUnrepresentable
	}
	return v, lensDisplayError(lensDisplayRat(v), bound), nil
}

func lensDisplayLower(x *big.Rat) (float64, error) {
	v, _ := x.Float64()
	if !finite(v) || v <= 0 {
		return 0, ErrBudget
	}
	if lensDisplayRat(v).Cmp(x) > 0 {
		v = math.Nextafter(v, 0)
	}
	if v <= 0 {
		return 0, ErrBudget
	}
	return v, nil
}

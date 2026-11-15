package geom

import (
	"context"
	"math/big"
)

type SphereLensMeasureOptions struct {
	// PrecisionBits sets an absolute width of 2^-PrecisionBits for Volume
	// and Area in model units. It defaults to 40 and must be in [8,128].
	PrecisionBits int
	Arithmetic    SurfacePatchOptions
}

// SphereLensParallelMeasures contains closed rational enclosures of the exact
// original lens and its convex dilation. IntegratedMeanCurvature is the
// coefficient of distance squared in the volume polynomial. NormalAngle is
// the angle between the two source normals at the common rim, in radians.
type SphereLensParallelMeasures struct {
	Volume, Area, OriginalVolume, OriginalArea      ExactParameterInterval
	IntegratedMeanCurvature, RimRadius, NormalAngle ExactParameterInterval
	Operations                                      int
}

// MeasureDilatedLensRational encloses area and volume without tessellation or
// conversion of the original rim to a native floating support. The heights of
// the two original spherical caps and the angle between their rim normals
// determine the convex parallel-body coefficients. Radical bounds and exact
// alternating arctangent series supply independent rational enclosures for the
// rim, angle, and pi. Arbitrary represented center directions are supported.
func (s SpherePairMaterial) MeasureDilatedLensRational(ctx context.Context, displacement *big.Rat, o SphereLensMeasureOptions) (SphereLensParallelMeasures, error) {
	var zero SphereLensParallelMeasures
	if ctx == nil || s.radius2[0] == nil || displacement == nil || displacement.Sign() < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return zero, ErrDomain
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 40
	}
	if o.PrecisionBits < 8 || o.PrecisionBits > 128 {
		return zero, ErrInvalid
	}
	limits, err := patchOptions(o.Arithmetic)
	if err != nil {
		return zero, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	a.check(displacement)
	for i, center := range s.center {
		for _, coordinate := range center {
			a.check(coordinate)
		}
		a.check(s.radius2[i])
	}
	one, two, three := big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1)
	radii := [2]*big.Rat{a.sqrt(s.radius2[0]), a.sqrt(s.radius2[1])}
	d2 := a.distance(s.center[0], s.center[1])
	f := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), d2), a.mul(two, d2))
	h2 := a.sub(s.radius2[0], a.mul(a.mul(f, f), d2))
	cosine := a.div(a.sub(a.add(s.radius2[0], s.radius2[1]), d2), a.mul(two, a.mul(radii[0], radii[1])))
	abs := new(big.Rat).Abs(cosine)
	ratio := a.div(a.sub(one, abs), a.add(one, abs))
	delta2 := a.mul(displacement, displacement)
	delta3 := a.mul(delta2, displacement)
	if a.err != nil {
		return zero, a.err
	}
	if h2.Sign() <= 0 || ratio.Sign() <= 0 {
		return zero, ErrConvergence
	}
	target := lensMeasurePowerOfTwo(o.PrecisionBits)
	n := lensMeasureIntervals{a}
	for bits := o.PrecisionBits + 12; bits <= 512; bits = min(513, bits+32) {
		axisLength, rim := n.root(d2, bits), n.root(h2, bits)
		atanArg := n.root(ratio, bits)
		p := lensMeasurePi(ctx, lensMeasurePowerOfTwo(bits+12), a)
		angle := n.scale(n.angle(atanArg, p, bits), two)
		if cosine.Sign() < 0 {
			angle = n.sub(p, angle)
		}
		if a.err != nil {
			return zero, a.err
		}
		area0, volume0, smooth := n.point(new(big.Rat)), n.point(new(big.Rat)), n.point(new(big.Rat))
		for i, fraction := range []*big.Rat{f, a.sub(one, f)} {
			cap := n.sub(n.point(radii[i]), n.scale(axisLength, fraction))
			if cap.lo.Sign() <= 0 {
				// Near tangency needs more bits to separate a positive cap.
				area0 = n.point(new(big.Rat))
				break
			}
			area0 = n.add(area0, n.scale(cap, a.mul(two, radii[i])))
			smooth = n.add(smooth, n.scale(cap, two))
			third := n.scale(cap, new(big.Rat).Inv(three))
			volume0 = n.add(volume0, n.mul(n.mul(cap, cap), n.sub(n.point(radii[i]), third)))
		}
		if a.err != nil {
			return zero, a.err
		}
		if area0.lo.Sign() <= 0 || angle.lo.Sign() <= 0 || volume0.lo.Sign() <= 0 {
			continue
		}
		// Work with coefficients divided by pi until the last multiplication.
		// Smooth spherical cap curvature contributes 2*(Ha+Hb); the corner
		// contributes one half its circumference times the normal angle.
		curvature := n.add(smooth, n.mul(rim, angle))
		volume := n.add(n.add(volume0, n.scale(area0, displacement)), n.add(n.scale(curvature, delta2), n.point(a.mul(big.NewRat(4, 3), delta3))))
		area := n.add(n.add(area0, n.scale(curvature, a.mul(two, displacement))), n.point(a.mul(big.NewRat(4, 1), delta2)))
		volume, area = n.mul(volume, p), n.mul(area, p)
		originalVolume, originalArea, mean := n.mul(volume0, p), n.mul(area0, p), n.mul(curvature, p)
		volumeWidth, areaWidth := a.sub(volume.hi, volume.lo), a.sub(area.hi, area.lo)
		if a.err != nil {
			return zero, a.err
		}
		if volumeWidth.Cmp(target) > 0 || areaWidth.Cmp(target) > 0 {
			continue
		}
		interval := func(q exactInterval) ExactParameterInterval { return ExactParameterInterval{q.lo, q.hi} }
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		return SphereLensParallelMeasures{interval(volume), interval(area), interval(originalVolume), interval(originalArea), interval(mean), interval(rim), interval(angle), b.operations}, nil
	}
	return zero, ErrConvergence
}

func lensMeasurePowerOfTwo(bits int) *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(bits)))
}

type lensMeasureIntervals struct{ a *sphereOverlapArithmetic }

func (n lensMeasureIntervals) point(x *big.Rat) exactInterval { return exactInterval{x, x} }
func (n lensMeasureIntervals) add(x, y exactInterval) exactInterval {
	return exactInterval{n.a.add(x.lo, y.lo), n.a.add(x.hi, y.hi)}
}
func (n lensMeasureIntervals) sub(x, y exactInterval) exactInterval {
	return exactInterval{n.a.sub(x.lo, y.hi), n.a.sub(x.hi, y.lo)}
}
func (n lensMeasureIntervals) mul(x, y exactInterval) exactInterval {
	if n.a.err != nil {
		return n.point(new(big.Rat))
	}
	p, err := intervalMultiply(x, y, n.a.b)
	n.a.err = err
	if err != nil {
		return n.point(new(big.Rat))
	}
	return p
}
func (n lensMeasureIntervals) scale(x exactInterval, y *big.Rat) exactInterval {
	return n.mul(x, n.point(y))
}
func (n lensMeasureIntervals) root(x *big.Rat, bits int) exactInterval {
	if n.a.err != nil {
		return n.point(new(big.Rat))
	}
	lo, hi, err := rationalSqrtBounds(x, bits, n.a.b)
	n.a.err = err
	if err != nil {
		return n.point(new(big.Rat))
	}
	return exactInterval{lo, hi}
}

func lensMeasurePi(ctx context.Context, tolerance *big.Rat, a *sphereOverlapArithmetic) exactInterval {
	n := lensMeasureIntervals{a}
	f := func(den int64) exactInterval {
		if a.err != nil {
			return n.point(new(big.Rat))
		}
		q, err := helicalAtanSmall(ctx, big.NewRat(1, den), tolerance, 4096, a.b)
		a.err = err
		if err != nil {
			return n.point(new(big.Rat))
		}
		return exactInterval{q[0], q[1]}
	}
	return n.sub(n.scale(f(5), big.NewRat(16, 1)), n.scale(f(239), big.NewRat(4, 1)))
}

func (n lensMeasureIntervals) angle(x, pi exactInterval, bits int) exactInterval {
	one := big.NewRat(1, 1)
	f := func(t *big.Rat) exactInterval {
		if n.a.err != nil {
			return n.point(new(big.Rat))
		}
		base := n.point(new(big.Rat))
		if t.Cmp(big.NewRat(1, 2)) > 0 {
			// atan(t) = pi/4 + atan((t-1)/(1+t)); the new magnitude is <=1/3.
			base = n.scale(pi, big.NewRat(1, 4))
			t = n.a.div(n.a.sub(t, one), n.a.add(one, t))
		}
		if n.a.err != nil {
			return n.point(new(big.Rat))
		}
		y, err := helicalAtanSmall(n.a.ctx, t, lensMeasurePowerOfTwo(bits+12), 4096, n.a.b)
		n.a.err = err
		if err != nil {
			return n.point(new(big.Rat))
		}
		return n.add(base, exactInterval{y[0], y[1]})
	}
	return exactInterval{f(x.lo).lo, f(x.hi).hi}
}

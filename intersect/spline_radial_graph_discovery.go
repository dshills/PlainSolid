package intersect

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
)

// DiscoverRadialGraphPairTubes recognizes an exact concentric quartic numerator
// on a unit chart and proposes adaptively certified original-support tubes.
// It does not establish global coverage; run CertifySplinePairBranches. The
// bounded first version requires representable 3/5 and 4/5 axis-switch values.
func DiscoverRadialGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
	return discoverConcentricGraphPairTubes(ctx, a, b, p, o, false)
}

// DiscoverConcentricEllipseGraphPairTubes recognizes exact anisotropic quartic
// concentric contours; proposals still require full independent pair coverage.
func DiscoverConcentricEllipseGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
	return discoverConcentricGraphPairTubes(ctx, a, b, p, o, true)
}
func discoverConcentricGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions, elliptic bool) (SplineSeedReport, error) {
	var e error
	var seeds SplineSeedReport
	if ctx == nil || !a.valid() || !b.valid() {
		return seeds, geom.ErrInvalid
	}
	pa, e := exactCoincidencePatch(a, p)
	if e != nil {
		return seeds, e
	}
	pb, e := exactCoincidencePatch(b, p)
	if e != nil {
		return seeds, e
	}
	maximum := o.Tubes.MaxOperations
	if maximum == 0 {
		maximum = 64000000
	}
	if maximum < 1 {
		return seeds, geom.ErrInvalid
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 512
	}
	if o.MaxTubes < 1 || o.MaxTubes > 4096 {
		return seeds, geom.ErrInvalid
	}
	radial, e := geom.DiscoverSplineGraphConcentricEllipses(ctx, pa, pb, geom.GraphPointContactOptions{MaxOperations: maximum, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		return seeds, e
	}
	if !elliptic && radial.MetricY.Cmp(big.NewRat(1, 1)) != 0 {
		return seeds, geom.ErrConvergence
	}
	used := radial.ExactOperations
	var center [2]float64
	for i, q := range radial.Center {
		center[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return seeds, e
		}
	}
	var radii, radiiY [2]float64
	for i, q := range radial.Radius {
		radii[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return seeds, e
		}
		radiiY[i], e = geom.RepresentedExactScalar(radial.RadiusY[i])
		if e != nil {
			return seeds, e
		}
		for _, q := range []*big.Rat{q, radial.RadiusY[i]} {
			for _, factor := range []*big.Rat{big.NewRat(3, 5), big.NewRat(4, 5)} {
				for _, origin := range radial.Center {
					for station := 0; station <= 64; station++ {
						offset := new(big.Rat).Mul(new(big.Rat).Mul(q, factor), big.NewRat(int64(station), 64))
						for _, value := range []*big.Rat{new(big.Rat).Add(origin, offset), new(big.Rat).Sub(origin, offset)} {
							if _, e := geom.RepresentedExactScalar(value); e != nil {
								return seeds, e
							}
						}
					}
				}
				if _, e = geom.RepresentedExactScalar(new(big.Rat).Mul(q, factor)); e != nil {
					return seeds, e
				}
			}
		}
	}
	for ellipseIndex, radius := range radii {
		radiusY := radiiY[ellipseIndex]
		for _, sx := range []float64{-1, 1} {
			for _, sy := range []float64{-1, 1} {
				for axis := 0; axis < 2; axis++ {
					limit := radius * .6
					if axis == 1 {
						limit = radiusY * .8
					}
					step := 16
					for part := 0; part < 64; {
						if e := ctx.Err(); e != nil {
							return seeds, e
						}
						if step > 64-part {
							step = 64 - part
						}
						lo, hi := limit*float64(part)/64, limit*float64(part+step)/64
						if maximum-used < 1 {
							return seeds, geom.ErrConvergence
						}
						depLo, depHi := radiusY*math.Sqrt(1-(hi/radius)*(hi/radius)), radiusY*math.Sqrt(1-(lo/radius)*(lo/radius))
						if axis == 1 {
							depLo, depHi = radius*math.Sqrt(1-(hi/radiusY)*(hi/radiusY)), radius*math.Sqrt(1-(lo/radiusY)*(lo/radiusY))
						}
						if radiusY == radius {
							depLo, depHi = math.Sqrt(radius*radius-hi*hi), math.Sqrt(radius*radius-lo*lo)
						}
						padding := .003
						if step == 2 {
							padding = .004
						}
						if step == 1 {
							padding = .002
						}
						freeLo, freeHi := center[0]+sx*lo, center[0]+sx*hi
						depMin, depMax := center[1]+sy*depLo, center[1]+sy*depHi
						if axis == 1 {
							freeLo, freeHi = center[1]+sy*lo, center[1]+sy*hi
							depMin, depMax = center[0]+sx*depLo, center[0]+sx*depHi
						}
						if freeLo > freeHi {
							freeLo, freeHi = freeHi, freeLo
						}
						if depMin > depMax {
							depMin, depMax = depMax, depMin
						}
						box := geom.ParameterRectangle{UMin: freeLo, UMax: freeHi, VMin: depMin - padding, VMax: depMax + padding}
						if axis == 1 {
							box = geom.ParameterRectangle{UMin: depMin - padding, UMax: depMax + padding, VMin: freeLo, VMax: freeHi}
						}
						other := box
						if axis == 0 {
							other.UMin -= padding
							other.UMax += padding
						} else {
							other.VMin -= padding
							other.VMax += padding
						}
						// Keep the nonlinear target coordinate free so its exact polynomial
						// remains in the uniform center residual instead of being widened
						// through an independent dependent-coordinate interval.
						box, other = other, box
						xa, xb := pa, pb
						xa, e = xa.Trim(box)
						if e != nil {
							return seeds, e
						}
						xb, e = xb.Trim(other)
						if e != nil {
							return seeds, e
						}
						proof, e := geom.CertifySurfacePairTube(xa, xb, geom.SurfacePairTubeOptions{FreeAxis: axis + 2, MaxOperations: maximum - used, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
						used += proof.Test.ExactOperations
						if e != nil || !proof.ProvenBranch {
							if step > 1 {
								step /= 2
								continue
							}
							return seeds, fmt.Errorf("tube radius=%g axis=%d part=%d: %v", radius, axis, part, e)
						}
						if len(seeds.Attempts) >= o.MaxTubes {
							return seeds, geom.ErrConvergence
						}
						seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: box, DomainB: other}, Tube: &proof})
						part += step
						step = 16
					}
				}
			}
		}
	}

	return seeds, nil
}

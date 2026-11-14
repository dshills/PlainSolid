package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// DiscoverRadialGraphContourTubes independently proves the complete supplied
// family of 1..4 nested radial graph contours, then certifies original-support
// tubes. Centers, radii and axis-switch stations must be exactly represented.
// This is a proposal report, not a coverage or topology certificate: callers
// must run CertifySplinePairBranches. Any failure publishes no partial report.
func DiscoverRadialGraphContourTubes(ctx context.Context, a, b Patch, center [2]*big.Rat, radii []*big.Rat, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
	empty := SplineSeedReport{}
	if ctx == nil || !a.valid() || !b.valid() {
		return empty, geom.ErrInvalid
	}
	maximum := o.Tubes.MaxOperations
	if maximum == 0 {
		maximum = 128000000
	}
	if maximum < 1 || maximum > 512000000 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 512
	}
	if o.MaxTubes < 1 || o.MaxTubes > 4096 {
		return empty, geom.ErrInvalid
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return empty, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return empty, e
	}
	proofBudget := maximum
	if proofBudget > 128000000 {
		proofBudget = 128000000
	}
	proof, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, center, radii, geom.GraphPointContactOptions{MaxOperations: proofBudget, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	// Read only the owned, revalidated coefficients from the certificate.
	used := proof.ExactOperations()
	charge := func() error {
		used++
		if used > maximum {
			return geom.ErrConvergence
		}
		return nil
	}
	center = proof.Center()
	radii = proof.Radii()
	var c [2]float64
	for i, q := range center {
		c[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return empty, e
		}
	}
	rr := make([]float64, len(radii))
	for i, q := range radii {
		rr[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return empty, e
		}
		for _, factor := range []*big.Rat{big.NewRat(3, 5), big.NewRat(4, 5)} {
			if e = charge(); e != nil {
				return empty, e
			}
			scaled := new(big.Rat).Mul(q, factor)
			if _, e = geom.RepresentedExactScalar(scaled); e != nil {
				return empty, e
			}
			for _, origin := range center {
				for station := 0; station <= 64; station++ {
					if e = ctx.Err(); e != nil {
						return empty, e
					}
					if e = charge(); e != nil {
						return empty, e
					}
					offset := new(big.Rat).Mul(scaled, big.NewRat(int64(station), 64))
					for _, value := range []*big.Rat{new(big.Rat).Add(origin, offset), new(big.Rat).Sub(origin, offset)} {
						if e = charge(); e != nil {
							return empty, e
						}
						if _, e = geom.RepresentedExactScalar(value); e != nil {
							return empty, e
						}
					}
				}
			}
		}
	}
	result, e := certifyRadialContourProposals(ctx, pa, pb, c, rr, rr, used, maximum, o)
	if e != nil {
		return empty, e
	}
	return result, nil
}

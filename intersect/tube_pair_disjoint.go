package intersect

import (
	"context"
	"errors"

	"plainsolid/geom"
)

func tubePatchRanges(s TubeSegment) [4][2]float64 {
	a, b := s.pa.Domain(), s.pb.Domain()
	return [4][2]float64{{a.UMin, a.UMax}, {a.VMin, a.VMax}, {b.UMin, b.UMax}, {b.VMin, b.VMax}}
}

func tubePairInteriorsSeparated(ctx context.Context, a, b TubeSegment, joined bool, o geom.SurfaceTransversalityOptions) (bool, error) {
	x, y := tubePatchRanges(a), tubePatchRanges(b)
	separate := func(p, q [4][2]float64) bool {
		for i := range 4 {
			if p[i][1] < q[i][0] || q[i][1] < p[i][0] {
				return true
			}
			if p[i][1] == q[i][0] || q[i][1] == p[i][0] {
				// A dependent coordinate is strictly interior. Two free
				// intervals that touch still need a junction proof.
				if i != a.axis || i != b.axis || joined {
					return true
				}
			}
		}
		return false
	}
	if separate(x, y) {
		return true, nil
	}
	// At a certified junction, if one tube's free coordinate is strictly
	// monotone along the other entire tube, the two interiors lie on opposite
	// sides of the common endpoint in that coordinate. This applies even if
	// their four-dimensional boxes overlap around the exact same junction.
	if joined && a.axis != b.axis {
		for _, pair := range [2][2]TubeSegment{{a, b}, {b, a}} {
			q, e := geom.CertifySurfacePairTangentComponent(pair[1].pa, pair[1].pb, pair[0].axis, o)
			if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) {
				return false, e
			}
			if q.Sign == pair[0].proof.FreeTangentSign() {
				return true, nil
			}
		}
	}
	// For nonadjacent pieces, contraction over entire intervals can show
	// that overlapping original boxes contain disjoint exact root enclosures.
	if !joined {
		contract := func(s TubeSegment) ([4][2]float64, error) {
			p := tubePatchRanges(s)
			x, e := s.proof.EncloseInterval(ctx, s.lo, s.hi, geom.SurfaceTubeIntervalOptions{MaxIterations: 2,
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) {
				return p, e
			}
			if x.HasBounds {
				p = [4][2]float64{{x.ParametersA.Min.X, x.ParametersA.Max.X}, {x.ParametersA.Min.Y, x.ParametersA.Max.Y},
					{x.ParametersB.Min.X, x.ParametersB.Max.X}, {x.ParametersB.Min.Y, x.ParametersB.Max.Y}}
			}
			return p, nil
		}
		p, e := contract(a)
		if e != nil {
			return false, e
		}
		q, e := contract(b)
		if e != nil {
			return false, e
		}
		for i := range 4 {
			if p[i][1] < q[i][0] || q[i][1] < p[i][0] {
				return true, nil
			}
		}
	}
	return false, nil
}

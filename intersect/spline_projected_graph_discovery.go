package intersect

import (
	"context"
	"plainsolid/geom"
)

// DiscoverProjectedAffineGraphPairTubes discovers in a selected model plane,
// then independently re-proves every seed against original XYZ supports. Axes
// are ordered distinct model components; all chart boxes remain original.
// The seed report is not full coverage/topology: use CertifySplinePairBranches
// with ProjectedAffineFallback and matching ProjectionAxes in Coverage.
func DiscoverProjectedAffineGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, axes [2]int, o SplinePlanePairDiscoveryOptions) (out SplineSeedReport, err error) {
	if ctx == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	pair := [2]Patch{a, b}
	for i := range pair {
		permuted, e := permuteSplineSupport(pair[i].Surface, [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}, p)
		if e != nil {
			return out, e
		}
		pair[i].Surface = permuted
	}
	discovered, e := DiscoverAffineXYGraphPairTubes(ctx, pair[0], pair[1], p, o)
	if e != nil {
		return out, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return out, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return out, e
	}
	for _, seed := range discovered.Attempts {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if seed.Tube == nil || !seed.Tube.ProvenBranch {
			return out, ErrUnresolved
		}
		qa, e := pa.Trim(seed.Cell.DomainA)
		if e != nil {
			return out, e
		}
		qb, e := pb.Trim(seed.Cell.DomainB)
		if e != nil {
			return out, e
		}
		options := geom.SurfacePairTubeOptions{FreeAxis: seed.Tube.FreeAxis, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits}
		proof, e := geom.CertifySurfacePairTube(qa, qb, options)
		if seed.BoundaryAnchor != nil {
			anchor, ae := seed.BoundaryAnchor.ExactOptions(options)
			if ae != nil {
				return out, ae
			}
			proof, e = geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, qa, qb, anchor)
		}
		if e != nil {
			return out, e
		}
		if !proof.ProvenBranch {
			return out, ErrUnresolved
		}
		seed.Tube = &proof
		out.Attempts = append(out.Attempts, seed)
	}
	return out, nil
}

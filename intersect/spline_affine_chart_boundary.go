package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// affineChartBoundarySites is a candidate generator only. A coordinate-plane
// support is proved over its whole original patch; rational boundary roots are
// then checked against the full target model by a certified triangular inverse.
// Unsupported planes, irrational roots and unresolved inversions add no sites.
// None of these omissions is an absence proof; caller coverage stays mandatory.
func affineChartBoundarySites(ctx context.Context, patches [2]*geom.SurfacePatch, remaining *int, bits int) ([][4]*big.Rat, error) {
	var sites [][4]*big.Rat
	for target, p := range patches {
		source := 1 - target
		d := p.Domain()
		if *remaining < 1 {
			return nil, geom.ErrConvergence
		}
		corner, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: *remaining, MaxCoefficientBits: bits})
		*remaining -= corner.ExactOperations
		if e != nil {
			return nil, e
		}
		for axis := 0; axis < 3; axis++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			level, e := geom.RepresentedExactScalar(corner.Point[axis])
			if e != nil {
				continue
			}
			var xyz [3][3]float64
			for i := range xyz {
				xyz[i][axis] = level
			}
			xyz[1][(axis+1)%3] = 1
			xyz[2][(axis+2)%3] = 1
			vec := func(v [3]float64) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
			if *remaining < 1 {
				return nil, geom.ErrConvergence
			}
			plane, e := geom.CertifySurfaceSupportingPlane(ctx, p, vec(xyz[0]), vec(xyz[1]), vec(xyz[2]), geom.SurfaceSupportingPlaneOptions{MaxDepth: 1, MaxCells: 64, MaxOperations: *remaining, MaxCoefficientBits: bits})
			*remaining -= plane.ExactOperations
			if e != nil || !plane.Complete || !plane.Coincident {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if *remaining < 1 {
					return nil, geom.ErrConvergence
				}
				continue
			}
			sd := patches[source].Domain()
			bounds := [2][2]float64{{sd.UMin, sd.UMax}, {sd.VMin, sd.VMax}}
			for fixed := 0; fixed < 2; fixed++ {
				for _, value := range bounds[fixed] {
					if *remaining < 1 {
						return nil, geom.ErrConvergence
					}
					iso, e := patches[source].IsoCurve(fixed, value, geom.SurfacePatchOptions{MaxOperations: *remaining, MaxCoefficientBits: bits})
					if e != nil {
						// IsoCurve cannot report failed work: conservatively consume its allowance.
						*remaining = 0
						return nil, e
					}
					*remaining -= iso.ExactOperations()
					if *remaining < 1 {
						return nil, geom.ErrConvergence
					}
					roots, e := iso.IntersectRawPlane(ctx, vec(xyz[0]), vec(xyz[1]), vec(xyz[2]), geom.SplinePlaneOptions{MaxOperations: *remaining, MaxCoefficientBits: bits, MaxSubdivisions: 1024, MaxDegree: 12, MaxControlPoints: 4096})
					*remaining -= roots.Operations
					if e != nil {
						return nil, e
					}
					if !roots.Complete || len(roots.Coincident) != 0 {
						continue
					}
					for _, hit := range roots.Hits {
						if hit.Parameter == nil {
							continue
						}
						found := false
						for first := 0; first < 3 && !found; first++ {
							if first == axis {
								continue
							}
							for fixedTarget := 0; fixedTarget < 2 && !found; fixedTarget++ {
								if *remaining < 1 {
									return nil, geom.ErrConvergence
								}
								second := 3 - axis - first
								inverse, e := geom.InvertTriangularSurfacePointRational(ctx, p, hit.Point, geom.TriangularSurfaceInverseOptions{RationalSurfaceInverseOptions: geom.RationalSurfaceInverseOptions{ProjectionAxes: [2]int{first, second}, MaxDepth: 4, MaxCells: 64, MaxOperations: *remaining, MaxCoefficientBits: bits}, FixedParameterAxis: fixedTarget, MaxRootSubdivisions: 1024})
								*remaining -= inverse.ExactOperations
								if ctx.Err() != nil {
									return nil, ctx.Err()
								}
								if e != nil || !inverse.Found || !inverse.ModelPointValidated || inverse.Parameters[0] == nil || inverse.Parameters[1] == nil {
									continue
								}
								q := [4]*big.Rat{}
								q[2*source+fixed] = new(big.Rat).SetFloat64(value)
								q[2*source+1-fixed] = new(big.Rat).Set(hit.Parameter)
								q[2*target] = new(big.Rat).Set(inverse.Parameters[0])
								q[2*target+1] = new(big.Rat).Set(inverse.Parameters[1])
								if len(sites) >= 64 {
									return nil, geom.ErrConvergence
								}
								sites = append(sites, q)
								found = true
							}
						}
					}
				}
			}
		}
	}
	return sites, nil
}

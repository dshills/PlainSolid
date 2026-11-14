package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// affineChartBoundarySites is a candidate generator only. A rational-plane
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
		planes, e := affineChartBoundaryPlanes(ctx, p, corner.Point, remaining, bits)
		if e != nil {
			return nil, e
		}
		for _, plane := range planes {
			if *remaining < 1 {
				return nil, geom.ErrConvergence
			}
			identity, e := geom.CertifySurfaceRationalPlaneIdentity(ctx, p, plane[0], plane[1], geom.SurfacePatchOptions{MaxOperations: *remaining, MaxCoefficientBits: bits})
			*remaining -= identity.ExactOperations
			if e != nil {
				return nil, e
			}
			if !identity.Proven {
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
					roots, e := iso.IntersectRationalPlane(ctx, plane[0], plane[1], geom.SplinePlaneOptions{MaxOperations: *remaining, MaxCoefficientBits: bits, MaxSubdivisions: 1024, MaxDegree: 12, MaxControlPoints: 4096})
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
							for second := 0; second < 3 && !found; second++ {
								if second == first {
									continue
								}
								for fixedTarget := 0; fixedTarget < 2 && !found; fixedTarget++ {
									if *remaining < 1 {
										return nil, geom.ErrConvergence
									}
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
	}
	return sites, nil
}

// Keep coordinate-plane proposals, and additionally derive an arbitrary rational
// plane through three exact original corners. Every proposal is subsequently
// checked against the entire support; three matching points are never a proof.
func affineChartBoundaryPlanes(ctx context.Context, p *geom.SurfacePatch, origin [3]*big.Rat, remaining *int, bits int) ([][2][3]*big.Rat, error) {
	planes := make([][2][3]*big.Rat, 0, 4)
	for axis := 0; axis < 3; axis++ {
		normal := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		normal[axis].SetInt64(1)
		planes = append(planes, [2][3]*big.Rat{origin, normal})
	}
	d := p.Domain()
	var points [2][3]*big.Rat
	for i, uv := range [][2]float64{{d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		if *remaining < 1 {
			return nil, geom.ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]), geom.SurfacePatchEvaluationOptions{MaxOperations: *remaining, MaxCoefficientBits: bits})
		*remaining -= jet.ExactOperations
		if e != nil {
			return nil, e
		}
		points[i] = jet.Point
	}
	check := func(q *big.Rat) error {
		*remaining--
		if *remaining < 0 || q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return geom.ErrConvergence
		}
		return nil
	}
	var delta [2][3]*big.Rat
	for i := range delta {
		for j := range delta[i] {
			delta[i][j] = new(big.Rat).Sub(points[i][j], origin[j])
			if e := check(delta[i][j]); e != nil {
				return nil, e
			}
		}
	}
	var normal [3]*big.Rat
	nonzero := false
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		left := new(big.Rat).Mul(delta[0][j], delta[1][k])
		if e := check(left); e != nil {
			return nil, e
		}
		right := new(big.Rat).Mul(delta[0][k], delta[1][j])
		if e := check(right); e != nil {
			return nil, e
		}
		normal[axis] = new(big.Rat).Sub(left, right)
		if e := check(normal[axis]); e != nil {
			return nil, e
		}
		nonzero = nonzero || normal[axis].Sign() != 0
	}
	if nonzero {
		planes = append(planes, [2][3]*big.Rat{origin, normal})
	}
	return planes, nil
}

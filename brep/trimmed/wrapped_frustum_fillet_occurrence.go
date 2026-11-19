//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// FilletedFrustumOccurrence is an immutable proper rational rigid occurrence.
// The native Solid stays in its original frame: this is not a re-authored or
// rounded world-space B-rep. Face IDs in all queries identify that original.
// Each inverse transformation has a 4096-bit intermediate arithmetic limit;
// exact root isolation keeps its separate caller-supplied work budget.
type FilletedFrustumOccurrence struct {
	source FilletedFrustumMaterial
	pose   placement.RationalRigid
}

// PlaceRigid proves the exact pose without converting any source support or
// trim coordinates to float64. Reflections and nonrigid maps are rejected by
// placement.NewRationalRigid; a zero pose or zero material is invalid.
func (q FilletedFrustumMaterial) PlaceRigid(ctx context.Context, pose placement.RationalRigid) (FilletedFrustumOccurrence, error) {
	if ctx == nil || q.recipe == nil {
		return FilletedFrustumOccurrence{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return FilletedFrustumOccurrence{}, e
	}
	m, e := pose.Matrix()
	if e != nil {
		return FilletedFrustumOccurrence{}, e
	}
	checked, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		return FilletedFrustumOccurrence{}, e
	}
	return FilletedFrustumOccurrence{q, checked}, nil
}

// Pose returns an owned exact original-to-world transform.
func (q FilletedFrustumOccurrence) Pose() ([3][4]*big.Rat, error) { return q.pose.Matrix() }

// Original retains native source IDs and original (not placed) supports.
func (q FilletedFrustumOccurrence) Original() FilletedFrustumMaterial { return q.source }

// ClassifyRational gives exact world material membership and original owners.
func (q FilletedFrustumOccurrence) ClassifyRational(ctx context.Context, p [3]*big.Rat) (FilletedFrustumMaterialPoint, error) {
	if q.source.recipe == nil {
		return FilletedFrustumMaterialPoint{}, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return FilletedFrustumMaterialPoint{}, e
	}
	return q.source.ClassifyRational(ctx, local)
}

// IntersectLineRational pulls back p+t*d exactly, leaving t, multiplicities,
// entry/exit semantics, discarded-support roots and native owners unchanged.
// Coincident support intervals remain outside the original query's contract.
func (q FilletedFrustumOccurrence) IntersectLineRational(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o geom.ExactCurvePlaneEventOptions) (FilletedObliqueLine, error) {
	if q.source.recipe == nil {
		return FilletedObliqueLine{}, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return FilletedObliqueLine{}, e
	}
	direction, e := q.pose.TransformVector(ctx, d, true, 4096)
	if e != nil {
		return FilletedObliqueLine{}, e
	}
	return q.source.IntersectLineRational(ctx, local, direction, min, max, o)
}

// MassProperties integrates the original analytic boundary and propagates its
// numerical centroid and centroidal inertia through the exact rigid pose.
// Area and volume are unchanged. Rational arithmetic applies the pose to the
// represented integration results before one final float64 rounding; it does
// not make the source quadrature exact. Change fields retain the original
// convergence estimates (not rigorous world error bounds).
func (q FilletedFrustumOccurrence) MassProperties(ctx context.Context, o parametric.IntegrationOptions) (parametric.Properties, error) {
	if ctx == nil || q.source.recipe == nil {
		return parametric.Properties{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Properties{}, e
	}
	m, e := q.pose.Matrix()
	if e != nil {
		return parametric.Properties{}, e
	}
	p, e := q.source.solid.MassProperties(ctx, o)
	if e != nil {
		return parametric.Properties{}, e
	}
	cv := [3]float64{p.Centroid.X, p.Centroid.Y, p.Centroid.Z}
	var c [3]*big.Rat
	for i, x := range cv {
		c[i] = new(big.Rat).SetFloat64(x)
		if c[i] == nil {
			return parametric.Properties{}, geom.ErrConvergence
		}
	}
	c, e = q.pose.TransformPoint(ctx, c, false, 4096)
	if e != nil {
		return parametric.Properties{}, e
	}
	for i, x := range c {
		cv[i], _ = x.Float64()
		if math.IsInf(cv[i], 0) {
			return parametric.Properties{}, geom.ErrConvergence
		}
	}
	var tensor [3][3]float64
	for i := 0; i < 3; i++ {
		for j := i; j < 3; j++ {
			sum := new(big.Rat)
			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					v := new(big.Rat).SetFloat64(p.Inertia[a][b])
					if v == nil {
						return parametric.Properties{}, geom.ErrConvergence
					}
					v.Mul(v, m[i][a])
					if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
						return parametric.Properties{}, geom.ErrConvergence
					}
					v.Mul(v, m[j][b])
					if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
						return parametric.Properties{}, geom.ErrConvergence
					}
					sum.Add(sum, v)
					if sum.Num().BitLen() > 4096 || sum.Denom().BitLen() > 4096 {
						return parametric.Properties{}, geom.ErrConvergence
					}
				}
			}
			tensor[i][j], _ = sum.Float64()
			if math.IsInf(tensor[i][j], 0) {
				return parametric.Properties{}, geom.ErrConvergence
			}
			tensor[j][i] = tensor[i][j]
		}
	}
	if e := ctx.Err(); e != nil {
		return parametric.Properties{}, e
	}
	p.Centroid = geom.Vec3{X: cv[0], Y: cv[1], Z: cv[2]}
	p.Inertia = tensor
	return p, nil
}

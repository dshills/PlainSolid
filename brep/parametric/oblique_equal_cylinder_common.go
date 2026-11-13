package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ObliqueEqualCylinderCommonResult retains the two genuine cylinder
// supports and the exact ideal volume and finite cap clearance evidence.
type ObliqueEqualCylinderCommonResult struct {
	Solid         Solid
	CylinderFaces [2][]int
	Measure       geom.EqualCylinderCommonMeasure
}

// ObliqueEqualCylinderCommon builds the common of equal-radius cylinders
// whose nonparallel raw directions have equal represented squared norm.
// Caps are raw axis parameter intervals about the intersection of the axes.
// All four caps must strictly clear the ideal common.
func ObliqueEqualCylinderCommon(ctx context.Context, center, u, v geom.Vec3, radius float64, uCaps, vCaps [2]float64, tolerance float64) (ObliqueEqualCylinderCommonResult, error) {
	var empty ObliqueEqualCylinderCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 || !center.IsFinite() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	w := [2][2]*big.Rat{}
	for i, c := range [2][2]float64{uCaps, vCaps} {
		if !finite(c[0]) || !finite(c[1]) || c[0] >= c[1] {
			return empty, geom.ErrInvalid
		}
		w[i] = [2]*big.Rat{q(c[0]), q(c[1])}
	}
	m, e := geom.MeasureEqualCylinderCommon(ctx, u, v, radius, &w[0], &w[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	if !m.CapsCover || m.CapTouchMask != 0 || radius/8 <= tolerance {
		return empty, geom.ErrConvergence
	}
	walls, e := geom.NewEqualCylinderCommonWalls(ctx, center, u, v, radius, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	var out ObliqueEqualCylinderCommonResult
	var faces []Face
	for i, s := range walls {
		owner, _, reverse := s.Ownership()
		out.CylinderFaces[owner] = append(out.CylinderFaces[owner], i)
		faces = append(faces, Face{Surface: obliqueCylinderChart{s, center, u, v, radius, i}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: reverse})
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	out.Measure = m
	return out, nil
}

type obliqueCylinderChart struct {
	geom.EqualCylinderCommonWall
	center, u, v geom.Vec3
	radius       float64
	index        int
}

func (s obliqueCylinderChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	b, e := s.HessianBounds()
	if e != nil {
		return 0, 0, 0, e
	}
	return upperBoreRat(b.UU), upperBoreRat(b.UV), 0, nil
}

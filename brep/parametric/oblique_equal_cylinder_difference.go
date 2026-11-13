package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ObliqueEqualCylinderDifferenceResult holds the two retained material
// components in increasing first-axis order. Their closures share the two
// exact model points reported by ContactNodes. They are individually closed
// solids; the pair is not asserted to have disjoint closures.
type ObliqueEqualCylinderDifferenceResult struct {
	Solids                                         [2]Solid
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	ContactNodes                                   [2]geom.EqualCylinderBisectorNode
	CommonVolumeSquared                            *big.Rat
}

// ObliqueEqualCylinderDifference subtracts the second finite cylinder from
// the first for intersecting equal-norm nonparallel raw axes and equal radii.
// All four axial caps must strictly clear the infinite-support common.
func ObliqueEqualCylinderDifference(ctx context.Context, center, u, v geom.Vec3, r float64, uCaps, vCaps [2]float64, tolerance float64) (ObliqueEqualCylinderDifferenceResult, error) {
	var empty ObliqueEqualCylinderDifferenceResult
	if ctx == nil || !center.IsFinite() || !finite(r) || r <= 0 || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	s, e := newObliqueCylinderUnionChart(ctx, center, u, v, r, uCaps, vCaps)
	if e != nil {
		return empty, e
	}
	if r/8 <= tolerance {
		return empty, geom.ErrConvergence
	}
	walls, e := geom.NewEqualCylinderCommonWalls(ctx, center, u, v, r, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	proof, e := geom.IntersectEqualRadiusCylinderBisectors(ctx, center, u, v, r, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	var out ObliqueEqualCylinderDifferenceResult
	for i, side := range []int{-1, 1} {
		var faces []Face
		for _, half := range []int{-1, 1} {
			for cap := range 2 {
				s.owner, s.side, s.half, s.cap = 0, side, half, cap == 1
				id := len(faces)
				if s.cap {
					out.StockCapFaces[i] = append(out.StockCapFaces[i], id)
				} else {
					out.StockWallFaces[i] = append(out.StockWallFaces[i], id)
				}
				reverse := side*half < 0
				if s.cap {
					reverse = !reverse
				}
				faces = append(faces, Face{Surface: s, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{0, 1}, Reversed: reverse})
			}
		}
		index := 2 + i
		c := walls[index]
		_, _, reverse := c.Ownership()
		out.CutterWallFaces[i] = []int{len(faces)}
		faces = append(faces, Face{Surface: obliqueCylinderChart{c, center, u, v, r, index}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: !reverse})
		out.Solids[i], e = joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return empty, e
		}
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	out.ContactNodes = proof.Nodes
	out.CommonVolumeSquared = new(big.Rat).Set(s.measure.VolumeSquared)
	return out, nil
}

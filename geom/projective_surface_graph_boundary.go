package geom

import (
	"context"
	"math/big"
)

// ProjectiveGraphContactCorner retains the original UV in both owners and
// their exactly equal physical model point.
type ProjectiveGraphContactCorner struct {
	OriginalUV [2][2]*big.Rat
	Point      [3]*big.Rat
}

// ProjectiveGraphContactEdge retains both original model-curve partitions.
// The directed normalized clock t on A maps to B's directed clock by
// BClockRatio*t/(1+(BClockRatio-1)*t). Each owner keeps its own original
// knot events. Within a piece, owner clock is affine in the piece's curve
// parameter as in SurfaceAffineCurvePiece.
type ProjectiveGraphContactEdge struct {
	Corners     [2]int
	Owners      [2]SurfaceAffineCurvePieces
	BClockRatio *big.Rat
}

// ProjectiveGraphContactBoundary is a complete closed boundary of a proved
// positive-area graph contact. The cycle runs counterclockwise in A's
// original chart. Its direction in B follows Overlap.Orientation.
type ProjectiveGraphContactBoundary struct {
	Overlap         ProjectiveSurfaceGraphOverlap
	Corners         [4]ProjectiveGraphContactCorner
	Edges           [4]ProjectiveGraphContactEdge
	Complete        bool
	ExactOperations int
}

// CertifyProjectiveGraphContactBoundary proves the whole original overlap and
// extracts both original analytic traces for every edge of its closed cycle.
// MaxPieces bounds each original edge partition and MaxOperations bounds the
// entire proof and extraction. No partial boundary is returned on failure.
func CertifyProjectiveGraphContactBoundary(ctx context.Context, source, target *SurfacePatch, axes [2]int,
	o SurfaceAffineCurveOptions) (result ProjectiveGraphContactBoundary, err error) {
	if o.MaxPieces < 0 {
		return result, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return result, err
	}
	var boundary ProjectiveGraphContactBoundary
	boundary.Overlap, err = CertifyProjectiveSurfaceGraphOverlap(ctx, source, target, axes, limits)
	b := &polynomialBudget{operations: boundary.Overlap.ExactOperations, maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if err != nil {
		return result, err
	}
	if !boundary.Overlap.Coincident || boundary.Overlap.NoArea {
		return result, ErrDomain
	}
	chart := boundary.Overlap
	domains := [2][2][2]*big.Rat{
		{{chart.Areas[0].U.Lower, chart.Areas[0].U.Upper}, {chart.Areas[0].V.Lower, chart.Areas[0].V.Upper}},
		{{chart.Areas[1].U.Lower, chart.Areas[1].U.Upper}, {chart.Areas[1].V.Lower, chart.Areas[1].V.Upper}},
	}
	owners := [2]*SurfacePatch{source, target}
	for i, corner := range [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		c := &boundary.Corners[i]
		for axis, end := range corner {
			c.OriginalUV[0][axis] = new(big.Rat).Set(domains[0][axis][end])
			j := axis
			if chart.SwapB {
				j = 1 - axis
			}
			if [2]bool{chart.ReverseBU, chart.ReverseBV}[axis] {
				end = 1 - end
			}
			c.OriginalUV[1][j] = new(big.Rat).Set(domains[1][j][end])
		}
		for owner, uv := range c.OriginalUV {
			if b.operations >= b.maxOperations {
				return result, ErrConvergence
			}
			jet, err := owners[owner].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{
				MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
			b.operations += jet.ExactOperations
			if err != nil {
				return result, err
			}
			if owner == 0 {
				c.Point = jet.Point
				continue
			}
			for axis, x := range jet.Point {
				if x.Cmp(c.Point[axis]) != 0 {
					return result, ErrConvergence
				}
			}
		}
	}
	for i := range boundary.Edges {
		j := (i + 1) % len(boundary.Corners)
		e := &boundary.Edges[i]
		e.Corners = [2]int{i, j}
		e.BClockRatio = new(big.Rat).Set(chart.BStationRatios[i%2])
		if i > 1 {
			e.BClockRatio.Inv(e.BClockRatio)
		}
		if err := b.check(e.BClockRatio); err != nil {
			return result, err
		}
		for owner, p := range owners {
			if b.operations >= b.maxOperations {
				return result, ErrConvergence
			}
			o.MaxPoles, o.MaxOperations, o.MaxCoefficientBits = limits.MaxPoles, b.maxOperations-b.operations, b.maxBits
			pieces, err := p.AffineParameterCurvePieces(ctx, boundary.Corners[i].OriginalUV[owner], boundary.Corners[j].OriginalUV[owner], o)
			b.operations += pieces.ExactOperations
			if err != nil {
				return result, err
			}
			if !pieces.Complete || !pieces.Continuous {
				return result, ErrConvergence
			}
			e.Owners[owner] = pieces
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	boundary.Complete = true
	return boundary, nil
}

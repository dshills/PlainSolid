package parabolacircle

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PrepareOsculationParabolaRays changes the clock of the two parabolic
// rays when the chart is transposed and the disk is above the parabola.
// Both original pcurves use the same clock as the model edge. The exact
// geometry, endpoints, subdivisions and free-coordinate jets are unchanged.
func PrepareOsculationParabolaRays(ctx context.Context, b trimmed.ExactBezierBoundary, bridge, vertex int, rays [2]int, proof edgecheck.Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if bridge < 0 || bridge >= len(b.Edges) || vertex < 0 || vertex >= len(b.Vertices) || rays[0] == rays[1] {
		return geom.ErrConvergence
	}
	// The original normal bridge has its fixed free coordinate in Y
	// and ends on the dependent X=1 wall for this orientation. Pinned
	// OCCT reports a false wire intersection for its identity clock.
	seam := b.Edges[bridge]
	a, z := b.Vertices[seam.Start], b.Vertices[seam.End]
	if a.Y != z.Y || a.X == z.X || max(a.X, z.X) != 1 {
		return nil
	}
	maxBits := proof.MaxBits
	if maxBits == 0 {
		maxBits = 16384 // The native edge reproof coefficient limit.
	}
	for _, id := range rays {
		if id < 0 || id >= len(b.Edges) {
			return geom.ErrConvergence
		}
		model := &b.Edges[id]
		if (model.Start == vertex) == (model.End == vertex) {
			return geom.ErrConvergence
		}
		speed := big.NewRat(2, 1)
		if model.End == vertex {
			speed.Inv(speed)
		}
		if err := reclockOsculationSpan(&model.Model, speed, maxBits); err != nil {
			return err
		}
		uses := 0
		for _, face := range b.Faces {
			for _, loop := range face.Loops {
				for i := range loop {
					if loop[i].Edge == id {
						if err := reclockOsculationSpan(&loop[i].UV, speed, maxBits); err != nil {
							return err
						}
						uses++
					}
				}
			}
		}
		if uses != 2 {
			return geom.ErrConvergence
		}
	}
	return ctx.Err()
}

// The endpoint-preserving parameter map t = speed*s/(1−s+speed*s)
// multiplies each homogeneous Bernstein pole i by speed^i after clearing
// the common positive denominator. The map is bijective on [0,1].
func reclockOsculationSpan(span *geom.ExactBezierSpan, speed *big.Rat, maxBits int) error {
	if span.Start != 0 || span.End != 1 || len(span.Control) < 2 {
		return geom.ErrConvergence
	}
	control := make([][4]*big.Rat, len(span.Control))
	factor := big.NewRat(1, 1)
	for i, row := range span.Control {
		for j, q := range row {
			coefficient := new(big.Rat).Mul(q, factor)
			if coefficient.Num().BitLen() > maxBits || coefficient.Denom().BitLen() > maxBits {
				return geom.ErrConvergence
			}
			control[i][j] = coefficient
		}
		factor.Mul(factor, speed)
	}
	span.Control = control
	return nil
}

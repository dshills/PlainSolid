package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type RationalGraphShellRegionOptions struct {
	Topology trimvalidate.Options
}

// RationalGraphShellRegion indexes one outer wire and its immediate hole
// wires. Material islands inside a hole become separate regions.
type RationalGraphShellRegion struct {
	Outer int
	Holes []int
}

// RationalGraphShellRegions proves a planar material arrangement for the
// corresponding oriented shell section. Rings follows original wire order.
// ProjectionAxes are ordered so positive planar winding agrees with the
// oriented plane. Loops bound the exact material, independent of display.
// This result does not build or attach a cap.
type RationalGraphShellRegions struct {
	ProjectionAxes [2]int
	Rings          []trimvalidate.RingPlacement
	Material       []RationalGraphShellRegion
}

func classifyRationalGraphShellRegions(ctx context.Context, section RationalGraphShellSection, faces []splineEmbeddingFace, plane polynomialPlaneEquation, limits geom.SurfacePatchOptions, o RationalGraphShellRegionOptions) (*RationalGraphShellRegions, error) {
	axes := plane.axes
	if plane.orientation < 0 {
		axes[0], axes[1] = axes[1], axes[0]
	}
	rings := make([]trimvalidate.Loop, len(section.Wires))
	for i, wire := range section.Wires {
		for _, id := range wire {
			b := section.Branches[id]
			for _, piece := range b.Pieces {
				h, e := projectedRationalGraphSectionControls(ctx, faces[b.Face].patch, piece, axes, limits)
				if e != nil {
					return nil, e
				}
				q, e := trimvalidate.ExactBezier(h)
				if e != nil {
					return nil, e
				}
				rings[i] = append(rings[i], q)
			}
		}
	}
	placements, proof, e := trimvalidate.ClassifyRingForest(ctx, rings, o.Topology)
	if e != nil {
		return nil, e
	}
	if proof.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	out := &RationalGraphShellRegions{ProjectionAxes: axes, Rings: placements}
	byOuter := make(map[int]int, len(placements))
	for i, r := range placements {
		if r.Depth%2 == 0 {
			if r.Orientation != 1 {
				return nil, geom.ErrConvergence
			}
			byOuter[i] = len(out.Material)
			out.Material = append(out.Material, RationalGraphShellRegion{Outer: i})
		} else if r.Orientation != -1 {
			return nil, geom.ErrConvergence
		}
	}
	for i, r := range placements {
		if r.Depth%2 != 0 {
			k, ok := byOuter[r.Parent]
			if !ok {
				return nil, geom.ErrConvergence
			}
			out.Material[k].Holes = append(out.Material[k].Holes, i)
		}
	}
	return out, nil
}

// The originating proof guarantees the selected model projections are affine
// on this entire knot cell. Exact derivatives at an interior anchor recover
// their coefficients in the original UV units. Homogeneous affine transport
// then preserves weights, endpoints, traversal, and complete curve coverage.
func projectedRationalGraphSectionControls(ctx context.Context, patch *geom.SurfacePatch, c PCurve, axes [2]int, limits geom.SurfacePatchOptions) ([][3]*big.Rat, error) {
	h, e := polynomialPlaneControls(c)
	if e != nil {
		return nil, e
	}
	var mid [2]*big.Rat
	first, last := h[0], h[len(h)-1]
	for i := range mid {
		a, b := new(big.Rat).Quo(first[i], first[2]), new(big.Rat).Quo(last[i], last[2])
		mid[i] = new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
	}
	j, e := patch.EvaluateRational(ctx, mid[0], mid[1], geom.SurfacePatchEvaluationOptions{
		MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	if e != nil {
		return nil, e
	}
	result := make([][3]*big.Rat, len(h))
	for i, p := range h {
		result[i][2] = p[2]
		for k, axis := range axes {
			result[i][k] = new(big.Rat).Mul(j.Point[axis], p[2])
			for a, derivative := range [2]*big.Rat{j.Du[axis], j.Dv[axis]} {
				delta := new(big.Rat).Sub(p[a], new(big.Rat).Mul(mid[a], p[2]))
				result[i][k].Add(result[i][k], new(big.Rat).Mul(derivative, delta))
			}
		}
	}
	return result, nil
}

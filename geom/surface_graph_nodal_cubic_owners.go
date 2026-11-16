package geom

import (
	"context"
	"math/big"
)

func bindNodalCubicSection(ctx context.Context, patches [2]*SurfacePatch, axes [2]int, residualAxis, free int,
	law nodalCubicLaw, alignment [2][3]*big.Rat, b *polynomialBudget) (*NodalCubicSection, error) {
	near, far, alpha, tau, err := nodalCubicLimits(law, b)
	if err != nil {
		return nil, err
	}
	out := &NodalCubicSection{
		ProjectionAxes: axes, FreeAxis: free, SourceDirection: law.direction, Scale: law.scale, LoopDepth: law.loopDepth,
		Domains: [2]ParameterRectangle{patches[0].Domain(), patches[1].Domain()}, Sectors: [4]int{0, 1, 0, 2},
	}
	out.ChartMaps, err = graphCubicChartMaps(out.Domains[0], free, law.direction, law.center, alignment, b)
	if err != nil {
		return nil, err
	}
	zero := new(big.Rat)
	exitHeight := new(big.Rat).Mul(tau, far)
	points := [4][2]*big.Rat{
		{zero, zero},
		{new(big.Rat).Neg(law.loopDepth), zero},
		{far, new(big.Rat).Neg(exitHeight)},
		{far, exitHeight},
	}
	var nodeJets [2]RationalSurfacePatchJet
	for i, point := range points {
		v := &out.Vertices[i]
		v.UV, err = applyGraphCubicChartMaps(out.ChartMaps, point, false, b)
		if err != nil {
			return nil, err
		}
		jets, err := graphCubicOriginalJets(ctx, patches, v.UV, b)
		if err != nil {
			return nil, err
		}
		for axis, value := range jets[0].Point {
			if value.Cmp(jets[1].Point[axis]) != 0 {
				return nil, ErrConvergence
			}
		}
		v.Point = jets[0].Point
		for owner := range patches {
			v.Boundary[owner] = graphCubicBoundary(out.Domains[owner], v.UV[owner])
			mask := v.Boundary[owner]
			if i < 2 && mask != 0 || i >= 2 && (mask == 0 || mask&(mask-1) != 0) {
				return nil, ErrConvergence
			}
		}
		if i == 0 {
			nodeJets = jets
		}
	}
	ends := [5]int{2, 0, 1, 0, 3}
	stations := [5]*big.Rat{new(big.Rat).Neg(tau), new(big.Rat).Neg(alpha), zero, alpha, tau}
	for i := range out.Edges {
		out.Edges[i] = NodalCubicEdge{Start: ends[i], End: ends[i+1],
			Range: [2]*big.Rat{new(big.Rat).Set(stations[i]), new(big.Rat).Set(stations[i+1])}}
	}

	// The two signs inside the bounds give separate regions at the node.
	// The positive sectors join around the strictly interior turning point.
	negativeDepth := new(big.Rat).Neg(law.loopDepth)
	half := big.NewRat(1, 2)
	witness := [3]*big.Rat{
		new(big.Rat).Mul(new(big.Rat).Sub(negativeDepth, near), half),
		new(big.Rat).Mul(negativeDepth, half),
		new(big.Rat).Mul(far, half),
	}
	for i, x := range witness {
		region := &out.Regions[i]
		region.UV, err = applyGraphCubicChartMaps(out.ChartMaps, [2]*big.Rat{x, zero}, false, b)
		if err != nil {
			return nil, err
		}
		jets, err := graphCubicOriginalJets(ctx, patches, region.UV, b)
		if err != nil {
			return nil, err
		}
		region.HeightSide = jets[0].Point[residualAxis].Cmp(jets[1].Point[residualAxis])
		if region.HeightSide != ([3]int{law.side, -law.side, -law.side})[i] {
			return nil, ErrConvergence
		}
	}

	// At the two preimages dx/dt is nonzero and dy/dx=t, so the lines
	// with slopes ±alpha cross transversely. These four outgoing vectors
	// bind the model-space germs to the two distinct original chart maps.
	edgeIDs := [4]int{3, 1, 2, 0}
	reversed := [4]bool{false, false, true, true}
	for i, signs := range [4][2]int64{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
		ray := &out.Rays[i]
		ray.Edge, ray.Reversed = edgeIDs[i], reversed[i]
		direction := [2]*big.Rat{big.NewRat(signs[0], 1), new(big.Rat).Mul(big.NewRat(signs[1], 1), alpha)}
		ray.UV, err = applyGraphCubicChartMaps(out.ChartMaps, direction, true, b)
		if err != nil {
			return nil, err
		}
		ray.Model, err = bindGraphCubicModelVector(ray.UV, nodeJets, b)
		if err != nil {
			return nil, err
		}
	}
	first, second := out.Rays[0].Model, out.Rays[1].Model
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(first[axes[0]], second[axes[1]]), new(big.Rat).Mul(first[axes[1]], second[axes[0]]))
	if err := b.check(determinant); err != nil {
		return nil, err
	}
	if determinant.Sign() == 0 {
		return nil, ErrDegenerate
	}
	out.ProjectionOrientation = determinant.Sign()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

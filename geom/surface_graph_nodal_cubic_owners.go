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
	out.ChartMaps, err = nodalCubicChartMaps(out.Domains[0], free, law, alignment, b)
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
		v.UV, err = applyNodalCubicChartMaps(out.ChartMaps, point, false, b)
		if err != nil {
			return nil, err
		}
		jets, err := nodalCubicOriginalJets(ctx, patches, v.UV, b)
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
			v.Boundary[owner] = nodalCubicBoundary(out.Domains[owner], v.UV[owner])
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
		region.UV, err = applyNodalCubicChartMaps(out.ChartMaps, [2]*big.Rat{x, zero}, false, b)
		if err != nil {
			return nil, err
		}
		jets, err := nodalCubicOriginalJets(ctx, patches, region.UV, b)
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
		ray.UV, err = applyNodalCubicChartMaps(out.ChartMaps, direction, true, b)
		if err != nil {
			return nil, err
		}
		for axis := range ray.Model {
			var tangent [2]*big.Rat
			for owner, jet := range nodeJets {
				tangent[owner] = new(big.Rat).Add(new(big.Rat).Mul(ray.UV[owner][0], jet.Du[axis]), new(big.Rat).Mul(ray.UV[owner][1], jet.Dv[axis]))
				if err = b.check(tangent[owner]); err != nil {
					return nil, err
				}
			}
			if tangent[0].Cmp(tangent[1]) != 0 {
				return nil, ErrConvergence
			}
			ray.Model[axis] = tangent[0]
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

func nodalCubicChartMaps(d ParameterRectangle, free int, law nodalCubicLaw, alignment [2][3]*big.Rat,
	b *polynomialBudget) (out [2][2][3]*big.Rat, err error) {
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for i, axis := range [2]int{free, 1 - free} {
		lo, hi := new(big.Rat).SetFloat64(limits[axis][0]), new(big.Rat).SetFloat64(limits[axis][1])
		width := new(big.Rat).Sub(hi, lo)
		row := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat).Add(lo, new(big.Rat).Mul(width, law.center[i]))}
		row[i].Set(width)
		if i == 0 && law.direction < 0 {
			row[i].Neg(row[i])
		}
		for _, q := range []*big.Rat{row[i], row[2]} {
			if err := b.check(q); err != nil {
				return out, err
			}
		}
		out[0][axis] = row
	}
	for axis := range alignment {
		for i := range out[1][axis] {
			x := new(big.Rat).Add(new(big.Rat).Mul(alignment[axis][0], out[0][0][i]), new(big.Rat).Mul(alignment[axis][1], out[0][1][i]))
			if i == 2 {
				x.Add(x, alignment[axis][2])
			}
			if err = b.check(x); err != nil {
				return out, err
			}
			out[1][axis][i] = x
		}
	}
	return out, nil
}

func applyNodalCubicChartMaps(m [2][2][3]*big.Rat, xy [2]*big.Rat, vector bool, b *polynomialBudget) (out [2][2]*big.Rat, err error) {
	for owner := range m {
		for axis, row := range m[owner] {
			x := new(big.Rat).Add(new(big.Rat).Mul(row[0], xy[0]), new(big.Rat).Mul(row[1], xy[1]))
			if !vector {
				x.Add(x, row[2])
			}
			if err = b.check(x); err != nil {
				return out, err
			}
			out[owner][axis] = x
		}
	}
	return out, nil
}

func nodalCubicBoundary(d ParameterRectangle, uv [2]*big.Rat) (mask uint8) {
	for i, edge := range [4]float64{d.UMin, d.UMax, d.VMin, d.VMax} {
		if uv[i/2].Cmp(new(big.Rat).SetFloat64(edge)) == 0 {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

func nodalCubicOriginalJets(ctx context.Context, patches [2]*SurfacePatch, uv [2][2]*big.Rat,
	b *polynomialBudget) (jets [2]RationalSurfacePatchJet, err error) {
	for owner, source := range patches {
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return jets, ErrConvergence
		}
		jets[owner], err = source.EvaluateRational(ctx, uv[owner][0], uv[owner][1],
			SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += jets[owner].ExactOperations
		if err != nil {
			return jets, err
		}
	}
	return jets, nil
}

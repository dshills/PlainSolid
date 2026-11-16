package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type parabolaCircleApexPartition struct {
	proof   *geom.SplineGraphParabolaCircleApexTangency
	regions [3]openGraphRegion
}

func (p parabolaCircleApexPartition) tangentSection() parabolaCircleTangentSection {
	proof := p.proof
	return parabolaCircleTangentSection{
		regions: p.regions, axis: proof.FreeAxis, diskRegion: proof.DiskRegion, parameters: proof.Parameters,
		point: proof.Point, circleCenter: proof.CircleCenter, slope: new(big.Rat),
		second: [2]*big.Rat{proof.ParabolaSecondDerivative, proof.CircleSecondDerivative},
	}
}

func proveParabolaCircleApexPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (p parabolaCircleApexPartition, err error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return p, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return p, err
	}
	p.proof, err = geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return p, err
	}
	open, err := splitParabolaCircleTangentOpenRegions(p.proof.ParabolaControls, p.proof.Parameters, axis)
	if err != nil {
		return p, err
	}
	copy(p.regions[:2], open[:])
	apex, err := exactGraphPartitionUV(p.proof.Parameters)
	if err != nil {
		return p, err
	}
	ring, _, err := exactGraphCircleRings(p.proof.CircleCenter, p.proof.RadiusSquared)
	if err != nil {
		return p, err
	}
	apexOccurrences := 0
	for _, piece := range ring {
		if piece.a == apex {
			apexOccurrences++
		}
	}
	if apexOccurrences != 1 {
		return p, geom.ErrConvergence
	}
	p.regions[2].loop = ring
	for i, side := range p.proof.RegionSides {
		p.regions[i].sign = side
	}
	return p, nil
}

func splitParabolaCircleTangentOpenRegions(controls [3][2]*big.Rat, parameters [2]*big.Rat, axis int) (out [2]openGraphRegion, err error) {
	open, err := parabolaOpenGraphRegions(controls, axis)
	if err != nil {
		return out, err
	}
	apex, err := exactGraphPartitionUV(parameters)
	if err != nil {
		return out, err
	}
	for region, original := range open {
		var loop []openSectionPiece
		for _, piece := range original.loop {
			if piece.side >= 0 {
				loop = append(loop, piece)
				continue
			}
			cut := parameters[axis]
			if ([2]float64{piece.a.X, piece.a.Y})[axis] == 1 {
				cut = new(big.Rat).Sub(big.NewRat(1, 1), cut)
			}
			curve := piece.c.(ExactQuadraticUV)
			stations := [3]*big.Rat{new(big.Rat), cut, big.NewRat(1, 1)}
			for i := range 2 {
				arc, err := curve.Restrict(stations[i], stations[i+1])
				if err != nil {
					return out, err
				}
				if arc.ends[1-i] != apex {
					return out, geom.ErrConvergence
				}
				loop = append(loop, openSectionPiece{a: arc.ends[0], b: arc.ends[1], c: arc, side: -1})
			}
		}
		out[region].loop = loop
	}
	return out, nil
}

func parabolaCircleApexRay(s Solid, edgeID, vertex, axis int, second *big.Rat) (int, error) {
	return parabolaCircleTangentRay(s, edgeID, vertex, axis, new(big.Rat), second)
}

// At the exact slope m the free-clock second derivative is
// (N_y”−m*N_x”−(v−m*u)*W”)/(W*(u')²). The weight-derivative terms
// cancel because v' equals m*u'.
func parabolaCircleTangentRay(s Solid, edgeID, vertex, axis int, slope, second *big.Rat) (int, error) {
	edge := s.edges[edgeID]
	if (edge.Start == vertex) == (edge.End == vertex) {
		return 0, geom.ErrConvergence
	}
	trim, ok := edge.Curve.(surfaceTrimCurve)
	if !ok {
		return 0, geom.ErrConvergence
	}
	curve, ok := trim.curve.(ExactQuadraticUV)
	if !ok {
		return 0, geom.ErrConvergence
	}
	if edge.End == vertex {
		curve = reversePCurve(curve).(ExactQuadraticUV)
	}
	uv, derivative, err := curve.EvaluateRational(new(big.Rat))
	if err != nil {
		return 0, err
	}
	f, d := derivative[axis], 1-axis
	if f.Sign() == 0 || derivative[d].Cmp(new(big.Rat).Mul(slope, f)) != 0 {
		return 0, geom.ErrConvergence
	}
	constantSecond := func(coordinate int) *big.Rat {
		coefficients := curve.control
		middle := new(big.Rat).Mul(big.NewRat(2, 1), coefficients[1][coordinate])
		diff := new(big.Rat).Sub(coefficients[2][coordinate], middle)
		return diff.Add(diff, coefficients[0][coordinate]).Mul(diff, big.NewRat(2, 1))
	}
	secondNumerator := constantSecond(d)
	secondNumerator.Sub(secondNumerator, new(big.Rat).Mul(slope, constantSecond(axis)))
	offset := new(big.Rat).Sub(uv[d], new(big.Rat).Mul(slope, uv[axis]))
	secondNumerator.Sub(secondNumerator, new(big.Rat).Mul(offset, constantSecond(2)))
	scale := new(big.Rat).Mul(f, f)
	scale.Mul(scale, curve.control[0][2])
	if secondNumerator.Quo(secondNumerator, scale).Cmp(second) != 0 {
		return 0, geom.ErrConvergence
	}
	return f.Sign(), nil
}

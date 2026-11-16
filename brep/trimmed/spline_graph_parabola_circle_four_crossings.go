package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

const fourCrossingParabolaPieces = 8

type parabolaCircleFourCrossingPartition struct {
	proof   *geom.SplineGraphParabolaCircleFourCrossings
	pieces  []openSectionPiece
	rays    [4][4]int
	regions [6]openGraphRegion
}

func proveParabolaCircleFourCrossingPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (parabolaCircleFourCrossingPartition, error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return parabolaCircleFourCrossingPartition{}, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return parabolaCircleFourCrossingPartition{}, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleFourCrossings(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return parabolaCircleFourCrossingPartition{}, err
	}
	return buildParabolaCircleFourCrossingPartition(proof)
}

func buildParabolaCircleFourCrossingPartition(proof *geom.SplineGraphParabolaCircleFourCrossings) (p parabolaCircleFourCrossingPartition, err error) {
	p.proof = proof
	var control [3][3]*big.Rat
	for i, c := range proof.ParabolaControls {
		control[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
	}
	parabola, err := NewExactQuadraticUV(control)
	if err != nil {
		return p, err
	}
	stations := [fourCrossingParabolaPieces + 1]*big.Rat{0: new(big.Rat), 8: big.NewRat(1, 1)}
	for i, node := range proof.Nodes {
		j := 2*i + 1
		stations[j] = node.Parameters[proof.FreeAxis]
		p.rays[i][0], p.rays[i][2] = j, j-1
	}
	for _, i := range [3]int{2, 4, 6} {
		// The cap and pocket each need a separate exact parabola vertex
		// so their native chord domains follow the original open regions.
		stations[i] = new(big.Rat).Quo(new(big.Rat).Add(stations[i-1], stations[i+1]), big.NewRat(2, 1))
	}
	for i := range fourCrossingParabolaPieces {
		trim, err := parabola.Restrict(stations[i], stations[i+1])
		if err != nil {
			return p, fmt.Errorf("trimmed: original parabola segment %d: %w", i, err)
		}
		p.pieces = append(p.pieces, openSectionPiece{a: trim.ends[0], b: trim.ends[1], c: trim, side: -1})
	}
	// The two mirror caps can have different original arc clocks.
	// A separate finite search covers represented subdivisions in this
	// profile while the prior arrangements retain their search budget.
	const capMaxDenominator = 32
	circle, nodes, err := exactParabolaCircleSectionRing(proof.Circle, proof.CircleRadius, proof.Nodes[:],
		[]geom.Vec2{p.pieces[1].b, p.pieces[5].b}, capMaxDenominator)
	if err != nil {
		return p, err
	}
	p.pieces = append(p.pieces, circle...)
	for i, position := range nodes {
		direction := proof.CircleCCWRays[i]
		p.rays[i][direction] = fourCrossingParabolaPieces + position
		p.rays[i][(direction+2)%4] = fourCrossingParabolaPieces + (position+len(circle)-1)%len(circle)
	}
	ccwForward := (proof.CircleSide < 0) != (proof.FreeAxis == 1)
	arc := func(a, b int, ccw bool) []openSectionPiece {
		if !ccw {
			a, b = b, a
		}
		var chain []openSectionPiece
		for j := nodes[a]; j != nodes[b]; j = (j + 1) % len(circle) {
			chain = append(chain, circle[j])
		}
		if !ccw {
			chain = reversedTwoCrossingCircleChain(chain)
		}
		return chain
	}
	nearLeft, nearMiddle, nearRight := arc(0, 1, ccwForward), arc(1, 2, ccwForward), arc(2, 3, ccwForward)
	far := arc(0, 3, !ccwForward)
	q := p.pieces[:fourCrossingParabolaPieces]
	nearExterior := []openSectionPiece{q[0]}
	nearExterior = append(nearExterior, nearLeft...)
	nearExterior = append(nearExterior, q[3:5]...)
	nearExterior = append(nearExterior, nearRight...)
	nearExterior = append(nearExterior, q[7])
	farExterior := append([]openSectionPiece{q[0]}, far...)
	farExterior = append(farExterior, q[7])

	pocket := append(slices.Clone(q[3:5]), reversedTwoCrossingCircleChain(nearMiddle)...)
	leftCap := append(slices.Clone(nearLeft), reversedTwoCrossingCircleChain(q[1:3])...)
	rightCap := append(slices.Clone(nearRight), reversedTwoCrossingCircleChain(q[5:7])...)
	diskRemainder := append(slices.Clone(q[1:3]), nearMiddle...)
	diskRemainder = append(diskRemainder, q[5:7]...)
	diskRemainder = append(diskRemainder, reversedTwoCrossingCircleChain(far)...)

	for i, loop := range [6][]openSectionPiece{nearExterior, farExterior, pocket, leftCap, rightCap, diskRemainder} {
		reverse := !ccwForward
		if i == 0 {
			reverse = ccwForward
		}
		if reverse {
			loop = reversedTwoCrossingCircleChain(loop)
		}
		if i < 2 {
			walls, err := saddleRectangleArc(loop[len(loop)-1].b, loop[0].a, geom.ParameterRectangle{UMax: 1, VMax: 1})
			if err != nil {
				return p, err
			}
			if len(walls) != 3 {
				return p, geom.ErrConvergence
			}
			loop = append(loop, walls...)
		}
		for j, trim := range loop {
			if trim.b != loop[(j+1)%len(loop)].a {
				return p, geom.ErrConvergence
			}
		}
		p.regions[i] = openGraphRegion{loop: loop, sign: proof.RegionSides[i]}
	}
	return p, nil
}

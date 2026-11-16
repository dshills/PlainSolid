package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

const twoCrossingCirclesParabolaPieces = 7

type parabolaTwoCrossingCirclesPartition struct {
	proof   *geom.SplineGraphParabolaTwoCrossingCircles
	pieces  []openSectionPiece
	circles [2][]int
	rays    [2][2][4]int
	regions [6]openGraphRegion
}

func proveParabolaTwoCrossingCirclesPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (parabolaTwoCrossingCirclesPartition, error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return parabolaTwoCrossingCirclesPartition{}, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return parabolaTwoCrossingCirclesPartition{}, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return parabolaTwoCrossingCirclesPartition{}, err
	}
	return buildParabolaTwoCrossingCirclesPartition(proof)
}

func buildParabolaTwoCrossingCirclesPartition(proof *geom.SplineGraphParabolaTwoCrossingCircles) (p parabolaTwoCrossingCirclesPartition, err error) {
	p.proof = proof
	axis := proof.FreeAxis
	var control [3][3]*big.Rat
	for i, c := range proof.ParabolaControls {
		control[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
	}
	parabola, err := NewExactQuadraticUV(control)
	if err != nil {
		return p, err
	}
	stations := [twoCrossingCirclesParabolaPieces + 1]*big.Rat{0: new(big.Rat), 7: big.NewRat(1, 1)}
	for order, i := range proof.CircleOrder {
		rank := 3*order + 1
		for j, node := range proof.Nodes[i] {
			position := rank + 2*j
			stations[position] = node.Parameters[axis]
			p.rays[i][j][0], p.rays[i][j][2] = position, position-1
		}
		// The parabola and a single circle arc can have the same two
		// endpoints. Keep distinct edge pairs by subdividing the interior
		// parabola in its original clock at an exact midpoint.
		stations[rank+1] = new(big.Rat).Quo(new(big.Rat).Add(stations[rank], stations[rank+2]), big.NewRat(2, 1))
	}
	for i := range twoCrossingCirclesParabolaPieces {
		part, err := parabola.Restrict(stations[i], stations[i+1])
		if err != nil {
			return p, fmt.Errorf("trimmed: original parabola segment %d: %w", i, err)
		}
		p.pieces = append(p.pieces, openSectionPiece{a: part.ends[0], b: part.ends[1], c: part, side: -1})
	}
	var exterior [2][]openSectionPiece
	for order, i := range proof.CircleOrder {
		arcs, nodes, err := exactTwoCrossingCircleRing(proof, i)
		if err != nil {
			return p, err
		}
		offset := len(p.pieces)
		for j := range arcs {
			p.circles[i] = append(p.circles[i], offset+j)
		}
		for j, n := range nodes {
			direction := proof.CircleCCWRays[i][j]
			p.rays[i][j][direction] = offset + n
			p.rays[i][j][(direction+2)%4] = offset + (n+len(arcs)-1)%len(arcs)
		}
		p.pieces = append(p.pieces, arcs...)
		chord := p.pieces[3*order+1 : 3*order+3]
		var sideSeen [2]bool
		for j, start := range nodes {
			var chain []openSectionPiece
			for k := start; k != nodes[1-j]; k = (k + 1) % len(arcs) {
				chain = append(chain, arcs[k])
			}
			side, err := twoCrossingCircleArcSide(chain[0], parabola, axis)
			if err != nil {
				return p, err
			}
			if sideSeen[side] {
				return p, geom.ErrConvergence
			}
			sideSeen[side] = true
			closing := chord
			if j == 0 {
				closing = reversedTwoCrossingCircleChain(chord)
			}
			cap := 2 + 2*i + side
			p.regions[cap] = openGraphRegion{loop: append(slices.Clone(chain), closing...), sign: proof.RegionSides[cap]}
			if j == 1 {
				chain = reversedTwoCrossingCircleChain(chain)
			}
			exterior[side] = append(exterior[side], p.pieces[3*order])
			exterior[side] = append(exterior[side], chain...)
		}
	}
	for side, chain := range exterior {
		chain = append(chain, p.pieces[twoCrossingCirclesParabolaPieces-1])
		if side == axis {
			chain = reversedTwoCrossingCircleChain(chain)
		}
		walls, err := saddleRectangleArc(chain[len(chain)-1].b, chain[0].a, geom.ParameterRectangle{UMax: 1, VMax: 1})
		if err != nil {
			return p, err
		}
		if len(walls) != 3 {
			return p, geom.ErrConvergence
		}
		p.regions[side] = openGraphRegion{loop: append(chain, walls...), sign: proof.RegionSides[side]}
	}
	for _, region := range p.regions {
		for i, piece := range region.loop {
			if piece.b != region.loop[(i+1)%len(region.loop)].a {
				return p, geom.ErrConvergence
			}
		}
	}
	return p, nil
}

func reversedTwoCrossingCircleChain(chain []openSectionPiece) []openSectionPiece {
	out := make([]openSectionPiece, len(chain))
	for i, piece := range chain {
		out[len(chain)-1-i] = reverseParabolaCircleCrossingPiece(piece)
	}
	return out
}

func twoCrossingCircleArcSide(piece openSectionPiece, parabola ExactQuadraticUV, axis int) (int, error) {
	point, _, err := piece.c.(ExactQuadraticUV).EvaluateRational(big.NewRat(1, 2))
	if err != nil {
		return 0, err
	}
	at, _, err := parabola.EvaluateRational(point[axis])
	if err != nil {
		return 0, err
	}
	// No other intersections exist anywhere on this connected arc.
	switch point[1-axis].Cmp(at[1-axis]) {
	case -1:
		return 0, nil
	case 1:
		return 1, nil
	default:
		return 0, geom.ErrConvergence
	}
}

func exactTwoCrossingCircleRing(proof *geom.SplineGraphParabolaTwoCrossingCircles, index int) ([]openSectionPiece, [2]int, error) {
	circle := proof.Circles[index]
	r := proof.CircleRadii[index]
	center := circle.Center
	type anchor struct {
		point, radial [2]*big.Rat
		node          int
	}
	var nodes [2]int
	anchors := make([]anchor, 0, 6)
	for _, d := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		a := anchor{node: -1}
		for i := range d {
			a.radial[i] = new(big.Rat).Mul(r, big.NewRat(d[i], 1))
			a.point[i] = new(big.Rat).Add(center[i], a.radial[i])
		}
		anchors = append(anchors, a)
	}
	for j, node := range proof.Nodes[index] {
		a := anchor{point: node.Parameters, node: j}
		for i := range a.radial {
			a.radial[i] = new(big.Rat).Sub(a.point[i], center[i])
		}
		existing := slices.IndexFunc(anchors, func(c anchor) bool {
			return a.point[0].Cmp(c.point[0]) == 0 && a.point[1].Cmp(c.point[1]) == 0
		})
		if existing < 0 {
			anchors = append(anchors, a)
		} else {
			anchors[existing].node = j
		}
	}
	// The cardinal anchors bound each consecutive physical CCW arc by
	// a quarter circle. Compare radii exactly, starting at physical +X.
	half := func(a anchor) bool {
		return a.radial[1].Sign() > 0 || (a.radial[1].Sign() == 0 && a.radial[0].Sign() > 0)
	}
	slices.SortFunc(anchors, func(a, b anchor) int {
		if half(a) != half(b) {
			if half(a) {
				return -1
			}
			return 1
		}
		cross := new(big.Rat).Sub(new(big.Rat).Mul(a.radial[0], b.radial[1]), new(big.Rat).Mul(a.radial[1], b.radial[0]))
		return -cross.Sign()
	})
	arcs := make([]openSectionPiece, len(anchors))
	for i, a := range anchors {
		if a.node >= 0 {
			nodes[a.node] = i
		}
		b := anchors[(i+1)%len(anchors)]
		q, err := shortRationalCircleArc(a.point, b.point, center, r)
		if err != nil {
			return nil, nodes, err
		}
		arcs[i] = openSectionPiece{a: q.ends[0], b: q.ends[1], c: q, side: -1}
	}
	return arcs, nodes, nil
}

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
	part, err := buildParabolaCrossingCirclesPartition(crossingCirclePartitionSource{
		axis: proof.FreeAxis, parabola: proof.ParabolaControls, circles: proof.Circles[:], radii: proof.CircleRadii[:],
		order: proof.CircleOrder[:], nodes: proof.Nodes[:], ccw: proof.CircleCCWRays[:], signs: proof.RegionSides[:],
	})
	if err != nil {
		return p, err
	}
	p.pieces, p.circles, p.rays, p.regions = part.pieces, [2][]int(part.circles), [2][2][4]int(part.rays), [6]openGraphRegion(part.regions)
	return p, nil
}

type crossingCirclePartitionSource struct {
	axis     int
	parabola [3][2]*big.Rat
	circles  []geom.GraphContactCircle
	radii    []*big.Rat
	order    []int
	nodes    [][2]geom.ParabolaCircleCrossing
	ccw      [][2]int
	signs    []int
}

type parabolaCrossingCirclesPartition struct {
	pieces  []openSectionPiece
	circles [][]int
	rays    [][2][4]int
	regions []openGraphRegion
}

func buildParabolaCrossingCirclesPartition(proof crossingCirclePartitionSource) (p parabolaCrossingCirclesPartition, err error) {
	p.circles = make([][]int, len(proof.circles))
	p.rays = make([][2][4]int, len(proof.circles))
	p.regions = make([]openGraphRegion, len(proof.signs))
	axis := proof.axis
	var control [3][3]*big.Rat
	for i, c := range proof.parabola {
		control[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
	}
	parabola, err := NewExactQuadraticUV(control)
	if err != nil {
		return p, err
	}
	parabolaPieces := 3*len(proof.circles) + 1
	stations := make([]*big.Rat, parabolaPieces+1)
	stations[0], stations[parabolaPieces] = new(big.Rat), big.NewRat(1, 1)
	for order, i := range proof.order {
		rank := 3*order + 1
		for j, node := range proof.nodes[i] {
			position := rank + 2*j
			stations[position] = node.Parameters[axis]
			p.rays[i][j][0], p.rays[i][j][2] = position, position-1
		}
		// The parabola and a single circle arc can have the same two
		// endpoints. Keep distinct edge pairs by subdividing the interior
		// parabola in its original clock at an exact midpoint.
		stations[rank+1] = new(big.Rat).Quo(new(big.Rat).Add(stations[rank], stations[rank+2]), big.NewRat(2, 1))
	}
	for i := range parabolaPieces {
		part, err := parabola.Restrict(stations[i], stations[i+1])
		if err != nil {
			return p, fmt.Errorf("trimmed: original parabola segment %d: %w", i, err)
		}
		p.pieces = append(p.pieces, openSectionPiece{a: part.ends[0], b: part.ends[1], c: part, side: -1})
	}
	var exterior [2][]openSectionPiece
	for order, i := range proof.order {
		chord := p.pieces[3*order+1 : 3*order+3]
		arcs, nodes, err := exactCrossingCircleRing(proof.circles[i], proof.radii[i], proof.nodes[i], chord[0].b)
		if err != nil {
			return p, fmt.Errorf("trimmed: original circle %d: %w", i, err)
		}
		offset := len(p.pieces)
		for j := range arcs {
			p.circles[i] = append(p.circles[i], offset+j)
		}
		for j, n := range nodes {
			direction := proof.ccw[i][j]
			p.rays[i][j][direction] = offset + n
			p.rays[i][j][(direction+2)%4] = offset + (n+len(arcs)-1)%len(arcs)
		}
		p.pieces = append(p.pieces, arcs...)
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
			p.regions[cap] = openGraphRegion{loop: append(slices.Clone(chain), closing...), sign: proof.signs[cap]}
			if j == 1 {
				chain = reversedTwoCrossingCircleChain(chain)
			}
			exterior[side] = append(exterior[side], p.pieces[3*order])
			exterior[side] = append(exterior[side], chain...)
		}
	}
	for side, chain := range exterior {
		chain = append(chain, p.pieces[parabolaPieces-1])
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
		p.regions[side] = openGraphRegion{loop: append(chain, walls...), sign: proof.signs[side]}
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

func exactCrossingCircleRing(circle geom.GraphContactCircle, r *big.Rat, junctions [2]geom.ParabolaCircleCrossing,
	chordMidpoint geom.Vec2) ([]openSectionPiece, [2]int, error) {
	arcs, nodes, err := exactParabolaCircleSectionRing(circle, r, junctions[:], []geom.Vec2{chordMidpoint}, 16)
	return arcs, [2]int(nodes), err
}

func exactParabolaCircleSectionRing(circle geom.GraphContactCircle, r *big.Rat, junctions []geom.ParabolaCircleCrossing,
	capMidpoints []geom.Vec2, maxDenominator int64) ([]openSectionPiece, []int, error) {
	center := circle.Center
	type anchor struct {
		point, radial [2]*big.Rat
		node          int
	}
	nodes := make([]int, len(junctions))
	anchors := make([]anchor, 0, 4+len(junctions))
	for _, d := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		a := anchor{node: -1}
		for i := range d {
			a.radial[i] = new(big.Rat).Mul(r, big.NewRat(d[i], 1))
			a.point[i] = new(big.Rat).Add(center[i], a.radial[i])
		}
		anchors = append(anchors, a)
	}
	for j, node := range junctions {
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
	arcs := make([]openSectionPiece, 0, len(anchors)+2)
	for i, a := range anchors {
		if a.node >= 0 {
			nodes[a.node] = len(arcs)
		}
		b := anchors[(i+1)%len(anchors)]
		q, err := shortRationalCircleArc(a.point, b.point, center, r)
		if err != nil {
			return nil, nodes, err
		}
		pieces := []ExactQuadraticUV{q}
		if a.node >= 0 && b.node >= 0 && a.node/2 == b.node/2 {
			chordMidpoint := capMidpoints[a.node/2]
			winding, err := geom.Orient2D(q.ends[0], q.ends[1], chordMidpoint)
			if err != nil {
				return nil, nodes, err
			}
			if winding <= 0 {
				pieces, err = subdivideTwoCrossingCircleCapArc(q, chordMidpoint, maxDenominator)
				if err != nil {
					return nil, nodes, err
				}
			}
		}
		for _, piece := range pieces {
			arcs = append(arcs, openSectionPiece{a: piece.ends[0], b: piece.ends[1], c: piece, side: -1})
		}
	}
	return arcs, nodes, nil
}

func subdivideTwoCrossingCircleCapArc(q ExactQuadraticUV, chordMidpoint geom.Vec2, maxDenominator int64) ([]ExactQuadraticUV, error) {
	// The exact geometry already proves a simple CCW cap. Its three
	// endpoint chords can turn clockwise when the parabola bows into
	// the cap. Find one or two represented interior circle points so
	// the native chord polygon is a positively oriented fan.
	type candidate struct {
		t     *big.Rat
		point geom.Vec2
		first bool
		last  bool
	}
	var points []candidate
	for denominator := int64(2); denominator <= maxDenominator; denominator++ {
		for numerator := int64(1); numerator < denominator; numerator++ {
			t := big.NewRat(numerator, denominator)
			if t.Denom().Int64() != denominator {
				continue
			}
			p, _, err := q.EvaluateRational(t)
			if err != nil {
				return nil, err
			}
			vertex, err := exactGraphPartitionUV(p)
			if err != nil {
				continue
			}
			// All three chart points have already been checked finite.
			first, _ := geom.Orient2D(q.ends[0], vertex, chordMidpoint)
			last, _ := geom.Orient2D(vertex, q.ends[1], chordMidpoint)
			if first > 0 && last > 0 {
				return splitTwoCrossingCircleArc(q, []*big.Rat{t})
			}
			points = append(points, candidate{t: t, point: vertex, first: first > 0, last: last > 0})
		}
	}
	for _, a := range points {
		if !a.first {
			continue
		}
		for _, z := range points {
			if !z.last || a.t.Cmp(z.t) >= 0 {
				continue
			}
			winding, _ := geom.Orient2D(a.point, z.point, chordMidpoint)
			if winding > 0 {
				return splitTwoCrossingCircleArc(q, []*big.Rat{a.t, z.t})
			}
		}
	}
	return nil, fmt.Errorf("trimmed: exact cap arc subdivision search exhausted: %w", geom.ErrConvergence)
}

func splitTwoCrossingCircleArc(q ExactQuadraticUV, stations []*big.Rat) ([]ExactQuadraticUV, error) {
	ends := append(slices.Clone(stations), big.NewRat(1, 1))
	out := make([]ExactQuadraticUV, 0, len(ends))
	a := new(big.Rat)
	for _, z := range ends {
		part, err := q.Restrict(a, z)
		if err != nil {
			return nil, err
		}
		out = append(out, part)
		a = z
	}
	return out, nil
}

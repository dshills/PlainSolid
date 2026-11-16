package step

import (
	"fmt"
	"math/big"
	"slices"
)

func repairConicalSupport(original, fixed *File, g conicalRepairGraph, budget *big.Rat) (ConicalRimFilletBoreRepairEvidence, []int64, error) {
	var e ConicalRimFilletBoreRepairEvidence
	fail := errConicalRimFilletBoreRepair
	cone := g.faces[g.cone]
	r, err := simple(original, cone.support, "CONICAL_SURFACE", 4)
	if err != nil {
		return e, nil, fail
	}
	e.ConeSupportID, e.ConeFaceID = cone.support, cone.id
	placement := r.Args[1].Ref
	p, err := conicalRepairFrame(original, placement)
	if err != nil || r.Args[1].Kind != Reference || g.incoming[placement] != 1 {
		return e, nil, fail
	}
	origin, err := mappedRationalCoordinate(original, p[0], "CARTESIAN_POINT", 3)
	if err != nil {
		return e, nil, err
	}
	axis, err := mappedRationalCoordinate(original, p[1], "DIRECTION", 3)
	if err != nil || !conicalExactDirection(axis, 2, 1) {
		return e, nil, fail
	}
	ref, err := mappedRationalCoordinate(original, p[2], "DIRECTION", 3)
	if err != nil || !conicalExactDirection(ref, 0, 1) {
		return e, nil, fail
	}
	radius, err := liftSourceRat(r.Args[2])
	if err != nil || radius.Sign() <= 0 {
		return e, nil, fail
	}
	var seam int64
	for _, u := range cone.loops[0] {
		edge := g.edges[u.id]
		if edge.start != edge.end {
			if seam != 0 && seam != u.id {
				return e, nil, fail
			}
			seam = u.id
		}
	}
	edge := g.edges[seam]
	if seam == 0 || !edge.sense || g.uses[seam][0].face != g.cone || g.uses[seam][1].face != g.cone || g.uses[seam][0].sense == g.uses[seam][1].sense {
		return e, nil, fail
	}
	e.SeamCurveID, e.ConeSeamVertexIDs = edge.curve, [2]int64{edge.start, edge.end}
	var poles [2]int64
	var ends [2][3]*big.Rat
	var radial [2]*big.Rat
	for i, id := range e.ConeSeamVertexIDs {
		poles[i], ends[i], err = conicalRepairVertex(original, id)
		if err != nil || ends[i][1].Cmp(origin[1]) != 0 {
			return e, nil, fail
		}
		radial[i] = new(big.Rat).Sub(ends[i][0], origin[0])
		if radial[i].Sign() <= 0 {
			return e, nil, fail
		}
		for _, u := range cone.loops[0] {
			rim := g.edges[u.id]
			if rim.start == id && rim.end == id {
				if e.ConeRimEdgeIDs[i] != 0 {
					return e, nil, fail
				}
				e.ConeRimEdgeIDs[i] = u.id
			}
		}
		if e.ConeRimEdgeIDs[i] == 0 {
			return e, nil, fail
		}
		e.OriginalRimStations[i] = ends[i][2].RatString()
	}
	dz := new(big.Rat).Sub(ends[1][2], ends[0][2])
	dr := new(big.Rat).Sub(radial[1], radial[0])
	if dz.Sign() <= 0 || dr.Sign() <= 0 {
		return e, nil, fail
	}
	slope := new(big.Rat).Quo(dr, dz)
	station := new(big.Rat).Sub(ends[0][2], origin[2])
	root := new(big.Rat).Sub(radial[0], new(big.Rat).Mul(slope, station))
	if root.Cmp(radius) != 0 {
		return e, nil, fail
	}
	angle, err := liftSourceRat(r.Args[3])
	if err != nil || angle.Sign() <= 0 || angle.Cmp(big.NewRat(1, 1)) >= 0 ||
		angle.Num().BitLen() > 256 || angle.Denom().BitLen() > 256 {
		return e, nil, fail
	}
	sinLo, sinHi := conicalRepairTrigInterval(angle, 1)
	cosLo, cosHi := conicalRepairTrigInterval(angle, 0)
	tanLo := new(big.Rat).Quo(sinLo, cosHi)
	tanHi := new(big.Rat).Quo(sinHi, cosLo)
	stationMax := conicalRepairMax(new(big.Rat).Abs(station), new(big.Rat).Abs(new(big.Rat).Sub(ends[1][2], origin[2])))
	drift := new(big.Rat).Mul(stationMax, conicalRepairMax(conicalRepairDifference(tanLo, slope), conicalRepairDifference(tanHi, slope)))
	if drift.Cmp(budget) > 0 {
		return e, nil, fmt.Errorf("%w: finite cone angle displacement exceeds the source-unit limit", fail)
	}
	seamDrift, err := conicalRepairLineBound(original, edge.curve, ends)
	if err != nil {
		return e, nil, err
	}
	if seamDrift.Cmp(budget) > 0 {
		return e, nil, fmt.Errorf("%w: finite line seam displacement exceeds the source-unit limit", fail)
	}
	e.OriginalAngle, e.ExactSlope = r.Args[3].Text, slope.RatString()
	e.ConeDisplacementBound, e.SeamDisplacementBound = drift.RatString(), seamDrift.RatString()
	line, _ := simple(original, edge.curve, "LINE", 3)
	axisID, err := addConicalRepairEntity(fixed, Record{"AXIS1_PLACEMENT", []Value{{Kind: String}, {Kind: Reference, Ref: p[0]}, {Kind: Reference, Ref: p[1]}}})
	if err != nil {
		return e, nil, err
	}
	segment := conicalRepairSegment(poles)
	generator, err := addConicalRepairEntity(fixed, segment)
	if err != nil {
		return e, nil, err
	}
	ovalSet(fixed, edge.curve, segment)
	ovalSet(fixed, cone.support, Record{"SURFACE_OF_REVOLUTION", []Value{r.Args[0], {Kind: Reference, Ref: generator}, {Kind: Reference, Ref: axisID}}})
	return e, []int64{placement, line.Args[1].Ref, line.Args[2].Ref}, nil
}

func conicalRepairFrame(f *File, id int64) ([]int64, error) {
	r, err := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		return nil, err
	}
	return namedReferences(r, 1, 2, 3)
}

func conicalRepairVertex(f *File, id int64) (int64, [3]*big.Rat, error) {
	r, err := simple(f, id, "VERTEX_POINT", 2)
	if err != nil || r.Args[1].Kind != Reference {
		return 0, [3]*big.Rat{}, errConicalRimFilletBoreRepair
	}
	p, err := mappedRationalCoordinate(f, r.Args[1].Ref, "CARTESIAN_POINT", 3)
	return r.Args[1].Ref, p, err
}

func conicalRepairSegment(poles [2]int64) Record {
	return Record{"B_SPLINE_CURVE_WITH_KNOTS", []Value{
		{Kind: String}, {Kind: Number, Text: "1"},
		{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: poles[0]}, {Kind: Reference, Ref: poles[1]}}},
		{Kind: Enumeration, Text: "UNSPECIFIED"}, {Kind: Enumeration, Text: "F"}, {Kind: Enumeration, Text: "F"},
		{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}},
		{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "1."}}},
		{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"},
	}}
}

func conicalRepairDifference(a, b *big.Rat) *big.Rat {
	return new(big.Rat).Abs(new(big.Rat).Sub(a, b))
}

// For 0 < angle < 1 the decreasing alternating terms give an exact
// enclosure between consecutive odd and even partial sums.
func conicalRepairTrigInterval(angle *big.Rat, power int) (*big.Rat, *big.Rat) {
	term := big.NewRat(1, 1)
	if power == 1 {
		term.Set(angle)
	}
	squared := new(big.Rat).Mul(angle, angle)
	even := new(big.Rat).Set(term)
	odd := new(big.Rat)
	sum := new(big.Rat).Set(term)
	for n := 1; n <= 16; n++ {
		p := int64(power + 2*n)
		term.Mul(term, squared).Quo(term, big.NewRat((p-1)*p, 1))
		if n%2 == 0 {
			sum.Add(sum, term)
			even.Set(sum)
		} else {
			sum.Sub(sum, term)
			odd.Set(sum)
		}
	}
	return odd, even
}

func conicalRepairLineBound(f *File, curve int64, ends [2][3]*big.Rat) (*big.Rat, error) {
	r, err := simple(f, curve, "LINE", 3)
	if err != nil {
		return nil, err
	}
	refs, err := namedReferences(r, 1, 2)
	if err != nil {
		return nil, err
	}
	p, err := mappedRationalCoordinate(f, refs[0], "CARTESIAN_POINT", 3)
	if err != nil {
		return nil, err
	}
	v, err := simple(f, refs[1], "VECTOR", 3)
	if err != nil || v.Args[1].Kind != Reference {
		return nil, errConicalRimFilletBoreRepair
	}
	magnitude, err := liftSourceRat(v.Args[2])
	if err != nil || magnitude.Sign() <= 0 {
		return nil, errConicalRimFilletBoreRepair
	}
	d, err := mappedRationalCoordinate(f, v.Args[1].Ref, "DIRECTION", 3)
	if err != nil || d[2].Sign() <= 0 {
		return nil, errConicalRimFilletBoreRepair
	}
	var chord [3]*big.Rat
	for i := range chord {
		chord[i] = new(big.Rat).Sub(ends[1][i], ends[0][i])
	}
	if mappedRationalDot(chord, d).Sign() <= 0 {
		return nil, errConicalRimFilletBoreRepair
	}
	maximum := new(big.Rat)
	for _, endpoint := range ends {
		t := new(big.Rat).Quo(new(big.Rat).Sub(endpoint[2], p[2]), d[2])
		bound := new(big.Rat)
		for axis := range 2 {
			q := new(big.Rat).Add(p[axis], new(big.Rat).Mul(t, d[axis]))
			bound.Add(bound, conicalRepairDifference(q, endpoint[axis]))
		}
		maximum.Set(conicalRepairMax(maximum, bound))
	}
	// Each projected endpoint is at most as far from the vertex as the
	// source line at equal Z. The projection preserves the directed
	// order. Affine interpolation of the paired segments bounds their
	// entire finite spans by the maximum endpoint displacement.
	return maximum, nil
}

func conicalExactDirection(v [3]*big.Rat, axis, sign int) bool {
	for i, x := range v {
		want := int64(0)
		if i == axis {
			want = int64(sign)
		}
		if x.Cmp(big.NewRat(want, 1)) != 0 {
			return false
		}
	}
	return true
}

func conicalRepairEdgeOrder(g conicalRepairGraph) []int64 {
	ids := make([]int64, 0, len(g.edges))
	for id := range g.edges {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func conicalRepairMax(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

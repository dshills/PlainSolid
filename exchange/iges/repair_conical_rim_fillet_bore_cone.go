package iges

import (
	"fmt"
	"math/big"
	"strconv"
)

func repairIGESConicalSupport(p *igesBoreRimSource, g igesConicalRepairGraph, edits map[int]topologyRecord, budget *big.Rat) (ConicalRimFilletBoreRepairEvidence, error) {
	var e ConicalRimFilletBoreRepairEvidence
	fail := errIGESConicalRimFilletBoreRepair
	cone := g.faces[g.cone]
	rec := p.records[cone.support]
	e.ConeFaceID, e.ConeSupportID = cone.id, cone.support
	var removed [3]int
	var root igesPlanarVec
	for i, component := range []int{1, 2, 5} {
		id, err := intValue(rec.values[component])
		if err != nil || g.supportRefs[id] != 1 {
			return e, fail
		}
		kind := 123
		if i == 0 {
			kind = 116
		}
		v, err := readIGESConicalCoordinate(p, id, kind)
		if err != nil || (i == 1 && !v.equal(igesConicalIntVector(0, 0, 1))) ||
			(i == 2 && !v.equal(igesConicalIntVector(1, 0, 0))) {
			return e, fail
		}
		removed[i] = id
		if i == 0 {
			root = v
		}
	}
	e.ConeRootPointID, e.ConeAxisID, e.ConePhaseID = removed[0], removed[1], removed[2]
	rootRadius, err := igesConicalRepairNumber(rec.values[3])
	if err != nil || rootRadius.Sign() <= 0 {
		return e, fail
	}
	for _, u := range cone.face.loops[0] {
		edge := g.edges[u.edge-1]
		if edge.start == edge.end {
			continue
		}
		if e.ConeSeamEdgeIndex != 0 && e.ConeSeamEdgeIndex != u.edge {
			return e, fail
		}
		if edge.uses[0].face != cone.id || edge.uses[1].face != cone.id {
			return e, fail
		}
		e.ConeSeamEdgeIndex, e.ConeSeamCurveID = u.edge, edge.curve
		e.ConeVertexIndices = [2]int{edge.start, edge.end}
	}
	if e.ConeSeamEdgeIndex == 0 || p.records[e.ConeSeamCurveID].kind != 110 {
		return e, fail
	}
	seam, err := p.readCurve(e.ConeSeamCurveID)
	if err != nil {
		return e, err
	}
	var ends [2]igesPlanarVec
	for i, index := range e.ConeVertexIndices {
		ends[i] = g.vertices[index-1]
		if !seam.ends[i].equal(ends[i]) {
			return e, fail
		}
	}
	if ends[0][2].Cmp(ends[1][2]) > 0 {
		ends[0], ends[1] = ends[1], ends[0]
		e.ConeVertexIndices[0], e.ConeVertexIndices[1] = e.ConeVertexIndices[1], e.ConeVertexIndices[0]
	}
	var radial, height [2]*big.Rat
	for i, end := range ends {
		radial[i] = new(big.Rat).Sub(end[0], root[0])
		height[i] = new(big.Rat).Sub(end[2], root[2])
		if end[1].Cmp(root[1]) != 0 || radial[i].Sign() <= 0 || height[i].Sign() < 0 {
			return e, fail
		}
		for _, u := range cone.face.loops[0] {
			rim := g.edges[u.edge-1]
			if rim.start == e.ConeVertexIndices[i] && rim.end == rim.start {
				if e.ConeRimEdgeIndices[i] != 0 || p.records[rim.curve].kind != 100 {
					return e, fail
				}
				e.ConeRimEdgeIndices[i] = u.edge
			}
		}
		if e.ConeRimEdgeIndices[i] == 0 {
			return e, fail
		}
		e.OriginalRimStations[i] = end[2].RatString()
	}
	dz := new(big.Rat).Sub(height[1], height[0])
	dr := new(big.Rat).Sub(radial[1], radial[0])
	if dz.Sign() <= 0 || dr.Sign() <= 0 {
		return e, fail
	}
	slope := new(big.Rat).Quo(dr, dz)
	intercept := new(big.Rat).Sub(radial[0], new(big.Rat).Mul(slope, height[0]))
	if intercept.Cmp(rootRadius) != 0 {
		return e, fail
	}
	angle, err := igesConicalRepairNumber(rec.values[4])
	if err != nil || angle.Sign() <= 0 || angle.Num().BitLen() > 256 || angle.Denom().BitLen() > 256 {
		return e, fail
	}
	lo, hi, err := igesConicalTangentBounds(angle)
	if err != nil {
		return e, err
	}
	// The original circular boundaries fix the finite axial extent. At
	// equal height and azimuth the displacement is the radial discrepancy
	// h*abs(tan(theta)-slope), maximized at the upper station.
	drift := new(big.Rat).Sub(slope, lo)
	drift.Abs(drift)
	if x := new(big.Rat).Abs(new(big.Rat).Sub(slope, hi)); x.Cmp(drift) > 0 {
		drift.Set(x)
	}
	drift.Mul(drift, height[1])
	if drift.Cmp(budget) > 0 {
		return e, fmt.Errorf("%w: finite type 194 angle displacement exceeds source-unit limit", fail)
	}
	net, err := exactIGESConicalRepairNet(root, ends, radial)
	if err != nil {
		return e, err
	}
	rec.kind, rec.values = 128, net
	edits[rec.id] = rec
	for _, id := range removed {
		edits[id] = topologyRecord{}
	}
	e.OriginalAngleDegrees, e.ExactSlope, e.ConeDisplacementBound = p.records[cone.support].values[4].text, slope.RatString(), drift.RatString()
	return e, nil
}

func readIGESConicalCoordinate(p *igesBoreRimSource, id, kind int) (igesPlanarVec, error) {
	r, err := p.entity(id, kind, false)
	if err != nil {
		return igesPlanarVec{}, err
	}
	if kind == 116 && len(r.values) == 5 {
		symbol, err := intValue(r.values[4])
		if err != nil || symbol != 0 {
			return igesPlanarVec{}, errIGESConicalRimFilletBoreRepair
		}
	} else if kind != 123 || len(r.values) != 4 {
		return igesPlanarVec{}, errIGESConicalRimFilletBoreRepair
	}
	return igesPlanarCoordinates(r.values[1:])
}

func igesConicalIntVector(x, y, z int64) igesPlanarVec {
	return igesPlanarVec{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
}

func exactIGESConicalRepairNet(root igesPlanarVec, ends [2]igesPlanarVec, radial [2]*big.Rat) ([]parameter, error) {
	values := make([]parameter, 0, 102)
	add := func(v ...int) {
		for _, x := range v {
			values = append(values, parameter{text: strconv.Itoa(x)})
		}
	}
	add(128, 8, 1, 2, 1, 1, 0, 0, 0, 0)
	add(0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4, 0, 0, 1, 1)
	for range ends {
		add(1, 1, 2, 2, 4, 2, 2, 1, 1)
	}
	azimuths := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, end := range ends {
		for _, axis := range azimuths {
			p := igesPlanarVec{new(big.Rat).Add(root[0], new(big.Rat).Mul(radial[i], big.NewRat(axis[0], 1))),
				new(big.Rat).Add(root[1], new(big.Rat).Mul(radial[i], big.NewRat(axis[1], 1))), end[2]}
			for _, q := range p {
				s, err := exactIGESSplineDecimal(q)
				if err != nil {
					return nil, err
				}
				values = append(values, parameter{text: s})
			}
		}
	}
	add(0, 4, 0, 1)
	return values, nil
}

// The IGES angle is in degrees. Machin's identity and consecutive
// alternating partial sums supply outward rational bounds for pi.
func igesConicalTangentBounds(degrees *big.Rat) (*big.Rat, *big.Rat, error) {
	a0, a1 := igesConicalAtanBounds(5)
	b0, b1 := igesConicalAtanBounds(239)
	factor := new(big.Rat).Quo(degrees, big.NewRat(180, 1))
	radians := func(a, b *big.Rat) *big.Rat {
		p := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a), new(big.Rat).Mul(big.NewRat(4, 1), b))
		return p.Mul(p, factor)
	}
	lo, hi := radians(a0, b1), radians(a1, b0)
	if lo.Sign() <= 0 || hi.Cmp(big.NewRat(1, 1)) >= 0 {
		return nil, nil, errIGESConicalRimFilletBoreRepair
	}
	sinLo, _ := igesConicalTrigBounds(lo, 1)
	_, cosHi := igesConicalTrigBounds(lo, 0)
	_, sinHi := igesConicalTrigBounds(hi, 1)
	cosLo, _ := igesConicalTrigBounds(hi, 0)
	return new(big.Rat).Quo(sinLo, cosHi), new(big.Rat).Quo(sinHi, cosLo), nil
}

func igesConicalAtanBounds(denominator int64) (*big.Rat, *big.Rat) {
	factor := big.NewRat(1, denominator*denominator)
	power := big.NewRat(1, denominator)
	sum := new(big.Rat)
	for n := int64(0); n < 24; n++ {
		term := new(big.Rat).Quo(power, big.NewRat(2*n+1, 1))
		if n%2 == 0 {
			sum.Add(sum, term)
		} else {
			sum.Sub(sum, term)
		}
		power.Mul(power, factor)
	}
	return sum, new(big.Rat).Add(sum, new(big.Rat).Quo(power, big.NewRat(49, 1)))
}

func igesConicalTrigBounds(angle *big.Rat, firstPower int64) (*big.Rat, *big.Rat) {
	term := big.NewRat(1, 1)
	if firstPower == 1 {
		term.Set(angle)
	}
	sum := new(big.Rat).Set(term)
	lo := new(big.Rat)
	x2 := new(big.Rat).Mul(angle, angle)
	for n := int64(1); n <= 12; n++ {
		p := firstPower + 2*n
		term.Mul(term, x2).Quo(term, big.NewRat(p*(p-1), 1))
		if n%2 == 0 {
			sum.Add(sum, term)
		} else {
			sum.Sub(sum, term)
			lo.Set(sum)
		}
	}
	return lo, sum
}

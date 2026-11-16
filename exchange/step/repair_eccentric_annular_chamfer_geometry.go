package step

import (
	"fmt"
	"math/big"
)

func repairEccentricAnnularWall(original, fixed *File, g conicalRepairGraph, face int, budget *big.Rat) (EccentricAnnularChamferWallRepair, []int64, error) {
	var e EccentricAnnularChamferWallRepair
	fail := errEccentricAnnularChamferRepair
	fc := g.faces[face]
	r, err := simple(original, fc.support, "CONICAL_SURFACE", 4)
	if err != nil || len(fc.loops) != 1 || len(fc.loops[0]) != 4 {
		return e, nil, fail
	}
	e.FaceID, e.ConeID = fc.id, fc.support
	e.PlacementID = r.Args[1].Ref
	p, err := conicalRepairFrame(original, e.PlacementID)
	if err != nil || r.Args[1].Kind != Reference || g.incoming[e.PlacementID] != 1 {
		return e, nil, fail
	}
	axis, err := mappedRationalCoordinate(original, p[1], "DIRECTION", 3)
	if err != nil {
		return e, nil, err
	}
	e.AxisDirectionID, e.AxisSign = p[1], axis[2].Sign()
	if e.AxisSign == 0 || !conicalExactDirection(axis, 2, e.AxisSign) {
		return e, nil, fail
	}
	x, err := mappedRationalCoordinate(original, p[2], "DIRECTION", 3)
	if err != nil || !conicalExactDirection(x, 0, 1) {
		return e, nil, fail
	}
	root, err := mappedRationalCoordinate(original, p[0], "CARTESIAN_POINT", 3)
	if err != nil {
		return e, nil, err
	}
	rootRadius, err := liftSourceRat(r.Args[2])
	if err != nil || rootRadius.Sign() <= 0 {
		return e, nil, fail
	}
	for _, u := range fc.loops[0] {
		edge := g.edges[u.id]
		if edge.start == edge.end {
			continue
		}
		if (e.SeamEdgeID != 0 && e.SeamEdgeID != u.id) || g.uses[u.id][0].face != face || g.uses[u.id][1].face != face {
			return e, nil, fail
		}
		e.SeamEdgeID, e.SeamCurveID = u.id, edge.curve
		e.SeamVertexIDs = [2]int64{edge.start, edge.end}
	}
	if e.SeamEdgeID == 0 {
		return e, nil, fail
	}
	var ends [2][3]*big.Rat
	var poles [2]int64
	var axial, radial [2]*big.Rat
	for i, id := range e.SeamVertexIDs {
		poles[i], ends[i], err = conicalRepairVertex(original, id)
		if err != nil || ends[i][1].Cmp(root[1]) != 0 {
			return e, nil, fail
		}
		radial[i] = new(big.Rat).Sub(ends[i][0], root[0])
		axial[i] = new(big.Rat).Sub(ends[i][2], root[2])
		axial[i].Mul(axial[i], axis[2])
		if radial[i].Sign() <= 0 {
			return e, nil, fail
		}
		for _, u := range fc.loops[0] {
			rim := g.edges[u.id]
			if rim.start == id && rim.end == id {
				if _, err := simple(original, rim.curve, "CIRCLE", 3); err != nil || e.RimEdgeIDs[i] != 0 {
					return e, nil, fail
				}
				e.RimEdgeIDs[i] = u.id
			}
		}
		if e.RimEdgeIDs[i] == 0 {
			return e, nil, fail
		}
		e.OriginalZStations[i], e.OriginalAxialStations[i] = ends[i][2].RatString(), axial[i].RatString()
	}
	dz := new(big.Rat).Sub(axial[1], axial[0])
	if dz.Sign() == 0 {
		return e, nil, fail
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(radial[1], radial[0]), dz)
	intercept := new(big.Rat).Sub(radial[0], new(big.Rat).Mul(slope, axial[0]))
	if slope.Sign() <= 0 || intercept.Cmp(rootRadius) != 0 {
		return e, nil, fail
	}
	angleDrift, err := annularChamferAngularDisplacement(r.Args[3], rootRadius, slope, axial)
	if err != nil {
		return e, nil, err
	}
	if angleDrift.Cmp(budget) > 0 {
		return e, nil, fmt.Errorf("%w: finite cone angle displacement exceeds source-unit limit", fail)
	}
	linePoles, lineEnds := poles, ends
	if !g.edges[e.SeamEdgeID].sense {
		linePoles[0], linePoles[1] = linePoles[1], linePoles[0]
		lineEnds[0], lineEnds[1] = lineEnds[1], lineEnds[0]
	}
	seamDrift, err := conicalRepairLineBound(original, e.SeamCurveID, lineEnds)
	if err != nil {
		return e, nil, fmt.Errorf("%w: original line direction: %v", fail, err)
	}
	if seamDrift.Cmp(budget) > 0 {
		return e, nil, fmt.Errorf("%w: finite line seam displacement exceeds source-unit limit", fail)
	}
	e.OriginalAngle, e.ExactSlope = r.Args[3].Text, slope.RatString()
	e.ConeDisplacementBound, e.SeamDisplacementBound = angleDrift.RatString(), seamDrift.RatString()
	line, _ := simple(original, e.SeamCurveID, "LINE", 3)

	// The support generator runs along the signed axis. Its direction can
	// differ from the original EDGE_CURVE, whose SameSense stays intact.
	genPoles := poles
	if dz.Sign() < 0 {
		genPoles[0], genPoles[1] = genPoles[1], genPoles[0]
	}
	e.CorrectedGeneratorID, err = addConicalRepairEntity(fixed, conicalRepairSegment(genPoles))
	if err != nil {
		return e, nil, err
	}
	pose, _ := simple(original, e.PlacementID, "AXIS2_PLACEMENT_3D", 4)
	ovalSet(fixed, e.PlacementID, Record{Type: "AXIS1_PLACEMENT", Args: pose.Args[:3]})
	segment := conicalRepairSegment(linePoles)
	segment.Args[0] = line.Args[0]
	ovalSet(fixed, e.SeamCurveID, segment)
	ovalSet(fixed, e.ConeID, Record{Type: "SURFACE_OF_REVOLUTION", Args: []Value{
		r.Args[0], {Kind: Reference, Ref: e.CorrectedGeneratorID}, {Kind: Reference, Ref: e.PlacementID},
	}})
	return e, []int64{p[2], line.Args[1].Ref, line.Args[2].Ref}, nil
}

func annularChamferAngularDisplacement(encoded Value, rootRadius, slope *big.Rat, heights [2]*big.Rat) (*big.Rat, error) {
	angle, err := liftSourceRat(encoded)
	if err != nil || angle.Sign() <= 0 || angle.Cmp(big.NewRat(1, 1)) >= 0 ||
		angle.Num().BitLen() > 256 || angle.Denom().BitLen() > 256 {
		return nil, errEccentricAnnularChamferRepair
	}
	sinLo, sinHi := conicalRepairTrigInterval(angle, 1)
	cosLo, cosHi := conicalRepairTrigInterval(angle, 0)
	lo := new(big.Rat).Quo(sinLo, cosHi)
	hi := new(big.Rat).Quo(sinHi, cosLo)
	for _, z := range heights {
		lowerSlope := lo
		if z.Sign() < 0 {
			lowerSlope = hi
		}
		lowerRadius := new(big.Rat).Add(rootRadius, new(big.Rat).Mul(z, lowerSlope))
		if lowerRadius.Sign() <= 0 {
			return nil, errEccentricAnnularChamferRepair
		}
	}
	// At equal signed height and azimuth the finite carriers differ by
	// abs(height*(tan(angle)-slope)). Its maximum is at an original rim.
	extent := conicalRepairMax(new(big.Rat).Abs(heights[0]), new(big.Rat).Abs(heights[1]))
	errorSlope := conicalRepairMax(conicalRepairDifference(lo, slope), conicalRepairDifference(hi, slope))
	return new(big.Rat).Mul(extent, errorSlope), nil
}

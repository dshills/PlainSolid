package step

import (
	"math/big"

	"plainsolid/geom"
)

// ConvexArcAxialTrimRepair identifies a source axial LINE with unequal
// endpoint corrections. The original parameter is preserved on its parallel
// support. These directed endpoint ranges identify the finite station shifts.
// The squared bounds are in source length units and remain invariant under
// the original proper rigid pose.
type ConvexArcAxialTrimRepair struct {
	EdgeID, LineID, OriginPointID int64
	VertexIDs                     [2]int64
	OriginalParameters            [2]*big.Rat
	RepairedParameters            [2]*big.Rat
	ProjectionDistanceSquared     [2]*big.Rat
	StationDistanceSquared        [2]*big.Rat
	SupportDistanceSquared        *big.Rat
}

func (q ConvexArcAxialTrimRepair) copied() ConvexArcAxialTrimRepair {
	for i := range q.VertexIDs {
		q.OriginalParameters[i] = new(big.Rat).Set(q.OriginalParameters[i])
		q.RepairedParameters[i] = new(big.Rat).Set(q.RepairedParameters[i])
		q.ProjectionDistanceSquared[i] = new(big.Rat).Set(q.ProjectionDistanceSquared[i])
		q.StationDistanceSquared[i] = new(big.Rat).Set(q.StationDistanceSquared[i])
	}
	q.SupportDistanceSquared = new(big.Rat).Set(q.SupportDistanceSquared)
	return q
}

// projectConvexArcLine projects a source point onto a proved affine model
// line. directionSquared is the positive squared physical parameter speed.
func projectConvexArcLine(point, origin, direction [3]*big.Rat, directionSquared *big.Rat) (parameter, residualSquared *big.Rat) {
	residual := occtLoopSub(point, origin)
	parameter = new(big.Rat).Quo(occtLoopDot(residual, direction), directionSquared)
	for j := range residual {
		residual[j].Sub(residual[j], new(big.Rat).Mul(parameter, direction[j]))
	}
	return parameter, occtLoopDot(residual, residual)
}

func proveConvexArcAxialTrim(f *File, edgeID int64, tangencies []ConvexArcTangentEvidence, targets [2][3]*big.Rat, tol2 *big.Rat) (ConvexArcAxialTrimRepair, [3]*big.Rat, [3]*big.Rat, error) {
	var empty ConvexArcAxialTrimRepair
	var noPoint [3]*big.Rat
	edge, e := simple(f, edgeID, "EDGE_CURVE", 5)
	if e != nil {
		return empty, noPoint, noPoint, e
	}
	line, e := simple(f, edge.Args[3].Ref, "LINE", 3)
	if e != nil {
		return empty, noPoint, noPoint, e
	}
	originRaw, directionRaw, e := convexUVLine(f, edge.Args[3].Ref, 3)
	if e != nil {
		return empty, noPoint, noPoint, e
	}
	origin, direction := [3]*big.Rat(originRaw), [3]*big.Rat(directionRaw)
	if direction[0].Sign() != 0 || direction[1].Sign() != 0 || direction[2].Sign() == 0 ||
		targets[0][0].Cmp(targets[1][0]) != 0 || targets[0][1].Cmp(targets[1][1]) != 0 {
		return empty, noPoint, noPoint, geom.ErrDomain
	}
	targetOrigin := [3]*big.Rat{targets[0][0], targets[0][1], origin[2]}
	shift := occtLoopSub(targetOrigin, origin)
	shiftSquared := occtLoopDot(shift, shift)
	if shiftSquared.Cmp(tol2) > 0 {
		return empty, noPoint, noPoint, geom.ErrDomain
	}
	out := ConvexArcAxialTrimRepair{
		EdgeID: edgeID, LineID: edge.Args[3].Ref, OriginPointID: line.Args[1].Ref,
		SupportDistanceSquared: shiftSquared,
		VertexIDs:              [2]int64{edge.Args[1].Ref, edge.Args[2].Ref},
	}
	var owners [2]ConvexArcTangentEvidence
	for _, q := range tangencies {
		for i, id := range out.VertexIDs {
			if q.VertexID == id {
				owners[i] = q
			}
		}
	}
	if owners[0].PlaneFaceID == 0 || owners[0].PlaneFaceID != owners[1].PlaneFaceID ||
		owners[0].CylinderFaceID != owners[1].CylinderFaceID || owners[0].CapFaceID == owners[1].CapFaceID {
		return empty, noPoint, noPoint, geom.ErrDomain
	}
	speedSquared := occtLoopDot(direction, direction)
	for i, q := range owners {
		if !convexUVSame(q.Point, targets[i]) {
			return empty, noPoint, noPoint, geom.ErrDomain
		}
		original, e := occtLoopTriple(f, q.PointID, "CARTESIAN_POINT")
		if e != nil {
			return empty, noPoint, noPoint, e
		}
		t, residualSquared := projectConvexArcLine(original, origin, direction, speedSquared)
		fixed, fixedResidual := projectConvexArcLine(targets[i], targetOrigin, direction, speedSquared)
		drift := new(big.Rat).Sub(t, fixed)
		stationSquared := new(big.Rat).Mul(new(big.Rat).Mul(drift, drift), speedSquared)
		// Linear correspondence across the two finite parameter intervals
		// has its maximum displacement at an endpoint. The station drift
		// and the parallel support shift are orthogonal.
		finiteDistance := new(big.Rat).Add(shiftSquared, stationSquared)
		if fixedResidual.Sign() != 0 || residualSquared.Cmp(tol2) > 0 || finiteDistance.Cmp(tol2) > 0 {
			return empty, noPoint, noPoint, geom.ErrDomain
		}
		out.OriginalParameters[i], out.RepairedParameters[i] = t, fixed
		out.ProjectionDistanceSquared[i], out.StationDistanceSquared[i] = residualSquared, stationSquared
	}
	sense, e := boolean(edge.Args[4])
	if e != nil || !convexArcDirectedLineInterval(out.OriginalParameters, sense) ||
		!convexArcDirectedLineInterval(out.RepairedParameters, sense) {
		return empty, noPoint, noPoint, geom.ErrDomain
	}
	return out, origin, targetOrigin, nil
}

func convexArcDirectedLineInterval(domain [2]*big.Rat, sense bool) bool {
	turn := domain[1].Cmp(domain[0])
	return turn != 0 && (turn > 0) == sense
}

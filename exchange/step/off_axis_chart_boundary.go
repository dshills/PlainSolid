package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// OffAxisCylinderChartBoundary proves a simple clockwise boundary in the
// unwrapped cylinder chart, CONDITIONAL on selecting the full rim domain and
// the floor/seam line segments between the reported chart corners.
// The two seam branches are assigned uniquely by source-exact UV corners and
// normalized coedge order. This is not proof of authored trim selection,
// periodic lifted closure, cylinder face coverage or material equivalence.
type OffAxisCylinderChartBoundary struct {
	BodyID, CylinderFaceID, RimEdgeID, FloorEdgeID            int64
	UStart, UEnd, FloorV, RimVLower                           *big.Rat
	SeamAssignments                                           [2]CylinderSeamAssignment // high-U then low-U
	PeriodicWidthDefect                                       [2]*big.Rat
	Orientation                                               int
	AssumesFullRimDomain, ChartJordan, PeriodicMismatchProven bool
	AssumesCornerSelectedSegments                             bool
	LiftedClosureProven                                       bool
}
type CylinderSeamAssignment struct {
	CoedgeID, EdgeID, PCurveID   int64
	Forward                      bool
	U                            *big.Rat
	StartParameter, EndParameter *big.Rat // signed traversal on source pcurve LINE
}

// CertifyOffAxisCylinderChartBoundary checks the bounded source chart graph.
// Its successful result is conditional, not an authored trim-selection proof.
func CertifyOffAxisCylinderChartBoundary(r io.Reader, l Limits, bodyID int64, endpointTolerance float64) (out OffAxisCylinderChartBoundary, err error) {
	fail := func() (OffAxisCylinderChartBoundary, error) { return OffAxisCylinderChartBoundary{}, geom.ErrDomain }
	if r == nil {
		return fail()
	}
	l, err = l.defaults()
	if err != nil {
		return out, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return out, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	top, err := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	pc, err := InspectOffAxisLinearPCurves(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	seam, err := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(raw), l, bodyID, endpointTolerance)
	if err != nil {
		return out, err
	}
	if !seam.OrderedCoverage {
		return fail()
	}

	floorLift, e := InspectOffAxisFloorCylinderLift(bytes.NewReader(raw), l, bodyID)
	if e != nil {
		return out, e
	}
	if !floorLift.SameParameterization || floorLift.AngleSlope.Sign() <= 0 {
		return fail()
	}
	rim := top.Intersection
	if !rim.IncreasingU || rim.DecreasingU {
		return fail()
	}
	_, poles, _, e := readResidualPolynomialDimension(bytes.NewReader(raw), l, rim.ParameterCurveID, 2)
	if e != nil {
		return out, e
	}
	bottom := pc.FloorCylinder.Origin[1]
	minimum := new(big.Rat).Set(poles[0][1])
	for _, p := range poles {
		if p[1].Cmp(bottom) <= 0 {
			return fail()
		}
		if p[1].Cmp(minimum) < 0 {
			minimum.Set(p[1])
		}
	}
	var face OffAxisTopologyFace
	found := false
	for _, f := range top.Faces {
		if f.SurfaceID == rim.CylinderID {
			face = f
			found = true
		}
	}
	if !found || face.SameSense || len(face.Bounds) != 1 || len(face.Bounds[0].Uses) != 4 {
		return fail()
	}
	uses := face.Bounds[0].Uses
	at := -1
	for i, u := range uses {
		if u.EdgeID == rim.EdgeID {
			if at >= 0 {
				return fail()
			}
			at = i
		}
	}
	if at < 0 {
		return fail()
	}
	ordered := make([]OffAxisTopologyUse, 4)
	for i := range ordered {
		ordered[i] = uses[(at+i)%4]
	}
	if !ordered[0].Forward || ordered[1].EdgeID != seam.EdgeID || ordered[1].Forward || ordered[3].EdgeID != seam.EdgeID || !ordered[3].Forward || ordered[2].Forward {
		return fail()
	}
	floorID := int64(0)
	for _, edge := range top.EdgeCurves {
		for _, id := range edge.PCurveIDs {
			if id == pc.FloorCylinder.PCurveID {
				floorID = edge.EdgeID
			}
		}
	}
	if floorID == 0 || ordered[2].EdgeID != floorID {
		return fail()
	}
	// Exact positive same-parameter floor lift binds decreasing chart U to
	// the source floor coedge's reversed model parameter direction.
	if pc.FloorCylinder.Direction[1].Sign() != 0 {
		return fail()
	}
	out = OffAxisCylinderChartBoundary{BodyID: bodyID, CylinderFaceID: face.FaceID, RimEdgeID: rim.EdgeID, FloorEdgeID: floorID, UStart: new(big.Rat).Set(rim.UStart), UEnd: new(big.Rat).Set(rim.UEnd), FloorV: new(big.Rat).Set(bottom), RimVLower: minimum, Orientation: -1, AssumesFullRimDomain: true, AssumesCornerSelectedSegments: true}
	for side, index := range []int{1, 3} {
		expectedU := rim.UEnd
		topV := poles[len(poles)-1][1]
		if side == 1 {
			expectedU = rim.UStart
			topV = poles[0][1]
		}
		branchIndex := -1
		for j, b := range pc.CylinderSeams {
			if b.Origin[0].Cmp(expectedU) == 0 {
				if branchIndex >= 0 {
					return fail()
				}
				branchIndex = j
			}
		}
		if branchIndex < 0 {
			return fail()
		}
		branch := pc.CylinderSeams[branchIndex]
		binding := seam.Branches[branchIndex]
		slope := new(big.Rat).Mul(branch.Direction[1], branch.Magnitude)
		parameter := func(v *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Sub(v, branch.Origin[1]), slope) }
		low, high := parameter(bottom), parameter(topV)
		projected := func(i int) *big.Rat {
			return new(big.Rat).Add(binding.ParameterOffset, new(big.Rat).Mul(binding.ParameterSlope, seam.ProjectionParameters[i]))
		}
		if low.Cmp(projected(0)) != 0 || high.Cmp(projected(1)) != 0 {
			return fail()
		}
		start, end := high, low
		if side == 1 {
			start, end = low, high
		}
		out.SeamAssignments[side] = CylinderSeamAssignment{CoedgeID: ordered[index].CoedgeID, EdgeID: seam.EdgeID, PCurveID: branch.PCurveID, Forward: ordered[index].Forward, U: new(big.Rat).Set(expectedU), StartParameter: start, EndParameter: end}
	}
	if out.SeamAssignments[0].PCurveID == out.SeamAssignments[1].PCurveID {
		return fail()
	}
	out.PeriodicWidthDefect = [2]*big.Rat{new(big.Rat).Set(rim.PositiveTurnDefect[0]), new(big.Rat).Set(rim.PositiveTurnDefect[1])}
	out.PeriodicMismatchProven = out.PeriodicWidthDefect[0].Sign() > 0 || out.PeriodicWidthDefect[1].Sign() < 0
	out.ChartJordan = true
	return out, nil
}

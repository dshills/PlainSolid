package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// FloorCylinderLiftEvidence retains the exact untrimmed relation
// U(t)=AngleOrigin+AngleSlope*t, V(t)=Height. SupportCircleEqual concerns point
// sets for all real t; SameParameterization additionally proves agreement with
// the model CIRCLE at the identical parameter t. No finite trim is inferred.
type FloorCylinderLiftEvidence struct {
	EdgeID, ModelCircleID, PCurveID, ParameterLineID, CylinderID int64
	AngleOrigin, AngleSlope, Height                              *big.Rat
	CenterDifference                                             [3]*big.Rat
	RadiusDifference                                             *big.Rat
	SupportCircleEqual, SameParameterization                     bool
	AuthoredTrimIntervalsKnown                                   bool
}

func InspectOffAxisFloorCylinderLift(r io.Reader, l Limits, bodyID int64) (out FloorCylinderLiftEvidence, err error) {
	if r == nil {
		return out, geom.ErrDomain
	}
	l, err = l.defaults()
	if err != nil {
		return
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return out, geom.ErrDomain
	}
	top, e := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if e != nil {
		return out, e
	}
	pc, e := InspectOffAxisLinearPCurves(bytes.NewReader(raw), l, bodyID)
	if e != nil {
		return out, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	out.CylinderID = top.Intersection.CylinderID
	out.PCurveID, out.ParameterLineID = pc.FloorCylinder.PCurveID, pc.FloorCylinder.CurveID
	for _, edge := range top.EdgeCurves {
		for _, id := range edge.PCurveIDs {
			if id == out.PCurveID {
				out.EdgeID, out.ModelCircleID = edge.EdgeID, edge.ModelCurveID
			}
		}
	}
	cyl, e := simple(f, out.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return out, e
	}
	id, e := ref(cyl.Args[1])
	if e != nil {
		return out, e
	}
	co, ca, e := sphereLiftFrame(f, id)
	if e != nil {
		return out, e
	}
	cr, e := liftSourceRat(cyl.Args[2])
	if e != nil {
		return out, e
	}
	circle, e := simple(f, out.ModelCircleID, "CIRCLE", 3)
	if e != nil {
		return out, e
	}
	id, e = ref(circle.Args[1])
	if e != nil {
		return out, e
	}
	mo, ma, e := sphereLiftFrame(f, id)
	if e != nil {
		return out, e
	}
	mr, e := liftSourceRat(circle.Args[2])
	if e != nil {
		return out, e
	}
	out.AngleOrigin = new(big.Rat).Set(pc.FloorCylinder.Origin[0])
	out.AngleSlope = new(big.Rat).Mul(pc.FloorCylinder.Direction[0], pc.FloorCylinder.Magnitude)
	out.Height = new(big.Rat).Set(pc.FloorCylinder.Origin[1])
	out.RadiusDifference = new(big.Rat).Sub(cr, mr)
	dot := new(big.Rat)
	for j := 0; j < 3; j++ {
		dot.Add(dot, new(big.Rat).Mul(ca[2][j], ma[2][j]))
		out.CenterDifference[j] = new(big.Rat).Sub(new(big.Rat).Add(co[j], new(big.Rat).Mul(out.Height, ca[2][j])), mo[j])
	}
	out.SupportCircleEqual = new(big.Rat).Abs(dot).Cmp(big.NewRat(1, 1)) == 0 && out.RadiusDifference.Sign() == 0
	for _, q := range out.CenterDifference {
		if q.Sign() != 0 {
			out.SupportCircleEqual = false
		}
	}
	// With rational authored phase, exact multiples of 2π are possible only at
	// zero. This conservative proof also requires matching zero-angle axes and
	// slope ±1, with the corresponding sine-axis orientation.
	out.SameParameterization = out.SupportCircleEqual && out.AngleOrigin.Sign() == 0 && new(big.Rat).Abs(out.AngleSlope).Cmp(big.NewRat(1, 1)) == 0
	for j := 0; j < 3; j++ {
		if ca[0][j].Cmp(ma[0][j]) != 0 || new(big.Rat).Mul(out.AngleSlope, ca[1][j]).Cmp(ma[1][j]) != 0 {
			out.SameParameterization = false
		}
	}
	return out, nil
}

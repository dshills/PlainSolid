package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// FloorCircleLiftEvidence compares C+A*cos(t)+B*sin(t) coefficients exactly.
// Differences are lifted planar pcurve minus authoritative model circle.
// IdenticalParameterization proves all real parameters, not selected edge trims.
type FloorCircleLiftEvidence struct {
	EdgeID, ModelCircleID, PCurveID, ParameterCircleID, PlaneID int64
	CenterDifference, CosineDifference, SineDifference          [3]*big.Rat
	IdenticalParameterization                                   bool
}

// InspectOffAxisFloorCircleLift supports signed-coordinate 3D frames and a
// represented exactly unit planar reference direction. It never normalizes or
// snaps source coefficients. A nonunit planar reference is unsupported.
func InspectOffAxisFloorCircleLift(r io.Reader, l Limits, bodyID int64) (out FloorCircleLiftEvidence, err error) {
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
	out.PCurveID, out.ParameterCircleID = pc.FloorPlanePCurveID, pc.FloorPlaneCircleID
	for _, edge := range top.EdgeCurves {
		for i, id := range edge.PCurveIDs {
			if id == out.PCurveID {
				out.EdgeID, out.ModelCircleID, out.PlaneID = edge.EdgeID, edge.ModelCurveID, edge.SupportIDs[i]
			}
		}
	}
	plane, e := simple(f, out.PlaneID, "PLANE", 2)
	if e != nil {
		return out, e
	}
	pid, e := ref(plane.Args[1])
	if e != nil {
		return out, e
	}
	po, pa, e := sphereLiftFrame(f, pid)
	if e != nil {
		return out, e
	}
	circle, e := simple(f, out.ModelCircleID, "CIRCLE", 3)
	if e != nil {
		return out, e
	}
	cid, e := ref(circle.Args[1])
	if e != nil {
		return out, e
	}
	co, ca, e := sphereLiftFrame(f, cid)
	if e != nil {
		return out, e
	}
	radius, e := liftSourceRat(circle.Args[2])
	if e != nil {
		return out, e
	}
	d := pc.FloorPlaneReference
	norm := new(big.Rat).Add(new(big.Rat).Mul(d[0], d[0]), new(big.Rat).Mul(d[1], d[1]))
	if norm.Cmp(big.NewRat(1, 1)) != 0 {
		return out, geom.ErrDomain
	}
	out.IdenticalParameterization = true
	for j := 0; j < 3; j++ {
		c := new(big.Rat).Set(po[j])
		for k := 0; k < 2; k++ {
			c.Add(c, new(big.Rat).Mul(pc.FloorPlaneCircleCenter[k], pa[k][j]))
		}
		a := new(big.Rat).Add(new(big.Rat).Mul(d[0], pa[0][j]), new(big.Rat).Mul(d[1], pa[1][j]))
		a.Mul(a, pc.FloorPlaneCircleRadius)
		b := new(big.Rat).Sub(new(big.Rat).Mul(d[0], pa[1][j]), new(big.Rat).Mul(d[1], pa[0][j]))
		b.Mul(b, pc.FloorPlaneCircleRadius)
		out.CenterDifference[j] = new(big.Rat).Sub(c, co[j])
		out.CosineDifference[j] = new(big.Rat).Sub(a, new(big.Rat).Mul(radius, ca[0][j]))
		out.SineDifference[j] = new(big.Rat).Sub(b, new(big.Rat).Mul(radius, ca[1][j]))
		if out.CenterDifference[j].Sign() != 0 || out.CosineDifference[j].Sign() != 0 || out.SineDifference[j].Sign() != 0 {
			out.IdenticalParameterization = false
		}
	}
	return out, nil
}

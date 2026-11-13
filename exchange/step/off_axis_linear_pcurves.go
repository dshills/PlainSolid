package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// SourceParameterLine describes an untrimmed STEP LINE in a parameter plane.
// Origin and Direction are raw source values; Direction is a unit coordinate axis.
// No finite parameter interval is authored by this representation.
type SourceParameterLine struct {
	PCurveID, CurveID int64
	Origin, Direction [2]*big.Rat
	Magnitude         *big.Rat
}

// OffAxisLinearPCurveEvidence preserves source pcurve semantics without choosing
// coedge branch parameters or inferring an edge's finite curve interval.
type OffAxisLinearPCurveEvidence struct {
	CylinderSeams                                                           [2]SourceParameterLine
	FloorCylinder                                                           SourceParameterLine
	FloorPlanePCurveID, FloorPlaneCircleID                                  int64
	FloorPlaneCircleCenter, FloorPlaneReference                             [2]*big.Rat
	FloorPlaneCircleRadius                                                  *big.Rat
	FloorVertexID                                                           int64
	FloorVertexCircleCoordinates                                            [3]*big.Rat
	FloorVertexZeroAngleSquaredGap                                          *big.Rat
	SeamAngleDifference, SeamAngleTurnDefectLower, SeamAngleTurnDefectUpper *big.Rat
	// LINE and CIRCLE records carry no explicit finite trim parameters.
	FloorCylinderHeightResidual, FloorPlaneRadiusResidual *big.Rat
	AuthoredTrimIntervalsKnown                            bool
}

// InspectOffAxisLinearPCurves reads the selected bore’s source parameter lines
// and planar floor circle without inferring unrecorded curve limits.
func InspectOffAxisLinearPCurves(r io.Reader, l Limits, bodyID int64) (out OffAxisLinearPCurveEvidence, err error) {
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
	if _, e = InspectOffAxisSimpleGeometry(bytes.NewReader(raw), l, bodyID); e != nil {
		return out, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	if !liftRadianContext(f, bodyID) {
		return out, geom.ErrDomain
	}
	coords := func(v Value, kind string, n int) ([]*big.Rat, error) {
		id, e := ref(v)
		if e != nil {
			return nil, e
		}
		p, e := simple(f, id, kind, 2)
		if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != n {
			return nil, geom.ErrDomain
		}
		a := make([]*big.Rat, n)
		for i := range a {
			a[i], e = liftSourceRat(p.Args[1].Items[i])
			if e != nil {
				return nil, e
			}
		}
		return a, nil
	}
	line := func(pc, id int64, axis int) (q SourceParameterLine, err error) {
		q.PCurveID, q.CurveID = pc, id
		p, e := simple(f, id, "LINE", 3)
		if e != nil {
			return q, e
		}
		o, e := coords(p.Args[1], "CARTESIAN_POINT", 2)
		if e != nil {
			return q, e
		}
		vid, e := ref(p.Args[2])
		if e != nil {
			return q, e
		}
		v, e := simple(f, vid, "VECTOR", 3)
		if e != nil {
			return q, e
		}
		d, e := coords(v.Args[1], "DIRECTION", 2)
		if e != nil {
			return q, e
		}
		q.Magnitude, e = liftSourceRat(v.Args[2])
		if e != nil || q.Magnitude.Sign() <= 0 {
			return q, geom.ErrDomain
		}
		for i := 0; i < 2; i++ {
			q.Origin[i], q.Direction[i] = o[i], d[i]
			if i != axis && d[i].Sign() != 0 {
				return q, geom.ErrDomain
			}
		}
		if new(big.Rat).Abs(d[axis]).Cmp(big.NewRat(1, 1)) != 0 {
			return q, geom.ErrDomain
		}
		return q, nil
	}
	for _, edge := range top.EdgeCurves {
		if edge.Seam && edge.SupportIDs[0] == top.Intersection.CylinderID {
			for i := 0; i < 2; i++ {
				out.CylinderSeams[i], e = line(edge.PCurveIDs[i], edge.ParameterCurveIDs[i], 1)
				if e != nil {
					return out, e
				}
			}
		}
		if edge.Seam || edge.EdgeID == top.Intersection.EdgeID {
			continue
		}
		for i := 0; i < 2; i++ {
			if edge.SupportIDs[i] == top.Intersection.CylinderID {
				out.FloorCylinder, e = line(edge.PCurveIDs[i], edge.ParameterCurveIDs[i], 0)
				if e != nil {
					return out, e
				}
			} else {
				out.FloorPlanePCurveID, out.FloorPlaneCircleID = edge.PCurveIDs[i], edge.ParameterCurveIDs[i]
				c, e := simple(f, edge.ParameterCurveIDs[i], "CIRCLE", 3)
				if e != nil {
					return out, e
				}
				out.FloorPlaneCircleRadius, e = liftSourceRat(c.Args[2])
				if e != nil || out.FloorPlaneCircleRadius.Sign() <= 0 {
					return out, geom.ErrDomain
				}
				id, e := ref(c.Args[1])
				if e != nil {
					return out, e
				}
				p, e := simple(f, id, "AXIS2_PLACEMENT_2D", 3)
				if e != nil {
					return out, e
				}
				o, e := coords(p.Args[1], "CARTESIAN_POINT", 2)
				if e != nil {
					return out, e
				}
				d, e := coords(p.Args[2], "DIRECTION", 2)
				if e != nil {
					return out, e
				}
				copy(out.FloorPlaneCircleCenter[:], o)
				copy(out.FloorPlaneReference[:], d)
				norm := new(big.Rat).Add(new(big.Rat).Mul(d[0], d[0]), new(big.Rat).Mul(d[1], d[1]))
				if norm.Sign() == 0 {
					return out, geom.ErrDomain
				}
			}
		}
		ec, e := simple(f, edge.EdgeID, "EDGE_CURVE", 5)
		if e != nil {
			return out, e
		}
		out.FloorVertexID, e = ref(ec.Args[1])
		if e != nil {
			return out, e
		}
		vp, e := simple(f, out.FloorVertexID, "VERTEX_POINT", 2)
		if e != nil {
			return out, e
		}
		p, e := coords(vp.Args[1], "CARTESIAN_POINT", 3)
		if e != nil {
			return out, e
		}
		c, e := simple(f, edge.ModelCurveID, "CIRCLE", 3)
		if e != nil {
			return out, e
		}
		id, _ := ref(c.Args[1])
		o, a, e := sphereLiftFrame(f, id)
		if e != nil {
			return out, e
		}
		radius, e := liftSourceRat(c.Args[2])
		if e != nil {
			return out, e
		}
		cyl, e := simple(f, top.Intersection.CylinderID, "CYLINDRICAL_SURFACE", 3)
		if e != nil {
			return out, e
		}
		cpid, _ := ref(cyl.Args[1])
		co, ca, e := sphereLiftFrame(f, cpid)
		if e != nil {
			return out, e
		}
		height := new(big.Rat)
		for j := 0; j < 3; j++ {
			height.Add(height, new(big.Rat).Mul(new(big.Rat).Sub(o[j], co[j]), ca[2][j]))
		}
		out.FloorCylinderHeightResidual = new(big.Rat).Sub(out.FloorCylinder.Origin[1], height)
		out.FloorPlaneRadiusResidual = new(big.Rat).Sub(out.FloorPlaneCircleRadius, radius)
		out.FloorVertexZeroAngleSquaredGap = new(big.Rat)
		for k := 0; k < 3; k++ {
			q := new(big.Rat)
			for j := 0; j < 3; j++ {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).Sub(p[j], o[j]), a[k][j]))
			}
			out.FloorVertexCircleCoordinates[k] = q
			delta := new(big.Rat).Set(q)
			if k == 0 {
				delta.Sub(delta, radius)
			}
			out.FloorVertexZeroAngleSquaredGap.Add(out.FloorVertexZeroAngleSquaredGap, new(big.Rat).Mul(delta, delta))
		}
	}
	if out.FloorPlaneCircleID == 0 || out.CylinderSeams[0].CurveID == 0 {
		return out, geom.ErrDomain
	}
	out.SeamAngleDifference = new(big.Rat).Sub(out.CylinderSeams[0].Origin[0], out.CylinderSeams[1].Origin[0])
	lo, hi := pcurvePiBounds()
	out.SeamAngleTurnDefectLower = new(big.Rat).Sub(out.SeamAngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), hi))
	out.SeamAngleTurnDefectUpper = new(big.Rat).Sub(out.SeamAngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), lo))
	return out, nil
}

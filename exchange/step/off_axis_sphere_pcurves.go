package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// SphericalSeamPCurveEvidence preserves two distinct untrimmed meridian lines.
// Branch order is SURFACE_CURVE association order, not coedge assignment.
type SphericalSeamPCurveEvidence struct {
	EdgeID, SphereID                                  int64
	Branches                                          [2]SourceParameterLine
	AngleDifference, TurnDefectLower, TurnDefectUpper *big.Rat
	VOriginDifference                                 *big.Rat
	SameVDirection, SameMagnitude                     bool
	AuthoredTrimIntervalsKnown                        bool
}

// InspectSphericalSeamPCurves requires constant-U, coordinate-axis V lines in
// a radian source context. It reports differences without selecting periodic
// representatives, reversing branches, or inventing finite trim parameters.
func InspectSphericalSeamPCurves(r io.Reader, l Limits, bodyID int64) (out SphericalSeamPCurveEvidence, err error) {
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
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	if !liftRadianContext(f, bodyID) {
		return out, geom.ErrDomain
	}
	for _, face := range top.Faces {
		if face.SurfaceKind == "SPHERICAL_SURFACE" {
			out.SphereID = face.SurfaceID
		}
	}
	coords := func(v Value, kind string) (a [2]*big.Rat, err error) {
		id, err := ref(v)
		if err != nil {
			return a, err
		}
		p, err := simple(f, id, kind, 2)
		if err != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
			return a, geom.ErrDomain
		}
		for i := range a {
			a[i], err = liftSourceRat(p.Args[1].Items[i])
			if err != nil {
				return a, err
			}
		}
		return
	}
	for _, edge := range top.EdgeCurves {
		if !edge.Seam || edge.SupportIDs[0] != out.SphereID {
			continue
		}
		out.EdgeID = edge.EdgeID
		for i := 0; i < 2; i++ {
			q := &out.Branches[i]
			q.PCurveID, q.CurveID = edge.PCurveIDs[i], edge.ParameterCurveIDs[i]
			line, e := simple(f, q.CurveID, "LINE", 3)
			if e != nil {
				return out, e
			}
			q.Origin, e = coords(line.Args[1], "CARTESIAN_POINT")
			if e != nil {
				return out, e
			}
			id, e := ref(line.Args[2])
			if e != nil {
				return out, e
			}
			v, e := simple(f, id, "VECTOR", 3)
			if e != nil {
				return out, e
			}
			q.Direction, e = coords(v.Args[1], "DIRECTION")
			if e != nil {
				return out, e
			}
			q.Magnitude, e = liftSourceRat(v.Args[2])
			if e != nil || q.Magnitude.Sign() <= 0 || q.Direction[0].Sign() != 0 || new(big.Rat).Abs(q.Direction[1]).Cmp(big.NewRat(1, 1)) != 0 {
				return out, geom.ErrDomain
			}
		}
	}
	if out.EdgeID == 0 {
		return out, geom.ErrDomain
	}
	a, b := out.Branches[0], out.Branches[1]
	out.AngleDifference = new(big.Rat).Sub(a.Origin[0], b.Origin[0])
	lo, hi := pcurvePiBounds()
	out.TurnDefectLower = new(big.Rat).Sub(out.AngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), hi))
	out.TurnDefectUpper = new(big.Rat).Sub(out.AngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), lo))
	out.VOriginDifference = new(big.Rat).Sub(a.Origin[1], b.Origin[1])
	out.SameVDirection = a.Direction[1].Cmp(b.Direction[1]) == 0
	out.SameMagnitude = a.Magnitude.Cmp(b.Magnitude) == 0
	return out, nil
}

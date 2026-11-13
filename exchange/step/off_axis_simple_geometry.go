package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// OffAxisSimpleGeometryEvidence reports exact source-decimal support residuals.
// Squared-radius residuals have length-squared units, not distance units.
// It does not certify pcurve lifts, trim intervals, face coverage or material.
type OffAxisSimpleGeometryEvidence struct {
	FloorEdgeID, CircleID, SeamEdgeID, LineID                    int64
	FloorCylinderSquaredRadiusResidual, FloorPlaneSignedDistance *big.Rat
	SeamCylinderSquaredRadiusResidual                            *big.Rat
	SeamEndpointLineSquaredDistances                             [2]*big.Rat
	SourceUncertainties                                          []SourceLengthUncertainty
}

// SourceLengthUncertainty preserves metadata without treating it as acceptance.
type SourceLengthUncertainty struct {
	EntityID, UnitID int64
	Value            *big.Rat
}

// InspectOffAxisSimpleGeometry requires signed-coordinate primitive frames,
// a circle coaxial with the cylinder and parallel to the floor, and a cylinder
// seam line parallel to its axis. Offsets and radius mismatches are reported
// exactly; unsupported nonparallel geometry is rejected rather than snapped.
func InspectOffAxisSimpleGeometry(r io.Reader, l Limits, bodyID int64) (out OffAxisSimpleGeometryEvidence, err error) {
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
	var planeID int64
	for _, face := range top.Faces {
		if face.SurfaceKind == "PLANE" {
			planeID = face.SurfaceID
		}
	}
	cyl, e := simple(f, top.Intersection.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return out, e
	}
	cp, _ := ref(cyl.Args[1])
	co, ca, e := sphereLiftFrame(f, cp)
	if e != nil {
		return out, e
	}
	radius, e := liftSourceRat(cyl.Args[2])
	if e != nil {
		return out, e
	}
	plane, e := simple(f, planeID, "PLANE", 2)
	if e != nil {
		return out, e
	}
	pp, _ := ref(plane.Args[1])
	po, pa, e := sphereLiftFrame(f, pp)
	if e != nil {
		return out, e
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, new(big.Rat).Mul(a[i], b[i]))
		}
		return v
	}
	sub := func(a, b [3]*big.Rat) (v [3]*big.Rat) {
		for i := range a {
			v[i] = new(big.Rat).Sub(a[i], b[i])
		}
		return
	}
	perp := func(a, axis [3]*big.Rat) *big.Rat {
		d := dot(a, axis)
		return new(big.Rat).Sub(dot(a, a), new(big.Rat).Mul(d, d))
	}
	parallel := func(a, b [3]*big.Rat) bool { return new(big.Rat).Abs(dot(a, b)).Cmp(big.NewRat(1, 1)) == 0 }
	for _, edge := range top.EdgeCurves {
		if edge.Seam && edge.SupportIDs[0] == top.Intersection.CylinderID {
			out.SeamEdgeID, out.LineID = edge.EdgeID, edge.ModelCurveID
			line, e := simple(f, edge.ModelCurveID, "LINE", 3)
			if e != nil {
				return out, e
			}
			point := func(v Value) (a [3]*big.Rat, err error) {
				id, err := ref(v)
				if err != nil {
					return a, err
				}
				p, err := simple(f, id, "CARTESIAN_POINT", 2)
				if err != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 3 {
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
			origin, e := point(line.Args[1])
			if e != nil {
				return out, e
			}
			vid, e := ref(line.Args[2])
			if e != nil {
				return out, e
			}
			v, e := simple(f, vid, "VECTOR", 3)
			if e != nil {
				return out, e
			}
			mag, e := liftSourceRat(v.Args[2])
			if e != nil || mag.Sign() <= 0 {
				return out, geom.ErrDomain
			}
			did, e := ref(v.Args[1])
			if e != nil {
				return out, e
			}
			d, e := simple(f, did, "DIRECTION", 2)
			if e != nil || d.Args[1].Kind != Aggregate || len(d.Args[1].Items) != 3 {
				return out, geom.ErrDomain
			}
			var dir [3]*big.Rat
			for i := range dir {
				dir[i], e = liftSourceRat(d.Args[1].Items[i])
				if e != nil {
					return out, e
				}
			}
			if !parallel(dir, ca[2]) || dot(dir, dir).Cmp(big.NewRat(1, 1)) != 0 {
				return out, geom.ErrDomain
			}
			out.SeamCylinderSquaredRadiusResidual = new(big.Rat).Sub(perp(sub(origin, co), ca[2]), new(big.Rat).Mul(radius, radius))
			ec, e := simple(f, edge.EdgeID, "EDGE_CURVE", 5)
			if e != nil {
				return out, e
			}
			for i := 0; i < 2; i++ {
				id, _ := ref(ec.Args[i+1])
				vp, e := simple(f, id, "VERTEX_POINT", 2)
				if e != nil {
					return out, e
				}
				p, e := point(vp.Args[1])
				if e != nil {
					return out, e
				}
				out.SeamEndpointLineSquaredDistances[i] = perp(sub(p, origin), dir)
			}
		} else if !edge.Seam && (edge.SupportIDs[0] == planeID || edge.SupportIDs[1] == planeID) {
			out.FloorEdgeID, out.CircleID = edge.EdgeID, edge.ModelCurveID
			circle, e := simple(f, edge.ModelCurveID, "CIRCLE", 3)
			if e != nil {
				return out, e
			}
			id, _ := ref(circle.Args[1])
			o, a, e := sphereLiftFrame(f, id)
			if e != nil {
				return out, e
			}
			rr, e := liftSourceRat(circle.Args[2])
			if e != nil || rr.Sign() <= 0 {
				return out, geom.ErrDomain
			}
			if !parallel(a[2], ca[2]) || !parallel(a[2], pa[2]) || perp(sub(o, co), ca[2]).Sign() != 0 {
				return out, geom.ErrDomain
			}
			out.FloorCylinderSquaredRadiusResidual = new(big.Rat).Sub(new(big.Rat).Mul(rr, rr), new(big.Rat).Mul(radius, radius))
			out.FloorPlaneSignedDistance = dot(sub(o, po), pa[2])
		}
	}
	if out.CircleID == 0 || out.LineID == 0 {
		return out, geom.ErrDomain
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "UNCERTAINTY_MEASURE_WITH_UNIT" {
			continue
		}
		v := en.Records[0].Args
		if len(v) != 4 || v[0].Kind != Typed || v[0].Text != "LENGTH_MEASURE" || len(v[0].Items) != 1 {
			return out, geom.ErrDomain
		}
		q, e := liftSourceRat(v[0].Items[0])
		if e != nil || q.Sign() < 0 {
			return out, geom.ErrDomain
		}
		u, e := ref(v[1])
		if e != nil {
			return out, e
		}
		out.SourceUncertainties = append(out.SourceUncertainties, SourceLengthUncertainty{id, u, q})
	}
	return out, nil
}

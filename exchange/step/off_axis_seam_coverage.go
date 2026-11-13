package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// OffAxisSeamCoverage describes all finite trims of the source cylinder seam
// LINE whose ordered endpoints lie within an explicit distance of their source
// vertices. ProjectionParameters exhibit one admissible choice. Every such
// trim covers GuaranteedCore and lies in PossibleEnvelope. The exact authored
// interval remains unknown. Parameter enclosures are conservative, not maximal.
type OffAxisSeamCoverage struct {
	EdgeID, LineID, CylinderID          int64
	VertexIDs                           [2]int64
	Uses                                []OffAxisTopologyUse
	ProjectionParameters                [2]*big.Rat
	EndpointParameterBounds             [2][2]*big.Rat
	EndpointLineSquaredDistances        [2]*big.Rat
	GuaranteedCore, PossibleEnvelope    [2]*big.Rat
	Branches                            [2]SeamBranchCoverage
	EndpointAdmissible, OrderedCoverage bool
	AuthoredTrimIntervalsKnown          bool
}

// SeamBranchCoverage maps the certified model parameter interval into each
// owner's LINE parameter using exact axial height only. Constant U is retained
// verbatim; angular lift equality and assignment to a coedge are NOT inferred.
type SeamBranchCoverage struct {
	PCurveID, CurveID                      int64
	Angle, ParameterOffset, ParameterSlope *big.Rat
	GuaranteedCore                         [2]*big.Rat
}

func BoundOffAxisCylinderSeamCoverage(r io.Reader, l Limits, bodyID int64, tolerance float64) (out OffAxisSeamCoverage, err error) {
	fail := func() (OffAxisSeamCoverage, error) { return OffAxisSeamCoverage{}, geom.ErrDomain }
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
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
	evidence, err := InspectOffAxisSimpleGeometry(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return out, err
	}
	point := func(v Value, kind string) (p [3]*big.Rat, e error) {
		id, e := ref(v)
		if e != nil {
			return p, e
		}
		rec, e := simple(f, id, kind, 2)
		if e != nil || rec.Args[1].Kind != Aggregate || len(rec.Args[1].Items) != 3 {
			return p, geom.ErrDomain
		}
		for i, x := range rec.Args[1].Items {
			p[i], e = liftSourceRat(x)
			if e != nil {
				return p, e
			}
		}
		return p, nil
	}
	line, e := simple(f, evidence.LineID, "LINE", 3)
	if e != nil {
		return out, e
	}
	origin, e := point(line.Args[1], "CARTESIAN_POINT")
	if e != nil {
		return out, e
	}
	vid, e := ref(line.Args[2])
	if e != nil {
		return out, e
	}
	vector, e := simple(f, vid, "VECTOR", 3)
	if e != nil {
		return out, e
	}
	dir, e := point(vector.Args[1], "DIRECTION")
	if e != nil {
		return out, e
	}
	mag, e := liftSourceRat(vector.Args[2])
	if e != nil || mag.Sign() <= 0 {
		return fail()
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
		return v
	}
	if dot(dir, dir).Cmp(big.NewRat(1, 1)) != 0 {
		return fail()
	}
	edge, e := simple(f, evidence.SeamEdgeID, "EDGE_CURVE", 5)
	if e != nil {
		return out, e
	}
	sense, e := boolean(edge.Args[4])
	if e != nil || !sense {
		return fail()
	}
	out.EdgeID, out.LineID, out.CylinderID = evidence.SeamEdgeID, evidence.LineID, top.Intersection.CylinderID
	tau := new(big.Rat).SetFloat64(tolerance)
	tau2 := new(big.Rat).Mul(tau, tau)
	pad := new(big.Rat).Quo(tau, mag)
	out.EndpointAdmissible = true
	for i := 0; i < 2; i++ {
		out.VertexIDs[i], e = ref(edge.Args[i+1])
		if e != nil {
			return fail()
		}
		v, e := simple(f, out.VertexIDs[i], "VERTEX_POINT", 2)
		if e != nil {
			return out, e
		}
		p, e := point(v.Args[1], "CARTESIAN_POINT")
		if e != nil {
			return out, e
		}
		out.ProjectionParameters[i] = new(big.Rat).Quo(dot(sub(p, origin), dir), mag)
		out.EndpointLineSquaredDistances[i] = new(big.Rat).Set(evidence.SeamEndpointLineSquaredDistances[i])
		if out.EndpointLineSquaredDistances[i].Cmp(tau2) > 0 {
			out.EndpointAdmissible = false
		}
		out.EndpointParameterBounds[i] = [2]*big.Rat{new(big.Rat).Sub(out.ProjectionParameters[i], pad), new(big.Rat).Add(out.ProjectionParameters[i], pad)}
	}
	for _, face := range top.Faces {
		for _, bound := range face.Bounds {
			for _, u := range bound.Uses {
				if u.EdgeID == out.EdgeID {
					if face.SurfaceID != out.CylinderID {
						return fail()
					}
					out.Uses = append(out.Uses, u)
				}
			}
		}
	}
	if len(out.Uses) != 2 || out.Uses[0].Forward == out.Uses[1].Forward {
		return fail()
	}
	// EDGE_CURVE same_sense=true requires increasing model LINE parameter.
	if out.ProjectionParameters[0].Cmp(out.ProjectionParameters[1]) >= 0 {
		return fail()
	}
	if !out.EndpointAdmissible {
		return out, nil
	}
	core := [2]*big.Rat{out.EndpointParameterBounds[0][1], out.EndpointParameterBounds[1][0]}
	if core[0].Cmp(core[1]) >= 0 {
		return out, nil
	}
	out.GuaranteedCore = core
	out.PossibleEnvelope = [2]*big.Rat{out.EndpointParameterBounds[0][0], out.EndpointParameterBounds[1][1]}
	out.OrderedCoverage = true
	cyl, e := simple(f, out.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	cid, e := ref(cyl.Args[1])
	if e != nil {
		return fail()
	}
	co, axes, e := sphereLiftFrame(f, cid)
	if e != nil {
		return out, e
	}
	h0 := dot(sub(origin, co), axes[2])
	hSlope := new(big.Rat).Mul(dot(dir, axes[2]), mag)
	if new(big.Rat).Abs(dot(dir, axes[2])).Cmp(big.NewRat(1, 1)) != 0 {
		return fail()
	}
	for i, branch := range pc.CylinderSeams {
		if branch.Direction[0].Sign() != 0 {
			return fail()
		}
		vSlope := new(big.Rat).Mul(branch.Direction[1], branch.Magnitude)
		if vSlope.Sign() == 0 {
			return fail()
		}
		offset := new(big.Rat).Quo(new(big.Rat).Sub(h0, branch.Origin[1]), vSlope)
		slope := new(big.Rat).Quo(hSlope, vSlope)
		limits := [2]*big.Rat{new(big.Rat).Add(offset, new(big.Rat).Mul(slope, core[0])), new(big.Rat).Add(offset, new(big.Rat).Mul(slope, core[1]))}
		if limits[0].Cmp(limits[1]) > 0 {
			limits[0], limits[1] = limits[1], limits[0]
		}
		for _, v := range []*big.Rat{offset, slope, limits[0], limits[1]} {
			if !sphereLiftBits(v) {
				return fail()
			}
		}
		out.Branches[i] = SeamBranchCoverage{PCurveID: branch.PCurveID, CurveID: branch.CurveID, Angle: new(big.Rat).Set(branch.Origin[0]), ParameterOffset: offset, ParameterSlope: slope, GuaranteedCore: limits}
	}
	return out, nil
}

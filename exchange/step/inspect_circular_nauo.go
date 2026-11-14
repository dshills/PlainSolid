package step

import (
	"io"
	"math/big"
)

// CircularNAUORimSite retains the original model circle, vertex and two owners.
// SquaredRadiusResidual is length squared; it is not a distance or tolerance.
type CircularNAUORimSite struct {
	EdgeID, CircleID, VertexID, CapFaceID        int64
	CylinderPCurveID, PlanePCurveID              int64
	Height, SquaredRadiusResidual, PlaneResidual *big.Rat
}

type CircularNAUOFailureKind string

const (
	CircularNAUOVertexOffSupport CircularNAUOFailureKind = "vertex-off-circular-support"
	CircularNAUOSeamOffSupport   CircularNAUOFailureKind = "seam-off-cylindrical-support"
	CircularNAUONonperiodicSeam  CircularNAUOFailureKind = "authored-seam-one-turn-mismatch"
)

type CircularNAUOFailedCondition struct {
	Kind     CircularNAUOFailureKind
	EntityID int64
}

// CircularNAUOInspection is source-bound evidence, not an imported solid.
// Rim pcurve full-support lift identities and closed directed edge incidence
// are checked, but finite parameter selection, repair and material are not proved.
// Even an empty FailedConditions slice is not a material acceptance certificate.
type CircularNAUOInspection struct {
	Product                                                   AssemblyProduct
	SourceBodyID, ShellID, CylinderID, SideFaceID, SeamEdgeID int64
	NAUOIDs                                                   []int64
	LengthUnit                                                float64
	Rims                                                      [2]CircularNAUORimSite // increasing original cylinder V
	SeamBranches                                              [2]SourceParameterLine // original association order
	SeamSquaredRadiusResidual                                 *big.Rat
	SeamAngleDifference, TurnDefectLower, TurnDefectUpper     *big.Rat
	FailedConditions                                          []CircularNAUOFailedCondition
}

func circularNAUOPCurve(f *File, id, owner int64) (int64, error) {
	p, e := simple(f, id, "PCURVE", 3)
	if e != nil || p.Args[1].Kind != Reference || p.Args[1].Ref != owner {
		return 0, mappedPlanarFailure()
	}
	rep, e := simple(f, p.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return 0, e
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return 0, mappedPlanarFailure()
	}
	if e = validatePCurveContext(f, rep.Args[2].Ref); e != nil {
		return 0, e
	}
	return ids[0], nil
}
func circularNAUOLine(f *File, pc, owner int64) (out SourceParameterLine, err error) {
	id, e := circularNAUOPCurve(f, pc, owner)
	if e != nil {
		return out, e
	}
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return out, e
	}
	o, e := mappedRationalCoordinate(f, r.Args[1].Ref, "CARTESIAN_POINT", 2)
	if e != nil {
		return out, e
	}
	v, e := simple(f, r.Args[2].Ref, "VECTOR", 3)
	if e != nil {
		return out, e
	}
	d, e := mappedRationalCoordinate(f, v.Args[1].Ref, "DIRECTION", 2)
	if e != nil {
		return out, e
	}
	m, e := liftSourceRat(v.Args[2])
	if e != nil || m.Cmp(big.NewRat(1, 1)) != 0 {
		return out, mappedPlanarFailure()
	}
	out = SourceParameterLine{PCurveID: pc, CurveID: id, Origin: [2]*big.Rat{o[0], o[1]}, Direction: [2]*big.Rat{d[0], d[1]}, Magnitude: m}
	return out, nil
}
func circularNAUOVertex(f *File, id int64) ([3]*big.Rat, error) {
	v, e := simple(f, id, "VERTEX_POINT", 2)
	if e != nil {
		return [3]*big.Rat{}, e
	}
	return mappedRationalCoordinate(f, v.Args[1].Ref, "CARTESIAN_POINT", 3)
}
func circularNAUOSub(a, b [3]*big.Rat) (v [3]*big.Rat) {
	for i := range v {
		v[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return
}
func circularNAUOEqual(a, b [3]*big.Rat) bool {
	for i := range a {
		if a[i].Cmp(b[i]) != 0 {
			return false
		}
	}
	return true
}

// InspectCircularNAUOProduct diagnoses a selected source-local three-face
// cylinder product in an otherwise structurally validated XCAF NAUO assembly.
// It requires signed coordinate support frames, exactly owned two circle rims,
// two cap planes and a twice-used line seam, with explicit original PCURVEs.
// Malformed ownership returns zero evidence; valid source discrepancies are
// reported as exact decimal residuals. No tolerance is applied and no geometry
// is reconstructed. Other product geometry is not certified by this inspection.
func InspectCircularNAUOProduct(r io.Reader, l Limits, productID int64) (CircularNAUOInspection, error) {
	fail := func() (CircularNAUOInspection, error) { return CircularNAUOInspection{}, mappedPlanarFailure() }
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	if lim.MaxFaces < 3 || lim.MaxVertices < 2 || lim.MaxEdgeUses < 6 {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return CircularNAUOInspection{}, e
	}
	f := a.Source
	if len(a.Products) > 8 || len(a.Occurrences) > 32 || len(f.Entities) > 8192 {
		return fail()
	}
	out := CircularNAUOInspection{LengthUnit: a.LengthUnit}
	for _, p := range a.Products {
		if p.DefinitionID == productID {
			out.Product = p
		}
	}
	if len(out.Product.Representations) != 1 {
		return fail()
	}
	if _, e = simple(f, out.Product.Representations[0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); e != nil {
		return fail()
	}
	ids, advanced, e := a.productItems(productID)
	if e != nil || len(ids) != 1 || !advanced[0] {
		return fail()
	}
	out.SourceBodyID = ids[0]
	if !liftRadianContext(f, ids[0]) {
		return fail()
	}
	closure, e := nauoClosure(f, ids, lim.MaxDepth)
	if e != nil {
		return fail()
	}
	for _, p := range a.Products {
		if p.DefinitionID != productID {
			others, _, e := a.productItems(p.DefinitionID)
			if e != nil {
				return fail()
			}
			other, e := nauoClosure(f, others, lim.MaxDepth)
			if e != nil {
				return fail()
			}
			for id := range other {
				if closure[id] {
					for _, rec := range f.Entities[id].Records {
						if planarSplineOwnedType(rec.Type) || rec.Type == "SEAM_CURVE" || rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION" || rec.Type == "VECTOR" || rec.Type == "AXIS2_PLACEMENT_3D" {
							return fail()
						}
					}
				}
			}
		}
	}
	for _, u := range a.Occurrences {
		if u.Child == productID {
			out.NAUOIDs = append(out.NAUOIDs, u.ID)
		}
	}
	if len(out.NAUOIDs) == 0 {
		return fail()
	}
	body, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	out.ShellID = body.Args[1].Ref
	shell, e := simple(f, out.ShellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 3)
	if e != nil || len(faces) != 3 {
		return fail()
	}
	type cap struct {
		id, surface int64
		record      Record
		uses        []analyticUse
	}
	caps := []cap{}
	var side []analyticUse
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		surface := f.Entities[face.Args[2].Ref]
		if len(surface.Records) != 1 {
			return fail()
		}
		switch surface.Records[0].Type {
		case "CYLINDRICAL_SURFACE":
			if out.CylinderID != 0 {
				return fail()
			}
			out.CylinderID = surface.ID
			out.SideFaceID = id
			face, side, e = analyticFaceLoop(f, id, 4)
			if e != nil {
				return fail()
			}
			sense, err := boolean(face.Args[3])
			if e != nil || err != nil || !sense {
				return fail()
			}
		case "PLANE":
			rec, uses, e := analyticFaceLoop(f, id, 1)
			if e != nil {
				return fail()
			}
			caps = append(caps, cap{id, surface.ID, rec, uses})
		default:
			return fail()
		}
	}
	if out.CylinderID == 0 || len(caps) != 2 {
		return fail()
	}
	cyl, e := simple(f, out.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	co, ca, e := sphereLiftFrame(f, cyl.Args[1].Ref)
	if e != nil {
		return fail()
	}
	radius, e := liftSourceRat(cyl.Args[2])
	if e != nil || radius.Sign() <= 0 {
		return fail()
	}
	r2 := new(big.Rat).Mul(radius, radius)
	owners := map[int64][]analyticUse{}
	for _, u := range side {
		owners[u.id] = append(owners[u.id], u)
	}
	if len(owners) != 3 {
		return fail()
	}
	for id, uses := range owners {
		if len(uses) == 2 {
			if out.SeamEdgeID != 0 || uses[0].sense == uses[1].sense {
				return fail()
			}
			out.SeamEdgeID = id
		} else if len(uses) != 1 {
			return fail()
		}
	}
	if out.SeamEdgeID == 0 {
		return fail()
	}
	rimVertices := map[int64]bool{}
	capSeen := map[int64]bool{}
	for i, c := range caps {
		u := c.uses[0]
		uses := owners[u.id]
		if len(uses) != 1 || uses[0].sense == u.sense || capSeen[u.id] {
			return fail()
		}
		capSeen[u.id] = true
		start, end, g, e := analyticEdge(f, u.id)
		if e != nil || start != end {
			return fail()
		}
		rimVertices[start] = true
		sc, e := simple(f, g, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
			return fail()
		}
		pc, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pc) != 2 || pc[0] == pc[1] {
			return fail()
		}
		circle, e := simple(f, sc.Args[1].Ref, "CIRCLE", 3)
		if e != nil {
			return fail()
		}
		o, axes, e := sphereLiftFrame(f, circle.Args[1].Ref)
		if e != nil {
			return fail()
		}
		cr, e := liftSourceRat(circle.Args[2])
		if e != nil || cr.Cmp(radius) != 0 || !circularNAUOEqual(axes[0], ca[0]) || !circularNAUOEqual(axes[2], ca[2]) {
			return fail()
		}
		delta := circularNAUOSub(o, co)
		height := mappedRationalDot(delta, ca[2])
		if mappedRationalDot(delta, ca[0]).Sign() != 0 || mappedRationalDot(delta, ca[1]).Sign() != 0 {
			return fail()
		}
		plane, e := simple(f, c.surface, "PLANE", 2)
		if e != nil {
			return fail()
		}
		po, pa, e := sphereLiftFrame(f, plane.Args[1].Ref)
		if e != nil || !circularNAUOEqual(po, o) || !circularNAUOEqual(pa[0], axes[0]) || !circularNAUOEqual(pa[2], axes[2]) {
			return fail()
		}
		line, e := circularNAUOLine(f, pc[0], out.CylinderID)
		if e != nil || line.Origin[0].Sign() != 0 || line.Origin[1].Cmp(height) != 0 || line.Direction[0].Cmp(big.NewRat(1, 1)) != 0 || line.Direction[1].Sign() != 0 {
			return fail()
		}
		pid, e := circularNAUOPCurve(f, pc[1], c.surface)
		if e != nil {
			return fail()
		}
		uv, e := simple(f, pid, "CIRCLE", 3)
		if e != nil {
			return fail()
		}
		ur, e := liftSourceRat(uv.Args[2])
		if e != nil || ur.Cmp(radius) != 0 {
			return fail()
		}
		frame, e := simple(f, uv.Args[1].Ref, "AXIS2_PLACEMENT_2D", 3)
		if e != nil {
			return fail()
		}
		uvo, e := mappedRationalCoordinate(f, frame.Args[1].Ref, "CARTESIAN_POINT", 2)
		if e != nil || uvo[0].Sign() != 0 || uvo[1].Sign() != 0 {
			return fail()
		}
		uvd, e := mappedRationalCoordinate(f, frame.Args[2].Ref, "DIRECTION", 2)
		if e != nil || uvd[0].Cmp(big.NewRat(1, 1)) != 0 || uvd[1].Sign() != 0 {
			return fail()
		}
		vertex, e := circularNAUOVertex(f, start)
		if e != nil {
			return fail()
		}
		v := circularNAUOSub(vertex, o)
		z := mappedRationalDot(v, axes[2])
		radial := new(big.Rat).Sub(mappedRationalDot(v, v), new(big.Rat).Mul(z, z))
		residual := new(big.Rat).Sub(radial, r2)
		out.Rims[i] = CircularNAUORimSite{EdgeID: u.id, CircleID: sc.Args[1].Ref, VertexID: start, CapFaceID: c.id, CylinderPCurveID: pc[0], PlanePCurveID: pc[1], Height: height, SquaredRadiusResidual: residual, PlaneResidual: z}
		if residual.Sign() != 0 || z.Sign() != 0 {
			out.FailedConditions = append(out.FailedConditions, CircularNAUOFailedCondition{CircularNAUOVertexOffSupport, start})
		}
	}
	if len(rimVertices) != 2 {
		return fail()
	}
	if out.Rims[0].Height.Cmp(out.Rims[1].Height) > 0 {
		out.Rims[0], out.Rims[1] = out.Rims[1], out.Rims[0]
	}
	if out.Rims[0].Height.Sign() != 0 || out.Rims[1].Height.Sign() <= 0 {
		return fail()
	}
	for i, rim := range out.Rims {
		face := f.Entities[rim.CapFaceID].Records[0]
		sense, e := boolean(face.Args[3])
		if e != nil || sense != (i == 1) {
			return fail()
		}
		u := owners[rim.EdgeID][0]
		if u.sense != (i == 0) {
			return fail()
		}
	}
	start, end, g, e := analyticEdge(f, out.SeamEdgeID)
	if e != nil || start != out.Rims[0].VertexID || end != out.Rims[1].VertexID {
		return fail()
	}
	sc, e := simple(f, g, "SEAM_CURVE", 4)
	if e != nil || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
		return fail()
	}
	pcs, e := listRefs(sc.Args[2], 2)
	if e != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
		return fail()
	}
	line, e := simple(f, sc.Args[1].Ref, "LINE", 3)
	if e != nil {
		return fail()
	}
	origin, e := mappedRationalCoordinate(f, line.Args[1].Ref, "CARTESIAN_POINT", 3)
	if e != nil {
		return fail()
	}
	vector, e := simple(f, line.Args[2].Ref, "VECTOR", 3)
	if e != nil {
		return fail()
	}
	direction, e := mappedRationalCoordinate(f, vector.Args[1].Ref, "DIRECTION", 3)
	if e != nil || !circularNAUOEqual(direction, ca[2]) {
		return fail()
	}
	mag, e := liftSourceRat(vector.Args[2])
	if e != nil || mag.Cmp(big.NewRat(1, 1)) != 0 {
		return fail()
	}
	low, e := circularNAUOVertex(f, start)
	if e != nil || !circularNAUOEqual(low, origin) {
		return fail()
	}
	high, e := circularNAUOVertex(f, end)
	if e != nil {
		return fail()
	}
	dv := circularNAUOSub(high, low)
	for j := range dv {
		if dv[j].Cmp(new(big.Rat).Mul(out.Rims[1].Height, ca[2][j])) != 0 {
			return fail()
		}
	}
	// Source coedge adjacency, including the twice-used seam, is checked by IDs.
	for i, u := range side {
		s, t, _, e := analyticEdge(f, u.id)
		if e != nil {
			return fail()
		}
		if !u.sense {
			s, t = t, s
		}
		next := side[(i+1)%len(side)]
		ns, nt, _, e := analyticEdge(f, next.id)
		if e != nil {
			return fail()
		}
		if !next.sense {
			ns = nt
		}
		if t != ns {
			return fail()
		}
		_ = s
	}
	delta := circularNAUOSub(origin, co)
	axial := mappedRationalDot(delta, ca[2])
	radial := new(big.Rat).Sub(mappedRationalDot(delta, delta), new(big.Rat).Mul(axial, axial))
	out.SeamSquaredRadiusResidual = new(big.Rat).Sub(radial, r2)
	if out.SeamSquaredRadiusResidual.Sign() != 0 {
		out.FailedConditions = append(out.FailedConditions, CircularNAUOFailedCondition{CircularNAUOSeamOffSupport, out.SeamEdgeID})
	}
	for i, pc := range pcs {
		q, e := circularNAUOLine(f, pc, out.CylinderID)
		if e != nil || q.Origin[1].Sign() != 0 || q.Direction[0].Sign() != 0 || q.Direction[1].Cmp(big.NewRat(1, 1)) != 0 {
			return fail()
		}
		out.SeamBranches[i] = q
	}
	if out.SeamBranches[1].Origin[0].Sign() != 0 || out.SeamBranches[0].Origin[0].Sign() <= 0 {
		return fail()
	}
	out.SeamAngleDifference = new(big.Rat).Sub(out.SeamBranches[0].Origin[0], out.SeamBranches[1].Origin[0])
	lo, hi := pcurvePiBounds()
	out.TurnDefectLower = new(big.Rat).Sub(out.SeamAngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), hi))
	out.TurnDefectUpper = new(big.Rat).Sub(out.SeamAngleDifference, new(big.Rat).Mul(big.NewRat(2, 1), lo))
	if out.TurnDefectLower.Sign() > 0 || out.TurnDefectUpper.Sign() < 0 {
		out.FailedConditions = append(out.FailedConditions, CircularNAUOFailedCondition{CircularNAUONonperiodicSeam, out.SeamEdgeID})
	}
	return out, nil
}

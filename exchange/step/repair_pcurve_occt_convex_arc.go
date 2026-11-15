package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ConvexArcPCurveAgreement bounds the original owner lift against its original
// model parameterization (with explicitly repaired axial line origin). The
// bound holds for all real parameters of these supported LINE/CIRCLE curves.
// It does not assert exact original finite trim closure.
type ConvexArcPCurveAgreement struct {
	EdgeID, SurfaceCurveID, PCurveID, FaceID, SupportID, GeometryID int64
	Kind                                                            string
	DistanceSquaredUpper                                            *big.Rat
}
type PCurveRepairedOCCTConvexArcExtrusion struct {
	source     string
	material   RepairedOCCTConvexArcExtrusion
	agreements []ConvexArcPCurveAgreement
	tolerance  float64
}

func (m PCurveRepairedOCCTConvexArcExtrusion) OriginalSTEP() string { return m.source }
func (m PCurveRepairedOCCTConvexArcExtrusion) CorrectedModelSTEP() string {
	return m.material.CorrectedSTEP()
}
func (m PCurveRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid { return m.material.Solid() }
func (m PCurveRepairedOCCTConvexArcExtrusion) Descriptor() ExactCircularLoopExtrusion {
	return m.material.Descriptor()
}
func (m PCurveRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	return m.material.Repairs()
}
func (m PCurveRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.material.Tangencies()
}
func (m PCurveRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.material.Provenance()
}
func (m PCurveRepairedOCCTConvexArcExtrusion) PCurves() []ConvexArcPCurveAgreement {
	out := append([]ConvexArcPCurveAgreement(nil), m.agreements...)
	for i := range out {
		out[i].DistanceSquaredUpper = new(big.Rat).Set(out[i].DistanceSquaredUpper)
	}
	return out
}

func convexUVVector(f *File, id int64, kind string, n int) ([]*big.Rat, error) {
	r, e := simple(f, id, kind, 2)
	if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != n {
		return nil, geom.ErrDomain
	}
	q := make([]*big.Rat, n)
	for i, v := range r.Args[1].Items {
		q[i], e = liftSourceRat(v)
		if e != nil || q[i].Num().BitLen() > 256 || q[i].Denom().BitLen() > 256 {
			return nil, geom.ErrDomain
		}
	}
	return q, nil
}
func convexUVUnit(q []*big.Rat) bool {
	s := new(big.Rat)
	for _, v := range q {
		s.Add(s, new(big.Rat).Mul(v, v))
	}
	return s.Cmp(big.NewRat(1, 1)) == 0
}
func convexUVLine(f *File, id int64, n int) (p, d []*big.Rat, e error) {
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return nil, nil, e
	}
	p, e = convexUVVector(f, r.Args[1].Ref, "CARTESIAN_POINT", n)
	if e != nil {
		return
	}
	v, e := simple(f, r.Args[2].Ref, "VECTOR", 3)
	if e != nil {
		return nil, nil, e
	}
	d, e = convexUVVector(f, v.Args[1].Ref, "DIRECTION", n)
	if e != nil || !convexUVUnit(d) {
		return nil, nil, geom.ErrDomain
	}
	mag, e := liftSourceRat(v.Args[2])
	if e != nil || mag.Sign() <= 0 {
		return nil, nil, geom.ErrDomain
	}
	for i := range d {
		d[i] = new(big.Rat).Mul(d[i], mag)
	}
	return
}
func convexUVCross(a, b [3]*big.Rat) (c [3]*big.Rat) {
	for i := range c {
		j, k := (i+1)%3, (i+2)%3
		c[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return
}
func convexUVFrame(f *File, id int64) (o, x, y, z [3]*big.Rat, e error) {
	o, z, x, e = occtLoopFrame(f, id)
	if e != nil {
		return
	}
	if !convexUVUnit(x[:]) || !convexUVUnit(z[:]) {
		e = geom.ErrDomain
		return
	}
	y = convexUVCross(z, x)
	return
}
func convexUVLift(o, x, y [3]*big.Rat, u, v *big.Rat) (p [3]*big.Rat) {
	for i := range p {
		p[i] = new(big.Rat).Add(o[i], new(big.Rat).Add(new(big.Rat).Mul(x[i], u), new(big.Rat).Mul(y[i], v)))
	}
	return
}
func convexUVSame(a, b [3]*big.Rat) bool {
	for i := range a {
		if a[i].Cmp(b[i]) != 0 {
			return false
		}
	}
	return true
}
func convexUVZero() (p [3]*big.Rat) {
	for i := range p {
		p[i] = new(big.Rat)
	}
	return
}

func boundConvexOwnerPCurve(f, corrected *File, model, geometry, support int64) (string, *big.Rat, error) {
	bad := func() (string, *big.Rat, error) { return "", nil, geom.ErrDomain }
	m := f.Entities[model]
	s := f.Entities[support]
	if len(m.Records) != 1 || len(s.Records) != 1 {
		return bad()
	}
	mr, sr := m.Records[0], s.Records[0]
	if (sr.Type != "PLANE" || len(sr.Args) != 2) && (sr.Type != "CYLINDRICAL_SURFACE" || len(sr.Args) != 3) {
		return bad()
	}
	o, x, y, z, e := convexUVFrame(f, sr.Args[1].Ref)
	if e != nil {
		return bad()
	}
	var radius *big.Rat
	if sr.Type == "CYLINDRICAL_SURFACE" {
		radius, e = liftSourceRat(sr.Args[2])
		if e != nil || radius.Sign() <= 0 {
			return bad()
		}
	}
	if mr.Type == "LINE" {
		p, d, e := convexUVLine(corrected, model, 3)
		if e != nil {
			return bad()
		}
		uv, vd, e := convexUVLine(f, geometry, 2)
		if e != nil {
			return bad()
		}
		mp, md := [3]*big.Rat{p[0], p[1], p[2]}, [3]*big.Rat{d[0], d[1], d[2]}
		if sr.Type == "PLANE" {
			lift := convexUVLift(o, x, y, uv[0], uv[1])
			direction := convexUVLift(convexUVZero(), x, y, vd[0], vd[1])
			if !convexUVSame(direction, md) {
				return bad()
			}
			delta := occtLoopSub(lift, mp)
			return "plane-line", occtLoopDot(delta, delta), nil
		}
		if vd[0].Sign() != 0 {
			return bad()
		}
		direction := convexUVZero()
		for i := range direction {
			direction[i].Mul(z[i], vd[1])
		}
		if !convexUVSame(direction, md) {
			return bad()
		}
		sine, cosine, remainder, e := liftTrigBernstein([]*big.Rat{uv[0], uv[0]})
		if e != nil {
			return bad()
		}
		sum := new(big.Rat)
		for i := range mp {
			approx := new(big.Rat).Add(o[i], new(big.Rat).Mul(z[i], uv[1]))
			approx.Add(approx, new(big.Rat).Mul(radius, new(big.Rat).Add(new(big.Rat).Mul(x[i], cosine[0]), new(big.Rat).Mul(y[i], sine[0]))))
			delta := new(big.Rat).Abs(new(big.Rat).Sub(approx, mp[i]))
			err := new(big.Rat).Mul(radius, new(big.Rat).Mul(remainder, new(big.Rat).Add(new(big.Rat).Abs(x[i]), new(big.Rat).Abs(y[i]))))
			delta.Add(delta, err)
			sum.Add(sum, new(big.Rat).Mul(delta, delta))
		}
		return "cylinder-line", sum, nil
	}
	if mr.Type != "CIRCLE" || len(mr.Args) != 3 {
		return bad()
	}
	mo, mx, my, _, e := convexUVFrame(f, mr.Args[1].Ref)
	if e != nil {
		return bad()
	}
	rad, e := liftSourceRat(mr.Args[2])
	if e != nil || rad.Sign() <= 0 {
		return bad()
	}
	if sr.Type == "CYLINDRICAL_SURFACE" {
		uv, vd, e := convexUVLine(f, geometry, 2)
		if e != nil || uv[0].Sign() != 0 || vd[0].Cmp(big.NewRat(1, 1)) != 0 || vd[1].Sign() != 0 || radius.Cmp(rad) != 0 || !convexUVSame(x, mx) || !convexUVSame(y, my) {
			return bad()
		}
		center := convexUVZero()
		for i := range center {
			center[i].Add(o[i], new(big.Rat).Mul(z[i], uv[1]))
		}
		delta := occtLoopSub(center, mo)
		return "cylinder-circle", occtLoopDot(delta, delta), nil
	}
	pc, e := simple(f, geometry, "CIRCLE", 3)
	if e != nil {
		return bad()
	}
	pr, e := liftSourceRat(pc.Args[2])
	if e != nil || pr.Cmp(rad) != 0 {
		return bad()
	}
	frame, e := simple(f, pc.Args[1].Ref, "AXIS2_PLACEMENT_2D", 3)
	if e != nil {
		return bad()
	}
	center, e := convexUVVector(f, frame.Args[1].Ref, "CARTESIAN_POINT", 2)
	if e != nil {
		return bad()
	}
	ref, e := convexUVVector(f, frame.Args[2].Ref, "DIRECTION", 2)
	if e != nil || !convexUVUnit(ref) {
		return bad()
	}
	lx := convexUVLift(convexUVZero(), x, y, ref[0], ref[1])
	ly := convexUVLift(convexUVZero(), x, y, new(big.Rat).Neg(ref[1]), ref[0])
	if !convexUVSame(lx, mx) || !convexUVSame(ly, my) {
		return bad()
	}
	delta := occtLoopSub(convexUVLift(o, x, y, center[0], center[1]), mo)
	return "plane-circle", occtLoopDot(delta, delta), nil
}

// RepairPCurveOCCTConvexArcExtrusion authenticates the actual default OCCT
// single-fillet graph and both authoritative owner pcurves. A model-only proof
// view is used only after every removed record is owned and every entire owner
// lift is bounded against the corrected model. Original pcurves remain in the
// retained source; output material is explicitly reconstructed model geometry.
func RepairPCurveOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (PCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero PCurveRepairedOCCTConvexArcExtrusion
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	view, agreements, e := convexPCurveModelView(f, 12, false)
	if e != nil {
		return zero, e
	}
	modelRaw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	material, e := RepairOCCTConvexArcExtrusion(ctx, bytes.NewReader(modelRaw), l, o)
	if e != nil {
		return zero, e
	}
	corrected, e := Parse(bytes.NewBufferString(material.CorrectedSTEP()), l)
	if e != nil {
		return zero, e
	}
	tol := new(big.Rat).SetFloat64(o.MaxDistance)
	tol.Mul(tol, tol)
	for i := range agreements {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		q := &agreements[i]
		sc, _ := simple(f, q.SurfaceCurveID, "SURFACE_CURVE", 4)
		q.Kind, q.DistanceSquaredUpper, e = boundConvexOwnerPCurve(f, corrected, sc.Args[1].Ref, q.GeometryID, q.SupportID)
		if e != nil || q.DistanceSquaredUpper.Cmp(tol) > 0 {
			return zero, geom.ErrDomain
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return PCurveRepairedOCCTConvexArcExtrusion{source: string(raw), material: material, agreements: agreements, tolerance: o.MaxDistance}, nil
}

func convexPCurveModelView(f *File, edgeCount int, rational bool) (*File, []ConvexArcPCurveAgreement, error) {
	owners := map[int64][]occtLoopFace{}
	refs := map[int64][]int64{}
	var collect func(int64, Value)
	collect = func(owner int64, v Value) {
		if v.Kind == Reference {
			refs[v.Ref] = append(refs[v.Ref], owner)
		}
		for _, q := range v.Items {
			collect(owner, q)
		}
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				collect(id, v)
			}
		}
		if _, err := simple(f, id, "ADVANCED_FACE", 4); err == nil {
			face, err := occtLoopReadFace(f, id)
			if err != nil {
				return nil, nil, err
			}
			for _, u := range face.uses {
				owners[u.id] = append(owners[u.id], face)
			}
		}
	}
	if len(owners) != edgeCount {
		return nil, nil, geom.ErrDomain
	}
	view := &File{Header: f.Header, Order: append([]int64(nil), f.Order...), Entities: make(map[int64]Entity, len(f.Entities))}
	for id, en := range f.Entities {
		view.Entities[id] = en
	}
	remove := map[int64]bool{}
	var agreements []ConvexArcPCurveAgreement
	for _, eid := range f.Order {
		faces, ok := owners[eid]
		if !ok {
			continue
		}
		if len(faces) != 2 || faces[0].id == faces[1].id || faces[0].surface == faces[1].surface {
			return nil, nil, geom.ErrDomain
		}
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil {
			return nil, nil, e
		}
		scid := edge.Args[3].Ref
		sc, e := simple(f, scid, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[0].Kind != String || sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" || len(refs[scid]) != 1 || refs[scid][0] != eid {
			return nil, nil, geom.ErrDomain
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
			return nil, nil, geom.ErrDomain
		}
		seen := map[int64]bool{}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil || pc.Args[0].Kind != String || len(refs[pcid]) != 1 || refs[pcid][0] != scid {
				return nil, nil, geom.ErrDomain
			}
			support := pc.Args[1].Ref
			var face int64
			for _, q := range faces {
				if q.surface == support {
					face = q.id
				}
			}
			if face == 0 || seen[face] {
				return nil, nil, geom.ErrDomain
			}
			seen[face] = true
			drid := pc.Args[2].Ref
			dr, e := simple(f, drid, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil || dr.Args[0].Kind != String || len(refs[drid]) != 1 || refs[drid][0] != pcid {
				return nil, nil, geom.ErrDomain
			}
			gs, e := listRefs(dr.Args[1], 1)
			if e != nil || len(gs) != 1 || len(refs[gs[0]]) != 1 || refs[gs[0]][0] != drid || validatePCurveContext(f, dr.Args[2].Ref) != nil {
				return nil, nil, geom.ErrDomain
			}
			closure, e := nauoClosure(f, []int64{gs[0], dr.Args[2].Ref}, 16)
			if e != nil {
				return nil, nil, e
			}
			for id := range closure {
				for _, rec := range f.Entities[id].Records {
					switch rec.Type {
					case "LINE", "CIRCLE", "AXIS2_PLACEMENT_2D", "CARTESIAN_POINT", "DIRECTION", "VECTOR":
						if len(rec.Args) == 0 || rec.Args[0].Kind != String {
							return nil, nil, geom.ErrDomain
						}
					case "BOUNDED_CURVE", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "CURVE", "GEOMETRIC_REPRESENTATION_ITEM", "RATIONAL_B_SPLINE_CURVE", "REPRESENTATION_ITEM":
						if !rational {
							return nil, nil, geom.ErrDomain
						}
					case "GEOMETRIC_REPRESENTATION_CONTEXT", "PARAMETRIC_REPRESENTATION_CONTEXT", "REPRESENTATION_CONTEXT":
					default:
						return nil, nil, geom.ErrDomain
					}
				}
				remove[id] = true
			}
			remove[pcid], remove[drid] = true, true
			agreements = append(agreements, ConvexArcPCurveAgreement{EdgeID: eid, SurfaceCurveID: scid, PCurveID: pcid, FaceID: face, SupportID: support, GeometryID: gs[0]})
		}
		remove[scid] = true
		edge.Args = append([]Value(nil), edge.Args...)
		edge.Args[3] = sc.Args[1]
		view.Entities[eid] = Entity{ID: eid, Records: []Record{edge}}
	}
	// Every discarded incoming reference is internal, except the one edge's
	// registered SURFACE_CURVE link replaced above. No original geometry is hidden.
	for id := range remove {
		for _, owner := range refs[id] {
			if remove[owner] {
				continue
			}
			ed, e := simple(f, owner, "EDGE_CURVE", 5)
			if e != nil || ed.Args[3].Ref != id {
				return nil, nil, geom.ErrDomain
			}
			if _, e = simple(f, id, "SURFACE_CURVE", 4); e != nil {
				return nil, nil, geom.ErrDomain
			}
		}
		delete(view.Entities, id)
	}
	view.Order = nil
	for _, id := range f.Order {
		if !remove[id] {
			view.Order = append(view.Order, id)
		}
	}
	return view, agreements, nil
}

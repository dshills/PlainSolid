package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SplineShellPCurveEvidence retains an original bounded two-pole UV spline and
// its unique owning model edge and spline surface.
type SplineShellPCurveEvidence struct {
	CurveID, PCurveID, SurfaceID, SurfaceCurveID, EdgeID int64
	PoleIDs                                              [2]int64
	Knots                                                [2]string
	Poles                                                [2][2]string
}

// NonplanarSplineShellSource preserves a single authenticated six-face curved
// rectangular spline material with original bounded spline pcurves.
type NonplanarSplineShellSource struct {
	Model                  SplineShellModel
	original               string
	bodyID                 int64
	faces, edges, surfaces []int64
	curves                 []SplineShellPCurveEvidence
}

func (m NonplanarSplineShellSource) OriginalSTEP() string { return m.original }
func (m NonplanarSplineShellSource) PCurves() []SplineShellPCurveEvidence {
	return append([]SplineShellPCurveEvidence(nil), m.curves...)
}
func (m NonplanarSplineShellSource) SourceFaces() []int64 { return append([]int64(nil), m.faces...) }
func (m NonplanarSplineShellSource) SourceEdges() []int64 { return append([]int64(nil), m.edges...) }
func noncoplanarSplinePoles(f *File, r Record) bool {
	var p [][3]*big.Rat
	for _, row := range r.Args[3].Items {
		for _, v := range row.Items {
			pr, e := simple(f, v.Ref, "CARTESIAN_POINT", 2)
			if e != nil || len(pr.Args[1].Items) != 3 {
				return false
			}
			var q [3]*big.Rat
			for j, x := range pr.Args[1].Items {
				q[j], e = affineSplineSourceRat(x)
				if e != nil {
					return false
				}
			}
			p = append(p, q)
		}
	}
	if len(p) < 4 {
		return false
	}
	sub := func(a, b [3]*big.Rat) (q [3]*big.Rat) {
		for j := range q {
			q[j] = new(big.Rat).Sub(a[j], b[j])
		}
		return
	}
	for i := 1; i < len(p); i++ {
		a := sub(p[i], p[0])
		for j := i + 1; j < len(p); j++ {
			n := mappedRationalCross(a, sub(p[j], p[0]))
			for k := j + 1; k < len(p); k++ {
				if mappedRationalDot(n, sub(p[k], p[0])).Sign() != 0 {
					return true
				}
			}
		}
	}
	return false
}
func ReadNonplanarSplineShellSource(ctx context.Context, r io.Reader, l Limits) (NonplanarSplineShellSource, error) {
	return readNonplanarSplineShellSource(ctx, r, l, 2)
}

func readNonplanarSplineShellSource(ctx context.Context, r io.Reader, l Limits, maxDegree int) (NonplanarSplineShellSource, error) {
	var zero NonplanarSplineShellSource
	fail := func() (NonplanarSplineShellSource, error) { return zero, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 2048)
	l.MaxValues = min(l.MaxValues, 32768)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(owner int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], owner)
		}
		for _, q := range v.Items {
			walk(owner, q)
		}
	}
	var maxID int64
	for id, en := range f.Entities {
		maxID = max(maxID, id)
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	one := func(id int64, kind string, n int) (int64, Record, error) {
		if len(incoming[id]) != 1 {
			return 0, Record{}, geom.ErrInvalid
		}
		owner := incoming[id][0]
		r, e := simple(f, owner, kind, n)
		return owner, r, e
	}
	m := NonplanarSplineShellSource{original: string(raw)}
	var roots []int64
	nonplanar := false
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, r := range en.Records {
			switch r.Type {
			case "ADVANCED_BREP_SHAPE_REPRESENTATION":
				roots = append(roots, id)
			case "MANIFOLD_SOLID_BREP":
				if m.bodyID != 0 {
					return fail()
				}
				m.bodyID = id
			case "ADVANCED_FACE":
				m.faces = append(m.faces, id)
			case "EDGE_CURVE":
				m.edges = append(m.edges, id)
			case "CARTESIAN_POINT":
				if len(r.Args) != 2 || r.Args[1].Kind != Aggregate {
					return fail()
				}
				for _, v := range r.Args[1].Items {
					if _, e = wrappedIsoSourceNumber(v); e != nil {
						return fail()
					}
				}
			case "B_SPLINE_SURFACE_WITH_KNOTS":
				if len(en.Records) != 1 || len(r.Args) != 13 || r.Args[3].Kind != Aggregate || len(r.Args[3].Items) > maxDegree+1 {
					return fail()
				}
				for _, row := range r.Args[3].Items {
					if row.Kind != Aggregate || len(row.Items) > maxDegree+1 {
						return fail()
					}
				}
				for _, index := range []int{1, 2} {
					q, e := affineSplineSourceRat(r.Args[index])
					if e != nil || !q.IsInt() || q.Cmp(big.NewRat(1, 1)) < 0 || q.Cmp(big.NewRat(int64(maxDegree), 1)) > 0 {
						return fail()
					}
				}
				for _, index := range []int{10, 11} {
					for _, v := range r.Args[index].Items {
						if _, e = wrappedIsoSourceNumber(v); e != nil {
							return fail()
						}
					}
				}
				nonplanar = nonplanar || noncoplanarSplinePoles(f, r)
				m.surfaces = append(m.surfaces, id)
			case "B_SPLINE_CURVE_WITH_KNOTS":
				if len(en.Records) != 1 || len(r.Args) != 9 {
					return fail()
				}
				for _, v := range r.Args[7].Items {
					if _, e = wrappedIsoSourceNumber(v); e != nil {
						return fail()
					}
				}
			case "RATIONAL_B_SPLINE_CURVE", "RATIONAL_B_SPLINE_SURFACE", "MAPPED_ITEM", "REPRESENTATION_RELATIONSHIP", "ITEM_DEFINED_TRANSFORMATION":
				return fail()
			}
		}
	}
	if !nonplanar || len(roots) != 1 || m.bodyID == 0 || len(m.faces) != 6 || len(m.surfaces) != 6 || len(m.edges) != 12 {
		return fail()
	}
	m.faces, e = bodyFaceIDs(f, m.bodyID)
	if e != nil || len(m.faces) != 6 {
		return fail()
	}
	m.surfaces = nil
	for _, fid := range m.faces {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil || face.Args[2].Kind != Reference {
			return fail()
		}
		m.surfaces = append(m.surfaces, face.Args[2].Ref)
	}
	closure, e := nauoClosure(f, roots, 32)
	if e != nil {
		return fail()
	}
	geometry := map[string]bool{}
	for _, k := range []string{"CARTESIAN_POINT", "DIRECTION", "AXIS2_PLACEMENT_3D", "B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_SURFACE_WITH_KNOTS", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "EDGE_CURVE", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "PCURVE", "SURFACE_CURVE", "VERTEX_POINT", "DEFINITIONAL_REPRESENTATION"} {
		geometry[k] = true
	}
	metadata := map[string]bool{}
	for _, kind := range []string{"ADVANCED_BREP_SHAPE_REPRESENTATION", "APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION_SHAPE", "PRODUCT_RELATED_PRODUCT_CATEGORY", "SHAPE_DEFINITION_REPRESENTATION", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "LENGTH_UNIT", "NAMED_UNIT", "PARAMETRIC_REPRESENTATION_CONTEXT", "PLANE_ANGLE_UNIT", "REPRESENTATION_CONTEXT", "SI_UNIT", "SOLID_ANGLE_UNIT", "UNCERTAINTY_MEASURE_WITH_UNIT"} {
		metadata[kind] = true
	}
	for _, en := range f.Entities {
		for _, r := range en.Records {
			if !geometry[r.Type] && !metadata[r.Type] {
				return fail()
			}
			var integers []Value
			if r.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
				integers = append(integers, r.Args[1])
				integers = append(integers, r.Args[6].Items...)
			}
			if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
				integers = append(integers, r.Args[1], r.Args[2])
				integers = append(integers, r.Args[8].Items...)
				integers = append(integers, r.Args[9].Items...)
			}
			for _, v := range integers {
				q, e := affineSplineSourceRat(v)
				if e != nil || !q.IsInt() || q.Sign() <= 0 || q.Cmp(big.NewRat(int64(maxDegree+1), 1)) > 0 {
					return fail()
				}
			}
		}
	}
	for id, en := range f.Entities {
		for _, r := range en.Records {
			if geometry[r.Type] && !closure[id] {
				return fail()
			}
		}
	}
	removed := map[int64]bool{}
	for _, id := range f.Order {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			continue
		}
		rid, e := ref(pc.Args[2])
		if e != nil {
			return fail()
		}
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ids, e := listRefs(rep.Args[1], 1)
		if e != nil || len(ids) != 1 {
			return fail()
		}
		cid := ids[0]
		c, e := simple(f, cid, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return fail()
		}
		pids, e := listRefs(c.Args[2], 2)
		if e != nil || len(pids) != 2 {
			return fail()
		}
		own, _, e := one(cid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil || own != rid {
			return fail()
		}
		own, _, e = one(rid, "PCURVE", 3)
		if e != nil || own != id {
			return fail()
		}
		scid, sc, e := one(id, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		eid, _, e := one(scid, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		ev := SplineShellPCurveEvidence{CurveID: cid, PCurveID: id, SurfaceID: pc.Args[1].Ref, SurfaceCurveID: scid, EdgeID: eid, PoleIDs: [2]int64{pids[0], pids[1]}}
		q, e := affineSplineSourceRat(c.Args[1])
		if e != nil || q.Cmp(big.NewRat(1, 1)) != 0 {
			return fail()
		}
		if len(c.Args[7].Items) != 2 {
			return fail()
		}
		for j, pid := range pids {
			if len(incoming[pid]) != 1 || incoming[pid][0] != cid {
				return fail()
			}
			pr, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || len(pr.Args[1].Items) != 2 {
				return fail()
			}
			for k, v := range pr.Args[1].Items {
				q, e := affineSplineSourceRat(v)
				if e != nil {
					return fail()
				}
				ev.Poles[j][k] = q.RatString()
			}
			q, e := affineSplineSourceRat(c.Args[7].Items[j])
			if e != nil {
				return fail()
			}
			ev.Knots[j] = q.RatString()
			removed[pid] = true
		}
		model, e := DecodeBSplineCurve(f, sc.Args[1].Ref, 64)
		if e != nil {
			return fail()
		}
		lo, hi := model.Geometry.Domain()
		if new(big.Rat).SetFloat64(lo).RatString() != ev.Knots[0] || new(big.Rat).SetFloat64(hi).RatString() != ev.Knots[1] {
			return fail()
		}
		m.curves = append(m.curves, ev)
	}
	if len(m.curves) != 24 {
		return fail()
	}
	if e = normalizeAffineIsoPCurves(f, 64); e != nil {
		return zero, e
	}
	// Rebase helper-private generated records for the strict parser proof view.
	aliases := map[int64]int64{}
	var negative []int64
	for id := range f.Entities {
		if id < 0 {
			negative = append(negative, id)
		}
	}
	sort.Slice(negative, func(i, j int) bool { return negative[i] > negative[j] })
	for _, id := range negative {
		maxID++
		aliases[id] = maxID
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if id, ok := aliases[v.Ref]; v.Kind == Reference && ok {
			v.Ref = id
		}
		for i := range v.Items {
			v.Items[i] = rewrite(v.Items[i])
		}
		return v
	}
	for id, en := range f.Entities {
		for i := range en.Records {
			for j := range en.Records[i].Args {
				en.Records[i].Args[j] = rewrite(en.Records[i].Args[j])
			}
		}
		f.Entities[id] = en
	}
	for _, id := range negative {
		en := f.Entities[id]
		en.ID = aliases[id]
		f.Entities[en.ID] = en
		delete(f.Entities, id)
		f.Order = append(f.Order, en.ID)
	}
	order := f.Order[:0]
	for _, id := range f.Order {
		if removed[id] {
			delete(f.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	f.Order = order
	data, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	m.Model, e = ReadSplineShells(bytes.NewReader(data), l)
	if e != nil {
		return zero, e
	}
	if len(m.Model.Bodies) != 1 || len(m.Model.Bodies[0].Faces) != 6 {
		return fail()
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}

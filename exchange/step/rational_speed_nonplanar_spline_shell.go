package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// RationalSplineSpeedEvidence proves the original rational Bezier curve is its
// unchanged polynomial control curve composed with phi(t)=Ratio*t/(1+(Ratio-1)*t).
// Degree+1 entries are active. PCurveID is zero for a 3D model curve.
type RationalSplineSpeedEvidence struct {
	CurveID, SurfaceCurveID, EdgeID, PCurveID, SurfaceID int64
	Degree, Dimension                                    int
	PoleIDs                                              [3]int64
	Poles                                                [3][3]string
	Weights                                              [3]string
	Ratio                                                string
}

// RationalSpeedNonplanarSplineShellSource preserves original rational boundary
// parameterizations and a separately certified polynomial-speed native material.
// It does not claim that original and native curve parameter values are equal.
type RationalSpeedNonplanarSplineShellSource struct {
	Model                  SplineShellModel
	original               string
	bodyID                 int64
	faces, edges, surfaces []int64
	curves                 []SplineShellPCurveEvidence
	maps                   []RationalSplineSpeedEvidence
}

func (m RationalSpeedNonplanarSplineShellSource) OriginalSTEP() string { return m.original }
func (m RationalSpeedNonplanarSplineShellSource) SourceFaces() []int64 {
	return append([]int64(nil), m.faces...)
}
func (m RationalSpeedNonplanarSplineShellSource) SourceEdges() []int64 {
	return append([]int64(nil), m.edges...)
}

// PCurves returns normalized endpoint lines with original owner IDs.
// ParameterMaps retains the actual rational source basis and speed.
func (m RationalSpeedNonplanarSplineShellSource) PCurves() []SplineShellPCurveEvidence {
	return append([]SplineShellPCurveEvidence(nil), m.curves...)
}
func (m RationalSpeedNonplanarSplineShellSource) ParameterMaps() []RationalSplineSpeedEvidence {
	return append([]RationalSplineSpeedEvidence(nil), m.maps...)
}

// ReadRationalSpeedNonplanarSplineShellSource accepts a common positive Mobius
// speed per model edge and its two owner pcurves. All 36 original curves must
// be clamped single-span rational Bezier curves on [0,1], model degree 1..2,
// UV degree one, with geometric-progression weights. At least one ratio must
// differ from one. Exact positivity proves an increasing endpoint-fixing
// bijection, preserving directed intervals and the certified native material.
func ReadRationalSpeedNonplanarSplineShellSource(ctx context.Context, r io.Reader, l Limits) (RationalSpeedNonplanarSplineShellSource, error) {
	var zero RationalSpeedNonplanarSplineShellSource
	fail := func() (RationalSpeedNonplanarSplineShellSource, error) { return zero, geom.ErrInvalid }
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
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, x := range v.Items {
			walk(id, x)
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				walk(id, v)
			}
		}
	}
	one := func(id int64, kind string, n int) (int64, Record, error) {
		if len(incoming[id]) != 1 {
			return 0, Record{}, geom.ErrInvalid
		}
		id = incoming[id][0]
		q, e := simple(f, id, kind, n)
		return id, q, e
	}
	m := RationalSpeedNonplanarSplineShellSource{original: string(raw)}
	byCurve := map[int64]RationalSplineSpeedEvidence{}
	nonlinear := false
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		en := f.Entities[id]
		rational := false
		for _, rec := range en.Records {
			rational = rational || rec.Type == "RATIONAL_B_SPLINE_CURVE"
		}
		if !rational {
			continue
		}
		if len(m.maps) >= 36 {
			return fail()
		}
		records := map[string]Record{}
		for _, rec := range en.Records {
			if _, ok := records[rec.Type]; ok {
				return fail()
			}
			records[rec.Type] = rec
		}
		allowed := map[string]int{"BOUNDED_CURVE": 0, "B_SPLINE_CURVE": 5, "B_SPLINE_CURVE_WITH_KNOTS": 3, "CURVE": 0, "GEOMETRIC_REPRESENTATION_ITEM": 0, "RATIONAL_B_SPLINE_CURVE": 1, "REPRESENTATION_ITEM": 1}
		if len(records) != len(allowed) {
			return fail()
		}
		for k, n := range allowed {
			q, ok := records[k]
			if !ok || len(q.Args) != n {
				return fail()
			}
		}
		b, k, w, name := records["B_SPLINE_CURVE"], records["B_SPLINE_CURVE_WITH_KNOTS"], records["RATIONAL_B_SPLINE_CURVE"], records["REPRESENTATION_ITEM"]
		if b.Args[2].Kind != Enumeration || b.Args[2].Text != "UNSPECIFIED" {
			return fail()
		}
		for _, index := range []int{3, 4} {
			flag, e := boolean(b.Args[index])
			if e != nil || flag {
				return fail()
			}
		}
		d, e := affineSplineSourceRat(b.Args[0])
		if e != nil || !d.IsInt() || d.Cmp(big.NewRat(1, 1)) < 0 || d.Cmp(big.NewRat(2, 1)) > 0 || name.Args[0].Kind != String {
			return fail()
		}
		degree := int(d.Num().Int64())
		ids, e := listRefs(b.Args[1], 2)
		if e != nil || len(ids) != degree+1 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != len(ids) {
			return fail()
		}
		knots, _, _, e := affineSplineGreville(k.Args[0], k.Args[1], degree, len(ids))
		if e != nil || len(k.Args[1].Items) != 2 || knots[0] != "0" || knots[1] != "1" || !affineSplineKnotSpec(k.Args[2], k.Args[0], k.Args[1], degree) {
			return fail()
		}
		ev := RationalSplineSpeedEvidence{CurveID: id, Degree: degree}
		copy(ev.PoleIDs[:], ids)
		var weights [3]*big.Rat
		for i, pid := range ids {
			if len(incoming[pid]) != 1 || incoming[pid][0] != id {
				return fail()
			}
			p, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate {
				return fail()
			}
			dim := len(p.Args[1].Items)
			if dim < 2 || dim > 3 || (i > 0 && ev.Dimension != dim) {
				return fail()
			}
			ev.Dimension = dim
			for j, v := range p.Args[1].Items {
				if _, e := wrappedIsoSourceNumber(v); e != nil {
					return fail()
				}
				q, e := affineSplineSourceRat(v)
				if e != nil {
					return fail()
				}
				ev.Poles[i][j] = q.RatString()
			}
			weights[i], e = affineSplineSourceRat(w.Args[0].Items[i])
			if e != nil || weights[i].Sign() <= 0 {
				return fail()
			}
			ev.Weights[i] = weights[i].RatString()
		}
		ratio := new(big.Rat).Quo(weights[1], weights[0])
		ev.Ratio = ratio.RatString()
		nonlinear = nonlinear || ratio.Cmp(big.NewRat(1, 1)) != 0
		for i := 1; i < len(ids); i++ {
			want := new(big.Rat).Mul(weights[i-1], ratio)
			if want.Num().BitLen() > 1024 || want.Denom().BitLen() > 1024 || want.Cmp(weights[i]) != 0 {
				return fail()
			}
		}
		if ev.Dimension == 2 {
			if degree != 1 {
				return fail()
			}
			rid, rep, e := one(id, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			rids, e := listRefs(rep.Args[1], 1)
			if e != nil || len(rids) != 1 || rids[0] != id {
				return fail()
			}
			pcid, pc, e := one(rid, "PCURVE", 3)
			if e != nil || pc.Args[1].Kind != Reference {
				return fail()
			}
			ev.PCurveID = pcid
			ev.SurfaceID = pc.Args[1].Ref
			ev.SurfaceCurveID, _, e = one(pcid, "SURFACE_CURVE", 4)
			if e != nil {
				return fail()
			}
		} else {
			ev.SurfaceCurveID, _, e = one(id, "SURFACE_CURVE", 4)
			if e != nil {
				return fail()
			}
		}
		ev.EdgeID, _, e = one(ev.SurfaceCurveID, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		byCurve[id] = ev
		m.maps = append(m.maps, ev)
		// The Bernstein substitution identity is exact: B_i^n(phi(t)) =
		// r^i B_i^n(t)/(1+(r-1)t)^n. The unchanged poles define the private
		// polynomial-speed proof curve; no original source entity is discarded.
		en.Records = []Record{{Type: "B_SPLINE_CURVE_WITH_KNOTS", Args: []Value{name.Args[0], b.Args[0], b.Args[1], b.Args[2], b.Args[3], b.Args[4], k.Args[0], k.Args[1], k.Args[2]}}}
		f.Entities[id] = en
	}
	if len(m.maps) != 36 || !nonlinear {
		return fail()
	}
	models := 0
	for _, ev := range m.maps {
		if ev.Dimension != 3 {
			continue
		}
		models++
		sc, e := simple(f, ev.SurfaceCurveID, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[1].Kind != Reference || sc.Args[1].Ref != ev.CurveID {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
			return fail()
		}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil || pc.Args[2].Kind != Reference {
				return fail()
			}
			rep, e := simple(f, pc.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			ids, e := listRefs(rep.Args[1], 1)
			if e != nil || len(ids) != 1 {
				return fail()
			}
			uv, ok := byCurve[ids[0]]
			if !ok || uv.Dimension != 2 || uv.PCurveID != pcid || uv.SurfaceCurveID != ev.SurfaceCurveID || uv.EdgeID != ev.EdgeID || uv.Ratio != ev.Ratio {
				return fail()
			}
		}
	}
	if models != 12 {
		return fail()
	}
	view, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	proof, e := ReadNonplanarSplineShellSource(ctx, bytes.NewReader(view), l)
	if e != nil {
		return zero, e
	}
	m.Model = proof.Model
	m.bodyID = proof.bodyID
	m.faces = proof.faces
	m.edges = proof.edges
	m.surfaces = proof.surfaces
	m.curves = proof.curves
	return m, nil
}

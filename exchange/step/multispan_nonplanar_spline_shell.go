package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// MultispanNonplanarSplineShellSource retains original polynomial UV bases on
// a proved nonplanar six-face material. The private proof view changes only
// exactly affine UV representations, never the original 3D carriers or edges.
type MultispanNonplanarSplineShellSource struct {
	Model                  SplineShellModel
	original               string
	bodyID                 int64
	faces, edges, surfaces []int64
	curves                 []MultispanNURBSPCurveEvidence
}

func (m MultispanNonplanarSplineShellSource) OriginalSTEP() string { return m.original }
func (m MultispanNonplanarSplineShellSource) PCurves() []MultispanNURBSPCurveEvidence {
	return append([]MultispanNURBSPCurveEvidence(nil), m.curves...)
}
func (m MultispanNonplanarSplineShellSource) SourceFaces() []int64 {
	return append([]int64(nil), m.faces...)
}
func (m MultispanNonplanarSplineShellSource) SourceEdges() []int64 {
	return append([]int64(nil), m.edges...)
}

// ReadMultispanNonplanarSplineShellSource accepts 24 independently owned,
// clamped continuous polynomial UV splines of degree 1..8 and at most 17 poles.
// Every original coefficient must prove the same parameterized affine line;
// at least one UV curve must have an internal knot. Existing single-span
// source and generic readers retain their narrower contracts.
func ReadMultispanNonplanarSplineShellSource(ctx context.Context, r io.Reader, l Limits) (MultispanNonplanarSplineShellSource, error) {
	var zero MultispanNonplanarSplineShellSource
	fail := func() (MultispanNonplanarSplineShellSource, error) { return zero, geom.ErrInvalid }
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
	m := MultispanNonplanarSplineShellSource{original: string(raw)}
	removed := map[int64]bool{}
	internal := false
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			continue
		}
		if len(m.curves) >= 24 || pc.Args[1].Kind != Reference || pc.Args[2].Kind != Reference {
			return fail()
		}
		rid := pc.Args[2].Ref
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
		own, _, e := one(cid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil || own != rid {
			return fail()
		}
		own, _, e = one(rid, "PCURVE", 3)
		if e != nil || own != id {
			return fail()
		}
		scid, _, e := one(id, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		eid, _, e := one(scid, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		d, e := affineSplineSourceRat(c.Args[1])
		if e != nil || !d.IsInt() || d.Cmp(big.NewRat(1, 1)) < 0 || d.Cmp(big.NewRat(8, 1)) > 0 {
			return fail()
		}
		degree := int(d.Num().Int64())
		poles, e := listRefs(c.Args[2], 2)
		if e != nil || len(poles) < 2 || len(poles) > 17 {
			return fail()
		}
		ev := MultispanNURBSPCurveEvidence{CurveID: cid, PCurveID: id, SurfaceID: pc.Args[1].Ref, SurfaceCurveID: scid, EdgeID: eid, Degree: degree, Count: len(poles)}
		copy(ev.PoleIDs[:], poles)
		var g []*big.Rat
		ev.Knots, ev.Multiplicity, g, e = affineSplineGreville(c.Args[6], c.Args[7], degree, len(poles))
		if e != nil || !affineSplineKnotSpec(c.Args[8], c.Args[6], c.Args[7], degree) {
			return fail()
		}
		internal = internal || len(c.Args[7].Items) > 2
		for _, v := range c.Args[7].Items {
			if _, e := wrappedIsoSourceNumber(v); e != nil {
				return fail()
			}
		}
		var p [17][2]*big.Rat
		for i, pid := range poles {
			if len(incoming[pid]) != 1 || incoming[pid][0] != cid {
				return fail()
			}
			q, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || q.Args[1].Kind != Aggregate || len(q.Args[1].Items) != 2 {
				return fail()
			}
			for j, v := range q.Args[1].Items {
				if _, e := wrappedIsoSourceNumber(v); e != nil {
					return fail()
				}
				p[i][j], e = affineSplineSourceRat(v)
				if e != nil {
					return fail()
				}
				ev.Poles[i][j] = p[i][j].RatString()
			}
		}
		last := len(poles) - 1
		for i := range poles {
			for j := 0; j < 2; j++ {
				want := new(big.Rat).Add(p[0][j], new(big.Rat).Mul(g[i], new(big.Rat).Sub(p[last][j], p[0][j])))
				if want.Num().BitLen() > 1024 || want.Denom().BitLen() > 1024 || want.Cmp(p[i][j]) != 0 {
					return fail()
				}
			}
		}
		for i := 1; i < last; i++ {
			removed[poles[i]] = true
		}
		c.Args[1] = Value{Kind: Number, Text: "1"}
		c.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: poles[0]}, {Kind: Reference, Ref: poles[last]}}}
		c.Args[6] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}}
		c.Args[7] = Value{Kind: Aggregate, Items: []Value{c.Args[7].Items[0], c.Args[7].Items[len(c.Args[7].Items)-1]}}
		c.Args[8] = Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}
		en := f.Entities[cid]
		en.Records[0] = c
		f.Entities[cid] = en
		m.curves = append(m.curves, ev)
	}
	if !internal || len(m.curves) != 24 {
		return fail()
	}
	order := make([]int64, 0, len(f.Order))
	for _, id := range f.Order {
		if removed[id] {
			delete(f.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	f.Order = order
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
	return m, nil
}

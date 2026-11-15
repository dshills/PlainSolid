package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// AffineModelSplineEvidence retains the original complete 3D spline basis and
// unique model-edge ownership. Active poles are [0,Count); knots have nonzero multiplicity.
type AffineModelSplineEvidence struct {
	CurveID, SurfaceCurveID, EdgeID int64
	Degree, Count                   int
	PoleIDs                         [17]int64
	Poles                           [17][3]string
	Knots                           [17]string
	Multiplicity                    [17]int
}

// ModelSplineNURBSPlanarAssembly authenticates original affine multi-span
// supports, UV curves and straight 3D spline model edges without rounding.
type ModelSplineNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	planar   MultispanNURBSPlanarAssembly
	models   []AffineModelSplineEvidence
}

func (m ModelSplineNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m ModelSplineNURBSPlanarAssembly) ModelCurves() []AffineModelSplineEvidence {
	return append([]AffineModelSplineEvidence(nil), m.models...)
}
func (m ModelSplineNURBSPlanarAssembly) Surfaces() []MultispanNURBSSurfaceEvidence {
	return m.planar.Surfaces()
}
func (m ModelSplineNURBSPlanarAssembly) PCurves() []MultispanNURBSPCurveEvidence {
	return m.planar.PCurves()
}

func affineModelSplineView(ctx context.Context, f *File) ([]AffineModelSplineEvidence, error) {
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
	for id, e := range f.Entities {
		for _, r := range e.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	fail := func() ([]AffineModelSplineEvidence, error) { return nil, geom.ErrInvalid }
	removed := map[int64]bool{}
	var out []AffineModelSplineEvidence
	total := 0
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "B_SPLINE_CURVE_WITH_KNOTS" {
			continue
		}
		r := en.Records[0]
		if len(r.Args) != 9 {
			return fail()
		}
		ids, e := listRefs(r.Args[2], 2)
		if e != nil || len(ids) > 17 {
			return fail()
		}
		first, e := simple(f, ids[0], "CARTESIAN_POINT", 2)
		if e != nil || first.Args[1].Kind != Aggregate {
			return fail()
		}
		if len(first.Args[1].Items) == 2 {
			continue
		}
		if len(first.Args[1].Items) != 3 {
			return fail()
		}
		q, e := affineSplineSourceRat(r.Args[1])
		if e != nil || !q.IsInt() || !q.Num().IsInt64() {
			return fail()
		}
		d := int(q.Num().Int64())
		if d < 1 || d > 8 {
			return fail()
		}
		if d == 1 && len(r.Args[7].Items) == 2 {
			continue
		}
		total += len(ids)
		if total > 4096 {
			return fail()
		}
		ev := AffineModelSplineEvidence{CurveID: id, Degree: d, Count: len(ids)}
		copy(ev.PoleIDs[:], ids)
		var g []*big.Rat
		ev.Knots, ev.Multiplicity, g, e = affineSplineGreville(r.Args[6], r.Args[7], d, len(ids))
		if e != nil || !affineSplineKnotSpec(r.Args[8], r.Args[6], r.Args[7], d) {
			return fail()
		}
		var p [17][3]*big.Rat
		for i, pid := range ids {
			if len(incoming[pid]) != 1 || incoming[pid][0] != id {
				return fail()
			}
			point, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || point.Args[0].Kind != String || point.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 3 {
				return fail()
			}
			for j, v := range point.Args[1].Items {
				p[i][j], e = affineSplineSourceRat(v)
				if e != nil {
					return fail()
				}
				ev.Poles[i][j] = p[i][j].RatString()
			}
		}
		n := len(ids) - 1
		for i := range ids {
			for j := 0; j < 3; j++ {
				want := new(big.Rat).Add(p[0][j], new(big.Rat).Mul(g[i], new(big.Rat).Sub(p[n][j], p[0][j])))
				if want.Num().BitLen() > 1024 || want.Denom().BitLen() > 1024 || want.Cmp(p[i][j]) != 0 {
					return fail()
				}
			}
		}
		if len(incoming[id]) != 1 {
			return fail()
		}
		scid := incoming[id][0]
		sc, e := simple(f, scid, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[1].Kind != Reference || sc.Args[1].Ref != id || len(incoming[scid]) != 1 {
			return fail()
		}
		eid := incoming[scid][0]
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil || edge.Args[3].Kind != Reference || edge.Args[3].Ref != scid {
			return fail()
		}
		ev.SurfaceCurveID, ev.EdgeID = scid, eid
		for i := 1; i < n; i++ {
			removed[ids[i]] = true
		}
		r.Args[1] = Value{Kind: Number, Text: "1"}
		r.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ids[0]}, {Kind: Reference, Ref: ids[n]}}}
		r.Args[6] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}}
		r.Args[7] = Value{Kind: Aggregate, Items: []Value{r.Args[7].Items[0], r.Args[7].Items[len(r.Args[7].Items)-1]}}
		r.Args[8] = Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}
		en.Records[0] = r
		f.Entities[id] = en
		out = append(out, ev)
	}
	if len(out) == 0 {
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
	return out, nil
}
func ReadModelSplineNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (ModelSplineNURBSPlanarAssembly, error) {
	var zero ModelSplineNURBSPlanarAssembly
	if ctx == nil || r == nil {
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
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
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
	models, e := affineModelSplineView(ctx, f)
	if e != nil {
		return zero, e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	p, e := ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewReader(b), l)
	if e != nil {
		return zero, e
	}
	original, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	p.Geometry.Assembly = original
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return ModelSplineNURBSPlanarAssembly{p.Geometry, string(raw), p, models}, nil
}
func WriteModelSplineNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m ModelSplineNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadModelSplineNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := modelSplineNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := modelSplineNURBSNativeClaim(ctx, p)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(a, b) {
		return geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := io.WriteString(w, m.original)
	if e == nil && n != len(m.original) {
		e = io.ErrShortWrite
	}
	return e
}

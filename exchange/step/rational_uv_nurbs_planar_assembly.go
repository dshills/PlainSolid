package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// RationalAffinePCurveEvidence retains original positive-weight quadratic UV
// poles, knot interval and the complete model edge / face-support owner chain.
type RationalAffinePCurveEvidence struct {
	CurveID, PCurveID, SurfaceID, SurfaceCurveID, EdgeID int64
	PoleIDs                                              [3]int64
	Poles                                                [3][2]string
	Weights                                              [3]string
	Knots                                                [2]string
}

// RationalUVNURBSPlanarAssembly authenticates affine rational tensor supports
// and original rational affine owner pcurves under a separate bounded route.
type RationalUVNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	planar   ElevatedNURBSPlanarAssembly
	supports []TensorRationalAffineSurfaceEvidence
	uv       []RationalAffinePCurveEvidence
}

func (m RationalUVNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m RationalUVNURBSPlanarAssembly) RationalSupports() []TensorRationalAffineSurfaceEvidence {
	return append([]TensorRationalAffineSurfaceEvidence(nil), m.supports...)
}
func (m RationalUVNURBSPlanarAssembly) PCurves() []RationalAffinePCurveEvidence {
	return append([]RationalAffinePCurveEvidence(nil), m.uv...)
}

func rationalAffinePCurveView(ctx context.Context, f *File) ([]RationalAffinePCurveEvidence, error) {
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, q := range v.Items {
			walk(id, q)
		}
	}
	for id, en := range f.Entities {
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	fail := func() ([]RationalAffinePCurveEvidence, error) { return nil, geom.ErrInvalid }
	one := func(id int64, kind string, n int) (int64, Record, error) {
		if len(incoming[id]) != 1 {
			return 0, Record{}, geom.ErrInvalid
		}
		owner := incoming[id][0]
		r, e := simple(f, owner, kind, n)
		return owner, r, e
	}
	var out []RationalAffinePCurveEvidence
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		en := f.Entities[id]
		has := false
		for _, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_CURVE" {
				has = true
			}
		}
		if !has {
			continue
		}
		records := map[string]Record{}
		for _, r := range en.Records {
			if _, ok := records[r.Type]; ok {
				return fail()
			}
			records[r.Type] = r
		}
		allowed := map[string]int{"BOUNDED_CURVE": 0, "B_SPLINE_CURVE": 5, "B_SPLINE_CURVE_WITH_KNOTS": 3, "CURVE": 0, "GEOMETRIC_REPRESENTATION_ITEM": 0, "RATIONAL_B_SPLINE_CURVE": 1, "REPRESENTATION_ITEM": 1}
		if len(records) != len(allowed) {
			return fail()
		}
		for k, n := range allowed {
			r, ok := records[k]
			if !ok || len(r.Args) != n {
				return fail()
			}
		}
		b, k, w, name := records["B_SPLINE_CURVE"], records["B_SPLINE_CURVE_WITH_KNOTS"], records["RATIONAL_B_SPLINE_CURVE"], records["REPRESENTATION_ITEM"]
		d, e := affineSplineSourceRat(b.Args[0])
		if e != nil || d.Cmp(big.NewRat(2, 1)) != 0 || name.Args[0].Kind != String {
			return fail()
		}
		ids, e := listRefs(b.Args[1], 3)
		if e != nil || len(ids) != 3 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != 3 {
			return fail()
		}
		knots, _, _, e := affineSplineGreville(k.Args[0], k.Args[1], 2, 3)
		if e != nil || len(k.Args[1].Items) != 2 || !affineSplineKnotSpec(k.Args[2], k.Args[0], k.Args[1], 2) {
			return fail()
		}
		ev := RationalAffinePCurveEvidence{CurveID: id, Knots: [2]string{knots[0], knots[1]}}
		copy(ev.PoleIDs[:], ids)
		var p [3][2]*big.Rat
		var weights [3]*big.Rat
		different := false
		for i, pid := range ids {
			if len(incoming[pid]) != 1 || incoming[pid][0] != id {
				return fail()
			}
			pr, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || pr.Args[0].Kind != String || pr.Args[1].Kind != Aggregate || len(pr.Args[1].Items) != 2 {
				return fail()
			}
			weights[i], e = affineSplineSourceRat(w.Args[0].Items[i])
			if e != nil || weights[i].Sign() <= 0 {
				return fail()
			}
			ev.Weights[i] = weights[i].RatString()
			if weights[i].Cmp(weights[0]) != 0 {
				different = true
			}
			for c, v := range pr.Args[1].Items {
				p[i][c], e = affineSplineSourceRat(v)
				if e != nil {
					return fail()
				}
				ev.Poles[i][c] = p[i][c].RatString()
			}
		}
		if !different {
			return fail()
		}
		// Exact cubic Bernstein coefficients of N = W*(P0+t*(P2-P0)).
		for i := 0; i <= 3; i++ {
			for c := 0; c < 2; c++ {
				lhs, rhs := new(big.Rat), new(big.Rat)
				delta := new(big.Rat).Sub(p[2][c], p[0][c])
				for a := 0; a < 3; a++ {
					elev, times := new(big.Rat), new(big.Rat)
					if a == i {
						elev.SetFrac64(int64(3-i), 3)
					}
					if a == i-1 {
						elev.SetFrac64(int64(i), 3)
						times.Set(elev)
					}
					lhs.Add(lhs, new(big.Rat).Mul(new(big.Rat).Mul(elev, weights[a]), p[a][c]))
					term := new(big.Rat).Add(new(big.Rat).Mul(elev, p[0][c]), new(big.Rat).Mul(times, delta))
					rhs.Add(rhs, new(big.Rat).Mul(weights[a], term))
				}
				if lhs.Num().BitLen() > 4096 || lhs.Denom().BitLen() > 4096 || rhs.Num().BitLen() > 4096 || rhs.Denom().BitLen() > 4096 || lhs.Cmp(rhs) != 0 {
					return fail()
				}
			}
		}
		defID, def, e := one(id, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil || def.Args[1].Kind != Aggregate || len(def.Args[1].Items) != 1 || def.Args[1].Items[0].Ref != id {
			return fail()
		}
		pcID, pc, e := one(defID, "PCURVE", 3)
		if e != nil || pc.Args[2].Kind != Reference || pc.Args[2].Ref != defID || pc.Args[1].Kind != Reference {
			return fail()
		}
		scID, _, e := one(pcID, "SURFACE_CURVE", 4)
		if e != nil {
			return fail()
		}
		edgeID, edge, e := one(scID, "EDGE_CURVE", 5)
		if e != nil || edge.Args[3].Kind != Reference || edge.Args[3].Ref != scID {
			return fail()
		}
		ev.PCurveID, ev.SurfaceID, ev.SurfaceCurveID, ev.EdgeID = pcID, pc.Args[1].Ref, scID, edgeID
		pe := f.Entities[ids[1]]
		pr := pe.Records[0]
		for c := 0; c < 2; c++ {
			q := new(big.Rat).Quo(new(big.Rat).Add(p[0][c], p[2][c]), big.NewRat(2, 1))
			v, e := extrusionDirectionDecimal(q)
			if e != nil {
				return fail()
			}
			pr.Args[1].Items[c] = Value{Kind: Number, Text: v}
		}
		pe.Records[0] = pr
		f.Entities[ids[1]] = pe
		en.Records = []Record{{Type: "B_SPLINE_CURVE_WITH_KNOTS", Args: []Value{name.Args[0], b.Args[0], b.Args[1], b.Args[2], b.Args[3], b.Args[4], k.Args[0], k.Args[1], Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}}}}
		f.Entities[id] = en
		out = append(out, ev)
		if len(out) > 512 {
			return fail()
		}
	}
	if len(out) == 0 {
		return fail()
	}
	return out, nil
}
func ReadRationalUVNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (RationalUVNURBSPlanarAssembly, error) {
	var zero RationalUVNURBSPlanarAssembly
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
	supports, e := tensorRationalAffineSurfaceView(ctx, f)
	if e != nil {
		return zero, e
	}
	uv, e := rationalAffinePCurveView(ctx, f)
	if e != nil {
		return zero, e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	p, e := ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(b), l)
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
	return RationalUVNURBSPlanarAssembly{p.Geometry, string(raw), p, supports, uv}, nil
}
func WriteRationalUVNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m RationalUVNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadRationalUVNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := rationalUVNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := rationalUVNURBSNativeClaim(ctx, p)
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

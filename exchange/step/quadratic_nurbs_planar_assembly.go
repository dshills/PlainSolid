package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// QuadraticNURBSSurfaceEvidence identifies every original pole and knot of a
// degree-two tensor support proved identically affine, not sampled planar.
type QuadraticNURBSSurfaceEvidence struct {
	SurfaceID, FaceID int64
	PoleIDs           [3][3]int64
	Poles             [3][3][3]string
	UKnots, VKnots    [2]string
}

// QuadraticNURBSPCurveEvidence retains original owner IDs and the complete
// quadratic UV basis proved identical, in parameter, to its endpoint line.
type QuadraticNURBSPCurveEvidence struct {
	CurveID, PCurveID, SurfaceID, SurfaceCurveID, EdgeID int64
	PoleIDs                                              [3]int64
	Poles                                                [3][2]string
	Knots                                                [2]string
}

// QuadraticNURBSPlanarAssembly retains native planar geometry in original
// product coordinates and the complete original source. Its bounded quadratic
// support/pcurve identities complement the unchanged bilinear reader.
type QuadraticNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	surfaces []QuadraticNURBSSurfaceEvidence
	curves   []QuadraticNURBSPCurveEvidence
	frames   []ObliqueNURBSProductFrame
	points   []ObliqueNURBSPointEvidence
}

func (m QuadraticNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m QuadraticNURBSPlanarAssembly) Surfaces() []QuadraticNURBSSurfaceEvidence {
	return append([]QuadraticNURBSSurfaceEvidence(nil), m.surfaces...)
}
func (m QuadraticNURBSPlanarAssembly) PCurves() []QuadraticNURBSPCurveEvidence {
	return append([]QuadraticNURBSPCurveEvidence(nil), m.curves...)
}

func quadraticNURBSView(ctx context.Context, f *File) ([]QuadraticNURBSSurfaceEvidence, []QuadraticNURBSPCurveEvidence, error) {
	fail := func() ([]QuadraticNURBSSurfaceEvidence, []QuadraticNURBSPCurveEvidence, error) {
		return nil, nil, geom.ErrInvalid
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
	for id, en := range f.Entities {
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	single := func(id int64, kind string, n int) (int64, Record, error) {
		if len(incoming[id]) != 1 {
			return 0, Record{}, geom.ErrInvalid
		}
		owner := incoming[id][0]
		r, e := simple(f, owner, kind, n)
		return owner, r, e
	}
	rat := func(v Value) (*big.Rat, error) {
		q, e := liftSourceRat(v)
		if e != nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
			return nil, geom.ErrInvalid
		}
		return q, nil
	}
	degree := func(v Value) (int, error) {
		q, e := rat(v)
		if e != nil || !q.IsInt() || !q.Num().IsInt64() {
			return 0, geom.ErrInvalid
		}
		n := q.Num().Int64()
		if n < 1 || n > 2 {
			return 0, geom.ErrInvalid
		}
		return int(n), nil
	}
	point := func(id, owner int64, dim int) ([]*big.Rat, error) {
		if len(incoming[id]) != 1 || incoming[id][0] != owner {
			return nil, geom.ErrInvalid
		}
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil || r.Args[0].Kind != String || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != dim {
			return nil, geom.ErrInvalid
		}
		out := make([]*big.Rat, dim)
		for i, v := range r.Args[1].Items {
			out[i], e = rat(v)
			if e != nil {
				return nil, e
			}
		}
		return out, nil
	}
	knots := func(m, k Value) ([2]string, error) {
		var out [2]string
		if m.Kind != Aggregate || k.Kind != Aggregate || len(m.Items) != 2 || len(k.Items) != 2 {
			return out, geom.ErrInvalid
		}
		var last *big.Rat
		for i := 0; i < 2; i++ {
			mu, e := rat(m.Items[i])
			if e != nil || mu.Cmp(big.NewRat(3, 1)) != 0 {
				return out, geom.ErrInvalid
			}
			q, e := rat(k.Items[i])
			if e != nil || (last != nil && q.Cmp(last) <= 0) {
				return out, geom.ErrInvalid
			}
			out[i] = q.RatString()
			last = q
		}
		return out, nil
	}
	multiplicity := Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}}
	removed := map[int64]bool{}
	var surfaces []QuadraticNURBSSurfaceEvidence
	var curves []QuadraticNURBSPCurveEvidence
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		r := en.Records[0]
		switch r.Type {
		case "B_SPLINE_SURFACE_WITH_KNOTS":
			if len(r.Args) != 13 {
				return fail()
			}
			u, e := degree(r.Args[1])
			v, e2 := degree(r.Args[2])
			if e != nil || e2 != nil {
				return fail()
			}
			if u == 1 && v == 1 {
				continue
			}
			if u != 2 || v != 2 || r.Args[3].Kind != Aggregate || len(r.Args[3].Items) != 3 {
				return fail()
			}
			ev := QuadraticNURBSSurfaceEvidence{SurfaceID: id}
			ev.UKnots, e = knots(r.Args[8], r.Args[10])
			if e != nil {
				return fail()
			}
			ev.VKnots, e = knots(r.Args[9], r.Args[11])
			if e != nil {
				return fail()
			}
			var p [3][3][]*big.Rat
			for i, row := range r.Args[3].Items {
				ids, e := listRefs(row, 3)
				if e != nil || len(ids) != 3 {
					return fail()
				}
				for j, pid := range ids {
					ev.PoleIDs[i][j] = pid
					p[i][j], e = point(pid, id, 3)
					if e != nil {
						return fail()
					}
					for k, q := range p[i][j] {
						ev.Poles[i][j][k] = q.RatString()
					}
				}
			}
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					for k := 0; k < 3; k++ {
						want := new(big.Rat).Set(p[0][0][k])
						want.Add(want, new(big.Rat).Mul(big.NewRat(int64(i), 2), new(big.Rat).Sub(p[2][0][k], p[0][0][k])))
						want.Add(want, new(big.Rat).Mul(big.NewRat(int64(j), 2), new(big.Rat).Sub(p[0][2][k], p[0][0][k])))
						if want.Cmp(p[i][j][k]) != 0 {
							return fail()
						}
					}
					if i == 1 || j == 1 {
						removed[ev.PoleIDs[i][j]] = true
					}
				}
			}
			for _, owner := range incoming[id] {
				if face, e := simple(f, owner, "ADVANCED_FACE", 4); e == nil && face.Args[2].Kind == Reference && face.Args[2].Ref == id {
					if ev.FaceID != 0 {
						return fail()
					}
					ev.FaceID = owner
				}
			}
			if ev.FaceID == 0 {
				return fail()
			}
			r.Args[1], r.Args[2] = Value{Kind: Number, Text: "1"}, Value{Kind: Number, Text: "1"}
			r.Args[8], r.Args[9] = multiplicity, multiplicity
			r.Args[3] = Value{Kind: Aggregate, Items: []Value{{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ev.PoleIDs[0][0]}, {Kind: Reference, Ref: ev.PoleIDs[0][2]}}}, {Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ev.PoleIDs[2][0]}, {Kind: Reference, Ref: ev.PoleIDs[2][2]}}}}}
			en.Records[0] = r
			f.Entities[id] = en
			surfaces = append(surfaces, ev)
		case "B_SPLINE_CURVE_WITH_KNOTS":
			if len(r.Args) != 9 {
				return fail()
			}
			d, e := degree(r.Args[1])
			if e != nil {
				return fail()
			}
			if d == 1 {
				continue
			}
			if d != 2 {
				return fail()
			}
			ids, e := listRefs(r.Args[2], 3)
			if e != nil || len(ids) != 3 {
				return fail()
			}
			ev := QuadraticNURBSPCurveEvidence{CurveID: id, PoleIDs: [3]int64{ids[0], ids[1], ids[2]}}
			ev.Knots, e = knots(r.Args[6], r.Args[7])
			if e != nil {
				return fail()
			}
			var p [3][]*big.Rat
			for i, pid := range ids {
				p[i], e = point(pid, id, 2)
				if e != nil {
					return fail()
				}
				for j, q := range p[i] {
					ev.Poles[i][j] = q.RatString()
				}
			}
			for j := 0; j < 2; j++ {
				if new(big.Rat).Add(p[0][j], p[2][j]).Cmp(new(big.Rat).Mul(big.NewRat(2, 1), p[1][j])) != 0 {
					return fail()
				}
			}
			defID, _, e := single(id, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			pcID, pc, e := single(defID, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			scID, _, e := single(pcID, "SURFACE_CURVE", 4)
			if e != nil {
				return fail()
			}
			edgeID, _, e := single(scID, "EDGE_CURVE", 5)
			if e != nil {
				return fail()
			}
			ev.PCurveID, ev.SurfaceID, ev.SurfaceCurveID, ev.EdgeID = pcID, pc.Args[1].Ref, scID, edgeID
			removed[ids[1]] = true
			r.Args[1] = Value{Kind: Number, Text: "1"}
			r.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ids[0]}, {Kind: Reference, Ref: ids[2]}}}
			r.Args[6] = multiplicity
			en.Records[0] = r
			f.Entities[id] = en
			curves = append(curves, ev)
		}
	}
	if len(surfaces) == 0 || len(curves) == 0 {
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
	return surfaces, curves, nil
}
func ReadQuadraticNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (QuadraticNURBSPlanarAssembly, error) {
	var zero QuadraticNURBSPlanarAssembly
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
		return zero, geom.ErrInvalid
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	surfaces, curves, e := quadraticNURBSView(ctx, view)
	if e != nil {
		return zero, e
	}
	data, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	proof, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(data), l)
	if e != nil {
		return zero, e
	}
	original, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	proof.Geometry.Assembly = original
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return QuadraticNURBSPlanarAssembly{proof.Geometry, string(raw), surfaces, curves, proof.frames, proof.points}, nil
}
func WriteQuadraticNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m QuadraticNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := quadraticNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := quadraticNURBSNativeClaim(ctx, p)
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

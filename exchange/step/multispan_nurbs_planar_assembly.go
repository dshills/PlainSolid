package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// MultispanNURBSSurfaceEvidence identifies every original pole and knot of a
// tensor support of independent degrees 1..8, proved identically affine.
// Only indices below UCount/VCount are active; remaining entries are zero.
type MultispanNURBSSurfaceEvidence struct {
	SurfaceID, FaceID            int64
	UDegree, VDegree             int
	PoleIDs                      [17][17]int64
	Poles                        [17][17][3]string
	UKnots, VKnots               [17]string
	UMultiplicity, VMultiplicity [17]int
	UCount, VCount               int
}

// MultispanNURBSPCurveEvidence retains original owner IDs and the complete
// multispan UV basis proved identical, in parameter, to its endpoint line.
// Only indices below Count are active; remaining entries are zero.
type MultispanNURBSPCurveEvidence struct {
	CurveID, PCurveID, SurfaceID, SurfaceCurveID, EdgeID int64
	Degree                                               int
	PoleIDs                                              [17]int64
	Poles                                                [17][2]string
	Knots                                                [17]string
	Multiplicity                                         [17]int
	Count                                                int
}

// MultispanNURBSPlanarAssembly retains native planar geometry in original
// product coordinates and the complete original source. Its bounded multispan
// support/pcurve identities complement the unchanged bilinear reader.
type MultispanNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	surfaces []MultispanNURBSSurfaceEvidence
	curves   []MultispanNURBSPCurveEvidence
	frames   []ObliqueNURBSProductFrame
	points   []ObliqueNURBSPointEvidence
}

func (m MultispanNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m MultispanNURBSPlanarAssembly) Surfaces() []MultispanNURBSSurfaceEvidence {
	return append([]MultispanNURBSSurfaceEvidence(nil), m.surfaces...)
}
func (m MultispanNURBSPlanarAssembly) PCurves() []MultispanNURBSPCurveEvidence {
	return append([]MultispanNURBSPCurveEvidence(nil), m.curves...)
}

func multispanNURBSView(ctx context.Context, f *File) ([]MultispanNURBSSurfaceEvidence, []MultispanNURBSPCurveEvidence, error) {
	fail := func() ([]MultispanNURBSSurfaceEvidence, []MultispanNURBSPCurveEvidence, error) {
		return nil, nil, fmt.Errorf("step: unsupported exact multispan planar basis/owner: %w", geom.ErrInvalid)
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
		if n < 1 || n > 8 {
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
	knots := affineSplineGreville
	knotSpec := affineSplineKnotSpec
	ends := func(v Value) Value {
		return Value{Kind: Aggregate, Items: []Value{v.Items[0], v.Items[len(v.Items)-1]}}
	}
	hasInternal := false
	multiplicity := Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "2"}}}
	removed := map[int64]bool{}
	var surfaces []MultispanNURBSSurfaceEvidence
	var curves []MultispanNURBSPCurveEvidence
	controlCount := 0
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
			if u == 1 && v == 1 && len(r.Args[10].Items) == 2 && len(r.Args[11].Items) == 2 {
				continue
			}
			if r.Args[3].Kind != Aggregate || len(r.Args[3].Items) < 2 || len(r.Args[3].Items) > 17 || r.Args[3].Items[0].Kind != Aggregate {
				return fail()
			}
			ud, vd := u, v
			u, v = len(r.Args[3].Items)-1, len(r.Args[3].Items[0].Items)-1
			if v < 1 || v > 16 {
				return fail()
			}
			controlCount += (u + 1) * (v + 1)
			if controlCount > 4096 {
				return fail()
			}
			ev := MultispanNURBSSurfaceEvidence{SurfaceID: id, UDegree: ud, VDegree: vd, UCount: u + 1, VCount: v + 1}
			var gu, gv []*big.Rat
			ev.UKnots, ev.UMultiplicity, gu, e = knots(r.Args[8], r.Args[10], ud, u+1)
			if e != nil {
				return fail()
			}
			ev.VKnots, ev.VMultiplicity, gv, e = knots(r.Args[9], r.Args[11], vd, v+1)
			hasInternal = hasInternal || len(r.Args[10].Items) > 2 || len(r.Args[11].Items) > 2
			if e != nil {
				return fail()
			}
			var p [17][17][]*big.Rat
			for i, row := range r.Args[3].Items {
				ids, e := listRefs(row, v+1)
				if e != nil || len(ids) != v+1 {
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
			for i := 0; i <= u; i++ {
				for j := 0; j <= v; j++ {
					for k := 0; k < 3; k++ {
						want := new(big.Rat).Set(p[0][0][k])
						want.Add(want, new(big.Rat).Mul(gu[i], new(big.Rat).Sub(p[u][0][k], p[0][0][k])))
						want.Add(want, new(big.Rat).Mul(gv[j], new(big.Rat).Sub(p[0][v][k], p[0][0][k])))
						if want.Num().BitLen() > 1024 || want.Denom().BitLen() > 1024 {
							return fail()
						}
						if want.Cmp(p[i][j][k]) != 0 {
							return nil, nil, fmt.Errorf("step: surface %d pole (%d,%d) coordinate %d is not exactly affine: %w", id, i, j, k, geom.ErrInvalid)
						}
					}
					if (i > 0 && i < u) || (j > 0 && j < v) {
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
			if !knotSpec(r.Args[12], r.Args[8], r.Args[10], ud) || !knotSpec(r.Args[12], r.Args[9], r.Args[11], vd) {
				return fail()
			}
			r.Args[12] = Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}
			r.Args[1], r.Args[2] = Value{Kind: Number, Text: "1"}, Value{Kind: Number, Text: "1"}
			r.Args[8], r.Args[9] = multiplicity, multiplicity
			r.Args[10], r.Args[11] = ends(r.Args[10]), ends(r.Args[11])
			r.Args[3] = Value{Kind: Aggregate, Items: []Value{{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ev.PoleIDs[0][0]}, {Kind: Reference, Ref: ev.PoleIDs[0][v]}}}, {Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ev.PoleIDs[u][0]}, {Kind: Reference, Ref: ev.PoleIDs[u][v]}}}}}
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
			if d == 1 && len(r.Args[7].Items) == 2 {
				continue
			}
			ids, e := listRefs(r.Args[2], 2)
			if e != nil || len(ids) < 2 || len(ids) > 17 {
				return fail()
			}
			degreeOriginal := d
			d = len(ids) - 1
			controlCount += d + 1
			if controlCount > 4096 {
				return fail()
			}
			ev := MultispanNURBSPCurveEvidence{CurveID: id, Degree: degreeOriginal, Count: d + 1}
			copy(ev.PoleIDs[:], ids)
			var g []*big.Rat
			ev.Knots, ev.Multiplicity, g, e = knots(r.Args[6], r.Args[7], degreeOriginal, d+1)
			hasInternal = hasInternal || len(r.Args[7].Items) > 2
			if e != nil {
				return fail()
			}
			var p [17][]*big.Rat
			for i, pid := range ids {
				p[i], e = point(pid, id, 2)
				if e != nil {
					return fail()
				}
				for j, q := range p[i] {
					ev.Poles[i][j] = q.RatString()
				}
			}
			for i := 0; i <= d; i++ {
				for j := 0; j < 2; j++ {
					want := new(big.Rat).Add(p[0][j], new(big.Rat).Mul(g[i], new(big.Rat).Sub(p[d][j], p[0][j])))
					if want.Num().BitLen() > 1024 || want.Denom().BitLen() > 1024 {
						return fail()
					}
					if want.Cmp(p[i][j]) != 0 {
						return fail()
					}
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
			for i := 1; i < d; i++ {
				removed[ids[i]] = true
			}
			if !knotSpec(r.Args[8], r.Args[6], r.Args[7], degreeOriginal) {
				return fail()
			}
			r.Args[8] = Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}
			r.Args[1] = Value{Kind: Number, Text: "1"}
			r.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: ids[0]}, {Kind: Reference, Ref: ids[d]}}}
			r.Args[6] = multiplicity
			r.Args[7] = ends(r.Args[7])
			en.Records[0] = r
			f.Entities[id] = en
			curves = append(curves, ev)
		}
	}
	if !hasInternal || len(surfaces) == 0 || len(curves) == 0 {
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
func ReadMultispanNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (MultispanNURBSPlanarAssembly, error) {
	var zero MultispanNURBSPlanarAssembly
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
	surfaces, curves, e := multispanNURBSView(ctx, view)
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
	return MultispanNURBSPlanarAssembly{proof.Geometry, string(raw), surfaces, curves, proof.frames, proof.points}, nil
}
func WriteMultispanNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m MultispanNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := multispanNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := multispanNURBSNativeClaim(ctx, p)
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

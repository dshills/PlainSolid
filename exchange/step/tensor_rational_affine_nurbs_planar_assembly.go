package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// TensorRationalAffineSurfaceEvidence is the original degree-2 by degree-2
// rational tensor, proved affine by homogeneous Bernstein identities.
type TensorRationalAffineSurfaceEvidence struct {
	SurfaceID, FaceID int64
	PoleIDs           [3][3]int64
	Poles             [3][3][3]string
	Weights           [3][3]string
	UKnots, VKnots    [2]string
}
type TensorRationalAffineNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	planar   MultispanNURBSPlanarAssembly
	supports []TensorRationalAffineSurfaceEvidence
}

func (m TensorRationalAffineNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m TensorRationalAffineNURBSPlanarAssembly) RationalSupports() []TensorRationalAffineSurfaceEvidence {
	return append([]TensorRationalAffineSurfaceEvidence(nil), m.supports...)
}
func (m TensorRationalAffineNURBSPlanarAssembly) PCurves() []MultispanNURBSPCurveEvidence {
	return m.planar.PCurves()
}

func tensorRationalAffineSurfaceView(ctx context.Context, f *File) ([]TensorRationalAffineSurfaceEvidence, error) {
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
	fail := func() ([]TensorRationalAffineSurfaceEvidence, error) { return nil, geom.ErrInvalid }
	var out []TensorRationalAffineSurfaceEvidence
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		en := f.Entities[id]
		has := false
		for _, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_SURFACE" {
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
		allowed := map[string]int{"BOUNDED_SURFACE": 0, "B_SPLINE_SURFACE": 7, "B_SPLINE_SURFACE_WITH_KNOTS": 5, "GEOMETRIC_REPRESENTATION_ITEM": 0, "RATIONAL_B_SPLINE_SURFACE": 1, "REPRESENTATION_ITEM": 1, "SURFACE": 0}
		if len(records) != len(allowed) {
			return fail()
		}
		for k, n := range allowed {
			r, ok := records[k]
			if !ok || len(r.Args) != n {
				return fail()
			}
		}
		b, k, w, name := records["B_SPLINE_SURFACE"], records["B_SPLINE_SURFACE_WITH_KNOTS"], records["RATIONAL_B_SPLINE_SURFACE"], records["REPRESENTATION_ITEM"]
		du, e := affineSplineSourceRat(b.Args[0])
		dv, e2 := affineSplineSourceRat(b.Args[1])
		if e != nil || e2 != nil || du.Cmp(big.NewRat(2, 1)) != 0 || dv.Cmp(big.NewRat(2, 1)) != 0 || name.Args[0].Kind != String {
			return fail()
		}
		if b.Args[2].Kind != Aggregate || len(b.Args[2].Items) != 3 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != 3 {
			return fail()
		}
		uk, _, _, e := affineSplineGreville(k.Args[0], k.Args[2], 2, 3)
		vk, _, _, e2 := affineSplineGreville(k.Args[1], k.Args[3], 2, 3)
		if e != nil || e2 != nil || len(k.Args[2].Items) != 2 || len(k.Args[3].Items) != 2 || !affineSplineKnotSpec(k.Args[4], k.Args[0], k.Args[2], 2) || !affineSplineKnotSpec(k.Args[4], k.Args[1], k.Args[3], 2) {
			return fail()
		}
		ev := TensorRationalAffineSurfaceEvidence{SurfaceID: id, UKnots: [2]string{uk[0], uk[1]}, VKnots: [2]string{vk[0], vk[1]}}
		var p [3][3][3]*big.Rat
		var weights [3][3]*big.Rat
		differentU, differentV := false, false
		for i := 0; i < 3; i++ {
			ids, e := listRefs(b.Args[2].Items[i], 3)
			if e != nil || len(ids) != 3 || w.Args[0].Items[i].Kind != Aggregate || len(w.Args[0].Items[i].Items) != 3 {
				return fail()
			}
			for j, pid := range ids {
				ev.PoleIDs[i][j] = pid
				if len(incoming[pid]) != 1 || incoming[pid][0] != id {
					return fail()
				}
				pr, e := simple(f, pid, "CARTESIAN_POINT", 2)
				if e != nil || pr.Args[0].Kind != String || pr.Args[1].Kind != Aggregate || len(pr.Args[1].Items) != 3 {
					return fail()
				}
				weights[i][j], e = affineSplineSourceRat(w.Args[0].Items[i].Items[j])
				if e != nil || weights[i][j].Sign() <= 0 {
					return fail()
				}
				ev.Weights[i][j] = weights[i][j].RatString()
				if i > 0 && weights[i][j].Cmp(weights[0][j]) != 0 {
					differentU = true
				}
				if j > 0 && weights[i][j].Cmp(weights[i][0]) != 0 {
					differentV = true
				}
				for c, v := range pr.Args[1].Items {
					p[i][j][c], e = affineSplineSourceRat(v)
					if e != nil {
						return fail()
					}
					ev.Poles[i][j][c] = p[i][j][c].RatString()
				}
			}
		}
		if !differentU || !differentV {
			return fail()
		}
		// Elevate N and W to degree (3,3), and compare N = W*(P00+u*DU+v*DV).
		// Bernstein multiplication by t uses i/(d+1); elevation uses that
		// coefficient on the prior pole and its complement on the current pole.
		for i := 0; i <= 3; i++ {
			for j := 0; j <= 3; j++ {
				for c := 0; c < 3; c++ {
					lhs, rhs := new(big.Rat), new(big.Rat)
					du := new(big.Rat).Sub(p[2][0][c], p[0][0][c])
					dv := new(big.Rat).Sub(p[0][2][c], p[0][0][c])
					for a := 0; a < 3; a++ {
						for z := 0; z < 3; z++ {
							eu, evv, tu, tv := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
							if a == i {
								eu.SetFrac64(int64(3-i), 3)
							}
							if a == i-1 {
								eu.SetFrac64(int64(i), 3)
								tu.Set(eu)
							}
							if z == j {
								evv.SetFrac64(int64(3-j), 3)
							}
							if z == j-1 {
								evv.SetFrac64(int64(j), 3)
								tv.Set(evv)
							}
							base := new(big.Rat).Mul(eu, evv)
							lhs.Add(lhs, new(big.Rat).Mul(new(big.Rat).Mul(base, weights[a][z]), p[a][z][c]))
							term := new(big.Rat).Mul(base, p[0][0][c])
							term.Add(term, new(big.Rat).Mul(new(big.Rat).Mul(tu, evv), du))
							term.Add(term, new(big.Rat).Mul(new(big.Rat).Mul(eu, tv), dv))
							rhs.Add(rhs, new(big.Rat).Mul(weights[a][z], term))
						}
					}
					if lhs.Num().BitLen() > 4096 || lhs.Denom().BitLen() > 4096 || rhs.Num().BitLen() > 4096 || rhs.Denom().BitLen() > 4096 || lhs.Cmp(rhs) != 0 {
						return fail()
					}
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
		// Exact degree-two polynomial tensor view, with original evidence retained.
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				pid := ev.PoleIDs[i][j]
				pe := f.Entities[pid]
				pr := pe.Records[0]
				for c := 0; c < 3; c++ {
					q := new(big.Rat).Set(p[0][0][c])
					q.Add(q, new(big.Rat).Mul(big.NewRat(int64(i), 2), new(big.Rat).Sub(p[2][0][c], p[0][0][c])))
					q.Add(q, new(big.Rat).Mul(big.NewRat(int64(j), 2), new(big.Rat).Sub(p[0][2][c], p[0][0][c])))
					text, e := extrusionDirectionDecimal(q)
					if e != nil {
						return fail()
					}
					pr.Args[1].Items[c] = Value{Kind: Number, Text: text}
				}
				pe.Records[0] = pr
				f.Entities[pid] = pe
			}
		}
		args := []Value{name.Args[0], b.Args[0], b.Args[1], b.Args[2], b.Args[3], b.Args[4], b.Args[5], b.Args[6], k.Args[0], k.Args[1], k.Args[2], k.Args[3], Value{Kind: Enumeration, Text: "PIECEWISE_BEZIER_KNOTS"}}
		en.Records = []Record{{Type: "B_SPLINE_SURFACE_WITH_KNOTS", Args: args}}
		f.Entities[id] = en
		out = append(out, ev)
		if len(out) > 128 {
			return fail()
		}
	}
	if len(out) == 0 {
		return fail()
	}
	return out, nil
}
func ReadTensorRationalAffineNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (TensorRationalAffineNURBSPlanarAssembly, error) {
	var zero TensorRationalAffineNURBSPlanarAssembly
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
	return TensorRationalAffineNURBSPlanarAssembly{p.Geometry, string(raw), p, supports}, nil
}
func WriteTensorRationalAffineNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m TensorRationalAffineNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadTensorRationalAffineNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := tensorRationalAffineNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := tensorRationalAffineNURBSNativeClaim(ctx, p)
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

package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// RationalAffineSurfaceEvidence is the original degree-2 by degree-1
// rational tensor, proved affine by homogeneous Bernstein identities.
type RationalAffineSurfaceEvidence struct {
	SurfaceID, FaceID int64
	PoleIDs           [3][2]int64
	Poles             [3][2][3]string
	Weights           [3][2]string
	UKnots, VKnots    [2]string
}
type RationalAffineNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	planar   MultispanNURBSPlanarAssembly
	supports []RationalAffineSurfaceEvidence
}

func (m RationalAffineNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m RationalAffineNURBSPlanarAssembly) RationalSupports() []RationalAffineSurfaceEvidence {
	return append([]RationalAffineSurfaceEvidence(nil), m.supports...)
}
func (m RationalAffineNURBSPlanarAssembly) PCurves() []MultispanNURBSPCurveEvidence {
	return m.planar.PCurves()
}

func rationalAffineSurfaceView(ctx context.Context, f *File) ([]RationalAffineSurfaceEvidence, error) {
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
	fail := func() ([]RationalAffineSurfaceEvidence, error) { return nil, geom.ErrInvalid }
	var out []RationalAffineSurfaceEvidence
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
		if e != nil || e2 != nil || du.Cmp(big.NewRat(2, 1)) != 0 || dv.Cmp(big.NewRat(1, 1)) != 0 || name.Args[0].Kind != String {
			return fail()
		}
		if b.Args[2].Kind != Aggregate || len(b.Args[2].Items) != 3 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != 3 {
			return fail()
		}
		uk, _, _, e := affineSplineGreville(k.Args[0], k.Args[2], 2, 3)
		vk, _, _, e2 := affineSplineGreville(k.Args[1], k.Args[3], 1, 2)
		if e != nil || e2 != nil || len(k.Args[2].Items) != 2 || len(k.Args[3].Items) != 2 || !affineSplineKnotSpec(k.Args[4], k.Args[0], k.Args[2], 2) || !affineSplineKnotSpec(k.Args[4], k.Args[1], k.Args[3], 1) {
			return fail()
		}
		ev := RationalAffineSurfaceEvidence{SurfaceID: id, UKnots: [2]string{uk[0], uk[1]}, VKnots: [2]string{vk[0], vk[1]}}
		var p [3][2][3]*big.Rat
		var weights [3][2]*big.Rat
		different := false
		for i := 0; i < 3; i++ {
			ids, e := listRefs(b.Args[2].Items[i], 2)
			if e != nil || len(ids) != 2 || w.Args[0].Items[i].Kind != Aggregate || len(w.Args[0].Items[i].Items) != 2 {
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
				if weights[i][j].Cmp(weights[0][0]) != 0 {
					different = true
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
		if !different {
			return fail()
		}
		// Elevate N and W to degree (3,2), and compare N = W*(P00+u*DU+v*DV).
		// Bernstein multiplication by t uses i/(d+1); elevation uses that
		// coefficient on the prior pole and its complement on the current pole.
		for i := 0; i <= 3; i++ {
			for j := 0; j <= 2; j++ {
				for c := 0; c < 3; c++ {
					lhs, rhs := new(big.Rat), new(big.Rat)
					du := new(big.Rat).Sub(p[2][0][c], p[0][0][c])
					dv := new(big.Rat).Sub(p[0][1][c], p[0][0][c])
					for a := 0; a < 3; a++ {
						for z := 0; z < 2; z++ {
							eu, evv, tu, tv := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
							if a == i {
								eu.SetFrac64(int64(3-i), 3)
							}
							if a == i-1 {
								eu.SetFrac64(int64(i), 3)
								tu.Set(eu)
							}
							if z == j {
								evv.SetFrac64(int64(2-j), 2)
							}
							if z == j-1 {
								evv.SetFrac64(int64(j), 2)
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
		// Same entity/pole identities in the private polynomial view. Middle poles
		// are exact endpoint averages; original rational poles stay in evidence/source.
		for j := 0; j < 2; j++ {
			pid := ev.PoleIDs[1][j]
			pe := f.Entities[pid]
			pr := pe.Records[0]
			for c := 0; c < 3; c++ {
				q := new(big.Rat).Quo(new(big.Rat).Add(p[0][j][c], p[2][j][c]), big.NewRat(2, 1))
				s, e := extrusionDirectionDecimal(q)
				if e != nil {
					return fail()
				}
				pr.Args[1].Items[c] = Value{Kind: Number, Text: s}
			}
			pe.Records[0] = pr
			f.Entities[pid] = pe
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
func ReadRationalAffineNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (RationalAffineNURBSPlanarAssembly, error) {
	var zero RationalAffineNURBSPlanarAssembly
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
	supports, e := rationalAffineSurfaceView(ctx, f)
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
	return RationalAffineNURBSPlanarAssembly{p.Geometry, string(raw), p, supports}, nil
}
func WriteRationalAffineNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m RationalAffineNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadRationalAffineNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := rationalAffineNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := rationalAffineNURBSNativeClaim(ctx, p)
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

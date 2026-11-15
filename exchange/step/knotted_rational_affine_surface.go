package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// KnottedRationalSurfaceEvidence retains the original four-by-four rational
// tensor and both internal knots. Degree is two; multiplicities are (3,1,3).
type KnottedRationalSurfaceEvidence struct {
	SurfaceID, FaceID int64
	PoleIDs           [4][4]int64
	Poles             [4][4][3]string
	Weights           [4][4]string
	UKnots, VKnots    [3]string
	Multiplicity      [3]int
}

func knottedRationalAffineSurfaceView(ctx context.Context, f *File) ([]KnottedRationalSurfaceEvidence, error) {
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
	fail := func() ([]KnottedRationalSurfaceEvidence, error) { return nil, geom.ErrInvalid }
	var out []KnottedRationalSurfaceEvidence
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
		if b.Args[2].Kind != Aggregate || len(b.Args[2].Items) != 4 || w.Args[0].Kind != Aggregate || len(w.Args[0].Items) != 4 {
			return fail()
		}
		uk, _, gu, e := affineSplineGreville(k.Args[0], k.Args[2], 2, 4)
		vk, _, gv, e2 := affineSplineGreville(k.Args[1], k.Args[3], 2, 4)
		if e != nil || e2 != nil || len(k.Args[2].Items) != 3 || len(k.Args[3].Items) != 3 || !affineSplineKnotSpec(k.Args[4], k.Args[0], k.Args[2], 2) || !affineSplineKnotSpec(k.Args[4], k.Args[1], k.Args[3], 2) {
			return fail()
		}
		ev := KnottedRationalSurfaceEvidence{SurfaceID: id, UKnots: [3]string{uk[0], uk[1], uk[2]}, VKnots: [3]string{vk[0], vk[1], vk[2]}, Multiplicity: [3]int{3, 1, 3}}
		var p [4][4][3]*big.Rat
		var weights [4][4]*big.Rat
		differentU, differentV := false, false
		for i := 0; i < 4; i++ {
			ids, e := listRefs(b.Args[2].Items[i], 4)
			if e != nil || len(ids) != 4 || w.Args[0].Items[i].Kind != Aggregate || len(w.Args[0].Items[i].Items) != 4 {
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
		var ku, kv [3]*big.Rat
		for i := 0; i < 3; i++ {
			ku[i], e = affineSplineSourceRat(k.Args[2].Items[i])
			if e != nil {
				return fail()
			}
			kv[i], e = affineSplineSourceRat(k.Args[3].Items[i])
			if e != nil {
				return fail()
			}
		}
		if e = proveKnottedRationalAffine(p, weights, ku, kv); e != nil {
			return fail()
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
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				pid := ev.PoleIDs[i][j]
				pe := f.Entities[pid]
				pr := pe.Records[0]
				for c := 0; c < 3; c++ {
					q := new(big.Rat).Set(p[0][0][c])
					q.Add(q, new(big.Rat).Mul(gu[i], new(big.Rat).Sub(p[3][0][c], p[0][0][c])))
					q.Add(q, new(big.Rat).Mul(gv[j], new(big.Rat).Sub(p[0][3][c], p[0][0][c])))
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
		args := []Value{name.Args[0], b.Args[0], b.Args[1], b.Args[2], b.Args[3], b.Args[4], b.Args[5], b.Args[6], k.Args[0], k.Args[1], k.Args[2], k.Args[3], k.Args[4]}
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

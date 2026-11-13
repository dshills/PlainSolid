package trimmed

import (
	"context"
	"math/big"
	"sort"
	"testing"
)

func TestStoredMixedSharedEdgePrescribedCuts(t *testing.T) {
	s := solidEllipticalLensFixture(t)
	options := []SolidMixedFaceMeshOptions{{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .4}}}}, {Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .005}}}}}
	original := make([]CertifiedSolidMixedFaceMesh, 2)
	for i := range original {
		var e error
		original[i], e = s.TessellateMixedFaceCertified(context.Background(), i, options[i])
		if e != nil {
			t.Fatal(e)
		}
	}
	edge := original[0].Coedges[0].Edge
	find := func(r CertifiedSolidMixedFaceMesh) MixedFaceCoedgeSamples {
		for _, c := range r.Coedges {
			if c.Edge == edge {
				return c
			}
		}
		t.Fatal("missing common original edge")
		return MixedFaceCoedgeSamples{}
	}
	a, b := find(original[0]), find(original[1])
	if len(a.Vertices) == len(b.Vertices) {
		t.Fatal("fixture needs distinct adaptive partitions", len(a.Vertices))
	}
	values := []*big.Rat{}
	for _, c := range []MixedFaceCoedgeSamples{a, b} {
		for _, q := range c.ModelParameters {
			values = append(values, new(big.Rat).Set(q))
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
	union := []*big.Rat{}
	for _, q := range values {
		if len(union) == 0 || union[len(union)-1].Cmp(q) != 0 {
			union = append(union, q)
		}
	}
	t.Logf("shared original edge%d: source samples%d/%d, common%d", edge, len(a.Vertices), len(b.Vertices), len(union))
	for fi := 0; fi < 2; fi++ {
		originalUse := find(original[fi])
		f := s.faces[fi]
		loops := append([][]Use{f.Outer}, f.Holes...)
		cuts := make([][][]*big.Rat, len(loops))
		for li, l := range loops {
			cuts[li] = make([][]*big.Rat, len(l))
		}
		model := s.edges[edge]
		trim := make([]*big.Rat, len(union))
		for j, q := range union {
			v := new(big.Rat).Quo(new(big.Rat).Sub(q, ratInset(model.Range.Min)), new(big.Rat).Sub(ratInset(model.Range.Max), ratInset(model.Range.Min)))
			if originalUse.UseReversed {
				v.Sub(big.NewRat(1, 1), v)
			}
			trim[j] = v
		}
		sort.Slice(trim, func(i, j int) bool { return trim[i].Cmp(trim[j]) < 0 })
		cuts[originalUse.Loop][originalUse.Use] = trim
		options[fi].Face.Face.Cuts = cuts
		r, e := s.TessellateMixedFaceCertified(context.Background(), fi, options[fi])
		if e != nil {
			t.Fatal("prescribed stored face", fi, e)
		}
		current := find(r)
		for _, q := range union {
			found := false
			for j, tparam := range current.ModelParameters {
				if tparam.Cmp(q) == 0 {
					found = true
					f, _ := q.Float64()
					point, e := model.Curve.Eval(f)
					if e != nil || point.Sub(r.Mesh.Vertices[current.Vertices[j]]).Length() > 1e-10 {
						t.Fatal("model occurrence", e)
					}
					break
				}
			}
			if !found {
				t.Fatal("lost common model parameter", q)
			}
		}
		cuts[originalUse.Loop][originalUse.Use] = []*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
		if denied, e := s.TessellateMixedFaceCertified(context.Background(), fi, options[fi]); e == nil || len(denied.Mesh.Vertices) != 0 {
			t.Fatal("duplicate prescribed cut")
		}
	}
}

func TestMixedExactHolePrescribedDirection(t *testing.T) {
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	cuts := [][][]*big.Rat{make([][]*big.Rat, 4), make([][]*big.Rat, 4)}
	cuts[1][0] = []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)}
	o := MixedExactAffineMeshOptions{}
	o.Display.Deflection = .01
	r, e := TessellateMixedExactAffineRegionWithCutsCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{hole}, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, s := range r.Samples {
		if s.Loop != 1 || s.Trim != 0 {
			continue
		}
		for j, q := range s.Parameters {
			if q.Cmp(big.NewRat(1, 3)) == 0 {
				found = true
				uv, e := mixedExactEvaluate(hole[0], q)
				if e != nil {
					t.Fatal(e)
				}
				for k := range uv {
					if uv[k].Cmp(r.ExactUV[s.Vertices[j]][k]) != 0 {
						t.Fatal("original hole direction")
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("nonbinary hole cut omitted")
	}
}

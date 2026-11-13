package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactGraphPartitionActualScaledChartMismatch(t *testing.T) {
	_, a, _, _ := bezierSeamFixture(t, 3)
	h := a.HomogeneousControls()
	for i := range h {
		h[i][0].Mul(h[i][0], big.NewRat(8, 1))
	}
	b, e := NewExactBezierUV(h)
	if e != nil {
		t.Fatal(e)
	}
	patch := func(scale float64) *geom.SurfacePatch {
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, scale, scale}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	pa, pb := patch(1), patch(8)
	ca, _ := a.descriptor(context.Background(), geom.SurfacePatchOptions{})
	cb, _ := b.descriptor(context.Background(), geom.SurfacePatchOptions{})
	proof, e := geom.CompareSurfaceCurveCompositions(context.Background(), pa, ca, pb, cb, geom.SurfaceCurveAgreementOptions{})
	if e != nil || !proof.Complete || !proof.Equal {
		t.Fatal("independent exact model identity", e)
	}
	o := CurvedBezierGraphChainOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	ra, e := TessellateCurvedBezierGraphChainCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	rb, e := TessellateCurvedBezierGraphChainCertified(context.Background(), pb, []ExactBezierUV{b}, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	extract := func(r CertifiedCurvedBezierGraphChain) [][]*big.Rat {
		out := [][]*big.Rat{{}}
		for _, v := range r.GraphChain.Pieces[0].Curve.Vertices {
			if v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
				t.Fatal("not rational")
			}
			out[0] = append(out[0], v.Parameter.Lower)
		}
		return out
	}
	aa, bb := extract(ra), extract(rb)
	if len(aa[0]) == len(bb[0]) {
		t.Fatal("fixture did not produce mismatched partitions", len(aa[0]))
	}
	plan, e := PlanExactGraphChainPartition(context.Background(), aa, bb, ExactGraphChainPartitionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for owner, input := range [][][]*big.Rat{aa, bb} {
		indices := plan.Pieces[0].OriginalA
		if owner == 1 {
			indices = plan.Pieces[0].OriginalB
		}
		for i, q := range input[0] {
			if plan.Pieces[0].Parameters[indices[i]].Cmp(q) != 0 {
				t.Fatal("source occurrence lost")
			}
		}
	}
	t.Logf("source A=%d B=%d common=%d", len(aa[0]), len(bb[0]), plan.Parameters)
	for owner, patch := range []*geom.SurfacePatch{pa, pb} {
		trim := a
		if owner == 1 {
			trim = b
		}
		fixed, err := TessellateExactBezierGraphChainWithCutsCertified(context.Background(), patch, []ExactBezierUV{trim}, new(big.Rat), [][]*big.Rat{plan.Pieces[0].Parameters}, ExactBezierGraphFaceOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .002}}})
		if err != nil {
			t.Fatal("fixed consumer", owner, err)
		}
		line := fixed.Pieces[0].Curve
		for _, q := range plan.Pieces[0].Parameters {
			found := false
			for _, v := range line.Vertices {
				if v.Parameter.Lower.Cmp(q) == 0 && v.Parameter.Upper.Cmp(q) == 0 {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("mandatory cut omitted", q)
			}
		}
		if !line.Complete || line.MaximumError > .002 || fixed.FillSquaredDistance.Sign() < 0 || fixed.PolygonUVArea.Sign() <= 0 {
			t.Fatal("missing recertification")
		}
		if len(fixed.Mesh.Triangles) != len(fixed.Boundary)-2 {
			t.Fatal("polygon coverage triangle count")
		}
	}

	refined, err := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{b}, new(big.Rat), o, 2)
	if err != nil {
		t.Fatal("refined pair", err)
	}
	if len(refined.SharedVertices) < plan.Parameters {
		t.Fatal("common samples lost")
	}
	for _, v := range refined.SharedVertices {
		if refined.FaceA.Mesh.Vertices[v[0]] != refined.FaceB.Mesh.Vertices[v[1]] {
			t.Fatal("not bit equal")
		}
	}
	// Planner does not silently widen the existing strict acceptance boundary.
	if _, e = TessellatePairedBezierGraphChainsCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{b}, new(big.Rat), o); e == nil {
		t.Fatal("strict pair unexpectedly accepts unmatched partitions")
	}
}
func TestExactGraphPartitionBudgetsAndOwnership(t *testing.T) {
	a := [][]*big.Rat{{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)}}
	b := [][]*big.Rat{{new(big.Rat), big.NewRat(2, 3), big.NewRat(1, 1)}}
	r, e := PlanExactGraphChainPartition(context.Background(), a, b, ExactGraphChainPartitionOptions{})
	if e != nil || r.Parameters != 4 {
		t.Fatal(r, e)
	}
	a[0][1].SetInt64(0)
	if r.Pieces[0].Parameters[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("aliased input")
	}
	if _, e = PlanExactGraphChainPartition(context.Background(), a, b, ExactGraphChainPartitionOptions{}); e == nil {
		t.Fatal("duplicate accepted")
	}
	a[0][1].SetFrac64(1, 3)
	if r, e := PlanExactGraphChainPartition(context.Background(), a, b, ExactGraphChainPartitionOptions{MaxParameters: 5}); e == nil || len(r.Pieces) != 0 {
		t.Fatal("raw aggregate budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = PlanExactGraphChainPartition(ctx, a, b, ExactGraphChainPartitionOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestExactGraphFixedCutsAtomicLimits(t *testing.T) {
	_, q, _, _ := bezierSeamFixture(t, 3)
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cuts := [][]*big.Rat{{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)}}
	positive, err := TessellateExactBezierGraphChainWithCutsCertified(context.Background(), p, []ExactBezierUV{q}, new(big.Rat), cuts, ExactBezierGraphFaceOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .01}}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range positive.Pieces[0].Curve.Vertices {
		if v.Parameter.Lower.Cmp(big.NewRat(1, 3)) == 0 && v.Parameter.Upper.Cmp(big.NewRat(1, 3)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("nonbinary cut lost")
	}
	o := ExactBezierGraphFaceOptions{}
	o.Display.MaxOperations = 1
	r, e := TessellateExactBezierGraphChainWithCutsCertified(context.Background(), p, []ExactBezierUV{q}, new(big.Rat), cuts, o)
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("partial budget success")
	}
	cuts[0][1].SetInt64(0)
	if r, e = TessellateExactBezierGraphChainWithCutsCertified(context.Background(), p, []ExactBezierUV{q}, new(big.Rat), cuts, ExactBezierGraphFaceOptions{}); e == nil || len(r.Pieces) != 0 {
		t.Fatal("duplicate cut")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cuts[0][1].SetFrac64(1, 3)
	if _, e = TessellateExactBezierGraphChainWithCutsCertified(ctx, p, []ExactBezierUV{q}, new(big.Rat), cuts, ExactBezierGraphFaceOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

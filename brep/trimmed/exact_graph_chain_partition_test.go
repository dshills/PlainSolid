package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

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

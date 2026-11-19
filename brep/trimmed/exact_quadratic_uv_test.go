package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactQuadraticEdgeAgreementAcrossIrrationalKnot(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := p.InsertVKnot(.5, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, e := refined.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewExactQuadraticUV([3][3]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 2), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)},
	})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := q.ModelCurvePieces(ctx, refined, geom.SurfaceCurveCompositionOptions{}); !errors.Is(e, geom.ErrConvergence) || len(r.Unresolved) != 1 {
		t.Fatal(r, e)
	}
	model := parametric.Edge{Curve: surfaceTrimCurve{s, q.ends[0], q.ends[1], q}, Range: parametric.Interval{Max: 1}}
	use := Use{Curve: q, UVStart: q.ends[0], UVEnd: q.ends[1]}
	if ok, e := compareSplineEdgeUse(ctx, model, other, use, edgecheck.Options{}); e != nil || !ok {
		t.Fatal("continuous identity does not require constructing rational pieces", ok, e)
	}
	use.Curve = q.reversed()
	use.UVStart, use.UVEnd, use.Reversed = use.UVEnd, use.UVStart, true
	if ok, e := compareSplineEdgeUse(ctx, model, other, use, edgecheck.Options{}); e != nil || !ok {
		t.Fatal("opposite incidence lost an algebraic cut", ok, e)
	}
}

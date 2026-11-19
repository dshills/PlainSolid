package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeLowerParabolaTwoCirclesGraphRegions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaTwoCirclesStock(t, 1, [2]twoCircleStockProbe{
		twoCircleProbe(4, 2, 1, 16), twoCircleProbe(24, 4, 1, 32),
	}, false)
	r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 1, parametric.Intersection, 1e-9, opts)
	if err != nil || r.History().EnclosingRoof != 0 {
		t.Fatal("both lower circles must trim the lower roof region", err)
	}
	for order := range 2 {
		cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], 1, 1e-9, opts)
		if err != nil {
			t.Fatal(err)
		}
		h := cut.History()
		if order == 0 && (len(h) != 1 || h[0].Region != 0 || len(h[0].CircleEdges) != 8) {
			t.Fatal("wrong lower layer through-hole ownership", h)
		}
		if order == 1 && (len(h) != 3 || h[0].Region != 1 || h[1].Region != 2 || h[2].Region != 3) {
			t.Fatal("wrong separate upper cap and disk ownership", h)
		}
	}
}

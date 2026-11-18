package trimmed

import (
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestNativeStackedParabolaCirclesCutOwnerOrder(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	circles := [2]twoCircleStockProbe{twoCircleProbe(8, 9, 1, 16), twoCircleProbe(16, 27, 3, 32)}
	src := parabolaTwoCirclesStock(t, 0, circles, false)
	cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parts, history := cut.Components(), cut.History()
	if len(parts) != 3 || len(history) != 3 {
		t.Fatal("aligned free centers lost one of the two distinct lenses")
	}
	for i, c := range circles {
		checkNativeOffsetCircleEdges(t, parts[i+1], 0, history[i+1].CircleEdges, c)
	}
}

func checkNativeOffsetCircleEdges(t *testing.T, body Solid, axis int, ring []int, circle twoCircleStockProbe) {
	t.Helper()
	for _, id := range ring {
		edge := body.edges[id]
		for _, index := range [2]int{edge.Start, edge.End} {
			p := body.vertices[index]
			xy := [2]float64{p.X, p.Y}
			x := new(big.Rat).Sub(ratInset(xy[axis]), circle.x)
			y := new(big.Rat).Sub(ratInset(xy[1-axis]), circle.y)
			radius := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
			if p.Z != 2 || radius.Cmp(circle.radiusSquared) != 0 {
				t.Fatal("native circular edge displaced or swapped a source owner", p, circle)
			}
		}
	}
}

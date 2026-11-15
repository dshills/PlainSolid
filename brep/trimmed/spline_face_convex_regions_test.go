package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCoincidentConvexRegionsHoleAndDisconnected(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	b.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 100, false)
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop([]geom.Vec2{{X: .5, Y: .25}, {X: .25, Y: .5}, {X: .5, Y: .75}, {X: .75, Y: .5}}, 20)}
	b.faces[0].Holes = nil
	r, e := a.BuildCoincidentConvexRegions(context.Background(), 0, b, 0, CoincidentConvexRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 1 || r.UVArea[0].Cmp(big.NewRat(7, 8)) != 0 {
		t.Fatal("diamond hole", e)
	}
	if r.Components[0].Holes[0].SignedAreaA.Cmp(big.NewRat(-1, 8)) != 0 || r.CanceledSeams == 0 {
		t.Fatal("hole winding or no subdivision")
	}
	for _, loop := range append([]CoincidentChartLoop{r.Components[0].Outer}, r.Components[0].Holes...) {
		for _, edge := range loop.Edges {
			if len(edge.Sources) == 0 || edge.CurveA == nil || edge.CurveB == nil {
				t.Fatal("missing exact original edge")
			}
		}
	}
	b.faces[0].Outer = overlapRectangle(0, 1, .4375, .5625, 100, false)
	r, e = a.BuildCoincidentConvexRegions(context.Background(), 0, b, 0, CoincidentConvexRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 {
		t.Fatal("disconnected", e)
	}
	// Integral of retained width: 1/2+2*abs(y) for y measured from
	// midline, yielding 9/128 across the symmetric strip.
	if r.UVArea[0].Cmp(big.NewRat(9, 128)) != 0 {
		t.Fatal("independent strip area", r.UVArea[0])
	}
	b.faces[0].Outer = overlapRectangle(0, 1, .5, .75, 100, false)
	if q, e := a.BuildCoincidentConvexRegions(context.Background(), 0, b, 0, CoincidentConvexRegionOptions{}); e == nil || q.Complete {
		t.Fatal("pinched top point accepted")
	}
	b.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 100, false)
	if q, e := a.BuildCoincidentConvexRegions(context.Background(), 0, b, 0, CoincidentConvexRegionOptions{MaxSplitWork: 1}); e == nil || q.Complete || len(q.Components) != 0 {
		t.Fatal("atomic splitting quota")
	}
}

func TestCoincidentConvexRegionsMappedRationalBoundary(t *testing.T) {
	a, b := convexOverlapFixture(t)
	r, e := a.BuildCoincidentConvexRegions(context.Background(), 0, b, 0, CoincidentConvexRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || r.UVArea[0].Cmp(big.NewRat(13, 84)) != 0 || r.UVArea[1].Cmp(big.NewRat(65, 28)) != 0 {
		t.Fatal("mapped region", e)
	}
	for _, edge := range r.Components[0].Outer.Edges {
		for _, source := range edge.Sources {
			face, points := a.faces[0], edge.A
			if source.Owner.Source == 1 {
				face, points = b.faces[0], edge.B
			}
			u := face.Outer[source.Owner.Use]
			for i, v := range source.Range {
				for k, ends := range [][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
					exact := new(big.Rat).Add(ratInset(ends[0]), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))))
					if exact.Cmp(points[i][k]) != 0 {
						t.Fatal("source range lost")
					}
				}
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := a.BuildCoincidentConvexRegions(ctx, 0, b, 0, CoincidentConvexRegionOptions{}); e == nil || q.Complete {
		t.Fatal("cancelled result")
	}
}

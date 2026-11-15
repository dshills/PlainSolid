package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func checkChartRegionCurves(t *testing.T, r CoincidentChartRegions) {
	t.Helper()
	for _, component := range r.Components {
		loops := append([]CoincidentChartLoop{component.Outer}, component.Holes...)
		for li, loop := range loops {
			if li == 0 && loop.SignedAreaA.Sign() <= 0 || li > 0 && loop.SignedAreaA.Sign() >= 0 {
				t.Fatal("wrong material incidence")
			}
			for i, e := range loop.Edges {
				next := loop.Edges[(i+1)%len(loop.Edges)]
				for k := 0; k < 2; k++ {
					if e.A[1][k].Cmp(next.A[0][k]) != 0 {
						t.Fatal("open exact loop")
					}
				}
				for side, c := range []*geom.ExactSplineCurve{e.CurveA, e.CurveB} {
					net, _, err := c.BezierControlNet(context.Background(), geom.SurfacePatchOptions{})
					if err != nil {
						t.Fatal(err)
					}
					points := e.A
					if side == 1 {
						points = e.B
					}
					for j := 0; j < 2; j++ {
						for k := 0; k < 2; k++ {
							if new(big.Rat).Quo(net.Control[j][k], net.Control[j][3]).Cmp(points[j][k]) != 0 {
								t.Fatal("native rational curve lost source")
							}
						}
					}
				}
				if len(e.Sources) == 0 {
					t.Fatal("unowned exposed boundary")
				}
			}
		}
	}
}
func TestCoincidentChartRegionsOperationsAndHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	a.faces[0].Holes = nil
	b.faces[0].Outer = overlapRectangle(2.75, 4.25, .25, 2.75, 100, false)
	b.faces[0].Holes = nil
	for _, tc := range []struct {
		op    parametric.BooleanOp
		area  *big.Rat
		holes int
	}{{parametric.Intersection, big.NewRat(1, 4), 0}, {parametric.Difference, big.NewRat(3, 4), 1}, {parametric.Union, big.NewRat(1, 1), 0}} {
		r, e := a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, tc.op, CoincidentChartRegionOptions{})
		if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != tc.holes || r.UVArea[0].Cmp(tc.area) != 0 {
			t.Fatal(r, e)
		}
		checkChartRegionCurves(t, r)
		if tc.op != parametric.Intersection && r.CanceledSeams == 0 {
			t.Fatal("internal seams not canceled")
		}
		if tc.holes == 1 {
			for _, edge := range r.Components[0].Holes[0].Edges {
				if len(edge.Sources) != 1 || edge.Sources[0].Owner.Source != 1 {
					t.Fatal("hole tool ancestry", edge.Sources)
				}
			}
		}
	}
}
func TestCoincidentChartRegionsDisconnectedAndPinch(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{X: .125, Y: .125}, {X: .875, Y: .125}, {X: .875, Y: .875}, {X: .625, Y: .875}, {X: .625, Y: .375}, {X: .375, Y: .375}, {X: .375, Y: .875}, {X: .125, Y: .875}}, 10)
	b.faces[0].Outer = overlapRectangle(0, 1, .5, .75, 100, false)
	r, e := a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Intersection, CoincidentChartRegionOptions{})
	if e != nil || len(r.Components) != 2 || r.UVArea[0].Cmp(big.NewRat(1, 8)) != 0 {
		t.Fatal(r, e)
	}
	checkChartRegionCurves(t, r)
	a.faces[0].Outer = overlapRectangle(0, .5, 0, .5, 10, false)
	b.faces[0].Outer = overlapRectangle(.5, 1, .5, 1, 100, false)
	if r, e = a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Union, CoincidentChartRegionOptions{}); e == nil || r.Complete || len(r.Components) != 0 {
		t.Fatal("pinched rings welded", r, e)
	}
	r, e = a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Intersection, CoincidentChartRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 0 || r.UVArea[0].Sign() != 0 {
		t.Fatal("isolated point became material", r, e)
	}
}
func TestCoincidentChartRegionsLimitsAndSharedOwners(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	b.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 100, false)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	r, e := a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Union, CoincidentChartRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, edge := range r.Components[0].Outer.Edges {
		if len(edge.Sources) != 2 {
			t.Fatal("lost both original owners")
		}
	}
	if r, e = a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Union, CoincidentChartRegionOptions{MaxBoundaryEdges: 3}); e == nil || r.Complete || len(r.Components) != 0 {
		t.Fatal("atomic limit", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e = a.BuildCoincidentChartRegions(ctx, 0, b, 0, parametric.Union, CoincidentChartRegionOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestCoincidentChartRegionsNonbinaryNativeHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = overlapRectangle(.125, .875, .125, .875, 10, false)
	a.faces[0].Holes = nil
	b.faces[0].Outer = overlapRectangle(3, 4, 0, 2, 100, false)
	b.faces[0].Holes = nil
	r, e := a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Difference, CoincidentChartRegionOptions{})
	want := new(big.Rat).Sub(big.NewRat(9, 16), big.NewRat(2, 15))
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 1 || r.UVArea[0].Cmp(want) != 0 {
		t.Fatal(r, e)
	}
	checkChartRegionCurves(t, r)
	found := false
	for _, edge := range r.Components[0].Holes[0].Edges {
		for _, p := range edge.A {
			for _, v := range p {
				if v.Cmp(big.NewRat(1, 5)) == 0 || v.Cmp(big.NewRat(1, 3)) == 0 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("nonbinary source cuts rounded")
	}
}

func TestCoincidentChartRegionsIslandInsideHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	a.faces[0].Holes = [][]Use{overlapRectangle(.25, .75, .25, .75, 20, true)}
	b.faces[0].Outer = overlapRectangle(.375, .625, .375, .625, 100, false)
	b.faces[0].Holes = nil
	r, e := a.BuildCoincidentChartRegions(context.Background(), 0, b, 0, parametric.Union, CoincidentChartRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 || r.UVArea[0].Cmp(big.NewRat(13, 16)) != 0 {
		t.Fatal(r, e)
	}
	holes := 0
	for _, c := range r.Components {
		holes += len(c.Holes)
	}
	if holes != 1 {
		t.Fatal("island swallowed hole")
	}
	checkChartRegionCurves(t, r)
}

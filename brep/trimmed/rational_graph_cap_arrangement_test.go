package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalGraphPolygonalCapCutThroughHole(t *testing.T) {
	ctx := context.Background()
	stock := makeSquareRationalGraphTube(t)
	sec, e := stock.SectionRationalGraphSplineShell(ctx, [3]geom.Vec3{{Z: .5}, {X: 1, Z: .5}, {Y: 1, Z: .5}}, RationalGraphShellSectionOptions{Regions: &RationalGraphShellRegionOptions{}})
	if e != nil {
		t.Fatal(e)
	}
	cut := func(x float64) [3]geom.Vec3 { return [3]geom.Vec3{{X: x}, {X: x, Y: 1}, {X: x, Z: 1}} }
	for _, keep := range []bool{true, false} {
		r, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Cuts) != 4 || len(r.Seams) != 2 || len(r.Regions.Material) != 1 || len(r.Regions.Material[0].Holes) != 0 {
			t.Fatal("inner loop must become a notch with two disjoint cap seams", r)
		}
		area, e := r.Regions.BoundArea(ctx, big.NewRat(1, 1024))
		if e != nil || area.Projection.Lower.Cmp(big.NewRat(4, 1)) != 0 || area.Projection.Upper.Cmp(area.Projection.Lower) != 0 {
			t.Fatal("the exact symmetric half of the original annulus has area 4", area, e)
		}
		var ends [4]bool
		for _, seam := range r.Seams {
			a, b := r.Cuts[seam.From], r.Cuts[seam.To]
			if !a.KeepBefore || b.KeepBefore || a.Point[0].Cmp(big.NewRat(3, 2)) != 0 || b.Point[0].Cmp(big.NewRat(3, 2)) != 0 ||
				new(big.Rat).Abs(new(big.Rat).Sub(a.Point[1], b.Point[1])).Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("generated segment must span material, with original boundary ancestry", a, b)
			}
			for _, endpoint := range []int{seam.From, seam.To} {
				y := r.Cuts[endpoint].Point[1]
				if !y.IsInt() || y.Sign() < 0 || y.Cmp(big.NewRat(3, 1)) > 0 || ends[y.Num().Int64()] {
					t.Fatal("ambiguous terminal", endpoint, y)
				}
				ends[y.Num().Int64()] = true
			}
		}
		for _, q := range []struct {
			y float64
			l trimvalidate.Location
		}{{.5, trimvalidate.Boundary}, {1.5, trimvalidate.Outside}, {2.5, trimvalidate.Boundary}} {
			l, p, e := r.Regions.LocateRational(ctx, [3]*big.Rat{big.NewRat(3, 2), ratInset(q.y), big.NewRat(1, 2)})
			if e != nil || p.Status != trimvalidate.Certified || l != q.l {
				t.Fatal("line ownership across the removed hole", q, l, p, e)
			}
		}
	}
	if out, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1), true, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil || len(out.Cuts)+len(out.Seams) != 0 {
		t.Fatal("original vertex/coincident section must reject without partial output", e)
	}
	if out, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1.5), true, RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: 1}}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil {
		t.Fatal("budget exhaustion must reject atomically", e)
	}
}

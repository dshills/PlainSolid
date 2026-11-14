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

func TestRationalGraphCapCutsCurvedInnerWire(t *testing.T) {
	ctx := context.Background()
	// An outer square encloses a clockwise inner loop with a parabolic
	// top y=5/2-(x-1)²/2, three straight edges, and positive clearance.
	point := func(x, y float64) [3]*big.Rat { return [3]*big.Rat{ratInset(x), ratInset(y), big.NewRat(1, 1)} }
	line := func(a, b [3]*big.Rat) [][3]*big.Rat {
		var mid [3]*big.Rat
		for i := range mid {
			mid[i] = new(big.Rat).Quo(new(big.Rat).Add(a[i], b[i]), big.NewRat(2, 1))
		}
		return [][3]*big.Rat{a, mid, b}
	}
	a, b, c, d := point(0, 0), point(4, 0), point(4, 4), point(0, 4)
	w, x, y, z := point(1, 1), point(1, 2.5), point(3, .5), point(3, .25)
	controls := [][][][3]*big.Rat{{line(a, b), line(b, c), line(c, d), line(d, a)}, {line(w, x), {x, point(2, 2.5), y}, line(y, z), line(z, w)}}
	var wires [][]projectedGraphSectionPiece
	var rings []trimvalidate.Loop
	for i, row := range controls {
		var wire []projectedGraphSectionPiece
		var ring trimvalidate.Loop
		for j, h := range row {
			q, e := trimvalidate.ExactBezier(h)
			if e != nil {
				t.Fatal(e)
			}
			wire = append(wire, projectedGraphSectionPiece{RationalGraphCapEdgeSource{Wire: i, Branch: j, Face: j}, h})
			ring = append(ring, q)
		}
		wires, rings = append(wires, wire), append(rings, ring)
	}
	opts, _ := (trimvalidate.Options{}).WithDefaults()
	base, e := certifyProjectedRationalGraphRegions(ctx, wires, rings, horizontalPolynomialPlane(new(big.Rat)), opts)
	if e != nil {
		t.Fatal(e)
	}
	plane := [3]geom.Vec3{{X: 2}, {X: 2, Y: 1}, {X: 2, Z: 1}}
	if out, e := base.IntersectPolygonalHalfspace(ctx, plane, true, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil {
		t.Fatal("polygonal-only entry point accepted a genuinely curved inner edge", e)
	}
	out, e := base.IntersectRationalHalfspace(ctx, plane, true, RationalGraphShellRegionOptions{})
	if e != nil || len(out.Cuts) != 4 || len(out.Seams) != 2 || len(out.Regions.Material) != 1 || len(out.Regions.Material[0].Holes) != 0 {
		t.Fatal("rational inner wire must open into one notch", out, e)
	}
	// Half the outer square has area 8. Over x∈[2,3], subtract the
	// integral of the parabolic top above the linear lower edge: 43/48.
	area, e := out.Regions.BoundArea(ctx, big.NewRat(1, 1<<14))
	want := big.NewRat(341, 48)
	if e != nil || area.Projection.Lower.Cmp(want) > 0 || area.Projection.Upper.Cmp(want) < 0 {
		t.Fatal("independent retained curved cap area", want, area, e)
	}
	for _, cut := range out.Cuts {
		if cut.Source.Wire == 1 && cut.Source.Branch == 1 {
			if cut.Point[0].Cmp(big.NewRat(2, 1)) != 0 || cut.Point[1].Cmp(big.NewRat(2, 1)) != 0 || cut.Parameter.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("curved exact root and original parameter", cut)
			}
			return
		}
	}
	t.Fatal("missing curved inner terminal", out.Cuts)
}

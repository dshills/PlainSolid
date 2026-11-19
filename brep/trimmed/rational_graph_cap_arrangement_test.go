package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

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
			wire = append(wire, projectedGraphSectionPiece{source: RationalGraphCapEdgeSource{Wire: i, Branch: j, Face: j}, control: h})
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

// Compare true endpoint images, including seam and original continuations
// with different parameters. An overlapping enclosure is not sufficient.

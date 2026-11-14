package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestAlgebraicStockFootprintPerimeter(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	stock := makeSquareRationalGraphTube(t)
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	regions := RationalGraphShellRegionOptions{}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 12000000}
	planes := [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}
	p, err := stock.ClipAlgebraicGraphBand(ctx, planes, section, regions)
	if err != nil {
		t.Fatal(err)
	}
	band, err := p.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	interval := func(got geom.ExactParameterInterval, want *big.Rat) {
		t.Helper()
		if got.Lower.Cmp(want) > 0 || got.Upper.Cmp(want) < 0 ||
			new(big.Rat).Sub(got.Upper, got.Lower).Cmp(q(1, 2048)) > 0 {
			t.Fatal("exact perimeter enclosure", got, want)
		}
	}
	r, err := band.BoundFootprintPerimeter(ctx, q(1, 2048), opts)
	if err != nil {
		t.Fatal(err)
	}
	interval(r.Interval, q(16, 1)) // outer perimeter 12, inner perimeter 4
	interval(r.Seams, q(0, 1))
	interval(r.Stock, q(16, 1))
	for _, use := range r.Uses {
		if use.StockFace < 0 || use.Seam != -1 {
			t.Fatal("separated band may only have original wall owners", use)
		}
	}
	planes = [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: 5. / 16}, {X: 1, Z: 7. / 16}, {Y: 1, Z: 5. / 16}},
	}
	p, err = stock.ClipAlgebraicGraphCapPair(ctx, planes, section, regions)
	if err != nil {
		t.Fatal(err)
	}
	model, err := p.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, err = model.BoundFootprintPerimeter(ctx, q(1, 2048), opts)
	if err != nil {
		t.Fatal(err)
	}
	// The X>=3/2 rectangular footprint has perimeter 9. The
	// open notch adds two half-length segments while replacing an
	// equal vertical edge. The two generated seam stretches total 2.
	interval(r.Interval, q(10, 1))
	interval(r.Stock, q(8, 1))
	interval(r.Seams, q(2, 1))
	seams := 0
	for _, use := range r.Uses {
		if use.Seam >= 0 {
			seams++
			if use.StockFace != -1 {
				t.Fatal("generated seam was attributed to an original wall", use)
			}
		}
	}
	if seams != 2 {
		t.Fatal("only two distinct original seam stretches", seams)
	}
	bad, err := model.BoundFootprintPerimeter(ctx, q(1, 2048), geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Uses) != 0 || bad.Interval.Lower != nil {
		t.Fatal("partial projected length escaped a depleted budget", bad, err)
	}
}

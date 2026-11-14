package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCorrelatedRoofOrderAcrossSourceCharts(t *testing.T) {
	ctx := context.Background()
	// A=2+X and B=2+X+1/8. Separate height hulls overlap so they
	// cannot select an original roof for this entire footprint. B's
	// own U domain differs and uses a positive, nonconstant weight.
	r := big.NewRat
	a := &mixedRoofBox{
		domain: geom.ParameterRectangle{UMax: 1, VMax: 1}, floor: r(0, 1),
		linePower: roofPowerGrid{{r(2, 1)}, {r(1, 1)}},
	}
	b := &mixedRoofBox{
		domain: geom.ParameterRectangle{UMin: .25, UMax: 1.25, VMax: 1}, floor: r(1, 4),
		linePower: roofPowerGrid{{r(19, 8)}, {r(27, 8)}, {r(1, 1)}},
		weight:    roofPowerGrid{{r(1, 1)}, {r(1, 1)}, {r(0, 1)}}, minWeight: r(1, 1),
	}
	q := &MixedRoofMaterialQuery{op: parametric.Intersection, box: [2]*mixedRoofBox{a, b}}
	cells := mixedRoofVolumeFootprint(q)
	if len(cells) != 1 {
		t.Fatal(cells)
	}
	cell := cells[0]
	o := geom.SurfacePatchEvaluationOptions{MaxOperations: 10000, MaxCoefficientBits: 512}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	var h [2]geom.ExactParameterInterval
	for i, p := range cell.roof {
		uv, err := p.volumeRoofDomain(cell.xy, &w)
		if err != nil {
			t.Fatal(err)
		}
		h[i], err = p.roofHeightRange(uv, &w)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := q.fixedColumnPrimitives(cell.chart, h); ok {
		t.Fatal("fixture must need a joint height proof")
	}
	roof, floor, ok, err := q.correlatedColumnPrimitives(cell, h, make(roofOrderCache), &w)
	if err != nil || !ok || roof != ([2]int{1, 0}) || floor != ([2]int{0, -1}) {
		t.Fatal("must choose the original polynomial roof and cancel the weighted roof", roof, floor, ok, err)
	}
	volume, err := q.BoundVolume(ctx, MixedRoofVolumeOptions{SurfacePatchEvaluationOptions: o, MaxCells: 1, AbsoluteGap: r(1, 1000)})
	if err != nil || !volume.Converged || volume.Volume.Lower.Cmp(r(57, 32)) != 0 || volume.Volume.Upper.Cmp(r(57, 32)) != 0 {
		t.Fatal("exact original volume in one chart cell", volume, err)
	}
	mass, err := q.BoundMass(ctx, MixedRoofMassOptions{SurfacePatchEvaluationOptions: o, MaxCells: 1})
	if err != nil || mass.EnclosedCells != 0 || mass.Centroid[0].Lower.Cmp(r(49, 76)) != 0 || mass.Centroid[0].Upper.Cmp(r(49, 76)) != 0 {
		t.Fatal("exact correlated first moment", mass, err)
	}
	// Swapping the original roofs reverses the sign of the homogeneous
	// residual while preserving its zero set and strict separation.
	cache := make(roofOrderCache)
	p, err := cache.order([2]*mixedRoofBox{b, a}, cell.xy, &w)
	if err != nil || p.Lower.Sign() <= 0 {
		t.Fatal("swapping the original charts must reverse their exact order", p, err)
	}
}

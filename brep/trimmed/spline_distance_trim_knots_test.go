package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestSplineDistanceAlgebraicKnotWitness(t *testing.T) {
	ctx := context.Background()
	// Genuine C0 support crease z=|u-1/2|. Original UV curve (t²,t)
	// crosses its knot at irrational sqrt(1/2).
	var poles []geom.Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			z := u - .5
			if z < 0 {
				z = -z
			}
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := geom.NewBezier([]geom.Vec3{{}, {Y: .5}, {X: 1, Y: 1}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	exact, e := geom.NewExactSplineCurve(curve, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := p.BuildSurfaceCurveCompositionCells(ctx, exact, geom.SurfaceCurveAgreementOptions{})
	if e != nil || len(cells.Cells) != 2 {
		t.Fatal(cells, e)
	}
	if cells.Events[1].Lower.Cmp(cells.Events[1].Upper) == 0 {
		t.Fatal("irrational event rounded")
	}
	use := Use{Curve: SplineUV{Curve: curve, Range: UVInterval{Max: 1}, Reversed: true}}
	for _, query := range []geom.Vec3{{X: .1, Y: .2}, {X: .9, Y: .9}} {
		got, n, hit, e := composedKnotTrimWitness(ctx, p, use, query, 8, SplineBoundaryDistanceOptions{})
		if e != nil || !hit || n != 2 {
			t.Fatal(got, n, hit, e)
		}
		u := new(big.Rat).Mul(got.Parameter, got.Parameter)
		z := new(big.Rat).Sub(u, big.NewRat(1, 2))
		z.Abs(z)
		for i, r := range []*big.Rat{u, got.Parameter, z} {
			x, _ := r.Float64()
			lo := []float64{got.Point.Min.X, got.Point.Min.Y, got.Point.Min.Z}[i]
			hi := []float64{got.Point.Max.X, got.Point.Max.Y, got.Point.Max.Z}[i]
			if x < lo || x > hi {
				t.Fatal("original parameter map", got)
			}
		}
	}
	// Insert an original coedge knot; its two exact spans are normalized
	// independently but the returned parameter remains the original t.
	refined, e := curve.InsertKnot(.25, 1)
	if e != nil {
		t.Fatal(e)
	}
	use.Curve = SplineUV{Curve: refined, Range: UVInterval{Min: .125, Max: .875}, Reversed: true}
	got, n, hit, e := composedKnotTrimWitness(ctx, p, use, geom.Vec3{X: .7, Y: .8}, 8, SplineBoundaryDistanceOptions{})
	if e != nil || !hit || n != 3 || got.Parameter.Cmp(big.NewRat(1, 8)) < 0 || got.Parameter.Cmp(big.NewRat(7, 8)) > 0 {
		t.Fatal(got, n, hit, e)
	}
	u := new(big.Rat).Mul(got.Parameter, got.Parameter)
	x, _ := u.Float64()
	if x < got.Point.Min.X || x > got.Point.Max.X {
		t.Fatal("multispan parameter changed")
	}
	got, _, hit, e = composedKnotTrimWitness(ctx, p, use, geom.Vec3{}, 8, SplineBoundaryDistanceOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || hit || got.Parameter != nil {
		t.Fatal("non-atomic arithmetic quota", got, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, hit, e = composedKnotTrimWitness(canceled, p, use, geom.Vec3{}, 8, SplineBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || hit {
		t.Fatal(e)
	}
}

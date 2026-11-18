package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationOccurrenceAreaAndInertia(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	matrix := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}
	pose, err := placement.NewRationalRigid(matrix, 4096)
	if err != nil {
		t.Fatal(err)
	}
	solid := stationMassFixture(t, true, -4)
	placementOptions := WrappedRationalStationOccurrenceOptions{MaxTransformBits: 65536}
	original, err := solid.PlaceWrappedRationalStation(ctx, pose, placementOptions)
	if err != nil {
		t.Fatal(err)
	}
	areaOptions := WrappedRationalStationAreaOptions{AreaTolerance: r(1, 10)}
	want, err := solid.BoundWrappedRationalStationArea(ctx, areaOptions)
	if err != nil {
		t.Fatal(err)
	}
	area, err := original.BoundBoundaryArea(ctx, areaOptions)
	if err != nil || !reflect.DeepEqual(want, area) {
		t.Fatal("proper rigid occurrence changed the original area bounds", err)
	}
	inertiaOptions := WrappedRationalStationInertiaOptions{InertiaTolerance: r(1, 15), MaxCoefficientBits: 65536}
	actual, err := original.BoundCentroidalInertia(ctx, inertiaOptions)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, local := independentStationInertia([4]float64{.5, .75, .75, .75}, [][2]float64{{0, 3}, {1, 1}, {3, 2}})
	rotation := [3][3]float64{{.6, 0, .8}, {0, 1, 0}, {-.8, 0, .6}}
	for i := range rotation {
		for j := range rotation {
			var world float64
			for k := range rotation {
				for l := range rotation[k] {
					world += rotation[i][k] * local[k][l] * rotation[j][l]
				}
			}
			entry := actual.World[i][j]
			lo, _ := entry.Lower.Float64()
			hi, _ := entry.Upper.Float64()
			if world < lo || world > hi || new(big.Rat).Sub(entry.Upper, entry.Lower).Cmp(inertiaOptions.InertiaTolerance) > 0 {
				t.Fatal("invalid world inertia enclosure", i, j, lo, world, hi)
			}
		}
	}
	for i, v := range []int64{9, -23, 31} {
		matrix[i][3] = r(v, 23)
	}
	translatedPose, err := placement.NewRationalRigid(matrix, 4096)
	if err != nil {
		t.Fatal(err)
	}
	translated, err := solid.PlaceWrappedRationalStation(ctx, translatedPose, placementOptions)
	if err != nil {
		t.Fatal(err)
	}
	again, err := translated.BoundCentroidalInertia(ctx, inertiaOptions)
	if err != nil || !reflect.DeepEqual(again, actual) {
		t.Fatal("translation changed centroidal inertia", err)
	}
	actual.World[0][1].Lower.SetInt64(-100)
	if actual.World[1][0].Lower.Cmp(r(-100, 1)) == 0 || actual.Original.CentroidalInertia[0][1].Lower.Cmp(r(-100, 1)) == 0 {
		t.Fatal("world interval aliases its transpose or source")
	}
	limited, err := solid.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	if no, err := limited.BoundCentroidalInertia(ctx, inertiaOptions); !errors.Is(err, geom.ErrConvergence) || no.World[0][0].Lower != nil {
		t.Fatal("limited transform returned a partial tensor", err)
	}
}

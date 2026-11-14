package trimmed

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationOccurrenceMass(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	contains := func(interval WrappedStationMassInterval, want float64) {
		t.Helper()
		lo, _ := interval.Lower.Float64()
		hi, _ := interval.Upper.Float64()
		if want < lo-1e-10 || want > hi+1e-10 {
			t.Fatal("independent world integral", lo, want, hi)
		}
	}
	for _, asym := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact centroid", true: "asymmetric negative moments"}[asym], func(t *testing.T) {
			shift := 0.
			weights := [4]float64{.75, .75, .75, .75}
			if asym {
				shift = -4
				weights[0] = .5
			}
			body := stationMassFixture(t, asym, shift) // independently includes native reload
			faces := body.Faces()
			slices.Reverse(faces)
			body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 65536})
			if e != nil {
				t.Fatal(e)
			}
			o := WrappedRationalStationMassOptions{VolumeTolerance: r(1, 25), MaxCoefficientBits: 65536}
			got, e := q.BoundMassProperties(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			native, e := body.BoundWrappedRationalStationMass(ctx, o)
			if e != nil || !reflect.DeepEqual(native, got.Original) {
				t.Fatal("unchanged single integration", e)
			}
			if got.Volume.Lower.Cmp(native.Volume.Lower) != 0 || got.Volume.Upper.Cmp(native.Volume.Upper) != 0 || got.TransformOperations < 1 || got.TransformOperations > 1024 {
				t.Fatal("volume/transform count", got)
			}
			proof, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
			if e != nil || !reflect.DeepEqual(got.Original.Source.History, proof.History) {
				t.Fatal("original opaque owner identity", e)
			}
			area, mx, my := independentStationProfileMoments(weights, 4096)
			a2, x2, y2 := independentStationProfileMoments(weights, 2048)
			if math.Abs(area-a2)+math.Abs(mx-x2)+math.Abs(my-y2) > 1e-10 {
				t.Fatal("oracle convergence")
			}
			volume := 9 * area
			local := [3]float64{.25 + 35*mx/(18*area), -.5 + 35*my/(18*area), shift + 71./54}
			world := [3]float64{3./5*local[0] + 4./5*local[2] + 1./3, local[1] - 2./7, -4./5*local[0] + 3./5*local[2] + 5./11}
			contains(got.Volume, volume)
			for i := range world {
				contains(got.Centroid[i], world[i])
				contains(got.FirstMoments[i], volume*world[i])
			}
			if !asym {
				for i, want := range []*big.Rat{r(829, 540), r(-11, 14), r(1033, 990)} {
					if got.Centroid[i].Lower.Cmp(want) != 0 || got.Centroid[i].Upper.Cmp(want) != 0 {
						t.Fatal("exact rational world centroid", i, got.Centroid[i], want)
					}
				}
			}
			if asym {
				for i := range world {
					if got.FirstMoments[i].Upper.Sign() >= 0 {
						t.Fatal("signed moments lost", i, got.FirstMoments[i])
					}
				}
				if got.Centroid[0].Lower.Cmp(got.Centroid[0].Upper) == 0 {
					t.Fatal("unproved asymmetric exact centroid")
				}
			}
			// A tighter original partition remains a tighter affine enclosure despite
			// conservative loss of interval correlation.
			o.VolumeTolerance = r(1, 100)
			fine, e := q.BoundMassProperties(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			for i := range fine.FirstMoments {
				for j, pair := range [][2]WrappedStationMassInterval{{got.FirstMoments[i], fine.FirstMoments[i]}, {got.Centroid[i], fine.Centroid[i]}} {
					if pair[1].Lower.Cmp(pair[0].Lower) < 0 || pair[1].Upper.Cmp(pair[0].Upper) > 0 {
						t.Fatal("nonmonotone posed enclosure", i, j)
					}
				}
			}
			saved := new(big.Rat).Set(got.Volume.Lower)
			got.Original.Volume.Lower.SetInt64(-99)
			if got.Volume.Lower.Cmp(saved) != 0 {
				t.Fatal("volume alias")
			}
			saved.Set(got.FirstMoments[0].Lower)
			got.Original.FirstMoments[0].Lower.SetInt64(-99)
			if got.FirstMoments[0].Lower.Cmp(saved) != 0 {
				t.Fatal("first moment alias")
			}
			saved.Set(got.Centroid[0].Lower)
			got.Original.Centroid[0].Lower.SetInt64(-99)
			if got.Centroid[0].Lower.Cmp(saved) != 0 {
				t.Fatal("centroid alias")
			}
			got.Original.Source.History.WallFaces[0] = -99
			again, e := q.Source().ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
			if e != nil || !reflect.DeepEqual(again.History, proof.History) {
				t.Fatal("mutable report changed opaque original ancestry", e)
			}
		})
	}
}

func TestRationalStationOccurrenceMassAtomic(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body := stationMassFixture(t, false, 0)
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, 64)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	o := WrappedRationalStationMassOptions{VolumeTolerance: r(1, 25)}
	source, e := body.BoundWrappedRationalStationMass(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if source.Volume.Lower.Denom().BitLen() <= 64 {
		t.Fatal("fixture lacks interval transform overflow")
	}
	got, e := q.BoundMassProperties(ctx, o)
	if e == nil || got.Volume.Lower != nil || got.Original.Volume.Lower != nil {
		t.Fatal("non-atomic exact interval transform", got, e)
	}
	o.MaxOperations = 1
	if got, e := q.BoundMassProperties(ctx, o); e == nil || got.Volume.Lower != nil || got.Original.Volume.Lower != nil {
		t.Fatal("integration quota", got, e)
	}
	if _, e := (WrappedRationalStationOccurrence{}).BoundMassProperties(ctx, o); e == nil {
		t.Fatal("zero occurrence")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := q.BoundMassProperties(cancelled, o); e == nil {
		t.Fatal("cancel")
	}
}

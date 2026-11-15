package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func TestIntegrateAlignedSplineGraphPolynomialVolumes(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	for axis := 0; axis < 2; axis++ {
		a, _, _ := polynomialPairGraphFixture(t, axis, delta)
		b := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8)
		for _, tc := range []struct {
			reverse bool
			regions []int
			volumes []*big.Rat
		}{
			// Antidifferentiate the original height polynomial in the
			// dependent coordinate, then integrate each physical slab.
			{false, []int{0, 4}, []*big.Rat{big.NewRat(28550399214829, 2063645886382080), big.NewRat(590037023506439, 6190937659146240)}},
			{true, []int{1, 2, 3}, []*big.Rat{big.NewRat(4909741, 2063645886382080), big.NewRat(1, 31488737280), big.NewRat(160842989, 2063645886382080)}},
		} {
			report, e := IntegrateAlignedSplineGraphPolynomialVolumes(ctx, a, b, axis, tc.reverse, GraphPointContactOptions{})
			if e != nil || !report.Alignment.Arrangement.Proven || report.Reverse != tc.reverse ||
				!slices.Equal(report.Regions, tc.regions) || len(report.Volumes) != len(tc.volumes) {
				t.Fatal("complete aligned volume proof", axis, tc.reverse, report.Regions, e)
			}
			total := new(big.Rat)
			for i, want := range tc.volumes {
				if report.Volumes[i].Cmp(want) != 0 {
					t.Fatal("independent physical region volume", axis, tc.reverse, i, report.Volumes[i], want)
				}
				total.Add(total, want)
			}
			if report.Total.Cmp(total) != 0 {
				t.Fatal("exact aligned volume total", report.Total, total)
			}
			if axis == 0 {
				for _, work := range []int{report.ExactOperations, report.ExactOperations - 1} {
					r, e := IntegrateAlignedSplineGraphPolynomialVolumes(ctx, a, b, axis, tc.reverse, GraphPointContactOptions{MaxOperations: work})
					if work == report.ExactOperations {
						if e != nil || !reflect.DeepEqual(r, report) {
							t.Fatal("complete cumulative work is sufficient", e)
						}
					} else if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(r, AlignedSplineGraphPolynomialVolumes{ExactOperations: r.ExactOperations}) {
						t.Fatal("one-short budget publishes no partial proof", e)
					}
				}
			}
		}
		if r, e := IntegrateAlignedSplineGraphPolynomialVolumes(ctx, b, a, 0, false, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) ||
			!reflect.DeepEqual(r, AlignedSplineGraphPolynomialVolumes{ExactOperations: r.ExactOperations}) {
			t.Fatal("physical integration chart must be identity XY", axis, e)
		}
	}
}

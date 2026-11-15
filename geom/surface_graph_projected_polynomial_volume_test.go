package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func TestIntegrateProjectedSplineGraphPolynomialVolumes(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	reference := projectedPolynomialReference(t, 1, -7)
	for axis := 0; axis < 2; axis++ {
		physical, _, _ := polynomialPairGraphFixture(t, axis, delta)
		a := independentProjectedPolynomialRoof(physical)
		b := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8)
		for order, roofs := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			report, e := IntegrateProjectedSplineGraphPolynomialVolumes(ctx, reference, roofs[0], roofs[1], axis, GraphPointContactOptions{})
			regions := [2][]int{{0, 4}, {1, 2, 3}}[order]
			// Independent antiderivatives of the original physical height
			// difference evaluated at the original graph and domain limits.
			want := [2][]*big.Rat{
				{big.NewRat(28550399214829, 2063645886382080), big.NewRat(590037023506439, 6190937659146240)},
				{big.NewRat(4909741, 2063645886382080), big.NewRat(1, 31488737280), big.NewRat(160842989, 2063645886382080)},
			}[order]
			if e != nil || !report.Projection.Arrangement.Proven || len(report.Projection.Arrangement.CertifiedCells) != 6 ||
				report.ExactOperations != report.Projection.ExactOperations || !slices.Equal(report.Regions, regions) || len(report.Volumes) != len(want) {
				t.Fatal("complete projected physical integration", axis, order, report.Regions, e)
			}
			total := new(big.Rat)
			for i, v := range want {
				if report.Volumes[i].Cmp(v) != 0 || report.Volumes[i].Sign() <= 0 {
					t.Fatal("positive independent rational volume per component", axis, order, i, report.Volumes[i], v)
				}
				total.Add(total, v)
			}
			if report.Total.Cmp(total) != 0 {
				t.Fatal("exact sum of independent projected layer integrals", axis, order, report.Total, total)
			}
			if axis != 0 {
				continue
			}
			for _, work := range []int{report.ExactOperations, report.ExactOperations - 1} {
				got, e := IntegrateProjectedSplineGraphPolynomialVolumes(ctx, reference, roofs[0], roofs[1], axis, GraphPointContactOptions{MaxOperations: work})
				if work == report.ExactOperations {
					if e != nil || !reflect.DeepEqual(got, report) {
						t.Fatal("exact cumulative proof budget", order, e)
					}
				} else if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(got, ProjectedSplineGraphPolynomialVolumes{ExactOperations: got.ExactOperations}) {
					t.Fatal("one-short budget cannot publish partial volume or owner ledgers", order, e)
				}
			}
		}
	}
}

func TestIntegrateProjectedSplineGraphPolynomialVolumesRefusals(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	physical, _, _ := polynomialPairGraphFixture(t, 0, delta)
	a := independentProjectedPolynomialRoof(physical)
	b := independentPolynomialArrangementRoof(t, 0, 1, 25.0/8)
	reference := projectedPolynomialReference(t, 1, 4)
	for _, tc := range []struct {
		ref, roof *SurfacePatch
		err       error
	}{
		{independentProjectedPolynomialRoof(reference), b, ErrConvergence},
		{reference, independentPolynomialArrangementRoof(t, 0, 1, 3), ErrUnrepresentable},
		{reference, independentPolynomialArrangementRoof(t, 0, .75, 25.0/8), ErrDomain},
	} {
		r, e := IntegrateProjectedSplineGraphPolynomialVolumes(ctx, tc.ref, a, tc.roof, 0, GraphPointContactOptions{})
		if !errors.Is(e, tc.err) || !reflect.DeepEqual(r, ProjectedSplineGraphPolynomialVolumes{ExactOperations: r.ExactOperations}) {
			t.Fatal("physical identity reference and exact original owner alignments", tc.err, e)
		}
	}
}

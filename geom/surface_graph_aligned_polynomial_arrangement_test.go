package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func independentPolynomialArrangementRoof(t *testing.T, axis int, width, freeKnot float64) *SurfacePatch {
	t.Helper()
	// This bilinear roof is authored independently in (s,h), with the
	// physical longitudinal coordinate width*(5-s)/3 and dependent
	// coordinate (h+1)/2. Its height is 3+XY/64.
	var poles []Vec3
	for _, free := range []float64{width, 0} {
		for _, dependent := range []float64{0, 1} {
			x, y := free, dependent
			if axis == 1 {
				x, y = y, x
			}
			poles = append(poles, Vec3{X: x, Y: y, Z: 3 + x*y/64})
		}
	}
	s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{2, 2, 5, 5}, []float64{-1, -1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(freeKnot, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(-3.0/8, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestAlignedSplineGraphPolynomialArrangement(t *testing.T) {
	ctx := context.Background()
	equalRat := func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	for axis := 0; axis < 2; axis++ {
		a, _, powers := polynomialPairGraphFixture(t, axis, delta)
		b := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8)
		if r, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, axis, GraphPointContactOptions{}); !errors.Is(e, ErrDomain) || r.Proven {
			t.Fatal("same-chart proof requires the same UV domain", axis, e)
		}

		originalUV := func(owner int, xy [2]*big.Rat) [2]*big.Rat {
			if owner == 0 {
				return xy
			}
			s := new(big.Rat).Sub(big.NewRat(5, 1), new(big.Rat).Mul(big.NewRat(3, 1), xy[axis]))
			h := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), xy[1-axis]), big.NewRat(1, 1))
			return [2]*big.Rat{s, h}
		}
		physicalUV := func(owner int, uv [2]*big.Rat) [2]*big.Rat {
			if owner == 0 {
				return uv
			}
			free := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(5, 1), uv[0]), big.NewRat(3, 1))
			dependent := new(big.Rat).Quo(new(big.Rat).Add(uv[1], big.NewRat(1, 1)), big.NewRat(2, 1))
			return graphPolynomialUV(axis, free, dependent)
		}
		orientations := [2]int{1, 2*axis - 1}
		for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			firstAxis := [2]int{axis, 0}[order]
			r, e := DiscoverAlignedSplineGraphPolynomialArrangement(ctx, pair[0], pair[1], firstAxis, GraphPointContactOptions{})
			p := r.Arrangement
			orders, firstStations := []int{3, 2}, stations
			cofactor := rationalPolynomial{big.NewRat(-1, 1)}
			if order == 1 {
				orders = []int{2, 3}
				firstStations = []*big.Rat{big.NewRat(11, 4), big.NewRat(17, 4)}
				cofactor = rationalPolynomial{big.NewRat(-2, 243)}
			}
			if e != nil || !p.Proven || p.FreeAxis != firstAxis || len(p.CertifiedCells) != 6 ||
				!slices.Equal(p.ContactOrder, orders) || !reflect.DeepEqual(p.Cofactor, []*big.Rat(cofactor)) ||
				len(p.Nodes) != 2 || len(p.Regions) != 5 || len(p.Edges) != 6 ||
				r.ExactOperations <= p.ExactOperations {
				t.Fatal("complete aligned original arrangement", axis, order, p.ContactOrder, p.Cofactor, len(p.CertifiedCells), e)
			}
			checkPolynomialArrangementJets(t, p, firstStations, orders, cofactor)

			for k, in := range [][2]*big.Rat{{new(big.Rat), new(big.Rat)}, {big.NewRat(1, 1), new(big.Rat)}, {new(big.Rat), big.NewRat(1, 1)}} {
				want := originalUV(1-order, physicalUV(order, in))
				for j, row := range r.TargetMap {
					q := row[2]
					if k > 0 {
						q = new(big.Rat).Add(q, row[k-1])
					}
					if q.Cmp(want[j]) != 0 {
						t.Fatal("exact original target affine map", axis, order, k, j, q, want[j])
					}
				}
			}
			for i, owner := range r.Owners {
				source := pair[i]
				j := (order + i) % 2
				free := [2]int{axis, 0}[j]
				if owner.Domain != source.Domain() || owner.FreeAxis != free || owner.ProjectionOrientation != orientations[j] ||
					len(owner.Vertices) != len(p.Vertices) || len(owner.Edges) != len(p.Edges) || len(owner.RegionWitnesses) != len(p.Regions) {
					t.Fatal("original owner chart ledger", axis, order, i, owner.Domain, owner.FreeAxis)
				}
				for k, v := range p.Vertices {
					want := originalUV(j, [2]*big.Rat{v.Point[0], v.Point[1]})
					uv := owner.Vertices[k]
					original, e := source.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
					if e != nil || !slices.EqualFunc(uv[:], want[:], equalRat) || !slices.EqualFunc(original.Point[:], v.Point[:], equalRat) {
						t.Fatal("original roof vertex", axis, order, i, k, uv, want, e)
					}
				}
				for k, edge := range owner.Edges {
					common := p.Edges[k]
					if edge.FreeRange[0].Cmp(owner.Vertices[common.Start][free]) != 0 ||
						edge.FreeRange[1].Cmp(owner.Vertices[common.End][free]) != 0 ||
						edge.FreeRange[1].Cmp(edge.FreeRange[0]) != 1-2*i {
						t.Fatal("directed original owner station range", axis, order, i, k, edge.FreeRange)
					}
					degree := int64(len(edge.Controls) - 1)
					for step := int64(0); step <= degree; step++ {
						param := big.NewRat(step, degree)
						uv := testGraphBezierValue(edge.Controls, param)
						station := new(big.Rat).Sub(edge.FreeRange[1], edge.FreeRange[0])
						station.Mul(station, param).Add(station, edge.FreeRange[0])
						if uv[free].Cmp(station) != 0 {
							t.Fatal("affine parameter over the original owner interval", axis, order, i, k, step)
						}
						xy := physicalUV(j, uv)
						branch := common.Branch
						if order == 1 {
							branch = 1 - branch
						}
						want := testGraphPowerValue(powers[branch], xy[axis])
						q, e := source.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
						wantZ := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(new(big.Rat).Mul(xy[0], xy[1]), big.NewRat(64, 1)))
						if e != nil || xy[1-axis].Cmp(want) != 0 || !slices.EqualFunc(q.Point[:], []*big.Rat{xy[0], xy[1], wantZ}, equalRat) {
							t.Fatal("exact original curve restrictions", axis, order, i, k, step, e)
						}
					}
				}
			}
			areas := []int64{41997, 59, 32, 211, 80581}
			if order == 1 {
				areas = []int64{41997, 211, 32, 59, 80581}
			}
			for i, region := range p.Regions {
				uv0, uv1 := r.Owners[0].RegionWitnesses[i], r.Owners[1].RegionWitnesses[i]
				pa, e := pair[0].EvaluateRational(ctx, uv0[0], uv0[1], SurfacePatchEvaluationOptions{})
				pb, other := pair[1].EvaluateRational(ctx, uv1[0], uv1[1], SurfacePatchEvaluationOptions{})
				if e != nil || other != nil || pa.Point[0].Cmp(pb.Point[0]) != 0 || pa.Point[1].Cmp(pb.Point[1]) != 0 ||
					pa.Point[2].Cmp(pb.Point[2]) != region.HeightSide {
					t.Fatal("signed witness on the two original roofs", axis, order, i, e, other)
				}
				area := testPolynomialStripArea(firstAxis, p.Edges, region)
				if order == 0 && i == len(p.Regions)-1 {
					area.Add(area, big.NewRat(1, 1))
				}
				if order == 1 {
					area.Mul(area, big.NewRat(int64(orientations[1]), 6))
					// The horizontal perimeter of the two exterior regions
					// has owner h=-1 and +1 respectively, and width 3.
					if i == 0 || i == len(p.Regions)-1 {
						area.Add(area, big.NewRat(1, 2))
					}
				}
				if want := big.NewRat(areas[i], 122880); area.Cmp(want) != 0 {
					t.Fatal("independent analytical model area", axis, order, i, area, want)
				}
			}
		}
	}
}

func TestAlignedSplineGraphPolynomialArrangementRefusals(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	a, _, _ := polynomialPairGraphFixture(t, 0, delta)
	b := independentPolynomialArrangementRoof(t, 0, 1, 25.0/8)
	r, e := DiscoverAlignedSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	exact, e := DiscoverAlignedSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{MaxOperations: r.ExactOperations})
	if e != nil || !reflect.DeepEqual(r, exact) {
		t.Fatal("one cumulative exact-work budget", r.ExactOperations, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		b    *SurfacePatch
		o    GraphPointContactOptions
		err  error
	}{
		{"inexact remapped knot", ctx, independentPolynomialArrangementRoof(t, 0, 1, 3), GraphPointContactOptions{}, ErrUnrepresentable},
		{"unequal projected domains", ctx, independentPolynomialArrangementRoof(t, 0, .75, 25.0/8), GraphPointContactOptions{}, ErrDomain},
		{"singular map", ctx, independentPolynomialArrangementRoof(t, 0, 0, 25.0/8), GraphPointContactOptions{}, ErrDegenerate},
		{"unavailable union cells", ctx, b, GraphPointContactOptions{MaxCells: 5}, ErrConvergence},
		{"alignment budget", ctx, b, GraphPointContactOptions{MaxOperations: 1}, ErrConvergence},
		{"owner ledger budget", ctx, b, GraphPointContactOptions{MaxOperations: r.ExactOperations - 1}, ErrConvergence},
		{"canceled", canceled, b, GraphPointContactOptions{}, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e := DiscoverAlignedSplineGraphPolynomialArrangement(tc.ctx, a, tc.b, 0, tc.o)
			if !errors.Is(e, tc.err) || !reflect.DeepEqual(q, AlignedSplineGraphPolynomialArrangement{ExactOperations: q.ExactOperations}) {
				t.Fatal("atomic refusal", e, q.ExactOperations)
			}
		})
	}
}

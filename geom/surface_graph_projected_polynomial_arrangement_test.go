package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func independentProjectedPolynomialRoof(p *SurfacePatch) *SurfacePatch {
	// Author the exact net with U=4X-2 and V=10-2Y.
	q := *p
	q.knotsU = make([]float64, len(p.knotsU))
	q.knotsV = make([]float64, len(p.knotsV))
	for i, u := range p.knotsU {
		q.knotsU[i] = 4*u - 2
	}
	for i, v := range p.knotsV {
		q.knotsV[len(q.knotsV)-1-i] = 10 - 2*v
	}
	q.net = make([]exactHomogeneous, len(p.net))
	for u := 0; u < p.countU; u++ {
		for v := 0; v < p.countV; v++ {
			q.net[u*p.countV+v] = p.net[u*p.countV+p.countV-1-v]
		}
	}
	return &q
}

func projectedPolynomialReference(t *testing.T, width, height float64) *SurfacePatch {
	t.Helper()
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{Z: height}, {Y: 1, Z: height}, {X: width, Z: height}, {X: width, Y: 1, Z: height}},
		[]float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestProjectedSplineGraphPolynomialArrangement(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	orders := []int{3, 2}
	delta := arrangementGap(t, stations, orders, rationalPolynomial{big.NewRat(-1, 1)})
	reference := projectedPolynomialReference(t, 1, -7)
	equalRat := func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }
	for axis := 0; axis < 2; axis++ {
		physical, _, powers := polynomialPairGraphFixture(t, axis, delta)
		a := independentProjectedPolynomialRoof(physical)
		b := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8)
		physicalMap := func(owner int, uv [2]*big.Rat) [2]*big.Rat {
			scale := func(q *big.Rat, m, c int64) *big.Rat {
				return new(big.Rat).Add(new(big.Rat).Mul(q, big.NewRat(m, 1)), big.NewRat(c, 1))
			}
			if owner == 0 {
				return [2]*big.Rat{scale(uv[0], 4, -2), scale(uv[1], -2, 10)}
			}
			return [2]*big.Rat{scale(uv[axis], -3, 5), scale(uv[1-axis], 2, -1)}
		}
		for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			got, e := DiscoverProjectedSplineGraphPolynomialArrangement(ctx, reference, sources[0], sources[1], axis, GraphPointContactOptions{})
			proof := got.Arrangement
			if e != nil || !proof.Proven || !slices.Equal(proof.ContactOrder, orders) || len(proof.CertifiedCells) != 6 ||
				proof.FreeAxis != axis || proof.Domain != reference.Domain() || got.ExactOperations <= proof.ExactOperations {
				t.Fatal("complete polynomial arrangement in the independent reference chart", axis, order, e)
			}
			checkPolynomialArrangementJets(t, proof, stations, orders, rationalPolynomial{big.NewRat(-1, 1)})
			for source, owner := range got.Owners {
				physicalOwner := source ^ order
				wantAxis := [2]int{axis, 0}[physicalOwner]
				wantOrientation := [2]int{-1, 2*axis - 1}[physicalOwner]
				wantDirection := [2]int{1 - 2*axis, -1}[physicalOwner]
				if owner.Domain != sources[source].Domain() || owner.FreeAxis != wantAxis || owner.ProjectionOrientation != wantOrientation {
					t.Fatal("original independent owner chart", axis, order, source, owner.Domain, owner.FreeAxis)
				}
				for i, v := range proof.Vertices {
					fromProof := owner.Vertices[i]
					want := physicalMap(physicalOwner, v.Parameters)
					q, e := sources[source].EvaluateRational(ctx, fromProof[0], fromProof[1], SurfacePatchEvaluationOptions{})
					if e != nil || !slices.EqualFunc(fromProof[:], want[:], equalRat) || !slices.EqualFunc(q.Point[:], v.Point[:], equalRat) {
						t.Fatal("both exact original vertex evaluations", axis, order, source, i, e)
					}
				}
				for i, edge := range owner.Edges {
					if edge.FreeRange[1].Cmp(edge.FreeRange[0]) != wantDirection {
						t.Fatal("owner-oriented exact station interval", source, i, edge.FreeRange)
					}
					degree := int64(len(edge.Controls) - 1)
					for step := int64(0); step <= degree; step++ {
						s := big.NewRat(step, degree)
						xy := testGraphBezierValue(proof.Edges[i].Controls, s)
						want := physicalMap(physicalOwner, xy)
						actual := testGraphBezierValue(edge.Controls, s)
						if !slices.EqualFunc(actual[:], want[:], equalRat) || xy[1-axis].Cmp(testGraphPowerValue(powers[proof.Edges[i].Branch], xy[axis])) != 0 {
							t.Fatal("exact original UV curve and independent physical polynomial", source, i, step)
						}
					}
				}
				for i, region := range proof.Regions {
					want := physicalMap(physicalOwner, region.Witness)
					if actual := owner.RegionWitnesses[i]; !slices.EqualFunc(actual[:], want[:], equalRat) {
						t.Fatal("original signed sector witness", source, i, actual, want)
					}
				}
			}
		}
	}
}

func TestProjectedSplineGraphPolynomialArrangementRefusals(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	physical, _, _ := polynomialPairGraphFixture(t, 0, delta)
	a := independentProjectedPolynomialRoof(physical)
	b := independentPolynomialArrangementRoof(t, 0, 1, 25.0/8)
	reference := projectedPolynomialReference(t, 1, 4)
	r, e := DiscoverProjectedSplineGraphPolynomialArrangement(ctx, reference, a, b, 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	full, e := DiscoverProjectedSplineGraphPolynomialArrangement(ctx, reference, a, b, 0, GraphPointContactOptions{MaxOperations: r.ExactOperations})
	if e != nil || !reflect.DeepEqual(r, full) {
		t.Fatal("complete cumulative exact work budget", r.ExactOperations, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		ctx       context.Context
		ref, roof *SurfacePatch
		options   GraphPointContactOptions
		err       error
	}{
		{ctx, reference, b, GraphPointContactOptions{MaxOperations: r.ExactOperations - 1}, ErrConvergence},
		{ctx, reference, b, GraphPointContactOptions{MaxCells: 5}, ErrConvergence},
		{ctx, reference, independentPolynomialArrangementRoof(t, 0, 1, 3), GraphPointContactOptions{}, ErrUnrepresentable},
		{ctx, projectedPolynomialReference(t, .75, -7), b, GraphPointContactOptions{}, ErrDomain},
		{cancelled, reference, b, GraphPointContactOptions{}, context.Canceled},
	} {
		got, e := DiscoverProjectedSplineGraphPolynomialArrangement(tc.ctx, tc.ref, a, tc.roof, 0, tc.options)
		if !errors.Is(e, tc.err) || !reflect.DeepEqual(got, ProjectedSplineGraphPolynomialArrangement{ExactOperations: got.ExactOperations}) {
			t.Fatal("a failed independent map or work proof publishes only operations", tc.err, e)
		}
	}
}

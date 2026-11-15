package geom

import (
	"context"
	"math/big"
	"testing"
)

func rationalCircleDiscoveryFixture(t *testing.T, radiusSquared, yScale float64) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var out [2]*SurfacePatch
	for owner := range out {
		poles := []Vec3{}
		term := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				z := 3 + u*v/64
				if owner == 0 {
					z += radiusSquared - term[i] - yScale*term[j]
				}
				poles = append(poles, Vec3{u, v, z})
			}
		}
		s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		out[owner], e = NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1]
}
func TestDiscoverRationalCircleArcs(t *testing.T) {
	ctx := context.Background()
	a, b := rationalCircleDiscoveryFixture(t, 5.0/64, 1)
	var e error
	a, e = a.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.InsertVKnot(.75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphRationalCircleArcs(ctx, pair[0], pair[1], RationalCircleArcDiscoveryOptions{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		if r.Certificate.InsideSide == r.Certificate.OutsideSide {
			t.Fatal("not transverse")
		}
		for _, curve := range r.Arcs {
			for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)} {
				q, e := curve.patch.EvaluateRational(ctx, s, new(big.Rat), SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				x, y := new(big.Rat).Sub(q.Point[0], big.NewRat(1, 2)), new(big.Rat).Sub(q.Point[1], big.NewRat(1, 2))
				if new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(big.NewRat(5, 64)) != 0 {
					t.Fatal("original circle equation")
				}
				qa, e := a.EvaluateRational(ctx, q.Point[0], q.Point[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				qb, e := b.EvaluateRational(ctx, q.Point[0], q.Point[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for k := range qa.Point {
					if qa.Point[k].Cmp(qb.Point[k]) != 0 {
						t.Fatal("original independent support")
					}
				}
			}
		}
		o := RationalCircleArcDiscoveryOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: r.ExactOperations}}
		if q, e := DiscoverSplineGraphRationalCircleArcs(ctx, pair[0], pair[1], o); e != nil || !q.Complete {
			t.Fatal("exact quota", e)
		}
		o.MaxOperations--
		if q, e := DiscoverSplineGraphRationalCircleArcs(ctx, pair[0], pair[1], o); e == nil || q.Complete || q.Arcs[0] != nil {
			t.Fatal("atomic quota", e)
		}
	}
}
func TestDiscoverRationalCircleArcsRejects(t *testing.T) {
	ctx := context.Background()
	a, b := rationalCircleDiscoveryFixture(t, 5.0/64, 1)
	for _, o := range []RationalCircleArcDiscoveryOptions{{MaxSearchSteps: 1}, {MaxSearchSteps: 65537}, {GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}}} {
		if r, e := DiscoverSplineGraphRationalCircleArcs(ctx, a, b, o); e == nil || r.Complete || r.Arcs[0] != nil {
			t.Fatal("unbounded/partial discovery")
		}
	}
	for _, v := range [][2]float64{{3.0 / 64, 1}, {5.0 / 64, 2}, {1, 1}} {
		a, b := rationalCircleDiscoveryFixture(t, v[0], v[1])
		if r, e := DiscoverSplineGraphRationalCircleArcs(ctx, a, b, RationalCircleArcDiscoveryOptions{}); e == nil || r.Complete || r.Arcs[0] != nil {
			t.Fatal("unsupported geometry accepted", v)
		}
	}
	refined, e := a.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	damaged := *refined
	damaged.net = append([]exactHomogeneous(nil), refined.net...)
	last := len(damaged.net) - 1
	damaged.net[last][2] = new(big.Rat).Add(damaged.net[last][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphRationalCircleArcs(ctx, &damaged, b, RationalCircleArcDiscoveryOptions{}); e == nil || r.Complete || r.Arcs[0] != nil {
		t.Fatal("first-cell-only conic accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphRationalCircleArcs(canceled, a, b, RationalCircleArcDiscoveryOptions{}); e == nil || r.Complete {
		t.Fatal("cancellation")
	}
}

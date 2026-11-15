package geom

import (
	"context"
	"math/big"
	"testing"
)

func rationalEllipseDiscoveryFixture(t *testing.T) (*SurfacePatch, *SurfacePatch) {
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
					z += 13.0/256 - 2*term[i] - (u-.5)*(v-.5) - 3*term[j]
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
func TestDiscoverRationalEllipseArcs(t *testing.T) {
	ctx := context.Background()
	a, b := rationalEllipseDiscoveryFixture(t)
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
		r, e := DiscoverSplineGraphRationalEllipseArcs(ctx, pair[0], pair[1], RationalEllipseArcDiscoveryOptions{})
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
				value := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(x, x))
				value.Add(value, new(big.Rat).Mul(x, y))
				value.Add(value, new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(y, y)))
				if value.Cmp(big.NewRat(13, 256)) != 0 {
					t.Fatal("independent original quadratic equation")
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
		o := RationalEllipseArcDiscoveryOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: r.ExactOperations}}
		if q, e := DiscoverSplineGraphRationalEllipseArcs(ctx, pair[0], pair[1], o); e != nil || !q.Complete {
			t.Fatal("exact quota", e)
		}
		o.MaxOperations--
		if q, e := DiscoverSplineGraphRationalEllipseArcs(ctx, pair[0], pair[1], o); e == nil || q.Complete || q.Arcs[0] != nil {
			t.Fatal("atomic quota", e)
		}
	}
}
func TestDiscoverRationalEllipseDirectionAndAtomicLimits(t *testing.T) {
	ctx := context.Background()
	a, b := rationalEllipseDiscoveryFixture(t)
	for _, o := range []RationalEllipseArcDiscoveryOptions{{MaxDirectionCoordinate: 1}, {MaxDirectionCoordinate: 65}, {GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}}} {
		r, e := DiscoverSplineGraphRationalEllipseArcs(ctx, a, b, o)
		if e == nil || r.Complete || r.Arcs[0] != nil {
			t.Fatal("atomic rejection", r, e)
		}
	}
	r, e := DiscoverSplineGraphRationalEllipseArcs(ctx, a, b, RationalEllipseArcDiscoveryOptions{MaxDirectionCoordinate: 2})
	if e != nil || !r.Complete {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphRationalEllipseArcs(cancelled, a, b, RationalEllipseArcDiscoveryOptions{}); e == nil || r.Complete {
		t.Fatal("cancel", e)
	}
	refined, e := a.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	damaged := *refined
	damaged.net = append([]exactHomogeneous(nil), refined.net...)
	last := len(damaged.net) - 1
	damaged.net[last][2] = new(big.Rat).Add(damaged.net[last][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphRationalEllipseArcs(ctx, &damaged, b, RationalEllipseArcDiscoveryOptions{}); e == nil || r.Complete {
		t.Fatal("unproved later cell", e)
	}
}

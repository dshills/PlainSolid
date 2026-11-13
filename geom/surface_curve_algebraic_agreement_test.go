package geom

import (
	"context"
	"math/big"
	"testing"
)

func algebraicSeamFixture(t *testing.T) (*SurfacePatch, *ExactSplineCurve, *SurfacePatch, *ExactSplineCurve) {
	a, _ := graphContactFixture(t, false, false)
	b, _ := graphContactFixture(t, false, false)
	// B swaps chart parameters but preserves the same model mapping after the
	// UV path is swapped. Different knot axes produce distinct irrational cuts.
	old := append([]exactHomogeneous(nil), b.net...)
	for i := 0; i < b.countU; i++ {
		for j := 0; j < b.countV; j++ {
			b.net[i*b.countV+j] = old[j*b.countV+i]
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.375, 1, SurfacePatchOptions{})
	controls := [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}
	u, e := NewExactHomogeneousBezier(context.Background(), controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range controls {
		controls[i][0], controls[i][1] = controls[i][1], controls[i][0]
	}
	v, e := NewExactHomogeneousBezier(context.Background(), controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return a, u, b, v
}
func TestAlgebraicComposedSeamWholeIdentity(t *testing.T) {
	a, u, b, v := algebraicSeamFixture(t)
	for _, reverse := range []bool{false, true} {
		if reverse {
			u, _ = u.RationalSegment(context.Background(), big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
			v, _ = v.RationalSegment(context.Background(), big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
		}
		r, e := CompareSurfaceCurveCompositions(context.Background(), a, u, b, v, SurfaceCurveAgreementOptions{})
		if e != nil || !r.Complete || !r.Equal || len(r.Events) != 4 || len(r.Intervals) != 3 {
			t.Fatal(r, e)
		}
		for i, interval := range r.Intervals {
			if !interval.Equal || interval.Witness.Cmp(r.Events[i].Upper) <= 0 || interval.Witness.Cmp(r.Events[i+1].Lower) >= 0 {
				t.Fatal("ordered exact coverage", r)
			}
		}
	}
}
func TestAlgebraicComposedSeamNonidentityAndLimits(t *testing.T) {
	a, u, b, v := algebraicSeamFixture(t)
	r, e := CompareSurfaceCurveCompositions(context.Background(), a, u, b, v, SurfaceCurveAgreementOptions{MaxEvents: 3})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for i := range b.net {
		b.net[i][2].Add(b.net[i][2], new(big.Rat).Mul(tiny, b.net[i][3]))
	}
	r, e = CompareSurfaceCurveCompositions(context.Background(), a, u, b, v, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || r.Equal {
		t.Fatal(r, e)
	}
	r, e = CompareSurfaceCurveCompositions(context.Background(), a, u, b, v, SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CompareSurfaceCurveCompositions(ctx, a, u, b, v, SurfaceCurveAgreementOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}

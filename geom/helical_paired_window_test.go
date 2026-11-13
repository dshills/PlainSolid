package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalPairedWindow(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .25}, {2, .5}}, 2, hand, -.25, 1.25)
		s, e := NewSeparatedHelicalStarts(p, 3)
		if e != nil {
			t.Fatal(e)
		}
		phase := [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}
		q, e := s.CertifyPairedWindow(context.Background(), big.NewRat(5, 2), 2, phase, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || !q.Present || q.WholeWindow || q.Phase[1].Cmp(big.NewRat(1, 4)) != 0 || q.Copies[1].Start != 0 || q.Copies[1].Turn != 1 {
			t.Fatal(q, e)
		}
		// At radius 5/2 the triangular profile slice is [1/8,3/8].
		// Mid-phase 1/8 advances both by 1/4; offsets are 4/3 and 2.
		if q.WitnessCutters[0][0].Cmp(big.NewRat(41, 24)) != 0 || q.WitnessCutters[0][1].Cmp(big.NewRat(47, 24)) != 0 || q.WitnessCutters[1][0].Cmp(big.NewRat(19, 8)) != 0 || q.WitnessGap[0].Cmp(big.NewRat(47, 24)) != 0 {
			t.Fatal(q.WitnessCutters, q.WitnessGap)
		}
		q, e = s.CertifyPairedWindow(context.Background(), big.NewRat(5, 2), -1, phase, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Present || q.Phase[0].Cmp(big.NewRat(3, 4)) != 0 || q.Copies[0].Start != 2 || q.Copies[0].Turn != -1 {
			t.Fatal(q, e)
		}
		q, e = s.CertifyPairedWindow(context.Background(), big.NewRat(5, 2), 5, phase, SurfacePatchEvaluationOptions{})
		if e != nil || q.Present || !q.Complete {
			t.Fatal(q, e)
		}
		q, e = s.CertifyPairedWindow(context.Background(), big.NewRat(5, 2), 0, phase, SurfacePatchEvaluationOptions{})
		if e != nil || !q.WholeWindow {
			t.Fatal(q, e)
		}
		if _, e = s.CertifyPairedWindow(context.Background(), big.NewRat(5, 2), 0, phase, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e = s.CertifyPairedWindow(ctx, big.NewRat(5, 2), 0, phase, SurfacePatchEvaluationOptions{}); e != context.Canceled {
			t.Fatal(e)
		}
	}
}

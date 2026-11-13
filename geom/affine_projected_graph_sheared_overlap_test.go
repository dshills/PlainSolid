package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestShearedProjectedGraphOverlap(t *testing.T) {
	ctx := context.Background()
	makePatch := func(shear float64, order [3]int) *SurfacePatch {
		poles := make([]Vec3, 9)
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				u2, v2 := float64(i*(i-1))/2, float64(j*(j-1))/2
				coordinates := [3]float64{u + shear*v, v, u2 + 2*shear*u*v + (1+shear*shear)*v2}
				poles[3*i+j] = Vec3{coordinates[order[0]], coordinates[order[1]], coordinates[order[2]]}
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		s, e := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	for _, test := range []struct {
		order [3]int
		axes  [2]int
		sign  int
	}{{[3]int{0, 2, 1}, [2]int{0, 2}, 1}, {[3]int{0, 2, 1}, [2]int{2, 0}, -1}, {[3]int{2, 0, 1}, [2]int{1, 2}, 1}} {
		a, b := makePatch(0, test.order), makePatch(1.5, test.order)
		r, e := CertifyShearedProjectedGraphOverlap(ctx, a, b, test.axes, SurfacePatchOptions{})
		if e != nil || !r.Coincident || len(r.A) != 3 || len(r.ModelCorners) != 3 || r.ProjectionOrientation != [2]int{test.sign, test.sign} {
			t.Fatal(r, e)
		}
		found := false
		for i, uv := range r.A {
			if uv[1].Cmp(big.NewRat(2, 3)) == 0 {
				found = true
			}
			ja, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			jb, e := b.EvaluateRational(ctx, r.B[i][0], r.B[i][1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if r.ModelCorners[i][axis].Cmp(ja.Point[axis]) != 0 || ja.Point[axis].Cmp(jb.Point[axis]) != 0 {
					t.Fatal("lost original XYZ")
				}
			}
		}
		if !found {
			t.Fatal("missing nonbinary polygon corner")
		}
		if bad, e := CertifyShearedProjectedGraphOverlap(ctx, a, b, test.axes, SurfacePatchOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) || bad.Coincident {
			t.Fatal(bad, e)
		}
		b.net[0][r.ResidualAxis] = new(big.Rat).Add(b.net[0][r.ResidualAxis], big.NewRat(1, 10000))
		if bad, e := CertifyShearedProjectedGraphOverlap(ctx, a, b, test.axes, SurfacePatchOptions{}); e == nil || bad.Coincident {
			t.Fatal("mixed contact accepted", bad, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := CertifyShearedProjectedGraphOverlap(cancelled, a, b, test.axes, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}

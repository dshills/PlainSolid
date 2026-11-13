package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestShearedAffineGraphOverlap(t *testing.T) {
	ctx := context.Background()
	makePatch := func(shear float64) *SurfacePatch {
		p := make([]Vec3, 9)
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				u2, v2 := float64(i*(i-1))/2, float64(j*(j-1))/2
				p[3*i+j] = Vec3{u + shear*v, v, u2 + 2*shear*u*v + (1+shear*shear)*v2}
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		s, e := NewBSplineSurface(2, 2, 3, 3, p, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	a := makePatch(0)
	for _, shear := range []float64{.5, 1.5} {
		b := makePatch(shear)
		r, e := CertifyShearedAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{})
		if e != nil || !r.Coincident || r.Orientation != 1 {
			t.Fatal(r, e)
		}
		want := 4
		if shear == 1.5 {
			want = 3
		}
		if len(r.A) != want {
			t.Fatal(r.A)
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
				if ja.Point[axis].Cmp(jb.Point[axis]) != 0 {
					t.Fatal("corner mismatch")
				}
			}
		}
		if shear == 1.5 && !found {
			t.Fatal("missing rational cut")
		}
		reverse, e := CertifyShearedAffineXYGraphOverlap(ctx, b, a, SurfacePatchOptions{})
		if e != nil || !reverse.Coincident || len(reverse.A) != want {
			t.Fatal(reverse, e)
		}
		if _, e := CertifyAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{}); e == nil {
			t.Fatal("legacy strict map broadened")
		}
		if _, e := CertifyShearedAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}

		reflected := *b
		reflected.net = make([]exactHomogeneous, len(b.net))
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				reflected.net[3*i+j] = b.net[3*(2-i)+j]
			}
		}
		reflectedProof, e := CertifyShearedAffineXYGraphOverlap(ctx, a, &reflected, SurfacePatchOptions{})
		if e != nil || !reflectedProof.Coincident || reflectedProof.Orientation != -1 {
			t.Fatal(reflectedProof, e)
		}
		b.net[0][2] = new(big.Rat).Add(b.net[0][2], big.NewRat(1, 10000))
		if bad, e := CertifyShearedAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{}); e == nil || bad.Coincident {
			t.Fatal("perturbed", bad, e)
		}
	}
}

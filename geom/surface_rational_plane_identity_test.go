package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSurfaceRationalPlaneIdentity(t *testing.T) {
	poles := []Vec3{}
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{u, 19./64 + u/2 + v/4, v})
		}
	}
	s, e := NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 2, 1, 1, 2})
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Nonbinary origin lies in the same plane; normal is scaled by exact 1/3.
	origin := [3]*big.Rat{big.NewRat(2, 3), new(big.Rat).Add(big.NewRat(19, 64), big.NewRat(1, 3)), new(big.Rat)}
	normal := [3]*big.Rat{big.NewRat(1, 6), big.NewRat(-1, 3), big.NewRat(1, 12)}
	proof, e := CertifySurfaceRationalPlaneIdentity(context.Background(), p, origin, normal, SurfacePatchOptions{})
	if e != nil || !proof.Proven {
		t.Fatal(proof, e)
	}
	for _, limit := range []int{proof.ExactOperations, proof.ExactOperations - 1} {
		r, e := CertifySurfaceRationalPlaneIdentity(context.Background(), p, origin, normal, SurfacePatchOptions{MaxOperations: limit})
		if limit == proof.ExactOperations {
			if e != nil || !r.Proven {
				t.Fatal(r, e)
			}
		} else if e == nil || r.Proven {
			t.Fatal("budget published identity")
		}
	}
	normal[2] = big.NewRat(1, 11)
	if r, e := CertifySurfaceRationalPlaneIdentity(context.Background(), p, origin, normal, SurfacePatchOptions{}); e != nil || r.Proven {
		t.Fatal("wrong covector", r, e)
	}
	zero := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	if r, e := CertifySurfaceRationalPlaneIdentity(context.Background(), p, origin, zero, SurfacePatchOptions{}); e == nil || r.Proven {
		t.Fatal("zero normal", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := CertifySurfaceRationalPlaneIdentity(ctx, p, origin, normal, SurfacePatchOptions{}); e != context.Canceled || r.Proven {
		t.Fatal(r, e)
	}
}

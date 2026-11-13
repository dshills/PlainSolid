package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestDiscoverClippedParabolicIsoPlaneTubes(t *testing.T) {
	var poles []Vec3
	for _, u := range []float64{0, 1} {
		for j, z := range []float64{25.0/1024 - .25, 25.0/1024 + .25, 25.0/1024 - .25} {
			poles = append(poles, Vec3{u, float64(j) / 2, z})
		}
	}
	s, _ := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{.75, u, v} })
	r, e := DiscoverSurfaceIsoPlaneTubes(context.Background(), a, b, 0, .75, 11.0/32, 21.0/32, SurfaceArcTubeDiscoveryOptions{})
	if e != nil || !r.Complete || len(r.Tubes) < 2 {
		t.Fatal(r, e)
	}
	for _, parameter := range []float64{11.0 / 32, 21.0 / 32} {
		found := false
		hint := [4]*big.Rat{big.NewRat(3, 4), new(big.Rat).SetFloat64(parameter), new(big.Rat).SetFloat64(parameter), new(big.Rat)}
		for _, tube := range r.Tubes {
			for _, upper := range []bool{false, true} {
				cert, e := tube.Proof.CertifyEndpoint(context.Background(), upper, hint, TubeEndpointOptions{})
				if e == nil && cert.Proven {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("unproved terminal", parameter, r)
		}
	}
}

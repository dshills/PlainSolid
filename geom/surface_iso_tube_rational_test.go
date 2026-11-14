package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func rationalIsoTubeFixture(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var poles []Vec3
	for i, u := range []float64{-1, 0, 1} {
		top := []float64{.25, .125, .25}[i]
		poles = append(poles, Vec3{0, u, -.5}, Vec3{0, u, top})
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{-1, -1, -1, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{-1, 0, -1}, {-1, 0, 0}, {1, 0, -1}, {1, 0, 0}}, []float64{-1, -1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(p, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return a, b
}
func TestRationalIsoPlaneTargetFreeEndpoint(t *testing.T) {
	a, b := rationalIsoTubeFixture(t)
	ctx := context.Background()
	end := big.NewRat(8, 11)
	r, e := DiscoverSurfaceIsoPlaneRationalTubes(ctx, a, b, 0, 0, new(big.Rat), end, SurfaceArcTubeDiscoveryOptions{})
	if e != nil || !r.Complete || len(r.Tubes) < 2 {
		t.Fatal("rational clip", len(r.Tubes), e)
	}
	found := false
	for _, tube := range r.Tubes {
		if tube.ExactSourceSpan[0] == nil || tube.ExactSourceSpan[1] == nil {
			t.Fatal("lost exact span")
		}
		if tube.ExactSourceSpan[1].Cmp(end) == 0 {
			if tube.FreeAxis != 3 {
				t.Fatal("nonbinary source endpoint selected as free", tube.FreeAxis)
			}
			cert, e := tube.Proof.CertifyEndpoint(ctx, true, [4]*big.Rat{new(big.Rat), end, new(big.Rat), big.NewRat(1, 1)}, TubeEndpointOptions{})
			if e != nil || !cert.Proven || cert.Point[2].Sign() != 0 {
				t.Fatal("exact original endpoint", e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no exact upper terminal")
	}
	// Reverse the original plane V units; the same model endpoint is now its lower end.
	ps, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{-1, 0, 0}, {-1, 0, -1}, {1, 0, 0}, {1, 0, -1}}, []float64{-1, -1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	reversed, e := NewSurfacePatch(ps, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rr, e := DiscoverSurfaceIsoPlaneRationalTubes(ctx, a, reversed, 0, 0, new(big.Rat), end, SurfaceArcTubeDiscoveryOptions{})
	if e != nil || !rr.Complete {
		t.Fatal("reversed plane chart", e)
	}
	matched := false
	for _, tube := range rr.Tubes {
		if tube.ExactSourceSpan[1].Cmp(end) == 0 {
			cert, e := tube.Proof.CertifyEndpoint(ctx, false, [4]*big.Rat{new(big.Rat), end, new(big.Rat), new(big.Rat)}, TubeEndpointOptions{})
			if e != nil || !cert.Proven {
				t.Fatal("reversed exact terminal", e)
			}
			matched = true
		}
	}
	if !matched {
		t.Fatal("missing reversed terminal")
	}

	beyond := new(big.Rat).Add(end, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 60)))
	bad, e := DiscoverSurfaceIsoPlaneRationalTubes(ctx, a, b, 0, 0, new(big.Rat), beyond, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, ErrDomain) || bad.Complete || len(bad.Tubes) > 0 {
		t.Fatal("near chart exit", e)
	}
	bad, e = DiscoverSurfaceIsoPlaneRationalTubes(ctx, a, b, 0, 0, new(big.Rat), end, SurfaceArcTubeDiscoveryOptions{MaxOperations: 1})
	if e == nil || bad.Complete || len(bad.Tubes) > 0 {
		t.Fatal("atomic initial budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DiscoverSurfaceIsoPlaneRationalTubes(canceled, a, b, 0, 0, new(big.Rat), end, SurfaceArcTubeDiscoveryOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func independentEndpointTube(t *testing.T, right bool, delta *big.Rat) (VerifiedSurfaceTube, [4]*big.Rat) {
	t.Helper()
	makePatch := func(slope, offset float64) *SurfacePatch {
		s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{-1, 0, offset}, {-1, 1, slope + offset}, {1, 0, offset}, {1, 1, slope + offset}}, []float64{-1, -1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		return p
	}
	a, b := makePatch(0, 0), makePatch(3, -1)
	coordinate := big.NewRat(1, 3)
	if right {
		a, b = makePatch(6, -2), makePatch(0, 0)
		if delta != nil {
			for i := range a.net {
				a.net[i][2] = new(big.Rat).Add(a.net[i][2], new(big.Rat).Mul(delta, a.net[i][3]))
			}
			budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
			if e := a.computeBounds(budget); e != nil {
				t.Fatal(e)
			}
			coordinate = new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(2, 1), delta), big.NewRat(6, 1))
		}
	}
	if right {
		a, _ = a.Trim(ParameterRectangle{UMin: -.2, UMax: 0, VMin: .25, VMax: .4})
		b, _ = b.Trim(ParameterRectangle{UMin: -.3, UMax: .1, VMin: .25, VMax: .4})
	} else {
		a, _ = a.Trim(ParameterRectangle{UMin: 0, UMax: .2, VMin: .25, VMax: .4})
		b, _ = b.Trim(ParameterRectangle{UMin: -.1, UMax: .3, VMin: .25, VMax: .4})
	}
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return tube, [4]*big.Rat{big.NewRat(0, 1), new(big.Rat).Set(coordinate), big.NewRat(0, 1), new(big.Rat).Set(coordinate)}
}
func TestDifferentPairTubeEndpointExactEquality(t *testing.T) {
	left, lh := independentEndpointTube(t, false, nil)
	right, rh := independentEndpointTube(t, true, nil)
	r, e := CompareVerifiedTubeEndpoints(context.Background(), left, right, false, true, lh, rh, TubeEndpointOptions{})
	if e != nil || !r.Complete || !r.Equal || !r.Left.Proven || !r.Right.Proven || r.Left.Point[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	lh[1].SetInt64(8)
	r.Left.Point[1].SetInt64(9)
	fresh, e := right.CertifyEndpoint(context.Background(), true, rh, TubeEndpointOptions{})
	if e != nil || fresh.Point[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(fresh, e)
	}
}
func TestDifferentPairTubeEndpointNearbyRationalsStayDistinct(t *testing.T) {
	left, lh := independentEndpointTube(t, false, nil)
	delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	right, rh := independentEndpointTube(t, true, delta)
	r, e := CompareVerifiedTubeEndpoints(context.Background(), left, right, false, true, lh, rh, TubeEndpointOptions{})
	if e != nil || !r.Complete || r.Equal || !r.Left.Proven || !r.Right.Proven {
		t.Fatal(r, e)
	}
	x, _ := r.Left.Point[1].Float64()
	y, _ := r.Right.Point[1].Float64()
	if x != y {
		t.Fatal("fixture must round to the same float", x, y)
	}
}
func TestDifferentPairTubeEndpointRejectsWrongAndLimits(t *testing.T) {
	left, lh := independentEndpointTube(t, false, nil)
	right, rh := independentEndpointTube(t, true, nil)
	rh[1] = big.NewRat(333, 1000)
	r, e := CompareVerifiedTubeEndpoints(context.Background(), left, right, false, true, lh, rh, TubeEndpointOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.Right.Proven {
		t.Fatal(r, e)
	}
	right, rh = independentEndpointTube(t, true, nil)
	r, e = CompareVerifiedTubeEndpoints(context.Background(), left, right, false, true, lh, rh, TubeEndpointOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = left.CertifyEndpoint(ctx, false, lh, TubeEndpointOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var invalid VerifiedSurfaceTube
	_, e = invalid.CertifyEndpoint(context.Background(), false, lh, TubeEndpointOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

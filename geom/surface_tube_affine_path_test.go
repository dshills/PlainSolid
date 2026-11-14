package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestVerifiedTubeAffinePathCurvedModel(t *testing.T) {
	build := func(second bool) *SurfacePatch {
		poles := make([]Vec3, 6)
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				x, y, z := float64(i)/2, float64(j), 0.0
				if i == 2 {
					z = 1
				}
				if second {
					y = .5
					z += float64(j) - .25
				}
				poles[2*i+j] = Vec3{x, y, z}
			}
		}
		s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		return p
	}
	a, _ := build(false).Trim(ParameterRectangle{UMin: .49, UMax: .51, VMin: .49, VMax: .51})
	b, _ := build(true).Trim(ParameterRectangle{UMin: .48, UMax: .52, VMin: .1, VMax: .4})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	start, end := affineTubeLineEndpoints()

	proof, e := tube.CertifyAffineParameterPath(context.Background(), start, end, TubeAffineLineOptions{})
	if e != nil || !proof.Proven {
		t.Fatal(proof, e)
	}
	// Independent Bernstein identity for z=x² on the entire returned curve.
	patch := proof.Curve.patch
	net := make([]exactHomogeneous, patch.countU)
	for i := range net {
		net[i] = patch.net[i*patch.countV]
	}
	n := len(net) - 1
	span := new(big.Rat).Sub(end[0], start[0])
	for i, h := range net {
		linear := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(start[0], span))
		linear.Mul(linear, big.NewRat(int64(i), int64(n)))
		quadratic := new(big.Rat).Mul(span, span)
		quadratic.Mul(quadratic, big.NewRat(int64(i*(i-1)), int64(n*(n-1))))
		want := new(big.Rat).Add(new(big.Rat).Mul(start[0], start[0]), new(big.Rat).Add(linear, quadratic))
		if new(big.Rat).Quo(h[2], h[3]).Cmp(want) != 0 {
			t.Fatal("nonlinear model coefficient", i, h, want)
		}
	}
	for _, cap := range []int{proof.ExactOperations, proof.ExactOperations - 1} {
		got, e := tube.CertifyAffineParameterPath(context.Background(), start, end, TubeAffineLineOptions{MaxOperations: cap})
		if cap == proof.ExactOperations {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || got.Proven || got.Curve != nil {
			t.Fatal("atomic quota")
		}
	}
	wrong := end
	wrong[3] = big.NewRat(1, 3)
	if got, e := tube.CertifyAffineParameterPath(context.Background(), start, wrong, TubeAffineLineOptions{}); e == nil || got.Proven {
		t.Fatal("false path")
	}
}

package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveGraphBoundaryHeightOrderAcrossDifferentOriginalClocks(t *testing.T) {
	ctx := context.Background()
	chart := func(left, right, weight, lowerShift, upperShift float64) *SurfacePatch {
		t.Helper()
		var poles []Vec3
		for _, x := range []float64{left, right} {
			poles = append(poles, Vec3{x, 0, 1 + lowerShift}, Vec3{x, 1, 1 + x + upperShift})
		}
		knots := []float64{0, 0, 1, 1}
		s, err := NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, []float64{1, weight, 1, weight})
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewSurfacePatch(s, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := chart(0, 1, 1, 0, 0)
	b := chart(1, 3, 2, .25, .25)
	z := chart(1, 3, 2, 0, 0)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for _, tc := range []struct {
			a, b  *SurfacePatch
			sign  int
			clock *big.Rat
		}{{a, b, -1, big.NewRat(1, 2)}, {b, a, 1, big.NewRat(2, 1)}, {a, z, 0, big.NewRat(1, 2)}} {
			r, err := CertifyProjectiveGraphBoundaryHeightOrder(ctx, tc.a, tc.b, axes, SurfaceAffineCurveOptions{})
			gap := big.NewRat(int64(tc.sign), 4)
			if err != nil || !r.Complete || r.Sign != tc.sign || r.Boundary.BClockRatio.Cmp(tc.clock) != 0 ||
				r.Difference.Lower.Cmp(gap) != 0 || r.Difference.Upper.Cmp(gap) != 0 {
				t.Fatal("wrong rational enclosure on the original neighboring edge", axes, tc.sign, r, err)
			}
			budget := SurfaceAffineCurveOptions{TubeAffineLineOptions: TubeAffineLineOptions{MaxOperations: r.ExactOperations - 1}}
			if short, err := CertifyProjectiveGraphBoundaryHeightOrder(ctx, tc.a, tc.b, axes, budget); !errors.Is(err, ErrConvergence) || short.Complete {
				t.Fatal("short arithmetic budget must refuse", short, err)
			}
		}
	}
	c := chart(1, 3, 2, -.5, .5)
	r, err := CertifyProjectiveGraphBoundaryHeightOrder(ctx, a, c, [2]int{0, 1}, SurfaceAffineCurveOptions{})
	if !errors.Is(err, ErrConvergence) || r.Complete || r.Difference.Lower.Cmp(big.NewRat(-1, 2)) != 0 || r.Difference.Upper.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("a real edge crossing cannot certify a strict order", r, err)
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if r, err := CertifyProjectiveGraphBoundaryHeightOrder(cancel, a, b, [2]int{0, 1}, SurfaceAffineCurveOptions{}); r.Complete || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled boundary proof cannot certify order", r, err)
	}
}

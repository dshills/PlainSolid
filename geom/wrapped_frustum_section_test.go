package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactFrustumSectionNonbinaryParameter(t *testing.T) {
	ctx := context.Background()
	for _, circular := range []bool{false, true} {
		w, _, e := DraftWrappedFrustum(ctx, queryCylinder(t, circular), 0, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s, e := SectionWrappedFrustum(ctx, w, 0, SurfacePatchOptions{})
		if e != nil || s.ParameterV.Cmp(big.NewRat(1, 3)) != 0 || s.Radius.Cmp(big.NewRat(4, 3)) != 0 || s.Cap != 0 {
			t.Fatal(s, e)
		}
		jet, e := s.Curve.patch.EvaluateRational(ctx, big.NewRat(0, 1), big.NewRat(0, 1), SurfacePatchEvaluationOptions{})
		if e != nil || jet.Point[0].Cmp(big.NewRat(4, 3)) != 0 || jet.Point[1].Sign() != 0 || jet.Point[2].Sign() != 0 {
			t.Fatal("rounded nonbinary section", jet, e)
		}
		perimeter, e := BoundExactCurveLengthContext(ctx, s.Curve, CurveLengthBoundOptions{Tolerance: .01})
		if e != nil || !perimeter.Complete || perimeter.Upper-perimeter.Lower > .01 {
			t.Fatal(perimeter, e)
		}
		t.Logf("circular=%v perimeter gap=%g spans=%d operations=%d", circular, perimeter.Upper-perimeter.Lower, perimeter.Spans, perimeter.ExactOperations)
		want := 2 * math.Pi * 4 / 3
		if !circular {
			// Independent Simpson integral of a single rational .75-weight quadrant.
			sum := 0.
			const n = 4096
			for i := 0; i <= n; i++ {
				t0 := float64(i) / n
				q := 1 - t0
				den := q*q + 1.5*q*t0 + t0*t0
				dd := 1.5*(1-2*t0) - 2*q + 2*t0
				x := q*q + 1.5*q*t0
				y := 1.5*q*t0 + t0*t0
				dx := -2*q + 1.5*(1-2*t0)
				dy := 1.5*(1-2*t0) + 2*t0
				vx, vy := (dx*den-x*dd)/(den*den), (dy*den-y*dd)/(den*den)
				factor := 2.
				if i%2 == 1 {
					factor = 4
				}
				if i == 0 || i == n {
					factor = 1
				}
				sum += factor * math.Hypot(vx, vy) / (3 * n)
			}
			want = 4 * sum * 4 / 3
		}
		if want < perimeter.Lower || want > perimeter.Upper {
			t.Fatal("independent perimeter", want, perimeter)
		}
		cap, e := SectionWrappedFrustum(ctx, w, 2, SurfacePatchOptions{})
		if e != nil || cap.Cap != CylinderUpperCap || cap.ParameterV.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(cap, e)
		}
		denied, e := SectionWrappedFrustum(ctx, w, 0, SurfacePatchOptions{MaxOperations: 1})
		if e == nil || denied.Curve != nil {
			t.Fatal("partial limit", e)
		}
		_, e = SectionWrappedFrustum(ctx, w, 3, SurfacePatchOptions{})
		if !errors.Is(e, ErrDomain) {
			t.Fatal(e)
		}
	}
}

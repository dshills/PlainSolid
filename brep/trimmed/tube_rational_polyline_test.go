package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func rationalTubeFixture(t *testing.T) geom.VerifiedSurfaceTube {
	t.Helper()
	k := []float64{0, 0, 0, 1, 1, 1}
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	source, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.Trim(geom.ParameterRectangle{UMin: 7. / 32, UMax: .25, VMin: 5. / 8, VMax: 11. / 16})
	if e != nil {
		t.Fatal(e)
	}
	linear := []float64{0, 0, 1, 1}
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, linear, linear, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewSurfacePatch(plane, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Trim(geom.ParameterRectangle{UMin: 3. / 16, UMax: 9. / 32, VMin: 19. / 32, VMax: 23. / 32})
	if e != nil {
		t.Fatal(e)
	}
	tube, e := geom.NewVerifiedSurfaceTube(a, b, geom.SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil {
		t.Fatal(e)
	}
	return tube
}
func TestCertifiedRationalTubePolyline(t *testing.T) {
	ctx := context.Background()
	tube := rationalTubeFixture(t)
	start, end := big.NewRat(7, 30), big.NewRat(1, 4)
	o := RationalTubePolylineOptions{Deflection: .005}
	got, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Samples) != len(got.SquaredErrors)+1 || len(got.SquaredErrors) < 2 || got.Samples[0].Free.Cmp(start) != 0 || got.Samples[len(got.Samples)-1].Free.Cmp(end) != 0 {
		t.Fatal("exact domain coverage")
	}
	irrational := 0
	for _, sample := range got.Samples {
		if !sample.Identity.Proven || sample.Identity.ModelEvent == nil {
			t.Fatal("missing exact graph identity")
		}
		kind, e := sample.Identity.Parameters[1].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if kind.Kind == geom.ExactRootKindIrrational {
			irrational++
		}
	}
	if irrational == 0 {
		t.Fatal("fixture dependent graph roots must remain algebraic")
	}
	for i, bound := range got.SquaredErrors {
		if bound.Cmp(big.NewRat(1, 40000)) > 0 {
			t.Fatal("deflection bound")
		}
		a, b := got.Samples[i], got.Samples[i+1]
		if a.Free.Cmp(b.Free) >= 0 {
			t.Fatal("exact ordering")
		}
		u, _ := a.Free.Float64()
		v, _ := b.Free.Float64()
		limit, _ := bound.Float64()
		for j := 0; j <= 16; j++ {
			lambda := float64(j) / 16
			x := (1-lambda)*u + lambda*v
			actual := geom.Vec3{X: x, Y: math.Sqrt(.5 - x*x), Z: .5}
			display := a.Point.Scale(1 - lambda).Add(b.Point.Scale(lambda))
			delta := actual.Sub(display)
			if delta.Dot(delta) > limit+1e-18 {
				t.Fatal("independent circle correspondence", delta.Dot(delta), limit)
			}
		}
	}
	// Adjacent rational subarcs consume the same externally selected endpoint.
	mid := got.Samples[len(got.Samples)/2]
	leftOptions := o
	leftOptions.EndpointPoints = &[2]geom.Vec3{got.Samples[0].Point, mid.Point}
	left, e := TessellateRationalTubeGraphCertified(ctx, tube, start, mid.Free, leftOptions)
	if e != nil {
		t.Fatal(e)
	}
	rightOptions := o
	rightOptions.EndpointPoints = &[2]geom.Vec3{mid.Point, got.Samples[len(got.Samples)-1].Point}
	right, e := TessellateRationalTubeGraphCertified(ctx, tube, mid.Free, end, rightOptions)
	if e != nil {
		t.Fatal(e)
	}
	if left.Samples[len(left.Samples)-1].Point != right.Samples[0].Point {
		t.Fatal("shared endpoint changed")
	}
	wrong := o
	wrong.MaxDepth = 2
	badEnd := got.Samples[len(got.Samples)-1].Point
	badEnd.Z += .1
	wrong.EndpointPoints = &[2]geom.Vec3{got.Samples[0].Point, badEnd}
	if partial, err := TessellateRationalTubeGraphCertified(ctx, tube, start, end, wrong); err == nil || len(partial.Samples) != 0 {
		t.Fatal("bad shared display not independently bounded")
	}

	low := o
	low.MaxOperations = 1
	if partial, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, low); e == nil || len(partial.Samples) != 0 {
		t.Fatal("operation budget")
	}
	low = o
	low.MaxSegments = 1
	if partial, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, low); e == nil || len(partial.Samples) != 0 {
		t.Fatal("segment budget")
	}
	low = o
	low.Point.MaxOperations = 1
	if partial, err := TessellateRationalTubeGraphCertified(ctx, tube, start, end, low); err == nil || len(partial.Samples) != 0 {
		t.Fatal("per-point work limit ignored")
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TessellateRationalTubeGraphCertified(cancelled, tube, start, end, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	t.Log("segments", len(got.SquaredErrors), "operations", got.ExactOperations, "irrational graph samples", irrational)
}

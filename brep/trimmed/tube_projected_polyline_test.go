package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func projectedRationalTubeFixture(t *testing.T) geom.VerifiedSurfaceTube {
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
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Z: float64(j) / 2, Y: z})
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
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5}, {Z: 1, Y: .5}, {X: 1, Y: .5}, {X: 1, Z: 1, Y: .5}}, linear, linear, nil)
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

func TestCertifiedProjectedRationalTubePolyline(t *testing.T) {
	ctx := context.Background()
	tube := projectedRationalTubeFixture(t)
	start, end := big.NewRat(7, 30), big.NewRat(1, 4)
	axes := [2]int{0, 2}
	o := RationalTubePolylineOptions{Deflection: .005, Projection: &axes}
	r, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, o)
	if e != nil {
		t.Fatal(e)
	}
	irrational := false
	for _, p := range r.Samples {
		kind, e := p.Identity.Parameters[1].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		irrational = irrational || kind.Kind == geom.ExactRootKindIrrational
	}
	if !irrational {
		t.Fatal("expected algebraic dependent coordinate")
	}
	for i, bound := range r.SquaredErrors {
		lo, _ := r.Samples[i].Free.Float64()
		hi, _ := r.Samples[i+1].Free.Float64()
		limit, _ := bound.Float64()
		if limit > .005*.005 {
			t.Fatal(limit)
		}
		for j := 0; j <= 16; j++ {
			f := float64(j) / 16
			x := lo + (hi-lo)*f
			actual := geom.Vec3{X: x, Y: .5, Z: math.Sqrt(.5 - x*x)}
			display := r.Samples[i].Point.Scale(1 - f).Add(r.Samples[i+1].Point.Scale(f))
			d := actual.Sub(display)
			if d.Dot(d) > limit+1e-18 {
				t.Fatal("independent XZ circle", d, limit)
			}
		}
	}
	o.Projection = nil
	if _, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, o); e == nil {
		t.Fatal("old XY path unexpectedly accepted degenerate graph projection")
	}
	o.Projection = &axes
	o.MaxSegments = 1
	if bad, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, o); !errors.Is(e, geom.ErrConvergence) || len(bad.Samples) != 0 {
		t.Fatal(e)
	}
	badAxes := [2]int{2, 2}
	o.Projection = &badAxes
	if _, e := TessellateRationalTubeGraphCertified(ctx, tube, start, end, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}

func TestProjectedPolylineEnclosureLimits(t *testing.T) {
	r := geom.SurfaceTubeIntervalEnclosure{HasBounds: true, Bounds: geom.Box{Min: geom.Vec3{}, Max: geom.Vec3{X: 1, Y: 1, Z: 1}}, ExactOperations: 10}
	if _, e := projectedPolylineEnclosure(r, geom.ErrConvergence, 11); e != nil {
		t.Fatal("lost valid enclosure", e)
	}
	for _, q := range []struct {
		err   error
		limit int
	}{{geom.ErrConvergence, 10}, {context.Canceled, 11}, {geom.ErrInvalid, 11}} {
		if _, e := projectedPolylineEnclosure(r, q.err, q.limit); e == nil {
			t.Fatal("ignored budget or nonrefinement error")
		}
	}
	r.HasBounds = false
	if _, e := projectedPolylineEnclosure(r, geom.ErrConvergence, 11); e == nil {
		t.Fatal("unproved box accepted")
	}
}

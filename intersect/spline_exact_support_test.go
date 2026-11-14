package intersect

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestTraceSplinePairWithNonbinarySupport(t *testing.T) {
	ctx := context.Background()
	// Unlike a rounded spline, this chart's X=1/3 endpoint is an actual
	// incidence. Its original homogeneous controls store it exactly.
	var net [][4]*big.Rat
	for _, x := range []int64{1, 2} {
		for _, y := range []int64{0, 1} {
			w := big.NewRat(1, 1)
			net = append(net, [4]*big.Rat{new(big.Rat).Mul(big.NewRat(x, 3), w), new(big.Rat).Mul(big.NewRat(y, 1), w), big.NewRat((x-1)*y, 3), w})
		}
	}
	patch, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, net, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = patch.ToBSplineSurface(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("fixture must have no lossless float spline representation", e)
	}
	exact, e := geom.NewExactBezierSurface(patch)
	if e != nil {
		t.Fatal(e)
	}
	knots := []float64{0, 0, 1, 1}
	other, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, b := Patch{exact, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}}, Patch{other, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}}
	r, e := TraceSplinePair(ctx, a, b, SplinePairTraceOptions{})
	if e != nil || !r.Complete || !r.Coverage.CoversAllContacts || len(r.Curves) != 1 || len(r.Curves[0]) == 0 {
		t.Fatal("full nonbinary contact branch was not qualified", r.Issues, e)
	}
	curve := r.Curves[0][0]
	var endpoints [2]string
	for i := range endpoints {
		h, e := DiscoverRationalSplineTubeTerminal(ctx, curve, i, a, b, geom.TubeEndpointEdgeOptions{})
		if e != nil || !h.Proof.Proven || h.Proof.Point[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("terminal did not retain the original exact source point", h, e)
		}
		wantZ := new(big.Rat).Quo(new(big.Rat).Sub(h.Proof.Point[0], big.NewRat(1, 3)), big.NewRat(2, 1))
		if h.Proof.Point[2].Cmp(wantZ) != 0 {
			t.Fatal("terminal left the original curved support", h.Proof.Point)
		}
		endpoints[i] = h.Proof.Point[0].RatString()
	}
	if endpoints != [2]string{"1/3", "2/3"} && endpoints != [2]string{"2/3", "1/3"} {
		t.Fatal("nonbinary endpoints were rounded", endpoints)
	}
	if curve.TrimA().SourceSurface() != exact || curve.TrimB().Support() != other {
		t.Fatal("paired trims lost their original supports")
	}
	if r, e := TraceSplinePair(ctx, a, a, SplinePairTraceOptions{}); !errors.Is(e, ErrUnresolved) || !r.Coincidence.Found || r.Complete {
		t.Fatal("self coincidence must stay separate from finite contacts", r.Coincidence, e)
	}
	if _, e := FindSplineCoincidence(ctx, a, b, SplineCoincidenceOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("nonbinary patch bypassed the caller's limit", e)
	}
}

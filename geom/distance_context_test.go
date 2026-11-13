package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

// cancelAtCheck deterministically cancels an actual context after search has
// established bounds, avoiding timing-dependent goroutine sleeps in regression.
type cancelAtCheck struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *cancelAtCheck) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func distanceCancelContext(n int) *cancelAtCheck {
	ctx, cancel := context.WithCancel(context.Background())
	return &cancelAtCheck{ctx, cancel, n}
}
func TestCertifiedDistancePreCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	c, _ := NewBezier([]Vec3{{}, {X: 1}}, nil)
	a, e := BoundPointSurfaceDistanceContext(ctx, Vec3{}, p, SurfaceDistanceOptions{})
	if !errors.Is(e, context.Canceled) || a.Complete || !math.IsInf(a.SquaredDistanceMax, 1) {
		t.Fatal(a, e)
	}
	b, e := BoundSurfacePairDistanceContext(ctx, p, p, SurfacePairDistanceOptions{})
	if !errors.Is(e, context.Canceled) || b.Complete || !math.IsInf(b.SquaredDistanceMax, 1) {
		t.Fatal(b, e)
	}
	d, e := BoundPointCurveDistanceContext(ctx, Vec3{}, c, CurveDistanceOptions{})
	if !errors.Is(e, context.Canceled) || d.Complete || !math.IsInf(d.SquaredDistanceMax, 1) {
		t.Fatal(d, e)
	}
	f, e := BoundCurvePairDistanceContext(ctx, c, c, CurvePairDistanceOptions{})
	if !errors.Is(e, context.Canceled) || f.Complete || !math.IsInf(f.SquaredDistanceMax, 1) {
		t.Fatal(f, e)
	}
	g, e := BoundCurveSurfaceDistanceContext(ctx, c, p, CurveSurfaceDistanceOptions{})
	if !errors.Is(e, context.Canceled) || g.Complete || !math.IsInf(g.SquaredDistanceMax, 1) {
		t.Fatal(g, e)
	}
}
func TestCertifiedDistanceCancellationRetainsWitnessBounds(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	ctx := distanceCancelContext(4)
	r, e := BoundPointSurfaceDistanceContext(ctx, Vec3{2, .3, 0}, p, SurfaceDistanceOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || !r.HasUpperPoint || r.VisitedCells != 1 || r.SquaredDistanceMin > 1 || r.SquaredDistanceMax < 1 {
		t.Fatal(r, e)
	}
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .3} })
	ctx = distanceCancelContext(4)
	pair, e := BoundSurfacePairDistanceContext(ctx, a, b, SurfacePairDistanceOptions{})
	if !errors.Is(e, context.Canceled) || pair.Complete || !pair.HasWitness || pair.VisitedPairs != 1 || pair.SquaredDistanceMin > .03 || pair.SquaredDistanceMax < .03 {
		t.Fatal(pair, e)
	}
}
func TestCertifiedDistanceContextUncancelledMatches(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, e := BoundPointSurfaceDistance(Vec3{.5, .5, 2}, p, SurfaceDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := BoundPointSurfaceDistanceContext(context.Background(), Vec3{.5, .5, 2}, p, SurfaceDistanceOptions{})
	if e != nil || a != b {
		t.Fatal(a, b, e)
	}
}

func TestCurveDistanceContextsCancelDuringSearch(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	other, _ := NewBezier([]Vec3{{.3, .2, 1}, {.4, .3, 2}}, nil)
	surface := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{.3 + u, .2 + v, 1 + u*v} })
	point, e := BoundPointCurveDistanceContext(distanceCancelContext(6), Vec3{.3, .2, 1}, curve, CurveDistanceOptions{})
	if !errors.Is(e, context.Canceled) || !point.HasWitness || point.Complete || point.VisitedCells != 1 || point.SquaredDistanceMin > point.SquaredDistanceMax {
		t.Fatal(point, e)
	}
	pair, e := BoundCurvePairDistanceContext(distanceCancelContext(7), curve, other, CurvePairDistanceOptions{})
	if !errors.Is(e, context.Canceled) || !pair.HasWitness || pair.Complete || pair.VisitedPairs != 1 || pair.SquaredDistanceMin > pair.SquaredDistanceMax {
		t.Fatal(pair, e)
	}
	mixed, e := BoundCurveSurfaceDistanceContext(distanceCancelContext(6), curve, surface, CurveSurfaceDistanceOptions{})
	if !errors.Is(e, context.Canceled) || !mixed.HasWitness || mixed.Complete || mixed.VisitedPairs != 1 || mixed.SquaredDistanceMin > mixed.SquaredDistanceMax {
		t.Fatal(mixed, e)
	}
}

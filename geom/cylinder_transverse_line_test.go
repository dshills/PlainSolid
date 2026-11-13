package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestTransverseCylinderRegularizedLine(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: -1}, Min: -12, Max: -8, Radius: 3}
	u, _ := NewTransverseCylinderSetQuery(a, b, false)
	r, e := u.ClipLine(ctx, Vec3{X: 3}, Vec3{Z: 2}, -1, 7, o)
	if e != nil || len(r.Material.Intervals) != 1 || len(r.Material.Points) != 0 || len(r.Sources.Hits) != 3 {
		t.Fatal(r, e)
	}
	iv := r.Material.Intervals[0]
	if iv.StartBounds.Lower.Cmp(big.NewRat(0, 1)) != 0 || iv.EndBounds.Upper.Cmp(big.NewRat(6, 1)) != 0 || iv.Boundary != 0 || iv.StartBoundary != RawCylinderFirstMin || iv.EndBoundary != RawCylinderSecondMin {
		t.Fatal(iv)
	}
	if r.Sources.Hits[1].Bounds.Lower.Cmp(big.NewRat(4, 1)) != 0 || r.Sources.Hits[1].Boundary != RawCylinderFirstMax|RawCylinderSecondMax {
		t.Fatal("canceled cap event", r.Sources)
	}
	b.Min, b.Max = -8, 0
	d, _ := NewTransverseCylinderSetQuery(a, b, true)
	r, e = d.ClipLine(ctx, Vec3{Z: 8}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Material.Intervals) != 1 || len(r.Material.Points) != 0 {
		t.Fatal(r, e)
	}
	iv = r.Material.Intervals[0]
	if iv.StartBounds.Lower.Cmp(big.NewRat(-5, 1)) != 0 || iv.EndBounds.Upper.Cmp(big.NewRat(1, 1)) != 0 || iv.Boundary != RawCylinderFirstMax || iv.EndBoundary != RawCylinderFirstMax|RawCylinderSecondSide {
		t.Fatal("regularized cap interval", iv)
	}
	found := false
	for _, v := range r.Sources.Intervals {
		if v.StartBounds.Lower.Cmp(big.NewRat(1, 1)) == 0 && v.EndBounds.Upper.Cmp(big.NewRat(5, 1)) == 0 && v.Boundary == RawCylinderFirstMax|RawCylinderSecondMin {
			found = true
		}
	}
	if !found {
		t.Fatal("canceled cap source incidence", r.Sources)
	}
	// Compare against the deliberately different closed-set operation.
	closed, _ := NewCylinderPairCommon(a, b)
	c, e := closed.ClipLineClosedDifference(ctx, Vec3{Z: 8}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(c.Intervals) == 0 || c.Intervals[len(c.Intervals)-1].EndBounds.Upper.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("closed difference contrast", c, e)
	}
}
func TestTransverseCylinderIrrationalLineAndLimits(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: -1}, Min: -8, Max: 0, Radius: 3}
	d, _ := NewTransverseCylinderSetQuery(a, b, true)
	r, e := d.ClipLine(ctx, Vec3{Y: 1, Z: 4}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Material.Intervals) != 1 || len(r.Sources.Hits) != 4 {
		t.Fatal(r, e)
	}
	v := r.Material.Intervals[0]
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	if square(v.StartBounds.Lower).Cmp(big.NewRat(24, 1)) < 0 || square(v.StartBounds.Upper).Cmp(big.NewRat(24, 1)) > 0 {
		t.Fatal("-sqrt24 enclosure", v)
	}
	lo := new(big.Rat).Sub(big.NewRat(4, 1), v.EndBounds.Upper)
	hi := new(big.Rat).Sub(big.NewRat(4, 1), v.EndBounds.Lower)
	if square(lo).Cmp(big.NewRat(8, 1)) > 0 || square(hi).Cmp(big.NewRat(8, 1)) < 0 {
		t.Fatal("4-sqrt8 enclosure", v)
	}
	// Three distinct side events are closer than the requested display bracket.
	o.ParameterTolerance = 1e-3
	r, e = d.ClipLine(ctx, Vec3{Y: math.Nextafter(3, 0), Z: 4}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Sources.Hits) != 4 {
		t.Fatal("near event order", r, e)
	}
	for i := 1; i < len(r.Sources.Hits); i++ {
		order, e := r.Sources.Hits[i-1].Parameter.Compare(ctx, r.Sources.Hits[i].Parameter, ExactCurvePlaneEventOptions{})
		if e != nil || !order.Proven || order.Order >= 0 {
			t.Fatal("source event order", order, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = d.ClipLine(canceled, Vec3{}, Vec3{X: 1}, -10, 10, o)
	if e != context.Canceled || len(r.Material.Intervals)+len(r.Sources.Hits)+len(r.Sources.Intervals) != 0 {
		t.Fatal("cancel atomic", r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = d.ClipLine(ctx, Vec3{Y: 1, Z: 4}, Vec3{X: 1}, -10, 10, o)
	if e == nil || len(r.Material.Intervals)+len(r.Sources.Hits)+len(r.Sources.Intervals) != 0 {
		t.Fatal("limit atomic", r, e)
	}
}

// Deterministically cancel during isolation, rather than relying on timing.
type transverseLineCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
}

func (c *transverseLineCancelContext) Err() error {
	c.calls++
	if c.calls == 12 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestTransverseCylinderLineMidIsolationCancellation(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &transverseLineCancelContext{Context: base, cancel: cancel}
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: 1}, Max: 8, Radius: 3}
	s, _ := NewTransverseCylinderSetQuery(a, b, true)
	r, e := s.ClipLine(ctx, Vec3{Y: 1, Z: 4}, Vec3{X: 1}, -10, 10, ToroidLineOptions{ParameterTolerance: 1e-12})
	if e != context.Canceled || ctx.calls < 12 || len(r.Material.Intervals)+len(r.Material.Points)+len(r.Sources.Hits)+len(r.Sources.Intervals) != 0 {
		t.Fatal("non-atomic cancellation", ctx.calls, r, e)
	}
}

func TestTransverseCylinderLineTangencyAndGenerator(t *testing.T) {
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: 1}, Max: 8, Radius: 3}
	s, _ := NewTransverseCylinderSetQuery(a, b, true)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	r, e := s.ClipLine(context.Background(), Vec3{Y: 5, Z: 4}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Material.Intervals) != 0 || len(r.Material.Points) != 1 || r.Material.Points[0].Multiplicity != 2 || r.Material.Points[0].Boundary != RawCylinderFirstSide {
		t.Fatal("isolated tangency", r, e)
	}
	r, e = s.ClipLine(context.Background(), Vec3{X: -5}, Vec3{Z: 1}, -1, 9, o)
	if e != nil || len(r.Material.Intervals) != 1 || r.Material.Intervals[0].Boundary != RawCylinderFirstSide || r.Material.Intervals[0].StartBoundary != RawCylinderFirstSide|RawCylinderFirstMin || r.Material.Intervals[0].EndBoundary != RawCylinderFirstSide|RawCylinderFirstMax {
		t.Fatal("continuous generator", r, e)
	}
}

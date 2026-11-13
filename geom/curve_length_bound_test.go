package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedCurveLengthStraight(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {3, 4, 0}}, nil)
	r, e := BoundCurveLength(c, CurveLengthBoundOptions{})
	if e != nil || !r.Complete || r.Lower != 5 || r.Upper != 5 {
		t.Fatal(r, e)
	}
}
func TestCertifiedCurveLengthRationalQuarter(t *testing.T) {
	c, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	r, e := BoundCurveLength(c, CurveLengthBoundOptions{Tolerance: .01})
	if e != nil || !r.Complete || r.Lower > math.Pi/2+1e-14 || r.Upper < math.Pi/2-1e-14 {
		t.Fatal(r, e)
	}
	refined, _ := c.InsertKnot(.3, 1)
	s, e := BoundCurveLength(refined, CurveLengthBoundOptions{Tolerance: .01})
	if e != nil || s.Lower > r.Upper || s.Upper < r.Lower {
		t.Fatal(r, s, e)
	}
}
func TestCertifiedCurveLengthLimitsAndCancel(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	r, e := BoundCurveLength(c, CurveLengthBoundOptions{MaxSpans: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.Lower > r.Upper || !finite(r.Upper) {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = BoundCurveLengthContext(ctx, c, CurveLengthBoundOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestCertifiedCurveLengthInProgressCancel(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	r, e := BoundCurveLengthContext(distanceCancelContext(4), c, CurveLengthBoundOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || r.Spans < 2 || r.Lower > r.Upper || !finite(r.Upper) {
		t.Fatal(r, e)
	}
}

package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedCurveDerivativesLinearScale(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {3, 4, 0}}, []float64{2, 2, 6, 6}, nil)
	r, e := BoundCurveDerivatives(c, CurveDerivativeBoundOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 1 {
		t.Fatal(r, e)
	}
	cell := r.Cells[0]
	if cell.SpeedUpper != 1.25 || cell.AccelerationUpper != 0 || cell.ChordDeviationUpper != 0 || cell.First[0].Min != .75 || cell.First[1].Max != 1 {
		t.Fatal(cell)
	}
}
func TestCertifiedCurveDerivativesRationalCircle(t *testing.T) {
	w := math.Sqrt(.5)
	c, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, w, 1})
	r, e := BoundCurveDerivatives(c, CurveDerivativeBoundOptions{MaxParameterWidth: .125})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, cell := range r.Cells {
		a, b := cell.Domain.Start, cell.Domain.End
		pa, _ := c.Eval(a)
		pb, _ := c.Eval(b)
		for j := 0; j <= 10; j++ {
			u := float64(j) / 10
			t0 := a + (b-a)*u
			W := 1 + 2*(w-1)*t0 + 2*(1-w)*t0*t0
			Wp := 2*(w-1) + 4*(1-w)*t0
			Wpp := 4 * (1 - w)
			X := 1 + 2*(w-1)*t0 + (1-2*w)*t0*t0
			Xp := 2*(w-1) + 2*(1-2*w)*t0
			Xpp := 2 * (1 - 2*w)
			Y := 2*w*t0 + (1-2*w)*t0*t0
			Yp := 2*w + 2*(1-2*w)*t0
			Ypp := 2 * (1 - 2*w)
			for axis, q := range [][3]float64{{X, Xp, Xpp}, {Y, Yp, Ypp}} {
				position := q[0] / W
				first := (q[1] - position*Wp) / W
				second := (q[2] - position*Wpp - 2*first*Wp) / W
				if first < cell.First[axis].Min-1e-12 || first > cell.First[axis].Max+1e-12 || second < cell.Second[axis].Min-1e-12 || second > cell.Second[axis].Max+1e-12 {
					t.Fatal(cell, t0, axis, first, second)
				}
			}
			point, _ := c.Eval(t0)
			chord := pa.Scale(1 - u).Add(pb.Scale(u))
			if point.Sub(chord).Length() > cell.ChordDeviationUpper+1e-14 {
				t.Fatal(cell, t0)
			}
		}
	}
}
func TestCertifiedCurveDerivativesKnotsAndLimits(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {1, 2, 0}}, []float64{0, 0, .5, 1, 1}, nil)
	r, e := BoundCurveDerivatives(c, CurveDerivativeBoundOptions{})
	if e != nil || len(r.Cells) != 2 || r.Cells[0].First[0].Min != 2 || r.Cells[1].First[1].Min != 4 || r.Cells[0].ChordDeviationUpper != 0 || r.Cells[1].ChordDeviationUpper != 0 {
		t.Fatal(r, e)
	}
	_, e = BoundCurveDerivatives(c, CurveDerivativeBoundOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = BoundCurveDerivativesContext(ctx, c, CurveDerivativeBoundOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestCertifiedIsoDerivatives(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 3}}, nil)
	iso := isoIntersectionFixture(t, c)
	r, e := iso.BoundDerivatives(CurveDerivativeBoundOptions{})
	if e != nil || !r.Complete || r.Cells[0].AccelerationUpper != 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = iso.BoundDerivativesContext(ctx, CurveDerivativeBoundOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

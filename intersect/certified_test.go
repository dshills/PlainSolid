package intersect

import (
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestCertifiedSplinePlanePairedContours(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	z := [3]float64{.25, -.25, .25}
	poles := make([]geom.Vec3, 0, 9)
	for i := range 3 {
		for j := range 3 {
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z[i] + z[j] - .12})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	cs, r, e := TraceSplinePlane(s, f, SplinePlaneOptions{})
	if e != nil || !r.Complete || len(cs) != 1 || !cs[0].Closed {
		t.Fatal(r, e)
	}
	for i, seg := range cs[0].Segments {
		for _, u := range []float64{0, .2, .5, .8, 1} {
			a, e := seg.SplineTrim().EvalUV(u)
			if e != nil {
				t.Fatal(i, u, e)
			}
			b, e := seg.PlaneTrim().EvalUV(u)
			if e != nil || math.Abs(a.X-b.X)+math.Abs(a.Y-b.Y) > 1e-10 {
				t.Fatal(i, a, b, e)
			}
			dx, dy := a.X-.5, a.Y-.5
			d, e := seg.PlaneTrim().DerivativeUV(u)
			if e != nil || math.Abs(dx*dx+dy*dy-.12) > 1e-8 || math.Abs(dx*d.X+dy*d.Y) > 1e-8 {
				t.Fatal(a, d, e)
			}
		}
		j := (i + 1) % len(cs[0].Segments)
		a, e := seg.PlaneTrim().EvalUV(1)
		if e != nil {
			t.Fatal(e)
		}
		b, e := cs[0].Segments[j].PlaneTrim().EvalUV(0)
		if e != nil || a != b {
			t.Fatal(i, j, a, b, e)
		}
	}
	if _, _, e = TraceSplinePlane(s, f, SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("expected work limit", e)
	}
	c, r, e := TraceSplinePlane(s, f, SplinePlaneOptions{Domain: geom.ParameterRectangle{UMin: 0, UMax: .125, VMin: 0, VMax: 1}})
	if e != nil || !r.Complete || len(c) != 0 {
		t.Fatal(c, r, e)
	}
}

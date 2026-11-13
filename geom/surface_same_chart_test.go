package geom

import (
	"context"
	"testing"
)

func TestCompareSurfaceChartsProportionalWeights(t *testing.T) {
	s := quarterCylinder(t)
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	d := s.Data()
	for i := range d.Weights {
		d.Weights[i] *= .5
	}
	other, e := NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewSurfacePatch(other, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e := CompareSurfaceCharts(context.Background(), p, q, SurfaceChartComparisonOptions{})
	if e != nil || !c.Identical {
		t.Fatal(c, e)
	}
	r, e := BoundSurfaceSurface(p, q, SurfaceSurfaceIsolationOptions{})
	if e != nil || len(r.Coincident) != 1 || !r.Complete {
		t.Fatal(r, e)
	}
	moved, e := q.Transformed(Translation(Vec3{Z: .125}))
	if e != nil {
		t.Fatal(e)
	}
	c, e = CompareSurfaceCharts(context.Background(), p, moved, SurfaceChartComparisonOptions{})
	if e != nil || c.Identical {
		t.Fatal(c, e)
	}
}

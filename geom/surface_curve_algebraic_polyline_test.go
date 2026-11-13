package geom

import (
	"context"
	"math"
	"testing"
)

func TestAlgebraicCellCertifiedPolyline(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	collection, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var previous Vec3
	for cellIndex, cell := range collection.Cells {
		options := ComposedCurvePolylineOptions{Deflection: 1e-3}
		if cellIndex == 1 {
			end, e := cell.EncloseEndpoint(context.Background(), 1, ComposedCurveEndpointOptions{Tolerance: 1e-6})
			if e != nil {
				t.Fatal(e)
			}
			points := [2]Vec3{previous, end.Point}
			options.EndpointPoints = &points
		}
		r, e := cell.TessellateCertified(context.Background(), options)
		if e != nil || !r.Complete || len(r.Segments) < 2 || len(r.Vertices) != len(r.Segments)+1 || r.MaximumError > options.Deflection {
			t.Fatal(r, e)
		}
		if cellIndex == 1 && r.Vertices[0].Point != previous {
			t.Fatal("junction not shared")
		}
		previous = r.Vertices[len(r.Vertices)-1].Point
		parameter := func(vertex ComposedPolylineVertex) float64 {
			if vertex.Algebraic {
				return math.Sqrt(.5)
			}
			value, _ := vertex.Parameter.Lower.Float64()
			return value
		}
		model := func(t float64) Vec3 { return Vec3{t * t, t, 2*t*t*t*t + t*t - t + .5} }
		for _, segment := range r.Segments {
			a, b := r.Vertices[segment.Vertices[0]], r.Vertices[segment.Vertices[1]]
			start, end := parameter(a), parameter(b)
			if start >= end {
				t.Fatal("parameter order")
			}
			for _, fraction := range []float64{0, .1, .25, .5, .75, .9, 1} {
				point := model(start + (end-start)*fraction)
				chord := a.Point.Scale(1 - fraction).Add(b.Point.Scale(fraction))
				if point.Sub(chord).Length() > segment.ErrorUpper+2e-14 {
					t.Fatal("independent chord error", point.Sub(chord).Length(), segment.ErrorUpper)
				}
			}
		}
	}
}
func TestAlgebraicCellPolylineRejectsLimitsAndWrongSharedPoint(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	collection, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell := collection.Cells[0]
	r, e := cell.TessellateCertified(context.Background(), ComposedCurvePolylineOptions{Deflection: 1e-4, MaxSegments: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	for _, segment := range r.Segments {
		if segment.ErrorUpper > 1e-4 {
			t.Fatal("invalid retained prefix")
		}
	}
	wrong := [2]Vec3{{X: 10}, {X: 10}}
	r, e = cell.TessellateCertified(context.Background(), ComposedCurvePolylineOptions{EndpointPoints: &wrong})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = cell.TessellateCertified(ctx, ComposedCurvePolylineOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}

package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestLineEllipseContacts(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	ellipse, _ := NewEllipse(frame, 2, 1)
	for _, tt := range []struct {
		y       float64
		count   int
		tangent bool
	}{{0, 2, false}, {1, 1, true}, {1.1, 0, false}, {math.Nextafter(1, 0), 2, false}, {math.Nextafter(1, 2), 0, false}} {
		line, _ := NewLine(Vec3{-3, tt.y, 0}, Vec3{1, 0, 0})
		result, e := IntersectLineEllipse(line, ellipse, PlanarLineOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
		for _, hit := range result.Hits {
			if hit.Tangent != tt.tangent || hit.ApproximateCoplanarity || hit.Residual > 1e-12 {
				t.Fatal(hit)
			}
			point, _ := ellipse.Eval(hit.EllipseParameter)
			near(t, point, hit.Point, 1e-12)
		}
	}
	line, _ := NewLine(Vec3{-3, 0, 0}, Vec3{1, 0, 0})
	result, e := IntersectLineEllipse(line, ellipse, PlanarLineOptions{LineSurfaceOptions: LineSurfaceOptions{UseParameterRange: true, MinParameter: 1, MaxParameter: 1}})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Parameter != 1 {
		t.Fatal(result, e)
	}
}
func TestRandomLineEllipseOracle(t *testing.T) {
	random := rand.New(rand.NewSource(19927))
	frame, _ := FrameFromNormal(Vec3{2, -3, 5}, Vec3{1, 2, 3})
	ellipse, _ := NewEllipse(frame, 3, 2)
	for trial := 0; trial < 150; trial++ {
		height := (2*random.Float64() - 1) * 1.8
		line, _ := NewLine(frame.ToWorld(Vec3{-5, height, 0}), frame.XAxis())
		result, e := IntersectLineEllipse(line, ellipse, PlanarLineOptions{})
		if e != nil || len(result.Hits) != 2 {
			t.Fatal(result, e)
		}
		half := 3 * math.Sqrt(1-height*height/4)
		for i, want := range []float64{5 - half, 5 + half} {
			if math.Abs(result.Hits[i].Parameter-want) > 5e-13 || result.Hits[i].Residual > 5e-13 {
				t.Fatal(result.Hits[i], want)
			}
		}
	}
}
func TestSplineLineRootsAndTangency(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	line, _ := NewLine(Vec3{}, Vec3{1, 0, 0})
	curve, _ := NewBezier([]Vec3{{0, -4, 0}, {1.0 / 3, 7, 0}, {2.0 / 3, -7, 0}, {1, 4, 0}}, nil)
	result, e := IntersectSplineLine(curve, line, plane, SplineLineOptions{})
	if e != nil || len(result.Hits) != 3 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{.2, .5, .8} {
		hit := result.Hits[i]
		if math.Abs(hit.Parameter-want) > 1e-10 || math.Abs(hit.LineParameter-want) > 1e-10 || hit.LineResidual > 1e-8 || hit.Multiplicity != 1 {
			t.Fatal(hit, want)
		}
	}
	curve, _ = NewBezier([]Vec3{{0, 1, 0}, {.5, -2, 0}, {1, 4, 0}}, nil)
	result, e = IntersectSplineLine(curve, line, plane, SplineLineOptions{})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Multiplicity != 2 || !result.Hits[0].Tangent {
		t.Fatal(result, e)
	}
	reverse, _ := NewLine(Vec3{}, Vec3{-1, 0, 0})
	reversed, e := IntersectSplineLine(curve, reverse, plane, SplineLineOptions{})
	if e != nil || len(reversed.Hits) != 1 || math.Abs(reversed.Hits[0].LineParameter+result.Hits[0].LineParameter) > 1e-14 {
		t.Fatal(reversed, e)
	}
}
func TestSplineLineOverlapsAndClosedCurve(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	line, _ := NewLine(Vec3{}, Vec3{1, 0, 0})
	curve, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {2, 1, 0}}, []float64{0, 0, 1, 2, 2}, nil)
	result, e := IntersectSplineLine(curve, line, plane, SplineLineOptions{})
	if e != nil || len(result.Coincident) != 1 || len(result.Hits) != 1 || result.Coincident[0].Span.Start != 0 || result.Coincident[0].Span.End != 1 || result.Coincident[0].MaxControlResidual != 0 {
		t.Fatal(result, e)
	}
	circle, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 1)
	curve, _ = circle.RationalBSpline()
	result, e = IntersectSplineLine(curve, line, plane, SplineLineOptions{})
	if e != nil || len(result.Hits) != 4 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{0, 2, 2, 4} {
		if result.Hits[i].Parameter != want {
			t.Fatal(result.Hits[i], want)
		}
	}
}
func TestPlanarIntersectionsRejectUnsupported(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	line, _ := NewLine(Vec3{}, Vec3{1, 0, 0})
	curve, _ := NewBezier([]Vec3{{0, -1, .001}, {1, 1, .001}}, nil)
	if _, e := IntersectSplineLine(curve, line, plane, SplineLineOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	curve, _ = NewBezier([]Vec3{{0, 0, 1e-7}, {1, 0, 1e-7}}, nil)
	if _, e := IntersectSplineLine(curve, line, plane, SplineLineOptions{PlanarityTolerance: 1e-6, SplinePlaneOptions: SplinePlaneOptions{DistanceTolerance: 1e-9}}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ellipse, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 1)
	vertical, _ := NewLine(Vec3{1, 0, -1}, Vec3{0, 0, 1})
	if _, e := IntersectLineEllipse(vertical, ellipse, PlanarLineOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	far, _ := NewLine(Vec3{0, 0, 1}, Vec3{1, 0, 0})
	if _, e := IntersectLineEllipse(far, ellipse, PlanarLineOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

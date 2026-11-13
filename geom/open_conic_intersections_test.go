package geom

import (
	"errors"
	"math"
	"testing"
)

func TestLineParabolaClassification(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	for _, tc := range []struct {
		x                   float64
		count, multiplicity int
	}{{1, 2, 1}, {0, 1, 2}, {-math.SmallestNonzeroFloat64, 0, 0}, {math.SmallestNonzeroFloat64, 2, 1}} {
		line, _ := NewLine(Vec3{X: tc.x}, Vec3{Y: 1})
		r, e := IntersectLineParabola(line, p, PlanarLineOptions{})
		if e != nil || len(r.Hits) != tc.count {
			t.Fatal(tc, r, e)
		}
		for _, hit := range r.Hits {
			if hit.Multiplicity != tc.multiplicity {
				t.Fatal(hit)
			}
		}
	}
	line, _ := NewLine(Vec3{}, Vec3{X: 1})
	r, e := IntersectLineParabola(line, p, PlanarLineOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Multiplicity != 1 {
		t.Fatal(r, e)
	}
}
func TestLineHyperbolaBranchesAndTangency(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	for _, branch := range []int{-1, 1} {
		h, _ := NewHyperbolaBranch(f, 2, 1, branch)
		line, _ := NewLine(Vec3{}, Vec3{X: 1})
		r, e := IntersectLineHyperbola(line, h, PlanarLineOptions{})
		if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameter != float64(branch)*2 {
			t.Fatal(r, e)
		}
		for _, x := range []float64{2, math.Nextafter(2, 0), math.Nextafter(2, 3)} {
			line, _ := NewLine(Vec3{X: float64(branch) * x}, Vec3{Y: 1})
			r, e := IntersectLineHyperbola(line, h, PlanarLineOptions{})
			count := 2
			if x == 2 {
				count = 1
			} else if x < 2 {
				count = 0
			}
			if e != nil || len(r.Hits) != count {
				t.Fatal(x, r, e)
			}
			if x == 2 && r.Hits[0].Multiplicity != 2 {
				t.Fatal(r)
			}
		}
	}
}
func TestLineOpenConicSkewAndSplineAgreement(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	skew, _ := NewLine(Vec3{Z: 1}, Vec3{Y: 1})
	if _, e := IntersectLineParabola(skew, p, PlanarLineOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	line, _ := NewLine(Vec3{X: .25}, Vec3{Y: 1})
	r, e := IntersectLineParabola(line, p, PlanarLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, _ := p.RationalBSpline(-1, 1)
	for _, hit := range r.Hits {
		point, e := s.Eval(hit.ConicParameter)
		if e != nil || point.Distance(hit.Point) > 1e-13 {
			t.Fatal(point, hit, e)
		}
	}
}

func TestLineOpenConicSignedClipping(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	line, _ := NewLine(Vec3{X: 1}, Vec3{Y: 1})
	for _, span := range [][2]float64{{-2, -2}, {2, 2}, {-1, 1}} {
		r, e := IntersectLineParabola(line, p, PlanarLineOptions{LineSurfaceOptions: LineSurfaceOptions{UseParameterRange: true, MinParameter: span[0], MaxParameter: span[1]}})
		want := 1
		if span[0] == -1 {
			want = 0
		}
		if e != nil || len(r.Hits) != want {
			t.Fatal(span, r, e)
		}
	}
}

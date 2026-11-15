package geom

import (
	"errors"
	"math"
	"testing"
)

func TestPeriodicSplineSurfaceTensorAndPhysicalPartials(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 2, DegreeV: 1, CountU: 3, CountV: 3,
		Poles:   []Vec3{{0, 0, 0}, {0, 1, 1}, {1, 2, 2}, {1, 0, 2}, {2, 1, 3}, {1, 3, 4}, {3, 0, 1}, {4, 1, 1}, {2, 3, 3}},
		Weights: []float64{2, 3, 2, 3, 1, 2, 1, 2, 3},
	}
	uAxis, vAxis := &PeriodicSplineAxis{Start: -2, Period: 6}, &PeriodicSplineAxis{Start: 1, Period: 9}
	surface, err := NewPeriodicBSplineSurface(data, uAxis, vAxis)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c, d := surface.Domain()
	if a != -2 || b != 4 || c != 1 || d != 10 {
		t.Fatalf("domain = %v %v %v %v", a, b, c, d)
	}
	// In cell U=0,V=1 the local knots have widths 2 and 3. At u=-1
	// and v=4.75 the uniform quadratic basis is (1,6,1)/8 and
	// the linear basis is (3,1)/4. Evaluate the original net directly.
	var numerator Vec3
	var denominator float64
	for i, u := range []float64{1. / 8, 6. / 8, 1. / 8} {
		for j, v := range []float64{.75, .25} {
			k := 3*i + j + 1
			w := u * v * data.Weights[k]
			numerator = numerator.Add(data.Poles[k].Scale(w))
			denominator += w
		}
	}
	want := numerator.Scale(1 / denominator)
	for _, periods := range [][2]int{{0, 0}, {-3, 2}, {4, -4}} {
		u, v := -1+6*float64(periods[0]), 4.75+9*float64(periods[1])
		actual, err := surface.Eval(u, v)
		if err != nil || actual.Distance(want) > 1e-13 {
			t.Fatalf("periods %v: %v, want %v (%v)", periods, actual, want, err)
		}
		du, dv, err := surface.Partials(u, v)
		if err != nil {
			t.Fatal(err)
		}
		const h = 1e-5
		for axis, partial := range []Vec3{du, dv} {
			delta := [2]float64{}
			delta[axis] = h
			left, el := surface.Eval(u-delta[0], v-delta[1])
			right, er := surface.Eval(u+delta[0], v+delta[1])
			if el != nil || er != nil || right.Sub(left).Scale(.5/h).Distance(partial) > 1e-8 {
				t.Fatalf("partial %d: %v, samples %v and %v, %v %v", axis, partial, left, right, el, er)
			}
		}
	}
	seam, err := surface.Eval(a, c)
	end, endErr := surface.Eval(b, d)
	leftU, leftV, leftErr := surface.Partials(a, c)
	rightU, rightV, rightErr := surface.Partials(b, d)
	if err != nil || endErr != nil || leftErr != nil || rightErr != nil || seam != end || leftU != rightU || leftV != rightV {
		t.Fatalf("period endpoints do not select the same seam: %v %v", seam, end)
	}
	left, _, err := surface.Partials(math.Nextafter(b, a), 4.75)
	right, _, rightErr := surface.Partials(a, 4.75)
	if err != nil || rightErr != nil || left.Distance(right) > 1e-13 {
		t.Fatalf("quadratic seam derivatives disagree: %v %v", left, right)
	}
	copy, copyU, copyV := surface.Data()
	copy.Poles[0].X, copy.Weights[0], copyU.Start, copyV.Period = 77, 77, 77, 77
	data.Poles[1].X, data.Weights[1], uAxis.Start, vAxis.Period = 99, 99, 99, 99
	current, err := surface.Eval(-1, 4.75)
	if err != nil || current.Distance(want) > 1e-13 {
		t.Fatal("caller mutation changed stored geometry", current, err)
	}
}

func TestPeriodicSplineSurfaceOneAxisDomain(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 2, CountV: 3,
		KnotsU: []float64{2, 2, 4, 4},
		Poles:  []Vec3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 6}, {1, 0, 6}, {0, 1, 6}},
	}
	vAxis := &PeriodicSplineAxis{Start: -1, Period: 3}
	surface, err := NewPeriodicBSplineSurface(data, nil, vAxis)
	if err != nil {
		t.Fatal(err)
	}
	point, err := surface.Eval(3, -3.75)
	if err != nil || point != (Vec3{.25, 0, 3}) {
		t.Fatalf("mixed axis point: %v, %v", point, err)
	}
	du, dv, err := surface.Partials(3, 2)
	if err != nil || du != (Vec3{0, 0, 3}) || dv != (Vec3{1, 0, 0}) {
		t.Fatalf("mixed axis partials: %v, %v, %v", du, dv, err)
	}
	n, err := surface.Normal(3, 2)
	if err != nil || n != (Vec3{0, 1, 0}) {
		t.Fatalf("normal: %v, %v", n, err)
	}
	if _, err := surface.Eval(1, 1); !errors.Is(err, ErrDomain) {
		t.Fatalf("ordinary axis domain: %v", err)
	}
	if _, err := surface.Eval(3, math.Inf(1)); !errors.Is(err, ErrDomain) {
		t.Fatalf("periodic nonfinite argument: %v", err)
	}
	if _, err := NewPeriodicBSplineSurface(data, vAxis, vAxis); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ambiguous cyclic knot vector: %v", err)
	}
	if _, err := NewPeriodicBSplineSurface(data, nil, &PeriodicSplineAxis{Start: 1e100, Period: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unrepresentably short period: %v", err)
	}
}

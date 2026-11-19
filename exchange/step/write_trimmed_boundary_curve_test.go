package step

import (
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// TestBoundaryCurveOf holds the curve the writer emits for an edge to the
// stored curve of the edge: the same point at every parameter, a major
// radius that is not the smaller one, and a conic that turns
// counterclockwise about the z axis of its frame as its parameter grows.
func TestBoundaryCurveOf(t *testing.T) {
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: -2, Z: 3}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: -1, Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	ellipse := func(rx, ry float64) geom.Curve {
		e, err := geom.NewEllipse(frame, rx, ry)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	line, err := geom.NewLine(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 2, Y: -1, Z: 5})
	if err != nil {
		t.Fatal(err)
	}
	s, c := math.Sincos(0.4)
	turn := geom.Affine{{c, -s, 0, 3}, {s, c, 0, -1}, {0, 0, 1, 2}}
	mirror := geom.Affine{{c, s, 0, 3}, {s, -c, 0, -1}, {0, 0, 1, 2}}
	for _, k := range []struct {
		name  string
		curve geom.Curve
		lo    float64
		hi    float64
	}{
		{"line", line, -1, 4},
		{"circle", ellipse(2, 2), 0.3, 1.8},
		{"ellipse, major axis first", ellipse(3, 2), 5, 6.2},
		{"ellipse, minor axis first", ellipse(2, 3), -0.5, 2.5},
		{"line, turned", parametric.MapCurve(line, turn), 0, 2},
		{"ellipse, minor axis first, turned", parametric.MapCurve(ellipse(2, 3), turn), 1, 3},
		{"line, mirrored", parametric.MapCurve(line, mirror), 0, 2},
		{"ellipse, major axis first, mirrored", parametric.MapCurve(ellipse(3, 2), mirror), 1, 3},
		{"ellipse, minor axis first, mirrored", parametric.MapCurve(ellipse(2, 3), mirror), 4, 9},
	} {
		t.Run(k.name, func(t *testing.T) {
			got, err := boundaryCurveOf(parametric.Edge{Curve: k.curve, Range: parametric.Interval{Min: k.lo, Max: k.hi}})
			if err != nil {
				t.Fatal(err)
			}
			if got.major < got.minor {
				t.Errorf("major radius %g, minor radius %g", got.major, got.minor)
			}
			for i := 0; i <= 8; i++ {
				at := k.lo + (k.hi-k.lo)*float64(i)/8
				want, err := k.curve.Eval(at)
				if err != nil {
					t.Fatal(err)
				}
				if d := got.at(at).Distance(want); d > 1e-12 {
					t.Errorf("at %g the written curve is %g from the stored one", at, d)
				}
				if got.line {
					continue
				}
				// The stored curve leaves the point counterclockwise about
				// the z axis of the frame that is written.
				velocity, err := k.curve.Derivative(at)
				if err != nil {
					t.Fatal(err)
				}
				if want.Sub(got.frame.Origin()).Cross(velocity).Dot(got.frame.ZAxis()) <= 0 {
					t.Errorf("at %g the stored curve turns clockwise in the written frame", at)
				}
			}
		})
	}
	for _, k := range []struct {
		name string
		edge parametric.Edge
		want error
	}{
		{"collapsed", parametric.Edge{}, ErrTrimmedBoundary},
		{"a range that does not grow", parametric.Edge{Curve: line, Range: parametric.Interval{Min: 1, Max: 1}}, geom.ErrInvalid},
		{"more than a turn", parametric.Edge{Curve: ellipse(2, 2), Range: parametric.Interval{Max: 7}}, geom.ErrInvalid},
		{"scaled", parametric.Edge{Curve: parametric.MapCurve(line, geom.Affine{{2, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}), Range: parametric.Interval{Max: 1}}, ErrTrimmedBoundary},
	} {
		t.Run(k.name, func(t *testing.T) {
			if _, err := boundaryCurveOf(k.edge); !errors.Is(err, k.want) {
				t.Fatalf("got %v, want %v", err, k.want)
			}
		})
	}
}

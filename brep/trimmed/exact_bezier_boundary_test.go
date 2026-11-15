package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestExtractExactBezierBoundarySteppedFuse(t *testing.T) {
	ctx := context.Background()
	src := clippedPolynomialStock(t, 1, 0, 0)
	fuse, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[2], src[1], 1, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	solid := fuse.Boundary()
	boundary, err := solid.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(boundary.Faces) != 20 || len(boundary.Edges) != 46 || len(boundary.Vertices) != 28 {
		t.Fatalf("boundary sizes: %d faces, %d edges, %d vertices", len(boundary.Faces), len(boundary.Edges), len(boundary.Vertices))
	}
	incidence := make([]int, len(boundary.Edges))
	curvedSeams, curvedWalls := 0, 0
	for i, f := range boundary.Faces {
		if (f.Spline == nil) == (f.Affine == nil) {
			t.Fatalf("face %d has ambiguous support", i)
		}
		for _, loop := range f.Loops {
			for _, u := range loop {
				incidence[u.Edge]++
				m := boundary.Edges[u.Edge].Model
				if len(u.UV.Control) > 2 {
					if f.Spline != nil {
						curvedSeams++
					} else {
						curvedWalls++
					}
				}
				for _, q := range []float64{0, .13, .4, .81, 1} {
					uv := evaluateBoundaryTestBezier(u.UV, q)
					want := evaluateBoundaryTestBezier(m, q)
					got, e := solid.faces[i].Surface.Eval(uv.X, uv.Y)
					delta := got.Sub(want)
					if e != nil || delta.Dot(delta) > 1e-24 {
						t.Fatalf("face %d edge %d direction %v parameter %v: %v != %v (%v)", i, u.Edge, u.Reversed, q, got, want, e)
					}
				}
			}
		}
	}
	for i, count := range incidence {
		if count != 2 {
			t.Fatalf("exported edge %d incidence %d", i, count)
		}
	}
	if curvedSeams == 0 || curvedWalls == 0 {
		t.Fatalf("expected original curved seams and walls: %d, %d", curvedSeams, curvedWalls)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := solid.ExtractExactBezierBoundary(canceled, edgecheck.Options{}); !errors.Is(err, context.Canceled) || len(got.Edges) != 0 {
		t.Fatalf("canceled extraction returned geometry: %v", err)
	}
}

func evaluateBoundaryTestBezier(span geom.ExactBezierSpan, t float64) geom.Vec3 {
	p := make([][4]float64, len(span.Control))
	for i, h := range span.Control {
		for d, x := range h {
			p[i][d], _ = x.Float64()
		}
	}
	for n := len(p) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			for d := range p[i] {
				p[i][d] = (1-t)*p[i][d] + t*p[i+1][d]
			}
		}
	}
	return geom.Vec3{X: p[0][0] / p[0][3], Y: p[0][1] / p[0][3], Z: p[0][2] / p[0][3]}
}

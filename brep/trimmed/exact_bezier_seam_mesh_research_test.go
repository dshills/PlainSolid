//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactBezierSharedSeamCertified(t *testing.T) {
	for _, degree := range []int{3, 4} {
		a, ta, b, tb := bezierSeamFixture(t, degree)
		o := ExactBezierSeamMeshOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .002}}}
		r, e := TessellateExactBezierSeamCertified(context.Background(), a, ta, b, tb, o)
		if e != nil {
			t.Fatal(e)
		}
		if !r.Agreement.Equal || !r.Polyline.Complete || len(r.UsesB) < 2 {
			t.Fatal("missing shared certified seam")
		}
		for i, s := range r.Polyline.Segments {
			v := r.UsesB[len(r.UsesB)-1-i]
			if v != [2]int{s.Vertices[1], s.Vertices[0]} {
				t.Fatal("opposite ownership lost")
			}
			if s.ErrorUpper > o.Display.Deflection {
				t.Fatal("deflection")
			}
		}
		// Independently sample the analytic graph (u,v,u*v), not the composed curve.
		for i := 0; i <= 200; i++ {
			uv, err := ta.EvalUV(float64(i) / 200)
			if err != nil {
				t.Fatal(err)
			}
			point := geom.Vec3{X: uv.X, Y: uv.Y, Z: uv.X * uv.Y}
			best := 1e9
			for _, s := range r.Polyline.Segments {
				x, y := r.Polyline.Vertices[s.Vertices[0]].Point, r.Polyline.Vertices[s.Vertices[1]].Point
				d := y.Sub(x)
				f := max(0., min(1., point.Sub(x).Dot(d)/d.Dot(d)))
				distance := point.Sub(x.Add(d.Scale(f))).Length()
				best = min(best, distance)
			}
			if best > o.Display.Deflection {
				t.Fatalf("independent graph point distance %g", best)
			}
		}
		// Equal endpoints do not establish equal interiors.
		h := tb.HomogeneousControls()
		h[1][0].Add(h[1][0], big.NewRat(1, 32))
		wrong, _ := NewExactBezierUV(h)
		failed, e := TessellateExactBezierSeamCertified(context.Background(), a, ta, b, wrong, o)
		if e == nil || len(failed.Polyline.Vertices) != 0 {
			t.Fatal("accepted unequal interior", e)
		}
		same, err := TessellateExactBezierSeamCertified(context.Background(), a, ta, b, tb.reversed(), o)
		if err == nil || len(same.Polyline.Vertices) != 0 {
			t.Fatal("accepted same-oriented owners")
		}
		limited := o
		limited.Agreement.MaxOperations = 1
		denied, err := TessellateExactBezierSeamCertified(context.Background(), a, ta, b, tb, limited)
		if err == nil || len(denied.Polyline.Vertices) != 0 {
			t.Fatal("ignored agreement budget")
		}
		o.Display.MaxSegments = 1
		failed, e = TessellateExactBezierSeamCertified(context.Background(), a, ta, b, tb, o)
		if e == nil || len(failed.Polyline.Vertices) != 0 {
			t.Fatal("partial result on limit")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, e = TessellateExactBezierSeamCertified(ctx, a, ta, b, tb, o)
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}

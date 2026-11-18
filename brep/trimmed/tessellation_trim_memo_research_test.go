//go:build research

package trimmed

import (
	"context"
	"errors"
	"plainsolid/brep/parametric"
	"reflect"
	"sync"
	"testing"
)

func TestTessellationTrimMemoEquivalence(t *testing.T) {
	ctx := context.Background()
	// The spline graph requires ordinary trim refinement even when analytic
	// primitive meshers are available.
	stock, _ := polynomialClipStock(t)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{3, 3, 3. / 16}, 0)
	result, e := CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := result.Solid
	before := s.Faces()
	for _, o := range []parametric.TessellationOptions{{Deflection: .1, Angle: .2}, {Deflection: .01, MaxTriangles: 1}, {Deflection: .1, AnalyticBound: true}} {
		old, oe := s.tessellateWithTrimMemo(ctx, o, nil)
		memo := &tessellationTrimMemo{limit: tessellationTrimMemoLimit}
		got, ge := s.tessellateWithTrimMemo(ctx, o, memo)
		if !errors.Is(ge, oe) || !reflect.DeepEqual(old, got) {
			t.Fatal("changed mesh or error", oe, ge)
		}
		if oe == nil {
			if memo.hits == 0 {
				t.Fatal("no reuse")
			}
			if e = got.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			tiny, te := s.tessellateWithTrimMemo(ctx, o, &tessellationTrimMemo{limit: 1})
			if te != nil || !reflect.DeepEqual(old, tiny) {
				t.Fatal("saturation changed mesh", te)
			}
		}
	}
	if !reflect.DeepEqual(before, s.Faces()) {
		t.Fatal("source mutation")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = s.Tessellate(canceled, parametric.TessellationOptions{Deflection: .1})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .3})
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
}

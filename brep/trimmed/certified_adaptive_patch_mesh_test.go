package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func localizedCertifiedPatch(t testing.TB) *geom.SurfacePatch {
	t.Helper()
	var poles []geom.Vec3
	for i, x := range []float64{0, .25, .5, .75, 1} {
		z := 0.
		if i == 3 {
			z = 1
		}
		for _, y := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(2, 1, 5, 2, poles, []float64{0, 0, 0, .5, .5, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestAdaptiveCertifiedGridLocalRefinement(t *testing.T) {
	ctx := context.Background()
	p := localizedCertifiedPatch(t)
	o := CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 16}
	uniform, e := TessellateKnotPatchCertified(ctx, p, o)
	if e != nil {
		t.Fatal(e)
	}
	adaptive, e := TessellateAdaptivePatchCertified(ctx, p, o)
	if e != nil || !adaptive.BothDirections {
		t.Fatal(e)
	}
	if len(adaptive.Mesh.Triangles) >= len(uniform.Mesh.Triangles) {
		t.Fatal("no localized refinement savings", len(adaptive.Mesh.Triangles), len(uniform.Mesh.Triangles))
	}
	for _, u := range adaptive.UParameters {
		if u > 0 && u < .5 {
			t.Fatal("flat interval unnecessarily refined", u)
		}
	}
	incidence := map[[2]int]int{}
	for _, tri := range adaptive.Mesh.Triangles {
		for i, a := range tri {
			b := tri[(i+1)%3]
			if a > b {
				a, b = b, a
			}
			incidence[[2]int{a, b}]++
		}
	}
	boundary := 0
	for _, n := range incidence {
		if n == 1 {
			boundary++
		} else if n != 2 {
			t.Fatal("nonconforming edge incidence")
		}
	}
	if boundary != 2*(len(adaptive.UParameters)+len(adaptive.VParameters)-2) {
		t.Fatal("hanging grid node")
	}
	t.Logf("triangles adaptive=%d uniform=%d", len(adaptive.Mesh.Triangles), len(uniform.Mesh.Triangles))
}
func TestAdaptiveWrappedExactSeamSamples(t *testing.T) {
	ctx := context.Background()
	w := independentWrappedToroid(t)
	r, e := TessellateAdaptiveWrappedPatchCertified(ctx, w, CertifiedPatchMeshOptions{Deflection: .5, MaxSubdivisions: 16})
	if e != nil || !r.BothDirections {
		t.Fatal(e)
	}
	if e := r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	d := w.Domain()
	for _, u := range r.UParameters {
		a, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(u), ratInset(d.VMin), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(u), ratInset(d.VMax), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for k := 0; k < 3; k++ {
			if a.Point[k].Cmp(b.Point[k]) != 0 {
				t.Fatal("nonidentical exact seam sample")
			}
		}
	}
	for _, v := range r.VParameters {
		a, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(d.UMin), ratInset(v), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(d.UMax), ratInset(v), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for k := 0; k < 3; k++ {
			if a.Point[k].Cmp(b.Point[k]) != 0 {
				t.Fatal("nonidentical U seam sample")
			}
		}
	}
	denied, e := TessellateAdaptiveWrappedPatchCertified(ctx, w, CertifiedPatchMeshOptions{Deflection: .0001, MaxTriangleChecks: 1})
	if e == nil || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("partial adaptive budget result", e)
	}
	for _, b := range r.SquaredErrors {
		if b.Cmp(big.NewRat(1, 4)) > 0 {
			t.Fatal("false tolerance")
		}
	}
}
func BenchmarkCertifiedKnotPatchMesh(b *testing.B) {
	p := localizedCertifiedPatch(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := TessellateKnotPatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 16}); e != nil {
			b.Fatal(e)
		}
	}
}
func BenchmarkCertifiedAdaptivePatchMesh(b *testing.B) {
	p := localizedCertifiedPatch(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := TessellateAdaptivePatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 16}); e != nil {
			b.Fatal(e)
		}
	}
}
func ExampleTessellateKnotPatchCertified() {
	s, _ := geom.NewBSplineSurface(1, 1, 3, 2, []geom.Vec3{{}, {Y: 1}, {X: .25}, {X: .25, Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, .25, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, _ := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	result, err := TessellateKnotPatchCertified(context.Background(), patch, CertifiedPatchMeshOptions{Deflection: 1e-6})
	fmt.Println(len(result.Mesh.Triangles), result.BothDirections, err)
	// Output: 4 true <nil>
}

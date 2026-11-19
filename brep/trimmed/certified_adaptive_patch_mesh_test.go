package trimmed

import (
	"context"
	"fmt"
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
func ExampleTessellateKnotPatchCertified() {
	s, _ := geom.NewBSplineSurface(1, 1, 3, 2, []geom.Vec3{{}, {Y: 1}, {X: .25}, {X: .25, Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, .25, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, _ := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	result, err := TessellateKnotPatchCertified(context.Background(), patch, CertifiedPatchMeshOptions{Deflection: 1e-6})
	fmt.Println(len(result.Mesh.Triangles), result.BothDirections, err)
	// Output: 4 true <nil>
}

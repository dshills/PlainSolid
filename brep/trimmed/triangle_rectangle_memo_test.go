package trimmed

import (
	"context"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestTriangleRectangleMemoExactEquivalence(t *testing.T) {
	ctx := context.Background()
	w := certifiedWrappedFrustumFixture(t, true, 2)
	p := w.FundamentalPatch()
	r, e := TessellateWrappedKnotPatchCertified(ctx, w, CertifiedPatchMeshOptions{Deflection: .1})
	if e != nil {
		t.Fatal(e)
	}
	var memo triangleRectangleMemo
	for i, uv := range r.UVTriangles {
		tri := r.Mesh.Triangles[i]
		model := [3]geom.Vec3{r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]}
		old, e := BoundSplineTriangleDeviation(ctx, p, uv, model)
		if e != nil {
			t.Fatal(e)
		}
		got, e := boundSplineTriangleDeviationPrepared(ctx, p, uv, model, &memo)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(old, got) || got.SquaredDistance.Cmp(r.SquaredErrors[i]) != 0 {
			t.Fatal("changed exact certificate", i)
		}
		// Caller-owned bound mutation must not affect subsequent certificates.
		got.SquaredDistance.SetInt64(-1)
		again, e := boundSplineTriangleDeviationPrepared(ctx, p, uv, model, &memo)
		if e != nil || !reflect.DeepEqual(old, again) {
			t.Fatal("mutable cached evidence", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = boundSplineTriangleDeviationPrepared(canceled, p, r.UVTriangles[0], [3]geom.Vec3{}, &memo); e != context.Canceled {
		t.Fatal(e)
	}
	if q, e := TessellateWrappedKnotPatchCertified(ctx, w, CertifiedPatchMeshOptions{Deflection: .1, MaxTriangleChecks: 1}); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("work budget bypass")
	}
}

func BenchmarkTriangleRectanglePreparation(b *testing.B) {
	for _, cached := range []bool{false, true} {
		name := "uncached"
		if cached {
			name = "one-entry"
		}
		b.Run(name, func(b *testing.B) {
			p, uv, model := sparseResidualFixture(b, false)
			var u [3]geom.Vec2
			var v [3]geom.Vec3
			for i := range u {
				u[i].X, _ = uv[i][0].Float64()
				u[i].Y, _ = uv[i][1].Float64()
				v[i].X, _ = model[i][0].Float64()
				v[i].Y, _ = model[i][1].Float64()
				v[i].Z, _ = model[i][2].Float64()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var memo triangleRectangleMemo
				var m *triangleRectangleMemo
				if cached {
					m = &memo
				}
				for j := 0; j < 2; j++ {
					if _, e := boundSplineTriangleDeviationPrepared(context.Background(), p, u, v, m); e != nil {
						b.Fatal(e)
					}
					u[1], u[2] = u[2], u[1]
					v[1], v[2] = v[2], v[1]
				}
			}
		})
	}
}

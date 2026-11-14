package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestCertifiedWrappedFrustumContinuousNormals(t *testing.T) {
	ctx := context.Background()
	for _, upper := range []float64{2, 1} {
		w := certifiedWrappedFrustumFixture(t, true, upper)
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
		r, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, w, .3, o)
		if e != nil {
			t.Fatal(upper, e)
		}
		old, e := TessellateWrappedFrustumClosedCertified(ctx, w, o)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(r.CertifiedWrappedFrustumMesh, old) {
			t.Fatal("angular proof changed existing display")
		}
		bound, _ := r.SquaredNormalTangent.Float64()
		for i, tri := range r.Lateral.Mesh.Triangles {
			a, b, c := r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]
			facet := b.Sub(a).Cross(c.Sub(a))
			uv := r.Lateral.UVTriangles[i]
			for k := 0; k <= 8; k++ {
				// Independent analytic circle/cone normal at interior UV correspondence.
				f := float64(k) / 8
				u := uv[0].X*(1-f) + uv[1].X*f
				v := uv[0].Y*(1-f) + uv[1].Y*f
				jet, e := w.FundamentalPatch().EvaluateRational(ctx, ratInset(u), ratInset(v), geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				x, _ := jet.Point[0].Float64()
				y, _ := jet.Point[1].Float64()
				radius := math.Hypot(x, y)
				source := geom.Vec3{X: 5 * x / radius, Y: 5 * y / radius, Z: 2 - upper}
				dot := facet.Dot(source)
				tan2 := facet.Cross(source).Dot(facet.Cross(source)) / (dot * dot)
				if dot <= 0 || tan2 > bound+1e-12 {
					t.Fatal("normal enclosure", upper, tan2, bound)
				}
			}
		}
		if r, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, w, 1e-6, o); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("insufficient angular mesh accepted")
		}
	}
}

func TestCertifiedWrappedFrustumNormalRejections(t *testing.T) {
	ctx := context.Background()
	w := certifiedWrappedFrustumFixture(t, true, 1)
	for _, angle := range []float64{0, -1, math.NaN(), math.Inf(1), 1.01} {
		if r, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, w, angle, CertifiedWrappedFrustumMeshOptions{}); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal(angle, e)
		}
	}
	noncircle := certifiedWrappedFrustumFixture(t, false, 1)
	if _, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, noncircle, .3, CertifiedWrappedFrustumMeshOptions{}); e == nil {
		t.Fatal("noncircle accepted")
	}
	if r, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, w, .3, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05, MaxTriangleChecks: 1}}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("budget", e)
	}
	// A reflected chart has the wrong canonical outward orientation; no sampled
	// normal repair or implicit placement interpretation is allowed.
	source, e := w.FundamentalPatch().ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	d := source.Data()
	for i := range d.Poles {
		d.Poles[i].X = -d.Poles[i].X
	}
	reflected, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(reflected, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rw, _, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, rw, .3, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("reflected noncanonical chart accepted")
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TessellateWrappedCircularFrustumNormalsCertified(cancelCtx, w, .3, CertifiedWrappedFrustumMeshOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}

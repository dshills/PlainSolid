package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestCertifiedWrappedBoreMaterialNormals(t *testing.T) {
	ctx := context.Background()
	for _, upper := range []float64{2, 1} {
		outer := certifiedWrappedFrustumFixture(t, true, upper)
		inner := scaledWrappedCylinderFixture(t, outer, .5)
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .1}}
		r, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, .4, o)
		if e != nil {
			t.Fatal(upper, e)
		}
		old, e := TessellateWrappedBoreClosedCertified(ctx, outer, inner, o)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(old, r.CertifiedWrappedBoreMesh) {
			t.Fatal("changed closed display")
		}
		bound, _ := r.SquaredNormalTangent.Float64()
		seen := [4]bool{}
		for i, tri := range r.Mesh.Triangles {
			owner := r.OriginalFace[i]
			seen[owner] = true
			a, b, c := r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]
			facet := b.Sub(a).Cross(c.Sub(a))
			for k := 0; k <= 4; k++ {
				point := a.Scale(1 - float64(k)/4).Add(b.Scale(float64(k) / 4))
				source := geom.Vec3{Z: -1}
				if owner < 2 {
					scale := 1.
					if owner == 1 {
						scale = .5
					}
					radius := math.Hypot(point.X, point.Y)
					source = geom.Vec3{X: 5 * point.X / radius, Y: 5 * point.Y / radius, Z: scale * (2 - upper)}
					if owner == 1 {
						source = source.Scale(-1)
					}
				} else if owner == 3 {
					source.Z = 1
				}
				dot := facet.Dot(source)
				cross := facet.Cross(source)
				tan2 := cross.Dot(cross) / (dot * dot)
				if dot <= 0 || tan2 > bound+1e-11 {
					t.Fatal("material normal", owner, tan2, bound)
				}
			}
		}
		if seen != ([4]bool{true, true, true, true}) {
			t.Fatal(seen)
		}
		if r, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, 1e-8, o); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("angular limit accepted")
		}
	}
}

func TestCertifiedWrappedBoreNormalLimits(t *testing.T) {
	ctx := context.Background()
	outer := certifiedWrappedFrustumFixture(t, true, 1)
	inner := scaledWrappedCylinderFixture(t, outer, .5)
	for _, angle := range []float64{0, -1, math.NaN(), 1.1} {
		if r, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, angle, CertifiedWrappedFrustumMeshOptions{}); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal(angle, e)
		}
	}
	noncircle := certifiedWrappedFrustumFixture(t, false, 1)
	if _, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, noncircle, .4, CertifiedWrappedFrustumMeshOptions{}); e == nil {
		t.Fatal("noncircular inner wall")
	}
	if r, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, .4, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .1, MaxTriangleChecks: 1}}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TessellateWrappedCircularBoreNormalsCertified(canceled, outer, inner, .4, CertifiedWrappedFrustumMeshOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}

//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestPlacedWrappedBoreExactCertificates(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	outer := certifiedWrappedFrustumFixture(t, true, 1)
	inner := scaledWrappedCylinderFixture(t, outer, .5)
	pose, e := placement.New([3]int{-3, 1, -2}, geom.Vec3{X: 8})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .1}}
	r, e := TessellatePlacedWrappedCircularBoreNormalsCertified(ctx, outer, inner, pose, .4, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(r.Mesh.Triangles, r.Local.Mesh.Triangles) {
		t.Fatal("incidence changed")
	}
	inverse, e := pose.Inverse()
	if e != nil {
		t.Fatal(e)
	}
	back, e := inverse.Matrix()
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range r.Mesh.Vertices {
		if back.Apply(p) != r.Local.Mesh.Vertices[i] {
			t.Fatal("not lossless")
		}
	}
	report, e := collision.SelfIntersections(r.Mesh, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("independent world contacts", report, e)
	}
	volume := new(big.Rat)
	bound, _ := r.Local.SquaredNormalTangent.Float64()
	for i, tri := range r.Mesh.Triangles {
		a, b, c := r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]
		volume.Add(volume, ddDot(exactBoundPoint(a), ddCross(exactBoundPoint(b), exactBoundPoint(c))))
		facet := b.Sub(a).Cross(c.Sub(a))
		owner := r.Local.OriginalFace[i]
		// Analytic world-axis-X normals; this does not use the returned matrix.
		source := geom.Vec3{X: 1}
		if owner < 2 {
			radius := math.Hypot(a.Y, a.Z)
			s := 1.
			if owner == 1 {
				s = .5
			}
			source = geom.Vec3{X: -s, Y: 5 * a.Y / radius, Z: 5 * a.Z / radius}
			if owner == 1 {
				source = source.Scale(-1)
			}
		} else if owner == 3 {
			source.X = -1
		}
		dot := facet.Dot(source)
		cross := facet.Cross(source)
		if dot <= 0 || cross.Dot(cross)/(dot*dot) > bound+1e-11 {
			t.Fatal("world normal", owner)
		}
	}
	volume.Quo(volume, big.NewRat(6, 1))
	if volume.Cmp(r.Local.SignedDisplayVolume) != 0 {
		t.Fatal("volume changed")
	}
	// No aliasing between canonical and placed display arrays.
	original := r.Local.Mesh.Vertices[0]
	r.Mesh.Vertices[0].X++
	if r.Local.Mesh.Vertices[0] != original {
		t.Fatal("mesh alias")
	}
}

func TestPlacedWrappedBoreRejectsRoundedTranslation(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	outer := certifiedWrappedFrustumFixture(t, true, 2)
	inner := scaledWrappedCylinderFixture(t, outer, .5)
	pose, e := placement.New([3]int{1, 2, 3}, geom.Vec3{Z: math.SmallestNonzeroFloat64})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := TessellatePlacedWrappedCircularBoreNormalsCertified(ctx, outer, inner, pose, .4, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .1}}); e == nil || len(r.Mesh.Vertices) != 0 || len(r.Local.Mesh.Vertices) != 0 {
		t.Fatal("rounded translation published", e)
	}
	if _, e := placement.New([3]int{-1, 2, 3}, geom.Vec3{}); e == nil {
		t.Fatal("reflection accepted")
	}
	if _, e := TessellatePlacedWrappedCircularBoreNormalsCertified(ctx, outer, inner, placement.Rigid{}, .4, CertifiedWrappedFrustumMeshOptions{}); e == nil {
		t.Fatal("zero pose accepted")
	}
}

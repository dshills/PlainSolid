package trimmed

import (
	"context"
	"errors"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func TestClosedDisplayEmbeddingRevalidation(t *testing.T) {
	box, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	shell := CertifiedClosedQuadraticShell{CertifiedQuadraticTrimCollection: CertifiedQuadraticTrimCollection{Mesh: box}}
	for i := range box.Triangles {
		shell.OriginalFace = append(shell.OriginalFace, i/2)
	}
	result, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{})
	if e != nil || result.Status != DisplayEmbeddingCertified || !result.ClosedManifold || !result.Connected || !result.Collision.Complete {
		t.Fatal(result, e)
	}
	limited, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{MaxPairs: 1})
	if !errors.Is(e, collision.ErrLimit) || limited.Status == DisplayEmbeddingCertified {
		t.Fatal("false certificate after work limit", limited, e)
	}
	// The certificate is not cached: a topology-preserving caller mutation
	// pulls the top corner through the bottom and must expose exact contacts.
	shell.Mesh.Vertices = append([]geom.Vec3(nil), box.Vertices...)
	shell.Mesh.Vertices[6] = geom.Vec3{X: .5, Y: .5, Z: -.5}
	crossing, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{})
	if e != nil || crossing.Status != DisplayEmbeddingContact || len(crossing.ContactFaces) == 0 || !crossing.ClosedManifold {
		t.Fatal("missed embeddedness failure", crossing, e)
	}
	for i, contact := range crossing.Collision.Contacts {
		if crossing.ContactFaces[i] != [2]int{contact.TriangleA / 2, contact.TriangleB / 2} {
			t.Fatal("lost source face contact ancestry")
		}
	}
	partial, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{MaxContacts: 1})
	if !errors.Is(e, collision.ErrLimit) || partial.Status != DisplayEmbeddingContact {
		t.Fatal("lost exact partial contact evidence", partial, e)
	}
	shell.Mesh = box
	shell.Mesh.Triangles = append([][3]int(nil), box.Triangles[1:]...)
	open, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{})
	if e == nil || open.Status == DisplayEmbeddingCertified {
		t.Fatal("mutated open topology certified")
	}
	// A disjoint or nested pair is intentionally not a single connected shell.
	inner, e := mesh.Box(geom.Vec3{X: .25, Y: .25, Z: .25}, geom.Vec3{X: .75, Y: .75, Z: .75})
	if e != nil {
		t.Fatal(e)
	}
	combined := mesh.Mesh{Vertices: append(append([]geom.Vec3(nil), box.Vertices...), inner.Vertices...), Triangles: append([][3]int(nil), box.Triangles...)}
	for _, tri := range inner.Triangles {
		combined.Triangles = append(combined.Triangles, [3]int{tri[0] + 8, tri[1] + 8, tri[2] + 8})
	}
	shell.Mesh = combined
	disconnected, e := shell.CheckDisplayEmbedding(context.Background(), collision.Options{})
	if e == nil || disconnected.Connected || disconnected.Status == DisplayEmbeddingCertified {
		t.Fatal("nested disconnected assembly certified")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = shell.CheckDisplayEmbedding(ctx, collision.Options{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

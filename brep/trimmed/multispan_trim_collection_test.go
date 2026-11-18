package trimmed

import (
	"context"
	"errors"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func multiSpanCollectionFixture(t *testing.T) ([]QuadraticTrimStitchFace, []QuadraticTrimStitchSeam) {
	t.Helper()
	a, b := multiSpanJoinFixture(t)
	inputs := []QuadraticTrimStitchFace{{Patch: a.Patch, Outer: a.Outer}, {Patch: b.Patch, Outer: b.Outer}, {Patch: b.Patch}}
	corners := []geom.Vec3{{Y: .5}, {X: 1, Y: .5}, {X: 1, Y: .75}, {Y: .75}}
	for j, p := range corners {
		c, e := geom.NewBSpline(1, []geom.Vec3{p, corners[(j+1)%4]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		inputs[2].Outer = append(inputs[2].Outer, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	return inputs, []QuadraticTrimStitchSeam{{FaceA: 0, TrimA: 0, FaceB: 1, TrimB: 0}, {FaceA: 1, TrimA: 2, FaceB: 2, TrimB: 0}}
}

func TestMultiSpanTrimCollectionThreeFaceChain(t *testing.T) {
	slowtest.Skip(t)
	inputs, seams := multiSpanCollectionFixture(t)
	o := MultiSpanTrimCollectionOptions{Join: MultiSpanTrimFaceJoinOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .2}}}, CheckDisplayEmbedding: true}
	got, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, seams, o)
	if e != nil {
		t.Fatal(e)
	}
	if !got.DisplayEmbedded || !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("missing exact display qualification")
	}
	if len(got.Seams) != 2 || len(got.Faces) != 3 {
		t.Fatal("missing provenance")
	}
	// Source supports are a graph over the independently known strip
	// [0,1] x [-1/4,1/2]; projected triangles must cover area3/4.
	area := 0.
	edges := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		cross := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
		if cross <= 0 {
			t.Fatal("folded display projection")
		}
		area += cross / 2
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
	}
	if area < .75-1e-12 || area > .75+1e-12 {
		t.Fatal("projected coverage", area)
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 1 {
		t.Fatal("not a disk")
	}
	seen := map[int]bool{}
	for si, ids := range got.SeamVertices {
		for _, id := range ids {
			if seen[id] {
				t.Fatal("undeclared junction")
			}
			seen[id] = true
		}
		for i := 0; i+1 < len(ids); i++ {
			a, b := ids[i], ids[i+1]
			if a > b {
				a, b = b, a
			}
			if edges[[2]int{a, b}] != 2 {
				t.Fatal("seam incidence", si)
			}
		}
	}
	o.CheckDisplayEmbedding = false
	again, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, []QuadraticTrimStitchSeam{seams[1], seams[0]}, o)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Mesh, again.Mesh) || !reflect.DeepEqual(got.OriginalFace, again.OriginalFace) {
		t.Fatal("declaration-order-dependent assembly")
	}
	for fi, f := range got.Faces {
		for _, source := range f.OriginalFace {
			if source != fi {
				t.Fatal("face provenance")
			}
		}
	}
	t.Logf("triangles=%d vertices=%d seam samples=%d/%d BVH visits=%d", len(got.Mesh.Triangles), len(got.Mesh.Vertices), len(got.SeamVertices[0]), len(got.SeamVertices[1]), got.DisplayCollision.BroadPhasePairs)
	o.MaxTriangles = len(got.Mesh.Triangles) - 1
	if r, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, seams, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("aggregate budget", e)
	}
}

func TestMultiSpanTrimCollectionRejectsJunctionAndOverlap(t *testing.T) {
	slowtest.Skip(t)
	inputs, seams := multiSpanCollectionFixture(t)
	o := MultiSpanTrimCollectionOptions{Join: MultiSpanTrimFaceJoinOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .2}}}}
	bad := append([]QuadraticTrimStitchSeam(nil), seams...)
	bad[1].TrimA = 1
	if _, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, bad, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("adjacent unproved junction", e)
	}
	// Add an undeclared duplicate sheet: combinatorially disjoint, but forbidden
	// exact coplanar triangle contact under the optional display check.
	inputs = append(inputs, inputs[0])
	o.CheckDisplayEmbedding = true
	if r, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, seams, o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("overlapping sheets accepted")
	}
	inputs = inputs[:3]
	o.Collision = collision.Options{MaxPairs: 1}
	if r, e := StitchMultiSpanTrimCollectionCertified(context.Background(), inputs, seams, o); !errors.Is(e, collision.ErrLimit) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial display proof", e)
	}
}

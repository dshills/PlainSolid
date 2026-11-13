package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestEmbeddedNativeQuadraticSeamPreservingRefinement(t *testing.T) {
	a, b := mixedNativeJoinFixture(t)
	o := ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .3}}}}
	got, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, o, collision.Options{})
	if e != nil {
		t.Fatal(e)
	}
	independent, e := collision.SelfIntersections(got.Mesh, collision.Options{})
	if e != nil || !independent.Complete || len(independent.Contacts) != 0 {
		t.Fatal("independent sweep contact check", e)
	}
	if !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("no complete embedding certificate")
	}
	area := 0.
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		v := ((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)) / 2
		if v <= 0 {
			t.Fatal("independent XY fold")
		}
		area += v
	}
	if math.Abs(area-1.5) > 1e-12 {
		t.Fatal("independent rectangle coverage")
	}
	if got.Faces[0].Subdivisions != 1 || got.Faces[1].Subdivisions != 1 {
		t.Fatal("boundary subdivided")
	}
	if got.Faces[0].InteriorRefinementPasses+got.Faces[1].InteriorRefinementPasses == 0 {
		t.Fatal("fixture needs actual interior refinement")
	}
	// Shared IDs must correspond only to certified original curve partition
	// vertices: no further dyadic chord interpolation may appear on the seam.
	if len(got.SharedVertices) != len(got.Trims[0][0].NormalizedParameters) {
		t.Fatal("invented boundary samples")
	}
	t.Logf("triangles=%d seam=%d interior=%d/%d BVH visits=%d", len(got.Mesh.Triangles), len(got.SharedVertices), got.Faces[0].InteriorRefinementPasses, got.Faces[1].InteriorRefinementPasses, got.DisplayCollision.BroadPhasePairs)
	if r, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, o, collision.Options{MaxPairs: 1}); !errors.Is(e, collision.ErrLimit) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial embedding acceptance", e)
	}
	limited := o
	limited.Face.Face.Mesh.MaxSubdivisions = 1
	if r, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, limited, collision.Options{}); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial interior acceptance", e)
	}
}

func TestEmbeddedNativeQuadraticRejectsOverlappingSheets(t *testing.T) {
	a, _ := mixedNativeJoinFixture(t)
	a.Patch = partitionedSeamPatch(t, false)
	b := ExactQuadraticFaceJoinInput{Patch: partitionedSeamPatch(t, true), NativeOuter: append([]ExactQuadraticUV(nil), a.NativeOuter...)}
	// Reverse the rational parameter weights while retaining an increasing UV
	// path. The reflected chart then gives the opposite same-model seam, but
	// both source fills cover the SAME physical side of it.
	h := [3][3]*big.Rat{{new(big.Rat), big.NewRat(1, 32), big.NewRat(1, 4)}, {big.NewRat(1, 4), big.NewRat(1, 6), big.NewRat(1, 2)}, {big.NewRat(1, 1), big.NewRat(1, 8), big.NewRat(1, 1)}}
	var e error
	b.NativeOuter[0], e = NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	o := ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .3}}}}
	candidate, err := stitchExactQuadraticFaces(context.Background(), a, b, o, true)
	if err != nil {
		t.Fatal("overlap must reach geometry gate", err)
	}
	contacts, err := collision.SelfIntersections(candidate.Mesh, collision.Options{})
	if err != nil || !contacts.Complete || len(contacts.Contacts) == 0 {
		t.Fatal("independent overlapping sheet contact", err)
	}
	r, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, o, collision.Options{})
	if !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("overlapping sheets accepted", e)
	}
}

func TestEmbeddedNativeTriangleCentroidRefinement(t *testing.T) {
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			z := u * v
			if i == 2 {
				z -= v
			}
			if j == 2 {
				z -= u
			}
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	surface, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range poles {
		poles[i].Z = 0
	}
	flat, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	plane, e := geom.NewSurfacePatch(flat, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	makeFace := func(patch *geom.SurfacePatch, points []geom.Vec3, shared int) ExactQuadraticFaceJoinInput {
		f := ExactQuadraticFaceJoinInput{Patch: patch, SharedTrim: shared}
		for i, a := range points {
			f.NativeOuter = append(f.NativeOuter, exactNativeLine(t, a, points[(i+1)%len(points)]))
		}
		return f
	}
	// The curved face z=u*v*(1-u-v) vanishes on all three boundary edges
	// but has a genuine interior bump. This exercises the centroid fallback
	// for a seed triangle with no splittable boundary edge.
	a := makeFace(p, []geom.Vec3{{}, {X: 1}, {Y: 1}}, 1)
	b := makeFace(plane, []geom.Vec3{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, 2)
	o := ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}}
	got, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, o, collision.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Faces[0].InteriorRefinementPasses == 0 {
		t.Fatal("all-boundary seed not refined")
	}
	if len(got.SharedVertices) != 2 || len(got.Faces[0].BoundaryLoops[0]) != 3 || len(got.Faces[1].BoundaryLoops[0]) != 3 {
		t.Fatal("boundary changed")
	}
	if !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("centroid embedding")
	}
	o.Face.Face.Mesh.Deflection = .001
	o.Face.Face.Mesh.MaxSubdivisions = 2
	if r, e := StitchEmbeddedExactQuadraticFacesCertified(context.Background(), a, b, o, collision.Options{}); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("uncertifiable refinement accepted", e)
	}
}

package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestShellDisplayBoundaryObliqueContacts(t *testing.T) {
	// Both segments lie in the oblique plane z=x+y. All endpoints differ;
	// checking only duplicate model/display vertices would miss the crossing.
	base := []geom.Vec3{{X: -1, Y: -1, Z: -2}, {X: 1, Y: 1, Z: 2}, {X: -1, Y: 1}, {X: 1, Y: -1}}
	edges := [][]int{{0, 1}, {2, 3}}
	if _, e := checkBoundaryPolylineEmbedding(context.Background(), base, edges, 1); e == nil {
		t.Fatal("oblique crossing accepted")
	}
	// Lift one line by one smallest positive binary64 Z increment. It is now
	// skew: exact predicates must not invent a contact from numerical proximity.
	base[2].Z = math.SmallestNonzeroFloat64
	base[3].Z = math.SmallestNonzeroFloat64
	if pairs, e := checkBoundaryPolylineEmbedding(context.Background(), base, edges, 1); e != nil || pairs != 1 {
		t.Fatal("exact skew separation", pairs, e)
	}
	// A nonindexed endpoint in the interior of another line is still forbidden.
	points := []geom.Vec3{{}, {X: 2, Y: 2, Z: 2}, {X: 1, Y: 1, Z: 1}, {X: 1, Y: 2, Z: 3}}
	if _, e := checkBoundaryPolylineEmbedding(context.Background(), points, edges, 1); e == nil {
		t.Fatal("T contact accepted")
	}
	// Indexed endpoint ownership permits a fan of display lines, but not
	// overlapping collinear rays even when those rays share the endpoint ID.
	points = []geom.Vec3{{}, {X: 1, Y: 2, Z: 3}, {X: -2, Y: 1, Z: 4}}
	if _, e := checkBoundaryPolylineEmbedding(context.Background(), points, [][]int{{0, 1}, {0, 2}}, 1); e != nil {
		t.Fatal(e)
	}
	points[2] = geom.Vec3{X: 2, Y: 4, Z: 6}
	if _, e := checkBoundaryPolylineEmbedding(context.Background(), points, [][]int{{0, 1}, {0, 2}}, 1); e == nil {
		t.Fatal("collinear shared overlap accepted")
	}
	var empty AlgebraicShellBoolean
	for _, limit := range []int{-1, 16000001} {
		r, e := empty.TessellateEmbeddedBoundaryCertified(context.Background(), AlgebraicShellBoundaryEmbeddingOptions{MaxPairs: limit})
		if !errors.Is(e, geom.ErrInvalid) || len(r.Boundary.Vertices) != 0 {
			t.Fatal(e)
		}
	}
}

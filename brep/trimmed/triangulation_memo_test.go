package trimmed

import (
	"context"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

// TestUVTriangulationMemo holds the memory of chart triangulations to what
// a caller can see of it: a triangulation asked for again is the one that
// was made, what a caller does to the triangles it was given does not
// reach the next caller, and a different limit is a different question.
// It also holds the memory to its bound.
func TestUVTriangulationMemo(t *testing.T) {
	square := func(at, side float64) []geom.Vec3 {
		return []geom.Vec3{{X: at, Y: at}, {X: at + side, Y: at}, {X: at + side, Y: at + side}, {X: at, Y: at + side}}
	}
	hole := []geom.Vec3{{X: 2, Y: 2}, {X: 2, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 2}}
	rings := [][]geom.Vec3{square(0, 6), hole}
	first, err := triangulateUVBounded(context.Background(), rings, brep.MaxHoledFaceVertices)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := triangulateUVWork(context.Background(), rings, brep.MaxHoledFaceVertices)
	if err != nil || !reflect.DeepEqual(first, fresh) {
		t.Fatalf("the remembered triangulation differs from one made now: %v", err)
	}
	kept := append([][3]geom.Vec2(nil), first...)
	first[0][0].X = 99
	again, err := triangulateUVBounded(context.Background(), rings, brep.MaxHoledFaceVertices)
	if err != nil || !reflect.DeepEqual(again, kept) {
		t.Fatalf("a change to a triangulation that was handed out reached the memory: %v", err)
	}
	if uvTriangulations.key(rings, 300) == uvTriangulations.key(rings, brep.MaxHoledFaceVertices) {
		t.Fatal("two limits have one key")
	}
	for i := 0; uvTriangulations.vertices <= uvMemoVertices; i++ {
		if _, err := triangulateUVBounded(context.Background(), [][]geom.Vec3{square(float64(i), 6), {{X: float64(i) + 2, Y: float64(i) + 2}, {X: float64(i) + 2, Y: float64(i) + 3}, {X: float64(i) + 3, Y: float64(i) + 3}, {X: float64(i) + 3, Y: float64(i) + 2}}}, brep.MaxHoledFaceVertices); err != nil {
			t.Fatal(err)
		}
		if i > uvMemoVertices {
			t.Fatal("the memory does not fill")
		}
	}
	if _, err := triangulateUVBounded(context.Background(), [][]geom.Vec3{square(-9, 2)}, brep.MaxHoledFaceVertices); err != nil {
		t.Fatal(err)
	}
	if len(uvTriangulations.made) != 1 || uvTriangulations.vertices != 4 {
		t.Fatalf("the full memory was not emptied: %d entries, %d vertices", len(uvTriangulations.made), uvTriangulations.vertices)
	}
}

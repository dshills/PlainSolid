package boolean

import (
	"reflect"
	"testing"
)

// TestSources covers the face source list of P5: no duplicates, A before
// B, then by index, with both faces of a shared piece.
func TestSources(t *testing.T) {
	got := sourcesOf([]piece{
		{op: 1, face: 4, mate: -1},
		{op: 0, face: 3, mate: 2},
		{op: 0, face: 1, mate: -1},
		{op: 0, face: 3, mate: -1},
		{op: 1, face: 2, mate: 3},
	})
	if want := [][2]int{{0, 1}, {0, 3}, {1, 2}, {1, 4}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

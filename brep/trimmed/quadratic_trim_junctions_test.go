package trimmed

import (
	"plainsolid/mesh"
	"testing"
)

func TestQuadraticJunctionFanCyclesAndDisconnectedLinks(t *testing.T) {
	// Link topology is independent of displayed vertex positions. The full
	// stitcher separately checks finite triangles and exact model identities.
	for _, fixture := range []struct {
		triangles [][3]int
		valid     bool
	}{
		{[][3]int{{0, 1, 2}, {0, 2, 3}, {0, 3, 1}}, true},
		{[][3]int{{0, 1, 2}, {0, 2, 3}}, true},
		{[][3]int{{0, 1, 2}, {0, 3, 4}}, false},
		{[][3]int{{0, 1, 2}, {0, 2, 1}}, false},
		{[][3]int{{0, 1, 2}, {0, 2, 3}, {0, 2, 4}}, false},
	} {
		e := validateQuadraticJunctionFans(mesh.Mesh{Triangles: fixture.triangles}, []int{0})
		if (e == nil) != fixture.valid {
			t.Fatal("incorrect path/cycle fan classification", fixture.triangles, e)
		}
	}
}

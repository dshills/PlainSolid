package collision

import (
	"fmt"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"testing"
)

func TestExactTriangleMemoBoundAndImmutability(t *testing.T) {
	m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: 1, Z: 1}, {X: 3}, {X: 3, Y: 2}}}
	tris := [][3]int{{0, 1, 2}, {3, 4, 5}, {1, 6, 7}, {0, 2, 4}}
	for i := 0; i < exactTriangleMemoLimit+2; i++ {
		m.Triangles = append(m.Triangles, tris[i%len(tris)])
	}
	memo := newExactTriangleMemo(m)
	if len(memo.values) != exactTriangleMemoLimit {
		t.Fatal(len(memo.values))
	}
	for _, i := range []int{0, 1, 2, 3, exactTriangleMemoLimit, exactTriangleMemoLimit + 1} {
		for _, j := range []int{0, 1, 2, 3} {
			a, b := memo.get(i), memo.get(j)
			before := fmt.Sprint(a, b)
			k, p := preparedTriangleIntersection(a, b)
			want, points := triangleIntersection(triangle(m, i), triangle(m, j))
			if k != want || !reflect.DeepEqual(p, points) {
				t.Fatal(i, j, k, want)
			}
			if fmt.Sprint(a, b) != before {
				t.Fatal("cached exact numbers mutated")
			}
		}
	}
	for i, v := range memo.values {
		if i > 3 && v != nil {
			t.Fatal("unvisited triangle eagerly cached", i)
		}
	}
}

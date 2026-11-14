package plaincadbridge

import (
	"math"
	"testing"
)

func TestBlindPocketRetainsAllOuterCapTreatments(t *testing.T) {
	for _, kind := range []string{"fillet", "chamfer"} {
		for i, pair := range [][2]vector{{{0, 0, 10}, {40, 0, 10}}, {{40, 0, 10}, {40, 30, 10}}, {{40, 30, 10}, {0, 30, 10}}, {{0, 30, 10}, {0, 0, 10}}} {
			t.Run(kind+string(rune('0'+i)), func(t *testing.T) {
				b := New()
				stock := box(t, b, vector{}, vector{40, 30, 10})
				tool := box(t, b, vector{15, 10, 8}, vector{25, 20, 10})
				pocket := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
				if !b.shapes[pocket.Handle].edgeAvailable(pair[0].vec(), pair[1].vec()) {
					t.Fatal("edge unavailable")
				}
				key := "radius"
				section := 1 - math.Pi/4
				if kind == "chamfer" {
					key = "distance"
					section = .5
				}
				out := shape(t, b, map[string]any{"op": kind, "handle": pocket.Handle, "edgeStart": pair[0], "edgeEnd": pair[1], "normal": vector{0, 0, 1}, key: 1})
				near(t, out.GeometryAssertions.Volume, 11800-pair[1].vec().Sub(pair[0].vec()).Length()*section)
				verifyRetainedPlanarBore(t, b, out)
			})
		}
	}
}

func TestBlindPocketTreatmentRejectsPocketWallIntersection(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{40, 30, 10})
	tool := box(t, b, vector{15, .5, 8}, vector{25, 10.5, 10})
	pocket := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	start, end, n := vector{0, 0, 10}, vector{40, 0, 10}, vector{0, 0, 1}
	for _, kind := range []string{"fillet", "chamfer"} {
		count, next := len(b.shapes), b.next
		_, e := b.treatEdge(b.shapes[pocket.Handle], request{Op: kind, Normal: &n, EdgeStart: &start, EdgeEnd: &end, Radius: 1, Distance: 1})
		if e == nil || len(b.shapes) != count || b.next != next {
			t.Fatal("accepted intersecting treatment or allocated failed result", kind, e)
		}
	}
}

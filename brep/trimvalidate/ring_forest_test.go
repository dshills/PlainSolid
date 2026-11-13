package trimvalidate

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestClassifyRingForest(t *testing.T) {
	box := func(x, y, size float64, cw bool) Loop {
		p := [4]geom.Vec2{{X: x, Y: y}, {X: x + size, Y: y}, {X: x + size, Y: y + size}, {X: x, Y: y + size}}
		if cw {
			p[1], p[3] = p[3], p[1]
		}
		ring := make(Loop, len(p))
		for i, a := range p {
			ring[i] = Segment{Start: a, End: p[(i+1)%len(p)]}
		}
		return ring
	}
	ctx := context.Background()
	// Input order gives no hint about enclosure or orientation.
	rings := []Loop{box(3, 3, 2, false), box(12, 0, 1, true), box(2, 2, 6, true), box(0, 0, 10, false)}
	want := []RingPlacement{{2, 2, 1}, {-1, 0, -1}, {3, 1, -1}, {-1, 0, 1}}
	got, proof, e := ClassifyRingForest(ctx, rings, Options{})
	if e != nil || proof.Status != Certified || len(got) != len(want) {
		t.Fatal(got, proof, e)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ring %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if bad, proof, e := ClassifyRingForest(ctx, rings, Options{MaxWork: 1}); e != nil || proof.Status != Unresolved || bad != nil {
		t.Fatal("work exhausted after exposing a partial forest", bad, proof, e)
	}
	rings[1] = box(10, 0, 1, true)
	if bad, proof, e := ClassifyRingForest(ctx, rings, Options{}); e != nil || proof.Status != Invalid || bad != nil {
		t.Fatal("contacting rings admitted", bad, proof, e)
	}
}

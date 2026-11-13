package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestBoundedCircularAnchorDoesNotSnap(t *testing.T) {
	c := EllipseUV{Center: geom.Vec2{X: 5, Y: 7}, Cos: geom.Vec2{X: 3}, Sin: geom.Vec2{Y: 3}, From: .37, To: .9}
	start, _ := c.EvalUV(0)
	end, _ := c.EvalUV(1)
	authored := start
	authored.X = math.Nextafter(authored.X, math.Inf(1))
	a, err := newBoundedAnchoredCircularUV(c, authored, end)
	if err != nil {
		t.Fatal(err)
	}
	at, _ := a.EvalUV(0)
	if at != authored || at == start {
		t.Fatal("silently snapped endpoint", at)
	}
	mid, _ := a.EvalUV(.5)
	want, _ := c.EvalUV(.5)
	if mid != want {
		t.Fatal("replaced interior curve")
	}
	// Independent large perturbation exceeds the finite ULP allowance.
	bad := authored
	bad.X += 1e-6
	if _, err := newBoundedAnchoredCircularUV(c, bad, end); err == nil {
		t.Fatal("accepted displaced endpoint")
	}
}
func TestArcLoopEndpointToleranceAndReload(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	arc := ExtrusionSegment{Center: geom.Vec2{X: 5, Y: 7}, Radius: 3, StartAngle: .37, Sweep: .93}
	arc.Start = arcLoopPoint(arc, arc.StartAngle)
	arc.End = arcLoopPoint(arc, arc.StartAngle+arc.Sweep)
	arc.Start.X = math.Nextafter(arc.Start.X, math.Inf(1))
	loop := []ExtrusionSegment{arc, {Start: arc.End, End: arc.Start}}
	r, err := ExtrudeArcLoop(context.Background(), frame, loop, 2, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	before := r.Solid.Vertices()
	var encoded bytes.Buffer
	if err := WriteJSON(&encoded, r.Solid); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadJSON(&encoded, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	after := restored.Vertices()
	if len(before) != len(after) {
		t.Fatal("vertex count")
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("reload snapped vertex", i)
		}
	}
	// Preserve closure while exceeding the independent authored-endpoint allowance.
	loop[0].Start.X += 1e-6
	loop[1].End = loop[0].Start
	if out, err := ExtrudeArcLoop(context.Background(), frame, loop, 2, 1e-9); err == nil || len(out.Solid.Faces()) != 0 {
		t.Fatal("invalid perturbation published")
	}
	// A tolerance larger than the perturbation does not override the ULP gate.
	if _, err := ExtrudeArcLoop(context.Background(), frame, loop, 2, 1e-3); err == nil {
		t.Fatal("tolerance bypassed ULP gate")
	}
}

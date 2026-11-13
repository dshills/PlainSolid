package plaincadbridge

import (
	"context"
	"encoding/json"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestNativeDisplayEdgesCylinder(t *testing.T) {
	b := New()
	shape, e := b.circle(geom.Vec3{}, geom.Vec3{Z: 1}, 5, 10)
	if e != nil {
		t.Fatal(e)
	}
	var out displayEdges
	data := invoke(t, b, map[string]any{"op": "displayEdges", "handle": shape.Handle, "maxSegments": 1000, "edgeUses": 100})
	if e = json.Unmarshal(data, &out); e != nil {
		t.Fatal(e)
	}
	if len(out.Positions)/6 != 128 || out.EdgeUses <= 0 {
		t.Fatal(len(out.Positions), out.EdgeUses)
	}
	levels := map[float64]bool{}
	for i := 0; i < len(out.Positions); i += 6 {
		p := out.Positions[i : i+6]
		if p[2] != p[5] || math.Abs(math.Hypot(p[0], p[1])-5) > 1e-10 || math.Abs(math.Hypot(p[3], p[4])-5) > 1e-10 {
			t.Fatal(p)
		}
		levels[p[2]] = true
	}
	if len(levels) != 2 || !levels[0] || !levels[10] {
		t.Fatal(levels)
	}
	for _, q := range []string{`{"op":"displayEdges","handle":"` + shape.Handle + `","maxSegments":2}`, `{"op":"displayEdges","handle":"` + shape.Handle + `","maxSegments":1000,"edgeUses":0}`} {
		r := b.Call(q)
		if !strings.Contains(r, "budget") || strings.Contains(r, `"ok":true`) {
			t.Fatal(r)
		}
	}
}
func TestNativeDisplayEdgesCapsuleAndBox(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1})
	body, e := trimmed.ExtrudeCapsule(context.Background(), frame, geom.Vec2{X: -3}, geom.Vec2{X: 3}, 1, 5)
	if e != nil {
		t.Fatal(e)
	}
	out, e := nativeDisplayEdges(stored{analytic: &analyticBody{solid: body}}, 1000, nil)
	if e != nil || len(out.Positions) == 0 {
		t.Fatal(out, e)
	}
	for i := 0; i < len(out.Positions); i += 6 {
		if out.Positions[i+2] != out.Positions[i+5] || (out.Positions[i+2] != 4 && out.Positions[i+2] != 9) {
			t.Fatal("smooth vertical join retained", out.Positions[i:i+6])
		}
	}
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	out, e = nativeDisplayEdges(stored{solid: box}, 12, nil)
	if e != nil || len(out.Positions) != 72 || out.EdgeUses != 24 {
		t.Fatal(out, e)
	}
}

func TestNativeDisplayEdgesRoundedCylinder(t *testing.T) {
	for _, axis := range []vector{{0, 0, 1}, {1, 0, 0}, {0, -1, 0}} {
		b := New()
		source := circle(t, b, vector{}, axis, 5, 10)
		rounded := shape(t, b, map[string]any{"op": "filletCylinderRims", "handle": source.Handle, "normal": axis, "rimCenters": []vector{coordinates(axis.vec().Scale(10))}, "radius": 1})
		var out displayEdges
		if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "displayEdges", "handle": rounded.Handle, "maxSegments": 1000}), &out); e != nil {
			t.Fatal(e)
		}
		if len(out.Positions)/6 != 64 {
			t.Fatal("expected untouched rim only", len(out.Positions)/6)
		}
		for i := 0; i < len(out.Positions); i += 3 {
			p := geom.Vec3{X: out.Positions[i], Y: out.Positions[i+1], Z: out.Positions[i+2]}
			if math.Abs(p.Dot(axis.vec())) > 1e-12 || math.Abs(p.Length()-5) > 1e-10 {
				t.Fatal("fillet tangent boundary or seam retained", p)
			}
		}
	}
}

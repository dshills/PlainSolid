package plaincadbridge

import (
	"encoding/base64"
	"math"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

func TestSTEPIndependentlyAuthoredRoundedBlindAndThroughBores(t *testing.T) {
	data, err := os.ReadFile("../../exchange/step/testdata/rounded-pocket-occt.step")
	if err != nil {
		t.Fatal(err)
	}
	b := New()
	stock, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	p := b.shapes[stock.Handle].analytic.stepRounded
	if p == nil || len(p.Holes) != 1 || len(p.Blind) != 1 || math.Abs(p.Height) != 3 || p.Blind[0].Depth != 1 {
		t.Fatal("independent STEP hole recipes were not preserved")
	}
	moved := shape(t, b, map[string]any{"op": "transform", "handle": stock.Handle,
		"matrix": geom.Affine{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	out, err := b.exportSTEP([]string{stock.Handle, moved.Handle})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.After) != 2 {
		t.Fatal("expected two independently placed bodies")
	}
	for i, src := range []Shape{stock, moved} {
		for j := range src.Bounds.Min {
			near(t, out.After[i].Bounds.Min[j], src.Bounds.Min[j])
			near(t, out.After[i].Bounds.Max[j], src.Bounds.Max[j])
		}
		near(t, out.After[i].Volume, src.GeometryAssertions.Volume)
		near(t, out.After[i].SurfaceArea, src.GeometryAssertions.SurfaceArea)
	}
}

func TestSTEPBridgeLongEdgeTreatmentAndFullChain(t *testing.T) {
	b := New()
	f, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	body := edgeBody{frame: f, length: 20, width: 10, height: 10,
		corners: [4]fillet.LongEdgeCorner{2: {YSetback: 1, ZSetback: 1}, 3: {Radius: 1}}}
	for _, full := range []bool{false, true} {
		if full {
			body.bore = &trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
			body.pocket = &trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}
			body.boss = &trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}
		}
		original, err := rebuildEdge(body)
		if err != nil {
			t.Fatal(err)
		}
		input, err := b.putEdge(original)
		if err != nil {
			t.Fatal(err)
		}
		out, err := b.exportSTEP([]string{input.Handle})
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(out.Base64)
		if err != nil {
			t.Fatal(err)
		}
		readback, err := b.importSTEP(string(data))
		if err != nil {
			t.Fatal(err)
		}
		if b.shapes[readback.Handle].treated == nil || readback.Bounds != input.Bounds {
			t.Fatal("machined stock STEP lost its native body")
		}
		near(t, readback.GeometryAssertions.Volume, input.GeometryAssertions.Volume)
		near(t, readback.GeometryAssertions.SurfaceArea, input.GeometryAssertions.SurfaceArea)
	}
}

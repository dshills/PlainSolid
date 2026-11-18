package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAttachSteppedGroovedShaftHead(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{Z: 5}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	tip, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := SteppedShaft{Frame: frame, ShaftRadius: 4, ShaftLength: 5, HeadRadius: 6, HeadHeight: 2, Socket: []geom.Vec2{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}}, SocketFloor: 1}
	for _, hand := range []geom.HelixHandedness{geom.LeftHanded, geom.RightHanded} {
		r := HelicalGrooveRecipe{Frame: tip, OuterRadius: 4, Height: 5, Profile: []geom.Vec2{{X: 3.5, Y: 1.125}, {X: 4, Y: 1}, {X: 4, Y: 1.25}}, Lead: 1, Start: .25, End: 2.25, Hand: hand, Starts: 2}
		groove, e := GrooveShaftWithTipRelief(ctx, r, 3, .5)
		if e != nil {
			t.Fatal(e)
		}
		var original bytes.Buffer
		if e = WriteJSON(&original, groove.Solid); e != nil {
			t.Fatal(e)
		}
		joined, e := AttachSteppedShaftHead(ctx, p, groove.Solid)
		if e != nil {
			t.Fatal(hand, e)
		}
		before, e := groove.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		after, e := joined.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16000000})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(after.Volume-before.Volume-(72*math.Pi-4)) > 1e-6 {
			t.Fatal(before.Volume, after.Volume)
		}
		m, e := joined.TessellateHelical(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var persisted, unchanged bytes.Buffer
		if e = WriteJSON(&persisted, joined); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadJSON(&persisted, JSONLimits{}); e != nil {
			t.Fatal(e)
		}
		if e = WriteJSON(&unchanged, groove.Solid); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(original.Bytes(), unchanged.Bytes()) {
			t.Fatal("input groove mutated")
		}
		bad := p
		bad.ShaftLength = 4
		if _, e = AttachSteppedShaftHead(ctx, bad, groove.Solid); e == nil {
			t.Fatal("wrong interface accepted")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e = AttachSteppedShaftHead(canceled, p, groove.Solid); e == nil {
			t.Fatal("ignored cancellation")
		}

	}
}

package plaincadbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

func TestTwinPulleyCoaxialAnnularJoin(t *testing.T) {
	// Both original pulley occurrences in 08-belt-drive.pcaddoc.
	// The rim starts at z=16. The hub starts at pulley_height+15
	// and is 6 mm tall, so exactly 5 mm remain outside the rim.
	for _, x := range []float64{-40, 40} {
		for _, height := range []float64{10, 12} {
			t.Run(fmt.Sprintf("%g/%g", x, height), func(t *testing.T) {
				b := New()
				rim := twinPulleyAnnulus(t, b, x, 16, 24, 8, height, 1)
				hub := twinPulleyAnnulus(t, b, x, height+15, 12, 8, 6, 1)
				out := shape(t, b, map[string]any{"op": "fuse", "a": rim.Handle, "b": hub.Handle})
				volume := (512*height + 80*5) * math.Pi
				area := (1024 + 64*height + 40*5) * math.Pi
				assert := out.GeometryAssertions
				if !assert.Valid || assert.SolidCount != 1 || math.Abs(assert.Volume-volume) > 1e-10 ||
					math.Abs(assert.SurfaceArea-area) > 1e-10 ||
					out.Bounds.Min != (vector{x - 24, -24, 16}) || out.Bounds.Max != (vector{x + 24, 24, 21 + height}) {
					t.Fatal("changed original stepped pulley geometry", out)
				}
				body := b.shapes[out.Handle].analytic
				if body == nil || body.coaxial == nil || body.coaxial.inner != 8 || len(body.coaxial.spans) != 2 {
					t.Fatal("missing exact annular stock")
				}
				topo := body.solid.Topology()
				if len(topo.Shells) != 1 || topo.Faces != 6 || topo.EulerCharacteristic != 0 || topo.CollapsedEdges != 0 {
					t.Fatal("wrong closed annular topology", topo)
				}
				checkTwinPulleyBore(t, body, x, height)
				props, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if err != nil || math.Abs(props.Volume-volume) > 1e-8 || math.Abs(props.Area-area) > 1e-8 {
					t.Fatal("analytic boundary changed independent material measures", props, err)
				}
				preview, err := body.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100_000})
				if err != nil || preview.ValidateSolid() != nil || preview.SignedVolume() <= 0 || math.Abs(preview.SignedVolume()-volume) > volume*.01 {
					t.Fatal("preview changed annular material or winding", preview.SignedVolume(), err)
				}
				if rim.GeometryAssertions.Volume != b.shapes[rim.Handle].result.GeometryAssertions.Volume ||
					hub.GeometryAssertions.Volume != b.shapes[hub.Handle].result.GeometryAssertions.Volume {
					t.Fatal("mutated an original operand")
				}
			})
		}
	}
}

func checkTwinPulleyBore(t *testing.T, body *analyticBody, x, height float64) {
	t.Helper()
	// The single original bore generator is unbroken across the step.
	// The two other axial generators retain their authored outside radii.
	generators := map[float64][2]float64{8: {16, height + 21}, 12: {height + 16, height + 21}, 24: {16, height + 16}}
	for _, face := range body.solid.Faces() {
		first, err := face.Surface.Eval(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		last, err := face.Surface.Eval(0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if first.Z == last.Z {
			continue
		}
		radius := math.Hypot(first.X-x, first.Y)
		if want, ok := generators[radius]; !ok || first.X != last.X || first.Y != last.Y ||
			[2]float64{min(first.Z, last.Z), max(first.Z, last.Z)} != want {
			t.Fatal("changed source bore or outside radial generator", first, last)
		}
		delete(generators, radius)
	}
	if len(generators) != 0 {
		t.Fatal("missing a source axial generator", generators)
	}
}

func twinPulleyAnnulus(t *testing.T, b *Bridge, x, z, outer, inner, height, axis float64) Shape {
	t.Helper()
	return shape(t, b, map[string]any{
		"op": "mixedExtrude",
		"profile": map[string]any{
			"kind": "disk", "origin": vector{0, 0, z}, "normal": vector{0, 0, axis},
			"xDirection": vector{1, 0, 0}, "center": []float64{x, 0}, "radius": outer, "height": height,
			"circles": []any{map[string]any{"center": []float64{x, 0}, "radius": inner}},
		},
	})
}

func TestCoaxialAnnularJoinAtomicAndDisplayBudget(t *testing.T) {
	b := New()
	rim := twinPulleyAnnulus(t, b, 40, 16, 24, 8, 10, 1)
	hub := twinPulleyAnnulus(t, b, 40, 31, 12, 8, 6, -1)
	out := shape(t, b, map[string]any{"op": "fuse", "a": hub.Handle, "b": rim.Handle})
	body := b.shapes[out.Handle].analytic.coaxial
	if body.inner != 8 || out.Bounds.Min != (vector{16, -24, 16}) || out.Bounds.Max != (vector{64, 24, 31}) ||
		math.Abs(out.GeometryAssertions.Volume-5520*math.Pi) > 1e-10 {
		t.Fatal("opposed original axis changed the annular material", out)
	}
	opts := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128}
	m, err := body.tessellate(opts)
	if err != nil || len(m.Triangles) == 0 {
		t.Fatal(err)
	}
	opts.MaxTriangles = len(m.Triangles) - 1
	m, err = body.tessellate(opts)
	if !errors.Is(err, parametric.ErrBudget) || len(m.Triangles) != 0 {
		t.Fatal("published an incomplete bore or cap under a short triangle budget", err)
	}
	for _, other := range []Shape{
		twinPulleyAnnulus(t, b, 40, 25, 12, 7, 6, 1),
		twinPulleyAnnulus(t, b, 40, 27, 12, 8, 6, 1),
		twinPulleyAnnulus(t, b, -40, 25, 12, 8, 6, 1),
	} {
		prev, next := len(b.shapes), b.next
		raw := b.Call(fmt.Sprintf(`{"op":"fuse","a":%q,"b":%q}`, rim.Handle, other.Handle))
		var reply struct {
			OK bool
		}
		if err := json.Unmarshal([]byte(raw), &reply); err != nil || reply.OK || len(b.shapes) != prev || b.next != next {
			t.Fatal("unsupported annular union must refuse atomically", raw, err)
		}
	}
}

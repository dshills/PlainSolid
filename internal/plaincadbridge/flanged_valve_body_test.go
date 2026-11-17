package plaincadbridge

import (
	"context"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

func TestFlangedValveBodyJoinAndThroughPattern(t *testing.T) {
	for _, height := range []float64{32, 36} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			b := New()
			flange := valveAnnulus(t, b, 0, 46, 16, 8)
			chamfered := shape(t, b, map[string]any{"op": "capChamfer", "handle": flange.Handle, "distance": .6,
				"origin": vector{}, "normal": vector{0, 0, -1}})
			wall := valveAnnulus(t, b, 7, 34, 16, height-7)
			a, c := chamfered.Handle, wall.Handle
			if height == 36 {
				a, c = c, a // A Fuse keeps the same source caps in either order.
			}
			joined := shape(t, b, map[string]any{"op": "fuse", "a": a, "b": c})
			want := math.Pi * ((46*46-16*16)*8 - (46+16)*.6*.6 + (34*34-16*16)*(height-8))
			near(t, joined.GeometryAssertions.Volume, want)
			probe := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 16, height)
			_, removed, handled, err := cutThroughProfiles(b.shapes[joined.Handle], []stored{b.shapes[probe.Handle]})
			if !handled || err == nil || removed != 0 {
				t.Fatal("treated the continuous original bore as new cut material", handled, removed, err)
			}
			tools := make([]string, 0, len(valveBoltCenters))
			for _, xy := range valveBoltCenters {
				tool := circle(t, b, vector{xy[0], xy[1], 0}, vector{0, 0, 1}, 3.2, height+1e-7)
				volume, err := b.overlapVolume(joined.Handle, tool.Handle)
				if err != nil {
					t.Fatal(err)
				}
				near(t, volume.(map[string]float64)["volume"], math.Pi*3.2*3.2*8)
				tools = append(tools, tool.Handle)
			}
			drilled := shape(t, b, map[string]any{"op": "cutMany", "a": joined.Handle, "tools": tools})
			want -= math.Pi * 4 * 3.2 * 3.2 * 8
			wantArea := 2 * math.Pi * ((46+16)*8 + 46*46 - 16*16 + (46+16)*(math.Sqrt2-2)*.6 +
				(34+16)*(height-8) + 4*(3.2*8-3.2*3.2))
			near(t, drilled.GeometryAssertions.SurfaceArea, wantArea)
			near(t, drilled.GeometryAssertions.Volume, want)
			body := b.shapes[drilled.Handle].analytic
			caps := body.solid.Faces()
			for _, id := range []int{body.annular.lowerFace, body.annular.upperFace} {
				if len(caps[id].Holes) != 5 {
					t.Fatal("original flange cap lost a bolt or center wire", id, len(caps[id].Holes))
				}
			}
			integral, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 8000000})
			if err != nil || math.Abs(integral.Volume/want-1) > 1e-6 || math.Abs(integral.Area/wantArea-1) > 1e-6 {
				t.Fatal("source topology mass", integral.Volume, integral.Area, want, wantArea, err)
			}
			m, err := b.shapes[drilled.Handle].tessellate(request{})
			if err != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()/want-1) > .02 {
				t.Fatal("joined flange display lost positive closed material", m.SignedVolume(), want, err)
			}
		})
	}
}

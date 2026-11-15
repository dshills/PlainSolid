package plaincadbridge

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRationalSTEPBoreClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw := stepRepairFixture(t, "occt-both-default-tilted-oblique-prism-fillet-through-bore")
	source, err := step.InspectRationalPosedConvexArcThroughBore(ctx, strings.NewReader(raw), step.Limits{}, step.ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	var canonical bytes.Buffer
	if err := source.WriteCanonicalSTEP(ctx, &canonical); err != nil {
		t.Fatal(err)
	}
	strict, err := step.ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, bytes.NewReader(canonical.Bytes()), step.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	profile := stepArcLoopSupport{descriptor: strict.Exterior, exactPlacement: true}
	o := parametric.TessellationOptions{Deflection: .25, Angle: .1, MaxTriangles: 4000}
	display, qualified, err := profile.tessellateWithHole(ctx, o, &strict.Hole)
	if err != nil || !qualified {
		t.Fatal("curved cap through-hole display failed", err)
	}
	if len(display.Triangles) == 0 || len(display.Vertices)-len(display.Triangles)/2 != 0 {
		t.Fatal("closed tessellation lost the through-hole topology")
	}
	for _, tri := range display.Triangles {
		a, b, c := display.Vertices[tri[0]], display.Vertices[tri[1]], display.Vertices[tri[2]]
		if a.Z != b.Z || b.Z != c.Z {
			continue
		}
		corners := [3]geom.Vec2{{X: a.X, Y: a.Y}, {X: b.X, Y: b.Y}, {X: c.X, Y: c.Y}}
		var signs [3]int
		for j, q := range corners {
			signs[j], err = geom.Orient2D(q, corners[(j+1)%3], strict.Hole.Center)
			if err != nil {
				t.Fatal(err)
			}
		}
		if signs[0] >= 0 && signs[1] >= 0 && signs[2] >= 0 || signs[0] <= 0 && signs[1] <= 0 && signs[2] <= 0 {
			t.Fatal("a display cap triangle covers the actual bore center", tri)
		}
	}
	world, err := strict.Pose.RoundMesh(ctx, display, rigidplacement.MeshRoundingOptions{MaxTriangles: o.MaxTriangles})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := rigidplacement.PrepareClosedDisplay(ctx, world.Mesh, nil, rigidplacement.ClosedDisplayOptions{})
	if err != nil || len(prepared.Shells()) != 1 || prepared.Shells()[0].Depth != 0 {
		t.Fatal("tilted world display lost its embedded oriented shell", err)
	}
	mass, err := world.Mesh.MassProperties()
	if err != nil || math.Abs(mass.Volume/28.096231102869041-1) > .001 {
		t.Fatal("tilted world display changed the bore volume", mass.Volume, err)
	}
	o.MaxTriangles = 40
	if m, _, err := profile.tessellateWithHole(ctx, o, &strict.Hole); err == nil || len(m.Triangles) != 0 {
		t.Fatal("hole display exceeded the caller budget", err)
	}
}

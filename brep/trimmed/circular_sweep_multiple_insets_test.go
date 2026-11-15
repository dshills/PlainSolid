package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func multipleSweepInsetsFixture() (ExactCircularSweep, []ExactCircularSweep) {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	host := ExactCircularSweep{
		Outer: exactLoopPolygon(p(0, 0), p(12, 0), p(12, 8), p(0, 8)),
		Holes: [][]ExactCircularLoopSegment{exactLoopQuarterDisk(p(1, 1), .5, true)}, Bottom: -2, Top: 0,
	}
	return host, []ExactCircularSweep{
		{Outer: exactLoopQuarterDisk(p(3, 4), 1.5, false), Holes: [][]ExactCircularLoopSegment{exactLoopQuarterDisk(p(3, 4), .5, true)}, Bottom: 0, Top: 3},
		{Outer: exactLoopQuarterDisk(p(8, 5), 2, false), Holes: [][]ExactCircularLoopSegment{exactLoopQuarterDisk(p(8, 5), .75, true)}, Bottom: 0, Top: 1},
		{Outer: exactLoopPolygon(p(6, 3), p(10, 3), p(10, 5), p(6, 5)), Holes: [][]ExactCircularLoopSegment{exactLoopQuarterDisk(p(8, 4), .5, true)}, Bottom: -3, Top: -2},
	}
}

func TestExactCircularSweepMultipleInsetsKeepOriginalFloors(t *testing.T) {
	ctx := context.Background()
	host, guests := multipleSweepInsetsFixture()
	a, err := JoinExactCircularSweepInsets(ctx, host, guests, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	faces, edges, vertices := a.Solid.Faces(), a.Solid.Edges(), a.Solid.Vertices()
	if len(faces) != 40 || len(edges) != 96 || len(vertices) != 64 ||
		len(faces[0].Holes) != 2 || len(faces[1].Holes) != 3 || len(a.HostFaces[0]) != 2 || len(a.HostFaces[1]) != 3 {
		t.Fatal("lost original topology or separate exposed floors")
	}
	floor := [3]int{19, 29, 39}
	for i, g := range a.Guests {
		if g.EdgeOffset != 24*(i+1) || g.VertexOffset != 16*(i+1) || g.Faces[g.GuestCap] != -1 ||
			g.HostCap != [3]int{1, 1, 0}[i] || faces[floor[i]].Surface != faces[g.HostCap].Surface {
			t.Fatalf("guest %d lost source order or its original host support: %+v", i, g)
		}
		rim := g.EdgeOffset + 8*g.GuestCap + 4
		found := false
		for _, use := range faces[floor[i]].Outer {
			found = found || use.Edge == rim
		}
		if !found {
			t.Fatalf("floor %d did not retain its original guest opening", i)
		}
	}
	m, err := a.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(m.Volume-(200+8.6875*math.Pi)) > 1e-7 || math.Abs(m.Area-(284+20*math.Pi)) > 1e-7 {
		t.Fatalf("wrong joined material: %+v %v", m, err)
	}
	display, err := a.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .25, Angle: .4, MaxSubdivisions: 16, MaxTriangles: 2_000})
	if err != nil {
		t.Fatal(err)
	}
	if err := display.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_SWEEP_INSET"); bin != "" {
		raw, err := exec.CommandContext(ctx, bin, "multiple").CombinedOutput()
		var ref struct {
			Solids       int
			Volume, Area float64
		}
		if err != nil || json.Unmarshal(raw, &ref) != nil || ref.Solids != 1 ||
			math.Abs(ref.Volume-m.Volume) > 1e-7 || math.Abs(ref.Area-m.Area) > 1e-7 {
			t.Fatalf("pinned OCCT material differs: %+v %v %s", ref, err, raw)
		}
	}
}

func TestExactCircularSweepMultipleInsetsRejectsCrossingGuests(t *testing.T) {
	ctx := context.Background()
	host, guests := multipleSweepInsetsFixture()
	p := geom.Vec2{X: 4, Y: 4}
	guests[1].Outer = exactLoopQuarterDisk(p, 2, false)
	guests[1].Holes[0] = exactLoopQuarterDisk(p, .75, true)
	// Each guest must be valid and strictly inset independently.
	if _, err := JoinExactCircularSweepInset(ctx, host, guests[1], ExactCircularLoopOptions{}); err != nil {
		t.Fatal(err)
	}
	bad, err := JoinExactCircularSweepInsets(ctx, host, guests, ExactCircularLoopOptions{})
	if !errors.Is(err, geom.ErrDomain) || len(bad.Solid.Faces()) != 0 || len(bad.Guests) != 0 {
		t.Fatalf("returned material for intersecting same-cap guests: %v", err)
	}
}

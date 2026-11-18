package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func circularSweepInsetFixture() (ExactCircularSweep, ExactCircularSweep) {
	host := ExactCircularSweep{
		Outer:  exactLoopPolygon(geom.Vec2{}, geom.Vec2{X: 8}, geom.Vec2{X: 8, Y: 6}, geom.Vec2{Y: 6}),
		Holes:  [][]ExactCircularLoopSegment{exactLoopQuarterDisk(geom.Vec2{X: 1, Y: 1}, .5, true)},
		Bottom: -2, Top: 0,
	}
	guest := ExactCircularSweep{
		Outer:  exactLoopQuarterDisk(geom.Vec2{X: 4, Y: 3}, 2, false),
		Holes:  [][]ExactCircularLoopSegment{exactLoopQuarterDisk(geom.Vec2{X: 4, Y: 3}, .5, true)},
		Bottom: 0, Top: 3,
	}
	return host, guest
}

func TestExactCircularSweepInsetOpeningsRetainOriginalFloor(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, name := range []string{"above", "below"} {
		t.Run(name, func(t *testing.T) {
			host, guest := circularSweepInsetFixture()
			cap, removed := 1, 0
			if name == "below" {
				host.Bottom, host.Top = 0, 2
				guest.Bottom, guest.Top = -3, 0
				cap, removed = 0, 1
			}
			q, err := JoinExactCircularSweepInset(ctx, host, guest, ExactCircularLoopOptions{})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("original cap proof and continuous shared edges passed")
			faces, edges, vs := q.Solid.Faces(), q.Solid.Edges(), q.Solid.Vertices()
			if q.HostCap != cap || q.GuestCap != removed || q.GuestFaces[removed] != -1 ||
				q.GuestVertexOffset != 16 || q.GuestEdgeOffset != 24 || len(faces) != 20 ||
				len(edges) != 48 || len(vs) != 32 || !reflect.DeepEqual(q.HostFaces[cap], []int{cap, 19}) ||
				len(faces[cap].Holes) != 2 || len(faces[19].Holes) != 0 || faces[cap].Surface != faces[19].Surface {
				t.Fatalf("lost source topology or floor support: %+v", q.HostFaces)
			}
			for _, tcase := range []struct {
				face int
				p    geom.Vec2
				want trimvalidate.Location
			}{
				{cap, geom.Vec2{X: 1, Y: 1}, trimvalidate.Outside},
				{cap, geom.Vec2{X: 4, Y: 3}, trimvalidate.Outside},
				{cap, geom.Vec2{X: 7, Y: 3}, trimvalidate.Inside},
				{19, geom.Vec2{X: 4, Y: 3}, trimvalidate.Inside},
				{19, geom.Vec2{X: 3, Y: 3}, trimvalidate.Outside},
			} {
				got, proof, e := LocateInFaceTrim(ctx, faces[tcase.face], tcase.p, trimvalidate.Options{})
				if e != nil || proof.Status != trimvalidate.Certified || got != tcase.want {
					t.Fatalf("wrong retained cap/floor subset %v: %v %v %v", tcase, got, proof.Status, e)
				}
			}
			m, err := q.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
				t.Fatalf("material differs from independent annular union: %+v %v", m, err)
			}
			if name == "above" {
				t.Log("area and volume agree; checking display")
				mesh, err := q.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .25, Angle: .5, MaxSubdivisions: 16, MaxTriangles: 26000})
				if err != nil {
					t.Fatal(err)
				}
				if err = mesh.ValidateSolid(); err != nil {
					t.Fatal("open tessellation", err)
				}
				t.Logf("closed joined cap and floor mesh: %d triangles", len(mesh.Triangles))
				var b, again bytes.Buffer
				if err = WriteJSON(&b, q.Solid); err != nil {
					t.Fatal(err)
				}
				back, err := ReadJSONContext(ctx, bytes.NewReader(b.Bytes()), JSONLimits{})
				if err != nil {
					t.Fatal(err)
				}
				if err = WriteJSON(&again, back); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
					t.Fatalf("reopen changed exact cap and floor trims: %v", err)
				}
			}
		})
	}
}

func TestExactCircularSweepInsetRejectsUnprovedInterface(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		alter func(*ExactCircularSweep)
	}{
		{"cap gap", func(g *ExactCircularSweep) { g.Bottom = .0625 }},
		{"touches outside", func(g *ExactCircularSweep) {
			g.Outer = exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 3}, 2, false)
			g.Holes = nil
		}},
		{"contains host hole", func(g *ExactCircularSweep) {
			g.Outer = exactLoopQuarterDisk(geom.Vec2{X: 1, Y: 1}, .75, false)
			g.Holes = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, g := circularSweepInsetFixture()
			tc.alter(&g)
			q, err := JoinExactCircularSweepInset(ctx, h, g, ExactCircularLoopOptions{})
			if err == nil || len(q.Solid.Faces()) != 0 || len(q.HostFaces) > 0 {
				t.Fatalf("published unproved interface: %v", err)
			}
		})
	}
}

func TestExactCircularSweepInsetLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_SWEEP_INSET")
	if bin == "" {
		t.Skip("optional pinned OCCT annular inset reference")
	}
	out, err := exec.Command(bin).CombinedOutput()
	var m struct {
		Solids       int
		Volume, Area float64
	}
	if err != nil || json.Unmarshal(out, &m) != nil || m.Solids != 1 ||
		math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
		t.Fatalf("independent OCCT material differs: %+v %v %s", m, err, out)
	}
}

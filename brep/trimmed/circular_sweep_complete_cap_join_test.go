package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func completeCircularSweepPair() (ExactCircularSweep, ExactCircularSweep) {
	a, _ := circularSweepInsetFixture()
	a.Holes = append(a.Holes, exactLoopQuarterDisk(geom.Vec2{X: 6, Y: 4}, .5, true))
	rot := func(r []ExactCircularLoopSegment, j int) []ExactCircularLoopSegment {
		return append(append([]ExactCircularLoopSegment(nil), r[j:]...), r[:j]...)
	}
	b := ExactCircularSweep{Outer: rot(a.Outer, 1), Holes: [][]ExactCircularLoopSegment{rot(a.Holes[1], 3), rot(a.Holes[0], 1)}, Bottom: 0, Top: 3}
	return a, b
}

func TestExactCircularSweepCompleteCapJoinPreservesOriginalSeam(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b := completeCircularSweepPair()
	for _, first := range []bool{true, false} {
		x, y := a, b
		if !first {
			x, y = y, x
		}
		j, err := JoinExactCircularSweepCaps(ctx, x, y, ExactCircularLoopOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(j.Solid.Faces()) != 26 || len(j.Solid.Vertices()) != 36 || len(j.Solid.Edges()) != 60 ||
			len(j.SharedRimEdges) != 12 || j.Faces[0][j.Caps[0]] != -1 || j.Faces[1][j.Caps[1]] != -1 {
			t.Fatalf("complete seam topology did not retain both original wall sets: %+v", j.Caps)
		}
		for i, id := range j.SharedRimEdges {
			if j.Edges[1][12*j.Caps[1]+i] != id || j.Edges[0][id] != id {
				t.Fatalf("original edge owners do not share seam identity at %d", id)
			}
			count := 0
			for _, f := range j.Solid.Faces() {
				for _, u := range f.Outer {
					if u.Edge == id {
						count++
					}
				}
			}
			if count != 2 {
				t.Fatalf("unexpected seam incidence: %d", count)
			}
		}
		m, err := j.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Volume-(240-2.5*math.Pi)) > 1e-7 || math.Abs(m.Area-(236+9*math.Pi)) > 1e-7 {
			t.Fatalf("complete cap material differs: %+v %v", m, err)
		}
		if first {
			mesh, err := j.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .25, Angle: .5, MaxSubdivisions: 16, MaxTriangles: 2000})
			if err != nil {
				t.Fatal(err)
			}
			if err = mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestExactCircularSweepCompleteCapRejectsDifferentSupports(t *testing.T) {
	ctx := context.Background()
	a, b := completeCircularSweepPair()
	b.Holes[1][0].Center = geom.Vec2{X: .5, Y: .5}
	if _, err := ExtrudeExactDirectedCircularLoops(ctx, b.Outer, b.Holes, b.Bottom, b.Top, ExactCircularLoopOptions{}); err != nil {
		t.Fatalf("changed authored cap is not valid by itself: %v", err)
	}
	if r, err := JoinExactCircularSweepCaps(ctx, a, b, ExactCircularLoopOptions{}); err == nil || len(r.Solid.Faces()) != 0 || !strings.Contains(err.Error(), "rim support differs") {
		t.Fatalf("changed exact arc with same vertices was accepted: %v", err)
	}
	a, b = completeCircularSweepPair()
	b.Bottom = .125
	if r, err := JoinExactCircularSweepCaps(ctx, a, b, ExactCircularLoopOptions{}); err == nil || len(r.Solid.Faces()) != 0 {
		t.Fatalf("gap published as material seam: %v", err)
	}
}

func TestExactCircularSweepCompleteCapLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_SWEEP_INSET")
	if bin == "" {
		t.Skip("optional pinned OCCT complete cap reference")
	}
	raw, err := exec.Command(bin, "complete").CombinedOutput()
	var m struct {
		Solids       int
		Volume, Area float64
	}
	if err != nil || json.Unmarshal(raw, &m) != nil || m.Solids != 1 ||
		math.Abs(m.Volume-(240-2.5*math.Pi)) > 1e-7 || math.Abs(m.Area-(236+9*math.Pi)) > 1e-7 {
		t.Fatalf("independent OCCT complete cap material differs %+v %v %s", m, err, raw)
	}
}

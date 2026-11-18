package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func circularSweepCapOverlayExchangeFixture() [2]trimmed.ExactCircularSweep {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	ring := func(points ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
		out := make([]trimmed.ExactCircularLoopSegment, len(points))
		for i, v := range points {
			out[i] = trimmed.ExactCircularLoopSegment{Start: v, End: points[(i+1)%len(points)]}
		}
		return out
	}
	outer := ring(p(5, 0), p(0, 5), p(-5, 0), p(0, -5))
	for i := range outer {
		outer[i].Radius = 5
	}
	splitArc := func(s trimmed.ExactCircularLoopSegment, station geom.Vec2) []trimmed.ExactCircularLoopSegment {
		a, b := s, s
		a.End, b.Start = station, station
		return []trimmed.ExactCircularLoopSegment{a, b}
	}
	hole := ring(p(-1, -1), p(-1, 1), p(1, 1), p(1, -1))
	a := trimmed.ExactCircularSweep{Outer: outer, Holes: [][]trimmed.ExactCircularLoopSegment{
		append(splitArc(hole[0], p(-1, 0)), hole[1:]...),
	}, Bottom: -2, Top: 0}
	b := trimmed.ExactCircularSweep{Outer: append(splitArc(outer[0], p(3, 4)), outer[1:]...),
		Holes: [][]trimmed.ExactCircularLoopSegment{hole}, Bottom: 0, Top: 3}
	return [2]trimmed.ExactCircularSweep{a, b}
}

func TestExactCircularSweepCapOverlayAnalyticExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	sweeps := circularSweepCapOverlayExchangeFixture()
	frame, err := geom.NewFrame(geom.Vec3{X: 10, Y: -3, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, n trimmed.CircularSweepCapOverlay) {
		t.Helper()
		if len(n.Solid.Faces()) != 22 || len(n.SharedRimEdges) != 10 || len(n.Faces[0][1]) != 0 || len(n.Faces[1][0]) != 0 ||
			len(n.Faces[0][2]) != 2 || len(n.Faces[1][7]) != 2 || n.Edges[0][9][1].Map[1][1].RatString() != "-1/6" {
			t.Fatal("strict exchange lost either source subdivision or retained an internal cap")
		}
		m, err := n.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Volume-(125*math.Pi-20)) > 1e-7 || math.Abs(m.Area-(100*math.Pi+32)) > 1e-7 {
			t.Fatalf("wrong verified join material: %+v %v", m, err)
		}
	}
	live := func(t *testing.T, env, suffix string, raw []byte) {
		t.Helper()
		reader := os.Getenv(env)
		if reader == "" {
			return
		}
		path := filepath.Join(t.TempDir(), "overlay"+suffix)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		result, err := exec.CommandContext(ctx, reader, "read", path).CombinedOutput()
		i := bytes.LastIndexByte(result, '{')
		var m struct {
			Solids, Faces int
			Volume, Area  float64
		}
		if err != nil || i < 0 || json.Unmarshal(result[i:], &m) != nil || m.Solids != 1 || m.Faces != 22 ||
			math.Abs(m.Volume-(125*math.Pi-20)) > 1e-7 || math.Abs(m.Area-(100*math.Pi+32)) > 1e-7 {
			t.Fatalf("independent OCCT readback lost analytic join: %+v %v %s", m, err, result)
		}
	}
	t.Run("STEP", func(t *testing.T) {
		d := step.ExactCircularSweepCapOverlay{Name: "Two original rim partitions", Frame: frame, Sweeps: sweeps, LengthUnit: .001}
		var raw bytes.Buffer
		if err := step.WriteExactCircularSweepCapOverlay(ctx, &raw, d); err != nil {
			t.Fatal(err)
		}
		p, err := step.ReadExactCircularSweepCapOverlay(ctx, bytes.NewReader(raw.Bytes()), d, step.Limits{})
		if err != nil || p.Body <= 0 || p.Shell <= 0 || len(p.Faces[0][2]) != 2 || len(p.Faces[0][1]) != 0 {
			t.Fatalf("missing STEP original face roles: %v", err)
		}
		check(t, p.Native)
		live(t, "PLAINSOLID_OCCT_DIRECTED_CIRCULAR_STEP", ".step", raw.Bytes())
		d.Sweeps[1].Top++
		if p, err := step.ReadExactCircularSweepCapOverlay(ctx, bytes.NewReader(raw.Bytes()), d, step.Limits{}); err == nil || p.Source != nil {
			t.Fatal("returned STEP ownership when the other original cap yields a different boundary")
		}
	})
	t.Run("IGES", func(t *testing.T) {
		d := iges.ExactCircularSweepCapOverlay{Sweeps: sweeps, Unit: .001}
		var raw bytes.Buffer
		if err := iges.WriteExactCircularSweepCapOverlay(ctx, &raw, d); err != nil {
			t.Fatal(err)
		}
		p, err := iges.ReadExactCircularSweepCapOverlay(ctx, bytes.NewReader(raw.Bytes()), d, iges.Limits{})
		if err != nil || p.SourceSolid <= 0 || p.SourceShell <= 0 || len(p.Faces[1][7]) != 2 || len(p.Faces[1][0]) != 0 {
			t.Fatalf("missing IGES original face roles: %v", err)
		}
		check(t, p.Native)
		live(t, "PLAINSOLID_OCCT_PLANAR_SHELLS_IGES", ".igs", raw.Bytes())
		d.Sweeps[0].Bottom--
		if p, err := iges.ReadExactCircularSweepCapOverlay(ctx, bytes.NewReader(raw.Bytes()), d, iges.Limits{}); err == nil || p.SourceSolid != 0 {
			t.Fatal("returned IGES ownership after a mismatching source boundary")
		}
	})
}

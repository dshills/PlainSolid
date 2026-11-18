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

func TestExactCircularSweepMultipleInsetsAnalyticExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	loop := func(points ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
		out := make([]trimmed.ExactCircularLoopSegment, len(points))
		for i, v := range points {
			out[i] = trimmed.ExactCircularLoopSegment{Start: v, End: points[(i+1)%len(points)]}
		}
		return out
	}
	circle := func(x, y, r float64, reverse bool) []trimmed.ExactCircularLoopSegment {
		s := r
		if reverse {
			s = -r
		}
		data := loop(p(x+r, y), p(x, y+s), p(x-r, y), p(x, y-s))
		for i := range data {
			data[i].Radius, data[i].Center = r, p(x, y)
		}
		return data
	}
	bore := func(x, y, r float64) [][]trimmed.ExactCircularLoopSegment {
		return [][]trimmed.ExactCircularLoopSegment{circle(x, y, r, true)}
	}
	host := trimmed.ExactCircularSweep{Outer: loop(p(0, 0), p(12, 0), p(12, 8), p(0, 8)), Holes: bore(1, 1, .5), Bottom: -2}
	guests := []trimmed.ExactCircularSweep{
		{Outer: circle(3, 4, 1.5, false), Holes: bore(3, 4, .5), Top: 3},
		{Outer: circle(8, 5, 2, false), Holes: bore(8, 5, .75), Top: 1},
		{Outer: loop(p(6, 3), p(10, 3), p(10, 5), p(6, 5)), Holes: bore(8, 4, .5), Bottom: -3, Top: -2},
	}
	v, a := 200+8.6875*math.Pi, 284+20*math.Pi
	check := func(t *testing.T, p trimmed.CircularSweepInsets) {
		t.Helper()
		if len(p.HostFaces[0]) != 2 || len(p.HostFaces[1]) != 3 || len(p.Guests) != 3 || p.Guests[2].HostCap != 0 {
			t.Fatal("lost original owner or a separate floor")
		}
		m, err := p.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-a) > 1e-7 {
			t.Fatalf("wrong re-proved multi-inset material: %+v %v", m, err)
		}
	}
	live := func(t *testing.T, env, ext string, raw []byte) {
		t.Helper()
		bin := os.Getenv(env)
		if bin == "" {
			return
		}
		path := filepath.Join(t.TempDir(), "multiple"+ext)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		result, err := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		i := bytes.LastIndexByte(result, '{')
		var m struct {
			Solids, Faces int
			Volume, Area  float64
		}
		if err != nil || i < 0 || json.Unmarshal(result[i:], &m) != nil || m.Solids != 1 || m.Faces != 40 ||
			math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-a) > 1e-7 {
			t.Fatalf("independent reader lost the 40-face join: %+v %v %s", m, err, result)
		}
	}
	t.Run("STEP", func(t *testing.T) {
		frame, err := geom.NewFrame(geom.Vec3{X: 10, Y: -3, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
		if err != nil {
			t.Fatal(err)
		}
		d := step.ExactCircularSweepInsets{Name: "Three original inset guests", Frame: frame, Host: host, Guests: guests, LengthUnit: .001}
		var raw bytes.Buffer
		if err := step.WriteExactCircularSweepInsets(ctx, &raw, d); err != nil {
			t.Fatal(err)
		}
		got, err := step.ReadExactCircularSweepInsets(ctx, bytes.NewReader(raw.Bytes()), d, step.Limits{})
		if err != nil || got.Body <= 0 || len(got.HostFaces[1]) != 3 || got.GuestFaces[0][0] != 0 || got.GuestFaces[2][1] != 0 {
			t.Fatalf("lost STEP face ownership: %v", err)
		}
		check(t, got.Native)
		live(t, "PLAINSOLID_OCCT_DIRECTED_CIRCULAR_STEP", ".step", raw.Bytes())
		d.Guests = []trimmed.ExactCircularSweep{guests[1], guests[0], guests[2]}
		if bad, err := step.ReadExactCircularSweepInsets(ctx, bytes.NewReader(raw.Bytes()), d, step.Limits{}); err == nil || bad.Source != nil {
			t.Fatal("published ownership for a different original guest order")
		}
	})
	t.Run("IGES", func(t *testing.T) {
		d := iges.ExactCircularSweepInsets{Host: host, Guests: append([]trimmed.ExactCircularSweep(nil), guests...), Unit: .001}
		var raw bytes.Buffer
		if err := iges.WriteExactCircularSweepInsets(ctx, &raw, d); err != nil {
			t.Fatal(err)
		}
		got, err := iges.ReadExactCircularSweepInsets(ctx, bytes.NewReader(raw.Bytes()), d, iges.Limits{})
		if err != nil || got.SourceSolid <= 0 || len(got.HostFaces[0]) != 2 || got.GuestFaces[0][0] != 0 || got.GuestFaces[2][1] != 0 {
			t.Fatalf("lost IGES face ownership: %v", err)
		}
		check(t, got.Native)
		live(t, "PLAINSOLID_OCCT_PLANAR_SHELLS_IGES", ".igs", raw.Bytes())
		d.Guests[0].Top = 2
		if bad, err := iges.ReadExactCircularSweepInsets(ctx, bytes.NewReader(raw.Bytes()), d, iges.Limits{}); err == nil || bad.SourceSolid != 0 {
			t.Fatal("published an IGES result with a different original cap height")
		}
	})
}

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func exactSweepSTEPLineLoop(p ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
	a := make([]trimmed.ExactCircularLoopSegment, len(p))
	for i := range a {
		a[i] = trimmed.ExactCircularLoopSegment{Start: p[i], End: p[(i+1)%len(p)]}
	}
	return a
}

func exactSweepSTEPCircle(x, y, r float64, clockwise bool) []trimmed.ExactCircularLoopSegment {
	p := [4]geom.Vec2{{X: x + r, Y: y}, {X: x, Y: y + r}, {X: x - r, Y: y}, {X: x, Y: y - r}}
	if clockwise {
		p[1], p[3] = p[3], p[1]
	}
	a := exactSweepSTEPLineLoop(p[:]...)
	for i := range a {
		a[i].Center = geom.Vec2{X: x, Y: y}
		a[i].Radius = r
	}
	return a
}

func exactSweepInsetSTEPFixture(t *testing.T, below bool) ExactCircularSweepInset {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 10, Y: -3, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	d := ExactCircularSweepInset{Name: "Original host and hollow guest", Frame: frame, LengthUnit: .001,
		Host: trimmed.ExactCircularSweep{
			Outer: exactSweepSTEPLineLoop(geom.Vec2{}, geom.Vec2{X: 8}, geom.Vec2{X: 8, Y: 6}, geom.Vec2{Y: 6}),
			Holes: [][]trimmed.ExactCircularLoopSegment{exactSweepSTEPCircle(1, 1, .5, true)}, Bottom: -2, Top: 0},
		Guest: trimmed.ExactCircularSweep{Outer: exactSweepSTEPCircle(4, 3, 2, false), Holes: [][]trimmed.ExactCircularLoopSegment{exactSweepSTEPCircle(4, 3, .5, true)}, Bottom: 0, Top: 3},
	}
	if below {
		d.Host.Bottom, d.Host.Top = 0, 2
		d.Guest.Bottom, d.Guest.Top = -3, 0
	}
	return d
}

func TestExactCircularSweepInsetSTEPRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, below := range []bool{false, true} {
		t.Run(fmt.Sprint("guest-below-", below), func(t *testing.T) {
			d := exactSweepInsetSTEPFixture(t, below)
			var b, again bytes.Buffer
			if e := WriteExactCircularSweepInset(ctx, &b, d); e != nil {
				t.Fatal(e)
			}
			p, e := ReadExactCircularSweepInset(ctx, bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			native := p.Native
			if p.Descriptor.Frame != d.Frame || p.Descriptor.Host.Bottom != d.Host.Bottom || p.Descriptor.Guest.Top != d.Guest.Top ||
				len(p.HostFaces[native.HostCap]) != 2 || p.GuestFaces[native.GuestCap] != 0 || len(native.Solid.Faces()) != 20 ||
				len(native.Solid.Faces()[native.HostCap].Holes) != 2 || native.GuestEdgeOffset != 24 {
				t.Fatalf("canonical STEP lost original support and floor roles: %+v", p.HostFaces)
			}
			m, e := native.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
				t.Fatalf("wrong re-opened material %+v %v", m, e)
			}
			if e = WriteExactCircularSweepInset(ctx, &again, p.Descriptor); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatalf("strict canonical re-export differs: %v", e)
			}
			floor := p.HostFaces[native.HostCap][1]
			start := bytes.Index(b.Bytes(), []byte(fmt.Sprintf("#%d=ADVANCED_FACE(", floor)))
			if start < 0 {
				t.Fatal("missing floor record")
			}
			end := bytes.IndexByte(b.Bytes()[start:], '\n') + start
			raw := b.Bytes()[start:end]
			before, after := []byte(",.T.);"), []byte(",.F.);")
			if below {
				before, after = after, before
			}
			badline := bytes.Replace(raw, before, after, 1)
			if bytes.Equal(raw, badline) {
				t.Fatalf("missing floor orientation in %s", raw)
			}
			bad := bytes.Replace(b.Bytes(), raw, badline, 1)
			if x, e := ReadExactCircularSweepInset(ctx, bytes.NewReader(bad), Limits{}); e == nil || x.Source != nil {
				t.Fatal("published a changed host floor")
			}
		})
	}
}

func TestExactCircularSweepInsetSTEPLiveOCCT(t *testing.T) {
	reader := os.Getenv("PLAINSOLID_OCCT_DIRECTED_CIRCULAR_STEP")
	if reader == "" {
		t.Skip("optional pinned independent STEP reader")
	}
	for _, below := range []bool{false, true} {
		d := exactSweepInsetSTEPFixture(t, below)
		var b bytes.Buffer
		if e := WriteExactCircularSweepInset(context.Background(), &b, d); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "joined.step")
		if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out, e := exec.Command(reader, "read", path).CombinedOutput()
		i := bytes.LastIndexByte(out, '{')
		var m struct {
			Solids, Faces int
			Volume, Area  float64
		}
		if e != nil || i < 0 || json.Unmarshal(out[i:], &m) != nil || m.Solids != 1 || m.Faces != 20 ||
			math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
			t.Fatalf("independent STEP transfer below=%v: %+v %v %s", below, m, e, out)
		}
	}
}

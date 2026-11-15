package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func circularInsetIGESFixture(below bool) ExactCircularSweepInset {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	line := func(v ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
		s := make([]trimmed.ExactCircularLoopSegment, len(v))
		for i := range v {
			s[i] = trimmed.ExactCircularLoopSegment{Start: v[i], End: v[(i+1)%len(v)]}
		}
		return s
	}
	disk := func(x, y, r float64, cw bool) []trimmed.ExactCircularLoopSegment {
		v := []geom.Vec2{p(x+r, y), p(x, y+r), p(x-r, y), p(x, y-r)}
		if cw {
			v[1], v[3] = v[3], v[1]
		}
		s := line(v...)
		for i := range s {
			s[i].Center, s[i].Radius = p(x, y), r
		}
		return s
	}
	d := ExactCircularSweepInset{
		Host:  trimmed.ExactCircularSweep{Outer: line(p(0, 0), p(8, 0), p(8, 6), p(0, 6)), Holes: [][]trimmed.ExactCircularLoopSegment{disk(1, 1, .5, true)}, Bottom: -2, Top: 0},
		Guest: trimmed.ExactCircularSweep{Outer: disk(4, 3, 2, false), Holes: [][]trimmed.ExactCircularLoopSegment{disk(4, 3, .5, true)}, Bottom: 0, Top: 3},
		Unit:  .001,
	}
	if below {
		d.Host.Bottom, d.Host.Top = 0, 2
		d.Guest.Bottom, d.Guest.Top = -3, 0
	}
	return d
}

func TestExactCircularSweepInsetIGESReproof(t *testing.T) {
	ctx := context.Background()
	for _, below := range []bool{false, true} {
		t.Run(strconv.FormatBool(below), func(t *testing.T) {
			d := circularInsetIGESFixture(below)
			var original, roundtrip bytes.Buffer
			if e := WriteExactCircularSweepInset(ctx, &original, d); e != nil {
				t.Fatal(e)
			}
			out, e := ReadExactCircularSweepInset(ctx, bytes.NewReader(original.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if out.SourceSolid <= out.SourceShell || out.SourceEdgeList <= out.SourceVertexList ||
				out.GuestFaces[out.Native.GuestCap] != 0 || len(out.HostFaces[out.Native.HostCap]) != 2 ||
				out.Descriptor.Host.Bottom != d.Host.Bottom || out.Descriptor.Guest.Top != d.Guest.Top || out.Descriptor.Unit != d.Unit {
				t.Fatal("original member roles are missing")
			}
			m, e := out.Native.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
				t.Fatalf("wrong canonical IGES material: %+v %v", m, e)
			}
			if e = WriteExactCircularSweepInset(ctx, &roundtrip, out.Descriptor); e != nil || !bytes.Equal(original.Bytes(), roundtrip.Bytes()) {
				t.Fatalf("canonical source changed on reopen: %v", e)
			}
			a, roles, native, e := buildCircularSweepInsetIGES(ctx, d)
			if e != nil {
				t.Fatal(e)
			}
			sh := &a.records[roles.shell/2]
			p := strings.Split(strings.TrimSuffix(sh.data, ";"), ",")
			k := 2*native.HostFaces[native.HostCap][1] + 3
			if p[k] == "1" {
				p[k] = "0"
			} else {
				p[k] = "1"
			}
			sh.data = strings.Join(p, ",") + ";"
			b, e := a.file(d.Unit)
			if e != nil {
				t.Fatal(e)
			}
			if x, e := ReadExactCircularSweepInset(ctx, bytes.NewReader(b), Limits{}); e == nil || x.SourceSolid != 0 {
				t.Fatal("published changed floor orientation")
			}
		})
	}
}

func TestExactCircularSweepInsetIGESLiveOCCT(t *testing.T) {
	reader := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if reader == "" {
		t.Skip("optional pinned independent IGES reader")
	}
	for _, below := range []bool{false, true} {
		d := circularInsetIGESFixture(below)
		var b bytes.Buffer
		if e := WriteExactCircularSweepInset(context.Background(), &b, d); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "inset.igs")
		if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out, e := exec.Command(reader, "read", p).CombinedOutput()
		i := bytes.LastIndexByte(out, '{')
		var m struct {
			Solids, Faces int
			Volume, Area  float64
		}
		if e != nil || i < 0 || json.Unmarshal(out[i:], &m) != nil || m.Solids != 1 || m.Faces != 20 ||
			math.Abs(m.Volume-(96+10.75*math.Pi)) > 1e-7 || math.Abs(m.Area-(152+16.5*math.Pi)) > 1e-7 {
			t.Fatalf("independent IGES inset below=%v %+v %v %s", below, m, e, out)
		}
	}
}

func FuzzReadExactCircularSweepInsetIGES(f *testing.F) {
	var seed bytes.Buffer
	ctx := context.Background()
	if e := WriteExactCircularSweepInset(ctx, &seed, circularInsetIGESFixture(true)); e != nil {
		f.Fatal(e)
	}
	f.Add(seed.Bytes())
	f.Add([]byte("not an IGES model"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		p, e := ReadExactCircularSweepInset(ctx, bytes.NewReader(raw), Limits{MaxBytes: 1 << 17, MaxEntities: 640, MaxRecords: 2048})
		if e != nil {
			if p.SourceSolid != 0 || len(p.Native.Solid.Faces()) != 0 {
				t.Fatal("failed source exposed part of the join")
			}
			return
		}
		if p.SourceSolid < 1 || p.GuestFaces[p.Native.GuestCap] != 0 {
			t.Fatal("invalid joined ownership")
		}
	})
}

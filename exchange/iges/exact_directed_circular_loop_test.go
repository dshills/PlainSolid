package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func directedIGESFixture(holes bool) (ExactDirectedCircularLoopExtrusion, float64, float64, int) {
	point := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	ring := func(p ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
		s := make([]trimmed.ExactCircularLoopSegment, len(p))
		for i := range p {
			s[i].Start, s[i].End = p[i], p[(i+1)%len(p)]
		}
		return s
	}
	d := ExactDirectedCircularLoopExtrusion{Bottom: -1, Top: 1, Unit: .001}
	if holes {
		d.Outer = ring(point(0, 0), point(6, 0), point(6, 4), point(0, 4))
		for _, x := range []float64{2, 4} {
			h := ring(point(x+.5, 2), point(x, 1.5), point(x-.5, 2), point(x, 2.5))
			for i := range h {
				h[i].Center, h[i].Radius = point(x, 2), .5
			}
			d.Holes = append(d.Holes, h)
		}
		return d, 48 - math.Pi, 88 + 3*math.Pi, 14
	}
	d.Outer = ring(point(0, 0), point(4, 0), point(4, 1), point(2, 1), point(1, 2), point(1, 4), point(0, 4))
	d.Outer[3].Center, d.Outer[3].Radius = point(2, 2), 1
	return d, 16 - math.Pi/2, 44 + math.Pi/2, 9
}

func TestExactDirectedCircularLoopIGESLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if bin == "" {
		t.Skip("optional pinned independent IGES reader")
	}
	for _, item := range []struct {
		name  string
		holes bool
	}{{"reentrant", false}, {"two holes", true}} {
		t.Run(item.name, func(t *testing.T) {
			d, vol, area, faces := directedIGESFixture(item.holes)
			var w bytes.Buffer
			if err := WriteExactDirectedCircularLoopExtrusion(context.Background(), &w, d); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "directed.igs")
			if err := os.WriteFile(path, w.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := exec.Command(bin, "read", path).CombinedOutput()
			i := bytes.LastIndexByte(got, '{')
			var m struct {
				Solids, Faces int
				Volume, Area  float64
			}
			if err != nil || i < 0 || json.Unmarshal(got[i:], &m) != nil || m.Solids != 1 || m.Faces != faces ||
				math.Abs(m.Volume-vol) > 1e-8 || math.Abs(m.Area-area) > 1e-8 {
				t.Fatalf("independent IGES material: want %v %v %v, got %v (%s) %v", vol, area, faces, m, got, err)
			}
		})
	}
}

func TestExactDirectedCircularLoopIGESRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, item := range []struct {
		name  string
		holes bool
	}{{"reentrant", false}, {"two holes", true}} {
		t.Run(item.name, func(t *testing.T) {
			d, vol, area, faces := directedIGESFixture(item.holes)
			var b, again bytes.Buffer
			if err := WriteExactDirectedCircularLoopExtrusion(ctx, &b, d); err != nil {
				t.Fatal(err)
			}
			q, err := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(b.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if len(q.Native.Faces()) != faces || q.Descriptor.Unit != d.Unit || q.SourceSolid <= q.SourceShell ||
				q.SourceEdges <= q.SourceVertices || len(q.SourceWalls) != faces-2 || len(q.SourceVerticalCurves) != faces-2 ||
				len(q.SourceCapLoops[0]) != 1+len(d.Holes) || len(q.SourceCapLoops[1]) != 1+len(d.Holes) ||
				len(q.RingOffsets) != 2+len(d.Holes) || q.RingOffsets[len(q.RingOffsets)-1] != faces-2 {
				t.Fatalf("lost original IGES topology or units: %+v", q.RingOffsets)
			}
			m, err := q.Native.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(m.Volume-vol) > 1e-8 || math.Abs(m.Area-area) > 1e-8 {
				t.Fatalf("native material: %+v %v", m, err)
			}
			if err = WriteExactDirectedCircularLoopExtrusion(ctx, &again, q.Descriptor); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatalf("canonical re-export changed directed IGES: %v", err)
			}
			if out, e := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(b.Bytes()), Limits{MaxEntities: 2}); e == nil || out.SourceSolid != 0 {
				t.Fatal("ignored source entity quota")
			}
		})
	}
}

func TestExactDirectedCircularLoopIGESRejectEditedGraph(t *testing.T) {
	ctx := context.Background()
	d, _, _, _ := directedIGESFixture(true)
	for _, edit := range []string{"arc endpoint", "cap loop", "cylinder sense", "orphan"} {
		t.Run(edit, func(t *testing.T) {
			a, roles, _, err := buildDirectedIGES(ctx, d)
			if err != nil {
				t.Fatal(err)
			}
			at := func(id int) *nativeTopologyRecord { return &a.records[id/2] }
			switch edit {
			case "arc endpoint":
				edge := at(roles.rims[0][4])
				v := strings.Split(edge.data, ",")
				v[4] = "2.0000000000000001"
				edge.data = strings.Join(v, ",")
			case "cap loop":
				face := at(roles.caps[1])
				v := strings.Split(strings.TrimSuffix(face.data, ";"), ",")
				v[5], v[6] = v[6], v[5]
				face.data = strings.Join(v, ",") + ";"
			case "cylinder sense":
				shell := at(roles.shell)
				v := strings.Split(strings.TrimSuffix(shell.data, ";"), ",")
				v[3+2*6] = "1"
				shell.data = strings.Join(v, ",") + ";"
			case "orphan":
				a.add(116, 0, "116,0,0,0,0;")
			}
			b, err := a.file(d.Unit)
			if err != nil {
				t.Fatal(err)
			}
			x, e := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(b), Limits{})
			if e == nil || x.SourceSolid != 0 {
				t.Fatalf("published altered %s: %v", edit, e)
			}
		})
	}
	d.Holes[0][0].End.X += .125
	var out bytes.Buffer
	if err := WriteExactDirectedCircularLoopExtrusion(ctx, &out, d); err == nil || out.Len() != 0 {
		t.Fatal("writer emitted a disconnected original hole")
	}
}

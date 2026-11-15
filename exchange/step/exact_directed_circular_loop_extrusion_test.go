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
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func directedLoopSegments(v ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
	r := make([]trimmed.ExactCircularLoopSegment, len(v))
	for i, p := range v {
		r[i] = trimmed.ExactCircularLoopSegment{Start: p, End: v[(i+1)%len(v)]}
	}
	return r
}

func directedLoopSTEPFixture(t *testing.T, name string) ExactDirectedCircularLoopExtrusion {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 10, Y: 2, Z: -1}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	d := ExactDirectedCircularLoopExtrusion{Name: name, Frame: frame, Height: -2, LengthUnit: .001}
	if name == "clockwise corner" {
		d.Outer = directedLoopSegments(geom.Vec2{}, geom.Vec2{X: 3.75}, geom.Vec2{X: 4, Y: .25}, geom.Vec2{X: 4, Y: 1},
			geom.Vec2{X: 1.25, Y: 1}, geom.Vec2{X: 1, Y: 1.25}, geom.Vec2{X: 1, Y: 4}, geom.Vec2{Y: 4})
		d.Outer[1].Center, d.Outer[1].Radius = geom.Vec2{X: 3.75, Y: .25}, .25
		d.Outer[4].Center, d.Outer[4].Radius = geom.Vec2{X: 1.25, Y: 1.25}, .25
		return d
	}
	d.Outer = directedLoopSegments(geom.Vec2{}, geom.Vec2{X: 6}, geom.Vec2{X: 6, Y: 4}, geom.Vec2{Y: 4})
	for _, x := range []float64{2, 4} {
		h := directedLoopSegments(geom.Vec2{X: x + .5, Y: 2}, geom.Vec2{X: x, Y: 1.5}, geom.Vec2{X: x - .5, Y: 2}, geom.Vec2{X: x, Y: 2.5})
		for i := range h {
			h[i].Center, h[i].Radius = geom.Vec2{X: x, Y: 2}, .5
		}
		d.Holes = append(d.Holes, h)
	}
	return d
}

func TestExactDirectedCircularLoopExtrusionSTEPRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"clockwise corner", "two curved holes"} {
		t.Run(name, func(t *testing.T) {
			d := directedLoopSTEPFixture(t, name)
			var out bytes.Buffer
			if err := WriteExactDirectedCircularLoopExtrusion(ctx, &out, d); err != nil {
				t.Fatal(err)
			}
			p, err := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(out.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if p.Descriptor.Height != d.Height || p.Descriptor.Frame != d.Frame || p.Descriptor.LengthUnit != d.LengthUnit ||
				!reflect.DeepEqual(p.Descriptor.Outer, d.Outer) || len(p.Descriptor.Holes) != len(d.Holes) {
				t.Fatalf("STEP changed original circle pose or wires: %+v", p.Descriptor)
			}
			if len(d.Holes) > 0 && (!reflect.DeepEqual(p.Descriptor.Holes, d.Holes) || !reflect.DeepEqual(p.RingOffsets, []int{0, 4, 8, 12})) {
				t.Fatalf("STEP merged original inner topology: %+v", p.RingOffsets)
			}
			var again bytes.Buffer
			if err := WriteExactDirectedCircularLoopExtrusion(ctx, &again, p.Descriptor); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal("canonical directed STEP replay differs", err)
			}
			if legacy, err := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(out.Bytes()), Limits{}); err == nil || legacy.Body != 0 {
				t.Fatal("legacy CCW-only path admitted a new directed source")
			}
			cw := 4
			id := p.Rims[0][cw]
			raw := out.Bytes()
			start := bytes.Index(raw, []byte(fmt.Sprintf("#%d=", id)))
			if start < 0 {
				t.Fatalf("missing original CW rim entity %d", id)
			}
			end := bytes.IndexByte(raw[start:], '\n') + start
			line := raw[start:end]
			corrupt := bytes.Replace(line, []byte(",.F.);"), []byte(",.T.);"), 1)
			if bytes.Equal(corrupt, line) {
				t.Fatalf("original CW sense missing in %s", line)
			}
			bad := bytes.Replace(raw, line, corrupt, 1)
			if q, err := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{}); err == nil || q.Body != 0 {
				t.Fatal("changed circle orientation passed exact source verification")
			}
		})
	}
}

func TestExactDirectedCircularLoopExtrusionSTEPLiveOCCT(t *testing.T) {
	reader := os.Getenv("PLAINSOLID_OCCT_DIRECTED_CIRCULAR_STEP")
	if reader == "" {
		t.Skip("optional pinned independent STEP reader")
	}
	ctx := context.Background()
	for _, name := range []string{"clockwise corner", "two curved holes"} {
		t.Run(name, func(t *testing.T) {
			d := directedLoopSTEPFixture(t, name)
			var b bytes.Buffer
			if err := WriteExactDirectedCircularLoopExtrusion(ctx, &b, d); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "directed.step")
			if err := os.WriteFile(out, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			data, err := exec.Command(reader, "read", out).CombinedOutput()
			j := bytes.LastIndexByte(data, '{')
			var occt struct {
				Solids, Faces int
				Volume, Area  float64
			}
			if err != nil || j < 0 || json.Unmarshal(data[j:], &occt) != nil {
				t.Fatalf("pinned OCCT could not read directed analytic STEP: %v %s", err, data)
			}
			v, err := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(b.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := v.Native.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || occt.Solids != 1 || occt.Faces != len(v.Native.Faces()) ||
				math.Abs(occt.Volume-p.Volume) > 1e-7 || math.Abs(occt.Area-p.Area) > 1e-7 {
				t.Fatalf("directed CAD material differs: %+v / %+v %v", p, occt, err)
			}
		})
	}
}

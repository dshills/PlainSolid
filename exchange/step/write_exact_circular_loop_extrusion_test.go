package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func exactLoopSTEPFixture(t testing.TB, negative bool) ExactCircularLoopExtrusion {
	t.Helper()
	p := []geom.Vec2{{Y: -.625}, {X: 4, Y: -.625}, {X: 4.375, Y: .5}, {X: .375, Y: 3.5}, {X: -.625, Y: 3}, {X: -.625}}
	c := []geom.Vec2{{X: 4}, {Y: 3}, {}}
	var loop []trimmed.ExactCircularLoopSegment
	for i := range p {
		s := trimmed.ExactCircularLoopSegment{Start: p[i], End: p[(i+1)%6]}
		if i%2 == 1 {
			s.Center = c[i/2]
			s.Radius = .625
		}
		loop = append(loop, s)
	}
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	h := 2.
	if negative {
		h = -2
		f, e = geom.NewFrame(geom.Vec3{X: 1, Y: -2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
		if e != nil {
			t.Fatal(e)
		}
	}
	return ExactCircularLoopExtrusion{Name: "Exact nonbinary rounded triangle", Frame: f, Segments: loop, Height: h, LengthUnit: .001}
}
func TestExactCircularLoopExtrusionSTEP(t *testing.T) {
	ctx := context.Background()
	for _, negative := range []bool{false, true} {
		d := exactLoopSTEPFixture(t, negative)
		s, e := trimmed.ExtrudeExactCircularLoop(ctx, d.Segments, d.Height, trimmed.ExactCircularLoopOptions{})
		if e != nil {
			t.Fatal(e)
		}
		nonbinary := 0
		for _, f := range s.Faces()[2:] {
			p := f.Surface.(*geom.ExactBezierSurface)
			if _, e := p.ExactPatch().ToBSplineSurface(); errors.Is(e, geom.ErrUnrepresentable) {
				nonbinary++
			}
		}
		if nonbinary == 0 {
			t.Fatal("missing nonbinary controls")
		}
		var b bytes.Buffer
		if e = WriteExactCircularLoopExtrusion(ctx, &b, d); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, v := range f.Entities {
			for _, r := range v.Records {
				counts[r.Type]++
			}
		}
		for typ, want := range map[string]int{"ADVANCED_FACE": 8, "EDGE_CURVE": 18, "VERTEX_POINT": 12, "CIRCLE": 6, "CYLINDRICAL_SURFACE": 3, "PLANE": 5, "MANIFOLD_SOLID_BREP": 1, "PRODUCT": 1} {
			if counts[typ] != want {
				t.Fatal(typ, counts[typ])
			}
		}
		if counts["PCURVE"] != 0 || counts["B_SPLINE_SURFACE_WITH_KNOTS"] != 0 {
			t.Fatal("rounded spline substitution")
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		if _, e = trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{}); e != nil {
			t.Fatal(e)
		}
		if e = WriteExactCircularLoopExtrusion(ctx, sideDiskShortWriter{}, d); !errors.Is(e, io.ErrShortWrite) {
			t.Fatal(e)
		}
	}
}
func TestExactCircularLoopExtrusionSTEPReject(t *testing.T) {
	ctx := context.Background()
	for _, mutate := range []func(*ExactCircularLoopExtrusion){func(d *ExactCircularLoopExtrusion) { d.Segments[1].Radius = 1 }, func(d *ExactCircularLoopExtrusion) { d.Segments[1].End.X = math.Nextafter(d.Segments[1].End.X, 10) }, func(d *ExactCircularLoopExtrusion) { d.Height = 0 }, func(d *ExactCircularLoopExtrusion) {
		d.Frame, _ = geom.NewFrame(geom.Vec3{X: math.Ldexp(1, 54)}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	}, func(d *ExactCircularLoopExtrusion) {
		d.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: .6, Y: .8}, geom.Vec3{Z: 1})
	}, func(d *ExactCircularLoopExtrusion) { d.LengthUnit = math.NaN() }, func(d *ExactCircularLoopExtrusion) { d.Segments = d.Segments[:2] }, func(d *ExactCircularLoopExtrusion) { d.Name = string([]byte{255}) }} {
		d := exactLoopSTEPFixture(t, false)
		mutate(&d)
		var b bytes.Buffer
		if e := WriteExactCircularLoopExtrusion(ctx, &b, d); e == nil || b.Len() != 0 {
			t.Fatal("accepted invalid descriptor")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	var b bytes.Buffer
	if e := WriteExactCircularLoopExtrusion(c, &b, exactLoopSTEPFixture(t, false)); !errors.Is(e, context.Canceled) || b.Len() != 0 {
		t.Fatal(e)
	}
}
func TestExactCircularLoopExtrusionSTEPLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP")
	if bin == "" {
		t.Skip("optional independent analytic author and TransferRoots")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type report struct {
		Volume, Area                   float64
		Centroid                       [3]float64
		Solids, Faces, Edges, Vertices int
	}
	run := func(mode, path string) report {
		b, e := exec.CommandContext(ctx, bin, mode, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte("{\"volume\""))
		if at < 0 {
			t.Fatal(string(b))
		}
		var q report
		if e = json.Unmarshal(b[at:], &q); e != nil {
			t.Fatal(e)
		}
		if q.Solids != 1 || q.Faces != 8 || q.Edges != 18 || q.Vertices != 12 {
			t.Fatal(q)
		}
		if math.Abs(q.Volume-(27+25*math.Pi/32)) > 1e-8 || math.Abs(q.Area-(51+105*math.Pi/32)) > 1e-8 {
			t.Fatal("independent analytic measures", q)
		}
		return q
	}
	run("read", "testdata/occt-exact-rounded-triangle-extrusion.step")
	run("read", "testdata/occt-model-only-rounded-triangle-extrusion.step")
	for _, negative := range []bool{false, true} {
		mode := "write"
		if negative {
			mode = "write-negative"
		}
		source := filepath.Join(t.TempDir(), "independent.step")
		original := run(mode, source)
		run("read", source)
		var out bytes.Buffer
		if e := WriteExactCircularLoopExtrusion(ctx, &out, exactLoopSTEPFixture(t, negative)); e != nil {
			t.Fatal(e)
		}
		decoded, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(out.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var envelope bytes.Buffer
		if e = WriteExactCircularLoopExtrusionJSON(ctx, &envelope, decoded); e != nil {
			t.Fatal(e)
		}
		decoded, e = ReadExactCircularLoopExtrusionJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		out.Reset()
		if e = WriteExactCircularLoopExtrusion(ctx, &out, decoded.Descriptor); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native.step")
		if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		got := run("read", path)
		for j := range got.Centroid {
			if math.Abs(got.Centroid[j]-original.Centroid[j]) > 1e-9 {
				t.Fatal("posed centroid", got, original)
			}
		}
	}
}

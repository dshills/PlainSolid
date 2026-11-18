package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestIndependentCylinderSector(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/cylinder-sector.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-4*math.Pi) > 1e-7 || math.Abs(p.Area-(6*math.Pi+16)) > 1e-7 {
		t.Fatal(p, e)
	}
}

func TestCylinderSectorRoundtrip(t *testing.T) {
	t.Parallel()
	frame, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, angle := range []float64{math.Pi / 2, 1.3 * math.Pi} {
		var out bytes.Buffer
		if e := WriteCylinderSectors(&out, CylinderSectorModel{[]CylinderSectorBody{{"sector", frame, 2, 4, angle}}, .001}); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(out.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-8*angle) > 1e-7 || math.Abs(p.Area-(12*angle+16)) > 1e-7 {
			t.Fatal(p, e)
		}
	}
}
func TestOCCTCylinderSector(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, path, "write-step-cylinder-sector")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{}); e != nil {
		t.Fatal(e)
	}
	frame, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, angle := range []float64{math.Pi / 2, 1.3 * math.Pi} {
		var b bytes.Buffer
		if e := WriteCylinderSectors(&b, CylinderSectorModel{[]CylinderSectorBody{{"sector", frame, 2, 4, angle}}, .001}); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "ours.step")
		if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		if e := json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-8*angle) > 1e-6 || math.Abs(p.Area-(12*angle+16)) > 1e-6 || math.Abs(p.Centroid[0]-(1+4*math.Sin(angle)/(3*angle))) > 1e-8 || math.Abs(p.Centroid[1]-(2+4*(1-math.Cos(angle))/(3*angle))) > 1e-8 || math.Abs(p.Centroid[2]-5) > 1e-8 {
			t.Fatal(p, e)
		}
	}
}

func TestCylinderSectorMalformed(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/cylinder-sector.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"side-orientation", "radial-orientation", "scale", "uv-phase", "missing-edge", "axis-endpoint"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			// Entity identifiers belong to the independently authored pinned fixture.
			mutate := func(id int64, fn func(*Record)) { ent := f.Entities[id]; fn(&ent.Records[0]); f.Entities[id] = ent }
			switch kind {
			case "side-orientation":
				mutate(17, func(r *Record) { r.Args[3].Text = "F" })
			case "radial-orientation":
				mutate(262, func(r *Record) { r.Args[3].Text = "T" })
			case "scale":
				mutate(63, func(r *Record) { r.Args[2] = Value{Kind: Number, Text: "2."} })
			case "uv-phase":
				mutate(123, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "1."} })
			case "missing-edge":
				mutate(237, func(r *Record) { r.Args[1].Items = r.Args[1].Items[:3] })
			case "axis-endpoint":
				mutate(242, func(r *Record) { r.Args[2] = Value{Kind: Reference, Ref: 22} })
			}
			if _, e := analyticBody(f, 15); e == nil {
				t.Fatal("accepted invalid graph")
			}
		})
	}
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, angle := range []float64{0, 2 * math.Pi, math.NaN(), -1} {
		var out bytes.Buffer
		if e := WriteCylinderSectors(&out, CylinderSectorModel{[]CylinderSectorBody{{"", frame, 2, 4, angle}}, .001}); e == nil || out.Len() != 0 {
			t.Fatal(angle, e)
		}
	}
	if e := WriteCylinderSectors(shortWriter{}, CylinderSectorModel{[]CylinderSectorBody{{"", frame, 2, 4, 1}}, .001}); e == nil {
		t.Fatal("short writer accepted")
	}
}
func FuzzCylinderSectorSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/cylinder-sector.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 4, MaxEdgeUses: 2048})
	})
}

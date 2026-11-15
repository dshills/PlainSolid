package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

const planarPrismIGESFixture = "testdata/occt-planar-prism-hole.igs"

func checkPlanarPrismIGESTransfer(t *testing.T, src []byte) PlanarSolid {
	t.Helper()
	ctx := context.Background()
	m, err := ReadPlanarSolid(ctx, bytes.NewReader(src), PlanarSolidOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if top := m.Solid.Topology(); top.Vertices != 20 || top.Edges != 30 || top.Faces != 12 || top.Shells != 1 ||
		m.LengthUnit != .001 || len(m.SourceEdges) != 30 || len(m.SourceFaces) != 12 {
		t.Fatalf("original topology/ancestry not preserved: %+v", top)
	}
	loc, err := collision.NewPointLocator(ctx, m.Solid, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		p    geom.Vec3
		want collision.Location
	}{
		{geom.Vec3{X: .5, Y: .5, Z: 1}, collision.Inside},
		{geom.Vec3{X: 1.5, Y: 1.5, Z: 1}, collision.Outside},
		{geom.Vec3{X: 1, Y: 1.5, Z: 1}, collision.OnBoundary},
		{geom.Vec3{X: 1, Y: 4.5, Z: 1}, collision.Outside},
	} {
		got, err := loc.Locate(ctx, q.p)
		if err != nil || got.Location != q.want {
			t.Fatalf("point %v: %+v %v", q.p, got, err)
		}
	}
	p, err := loc.MassPropertiesExact(ctx, collision.ExactMassOptions{})
	if err != nil || p.Volume.RatString() != "50" ||
		p.Centroid[0].RatString() != "161/50" || p.Centroid[1].RatString() != "111/50" || p.Centroid[2].RatString() != "1" {
		t.Fatalf("original material measures: %+v %v", p, err)
	}
	return m
}

func TestPlanarSolidIGESOriginalAndCanonical(t *testing.T) {
	raw, err := os.ReadFile(planarPrismIGESFixture)
	if err != nil {
		t.Fatal(err)
	}
	m := checkPlanarPrismIGESTransfer(t, raw)
	if f := m.SourceFaces[10]; f.FaceID != 189 || f.PlaneID != 191 || len(f.LoopIDs) != 2 || f.LoopIDs[0] != 199 || f.LoopIDs[1] != 201 {
		t.Fatalf("lost independently authored hole identity: %+v", f)
	}
	ctx := context.Background()
	var out, repeated bytes.Buffer
	if err := WritePlanarSolid(ctx, &out, m.Solid, m.LengthUnit, PlanarSolidOptions{}); err != nil {
		t.Fatal(err)
	}
	reopened := checkPlanarPrismIGESTransfer(t, out.Bytes())
	if err := WritePlanarSolid(ctx, &repeated, reopened.Solid, reopened.LengthUnit, PlanarSolidOptions{}); err != nil || !bytes.Equal(out.Bytes(), repeated.Bytes()) {
		t.Fatalf("canonical output is not byte stable: %v", err)
	}
	s := m.Solid
	v := s.Vertices()
	for i, p := range v {
		v[i] = geom.Vec3{X: p.X + 2*p.Y, Y: 3*p.X + 7*p.Y, Z: 2*p.X - p.Y + p.Z}
	}
	s, err = brep.NewSolid(v, s.Edges(), s.Faces())
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := WritePlanarSolid(ctx, &out, s, .0254, PlanarSolidOptions{}); err != nil {
		t.Fatalf("exact oblique support and inch coordinate transfer: %v", err)
	}
	oblique, err := ReadPlanarSolid(ctx, &out, PlanarSolidOptions{})
	if err != nil || oblique.LengthUnit != .0254 {
		t.Fatalf("oblique reopen: %v", err)
	}
	p, err := collision.MeasureExactPlanarMass(ctx, oblique.Solid, collision.LocationOptions{}, collision.ExactMassOptions{})
	if err != nil || p.Volume.RatString() != "50" {
		t.Fatalf("determinant-one oblique transfer changed volume: %+v %v", p, err)
	}
}

func TestPlanarSolidIGESRejectsChangedSource(t *testing.T) {
	raw, err := os.ReadFile(planarPrismIGESFixture)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, c := range []struct {
		name  string
		id    int
		index int
		text  string
	}{
		{"rounded-vertex", 21, 8, "6.00000000000000001"},
		{"rounded-line", 19, 6, "2.00000000000000001"},
		{"shifted-plane", 9, 2, "1.E-18"},
		{"wrong-face-sense", 3, 3, "0"},
		{"wrong-shell-sense", 1, 2, "1"},
		{"wrong-hole-owner", 189, 5, "199"},
		{"broken-loop", 15, 5, "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mutant := makePlanarIGESMutation(t, raw, c.id, c.index, c.text)
			if m, err := ReadPlanarSolid(ctx, bytes.NewReader(mutant), PlanarSolidOptions{}); err == nil || m.SolidID != 0 || m.SourceFaces != nil {
				t.Fatalf("mutated original yielded a partial solid: %+v %v", m, err)
			}
		})
	}
	m := checkPlanarPrismIGESTransfer(t, raw)
	var partial bytes.Buffer
	o := PlanarSolidOptions{Validation: collision.LocationOptions{MaxTriangles: 1}}
	if err := WritePlanarSolid(ctx, &partial, m.Solid, m.LengthUnit, o); err == nil || partial.Len() != 0 {
		t.Fatalf("writer published an unqualified prefix: %v", err)
	}
}

func makePlanarIGESMutation(t *testing.T, raw []byte, id, index int, value string) []byte {
	t.Helper()
	return editPlanarIGESParameters(t, raw, func(r topologyRecord) bool {
		if r.id != id {
			return false
		}
		if r.values[index].text == value {
			t.Fatal("mutation would not change original input")
		}
		r.values[index].text = value
		return true
	})
}

func editPlanarIGESParameters(t *testing.T, raw []byte, edit func(topologyRecord) bool) []byte {
	t.Helper()
	d, err := readPlanarTopologySource(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	a := topologyAuthor{geometry: Model{Entities: []Entity{{Point: &geom.Vec3{}}}}}
	changed := false
	for _, r := range d.entities {
		if edit(r) {
			changed = true
		}
		p := make([]string, len(r.values))
		for i, v := range r.values {
			p[i] = v.text
		}
		form := 0
		if r.kind >= 500 || r.kind == 190 || r.kind == 402 {
			form = 1
		}
		a.add(r.kind, form, joinTopologyValues(p))
	}
	if !changed {
		t.Fatal("mutation target absent")
	}
	mutant, err := a.file(d.unit)
	if err != nil {
		t.Fatal(err)
	}
	return mutant
}

func FuzzPlanarSolidIGES(f *testing.F) {
	for _, name := range []string{planarPrismIGESFixture, "testdata/occt-planar-material-void.igs"} {
		src, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(src)
	}
	f.Add([]byte("incomplete IGES"))
	f.Fuzz(func(t *testing.T, data []byte) {
		o := PlanarSolidOptions{
			Limits:     Limits{MaxBytes: 65536, MaxRecords: 1024, MaxEntities: 256, MaxValues: 4096},
			Validation: collision.LocationOptions{MaxTriangles: 256, MaxWork: 50000},
		}
		m, err := ReadPlanarSolid(context.Background(), bytes.NewReader(data), o)
		if err == nil && (m.SolidID == 0 || len(m.SourceShells) == 0 || m.Solid.Topology().Shells != len(m.SourceShells) || m.Solid.Validate() != nil) {
			t.Fatal("successful read did not produce validated original shells")
		}
	})
}

func TestPlanarSolidIGESLiveOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PLANAR_PRISM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT planar IGES author and reader")
	}
	dir := t.TempDir()
	original, canonical := filepath.Join(dir, "original.igs"), filepath.Join(dir, "canonical.igs")
	if data, err := exec.Command(oracle, "write", original).CombinedOutput(); err != nil {
		t.Fatalf("OCCT writer: %v %s", err, data)
	}
	src, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	m := checkPlanarPrismIGESTransfer(t, src)
	var out bytes.Buffer
	if err := WritePlanarSolid(context.Background(), &out, m.Solid, m.LengthUnit, PlanarSolidOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonical, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{original, canonical} {
		data, err := exec.Command(oracle, "read", file).CombinedOutput()
		if err != nil {
			t.Fatalf("OCCT reader: %v %s", err, data)
		}
		var have struct {
			Solids, Faces int
			Volume, Area  float64
			Centroid      [3]float64
		}
		at := bytes.LastIndexByte(data, '{')
		if at < 0 || json.Unmarshal(data[at:], &have) != nil || have.Solids != 1 || have.Faces != 12 ||
			math.Abs(have.Volume-50) > 1e-9 || math.Abs(have.Area-102) > 1e-9 ||
			math.Abs(have.Centroid[0]-3.22) > 1e-9 || math.Abs(have.Centroid[1]-2.22) > 1e-9 || math.Abs(have.Centroid[2]-1) > 1e-9 {
			t.Fatalf("independent reader measures: %s", data)
		}
	}
}

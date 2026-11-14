package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestPlacedSphericalMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/placed-spherical-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPlacedSphericalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	center := geom.Vec3{X: 10, Y: -3, Z: 5}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-32*math.Pi/3) > 1e-7 || math.Abs(mass.Area-16*math.Pi) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
		t.Fatal(mass, e)
	}
	proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Cmp(new(big.Rat).SetFloat64(1e-18)) > 0 {
			t.Fatal("numerical world agreement bound")
		}
	}
	surface := m.Solid.Faces()[0].Surface
	for _, v := range []float64{-math.Pi / 2, -.7, 0, .9, math.Pi / 2} {
		a, e := surface.Eval(0, v)
		if e != nil {
			t.Fatal(e)
		}
		b, e := surface.Eval(2*math.Pi, v)
		if e != nil || a.Distance(b) > 1e-12 || math.Abs(a.Distance(center)-2) > 1e-12 {
			t.Fatal("posed seam/radius", a, b, e)
		}
	}
	verts := m.Solid.Vertices()
	if len(verts) != 2 || verts[0] != (geom.Vec3{X: 8, Y: -3, Z: 5}) || verts[1] != (geom.Vec3{X: 12, Y: -3, Z: 5}) {
		t.Fatal("posed poles", verts)
	}
	var plain bytes.Buffer
	if e = trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	badUV := append([]byte(nil), raw...)
	for _, line := range bytes.Split(badUV, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			id, _ := field(string(line[73:]))
			if id == 19 {
				copy(line[48:56], "      33")
			}
		}
	}
	if _, e = ReadPlacedSphericalMaterial(ctx, bytes.NewReader(badUV), Limits{}, 1e-9); e == nil {
		t.Fatal("world transform on UV accepted")
	}
	var native, retained, output bytes.Buffer
	if e = WritePlacedSphericalMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadPlacedSphericalMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WritePlacedSphericalSource(ctx, &retained, p); e != nil || !bytes.Equal(retained.Bytes(), raw) {
		t.Fatal("source changed", e)
	}
	if e = WritePlacedSpherical(ctx, &output, p.Local.Descriptor, p.Placement, p.Local.LengthUnit, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSphericalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("unplaced widened")
	}
	for _, kind := range []int{100, 196} {
		bad := append([]byte(nil), raw...)
		changed := false
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) != 80 || line[72] != 'D' {
				continue
			}
			k, _ := field(string(line[:8]))
			id, _ := field(string(line[73:]))
			if k == kind && id%2 == 1 {
				copy(line[48:56], "       0")
				changed = true
				break
			}
		}
		if !changed {
			t.Fatal("no leaf")
		}
		if _, e = ReadPlacedSphericalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("unplaced source leaf")
		}
	}
	for _, mutation := range []struct {
		prefix string
		field  int
		value  string
	}{{"124,", 1, "1"}, {"124,", 4, "11"}, {"502,", 2, "8"}, {"196,", 2, "44"}} {
		bad := append([]byte(nil), raw...)
		changed := false
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(mutation.prefix)) {
				continue
			}
			v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
			v[mutation.field] = mutation.value
			text := strings.Join(v, ",")
			if len(text) > 64 {
				t.Fatal("overflow")
			}
			copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
			changed = true
			break
		}
		if !changed {
			t.Fatal("missing mutation")
		}
		if _, e = ReadPlacedSphericalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("wrong pose vertex angle")
		}
	}
	var q placedSphericalEnvelope
	if e = json.Unmarshal(native.Bytes(), &q); e != nil {
		t.Fatal(e)
	}
	q.Placement[0][3]++
	bad, _ := json.Marshal(q)
	if _, e = ReadPlacedSphericalMaterialJSON(ctx, bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("forged pose")
	}
	flipped := geom.Affine{{1, 0, 0, 8}, {0, -1, 0, 2}, {0, 0, -1, -1}}
	var flip bytes.Buffer
	if e = WritePlacedSpherical(ctx, &flip, m.Local.Descriptor, flipped, .001, 1e-9); e != nil {
		t.Fatal(e)
	}
	f, e := ReadPlacedSphericalMaterial(ctx, &flip, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	fm, e := f.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || fm.Centroid.Distance(geom.Vec3{X: 8, Y: 2, Z: -1}) > 1e-8 {
		t.Fatal(fm, e)
	}
	for _, l := range []Limits{{MaxEntities: 16}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadPlacedSphericalMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	for _, pose := range []geom.Affine{
		{{-1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}},
		{{0.6, -0.8, 0, 0}, {0.8, 0.6, 0, 0}, {0, 0, 1, 0}},
		{{1, 0, 0, 1e20}, {0, 1, 0, 0}, {0, 0, 1, 0}},
	} {
		var dst bytes.Buffer
		if e := WritePlacedSpherical(ctx, &dst, m.Local.Descriptor, pose, 1, 1e-9); e == nil || dst.Len() != 0 {
			t.Fatal("unsupported or lossy pose accepted", e)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
		for _, b := range [][]byte{raw, output.Bytes()} {
			path := filepath.Join(t.TempDir(), "posed.igs")
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, path).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", data, e)
			}
			var q struct {
				Faces, Solids int
				Area, Volume  float64
				Center        [3]float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Faces != 1 || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-7 || math.Abs(q.Area-mass.Area) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
				t.Fatalf("%s", data)
			}
		}
	}
}
func FuzzPlacedSpherical(f *testing.F) {
	b, e := os.ReadFile("testdata/placed-spherical-material.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadPlacedSphericalMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}

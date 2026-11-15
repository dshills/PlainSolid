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
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func checkPlanarIGESVoids(t *testing.T, raw []byte, n, volume int) PlanarSolid {
	t.Helper()
	ctx := context.Background()
	m, err := ReadPlanarSolid(ctx, bytes.NewReader(raw), PlanarSolidOptions{})
	if err != nil {
		t.Fatal(err)
	}
	top := m.Solid.Topology()
	if top.Shells != n+1 || top.Faces != 6*(n+1) || len(m.SourceShells) != n+1 {
		t.Fatalf("wrong source shell count: %+v %+v", top, m.SourceShells)
	}
	for i, shell := range m.SourceShells {
		if shell.ShellID < 1 || shell.FaceStart != 6*i || shell.FaceEnd != 6*(i+1) || shell.Void != (i > 0) {
			t.Fatalf("wrong original shell owner %d: %+v", i, shell)
		}
	}
	loc, err := collision.NewPointLocator(ctx, m.Solid, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cavity := geom.Vec3{X: 4, Y: 4, Z: 4}
	if n == 2 {
		cavity = geom.Vec3{X: 2, Y: 2, Z: 2}
	}
	for _, trial := range []struct {
		p    geom.Vec3
		want collision.Location
	}{
		{cavity, collision.Outside},
		{geom.Vec3{X: .5, Y: .5, Z: .5}, collision.Inside},
		{geom.Vec3{X: 0, Y: 4, Z: 4}, collision.OnBoundary},
	} {
		have, err := loc.Locate(ctx, trial.p)
		if err != nil || have.Location != trial.want {
			t.Fatalf("planar material query for %v: %+v %v", trial.p, have, err)
		}
	}
	mass, err := loc.MassPropertiesExact(ctx, collision.ExactMassOptions{})
	if err != nil || mass.Volume.RatString() != strconv.Itoa(volume) {
		t.Fatalf("exact cavity material volume: %+v %v", mass, err)
	}
	for _, c := range mass.Centroid {
		if c.RatString() != "4" {
			t.Fatalf("symmetrical cavity stock lost its exact centroid: %+v", mass.Centroid)
		}
	}
	return m
}

func TestPlanarSolidIGESVoidsOriginalAndCanonical(t *testing.T) {
	raw, err := os.ReadFile("testdata/occt-planar-material-void.igs")
	if err != nil {
		t.Fatal(err)
	}
	m := checkPlanarIGESVoids(t, raw, 1, 448)
	if m.SolidID != 3 || m.SourceShells[0].ShellID != 5 || m.SourceShells[1].ShellID != 131 {
		t.Fatalf("detached original cavity ownership: %+v", m.SourceShells)
	}
	faces := m.Solid.Faces()
	var reordered []brep.Face
	for i := range 6 {
		reordered = append(reordered, faces[i+6], faces[i])
	}
	s, err := brep.NewSolid(m.Solid.Vertices(), m.Solid.Edges(), reordered)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var out, again bytes.Buffer
	if err := WritePlanarSolid(ctx, &out, s, m.LengthUnit, PlanarSolidOptions{}); err != nil {
		t.Fatal(err)
	}
	reopened := checkPlanarIGESVoids(t, out.Bytes(), 1, 448)
	if err := WritePlanarSolid(ctx, &again, reopened.Solid, reopened.LengthUnit, PlanarSolidOptions{}); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatalf("canonical reordered native shell has unstable bytes: %v", err)
	}
	for _, c := range []struct {
		name  string
		id    int
		index int
		value string
	}{
		{"wrong-association-target", 1, 2, "5"},
		{"duplicated-shell", 3, 4, "5"},
		{"inverted-void", 3, 5, "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mutant := makePlanarIGESMutation(t, raw, c.id, c.index, c.value)
			got, err := ReadPlanarSolid(ctx, bytes.NewReader(mutant), PlanarSolidOptions{})
			if err == nil || got.SolidID != 0 || got.SourceShells != nil {
				t.Fatalf("incorrect cavity ownership yielded a solid: %+v %v", got, err)
			}
		})
	}
}

func TestPlanarSolidIGESVoidsLiveOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT IGES cavity author and reader")
	}
	for _, c := range []struct {
		mode            string
		n, volume, area int
	}{{"write-void", 1, 448, 480}, {"write-two-voids", 2, 496, 432}} {
		t.Run(c.mode, func(t *testing.T) {
			dir := t.TempDir()
			a, b := filepath.Join(dir, "original.igs"), filepath.Join(dir, "canonical.igs")
			if data, err := exec.Command(oracle, c.mode, a).CombinedOutput(); err != nil {
				t.Fatalf("OCCT writer: %v %s", err, data)
			}
			raw, err := os.ReadFile(a)
			if err != nil {
				t.Fatal(err)
			}
			m := checkPlanarIGESVoids(t, raw, c.n, c.volume)
			var out bytes.Buffer
			if err := WritePlanarSolid(context.Background(), &out, m.Solid, m.LengthUnit, PlanarSolidOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(b, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			checkPlanarIGESVoids(t, out.Bytes(), c.n, c.volume)
			for _, f := range []string{a, b} {
				data, err := exec.Command(oracle, "read", f).CombinedOutput()
				if err != nil {
					t.Fatalf("OCCT reader: %v %s", err, data)
				}
				var have struct {
					Solids, Shells, Faces int
					Volume, Area          float64
					Centroid              [3]float64
				}
				i := bytes.LastIndexByte(data, '{')
				if i < 0 || json.Unmarshal(data[i:], &have) != nil || have.Solids != 1 || have.Shells != c.n+1 ||
					have.Faces != 6*(c.n+1) || math.Abs(have.Volume-float64(c.volume)) > 1e-9 || math.Abs(have.Area-float64(c.area)) > 1e-9 ||
					math.Abs(have.Centroid[0]-4) > 1e-9 || math.Abs(have.Centroid[1]-4) > 1e-9 || math.Abs(have.Centroid[2]-4) > 1e-9 {
					t.Fatalf("independent reader measures: %s", data)
				}
			}
		})
	}
}

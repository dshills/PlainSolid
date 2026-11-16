package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestReadEccentricAnnularChamferIGES(t *testing.T) {
	ctx, opts := context.Background(), geom.SurfacePatchOptions{}
	raw := readEccentricAnnularChamferIGESFixture(t, "exact")
	m, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(raw), Limits{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	top := m.Solid.Topology()
	if m.LengthUnit != .001 || !bytes.Equal(m.SourceIGES(), raw) ||
		top.Vertices != 32 || top.Edges != 38 || top.Faces != 8 || top.InnerWires != 2 || len(top.Shells) != 1 || top.EulerCharacteristic != 0 {
		t.Fatalf("source unit or topology changed: %g %v", m.LengthUnit, top)
	}
	sourceCopy := m.SourceIGES()
	sourceCopy[0]++
	if !bytes.Equal(m.SourceIGES(), raw) {
		t.Fatal("result source bytes alias caller storage")
	}
	if m.SourceGroupID != 1 || m.SourceSolidID != 3 || m.SourceShellID != 5 || m.SourceEdgeListID != 13 || m.SourceVertexListID != 17 ||
		!reflect.DeepEqual(m.NativeFaceSourceIDs, []int{61, 7, 67, 73, 87, 101, 113, 107}) {
		t.Fatalf("wrong original body and face owners: %v", m.NativeFaceSourceIDs)
	}
	source := [8]geom.Vec3{{X: 5.5, Z: 4}, {X: 6.75, Z: 9}, {X: 4.75, Z: 2}, {X: 6.5, Z: 10},
		{X: 3.6875, Y: 2, Z: 2}, {X: 4.875, Y: 2, Z: 10}, {X: 3.625, Y: 2, Z: 3}, {X: 4.25, Y: 2, Z: 8}}
	native := m.Solid.Vertices()
	seen := map[int]bool{}
	for i, index := range m.NativeVertexSourceIndices {
		if index == 0 {
			continue
		}
		if index < 1 || index > len(source) || seen[index] || native[i] != source[index-1] {
			t.Fatalf("moved or duplicated source vertex %d", index)
		}
		seen[index] = true
	}
	if len(seen) != 8 {
		t.Fatal("some original vertices were lost")
	}
	curveForEdge := [14]int{15, 19, 23, 27, 29, 33, 35, 39, 43, 47, 49, 53, 55, 59}
	counts := map[int]int{}
	for i, index := range m.NativeEdgeSourceIndices {
		if index < 1 || index > 14 || m.NativeCurveSourceIDs[i] != curveForEdge[index-1] {
			t.Fatalf("changed original curve for edge %d: %v", index, m.NativeCurveSourceIDs)
		}
		counts[index]++
	}
	for _, i := range []int{2, 3, 5, 7, 8, 9, 11, 13} {
		if counts[i] != 4 {
			t.Fatalf("full source circle %d lost its four native quadrants", i)
		}
	}
	for _, i := range []int{1, 4, 6, 10, 12, 14} {
		if counts[i] != 1 {
			t.Fatalf("original meridian %d did not retain one native edge", i)
		}
	}
	mass, err := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-726.98826561019746) > 1e-7 || math.Abs(mass.Area-593.89732690669496) > 1e-7 {
		t.Fatalf("native signed material differs from the standalone author: %v %v", mass, err)
	}
	var saved, canonical, repeated bytes.Buffer
	if err := trimmed.WriteJSON(&saved, m.Solid); err != nil {
		t.Fatal(err)
	}
	replay, err := trimmed.ReadJSONContext(ctx, &saved, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteEccentricAnnularChamfer(ctx, &canonical, m.Solid, m.LengthUnit, opts); err != nil {
		t.Fatal(err)
	}
	if err := WriteEccentricAnnularChamfer(ctx, &repeated, replay, m.LengthUnit, opts); err != nil ||
		!bytes.Equal(canonical.Bytes(), repeated.Bytes()) {
		t.Fatalf("native replay changed canonical IGES bytes: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IGES_IMPORT_READER"); oracle != "" {
		verifyEccentricAnnularChamferIGESConsumer(t, oracle, raw, source[:], 14, mass, "--full-rims")
		verifyEccentricAnnularChamferIGESConsumer(t, oracle, canonical.Bytes(), native, 38, mass)
	}
}

func TestReadEccentricAnnularChamferIGESReject(t *testing.T) {
	ctx, opts := context.Background(), geom.SurfacePatchOptions{}
	exact := readEccentricAnnularChamferIGESFixture(t, "exact")
	reject := func(t *testing.T, raw []byte, l Limits) {
		t.Helper()
		m, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(raw), l, opts)
		if err == nil || len(m.SourceIGES()) != 0 || m.Solid.Topology().Faces != 0 {
			t.Fatalf("accepted unproved input or returned partial material: %v", err)
		}
	}
	reject(t, readEccentricAnnularChamferIGESFixture(t, "rounded"), Limits{})
	reject(t, exact, Limits{MaxBytes: int64(len(exact) - 1)})
	reject(t, exact, Limits{MaxControlPoints: 107})
	reject(t, exact, Limits{MaxEntities: 58})
	for _, c := range []struct {
		name              string
		entity, parameter int
		prior, value      byte
	}{
		{"wrong group member", 1, 2, '3', '5'},
		{"inward original cap", 5, 9, '0', '1'},
		{"same direction on both seam uses", 11, 15, '0', '1'},
		{"meridian endpoint moved off support", 15, 2, '0', '1'},
		{"different full rim phase", 19, 5, '0', '1'},
		{"scaled rim transform", 21, 1, '1', '2'},
		{"changed affine knot", 9, 24, '1', '2'},
		{"changed homogeneous weight", 9, 27, '1', '2'},
		{"changed rational face pole", 9, 45, '0', '1'},
	} {
		t.Run(c.name, func(t *testing.T) {
			raw := bytes.Clone(exact)
			changeBoreRimIGESDigit(t, raw, c.entity, c.parameter, c.prior, c.value)
			reject(t, raw, Limits{})
		})
	}
	t.Run("original rounded seam with exact supports", func(t *testing.T) {
		raw := bytes.Clone(exact)
		// Retain the exact-net fixture's owner and sequence fields. Restore
		// the first meridian endpoint from OCCT's default angle export.
		before := []byte("110,5.5,0.,4.,6.75,0.,9.;")
		after := []byte("110,5.5,-1.347111479E-15,4.,6.75,0.,9.;")
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, before) {
				continue
			}
			if owner, err := field(string(line[64:72])); err != nil || owner != 15 {
				t.Fatalf("unexpected source meridian owner %d", owner)
			}
			copy(line, after)
			reject(t, raw, Limits{})
			return
		}
		t.Fatal("original stock meridian record was absent")
	})
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if m, err := ReadEccentricAnnularChamfer(canceled, bytes.NewReader(exact), Limits{}, opts); !errors.Is(err, context.Canceled) || len(m.SourceIGES()) != 0 {
		t.Fatalf("canceled read returned material: %v", err)
	}
}

func TestEccentricAnnularChamferPinnedIGESAuthor(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_ANNULAR_IGES_EXACT_AUTHOR")
	if author == "" {
		t.Skip("pinned OCCT exact IGES author not configured")
	}
	dir := t.TempDir()
	exact, rounded := filepath.Join(dir, "exact.igs"), filepath.Join(dir, "rounded.igs")
	if out, err := exec.Command(author, exact, rounded).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	directory := func(raw []byte) []byte {
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			if len(line) == 80 && line[72] == 'D' {
				return raw[bytes.Index(raw, line):]
			}
		}
		return nil
	}
	for name, p := range map[string]string{"exact": exact, "rounded": rounded} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		a, b := directory(got), directory(readEccentricAnnularChamferIGESFixture(t, name))
		if a == nil || b == nil || !bytes.Equal(a, b) {
			t.Fatalf("pinned author changed the %s directory or parameter records", name)
		}
	}
}

func readEccentricAnnularChamferIGESFixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-eccentric-annular-chamfer-" + kind + ".igs")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func verifyEccentricAnnularChamferIGESConsumer(t *testing.T, oracle string, data []byte, vertices []geom.Vec3,
	edges int, mass parametric.Properties, options ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "annular-chamfer.igs")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command(oracle, append([]string{path}, options...)...).CombinedOutput()
	var hit struct {
		Solids, Faces int
		Volume, Area  float64
		Vertices      [][3]float64
		Edges         [][3]int
		Caps          [][2]float64
		Bands         [][6]float64
	}
	start := bytes.Index(raw, []byte(`{"Solids":`))
	if err != nil || start < 0 || json.Unmarshal(raw[start:], &hit) != nil || hit.Solids != 1 || hit.Faces != 8 ||
		len(hit.Vertices) != len(vertices) || len(hit.Edges) != edges || hit.Volume <= 0 ||
		math.Abs(hit.Volume-mass.Volume) > 1e-8*mass.Volume || math.Abs(hit.Area-mass.Area) > 1e-8*mass.Area {
		t.Fatalf("pinned IGES consumer did not retain signed material: %v\n%s", err, raw)
	}
	exact := map[[3]float64]bool{}
	for _, p := range vertices {
		exact[[3]float64{p.X, p.Y, p.Z}] = true
	}
	for _, p := range hit.Vertices {
		if !exact[p] {
			t.Fatalf("pinned consumer moved or recreated vertex: %v", p)
		}
		delete(exact, p)
	}
	if !reflect.DeepEqual(hit.Caps, [][2]float64{{2, 2}, {10, 2}}) || len(hit.Bands) != 6 {
		t.Fatalf("pinned consumer changed a wall or annular cap: %v", hit)
	}
	remaining := [][6]float64{{0, 0, 2, 4.75, 4, 5.5}, {0, 0, 4, 5.5, 9, 6.75}, {0, 0, 9, 6.75, 10, 6.5},
		{1.5, 2, 2, 2.1875, 3, 2.125}, {1.5, 2, 3, 2.125, 8, 2.75}, {1.5, 2, 8, 2.75, 10, 3.375}}
	for _, band := range hit.Bands {
		match := -1
		for i, want := range remaining {
			ok := true
			for j := range want {
				ok = ok && math.Abs(band[j]-want[j]) < 1e-10
			}
			if ok {
				match = i
				break
			}
		}
		if match < 0 {
			t.Fatalf("pinned consumer changed an exact wall profile: %v", band)
		}
		remaining = append(remaining[:match], remaining[match+1:]...)
	}
}

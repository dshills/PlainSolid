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
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestReadConicalRimFilletBoreIGES(t *testing.T) {
	ctx := context.Background()
	raw := conicalRimFilletBoreIGESFixture(t, "exact")
	p, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(raw), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	top := p.Solid.Topology()
	if p.LengthUnit != .001 || !bytes.Equal(raw, p.SourceIGES()) || top.Vertices != 24 || top.Edges != 28 ||
		top.Faces != 6 || top.InnerWires != 2 || len(top.Shells) != 1 || top.EulerCharacteristic != 0 {
		t.Fatalf("wrong source unit or native topology: %g %v", p.LengthUnit, top)
	}
	original := p.SourceIGES()
	original[0] ^= 1
	if !bytes.Equal(raw, p.SourceIGES()) {
		t.Fatal("source bytes alias returned storage")
	}
	if p.SourceGroupID != 1 || p.SourceSolidID != 3 || p.SourceShellID != 5 || p.SourceEdgeListID != 13 || p.SourceVertexListID != 17 ||
		!reflect.DeepEqual(p.NativeFaceSourceIDs, []int{7, 53, 77, 65, 91, 105}) {
		t.Fatalf("wrong original body and face owners: %v", p.NativeFaceSourceIDs)
	}
	sourceVertices := [6]geom.Vec3{{X: 8.25, Y: 2, Z: .1875}, {X: 11.5, Y: 2, Z: 2.625}, {X: 7.6875, Y: 2},
		{X: 10.375, Y: 2, Z: 6}, {X: 2.5, Y: 4}, {X: 2.5, Y: 4, Z: 6}}
	seen := map[int]bool{}
	for i, v := range p.Solid.Vertices() {
		id := p.NativeVertexSourceIndices[i]
		if id == 0 {
			continue
		}
		if id < 1 || id > len(sourceVertices) || seen[id] || v != sourceVertices[id-1] {
			t.Fatalf("lost or moved original vertex %d", id)
		}
		seen[id] = true
	}
	if len(seen) != 6 {
		t.Fatal("lost original vertex ownership")
	}
	sourceCurves := [10]int{15, 19, 23, 27, 31, 35, 39, 43, 47, 51}
	counts := map[int]int{}
	for i, id := range p.NativeEdgeSourceIndices {
		if id < 1 || id > len(sourceCurves) || p.NativeCurveSourceIDs[i] != sourceCurves[id-1] {
			t.Fatalf("lost original edge ownership for native edge %d", i)
		}
		counts[id]++
	}
	for _, id := range []int{2, 3, 5, 7, 8, 9} {
		if counts[id] != 4 {
			t.Fatalf("full original rim %d lost its native quadrants", id)
		}
	}
	for _, id := range []int{1, 4, 6, 10} {
		if counts[id] != 1 {
			t.Fatalf("original meridian or straight seam %d was changed", id)
		}
	}
	mass, err := p.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-2282.4014009763955) > 1e-7 || math.Abs(mass.Area-1182.6440976399522) > 1e-7 {
		t.Fatalf("native body differs from independent author: %v %v", mass, err)
	}
	var first, second bytes.Buffer
	if err := trimmed.WriteJSON(&first, p.Solid); err != nil {
		t.Fatal(err)
	}
	replayed, err := trimmed.ReadJSONContext(ctx, bytes.NewReader(first.Bytes()), trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := trimmed.WriteJSON(&second, replayed); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("native topology did not replay deterministically: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_IGES_READER"); oracle != "" {
		verifyConicalRimBoreIGESComparator(t, oracle, raw, sourceVertices, mass)
	}
}

func TestReadConicalRimFilletBoreIGESReject(t *testing.T) {
	slowtest.Skip(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	exact := conicalRimFilletBoreIGESFixture(t, "exact")
	raw := conicalRimFilletBoreIGESFixture(t, "rounded")
	if _, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(raw), Limits{}, o); err == nil {
		t.Fatal("existing circular annular reader unexpectedly accepted the actual default source")
	}
	reject := func(t *testing.T, source []byte, l Limits) {
		t.Helper()
		p, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(source), l, o)
		if err == nil || len(p.SourceIGES()) != 0 || p.Solid.Topology().Faces != 0 {
			t.Fatalf("accepted unproved input or returned partial material: %v", err)
		}
	}
	reject(t, raw, Limits{})
	reject(t, exact, Limits{MaxBytes: int64(len(exact) - 1)})
	reject(t, exact, Limits{MaxControlPoints: 17})
	reject(t, exact, Limits{MaxEntities: 57})
	for _, tc := range []struct {
		name              string
		entity, parameter int
		prior, value      byte
	}{
		{"wrong group member", 1, 2, '3', '5'},
		{"lower rim repeats a cone use", 63, 20, '0', '1'},
		{"moved original bore seam", 51, 2, '4', '5'},
		{"off support rolling arc", 27, 5, '0', '1'},
		{"changed cone weight", 9, 27, '1', '2'},
		{"changed toroid radius", 55, 3, '7', '8'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := bytes.Clone(exact)
			changeBoreRimIGESDigit(t, p, tc.entity, tc.parameter, tc.prior, tc.value)
			reject(t, p, Limits{})
		})
	}
	t.Run("original rounded meridian placement with exact cone", func(t *testing.T) {
		p := bytes.Clone(exact)
		before := []byte("124,1.,0.,0.,7.6875,-0.,0.,1.,2.,0.,-1.,0.,0.9375;")
		after := []byte("124,1.,0.,0.,7.6875,-5.551115123E-17,0.,1.,2.,0.,-1.,0.,0.9375;")
		for _, line := range bytes.Split(p, []byte{'\n'}) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, before) {
				continue
			}
			owner, err := field(string(line[64:72]))
			if err != nil || owner != 29 {
				t.Fatalf("unexpected original meridian transform directory ID %d", owner)
			}
			copy(line, after)
			reject(t, p, Limits{})
			return
		}
		t.Fatal("original near-cardinal matrix control was absent")
	})
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, err := ReadConicalRimFilletBore(canceled, bytes.NewReader(exact), Limits{}, o); !errors.Is(err, context.Canceled) || len(p.SourceIGES()) != 0 {
		t.Fatalf("canceled read returned material: %v", err)
	}
}

func TestConicalRimFilletBorePinnedIGESAuthor(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_IGES_AUTHOR")
	if author == "" {
		t.Skip("pinned OCCT conical rim author not configured")
	}
	dir := t.TempDir()
	exact, rounded := filepath.Join(dir, "exact.igs"), filepath.Join(dir, "rounded.igs")
	if out, err := exec.Command(author, exact, rounded).CombinedOutput(); err != nil {
		t.Fatalf("pinned independent author: %v\n%s", err, out)
	}
	directory := func(raw []byte) []byte {
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			if len(line) == 80 && line[72] == 'D' {
				return raw[bytes.Index(raw, line):]
			}
		}
		return nil
	}
	for kind, path := range map[string]string{"exact": exact, "rounded": rounded} {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		x, y := directory(generated), directory(conicalRimFilletBoreIGESFixture(t, kind))
		if x == nil || y == nil || !bytes.Equal(x, y) {
			t.Fatalf("pinned author changed the %s IGES graph", kind)
		}
	}
}

func conicalRimFilletBoreIGESFixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-conical-double-rim-eccentric-bore-" + kind + ".igs")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func verifyConicalRimBoreIGESComparator(t *testing.T, oracle string, source []byte, original [6]geom.Vec3, mass parametric.Properties) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "conical-rim-fillet.igs")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command(oracle, path).CombinedOutput()
	var hit struct {
		Volume, Area float64
		Vertices     [][3]float64
		Edges        [3]int
		Faces        [][]float64
	}
	start := bytes.Index(raw, []byte(`{"volume":`))
	if err != nil || start < 0 || json.Unmarshal(raw[start:], &hit) != nil || hit.Edges != [3]int{6, 2, 2} ||
		hit.Volume <= 0 || math.Abs(hit.Volume-mass.Volume) > 1e-8*mass.Volume || math.Abs(hit.Area-mass.Area) > 1e-8*mass.Area ||
		len(hit.Vertices) != len(original) || len(hit.Faces) != 6 {
		t.Fatalf("pinned IGES comparator failed signed transfer: %v\n%s", err, raw)
	}
	for i, v := range hit.Vertices {
		p := original[i]
		if v != [3]float64{p.X, p.Y, p.Z} {
			t.Fatalf("pinned IGES reopen changed original vertex: %v", v)
		}
	}
	want := [][]float64{{3, 0, 2, .1875, 8.25, 2.625, 11.5},
		{2, 0, 2, .9375, 7.6875, .9375, 0, .1875}, {2, 0, 2, 4.125, 10.375, 1.875, 2.625, 6},
		{0, 0, 2}, {0, 6, 2}, {1, 1.5, 4, 1, 0, 6}}
	for i, support := range want {
		if len(hit.Faces[i]) != len(support) {
			t.Fatalf("pinned IGES reopen changed source support %d: %v", i, hit.Faces[i])
		}
		for j, scalar := range support {
			if math.Abs(hit.Faces[i][j]-scalar) > 1e-10 {
				t.Fatalf("pinned IGES reopen changed source support %d: %v", i, hit.Faces[i])
			}
		}
	}
}

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestPlanarEdgeContactPairSTEP(t *testing.T) {
	ctx := context.Background()
	m := Model{LengthUnit: .001}
	for i, p := range []geom.Vec3{{}, {X: 1, Y: 1}} {
		s, err := brep.Box(p, p.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		if err != nil {
			t.Fatal(err)
		}
		m.Bodies = append(m.Bodies, Body{Name: []string{"Stock", "Neighbour"}[i], Solid: s})
	}
	var out bytes.Buffer
	if p, err := WriteEdgeContactPair(ctx, &out, m, EdgeContactSTEPOptions{}); err != nil || p.SignedVolume.RatString() != "2" {
		t.Fatalf("exact STEP edge-contact write: %+v %v", p, err)
	}
	got, p, err := ReadEdgeContactPair(ctx, bytes.NewReader(out.Bytes()), EdgeContactSTEPOptions{})
	if err != nil || len(got.Bodies) != 2 || p.Start.X != 1 || p.Start.Y != 1 || p.End.X != 1 || p.End.Y != 1 ||
		got.Bodies[0].Name != "Stock" || got.Bodies[1].Name != "Neighbour" {
		t.Fatalf("exact STEP readback changed owners: %+v %v", p, err)
	}
	if b, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), VertexContactSTEPOptions{}); err == nil || b.Bodies != nil || p.SignedVolume != nil {
		t.Fatalf("vertex-only STEP import claimed an edge: %v", err)
	}
}

func TestPlanarEdgeContactPairSTEPLiveOCCT(t *testing.T) {
	checkPlanarEdgePairSTEPLiveOCCT(t, false)
}

func TestPlanarEdgeIntervalContactPairSTEPLiveOCCT(t *testing.T) {
	checkPlanarEdgePairSTEPLiveOCCT(t, true)
}

func checkPlanarEdgePairSTEPLiveOCCT(t *testing.T, interval bool) {
	t.Helper()
	author, reader := os.Getenv("PLAINSOLID_OCCT_PLANAR_CONTACT_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT STEP author and reader")
	}
	dir := t.TempDir()
	upstream, native := filepath.Join(dir, "upstream-edge.step"), filepath.Join(dir, "native-edge.step")
	mode, volume, area := "edge", 2., 12.
	read, write := ReadEdgeContactPair, WriteEdgeContactPair
	if interval {
		mode, volume, area = "edge-interval", 6., 28.
		read, write = ReadEdgeIntervalContactPair, WriteEdgeIntervalContactPair
	}
	if data, err := exec.Command(author, mode, upstream).CombinedOutput(); err != nil {
		t.Fatalf("OCCT original edge contact: %v %s", err, data)
	}
	src, err := os.ReadFile(upstream)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, p, err := read(ctx, bytes.NewReader(src), EdgeContactSTEPOptions{})
	if err != nil || len(m.Bodies) != 2 || p.SignedVolume.Cmp(new(big.Rat).SetFloat64(volume)) != 0 {
		t.Fatalf("original OCCT body edges: %+v %v", p, err)
	}
	if interval {
		if min(p.Start.Z, p.End.Z) != 1 || max(p.Start.Z, p.End.Z) != 3 {
			t.Fatalf("the original partial edge interval changed: %+v", p)
		}
		if _, _, err := ReadEdgeContactPair(ctx, bytes.NewReader(src), EdgeContactSTEPOptions{}); err == nil {
			t.Fatal("whole-edge reader admitted the partial original contact")
		}
	}
	var out bytes.Buffer
	if _, err := write(ctx, &out, m, EdgeContactSTEPOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(native, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{upstream, native} {
		data, err := exec.Command(reader, "read", path).CombinedOutput()
		var p struct {
			Solids, Faces int
			Volume, Area  float64
		}
		if err != nil || json.Unmarshal(data, &p) != nil || p.Solids != 2 || p.Faces != 12 || math.Abs(p.Volume-volume) > 1e-9 || math.Abs(p.Area-area) > 1e-9 {
			t.Fatalf("independent reader lost edge-contact bodies: %v %s", err, data)
		}
	}
}

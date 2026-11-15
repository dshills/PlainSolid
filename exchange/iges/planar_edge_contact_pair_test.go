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
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestPlanarEdgeContactPairIGES(t *testing.T) {
	ctx := context.Background()
	var bodies [2]brep.Solid
	for i, p := range []geom.Vec3{{}, {X: 1, Y: 1}} {
		s, err := brep.Box(p, p.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		if err != nil {
			t.Fatal(err)
		}
		bodies[i] = s
	}
	var out bytes.Buffer
	if _, err := WritePlanarEdgeContactPair(ctx, &out, bodies, .001, EdgeContactIGESOptions{}); err != nil {
		t.Fatal(err)
	}
	m, err := ReadPlanarEdgeContactPair(ctx, bytes.NewReader(out.Bytes()), EdgeContactIGESOptions{})
	if err != nil || m.Contact.SignedVolume.RatString() != "2" || m.LengthUnit != .001 {
		t.Fatalf("exact IGES edge-contact readback: %+v %v", m.Contact, err)
	}
	for _, a := range m.Bodies[0].SourceVertexIndices {
		for _, b := range m.Bodies[1].SourceVertexIndices {
			if a == b {
				t.Fatalf("file welded the two contacts through vertex list entry %d", a)
			}
		}
	}
	if b, err := ReadPlanarVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), PlanarVertexContactAssemblyOptions{}); err == nil || b.Bodies != nil {
		t.Fatalf("vertex-only IGES proof admitted an edge: %v", err)
	}
}

func TestPlanarEdgeContactPairIGESLiveOCCT(t *testing.T) {
	checkPlanarEdgePairIGESLiveOCCT(t, false)
}

func TestPlanarEdgeIntervalContactPairIGESLiveOCCT(t *testing.T) {
	checkPlanarEdgePairIGESLiveOCCT(t, true)
}

func checkPlanarEdgePairIGESLiveOCCT(t *testing.T, interval bool) {
	t.Helper()
	oracle := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT IGES author and reader")
	}
	dir := t.TempDir()
	upstream, native := filepath.Join(dir, "upstream-edge.igs"), filepath.Join(dir, "native-edge.igs")
	mode, volume, area := "write-edge-pair", 2., 12.
	read, write := ReadPlanarEdgeContactPair, WritePlanarEdgeContactPair
	if interval {
		mode, volume, area = "write-edge-interval", 6., 28.
		read, write = ReadPlanarEdgeIntervalContactPair, WritePlanarEdgeIntervalContactPair
	}
	if data, err := exec.Command(oracle, mode, upstream).CombinedOutput(); err != nil {
		t.Fatalf("OCCT original edge contact: %v %s", err, data)
	}
	raw, err := os.ReadFile(upstream)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, err := read(ctx, bytes.NewReader(raw), EdgeContactIGESOptions{})
	if err != nil || m.Contact.SignedVolume.Cmp(new(big.Rat).SetFloat64(volume)) != 0 {
		t.Fatalf("exact OCCT IGES body edges: %+v %v", m.Contact, err)
	}
	if interval {
		if min(m.Contact.Start.Z, m.Contact.End.Z) != 1 || max(m.Contact.Start.Z, m.Contact.End.Z) != 3 {
			t.Fatalf("the original IGES partial edge interval changed: %+v", m.Contact)
		}
		if _, err := ReadPlanarEdgeContactPair(ctx, bytes.NewReader(raw), EdgeContactIGESOptions{}); err == nil {
			t.Fatal("whole-edge reader admitted the partial original IGES contact")
		}
	}
	var out bytes.Buffer
	parts := [2]brep.Solid{m.Bodies[0].Solid, m.Bodies[1].Solid}
	if _, err := write(ctx, &out, parts, m.LengthUnit, EdgeContactIGESOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(native, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{upstream, native} {
		data, err := exec.Command(oracle, "read", path).CombinedOutput()
		var p struct {
			Solids, Faces int
			Volume, Area  float64
		}
		i := bytes.LastIndexByte(data, '{')
		if err != nil || i < 0 || json.Unmarshal(data[i:], &p) != nil || p.Solids != 2 || p.Faces != 12 || math.Abs(p.Volume-volume) > 1e-9 || math.Abs(p.Area-area) > 1e-9 {
			t.Fatalf("independent reader lost edge-contact bodies: %v %s", err, data)
		}
	}
}

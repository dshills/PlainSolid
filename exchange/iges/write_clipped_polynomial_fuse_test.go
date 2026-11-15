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
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialFuse(t *testing.T) {
	ctx := context.Background()
	input, err := os.Open("../testdata/native_clipped_polynomial_fuse.json")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	fuse, err := trimmed.ReadClippedPolynomialFuseJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var a, b, refused bytes.Buffer
	opts := ClippedPolynomialFuseIGESOptions{PoleTolerance: 1e-14}
	for _, w := range []*bytes.Buffer{&a, &b} {
		if err := WriteClippedPolynomialFuse(ctx, w, fuse, .001, Limits{}, opts); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("nondeterministic IGES export")
	}
	graph, err := readTopologySource(bytes.NewReader(a.Bytes()), Limits{}, true)
	if err != nil || graph.unit != .001 {
		t.Fatalf("standard graph units: %g (%v)", graph.unit, err)
	}
	counts := map[int]int{}
	for _, e := range graph.entities {
		counts[e.kind]++
	}
	for kind, count := range map[int]int{126: 138, 128: 20, 186: 1, 502: 1, 504: 1, 508: 20, 510: 20, 514: 1} {
		if counts[kind] != count {
			t.Fatalf("IGES type %d count %d, want %d", kind, counts[kind], count)
		}
	}
	verify := func(raw []byte, unit float64) error {
		return VerifyClippedPolynomialFuseIGES(ctx, bytes.NewReader(raw), fuse, unit, Limits{}, opts)
	}
	exported := a.Bytes()
	if err := verify(bytes.Replace(exported, []byte("PlainSolid exact"), []byte("PlainSolid check"), 1), .001); err != nil {
		t.Fatal("harmless banner change:", err)
	}
	if err := verify(exported, 1); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("changed material units accepted: %v", err)
	}
	modified := bytes.Clone(exported)
	changedUse := false
	for _, line := range bytes.Split(modified, []byte{'\n'}) {
		if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte("508,")) {
			continue
		}
		v := bytes.Split(line[:64], []byte{','})
		if len(v) < 7 || len(v[5]) != 1 || v[5][0] != '0' && v[5][0] != '1' {
			t.Fatal("could not identify a standard IGES loop orientation")
		}
		v[5][0] ^= 1
		changedUse = true
		break
	}
	if !changedUse {
		t.Fatal("could not exercise tampered loop orientation")
	}
	if err := verify(modified, .001); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("changed original coedge orientation accepted: %v", err)
	}
	if err := WriteClippedPolynomialFuse(ctx, &refused, fuse, .001, Limits{}, ClippedPolynomialFuseIGESOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || refused.Len() != 0 {
		t.Fatalf("nondecimal pole without opt-in returned bytes: %v", err)
	}
	for _, l := range []Limits{{MaxBytes: 256}, {MaxEntities: 4}, {MaxControlPoints: 4}} {
		if err := WriteClippedPolynomialFuse(ctx, &refused, fuse, .001, l, opts); err == nil || refused.Len() != 0 {
			t.Fatalf("resource limit %+v returned bytes: %v", l, err)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_IGES"); oracle != "" {
		path := filepath.Join(t.TempDir(), "clipped-fuse.iges")
		if err := os.WriteFile(path, a.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.Command(oracle, "read", path).CombinedOutput()
		var got struct {
			Volume, Area  float64
			Faces, Solids int
		}
		start := bytes.LastIndexByte(data, '{')
		if err != nil || start < 0 || json.Unmarshal(data[start:], &got) != nil {
			t.Fatalf("independent IGES reader: %v\n%s", err, data)
		}
		volume, _ := fuse.ExactVolume().Float64()
		if got.Solids != 1 || got.Faces != 20 || math.Abs(got.Volume-volume) > 1e-7 || math.Abs(got.Area-40.443802639426487) > 1e-7 {
			t.Fatalf("independent IGES measures: %+v; exact volume %.16g", got, volume)
		}
	}
}

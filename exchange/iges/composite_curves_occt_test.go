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
	"time"
)

func TestCompositeCurvesRetainedOCCT(t *testing.T) {
	b, e := os.ReadFile("testdata/composite-curve.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCompositeCurves(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Entities) != 3 {
		t.Fatal(len(m.Entities))
	}
	if e = WriteCompositeCurves(&bytes.Buffer{}, m); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTCompositeCurves(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_COMPOSITE")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_COMPOSITE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := filepath.Join(t.TempDir(), "source.igs")
	if b, e := exec.CommandContext(ctx, oracle, "write", source).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCompositeCurves(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(t.TempDir(), "output.igs")
	if e = os.WriteFile(output, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{source, output} {
		raw, e = exec.CommandContext(ctx, oracle, "read", p).Output()
		if e != nil {
			t.Fatal(e)
		}
		start := bytes.LastIndexByte(raw, '{')
		if start < 0 {
			t.Fatal(string(raw))
		}
		var got struct {
			Edges, Wires int
			Length       float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(raw[start:]), &got); e != nil {
			t.Fatal(e, string(raw))
		}
		if got.Edges != 2 || got.Wires != 1 || math.Abs(got.Length-(1+math.Pi/2)) > 1e-10 {
			t.Fatal(got)
		}
	}
}

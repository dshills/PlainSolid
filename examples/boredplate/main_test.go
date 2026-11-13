package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/document"
)

func TestBoredPlateOutputs(t *testing.T) {
	ctx := context.Background()
	p := t.TempDir()
	native, svg := filepath.Join(p, "plate.json"), filepath.Join(p, "plate.svg")
	if e := run(ctx, native, svg); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(native)
	if e != nil {
		t.Fatal(e)
	}
	d, e := document.Read(bytes.NewReader(b), 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Features) != 2 || s.Labels["plate.material"].Attributes["material"].Text != "Aluminum" {
		t.Fatal(s)
	}
	s, e = s.EvaluateFeatures(ctx)
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Trimmed["plate"].MassProperties(ctx, parametric.IntegrationOptions{})
	want := 192 - math.Pi*.8*.8*math.Sqrt(17)
	if e != nil || math.Abs(m.Volume-want) > 1e-8 {
		t.Fatal(m, want, e)
	}
	b, e = os.ReadFile(svg)
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Lines []struct {
			ID string `xml:"data-edge,attr"`
		} `xml:"g>line"`
	}
	if e = xml.Unmarshal(b, &doc); e != nil || len(doc.Lines) < 5 {
		t.Fatal(len(doc.Lines), e)
	}
}

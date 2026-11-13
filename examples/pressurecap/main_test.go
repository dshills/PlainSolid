package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/exchange"
)

func TestPressureCap(t *testing.T) {
	ctx := context.Background()
	native, stl := filepath.Join(t.TempDir(), "cap.json"), filepath.Join(t.TempDir(), "cap.stl")
	if e := run(ctx, native, stl); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(native)
	if e != nil {
		t.Fatal(e)
	}
	s, e := parametric.ReadJSON(f, parametric.JSONLimits{})
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	want := math.Pi * 14 * 14 * (10 - 14.0/3)
	if e != nil || math.Abs(p.Volume-want) > 1e-7 {
		t.Fatal(p, e)
	}
	f, e = os.Open(stl)
	if e != nil {
		t.Fatal(e)
	}
	m, e := exchange.ReadSTL(f, exchange.Limits{})
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.SignedVolume()/want-1) > .005 {
		t.Fatalf("STL volume %g vs exact %g", m.SignedVolume(), want)
	}
}

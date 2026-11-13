package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
)

func TestSplineEnvelopeExports(t *testing.T) {
	ctx := context.Background()
	d := t.TempDir()
	j, s := filepath.Join(d, "solid.json"), filepath.Join(d, "solid.stl")
	if e := run(ctx, j, s); e != nil {
		t.Fatal(e)
	}
	a, e := os.Open(j)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	solid, e := trimmed.ReadJSON(a, trimmed.JSONLimits{})
	if e != nil || solid.Topology().EulerCharacteristic != 2 {
		t.Fatal(solid.Topology(), e)
	}
	props, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(props.Volume-7./6) > 1e-8 {
		t.Fatal(props, e)
	}
	b, e := os.Open(s)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	m, e := exchange.ReadSTL(b, exchange.Limits{})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-7./6) > .02 {
		t.Fatal(m.SignedVolume(), e)
	}
}

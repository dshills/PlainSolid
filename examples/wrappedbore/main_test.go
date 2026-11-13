package main

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
)

func TestWrappedBoreSTEPAndNative(t *testing.T) {
	ctx := context.Background()
	m, e := build(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var stepFile, native bytes.Buffer
	if e = step.WriteWrappedBlindBore(&stepFile, m); e != nil {
		t.Fatal(e)
	}
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := step.ReadWrappedBlindBore(&stepFile, step.Limits{}, step.WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil || s.Name != m.Name || s.FloorName != m.FloorName || s.LengthUnit != .001 {
		t.Fatal(s.Name, s.FloorName, s.LengthUnit, e)
	}
	n, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []trimmed.Solid{s.Solid, n} {
		p, e := x.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		want := (400*40 - 64*15) * math.Pi
		if e != nil || math.Abs(p.Volume-want) > .001 || x.Topology().EulerCharacteristic != 2 {
			t.Fatal(p, x.Topology(), e)
		}
	}
}

package main

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
)

func TestExample(t *testing.T) {
	var data bytes.Buffer
	p, e := run(context.Background(), &data)
	if e != nil || math.Abs(p.Volume-math.Pi/64) > 1e-8 {
		t.Fatal(p, e)
	}
	s, e := trimmed.ReadJSON(&data, trimmed.JSONLimits{})
	if e != nil || s.Topology().Faces != 2 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(s.Topology(), e)
	}
}

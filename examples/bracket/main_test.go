package main

import (
	"math"
	"os"
	"path/filepath"
	"plainsolid/exchange"
	"testing"
)

func TestBracketRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bracket.stl")
	if e := run(path); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	m, e := exchange.ReadSTL(f, exchange.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.SignedVolume()-7520) > 1e-8 {
		t.Fatal(m.SignedVolume())
	}
}

package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"plainsolid/exchange"
	"testing"
)

func TestExtrudeDialogJoinThenCutExactVolume(t *testing.T) {
	b := New()
	prism := func(radius, height float64) Shape {
		return shape(t, b, map[string]any{"op": "prism", "outer": []vector{{-radius, -radius, 0}, {radius, -radius, 0}, {radius, radius, 0}, {-radius, radius, 0}}, "offset": vector{0, 0, height}})
	}
	stock, tool := prism(5, 10), prism(2.5, 12)
	joined := shape(t, b, map[string]any{"op": "fuse", "a": stock.Handle, "b": tool.Handle})
	near(t, joined.GeometryAssertions.Volume, 1050)
	cut := shape(t, b, map[string]any{"op": "cut", "a": joined.Handle, "b": tool.Handle})
	near(t, cut.GeometryAssertions.Volume, 750)
	for _, original := range []Shape{stock, tool, joined} {
		got := shape(t, b, map[string]any{"op": "inspect", "handle": original.Handle})
		near(t, got.GeometryAssertions.Volume, original.GeometryAssertions.Volume)
	}
	var encoded struct {
		Base64 string `json:"base64"`
	}
	if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "stl", "handle": cut.Handle}), &encoded); e != nil {
		t.Fatal(e)
	}
	data, e := base64.StdEncoding.DecodeString(encoded.Base64)
	if e != nil {
		t.Fatal(e)
	}
	m, e := exchange.ReadSTL(bytes.NewReader(data), exchange.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	p, e := m.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Volume, 750)
}

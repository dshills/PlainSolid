package plaincadbridge

import (
	"encoding/json"
	"testing"
)

// benchPlate is a plate with eight through bores in a row, cut one after
// another on the default bridge.
func benchPlate(b *testing.B, bridge *Bridge) string {
	ask := func(q map[string]any) string {
		data, err := json.Marshal(q)
		if err != nil {
			b.Fatal(err)
		}
		var reply answer
		if err := json.Unmarshal([]byte(bridge.Call(string(data))), &reply); err != nil || !reply.OK {
			b.Fatal(err, reply.Error)
		}
		return reply.Result.Handle
	}
	plate := ask(map[string]any{"op": "box", "min": vector{}, "max": vector{100, 60, 20}})
	for k := 0; k < 8; k++ {
		tool := ask(map[string]any{"op": "circle", "origin": vector{10 + 11*float64(k), 30, 0}, "normal": vector{0, 0, 1}, "radius": 3, "height": 20})
		plate = ask(map[string]any{"op": "cut", "a": plate, "b": tool})
	}
	return plate
}

// BenchmarkPlainBoreCuts is the time of the eight cuts.
func BenchmarkPlainBoreCuts(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchPlate(b, New())
	}
}

// BenchmarkPlainBoreMesh is the time of one display mesh of the plate.
func BenchmarkPlainBoreMesh(b *testing.B) {
	bridge := New()
	plate := benchPlate(b, bridge)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bridge.Call(`{"op":"mesh","handle":"` + plate + `","linearDeflection":0.1,"angularDeflection":0.2}`)
	}
}

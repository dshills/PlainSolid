package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// FuzzBoundedRequests covers the public JSON boundary without allowing mutated
// inputs to invoke tessellation, exchange readers, or unbounded modeling jobs.
func FuzzBoundedRequests(f *testing.F) {
	for _, s := range []string{``, `{`, `null`, `[]`, `{"op":"unknown"}`, `{"op":"box","min":[0,0,0],"max":[1,2,3]}`, `{"op":"clone","handle":"missing"}`, `{"op":"transform","handle":"missing","matrix":[[1,0,0,0],[0,1,0,0],[0,0,1,0]]}`, `{"op":"inspect","handle":"missing","extra":1}`, `{"op":"box"} {}`} {
		f.Add(s, uint8(0))
	}
	oversized := strings.Repeat(" ", MaxRequestBytes+1)
	f.Fuzz(func(t *testing.T, input string, mode uint8) {
		if len(input) > 4096 {
			return
		}
		b := New()
		base := box(t, b, vector{}, vector{2, 3, 4})
		clone := shape(t, b, map[string]any{"op": "clone", "handle": base.Handle})
		shift := float64(int(mode)%17 - 8)
		moved := shape(t, b, map[string]any{"op": "transform", "handle": clone.Handle, "matrix": [3][4]float64{{1, 0, 0, shift}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
		if moved.Handle == base.Handle || moved.Handle == clone.Handle || len(b.shapes) != 3 {
			t.Fatal("ownership")
		}
		if b.shapes[base.Handle].result.Bounds.Min[0] != 0 || moved.Bounds.Min[0] != shift {
			t.Fatal("transform mutated source")
		}
		// Exact size gate is tested on every seed, not left to chance mutations.
		checkRejected(t, b, oversized)
		checkRejected(t, b, `{"op":"clone","handle":"missing"}`)
		checkRejected(t, b, `{"op":"transform","handle":"`+base.Handle+`","matrix":[[0,0,0,0],[0,0,0,0],[0,0,0,0]]}`)
		// Only short, bounded box/handle operations may reach execute. Other valid
		// JSON is routed to an unknown operation; malformed input stays unchanged.
		var q map[string]json.RawMessage
		if json.Unmarshal([]byte(input), &q) == nil && q != nil {
			var op string
			_ = json.Unmarshal(q["op"], &op)
			switch op {
			case "box":
				for _, key := range []string{"min", "max"} {
					var a []float64
					if json.Unmarshal(q[key], &a) != nil || len(a) != 3 {
						return
					}
					for _, v := range a {
						if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 32 {
							return
						}
					}
				}
			case "clone", "inspect", "dispose":
				q["handle"] = json.RawMessage(`"missing"`)
				if mode&1 != 0 {
					q["handle"], _ = json.Marshal(clone.Handle)
					v, _ := json.Marshal(q)
					input = string(v)
				}
			case "transform":
				q["handle"], _ = json.Marshal(clone.Handle)
				var m [3][4]float64
				if json.Unmarshal(q["matrix"], &m) != nil {
					return
				}
				for _, row := range m {
					for _, v := range row {
						if math.Abs(v) > 4 {
							return
						}
					}
				}
			default:
				q["op"] = json.RawMessage(`"fuzz-unknown"`)
				v, _ := json.Marshal(q)
				input = string(v)
			}
			v, _ := json.Marshal(q)
			input = string(v)
		}
		before := requestState(b)
		var r response
		if e := json.Unmarshal([]byte(b.Call(input)), &r); e != nil {
			t.Fatal(e)
		}
		if !r.OK {
			if r.Error == nil || r.Error.Code == "INTERNAL" {
				t.Fatalf("internal response for %q: %+v", input, r.Error)
			}
			if before != requestState(b) {
				t.Fatalf("rejected request mutated registry: %q", input)
			}
		}
		if _, exists := b.shapes[base.Handle]; exists {
			invoke(t, b, map[string]any{"op": "dispose", "handle": base.Handle})
		}
		if _, ok := b.shapes[base.Handle]; ok {
			t.Fatal("dispose retained owner")
		}
		if _, ok := b.shapes[moved.Handle]; !ok {
			t.Fatal("dispose invalidated independent transformed handle")
		}
	})
}
func requestState(b *Bridge) string {
	out := map[string]any{}
	for h, s := range b.shapes {
		out[h] = []any{s.result, s.solid.Vertices(), s.solid.Edges(), s.solid.Faces()}
	}
	v, e := json.Marshal(out)
	if e != nil {
		panic(e)
	}
	return string(v)
}
func checkRejected(t *testing.T, b *Bridge, input string) {
	t.Helper()
	before := requestState(b)
	var r response
	if e := json.Unmarshal([]byte(b.Call(input)), &r); e != nil {
		t.Fatal(e)
	}
	if r.OK || r.Error == nil || r.Error.Code == "INTERNAL" {
		t.Fatalf("expected safe rejection: %+v", r)
	}
	if before != requestState(b) {
		t.Fatal("rejection changed handles or geometry")
	}
}

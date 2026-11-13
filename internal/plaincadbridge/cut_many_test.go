package plaincadbridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCutManyPlanarAtomicAndRedundantTools(t *testing.T) {
	b := New()
	base := box(t, b, vector{0, 0, 0}, vector{10, 10, 10})
	a := box(t, b, vector{0, 0, 0}, vector{2, 10, 10})
	c := box(t, b, vector{8, 0, 0}, vector{10, 10, 10})
	before := len(b.shapes)
	result := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": []string{a.Handle, a.Handle, c.Handle}})
	near(t, result.GeometryAssertions.Volume, 600)
	if len(b.shapes) != before+1 {
		t.Fatal("batch published intermediates")
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": base.Handle}).GeometryAssertions.Volume, 1000)
	for _, tools := range [][]string{nil, {}, {a.Handle, "missing"}} {
		before := len(b.shapes)
		request := map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools}
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		reply := b.Call(string(encoded))
		if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
			t.Fatal(reply)
		}
	}
}

func TestCutManyValidationDoesNotPublish(t *testing.T) {
	b := New()
	base := box(t, b, vector{0, 0, 0}, vector{10, 10, 10})
	planar := box(t, b, vector{0, 0, 0}, vector{2, 10, 10})
	cylinder := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 1, 10)
	cases := [][]string{{planar.Handle, cylinder.Handle}, make([]string, 33), {planar.Handle, "missing"}}
	for _, tools := range cases {
		before := len(b.shapes)
		encoded, _ := json.Marshal(map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
		reply := b.Call(string(encoded))
		if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
			t.Fatal(reply)
		}
	}
	missed := box(t, b, vector{20, 0, 0}, vector{21, 1, 1})
	before := len(b.shapes)
	reply := b.Call(`{"op":"cutMany","a":"` + base.Handle + `","tools":["` + missed.Handle + `"]}`)
	if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
		t.Fatal("no-op batch accepted", reply)
	}
}

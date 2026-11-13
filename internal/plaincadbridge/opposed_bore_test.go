package plaincadbridge

import (
	"math"
	"strings"
	"testing"
)

func TestOpposedBlindBoreMerge(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, reverse := range []bool{false, true} {
				b := New()
				stock := box(t, b, vector{-5, -5, -5}, vector{5, 5, 5})
				n, o := vector{}, vector{}
				n[axis] = sign
				o[axis] = -5 * sign
				a := circle(t, b, o, n, 1, 6)
				n[axis] = -sign
				o[axis] = 5 * sign
				c := circle(t, b, o, n, 1, 5)
				if reverse {
					a, c = c, a
				}
				first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": a.Handle})
				result := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": c.Handle})
				body := b.shapes[result.Handle].bore
				near(t, result.GeometryAssertions.Volume, 1000-10*math.Pi)
				if len(body.inputCylinders) != 2 || len(body.cylinders) != 1 || body.blind[0] {
					t.Fatal("lineage", body)
				}
				if body.inputCylinders[0] != b.shapes[a.Handle].cylinder || body.inputCylinders[1] != b.shapes[c.Handle].cylinder {
					t.Fatal("lost input order")
				}
				before := len(b.shapes)
				reply := b.Call(`{"op":"cut","a":"` + result.Handle + `","b":"` + c.Handle + `"}`)
				if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
					t.Fatal(reply)
				}
			}
		}
	}
}
func TestOpposedBlindBoreStrictOverlapAndUnrelated(t *testing.T) {
	b := New()
	s := box(t, b, vector{-5, -5, 0}, vector{5, 5, 10})
	a := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 1, 5)
	for _, h := range []float64{math.Nextafter(5, 0), 5, math.Nextafter(5, 6)} {
		c := circle(t, b, vector{0, 0, 10}, vector{0, 0, -1}, 1, h)
		_, ok := mergeOpposedBlindBores(b.shapes[s.Handle].solid, []*cylinderBody{b.shapes[a.Handle].cylinder, b.shapes[c.Handle].cylinder})
		if ok != (h > 5) {
			t.Fatal(h, ok)
		}
	}
	c := circle(t, b, vector{0, 0, 10}, vector{0, 0, -1}, 1, 6)
	other := circle(t, b, vector{3, 0, 0}, vector{0, 0, 1}, .5, 2)
	result := shape(t, b, map[string]any{"op": "cutMany", "a": s.Handle, "tools": []string{a.Handle, c.Handle, other.Handle}})
	body := b.shapes[result.Handle].bore
	mesh, err := b.shapes[result.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err = mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	moved := shape(t, b, map[string]any{"op": "transform", "handle": result.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	if len(b.shapes[moved.Handle].bore.inputCylinders) != 3 {
		t.Fatal("lost transformed lineage")
	}
	if len(body.inputCylinders) != 3 || len(body.blind) != 2 || body.blind[0] || !body.blind[1] {
		t.Fatal("unrelated blind history")
	}
	near(t, result.GeometryAssertions.Volume, 1000-10.5*math.Pi)
	for len(b.shapes) < MaxHandles {
		shape(t, b, map[string]any{"op": "clone", "handle": s.Handle})
	}
	before := len(b.shapes)
	reply := b.Call(`{"op":"cutMany","a":"` + s.Handle + `","tools":["` + a.Handle + `","` + c.Handle + `"]}`)
	if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
		t.Fatal(reply)
	}
}

func TestOpposedBlindBoreRejectsDifferentSupport(t *testing.T) {
	b := New()
	s := box(t, b, vector{-5, -5, 0}, vector{5, 5, 10})
	a := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 1, 6)
	for _, tc := range []struct {
		o vector
		r float64
	}{{vector{0, 0, 10}, 2}, {vector{math.SmallestNonzeroFloat64, 0, 10}, 1}} {
		c := circle(t, b, tc.o, vector{0, 0, -1}, tc.r, 6)
		if _, ok := mergeOpposedBlindBores(b.shapes[s.Handle].solid, []*cylinderBody{b.shapes[a.Handle].cylinder, b.shapes[c.Handle].cylinder}); ok {
			t.Fatal("different support merged")
		}
	}
}

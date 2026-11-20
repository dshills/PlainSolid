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
				// The two pockets overlap between the levels 0 and 1 and
				// are one through bore: its wall and two openings, and no
				// floor at either level.
				near(t, result.GeometryAssertions.Volume, 1000-10*math.Pi)
				near(t, result.GeometryAssertions.SurfaceArea, 600+18*math.Pi)
				for _, level := range []float64{0, 1} {
					at := vector{}
					at[axis] = level * sign
					if f, err := planarFace(t, b, result.Handle, at, n); err == nil {
						t.Fatal("a floor remains in the through bore", f)
					}
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
	mesh, err := b.shapes[result.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err = mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	moved := shape(t, b, map[string]any{"op": "transform", "handle": result.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	// The opposed pair is one through bore and the third tool one pocket:
	// the wall of each, two openings of the bore, and the pocket's floor in
	// place of its opening. The floor is still referenced after a move.
	near(t, result.GeometryAssertions.Volume, 1000-10.5*math.Pi)
	near(t, result.GeometryAssertions.SurfaceArea, 600+20*math.Pi)
	near(t, moved.GeometryAssertions.Volume, result.GeometryAssertions.Volume)
	near(t, moved.GeometryAssertions.SurfaceArea, result.GeometryAssertions.SurfaceArea)
	for _, floor := range []struct {
		handle        string
		point, normal vector
	}{{result.Handle, vector{3, 0, 2}, vector{0, 0, 1}}, {moved.Handle, vector{9, 11, 9}, vector{1, 0, 0}}} {
		f, err := planarFace(t, b, floor.handle, floor.point, floor.normal)
		if err != nil {
			t.Fatal(err)
		}
		near(t, f.Area, math.Pi/4)
	}
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

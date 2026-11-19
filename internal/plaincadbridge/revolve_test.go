package plaincadbridge

import (
	"math"
	"strings"
	"testing"
)

func TestRevolveAndCoaxialThroughBore(t *testing.T) {
	for _, angle := range []float64{2 * math.Pi, math.Pi / 2, -math.Pi / 2} {
		b := New()
		target := shape(t, b, map[string]any{
			"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 1, 0}, "angle": angle,
			"outer": []vector{{0, 0, 0}, {5, 0, 0}, {5, 10, 0}, {0, 10, 0}},
		})
		near(t, target.GeometryAssertions.Volume, math.Abs(angle)*125)
		tool := shape(t, b, map[string]any{"op": "circle", "origin": vector{0, 11, 0}, "normal": vector{0, -1, 0}, "radius": 2, "height": 11})
		q := map[string]any{"op": "cut", "a": target.Handle, "b": tool.Handle}
		got := shape(t, b, q)
		want := math.Abs(angle) * 105
		near(t, got.GeometryAssertions.Volume, want)
		m, err := b.shapes[got.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(angle, err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(angle, err)
		}
		mp, err := m.MassProperties()
		if err != nil || math.Abs(mp.Volume/want-1) >= .005 {
			t.Fatal(angle, mp.Volume, want, err)
		}
		q["b"] = shape(t, b, map[string]any{"op": "circle", "origin": vector{0, 11, 0}, "normal": vector{0, -1, 0}, "radius": 2, "height": 10}).Handle
		if angle == 2*math.Pi {
			// The full circular stock uses the finite blind-bore constructor.
			// Its entry is at Y=10 and the floor remains one unit above the
			// original lower cap.
			blind := shape(t, b, q)
			near(t, blind.GeometryAssertions.Volume, (250-36)*math.Pi)
			near(t, blind.GeometryAssertions.SurfaceArea, (150+36)*math.Pi)
			s := b.shapes[blind.Handle]
			floor, err := s.analytic.planarFace(vector{0, 1, 0}.vec(), vector{0, 1, 0}.vec())
			if err != nil {
				t.Fatal(err)
			}
			near(t, floor.Area, 4*math.Pi)
		} else {
			// On a partial sector the legacy path declines a tool that
			// stops short of the far cap. The general Boolean cuts the
			// blind bore, nine deep.
			blind := declined(t, b, "", "cut", target.Handle, q["b"].(string))
			near(t, blind.GeometryAssertions.Volume, math.Abs(angle)*107)
		}
		near(t, b.shapes[target.Handle].result.GeometryAssertions.Volume, math.Abs(angle)*125)
	}
}

func TestRevolvedCylinderCornerCutAndFuse(t *testing.T) {
	b := New()
	base := box(t, b, vector{}, vector{20, 20, 10})
	tool := shape(t, b, map[string]any{
		"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 1, 0}, "angle": 2 * math.Pi,
		"outer": []vector{{0, 0, 0}, {5, 0, 0}, {5, 10, 0}, {0, 10, 0}},
	})
	for op, want := range map[string]float64{"cut": 4000 - 62.5*math.Pi, "fuse": 4000 + 187.5*math.Pi} {
		got := shape(t, b, map[string]any{"op": op, "a": base.Handle, "b": tool.Handle})
		near(t, got.GeometryAssertions.Volume, want)
		m, err := b.shapes[got.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(op, err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(op, err)
		}
	}
}

func TestCircularRevolveTorusAndAxisValidation(t *testing.T) {
	b := New()
	r := map[string]any{
		"op": "revolveCircle", "origin": vector{5, -5, 0}, "direction": vector{0, 1, 0},
		"center": vector{10, 0, 0}, "normal": vector{0, 0, 1}, "radius": 2, "angle": math.Pi / 2,
	}
	got := shape(t, b, r)
	near(t, got.GeometryAssertions.Volume, 10*math.Pi*math.Pi)
	for i, p := range (vector{5, -2, -7}) {
		near(t, got.Bounds.Min[i], p)
	}
	for i, p := range (vector{12, 2, 0}) {
		near(t, got.Bounds.Max[i], p)
	}
	m, err := b.shapes[got.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if response := b.Call(`{"op":"revolveCircle","origin":[0,0,0],"direction":[1,0,0],"center":[10,0,0],"normal":[0,0,1],"radius":2,"angle":1.57}`); !strings.Contains(response, "must not cross") {
		t.Fatal(response)
	}
}

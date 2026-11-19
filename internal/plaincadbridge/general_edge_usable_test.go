package plaincadbridge

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"testing"
)

// TestGeneralTreatedStaysUsable issues to a chamfered and to a filleted
// general result what a part needs after a treatment (section 10 of
// specs/general-edge-treatment).
func TestGeneralTreatedStaysUsable(t *testing.T) {
	for _, c := range []struct {
		op   string
		size float64
		// crossed is what the box (8,−1,−1)–(10,3,3) removes: 2 along x
		// times its 3 by 3 section less the section of the treatment.
		crossed float64
	}{{"chamfer", 0.5, 2 * (9 - 0.125)}, {"fillet", 2, 2 * (9 - (4 - math.Pi))}} {
		t.Run(c.op, func(t *testing.T) {
			b := New()
			part := shape(t, b, treat(c.op, slottedBox(t, b).Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, c.size))
			v, a := part.GeometryAssertions.Volume, part.GeometryAssertions.SurfaceArea

			// F1: queries, display and exchange.
			sameShape(t, shape(t, b, q{"op": "inspect", "handle": part.Handle}), part)
			sameShape(t, shape(t, b, q{"op": "clone", "handle": part.Handle}), part)
			x, y, z := 1/math.Sqrt(14), 2/math.Sqrt(14), 3/math.Sqrt(14)
			cos, sin := math.Cos(0.7), math.Sin(0.7)
			turned := shape(t, b, q{"op": "transform", "handle": part.Handle, "matrix": [][]float64{
				{cos + x*x*(1-cos), x*y*(1-cos) - z*sin, x*z*(1-cos) + y*sin, 3},
				{y*x*(1-cos) + z*sin, cos + y*y*(1-cos), y*z*(1-cos) - x*sin, -2},
				{z*x*(1-cos) - y*sin, z*y*(1-cos) + x*sin, cos + z*z*(1-cos), 5}}})
			near(t, turned.GeometryAssertions.Volume, v)
			near(t, turned.GeometryAssertions.SurfaceArea, a)
			for _, analytic := range []bool{false, true} {
				var m Mesh
				if err := json.Unmarshal(invoke(t, b, q{"op": "mesh", "handle": part.Handle, "linearDeflection": 0.05,
					"angularDeflection": 0.2, "analyticNormals": analytic}), &m); err != nil || len(m.Indices) == 0 {
					t.Fatalf("mesh: %v, %d indices", err, len(m.Indices))
				}
			}
			for _, request := range []q{
				{"op": "stl", "handle": part.Handle},
				{"op": "displayEdges", "handle": part.Handle, "maxEdgeUses": 12000, "maxEdgeSegments": 20000},
				{"op": "displayEdges", "handle": part.Handle, "maxSegments": 1000, "edgeUses": 100},
				{"op": "drawingGeometry", "handle": part.Handle},
				{"op": "planarFace", "handle": part.Handle, "origin": vector{0, 10, 0}, "normal": vector{0, 1, 0}},
			} {
				invoke(t, b, request)
			}
			var exported struct{ Base64 string }
			if err := json.Unmarshal(invoke(t, b, q{"op": "exportStep", "handles": []string{part.Handle}}), &exported); err != nil {
				t.Fatal(err)
			}
			text, err := base64.StdEncoding.DecodeString(exported.Base64)
			if err != nil || len(text) == 0 {
				t.Fatalf("exported STEP: %v, %d bytes", err, len(text))
			}
			sameShape(t, shape(t, b, q{"op": "importStep", "source": string(text)}), part)

			// F2: further Booleans, issued as PlainCAD issues them.
			through := func(x float64) Shape { return circle(t, b, vector{x, 5, -1}, vector{0, 0, 1}, 1, 12) }
			near(t, shape(t, b, q{"op": "cut", "a": part.Handle, "b": through(9).Handle}).GeometryAssertions.Volume, v-10*math.Pi)
			near(t, shape(t, b, q{"op": "cut", "a": part.Handle, "b": box(t, b, vector{8, -1, -1}, vector{10, 3, 3}).Handle}).GeometryAssertions.Volume, v-c.crossed)
			near(t, shape(t, b, q{"op": "fuse", "a": part.Handle, "b": circle(t, b, vector{5, 5, 10}, vector{0, 0, 1}, 1, 3).Handle}).GeometryAssertions.Volume, v+3*math.Pi)
			near(t, shape(t, b, q{"op": "cutMany", "a": part.Handle, "tools": []string{through(3).Handle, through(9).Handle}}).GeometryAssertions.Volume, v-20*math.Pi)
			var overlap struct{ Volume float64 }
			if err := json.Unmarshal(invoke(t, b, q{"op": "overlapVolume", "a": part.Handle, "b": box(t, b, vector{0, 4, 4}, vector{1, 6, 6}).Handle}), &overlap); err != nil {
				t.Fatal(err)
			}
			near(t, overlap.Volume, 4)

			// F2 and F3: a second treatment of an untouched edge.
			if available(t, b, part.Handle, vector{0, 0, 0}, vector{20, 0, 0}) || !available(t, b, part.Handle, vector{0, 10, 0}, vector{20, 10, 0}) {
				t.Fatal("the treated edge is available, or the untouched edge is not")
			}
			again := shape(t, b, treat("chamfer", part.Handle, vector{0, 10, 0}, vector{20, 10, 0}, vector{0, 0, 1}, 0.5))
			requireGeneral(t, b, again, v-20*0.125, a+20*0.5*(math.Sqrt2-2)-0.25)
			again = shape(t, b, treat("fillet", part.Handle, vector{0, 10, 0}, vector{20, 10, 0}, vector{0, 0, 1}, 1))
			requireGeneral(t, b, again, v-20*(1-math.Pi/4), a+20*(math.Pi/2-2)-2*(1-math.Pi/4))
		})
	}
}

// TestGeneralTreatmentAgreesWithLegacy issues one request to stock that
// a legacy path treats and to the same stock as a general result, and
// requires the same shape (section 11.2).
func TestGeneralTreatmentAgreesWithLegacy(t *testing.T) {
	axis, side := vector{0, 0, 1}, vector{0, 1, 0}
	edges := func(pairs ...[2]vector) []any {
		var out []any
		for _, p := range pairs {
			out = append(out, edgeRef(p[0], p[1]))
		}
		return out
	}
	plain := func(t *testing.T, b *Bridge) Shape { return box(t, b, vector{}, vector{20, 10, 10}) }
	triangle := func(t *testing.T, b *Bridge) Shape {
		return shape(t, b, q{"op": "prism", "outer": []vector{{0, 0, 0}, {4, 0, 0}, {0, 3, 0}}, "offset": vector{0, 0, 5}})
	}
	l := func(t *testing.T, b *Bridge) Shape {
		return shape(t, b, q{"op": "prism", "offset": vector{0, 0, 10},
			"outer": []vector{{0, 0, 0}, {20, 0, 0}, {20, 10, 0}, {10, 10, 0}, {10, 20, 0}, {0, 20, 0}}})
	}
	bored := func(t *testing.T, b *Bridge) Shape {
		return shape(t, b, q{"op": "cut", "a": plain(t, b).Handle, "b": circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10).Handle})
	}
	pocketed := func(t *testing.T, b *Bridge) Shape {
		return shape(t, b, q{"op": "cut", "a": box(t, b, vector{}, vector{40, 30, 10}).Handle, "b": box(t, b, vector{15, 10, 8}, vector{25, 20, 10}).Handle})
	}
	holed := func(t *testing.T, b *Bridge) Shape {
		return shape(t, b, q{"op": "cut", "a": box(t, b, vector{}, vector{20, 20, 10}).Handle, "b": circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10).Handle})
	}
	top := [2]vector{{0, 0, 10}, {20, 0, 10}}
	for _, c := range []struct {
		name    string
		stock   func(*testing.T, *Bridge) Shape
		request q
	}{
		{"a box, fillet of a cap edge", plain, q{"op": "fillet", "normal": axis, "radius": 1.0, "edges": edges(top)}},
		{"a box, chamfer of a cap edge", plain, q{"op": "chamfer", "normal": axis, "distance": 1.0, "edges": edges(top)}},
		{"a box, fillet of two parallel cap edges", plain, q{"op": "fillet", "normal": axis, "radius": 1.0,
			"edges": edges(top, [2]vector{{0, 10, 10}, {20, 10, 10}})}},
		{"a box, chamfer of two parallel cap edges", plain, q{"op": "chamfer", "normal": axis, "distance": 1.0,
			"edges": edges(top, [2]vector{{0, 10, 10}, {20, 10, 10}})}},
		{"Q8, chamfer of two axial edges", triangle, q{"op": "chamfer", "normal": axis, "direction": axis, "distance": 5.0 / 16,
			"edges": edges([2]vector{{4, 0, 0}, {4, 0, 5}}, [2]vector{{0, 0, 0}, {0, 0, 5}})}},
		{"Q8, fillet of the acute axial edge", triangle, q{"op": "fillet", "normal": axis, "direction": axis, "radius": 5.0 / 16,
			"edges": edges([2]vector{{4, 0, 0}, {4, 0, 5}})}},
		{"Q9, fillet of a convex and a concave edge", l, q{"op": "fillet", "normal": side, "direction": axis, "radius": 1.0,
			"edges": edges([2]vector{{20, 10, 0}, {20, 10, 10}}, [2]vector{{10, 10, 0}, {10, 10, 10}})}},
		{"Q9, chamfer of a convex and a concave edge", l, q{"op": "chamfer", "normal": side, "direction": axis, "distance": 1.0,
			"edges": edges([2]vector{{20, 10, 0}, {20, 10, 10}}, [2]vector{{10, 10, 0}, {10, 10, 10}})}},
		{"a bored box, chamfer of a cap edge", bored, q{"op": "chamfer", "normal": axis, "distance": 0.5,
			"edges": edges([2]vector{{0, 0, 0}, {20, 0, 0}})}},
		{"a bored box, fillet of a cap edge", bored, q{"op": "fillet", "normal": axis, "radius": 1.0, "edges": edges(top)}},
		{"Q5, chamfer of a cap edge", pocketed, q{"op": "chamfer", "normal": axis, "distance": 1.0, "edges": edges([2]vector{{0, 0, 10}, {40, 0, 10}})}},
		{"Q5, fillet of a cap edge", pocketed, q{"op": "fillet", "normal": axis, "radius": 1.0, "edges": edges([2]vector{{0, 0, 10}, {40, 0, 10}})}},
		{"Q6, chamfer of a cap perimeter", holed, q{"op": "capChamfer", "normal": axis, "distance": 1.0, "edges": edges(
			[2]vector{{0, 0, 10}, {20, 0, 10}}, [2]vector{{20, 0, 10}, {20, 20, 10}}, [2]vector{{20, 20, 10}, {0, 20, 10}}, [2]vector{{0, 20, 10}, {0, 0, 10}})}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := New()
			stock := c.stock(t, b)
			if body := b.shapes[stock.Handle].analytic; body != nil && b.generalResult(body) {
				t.Fatal("the stock is a general result already: no legacy path treats it")
			}
			request := q{"handle": stock.Handle}
			for k, v := range c.request {
				request[k] = v
			}
			legacy := shape(t, b, request)
			if body := b.shapes[legacy.Handle].analytic; body != nil && b.generalResult(body) {
				t.Fatal("the legacy answer is a general result")
			}
			request["handle"] = lifted(t, b, stock).Handle
			treated := shape(t, b, request)
			if body := b.shapes[treated.Handle].analytic; body == nil || !b.generalResult(body) {
				t.Fatal("the lifted stock was not treated by the general mechanism")
			}
			sameShape(t, treated, legacy)
		})
	}
}

package plaincadbridge

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
)

func TestOverlappingHoleUnionAxesSignsAndDepths(t *testing.T) {
	area, perimeter := 16*math.Pi/3+2*math.Sqrt(3), 16*math.Pi/3
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, depth := range []float64{3, 10} {
				b := New()
				base := box(t, b, vector{-5, -5, -5}, vector{5, 5, 5})
				n := vector{}
				n[axis] = sign
				var tools []string
				for _, center := range []float64{-1, 1} {
					o := vector{}
					o[axis] = -5 * sign
					o[(axis+1)%3] = center
					c := circle(t, b, o, n, 2, depth)
					tools = append(tools, c.Handle)
				}
				before := len(b.shapes)
				cut := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
				if len(b.shapes) != before+1 {
					t.Fatal("union batch published intermediate handles")
				}
				near(t, cut.GeometryAssertions.Volume, 1000-area*depth)
				wantArea := 600 + perimeter*depth
				if depth == 10 {
					wantArea -= 2 * area
				}
				near(t, cut.GeometryAssertions.SurfaceArea, wantArea)
				body := b.shapes[cut.Handle].bore
				origin := vector{}
				origin[axis] = -5 * sign
				cap, err := body.planarFace(origin.vec(), n.vec())
				if err != nil {
					t.Fatal(err)
				}
				near(t, cap.Area, 100-area)
				near(t, cap.Normal[axis], -sign)
				if depth < 10 {
					origin[axis] += sign * depth
					floor, err := body.planarFace(origin.vec(), n.vec())
					if err != nil {
						t.Fatal(err)
					}
					near(t, floor.Area, area)
					near(t, floor.Normal[axis], -sign)
				}

				p, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil {
					t.Fatal(err)
				}
				near(t, p.Volume, cut.GeometryAssertions.Volume)
				near(t, p.Area, wantArea)
				m, err := b.shapes[cut.Handle].tessellate(request{})
				if err != nil {
					t.Fatal(err)
				}
				if err = m.ValidateSolid(); err != nil {
					t.Fatal(err)
				}
				if cut.Bounds != base.Bounds {
					t.Fatal("strictly interior union changed stock bounds")
				}
				near(t, b.shapes[base.Handle].result.GeometryAssertions.Volume, 1000)
			}
		}
	}
}
func TestHoleUnionDuplicatesContainmentAndTripleOverlap(t *testing.T) {
	b := New()
	base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 8})
	large := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 2, 8)
	small := circle(t, b, vector{.5, 0, 0}, vector{0, 0, 1}, 1, 8)
	for _, tools := range [][]string{{large.Handle, large.Handle}, {small.Handle, large.Handle, small.Handle}, {large.Handle, small.Handle}} {
		cut := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
		near(t, cut.GeometryAssertions.Volume, 800-32*math.Pi)
		before := len(b.shapes)
		reply := b.Call(`{"op":"cut","a":"` + cut.Handle + `","b":"` + small.Handle + `"}`)
		if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
			t.Fatal("singlecut redundant tool accepted", reply)
		}
	}
	var tools []string
	for _, x := range []float64{-1, 0, 1} {
		c := circle(t, b, vector{x, 0, 0}, vector{0, 0, 1}, 2, 8)
		tools = append(tools, c.Handle)
	}
	result := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
	perimeter := 12*math.Pi - 16*math.Acos(.25)
	area := perimeter + math.Sqrt(15)
	near(t, result.GeometryAssertions.Volume, 800-8*area)
	near(t, result.GeometryAssertions.SurfaceArea, 520+8*perimeter-2*area)
	body := b.shapes[result.Handle].bore
	p, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	near(t, p.Volume, result.GeometryAssertions.Volume)
	clone := shape(t, b, map[string]any{"op": "clone", "handle": result.Handle})
	invoke(t, b, map[string]any{"op": "dispose", "handle": result.Handle})
	moved := shape(t, b, map[string]any{"op": "transform", "handle": clone.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	near(t, moved.GeometryAssertions.Volume, clone.GeometryAssertions.Volume)
	mesh, err := b.shapes[moved.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err = mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
func TestHoleUnionUnsupportedBatchDoesNotPublish(t *testing.T) {
	b := New()
	base := box(t, b, vector{-6, -5, 0}, vector{6, 5, 8})
	first := circle(t, b, vector{-1, 0, 0}, vector{0, 0, 1}, 2, 3)
	for _, tc := range []struct{ x, r, h float64 }{{3, 2, 3}} {
		tool := circle(t, b, vector{tc.x, 0, 0}, vector{0, 0, 1}, tc.r, tc.h)
		before := len(b.shapes)
		data, _ := json.Marshal(map[string]any{"op": "cutMany", "a": base.Handle, "tools": []string{first.Handle, tool.Handle}})
		reply := b.Call(string(data))
		if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
			t.Fatal(reply)
		}
	}
}

func TestContainedUnionAnchorChangeDoesNotPublishTinyReduction(t *testing.T) {
	b := New()
	base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 8})
	large := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 2, 8)
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": large.Handle})
	for _, x := range []float64{-.5, -.25, .5} {
		contained := circle(t, b, vector{x, 0, 0}, vector{0, 0, 1}, 1, 8)
		for _, op := range []string{"cut", "cutMany"} {
			before := len(b.shapes)
			q := map[string]any{"op": op, "a": cut.Handle}
			if op == "cut" {
				q["b"] = contained.Handle
			} else {
				q["tools"] = []string{contained.Handle}
			}
			data, _ := json.Marshal(q)
			reply := b.Call(string(data))
			if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
				t.Fatal("roundoff published no-op", reply)
			}
		}
	}
}

func TestThroughUnionOpposedAxesAndInputOrders(t *testing.T) {
	for _, x := range []float64{0, 2} {
		for _, reverse := range []bool{false, true} {
			b := New()
			base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 8})
			a := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 2, 8)
			c := circle(t, b, vector{x, 0, 8}, vector{0, 0, -1}, 2, 8)
			tools := []string{a.Handle, c.Handle}
			if reverse {
				tools[0], tools[1] = tools[1], tools[0]
			}
			cut := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
			area, perimeter := 4*math.Pi, 4*math.Pi
			if x != 0 {
				area, perimeter = 16*math.Pi/3+2*math.Sqrt(3), 16*math.Pi/3
			}
			near(t, cut.GeometryAssertions.Volume, 800-area*8)
			near(t, cut.GeometryAssertions.SurfaceArea, 520+perimeter*8-2*area)
			body := b.shapes[cut.Handle].bore
			p, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, cut.GeometryAssertions.Volume)
			for _, z := range []float64{0, 8} {
				f, err := body.planarFace(vector{0, 0, z}.vec(), vector{0, 0, 1}.vec())
				if err != nil {
					t.Fatal(err)
				}
				near(t, f.Area, 100-area)
				want := -1.
				if z == 8 {
					want = 1
				}
				near(t, f.Normal[2], want)
			}
			near(t, b.shapes[a.Handle].cylinder.frame.Origin().Z, 0)
			near(t, b.shapes[c.Handle].cylinder.frame.Origin().Z, 8)
			near(t, b.shapes[c.Handle].cylinder.frame.ZAxis().Z, -1)
			if cut.Bounds != base.Bounds {
				t.Fatal("opposed-axis normalization changed bounds")
			}
		}
	}
}

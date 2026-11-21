package plaincadbridge

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
)

func TestUnequalDepthBoreAxesSignsAndCaps(t *testing.T) {
	unionArea, unionPerimeter := 16*math.Pi/3+2*math.Sqrt(3), 16*math.Pi/3
	diskArea, diskPerimeter := 4*math.Pi, 4*math.Pi
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, deep := range []float64{6, 10} {
				b := New()
				base := box(t, b, vector{-5, -5, -5}, vector{5, 5, 5})
				n := vector{}
				n[axis] = sign
				var tools []string
				for i, depth := range []float64{3, deep} {
					o := vector{}
					o[axis] = -5 * sign
					o[(axis+1)%3] = float64(2*i - 1)
					tool := circle(t, b, o, n, 2, depth)
					tools = append(tools, tool.Handle)
				}
				cut := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
				removed := unionArea*3 + diskArea*(deep-3)
				area := 600 + unionPerimeter*3 + diskPerimeter*(deep-3)
				if deep == 10 {
					area -= 2 * diskArea
				}
				near(t, cut.GeometryAssertions.Volume, 1000-removed)
				near(t, cut.GeometryAssertions.SurfaceArea, area)
				solid := boundary(t, b, cut.Handle)
				p, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil {
					t.Fatal(err)
				}
				near(t, p.Volume, cut.GeometryAssertions.Volume)
				near(t, p.Area, area)
				for _, tc := range []struct{ depth, area, normal float64 }{{0, 100 - unionArea, -sign}, {3, unionArea - diskArea, -sign}} {
					o := vector{}
					o[axis] = (-5 + tc.depth) * sign
					f, err := planarFace(t, b, cut.Handle, o, n)
					if err != nil {
						t.Fatal(err)
					}
					near(t, f.Area, tc.area)
					near(t, f.Normal[axis], tc.normal)
				}
				o := vector{}
				o[axis] = (-5 + deep) * sign
				f, err := planarFace(t, b, cut.Handle, o, n)
				if err != nil {
					t.Fatal(err)
				}
				want, normal := diskArea, -sign
				if deep == 10 {
					want, normal = 100-diskArea, sign
				}
				near(t, f.Area, want)
				near(t, f.Normal[axis], normal)
				m, err := b.shapes[cut.Handle].tessellate(request{})
				if err != nil {
					t.Fatal(err)
				}
				if err = m.ValidateSolid(); err != nil {
					t.Fatal(err)
				}
				if cut.Bounds != base.Bounds {
					t.Fatal("layered bore changed outer bounds")
				}
			}
		}
	}
}
func overlapMeasure(t *testing.T, b *Bridge, a, c string) float64 {
	t.Helper()
	before, next := len(b.shapes), b.next
	var result struct {
		Volume float64 `json:"volume"`
	}
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": a, "b": c}), &result); err != nil {
		t.Fatal(err)
	}
	if len(b.shapes) != before || b.next != next {
		t.Fatal("measure query allocated a handle")
	}
	return result.Volume
}
func TestSequentialHoleOppositeBlindEntryAndOverlapQuery(t *testing.T) {
	unionArea := 16*math.Pi/3 + 2*math.Sqrt(3)
	diskArea := 4 * math.Pi
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			base := box(t, b, vector{-5, -5, -5}, vector{5, 5, 5})
			n := vector{}
			n[axis] = sign
			o := vector{}
			o[axis] = -5 * sign
			o[(axis+1)%3] = -1
			first := circle(t, b, o, n, 2, 10)
			stock := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": first.Handle})
			o[axis] = 2 * sign
			o[(axis+1)%3] = 1
			second := circle(t, b, o, n, 2, 3)
			want := (unionArea - diskArea) * 3
			near(t, overlapMeasure(t, b, stock.Handle, second.Handle), want)
			near(t, overlapMeasure(t, b, second.Handle, stock.Handle), want)
			cut := shape(t, b, map[string]any{"op": "cutMany", "a": stock.Handle, "tools": []string{second.Handle}})
			near(t, cut.GeometryAssertions.Volume, stock.GeometryAssertions.Volume-want)
			near(t, overlapMeasure(t, b, cut.Handle, second.Handle), 0)
			before := len(b.shapes)
			reply := b.Call(`{"op":"cut","a":"` + cut.Handle + `","b":"` + second.Handle + `"}`)
			if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
				t.Fatal(reply)
			}
			moved := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
			near(t, moved.GeometryAssertions.Volume, cut.GeometryAssertions.Volume)
		}
	}
}
func TestOverlapVolumeAtFullHandleCapacity(t *testing.T) {
	b := New()
	base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 8})
	tool := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 2, 8)
	stock := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	second := circle(t, b, vector{1, 0, 0}, vector{0, 0, 1}, 2, 3)
	for len(b.shapes) < MaxHandles {
		shape(t, b, map[string]any{"op": "clone", "handle": base.Handle})
	}
	volume := overlapMeasure(t, b, stock.Handle, second.Handle)
	if volume <= 0 {
		t.Fatal("missed existing-stock overlap")
	}
	near(t, overlapMeasure(t, b, stock.Handle, tool.Handle), 0)
}

func TestOverlapVolumePreservesOrdinaryQueriesAndCommonGeometry(t *testing.T) {
	b := newLegacy()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 8})
	planar := box(t, b, vector{0, 0, 0}, vector{8, 8, 8})
	near(t, overlapMeasure(t, b, base.Handle, planar.Handle), 128)
	near(t, overlapMeasure(t, b, planar.Handle, base.Handle), 128)
	circleTool := circle(t, b, vector{0, 0, -2}, vector{0, 0, 1}, 1, 5)
	near(t, overlapMeasure(t, b, base.Handle, circleTool.Handle), 3*math.Pi)
	near(t, overlapMeasure(t, b, circleTool.Handle, base.Handle), 3*math.Pi)
	through := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 1, 8)
	stock := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": through.Handle})
	second := circle(t, b, vector{1, 0, 0}, vector{0, 0, 1}, 1, 4)
	if overlapMeasure(t, b, stock.Handle, second.Handle) <= 0 {
		t.Fatal("missed overlapping material")
	}
	// The legacy paths have no geometry for the common of a tool with a
	// bored stock and decline it. The general Boolean builds it: the unit
	// disk less its lens with the unit bore one away, over a height of 4.
	crescent := declined(t, b, "", "common", stock.Handle, second.Handle)
	near(t, crescent.GeometryAssertions.Volume, 4*(math.Pi/3+math.Sqrt(3)/2))
	grazing := circle(t, b, vector{3.5, 0, 0}, vector{0, 0, 1}, 1, 8)
	before := len(b.shapes)
	want := 8 * (math.Pi - math.Acos(.5) + .5*math.Sqrt(.75))
	near(t, overlapMeasure(t, b, base.Handle, grazing.Handle), want)
	near(t, overlapMeasure(t, b, grazing.Handle, base.Handle), want)
	if len(b.shapes) != before {
		t.Fatal("side-clipped overlap allocated a handle")
	}
}

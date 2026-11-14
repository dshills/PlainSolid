package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestSideOpeningPairScopedAndSuccessive(t *testing.T) {
	for _, depths := range [][2]float64{{6, 6}, {2, 6}, {5, 8}, {8, 8}} {
		b := New()
		stock := box(t, b, vector{5, 8, 10}, vector{25, 28, 18})
		left := circle(t, b, vector{13, 8, 10}, vector{0, 0, 1}, 3, depths[0])
		right := circle(t, b, vector{17, 8, 10}, vector{0, 0, 1}, 3, depths[1])
		shared := 9*math.Acos(2./3) - math.Sqrt(20)
		disk := 4.5 * math.Pi
		want := 3200 - disk*(depths[0]+depths[1]) + shared*min(depths[0], depths[1])
		single := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": left.Handle})
		near(t, overlapMeasure(t, b, single.Handle, right.Handle), single.GeometryAssertions.Volume-want)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{stock, []string{left.Handle, right.Handle}},
			{stock, []string{right.Handle, left.Handle}},
			{single, []string{right.Handle}},
		} {
			out, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(depths, err)
			}
			near(t, out.GeometryAssertions.Volume, want)
			if out.Bounds != stock.Bounds {
				t.Fatal(out.Bounds, stock.Bounds)
			}
			near(t, overlapMeasure(t, b, out.Handle, left.Handle), 0)
			mesh, err := b.shapes[out.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			moved := shape(t, b, map[string]any{"op": "transform", "handle": out.Handle, "matrix": [][]float64{{1, 0, 0, 0}, {0, 0, -1, 10}, {0, 1, 0, 20}}})
			export, err := b.exportSTEP([]string{moved.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(export.Base64)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := b.importSTEP(string(data))
			if err != nil {
				t.Fatal(err)
			}
			near(t, restored.GeometryAssertions.Volume, want)
			if restored.Bounds != moved.Bounds {
				t.Fatal(restored.Bounds, moved.Bounds)
			}
		}
	}
}

func TestSideOpeningPairOpposedScopes(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	lower := circle(t, b, vector{8, 0, 0}, vector{0, 0, 1}, 3, 5)
	upper := circle(t, b, vector{12, 0, 8}, vector{0, 0, -1}, 3, 6)
	shared := 9*math.Acos(2./3) - math.Sqrt(20)
	want := 3200 - 4.5*math.Pi*11 + 3*shared
	for _, pair := range [][2]Shape{{lower, upper}, {upper, lower}} {
		single := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pair[0].Handle})
		near(t, overlapMeasure(t, b, single.Handle, pair[1].Handle), single.GeometryAssertions.Volume-want)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{stock, []string{pair[0].Handle, pair[1].Handle}},
			{single, []string{pair[1].Handle}},
		} {
			out, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(err)
			}
			near(t, out.GeometryAssertions.Volume, want)
			near(t, overlapMeasure(t, b, out.Handle, pair[1].Handle), 0)
			if _, err := b.exportSTEP([]string{out.Handle}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSideOpeningPairDifferentStockSides(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		stockSize, first, next vector
		axis                   vector
		overlap, firstOffset   float64
	}{
		{"adjacent", vector{20, 20, 8}, vector{3.5, 0, 0}, vector{0, 3.5, 0}, vector{0, 0, 1}, 5, 0},
		{"opposite", vector{20, 8, 8}, vector{8, 1.5, 0}, vector{9, 6.5, 8}, vector{0, 0, -1}, 3, 1.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const radius = 3
			distance := math.Hypot(tc.next[0]-tc.first[0], tc.next[1]-tc.first[1])
			lens := 2*radius*radius*math.Acos(distance/(2*radius)) - distance*math.Sqrt(radius*radius-distance*distance/4)
			chord := math.Sqrt(radius*radius - tc.firstOffset*tc.firstOffset)
			segment := radius*radius*(math.Pi/2+math.Asin(tc.firstOffset/radius)) + tc.firstOffset*chord
			want := tc.stockSize[0]*tc.stockSize[1]*tc.stockSize[2] - 11*segment + tc.overlap*lens
			b := New()
			stock := box(t, b, vector{}, tc.stockSize)
			first := circle(t, b, tc.first, vector{0, 0, 1}, radius, 5)
			next := circle(t, b, tc.next, tc.axis, radius, 6)
			for _, pair := range [][2]Shape{{first, next}, {next, first}} {
				single := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pair[0].Handle})
				near(t, overlapMeasure(t, b, single.Handle, pair[1].Handle), single.GeometryAssertions.Volume-want)
				for _, cases := range []struct {
					base  Shape
					tools []string
				}{
					{stock, []string{pair[0].Handle, pair[1].Handle}},
					{single, []string{pair[1].Handle}},
				} {
					out, err := b.cutMany(cases.base.Handle, cases.tools)
					if err != nil {
						t.Fatal(err)
					}
					near(t, out.GeometryAssertions.Volume, want)
					if out.Bounds != stock.Bounds {
						t.Fatal(out.Bounds, stock.Bounds)
					}
					m, err := b.shapes[out.Handle].tessellate(request{})
					if err != nil {
						t.Fatal(err)
					}
					if err := m.ValidateSolid(); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestSideOpeningCutsRetainHistoryAndAcceptFurtherHoles(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{26, 20, 8})
	var drills []Shape
	for i, depth := range []float64{3, 6, 4, 8} {
		drills = append(drills, circle(t, b, vector{6 + 4*float64(i), 0, 0}, vector{0, 0, 1}, 3, depth))
	}
	third := func(order []int) Shape {
		t.Helper()
		handles := make([]string, len(order))
		for i, id := range order {
			handles[i] = drills[id].Handle
		}
		out, err := b.cutMany(stock.Handle, handles)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	firstThree := third([]int{0, 1, 2})
	reordered := third([]int{2, 0, 1, 1})
	near(t, reordered.GeometryAssertions.Volume, firstThree.GeometryAssertions.Volume)
	near(t, reordered.GeometryAssertions.SurfaceArea, firstThree.GeometryAssertions.SurfaceArea)
	exported, err := b.exportSTEP([]string{firstThree.Handle})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(exported.Base64)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	removed := overlapMeasure(t, b, restored.Handle, drills[3].Handle)
	four := shape(t, b, map[string]any{"op": "cut", "a": restored.Handle, "b": drills[3].Handle})
	groupedFour := third([]int{3, 2, 1, 0})
	near(t, four.GeometryAssertions.Volume, firstThree.GeometryAssertions.Volume-removed)
	near(t, four.GeometryAssertions.Volume, groupedFour.GeometryAssertions.Volume)
	near(t, four.GeometryAssertions.SurfaceArea, groupedFour.GeometryAssertions.SurfaceArea)
	near(t, overlapMeasure(t, b, four.Handle, drills[3].Handle), 0)
	near(t, overlapMeasure(t, b, firstThree.Handle, drills[3].Handle), removed)
	mirrored := shape(t, b, map[string]any{"op": "transform", "handle": four.Handle, "matrix": [][]float64{
		{-1, 0, 0, 26}, {0, 1, 0, 0}, {0, 0, 1, 0},
	}})
	if _, err := b.exportSTEP([]string{mirrored.Handle}); err != nil {
		t.Fatal(err)
	}
	near(t, overlapMeasure(t, b, four.Handle, drills[3].Handle), 0)

	// A tangent extra tool cannot publish a partial grouped result.
	tangent := circle(t, b, vector{24, 0, 0}, vector{0, 0, 1}, 3, 4)
	count := len(b.shapes)
	if _, err := b.cutMany(four.Handle, []string{tangent.Handle}); err == nil || len(b.shapes) != count {
		t.Fatal("unsupported cutter published a shape", err)
	}
}

// Exercise the handoff between two independently supported representations.
// The first cuts use the original exact path; adding a third rebuilds the
// complete stock from the original tool descriptors.
func TestSideOpeningCutsContinueStrictDualDisk(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 4, 0}, vector{15, 10, 6})
	first := circle(t, b, vector{3.5, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	second := circle(t, b, vector{7.5, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	third := circle(t, b, vector{11.5, 4, 0}, vector{0, 0, 1}, 2.5, 5)
	pair, err := b.cutMany(stock.Handle, []string{first.Handle, second.Handle})
	if err != nil || b.shapes[pair.Handle].analytic.sideDualDisk == nil {
		t.Fatal("expected the original exact two-cut family", pair, err)
	}
	removed := overlapMeasure(t, b, pair.Handle, third.Handle)
	combined := shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": third.Handle})
	r := b.shapes[combined.Handle].analytic.sideOpeningCuts
	if r == nil || len(r.Cuts) != 3 || combined.Bounds != stock.Bounds {
		t.Fatal("the grouped body must inherit the original stock and all three cutters", r, combined.Bounds)
	}
	grouped, err := b.cutMany(stock.Handle, []string{third.Handle, second.Handle, first.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, combined.GeometryAssertions.Volume, grouped.GeometryAssertions.Volume)
	near(t, combined.GeometryAssertions.SurfaceArea, grouped.GeometryAssertions.SurfaceArea)
	near(t, combined.GeometryAssertions.Volume, pair.GeometryAssertions.Volume-removed)
	data, err := writeSTEPBody(b.shapes[combined.Handle])
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := b.importSTEP(string(data))
	if err != nil || b.shapes[loaded.Handle].analytic.sideOpeningCuts == nil {
		t.Fatal("grouped cutter history must survive STEP reload", err)
	}
	near(t, loaded.GeometryAssertions.Volume, combined.GeometryAssertions.Volume)
}

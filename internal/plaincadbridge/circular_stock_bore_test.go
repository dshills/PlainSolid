package plaincadbridge

import (
	"encoding/base64"
	"math"
	"os"
	"strings"
	"testing"
)

func TestCircularStockFiniteEccentricHoleAndSTEP(t *testing.T) {
	for _, tc := range []struct {
		origin, axis  vector
		height, depth float64
		through       bool
	}{
		{vector{2, 0, -1}, vector{0, 0, 1}, 10, 8, true},
		{vector{2, 0, 8}, vector{0, 0, -1}, 3, 3, false},
		{vector{2, 0, 0}, vector{0, 0, 1}, 3, 3, false},
		{vector{2, 0, 5}, vector{0, 0, 1}, 3, 3, false},
	} {
		b := New()
		base := circle(t, b, vector{}, vector{0, 0, 1}, 5, 8)
		tool := circle(t, b, tc.origin, tc.axis, 1, tc.height)
		near(t, overlapMeasure(t, b, base.Handle, tool.Handle), math.Pi*tc.depth)
		near(t, overlapMeasure(t, b, tool.Handle, base.Handle), math.Pi*tc.depth)
		for _, q := range []map[string]any{
			{"op": "cut", "a": base.Handle, "b": tool.Handle},
			{"op": "cutMany", "a": base.Handle, "tools": []string{tool.Handle}},
		} {
			hole := shape(t, b, q)
			near(t, hole.GeometryAssertions.Volume, (200-tc.depth)*math.Pi)
			area := 130*math.Pi + 2*math.Pi*tc.depth
			if tc.through {
				area -= 2 * math.Pi
			}
			near(t, hole.GeometryAssertions.SurfaceArea, area)
			s := b.shapes[hole.Handle]
			mesh, err := s.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if _, err := b.exportSTEP([]string{hole.Handle}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSTEPIndependentlyAuthoredRoundBlindAndThroughBores(t *testing.T) {
	data, err := os.ReadFile("../../exchange/step/testdata/disk-pocket-occt.step")
	if err != nil {
		t.Fatal(err)
	}
	b := New()
	s, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	p := b.shapes[s.Handle].analytic.stepDisk
	if p == nil || len(p.Holes) != 1 || len(p.Blind) != 1 || p.Outer.Radius != 4 || p.Height != 3 || p.Blind[0].Depth != 1.25 {
		t.Fatal("independent STEP round stock was not preserved")
	}
	if _, err := b.exportSTEP([]string{s.Handle}); err != nil {
		t.Fatal(err)
	}
}

func TestCircularStockGroupedAndSuccessiveDisjointBores(t *testing.T) {
	b := New()
	stock := circle(t, b, vector{}, vector{0, 0, 1}, 6, 8)
	through := circle(t, b, vector{-3, 0, -1}, vector{0, 0, 1}, 1, 10)
	blind := circle(t, b, vector{3, 0, 8}, vector{0, 0, -1}, 1, 3)
	contained := circle(t, b, vector{3, 0, 8}, vector{0, 0, -1}, .5, 2)
	separate := circle(t, b, vector{0, 3, 6}, vector{0, 0, 1}, 1, 2)
	intersecting := circle(t, b, vector{-2, 0, 8}, vector{0, 0, -1}, 1, 2)
	tangent := circle(t, b, vector{-1, 0, 8}, vector{0, 0, -1}, 1, 2)
	cut := func(stock, tool Shape) Shape {
		return shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	}
	before := len(b.shapes)
	for _, ids := range [][]string{{contained.Handle, through.Handle, blind.Handle, through.Handle}, {blind.Handle, contained.Handle, through.Handle}} {
		grouped, err := b.cutMany(stock.Handle, ids)
		if err != nil {
			t.Fatal(err)
		}
		before++
		if len(b.shapes) != before {
			t.Fatal("group did not publish exactly one solid")
		}
		near(t, grouped.GeometryAssertions.Volume, 277*math.Pi)
		near(t, grouped.GeometryAssertions.SurfaceArea, 188*math.Pi)
	}
	first := cut(stock, through)
	for _, tool := range []Shape{blind, separate} {
		want := tool.GeometryAssertions.Volume
		near(t, overlapMeasure(t, b, first.Handle, tool.Handle), want)
		near(t, overlapMeasure(t, b, tool.Handle, first.Handle), want)
	}
	near(t, overlapMeasure(t, b, first.Handle, intersecting.Handle), 2*(math.Pi/3+math.Sqrt(3)/2))
	second := cut(first, blind)
	near(t, overlapMeasure(t, b, second.Handle, blind.Handle), 0)
	near(t, overlapMeasure(t, b, second.Handle, contained.Handle), 0)
	before = len(b.shapes)
	for _, tools := range [][]string{{contained.Handle}, {separate.Handle, tangent.Handle}, {separate.Handle, stock.Handle}} {
		if _, err := b.cutMany(second.Handle, tools); err == nil || len(b.shapes) != before {
			t.Fatalf("invalid group changed handles: %v", err)
		}
	}
	file, err := b.exportSTEP([]string{second.Handle})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	reread, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	final := cut(reread, separate)
	near(t, final.GeometryAssertions.Volume, 275*math.Pi)
	near(t, final.GeometryAssertions.SurfaceArea, 192*math.Pi)
	body := b.shapes[final.Handle]
	mesh, err := body.tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	drawing, err := body.drawingGeometry()
	if err != nil || len(drawing.Bores) != 3 || len(drawing.Warnings) != 0 {
		t.Fatalf("unexpected drawing bores %+v: %v", drawing, err)
	}
}

func TestCircularStockAuthoredEccentricInnerWireAndSignedCut(t *testing.T) {
	b := New()
	profile := mixedFixture("disk")
	profile.Center, profile.Height = []number{2, 0}, -5
	profile.Circles = []mixedProfileCircle{{Center: []number{4, 0}, Radius: 1}}
	base, err := b.mixedExtrude(profile)
	if err != nil {
		t.Fatal(err)
	}
	tool := circle(t, b, vector{-2, 0, 8}, vector{0, 0, 1}, 1, 2)
	near(t, overlapMeasure(t, b, base.Handle, tool.Handle), 2*math.Pi)
	hole, err := b.cutMany(base.Handle, []string{tool.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, hole.GeometryAssertions.Volume, 493*math.Pi)
	near(t, hole.GeometryAssertions.SurfaceArea, 312*math.Pi)
	outside := circle(t, b, vector{11.5, 0, 8}, vector{0, 0, 1}, 1, 2)
	if _, err := b.cutMany(base.Handle, []string{outside.Handle}); err == nil || !strings.Contains(err.Error(), "outside wall") {
		t.Fatalf("expected side-crossing rejection, got %v", err)
	}
}

func TestCircularStockDisjointOpposedBoresAndSTEP(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		b := New()
		stock := circle(t, b, vector{}, vector{0, 0, 1}, 6, 8)
		lower := circle(t, b, vector{-3, 0, 0}, vector{0, 0, 1}, 1, 6)
		upper := circle(t, b, vector{3, 0, 8}, vector{0, 0, -1}, 1, 5)
		through := circle(t, b, vector{0, 3, 0}, vector{0, 0, 1}, 1, 8)
		if reverse {
			lower, upper = upper, lower
		}
		for _, grouped := range []bool{false, true} {
			base := stock
			ids := []string{lower.Handle, upper.Handle, through.Handle}
			if !grouped {
				base = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": lower.Handle})
				ids = ids[1:]
				near(t, overlapMeasure(t, b, base.Handle, upper.Handle), upper.GeometryAssertions.Volume)
			}
			solid, err := b.cutMany(base.Handle, ids)
			if err != nil {
				t.Fatal(err)
			}
			near(t, solid.GeometryAssertions.Volume, 269*math.Pi)
			near(t, solid.GeometryAssertions.SurfaceArea, 204*math.Pi)
			for _, tool := range []Shape{lower, upper, through} {
				near(t, overlapMeasure(t, b, solid.Handle, tool.Handle), 0)
			}
			file, err := b.exportSTEP([]string{solid.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
			read, err := b.importSTEP(string(data))
			if err != nil {
				t.Fatal(err)
			}
			near(t, read.GeometryAssertions.Volume, solid.GeometryAssertions.Volume)
			near(t, read.GeometryAssertions.SurfaceArea, solid.GeometryAssertions.SurfaceArea)
			for _, tool := range []Shape{lower, upper, through} {
				near(t, overlapMeasure(t, b, read.Handle, tool.Handle), 0)
			}
			mesh, err := b.shapes[read.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCircularStockOverlappingOpposedAndSequentialBores(t *testing.T) {
	b := New()
	stock := circle(t, b, vector{}, vector{0, 0, 1}, 6, 8)
	lower := circle(t, b, vector{-1, 0, 0}, vector{0, 0, 1}, 2, 6)
	upper := circle(t, b, vector{1, 0, 8}, vector{0, 0, -1}, 2, 5)
	probe := circle(t, b, vector{0, 4, 0}, vector{0, 0, 1}, 1, 2)
	lens := 8*math.Pi/3 - 2*math.Sqrt(3)
	for _, reverse := range []bool{false, true} {
		if reverse {
			lower, upper = upper, lower
		}
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": lower.Handle})
		near(t, overlapMeasure(t, b, first.Handle, upper.Handle), upper.GeometryAssertions.Volume-3*lens)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{stock, []string{lower.Handle, upper.Handle}},
			{first, []string{upper.Handle}},
		} {
			body, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(err)
			}
			near(t, body.GeometryAssertions.Volume, 244*math.Pi+3*lens)
			near(t, body.GeometryAssertions.SurfaceArea, 204*math.Pi-2*lens)
			near(t, overlapMeasure(t, b, body.Handle, lower.Handle), 0)
			near(t, overlapMeasure(t, b, body.Handle, upper.Handle), 0)
			near(t, overlapMeasure(t, b, body.Handle, probe.Handle), 2*math.Pi)
			mesh, err := b.shapes[body.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			file, err := b.exportSTEP([]string{body.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
			reread, err := b.importSTEP(string(data))
			if err != nil {
				t.Fatal(err)
			}
			near(t, reread.GeometryAssertions.Volume, body.GeometryAssertions.Volume)
			near(t, reread.GeometryAssertions.SurfaceArea, body.GeometryAssertions.SurfaceArea)
			later, err := b.cutMany(reread.Handle, []string{probe.Handle})
			if err != nil {
				t.Fatal(err)
			}
			near(t, later.GeometryAssertions.Volume, body.GeometryAssertions.Volume-2*math.Pi)
		}
	}
}

func TestCircularStockCoaxialOpposedSTEPAndThroughCut(t *testing.T) {
	b := New()
	stock := circle(t, b, vector{}, vector{0, 0, 1}, 6, 8)
	lower := circle(t, b, vector{1, 0, 0}, vector{0, 0, 1}, 2, 2)
	upper := circle(t, b, vector{1, 0, 8}, vector{0, 0, -1}, 2, 3)
	through := circle(t, b, vector{1, 0, 0}, vector{0, 0, 1}, 2, 8)
	cut, err := b.cutMany(stock.Handle, []string{lower.Handle, upper.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, cut.GeometryAssertions.Volume, 268*math.Pi)
	near(t, overlapMeasure(t, b, cut.Handle, through.Handle), 12*math.Pi)
	file, err := b.exportSTEP([]string{cut.Handle})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	read, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	near(t, overlapMeasure(t, b, read.Handle, through.Handle), 12*math.Pi)
	opened, err := b.cutMany(read.Handle, []string{through.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, opened.GeometryAssertions.Volume, 256*math.Pi)
}

package plaincadbridge

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestPlainCADSideOpenDualDiskCuts(t *testing.T) {
	for _, depth := range []float64{3, 6} {
		bridge := New()
		stock := box(t, bridge, vector{0, 4, 0}, vector{10, 10, 6})
		left := circle(t, bridge, vector{3.5, 4, 6}, vector{0, 0, -1}, 2.5, depth)
		// Reversed drill direction, but the same clipped axial interval.
		right := circle(t, bridge, vector{6.5, 4, 6 - depth}, vector{0, 0, 1}, 2.5, depth)
		first := shape(t, bridge, map[string]any{"op": "cut", "a": stock.Handle, "b": left.Handle})
		angle := math.Pi - math.Acos(.6)
		opening := 6.25*angle + 3
		volume := 360 - depth*opening
		area := 312 + (5*angle-8)*depth
		if depth == 6 {
			area -= 2 * opening
		}
		near(t, overlapMeasure(t, bridge, first.Handle, right.Handle), depth*(opening-6.25*math.Pi/2))
		near(t, overlapMeasure(t, bridge, right.Handle, first.Handle), depth*(opening-6.25*math.Pi/2))
		for _, request := range []map[string]any{
			{"op": "cutMany", "a": stock.Handle, "tools": []string{left.Handle, right.Handle}},
			{"op": "cutMany", "a": stock.Handle, "tools": []string{right.Handle, left.Handle}},
			{"op": "cut", "a": first.Handle, "b": right.Handle},
			{"op": "cutMany", "a": first.Handle, "tools": []string{right.Handle}},
		} {
			out := shape(t, bridge, request)
			near(t, out.GeometryAssertions.Volume, volume)
			near(t, out.GeometryAssertions.SurfaceArea, area)
			if out.Bounds != stock.Bounds || out.GeometryAssertions.SolidCount != 1 {
				t.Fatal("dual cut lost stock extrema or solid ownership", out)
			}
			for _, tool := range []Shape{left, right} {
				near(t, overlapMeasure(t, bridge, out.Handle, tool.Handle), 0)
			}
			data, err := writeSTEPBody(bridge.shapes[out.Handle])
			if err != nil || bytes.Count(data, []byte("CYLINDRICAL_SURFACE(")) != 2 || bytes.Contains(data, []byte("PCURVE(")) {
				t.Fatal("retained exact STEP topology", err)
			}
			read, err := bridge.importSTEP(string(data))
			if err != nil || bridge.shapes[read.Handle].analytic.sideDualDisk == nil || read.Bounds != out.Bounds {
				t.Fatal("strict native STEP re-import", read, err)
			}
			near(t, read.GeometryAssertions.Volume, volume)
		}
		out := shape(t, bridge, map[string]any{"op": "cut", "a": first.Handle, "b": right.Handle})
		display, err := bridge.shapes[out.Handle].tessellate(request{})
		if err != nil {
			t.Fatal("default browser tessellation", err)
		}
		if err := display.ValidateSolid(); err != nil || len(display.Triangles) > 4000 {
			t.Fatal("default browser mesh is open or excessively large", len(display.Triangles), err)
		}
		bodyAt := bridge.shapes[out.Handle].analytic
		entryNormal := geom.Vec3{Z: 1}
		if _, err := bodyAt.planarFace(geom.Vec3{X: 5, Y: 5, Z: 6}, entryNormal); err == nil {
			t.Fatal("the missing part of an entry cap became a selectable face")
		}
		if depth == 3 {
			floor, err := bodyAt.planarFace(geom.Vec3{X: 5, Y: 5, Z: 3}, entryNormal)
			if err != nil || floor.Normal != (vector{0, 0, 1}) || math.Abs(floor.Area-opening) > 1e-4 {
				t.Fatal("wrong finite blind floor", floor, err)
			}
			_, err = bodyAt.planarFace(geom.Vec3{X: 10, Y: 8, Z: 3}, geom.Vec3{X: 1})
			if err == nil {
				t.Fatal("shared stock partition must not select an arbitrary face")
			}
		}
		control := box(t, bridge, vector{20, 1, 2}, vector{22, 4, 6})
		published := len(bridge.shapes)
		stepExport, err := bridge.exportSTEP([]string{out.Handle, control.Handle})
		if err != nil || len(stepExport.After) != 2 || len(bridge.shapes) != published {
			t.Fatal("mixed rooted STEP export did not independently read back both bodies", err)
		}
		near(t, stepExport.After[0].Volume, volume)
		near(t, stepExport.After[1].Volume, 24)
		if stepExport.After[0].Bounds != out.Bounds || stepExport.After[1].Bounds != control.Bounds {
			t.Fatal("mixed readback changed placement", stepExport.After)
		}
		body, err := bridge.shapes[out.Handle].analytic.transformed(geom.Affine{{-1, 0, 0, 12}, {0, 1, 0, 20}, {0, 0, 1, 1}})
		if err != nil || body.sideDualDisk == nil {
			t.Fatal("cardinal reflection lost dual cut recipe", err)
		}
		result, err := bridge.putAnalyticBody(body)
		if err != nil || result.Bounds != (Bounds{Min: vector{2, 24, 1}, Max: vector{12, 30, 7}}) {
			t.Fatal("reflected stock extrema", result.Bounds, err)
		}
	}
}

func TestPlainCADSideOpenDualDiskSTEPUnits(t *testing.T) {
	frame, err := geom.NewFrame(geom.Vec3{Y: 4}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	d := step.SideOpenDualDiskDescriptor{Name: "Pocket pair", Frame: frame, Max: geom.Vec2{X: 10, Y: 6},
		Discs:  [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5}, Radius: 2.5}},
		Height: 6, Depth: 3, Tolerance: 1e-9, LengthUnit: .01}
	var source bytes.Buffer
	if err := step.WriteSideOpenDualDiskProduct(context.Background(), &source, d); err != nil {
		t.Fatal(err)
	}
	b := New()
	s, err := b.importSTEP(source.String())
	if err != nil || b.shapes[s.Handle].analytic.sideDualDisk == nil || s.Bounds != (Bounds{Min: vector{0, 40, 0}, Max: vector{100, 100, 60}}) {
		t.Fatal("centimeter model must retain exact millimeter recipe", s, err)
	}
	near(t, s.GeometryAssertions.Volume, 1000*(360-3*(6.25*(math.Pi-math.Acos(.6))+3)))
	near(t, s.GeometryAssertions.SurfaceArea, 100*(312+3*(5*(math.Pi-math.Acos(.6))-8)))
}

func TestPlainCADUnequalSideOpenDualDiskCuts(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{60, 22, 6})
	left := circle(t, b, vector{20, 0, 0}, vector{0, 0, 1}, 13, 3)
	right := circle(t, b, vector{34, 0, 3}, vector{0, 0, -1}, 15, 3)
	first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": left.Handle})
	angleL, angleR := math.Pi/2+math.Asin(5./13), math.Pi/2+math.Asin(.6)
	opening := 84 + (169*angleL+225*angleR)/2
	volume, area := 7920-3*opening, 3624+3*(13*angleL+15*angleR-42)
	near(t, overlapMeasure(t, b, first.Handle, right.Handle), 3*(opening-169*math.Pi/2))
	for _, request := range []map[string]any{
		{"op": "cutMany", "a": stock.Handle, "tools": []string{left.Handle, right.Handle}},
		{"op": "cutMany", "a": stock.Handle, "tools": []string{right.Handle, left.Handle}},
		{"op": "cut", "a": first.Handle, "b": right.Handle},
	} {
		out := shape(t, b, request)
		near(t, out.GeometryAssertions.Volume, volume)
		near(t, out.GeometryAssertions.SurfaceArea, area)
		near(t, overlapMeasure(t, b, out.Handle, left.Handle), 0)
		if out.Bounds != stock.Bounds {
			t.Fatal("unequal cut lost its stock extrema", out.Bounds)
		}
		x, err := b.exportSTEP([]string{out.Handle})
		if err != nil || len(x.After) != 1 || x.After[0].Bounds != out.Bounds {
			t.Fatal("unequal exact export failed real readback", x.After, err)
		}
		near(t, x.After[0].Volume, volume)
	}
	final := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": right.Handle})
	var d, a optionalNumber
	d.set, d.value = true, .5
	a.set, a.value = true, .2
	m, err := b.shapes[final.Handle].tessellate(request{LinearDeflection: d, AngularDeflection: a})
	if err != nil {
		t.Fatal("unequal display", err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal("unequal display lost manifold", err)
	}
}

func TestPlainCADOffCenterSideOpenDualDisk(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{50, 12, 6})
	a := circle(t, b, vector{20, 0, 0}, vector{0, 0, 1}, 5, 3)
	c := circle(t, b, vector{24.5, 0, 0}, vector{0, 0, 1}, 8.5, 3)
	prior := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": a.Handle})
	left := math.Atan2(4, 3)
	right := math.Atan2(4, -7.5)
	opening := (25*left + 8.5*8.5*right + 18) / 2
	want := 3600 - 3*opening
	near(t, overlapMeasure(t, b, prior.Handle, c.Handle), 3*(opening-25*math.Pi/2))
	for _, request := range []map[string]any{
		{"op": "cutMany", "a": stock.Handle, "tools": []string{c.Handle, a.Handle}},
		{"op": "cut", "a": prior.Handle, "b": c.Handle},
	} {
		out := shape(t, b, request)
		near(t, out.GeometryAssertions.Volume, want)
		if out.Bounds != stock.Bounds {
			t.Fatal("off-center opening changed stock extrema", out.Bounds)
		}
		proof, e := b.exportSTEP([]string{out.Handle})
		if e != nil || len(proof.After) != 1 {
			t.Fatal("off-center export/readback", e)
		}
		near(t, proof.After[0].Volume, want)
		var d, angle optionalNumber
		d.set, d.value, angle.set, angle.value = true, .5, true, .2
		m, e := b.shapes[out.Handle].tessellate(requestForSideMesh(d, angle))
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal("off-center display", e)
		}
	}
}

func requestForSideMesh(deflection, angular optionalNumber) request {
	return request{LinearDeflection: deflection, AngularDeflection: angular}
}

func TestPlainCADSideOpeningFallbackAtNonbinaryCrossing(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 4, 0}, vector{10, 10, 6})
	a := circle(t, b, vector{3.5, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	c := circle(t, b, vector{6, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	before := len(b.shapes)
	tools := []stored{b.shapes[a.Handle], b.shapes[c.Handle]}
	if out, handled, err := b.cutSideDualDisk(b.shapes[stock.Handle], tools); !handled || !errors.Is(err, geom.ErrUnrepresentable) || out.Handle != "" || len(b.shapes) != before {
		t.Fatal("strict dual disk must reject without a partial publish", out, err)
	}
	out, err := b.cutMany(stock.Handle, []string{a.Handle, c.Handle})
	if err != nil || len(b.shapes) != before+1 || b.shapes[out.Handle].analytic.sideOpeningCuts == nil || len(b.shapes[out.Handle].analytic.sideOpeningCuts.Cuts) != 2 || out.Bounds != stock.Bounds {
		t.Fatal("analytic side-opening family must verify the represented topology independently", out, err)
	}
	want := 360 - 3*(6.25*(math.Pi-math.Acos(.5))+2.5*math.Sqrt(4.6875)/2)
	near(t, out.GeometryAssertions.Volume, want)
	data, err := writeSTEPBody(b.shapes[out.Handle])
	if err != nil || bytes.Count(data, []byte("CYLINDRICAL_SURFACE(")) < 2 {
		t.Fatal("analytic cylindrical supports must survive STEP export", err)
	}
	reload, err := b.importSTEP(string(data))
	if err != nil || b.shapes[reload.Handle].analytic.sideOpeningCuts == nil || len(b.shapes[reload.Handle].analytic.sideOpeningCuts.Cuts) != 2 || reload.Bounds != stock.Bounds {
		t.Fatal("STEP readback must recover the original stock and both cutters", reload, err)
	}
	near(t, reload.GeometryAssertions.Volume, want)
	mesh, err := b.shapes[out.Handle].tessellate(request{})
	if err != nil || mesh.ValidateSolid() != nil {
		t.Fatal("display of independently accepted analytic stock must close", err)
	}
}

func TestPlainCADExactWallSideOpenDualDisk(t *testing.T) {
	for _, depth := range []float64{3, 6} {
		b := New()
		stock := box(t, b, vector{}, vector{50, 22, 6})
		a := circle(t, b, vector{20, 0, 0}, vector{0, 0, 1}, 13, depth)
		c := circle(t, b, vector{24, 0, 0}, vector{0, 0, 1}, 15, depth)
		prior := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": a.Handle})
		angleL, angleR := math.Atan2(12, 5), math.Atan2(12, -9)
		opening := (169*angleL + 225*angleR + 48) / 2
		for i, req := range []map[string]any{
			{"op": "cutMany", "a": stock.Handle, "tools": []string{c.Handle, a.Handle}},
			{"op": "cut", "a": prior.Handle, "b": c.Handle},
		} {
			out := shape(t, b, req)
			want := 6600 - depth*opening
			near(t, out.GeometryAssertions.Volume, want)
			body := b.shapes[out.Handle].analytic.sideDualDisk
			if body == nil || !body.exactWalls || out.Bounds != stock.Bounds {
				t.Fatal("exact walls did not remain on their original stock", out.Bounds)
			}
			if i == 0 {
				var deflection, angular optionalNumber
				deflection.set, deflection.value, angular.set, angular.value = true, .5, true, .2
				m, e := b.shapes[out.Handle].tessellate(requestForSideMesh(deflection, angular))
				if e != nil || m.ValidateSolid() != nil {
					t.Fatal("exact wall display", e)
				}
			}
			data, e := writeSTEPBody(b.shapes[out.Handle])
			if e != nil || !bytes.Contains(data, []byte("PRODUCT('side-cut-exact-walls'")) || bytes.Count(data, []byte("CYLINDRICAL_SURFACE(")) != 2 {
				t.Fatal("export needs registered original analytic supports", e)
			}
			read, e := b.importSTEP(string(data))
			if e != nil || read.Bounds != stock.Bounds || !b.shapes[read.Handle].analytic.sideDualDisk.exactWalls {
				t.Fatal("STEP readback failed to restore the exact-wall recipe", e)
			}
			near(t, read.GeometryAssertions.Volume, want)
			if i == 0 {
				control := box(t, b, vector{60, 5, 0}, vector{62, 8, 4})
				proof, e := b.exportSTEP([]string{control.Handle, out.Handle})
				if e != nil || len(proof.After) != 2 || proof.After[1].Bounds != out.Bounds {
					t.Fatal("mixed export must read both independent products back", proof.After, e)
				}
				near(t, proof.After[0].Volume, 24)
				near(t, proof.After[1].Volume, want)
			}
		}
	}
}

func TestPlainCADOffsetSideOpenDualDisk(t *testing.T) {
	for i, depth := range []float64{3, 6} {
		b := New()
		stock := box(t, b, vector{}, vector{40, 10, 6})
		a := circle(t, b, vector{12.5, -7.5, 0}, vector{0, 0, 1}, 65.0/8, depth)
		c := circle(t, b, vector{22, -7.5, depth}, vector{0, 0, -1}, 87.0/8, depth)
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": a.Handle})
		wantV := []float64{2282.902744168073, 2165.805488336147}[i]
		wantA := []float64{1405.432569688520, 1332.800302155755}[i]
		near(t, overlapMeasure(t, b, first.Handle, c.Handle), first.GeometryAssertions.Volume-wantV)
		for _, req := range []map[string]any{
			{"op": "cutMany", "a": stock.Handle, "tools": []string{c.Handle, a.Handle}},
			{"op": "cut", "a": first.Handle, "b": c.Handle},
		} {
			s := shape(t, b, req)
			if s.Bounds != stock.Bounds || !b.shapes[s.Handle].analytic.sideDualDisk.offsetCenters() {
				t.Fatal("cutters' Y offset or stock bounds not retained", s.Bounds)
			}
			near(t, s.GeometryAssertions.Volume, wantV)
			near(t, s.GeometryAssertions.SurfaceArea, wantA)
			near(t, overlapMeasure(t, b, s.Handle, c.Handle), 0)
			m, e := b.shapes[s.Handle].tessellate(requestForSideMesh(optionalNumber{value: .5, set: true}, optionalNumber{value: .2, set: true}))
			if e != nil {
				t.Fatal("offset STL mesh", e)
			}
			mass, e := m.MassProperties()
			if e != nil || math.Abs(mass.Volume-wantV) >= .3 {
				t.Fatal("offset STL exceeds the machining volume check", mass.Volume, wantV, e)
			}
			data, e := writeSTEPBody(b.shapes[s.Handle])
			if e != nil || !bytes.Contains(data, []byte("PRODUCT('side-cut-offset-disks'")) {
				t.Fatal("offset export did not register its product class", e)
			}
			read, e := b.importSTEP(string(data))
			if e != nil || read.Bounds != stock.Bounds || !b.shapes[read.Handle].analytic.sideDualDisk.offsetCenters() {
				t.Fatal("native offset readback", read.Bounds, e)
			}
			near(t, read.GeometryAssertions.Volume, wantV)
		}
	}
}

func TestPlainCADUnequalCenterSideOpenDualDisk(t *testing.T) {
	for i, depth := range []float64{3, 6} {
		b := New()
		stock := box(t, b, vector{}, vector{60, 10, 6})
		a := circle(t, b, vector{7, -3, 0}, vector{0, 0, 1}, 5, depth)
		c := circle(t, b, vector{30, -20, depth}, vector{0, 0, -1}, 29, depth)
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": a.Handle})
		wantV := []float64{2786.5355714934303, 1973.0711429868713}[i]
		wantA := []float64{2056.2212253330799, 1530.1328316617905}[i]
		near(t, overlapMeasure(t, b, first.Handle, c.Handle), first.GeometryAssertions.Volume-wantV)
		for _, req := range []map[string]any{
			{"op": "cutMany", "a": stock.Handle, "tools": []string{c.Handle, a.Handle}},
			{"op": "cut", "a": first.Handle, "b": c.Handle},
		} {
			s := shape(t, b, req)
			if s.Bounds != stock.Bounds || !b.shapes[s.Handle].analytic.sideDualDisk.differentCenterY() {
				t.Fatal("different source ordinates or stock bounds were lost", s.Bounds)
			}
			near(t, s.GeometryAssertions.Volume, wantV)
			near(t, s.GeometryAssertions.SurfaceArea, wantA)
			near(t, overlapMeasure(t, b, s.Handle, c.Handle), 0)
			m, e := b.shapes[s.Handle].tessellate(requestForSideMesh(optionalNumber{value: .5, set: true}, optionalNumber{value: .2, set: true}))
			if e != nil {
				t.Fatal("different-center fabrication mesh", e)
			}
			mass, e := m.MassProperties()
			if e != nil || math.Abs(mass.Volume-wantV) >= .3 {
				t.Fatal("different-center fabrication mesh lost volume accuracy", mass.Volume, wantV, e)
			}
			data, e := writeSTEPBody(b.shapes[s.Handle])
			if e != nil || !bytes.Contains(data, []byte("PRODUCT('side-cut-unequal-centers'")) {
				t.Fatal("export did not retain its verified different-center class", e)
			}
			read, e := b.importSTEP(string(data))
			if e != nil || read.Bounds != stock.Bounds || !b.shapes[read.Handle].analytic.sideDualDisk.differentCenterY() {
				t.Fatal("native different-center STEP readback", read.Bounds, e)
			}
			near(t, read.GeometryAssertions.Volume, wantV)
		}
	}
}

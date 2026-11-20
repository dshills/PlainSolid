package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestGuidedRadiusThree(t *testing.T) {
	b := New()
	base := box(t, b, vector{-40, -25, 0}, vector{40, 25, 20})
	tool := circle(t, b, vector{-10, -8, -1e-7}, vector{0, 0, 1}, 3, 20.0000001)
	shape(t, b, map[string]any{"op": "common", "a": base.Handle, "b": tool.Handle})
}

func TestBlindAllAxesAndSigns(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			base := box(t, b, vector{-10, -10, -10}, vector{10, 10, 10})
			o, n := vector{}, vector{}
			o[axis] = -20 * sign
			n[axis] = sign
			tool := circle(t, b, o, n, 2, 25)
			cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
			near(t, cut.GeometryAssertions.Volume, 8000-60*math.Pi)
			near(t, cut.GeometryAssertions.SurfaceArea, 2400+60*math.Pi)
			solid := boundary(t, b, cut.Handle)
			p, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, cut.GeometryAssertions.Volume)
			near(t, p.Area, cut.GeometryAssertions.SurfaceArea)
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .2})
			if err != nil {
				t.Fatal(err)
			}
			if err = m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			mp, err := m.MassProperties()
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(mp.Volume-cut.GeometryAssertions.Volume) > cut.GeometryAssertions.Volume*.01 {
				t.Fatal("blind display volume differs")
			}

		}
	}
}

func TestBlindScopedRaisedStockAndMixedRecipe(t *testing.T) {
	b := New()
	base := box(t, b, vector{30, 0, 20}, vector{50, 10, 30})
	tool := circle(t, b, vector{35, 5, 0}, vector{0, 0, 1}, 2, 25)
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	near(t, cut.GeometryAssertions.Volume, 2000-20*math.Pi)
	near(t, cut.GeometryAssertions.SurfaceArea, 1000+20*math.Pi)
	for _, tc := range []struct{ z, area, n float64 }{{20, 200 - 4*math.Pi, -1}, {25, 4 * math.Pi, -1}, {30, 200, 1}} {
		f, err := planarFace(t, b, cut.Handle, vector{35, 5, tc.z}, vector{0, 0, 1})
		if err != nil {
			t.Fatal(err)
		}
		near(t, f.Area, tc.area)
		near(t, f.Normal[2], tc.n)
	}
	second := circle(t, b, vector{45, 5, 0}, vector{0, 0, 1}, 1, 35)
	common := shape(t, b, map[string]any{"op": "common", "a": cut.Handle, "b": second.Handle})
	near(t, common.GeometryAssertions.Volume, 10*math.Pi)
	clone := shape(t, b, map[string]any{"op": "clone", "handle": cut.Handle})
	invoke(t, b, map[string]any{"op": "dispose", "handle": cut.Handle})
	twice := shape(t, b, map[string]any{"op": "cut", "a": clone.Handle, "b": second.Handle})
	near(t, twice.GeometryAssertions.Volume, 2000-30*math.Pi)
	near(t, twice.GeometryAssertions.SurfaceArea, 1000+38*math.Pi)
	moved := shape(t, b, map[string]any{"op": "transform", "handle": twice.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	solid := boundary(t, b, moved.Handle)
	p, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	near(t, p.Volume, twice.GeometryAssertions.Volume)
	near(t, p.Area, twice.GeometryAssertions.SurfaceArea)
	f, err := planarFace(t, b, moved.Handle, vector{32, 43, 14}, vector{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	near(t, f.Area, 4*math.Pi)
	near(t, f.Normal[0], -1)
	m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .2})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": base.Handle}).GeometryAssertions.Volume, 2000)
}
func TestRadiallyDisjointCommonAndUnsupportedCavity(t *testing.T) {
	b := New()
	base := box(t, b, vector{0, 0, 0}, vector{20, 10, 10})
	for _, x := range []float64{35, 105} {
		tool := circle(t, b, vector{x, 5, 0}, vector{0, 0, 1}, 2, 25)
		for _, pair := range [][2]string{{base.Handle, tool.Handle}, {tool.Handle, base.Handle}} {
			c := shape(t, b, map[string]any{"op": "common", "a": pair[0], "b": pair[1]})
			near(t, c.GeometryAssertions.Volume, 0)
			if c.GeometryAssertions.SolidCount != 0 {
				t.Fatal(c)
			}
		}
	}
	// A tool that ends inside the stock at both ends leaves a closed cavity.
	// No legacy path represents one. The general Boolean cuts it.
	tool := circle(t, b, vector{5, 5, 2}, vector{0, 0, 1}, 2, 5)
	cavity := declined(t, b, "", "cut", base.Handle, tool.Handle)
	near(t, cavity.GeometryAssertions.Volume, 2000-20*math.Pi)
	near(t, cavity.GeometryAssertions.SurfaceArea, 1000+28*math.Pi)
}

func TestBlindFloorNearExitReferenceIsAmbiguous(t *testing.T) {
	b := New()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 8})
	tool := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 1, 8-5e-8)
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	// The floor is within tolerance of the far face. Either the two are two
	// faces on one plane, and a reference to that plane is refused, or the
	// floor is the far face, the bore is through, and the reference is the
	// far face with its opening. A reference to the floor alone, or to a
	// far face without an opening, is wrong.
	if f, err := planarFace(t, b, cut.Handle, vector{0, 0, 8}, vector{0, 0, 1}); err == nil {
		near(t, f.Area, 64-math.Pi)
	}
	before := len(b.shapes)
	reply := b.Call(`{"op":"cut","a":"` + cut.Handle + `","b":"` + tool.Handle + `"}`)
	if !strings.Contains(reply, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
		t.Fatal(reply)
	}
}

func TestMultipleBlindPocketsRetainSharedCaps(t *testing.T) {
	for _, opposite := range []bool{false, true} {
		b := New()
		base := box(t, b, vector{-10, -5, 0}, vector{10, 5, 10})
		a := circle(t, b, vector{-5, 0, -2}, vector{0, 0, 1}, 1, 6)
		z, n, h := float64(-2), float64(1), float64(8)
		if opposite {
			z, n, h = 12, -1, 8
		}
		c := circle(t, b, vector{5, 0, z}, vector{0, 0, n}, 1, h)
		first := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": a.Handle})
		cut := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": c.Handle})
		near(t, cut.GeometryAssertions.Volume, 2000-10*math.Pi)
		near(t, cut.GeometryAssertions.SurfaceArea, 1000+20*math.Pi)
		solid := boundary(t, b, cut.Handle)
		p, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		near(t, p.Volume, cut.GeometryAssertions.Volume)
		near(t, p.Area, cut.GeometryAssertions.SurfaceArea)
		m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .2})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		for _, z := range []float64{0, 10} {
			f, err := planarFace(t, b, cut.Handle, vector{0, 0, z}, vector{0, 0, 1})
			if err != nil {
				t.Fatal(err)
			}
			want := 200.
			if opposite {
				want -= math.Pi
			} else if z == 0 {
				want -= 2 * math.Pi
			}
			near(t, f.Area, want)
		}
	}
}

func TestGuidedOppositeEndBlindToolAllAxes(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			lo, hi := vector{-40, -25, -25}, vector{40, 25, 25}
			lo[axis] = 0
			hi[axis] = 20
			base := box(t, b, lo, hi)
			o, n := vector{10, 8, 8}, vector{}
			o[axis] = 16
			n[axis] = sign
			if sign < 0 {
				o[axis] = 4
			}
			tool := circle(t, b, o, n, 2, 4)
			cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
			near(t, cut.GeometryAssertions.Volume, base.GeometryAssertions.Volume-16*math.Pi)
			near(t, cut.GeometryAssertions.SurfaceArea, base.GeometryAssertions.SurfaceArea+16*math.Pi)
			solid := boundary(t, b, cut.Handle)
			p, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, cut.GeometryAssertions.Volume)
			near(t, p.Area, cut.GeometryAssertions.SurfaceArea)
			f, err := planarFace(t, b, cut.Handle, o, n)
			if err != nil {
				t.Fatal(err)
			}
			near(t, f.Area, 4*math.Pi)
			near(t, f.Normal[axis], sign)
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .2})
			if err != nil {
				t.Fatal(err)
			}
			if err = m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			// The cut leaves the tool as it was published.
			if kept := shape(t, b, map[string]any{"op": "inspect", "handle": tool.Handle}); kept.Bounds != tool.Bounds || kept.GeometryAssertions != tool.GeometryAssertions {
				t.Fatal("the cut changed its tool", kept, tool)
			}
		}
	}
}

func TestExitOpenBlindRejectsUnpreservedEndpoints(t *testing.T) {
	b := New()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 20})
	for i, tc := range []struct{ z, h float64 }{{16, math.Nextafter(20, 0) - 16}, {.1, 1e16}} {
		tool := circle(t, b, vector{0, 0, tc.z}, vector{0, 0, 1}, 1, tc.h)
		if i == 0 {
			// The legacy path declines a tool that stops one unit in the
			// last place short of the far face. Within tolerance that is
			// the far face, and the general Boolean cuts the pocket, four
			// deep.
			pocket := declined(t, b, "not a measurable entry-open blind pocket or through bore", "cut", base.Handle, tool.Handle)
			near(t, pocket.GeometryAssertions.Volume, 1280-4*math.Pi)
			near(t, pocket.GeometryAssertions.SurfaceArea, 768+8*math.Pi)
			continue
		}
		before := len(b.shapes)
		reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + tool.Handle + `"}`)
		if !strings.Contains(reply, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
			t.Fatal(reply)
		}
	}
}

// PlainCAD's default display mesh is also used for selected-body STL export.
func TestScopedHoleDefaultMeshVolume(t *testing.T) {
	b := New()
	base := box(t, b, vector{30, 0, 20}, vector{50, 10, 30})
	tool := circle(t, b, vector{35, 5, 0}, vector{0, 0, 1}, 3, 35)
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	m, err := b.shapes[cut.Handle].tessellate(request{LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	t.Logf("default bore display triangles: %d", len(m.Triangles))
	p, err := m.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.Volume/cut.GeometryAssertions.Volume-1) > .005 {
		t.Fatalf("default display/STL volume ratio=%g", p.Volume/cut.GeometryAssertions.Volume)
	}
}

func TestGuidedDefaultMeshRemovedVolumeAndNormalTurn(t *testing.T) {
	for _, height := range []float64{4, 20} {
		b := New()
		base := box(t, b, vector{-40, -25, 0}, vector{40, 25, 20})
		stock := base
		for _, xy := range [][2]float64{{10, 8}, {-10, -8}} {
			tool := circle(t, b, vector{xy[0], xy[1], 20 - height}, vector{0, 0, 1}, 3, height)
			stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
		}
		m, err := b.shapes[stock.Handle].tessellate(request{LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true}})
		if err != nil {
			t.Fatal(err)
		}
		p, err := m.MassProperties()
		if err != nil {
			t.Fatal(err)
		}
		removed := base.GeometryAssertions.Volume - stock.GeometryAssertions.Volume
		if math.Abs(p.Volume-stock.GeometryAssertions.Volume) > removed*.02 {
			t.Fatalf("removed-volume mesh error=%g", math.Abs(p.Volume-stock.GeometryAssertions.Volume)/removed)
		}
		walls := 0
		for _, tri := range m.Triangles {
			a, c, d := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
			if a.Z == c.Z && a.Z == d.Z {
				continue
			}
			for _, xy := range [][2]float64{{10, 8}, {-10, -8}} {
				normals := []geom.Vec3{a.Sub(geom.Vec3{X: xy[0], Y: xy[1], Z: a.Z}), c.Sub(geom.Vec3{X: xy[0], Y: xy[1], Z: c.Z}), d.Sub(geom.Vec3{X: xy[0], Y: xy[1], Z: d.Z})}
				if math.Abs(normals[0].Length()-3) > 1e-8 || math.Abs(normals[1].Length()-3) > 1e-8 || math.Abs(normals[2].Length()-3) > 1e-8 {
					continue
				}
				walls++
				for i := 0; i < 3; i++ {
					for j := i + 1; j < 3; j++ {
						turn := math.Atan2(normals[i].Cross(normals[j]).Length(), normals[i].Dot(normals[j]))
						if turn > .2+1e-12 {
							t.Fatalf("cylinder normal turn %g exceeds requested .2", turn)
						}
					}
				}
			}
		}
		if walls == 0 {
			t.Fatal("no cylinder mesh walls")
		}
	}
}
func TestAngularMappingUnderflowDoesNotDisableChecks(t *testing.T) {
	_, err := tessellationOptions(request{AngularDeflection: optionalNumber{value: number(math.SmallestNonzeroFloat64), set: true}})
	if !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("tiny requested angle: %v", err)
	}
}

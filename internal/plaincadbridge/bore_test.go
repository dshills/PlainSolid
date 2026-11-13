package plaincadbridge

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestThroughCircleCutAllAxesAndSigns(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			base := box(t, b, vector{-4, -4, -4}, vector{4, 4, 4})
			origin := vector{}
			n := vector{}
			n[axis] = sign
			origin[axis] = -4 * sign
			tool := circle(t, b, origin, n, 1, 8)
			cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
			near(t, cut.GeometryAssertions.Volume, 512-8*math.Pi)
			near(t, cut.GeometryAssertions.SurfaceArea, 384+14*math.Pi)
			if cut.Bounds != base.Bounds || cut.GeometryAssertions.SolidCount != 1 {
				t.Fatal(cut)
			}
			body := b.shapes[cut.Handle].bore
			// Independently integrate the retained curved boundary, not its display.
			p, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, cut.GeometryAssertions.Volume)
			near(t, p.Area, cut.GeometryAssertions.SurfaceArea)
			found := false
			for _, face := range body.solid.Faces() {
				if _, ok := face.Surface.(geom.CylinderSurface); ok {
					found = true
				}
			}
			if !found {
				t.Fatal("cylinder wall was faceted")
			}
			var face PlanarFace
			if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": cut.Handle, "origin": origin, "normal": n}), &face); err != nil {
				t.Fatal(err)
			}
			near(t, face.Area, 64-math.Pi)
			common := shape(t, b, map[string]any{"op": "common", "a": base.Handle, "b": tool.Handle})
			near(t, common.GeometryAssertions.Volume, 8*math.Pi)
		}
	}
}
func TestFiniteCylinderCoverageAndPartialCommon(t *testing.T) {
	b := New()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 8})
	for _, tc := range []struct{ z, height float64 }{{9, 8}, {-9, 8}, {math.SmallestNonzeroFloat64, 8}, {0, math.Nextafter(8, 0)}} {
		tool := circle(t, b, vector{0, 0, tc.z}, vector{0, 0, 1}, 1, tc.height)
		before := len(b.shapes)
		reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + tool.Handle + `"}`)
		if !strings.Contains(reply, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
			t.Fatalf("accepted partial finite tool %+v: %s", tc, reply)
		}
	}
	tool := circle(t, b, vector{0, 0, -2}, vector{0, 0, 1}, 1, 5)
	common := shape(t, b, map[string]any{"op": "common", "a": base.Handle, "b": tool.Handle})
	near(t, common.GeometryAssertions.Volume, 3*math.Pi)
	reversed := shape(t, b, map[string]any{"op": "common", "a": tool.Handle, "b": base.Handle})
	near(t, reversed.GeometryAssertions.Volume, common.GeometryAssertions.Volume)
	disjoint := circle(t, b, vector{0, 0, 10}, vector{0, 0, 1}, 1, 2)
	empty := shape(t, b, map[string]any{"op": "common", "a": base.Handle, "b": disjoint.Handle})
	if empty.GeometryAssertions.Volume != 0 || empty.GeometryAssertions.SolidCount != 0 {
		t.Fatal(empty)
	}
	tangent := circle(t, b, vector{0, 0, 8}, vector{0, 0, 1}, 1, 2)
	for _, pair := range [][2]string{{base.Handle, tangent.Handle}, {tangent.Handle, base.Handle}} {
		empty := shape(t, b, map[string]any{"op": "common", "a": pair[0], "b": pair[1]})
		if empty.GeometryAssertions.Volume != 0 {
			t.Fatal(empty)
		}
	}
	// An axis that crosses side faces cannot be treated as a perpendicular bore.
	skew := circle(t, b, vector{0, 0, -2}, vector{1, 0, 1}, 1, 20)
	reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + skew.Handle + `"}`)
	if !strings.Contains(reply, `"ok":false`) {
		t.Fatal(reply)
	}
	// Tangential tools remain unsupported. Transverse one-side crossings are
	// covered by TestPlacedSideCrossingCircularCut with exact segment measures.
	for _, x := range []float64{3} {
		tool := circle(t, b, vector{x, 0, -1}, vector{0, 0, 1}, 1, 10)
		reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + tool.Handle + `"}`)
		if !strings.Contains(reply, `"ok":false`) {
			t.Fatal(reply)
		}
	}
}
func TestBoreClonePlacementAndRepeatedCuts(t *testing.T) {
	b := New()
	base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 8})
	tool := circle(t, b, vector{-2, 0, -1}, vector{0, 0, 1}, 1, 10)
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	clone := shape(t, b, map[string]any{"op": "clone", "handle": cut.Handle})
	invoke(t, b, map[string]any{"op": "dispose", "handle": cut.Handle})
	second := circle(t, b, vector{2, 0, 0}, vector{0, 0, 1}, 1, 8)
	twice := shape(t, b, map[string]any{"op": "cut", "a": clone.Handle, "b": second.Handle})
	near(t, twice.GeometryAssertions.Volume, 800-16*math.Pi)
	before := len(b.shapes)
	rejected := b.Call(`{"op":"cut","a":"` + twice.Handle + `","b":"` + second.Handle + `"}`)
	if !strings.Contains(rejected, `"ok":false`) || len(b.shapes) != before {
		t.Fatal("overlapping bore accepted", rejected)
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": base.Handle}).GeometryAssertions.Volume, 800)
	moved := shape(t, b, map[string]any{"op": "transform", "handle": twice.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	near(t, moved.GeometryAssertions.Volume, twice.GeometryAssertions.Volume)
	near(t, moved.Bounds.Min[0], 7)
	near(t, moved.Bounds.Max[0], 15)
	var m Mesh
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "mesh", "handle": moved.Handle, "linearDeflection": .04, "angularDeflection": .2}), &m); err != nil {
		t.Fatal(err)
	}
	var encoded struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "stl", "handle": moved.Handle, "linearDeflection": .04, "angularDeflection": .2}), &encoded); err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(encoded.Base64)
	if err != nil {
		t.Fatal(err)
	}
	count := int(binary.LittleEndian.Uint32(data[80:84]))
	if count*3 != len(m.Indices) {
		t.Fatal("bore STL/display topology differs")
	}
	for tri := 0; tri < count; tri++ {
		for j := 0; j < 9; j++ {
			v := math.Float32frombits(binary.LittleEndian.Uint32(data[84+tri*50+12+j*4:]))
			if v != float32(m.Positions[tri*9+j]) {
				t.Fatal("bore STL/display vertex differs")
			}
		}
	}
	if len(m.Indices) == 0 {
		t.Fatal("empty bored display")
	}
	mesh, err := b.shapes[moved.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	props, err := mesh.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(props.Volume-moved.GeometryAssertions.Volume) > moved.GeometryAssertions.Volume*.01 {
		t.Fatal("display mass disagrees")
	}
}

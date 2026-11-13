package plaincadbridge

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func circle(t *testing.T, b *Bridge, origin, normal vector, r, h float64) Shape {
	return shape(t, b, map[string]any{"op": "circle", "origin": origin, "normal": normal, "radius": r, "height": h})
}
func TestExactCircleAllAxesAndSigns(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			n := vector{}
			n[axis] = 1
			s := circle(t, b, vector{3, 4, 5}, n, 2, sign*7)
			near(t, s.GeometryAssertions.Volume, 28*math.Pi)
			near(t, s.GeometryAssertions.SurfaceArea, 36*math.Pi)
			for i, origin := range []float64{3, 4, 5} {
				lo, hi := origin-2, origin+2
				if i == axis {
					lo = origin + math.Min(0, sign*7)
					hi = origin + math.Max(0, sign*7)
				}
				near(t, s.Bounds.Min[i], lo)
				near(t, s.Bounds.Max[i], hi)
			}
			if s.GeometryAssertions.SolidCount != 1 {
				t.Fatal(s)
			}
			var face PlanarFace
			if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": s.Handle, "origin": vector{3, 4, 5}, "normal": n}), &face); err != nil {
				t.Fatal(err)
			}
			near(t, face.Area, 4*math.Pi)
			clone := shape(t, b, map[string]any{"op": "clone", "handle": s.Handle})
			invoke(t, b, map[string]any{"op": "dispose", "handle": s.Handle})
			near(t, clone.GeometryAssertions.Volume, 28*math.Pi)
		}
	}
}
func TestCirclePlacementAndRejectedOperations(t *testing.T) {
	b := New()
	s := circle(t, b, vector{}, vector{0, 0, 1}, 2, 5)
	moved := shape(t, b, map[string]any{"op": "transform", "handle": s.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	near(t, moved.Bounds.Min[0], 7)
	near(t, moved.Bounds.Max[0], 12)
	near(t, moved.Bounds.Min[1], 6)
	near(t, moved.GeometryAssertions.Volume, 20*math.Pi)
	for _, input := range []string{
		`{"op":"circle","origin":[0,0,0],"normal":[0,0,0],"radius":1,"height":2}`,
		`{"op":"circle","origin":[0,0,0],"normal":[0,0,1],"radius":null,"height":2}`,
		`{"op":"circle","origin":[0,0,0],"normal":[0,0,1],"radius":1,"height":0}`,
		`{"op":"transform","handle":"HANDLE","matrix":[[2,0,0,0],[0,1,0,0],[0,0,1,0]]}`,
		`{"op":"shell","handle":"HANDLE","openFace":1,"wall":0.5}`,
	} {
		before := len(b.shapes)
		reply := b.Call(strings.ReplaceAll(input, "HANDLE", s.Handle))
		if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
			t.Fatal(reply)
		}
	}
}
func TestCircleMeshSTLUsesSameTolerance(t *testing.T) {
	b := New()
	s := circle(t, b, vector{}, vector{0, 0, 1}, 3, 4)
	q := map[string]any{"op": "mesh", "handle": s.Handle, "linearDeflection": .025, "angularDeflection": .15}
	var m Mesh
	if err := json.Unmarshal(invoke(t, b, q), &m); err != nil {
		t.Fatal(err)
	}
	q["op"] = "stl"
	var out struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(invoke(t, b, q), &out); err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(out.Base64)
	if err != nil {
		t.Fatal(err)
	}
	count := int(binary.LittleEndian.Uint32(data[80:84]))
	if count*3 != len(m.Indices) {
		t.Fatal("display/STL topology differs")
	}
	for tri := 0; tri < count; tri++ {
		for j := 0; j < 9; j++ {
			v := math.Float32frombits(binary.LittleEndian.Uint32(data[84+tri*50+12+j*4:]))
			if v != float32(m.Positions[tri*9+j]) {
				t.Fatalf("STL changed vertex %d", tri)
			}
		}
	}
}

func TestCylinderRetainedGeometryAndCapOrientation(t *testing.T) {
	for _, height := range []float64{-4, 4} {
		b := New()
		s := circle(t, b, vector{}, vector{0, 0, 1}, 3, height)
		c := b.shapes[s.Handle].cylinder
		p, err := c.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
		if err != nil {
			t.Fatal(err)
		}
		near(t, p.Volume, s.GeometryAssertions.Volume)
		near(t, p.Area, s.GeometryAssertions.SurfaceArea)
		wall, ok := c.solid.Faces()[0].Surface.(geom.CylinderSurface)
		if !ok {
			t.Fatal("stored wall is not an analytic cylinder")
		}
		for _, uv := range [][2]float64{{.37, .4}, {2.1, 3.2}} {
			point, err := wall.Eval(uv[0], uv[1])
			if err != nil {
				t.Fatal(err)
			}
			local := c.frame.ToLocal(point)
			near(t, local.X*local.X+local.Y*local.Y, 9)
		}
		for _, end := range []float64{0, height} {
			var face PlanarFace
			err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": s.Handle, "origin": vector{0, 0, end}, "normal": vector{0, 0, 1}}), &face)
			if err != nil {
				t.Fatal(err)
			}
			want := math.Copysign(1, height)
			if end == 0 {
				want = -want
			}
			near(t, face.Normal[2], want)
		}
		coarse, err := c.solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .4})
		if err != nil {
			t.Fatal(err)
		}
		fine, err := c.solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01, Angle: .1})
		if err != nil {
			t.Fatal(err)
		}
		if err := fine.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		cp, err := coarse.MassProperties()
		if err != nil {
			t.Fatal(err)
		}
		fp, err := fine.MassProperties()
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(fp.Volume-p.Volume) >= math.Abs(cp.Volume-p.Volume) || math.Abs(fp.Volume-p.Volume) > p.Volume*.01 {
			t.Fatal("display volume does not converge to original surface volume")
		}
	}
}

func TestCircleRejectsExplicitInvalidMeshOptions(t *testing.T) {
	b := New()
	s := circle(t, b, vector{}, vector{0, 0, 1}, 2, 3)
	for _, op := range []string{"mesh", "stl"} {
		for _, field := range []string{"linearDeflection", "angularDeflection"} {
			for _, value := range []string{"null", "0", "-1", "true", "\"0.1\"", "1e999"} {
				input := `{"op":"` + op + `","handle":"` + s.Handle + `","` + field + `":` + value + `}`
				if reply := b.Call(input); !strings.Contains(reply, `"ok":false`) || len(b.shapes) != 1 {
					t.Fatal(input, reply)
				}
			}
		}
	}
}

func TestCircleTessellationExhaustionIsLimit(t *testing.T) {
	b := New()
	s := circle(t, b, vector{}, vector{0, 0, 1}, 2, 3)
	reply := b.Call(`{"op":"mesh","handle":"` + s.Handle + `","linearDeflection":1e-20,"angularDeflection":0.2}`)
	if !strings.Contains(reply, `"code":"LIMIT"`) || len(b.shapes) != 1 {
		t.Fatal(reply)
	}
}

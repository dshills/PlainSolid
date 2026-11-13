package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/exchange"
)

func invoke(t *testing.T, b *Bridge, q any) json.RawMessage {
	t.Helper()
	data, e := json.Marshal(q)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *failure        `json:"error"`
	}
	if e = json.Unmarshal([]byte(b.Call(string(data))), &result); e != nil {
		t.Fatal(e)
	}
	if !result.OK {
		t.Fatalf("request %s: %+v", data, result.Error)
	}
	return result.Result
}
func shape(t *testing.T, b *Bridge, q any) Shape {
	t.Helper()
	var s Shape
	if e := json.Unmarshal(invoke(t, b, q), &s); e != nil {
		t.Fatal(e)
	}
	return s
}
func box(t *testing.T, b *Bridge, lo, hi vector) Shape {
	t.Helper()
	return shape(t, b, map[string]any{"op": "box", "min": lo, "max": hi})
}
func near(t *testing.T, a, b float64) {
	t.Helper()
	if math.Abs(a-b) > 1e-8*math.Max(1, math.Abs(b)) {
		t.Fatalf("got %.15g, want %.15g", a, b)
	}
}

func TestBoxMeshSTLAndOwnership(t *testing.T) {
	b := New()
	s := box(t, b, vector{}, vector{10, 20, 30})
	near(t, s.GeometryAssertions.Volume, 6000)
	near(t, s.GeometryAssertions.SurfaceArea, 2200)
	if s.GeometryAssertions.SolidCount != 1 || !s.GeometryAssertions.Valid {
		t.Fatal(s)
	}
	copy := shape(t, b, map[string]any{"op": "clone", "handle": s.Handle})
	if copy.Handle == s.Handle {
		t.Fatal("clone reused ownership")
	}
	var m Mesh
	if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "mesh", "handle": copy.Handle}), &m); e != nil {
		t.Fatal(e)
	}
	if len(m.Indices) != 36 || len(m.Normals) != 108 || len(m.Positions) != 108 {
		t.Fatal("wrong box mesh dimensions")
	}
	for i := 0; i < len(m.Normals); i += 3 {
		near(t, m.Normals[i]*m.Normals[i]+m.Normals[i+1]*m.Normals[i+1]+m.Normals[i+2]*m.Normals[i+2], 1)
	}
	var stl struct {
		Base64 string `json:"base64"`
	}
	if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "stl", "handle": copy.Handle}), &stl); e != nil {
		t.Fatal(e)
	}
	data, e := base64.StdEncoding.DecodeString(stl.Base64)
	if e != nil {
		t.Fatal(e)
	}
	mesh, e := exchange.ReadSTL(bytes.NewReader(data), exchange.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := mesh.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Volume, 6000)
	invoke(t, b, map[string]any{"op": "dispose", "handle": s.Handle})
	if got := b.Call(`{"op":"inspect","handle":"` + s.Handle + `"}`); !strings.Contains(got, `"code":"HANDLE"`) {
		t.Fatal(got)
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": copy.Handle}).GeometryAssertions.Volume, 6000)
}

func TestSignedHoledPrismsAndPlacement(t *testing.T) {
	for _, depth := range []float64{5, -5} {
		b := New()
		s := shape(t, b, map[string]any{"op": "prism", "outer": []vector{{0, 0, 0}, {10, 0, 0}, {10, 8, 0}, {0, 8, 0}}, "holes": [][]vector{{{2, 2, 0}, {4, 2, 0}, {4, 6, 0}, {2, 6, 0}}}, "offset": vector{0, 0, depth}})
		near(t, s.GeometryAssertions.Volume, 360)
		// A signed-axis rotation moves the depth direction from Z to X.
		moved := shape(t, b, map[string]any{"op": "transform", "handle": s.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
		near(t, moved.GeometryAssertions.Volume, 360)
		near(t, moved.Bounds.Min[0], 7+math.Min(0, depth))
		near(t, moved.Bounds.Max[0], 7+math.Max(0, depth))
		near(t, moved.Bounds.Max[1], 18)
		near(t, moved.Bounds.Max[2], 17)
	}
}

func TestBooleanMaterialComponents(t *testing.T) {
	b := New()
	a := box(t, b, vector{}, vector{10, 10, 10})
	tool := box(t, b, vector{2, 2, 2}, vector{8, 8, 8})
	cut := shape(t, b, map[string]any{"op": "cut", "a": a.Handle, "b": tool.Handle})
	near(t, cut.GeometryAssertions.Volume, 784)
	if cut.GeometryAssertions.SolidCount != 1 {
		t.Fatalf("cavity counted as a solid: %+v", cut)
	}
	common := shape(t, b, map[string]any{"op": "common", "a": a.Handle, "b": tool.Handle})
	near(t, common.GeometryAssertions.Volume, 216)
	far := box(t, b, vector{20, 0, 0}, vector{21, 1, 1})
	union := shape(t, b, map[string]any{"op": "fuse", "a": a.Handle, "b": far.Handle})
	near(t, union.GeometryAssertions.Volume, 1001)
	if union.GeometryAssertions.SolidCount != 2 {
		t.Fatal(union)
	}
	empty := shape(t, b, map[string]any{"op": "common", "a": a.Handle, "b": far.Handle})
	if empty.GeometryAssertions.Volume != 0 || empty.GeometryAssertions.SolidCount != 0 {
		t.Fatal(empty)
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": a.Handle}).GeometryAssertions.Volume, 1000)
}

func TestRectangularShellVolume(t *testing.T) {
	for _, opening := range []int{0, 1} {
		b := New()
		s := box(t, b, vector{}, vector{20, 16, 10})
		h := shape(t, b, map[string]any{"op": "shell", "handle": s.Handle, "openFace": opening, "wall": 2})
		near(t, h.GeometryAssertions.Volume, 20*16*10-16*12*8)
		if h.GeometryAssertions.SolidCount != 1 {
			t.Fatal(h)
		}
		before := len(b.shapes)
		got := b.Call(`{"op":"shell","handle":"` + s.Handle + `","openFace":0,"wall":20}`)
		if !strings.Contains(got, `"ok":false`) || len(b.shapes) != before {
			t.Fatal("failed shell published a handle", got)
		}
	}
}

func TestStrictRequestsAndLimits(t *testing.T) {
	b := New()
	for _, q := range []string{`{"op":"box","min":[0,0],"max":[1,1,1]}`, `{"op":"box","min":[0,0,0,0],"max":[1,1,1]}`, `{"op":"box","min":[0,0,0],"max":[1,1,1],"curve":"circle"}`, `{"op":"box"}`, `{"op":"capabilities"} {}`, `null`} {
		if got := b.Call(q); !strings.Contains(got, `"ok":false`) {
			t.Fatal(q, got)
		}
	}
	if got := b.Call(`{"op":"thread"}`); !strings.Contains(got, `"code":"UNSUPPORTED"`) {
		t.Fatal(got)
	}
	if got := b.Call(strings.Repeat(" ", MaxRequestBytes+1)); !strings.Contains(got, `"code":"LIMIT"`) {
		t.Fatal(got)
	}
	s := box(t, b, vector{}, vector{1, 1, 1})
	for len(b.shapes) < MaxHandles {
		shape(t, b, map[string]any{"op": "clone", "handle": s.Handle})
	}
	if got := b.Call(`{"op":"clone","handle":"` + s.Handle + `"}`); !strings.Contains(got, `"code":"LIMIT"`) {
		t.Fatal(got)
	}
	invoke(t, b, map[string]any{"op": "dispose", "handle": s.Handle})
	box(t, b, vector{}, vector{1, 1, 1})
}

func TestMalformedNumericFieldsCannotPublishGeometry(t *testing.T) {
	b := New()
	s := box(t, b, vector{}, vector{10, 10, 10})
	requests := []string{
		`{"op":"box","min":[null,0,0],"max":[1,1,1]}`,
		`{"op":"box","min":[-1,-1,-1],"max":[0,null,0]}`,
		`{"op":"prism","outer":[[null,0,0],[4,0,0],[4,4,0],[0,4,0]],"offset":[0,0,2]}`,
		`{"op":"prism","outer":[[0,0,0],[4,0,0],[4,4,0],[0,4,0]],"holes":[[[1,1,null],[2,1,0],[2,2,0],[1,2,0]]],"offset":[0,0,2]}`,
		`{"op":"prism","outer":[[0,0,0],[4,0,0],[4,4,0],[0,4,0]],"offset":[null,0,2]}`,
		`{"op":"transform","handle":"HANDLE","matrix":[[1,0,0,null],[0,1,0,0],[0,0,1,0]]}`,
		`{"op":"transform","handle":"HANDLE","matrix":[[1,0,0,0],null,[0,0,1,0]]}`,
		`{"op":"transform","handle":"HANDLE","matrix":null}`,
		`{"op":"shell","handle":"HANDLE","openFace":0,"wall":null}`,
		`{"op":"shell","handle":"HANDLE","openFace":null,"wall":1}`,
	}
	for _, token := range []string{`"0"`, `false`, `{}`, `[]`, `1e999`} {
		requests = append(requests,
			`{"op":"box","min":[`+token+`,0,0],"max":[1,1,1]}`,
			`{"op":"transform","handle":"HANDLE","matrix":[[1,0,0,`+token+`],[0,1,0,0],[0,0,1,0]]}`,
			`{"op":"shell","handle":"HANDLE","openFace":0,"wall":`+token+`}`,
			`{"op":"shell","handle":"HANDLE","openFace":`+token+`,"wall":1}`,
		)
	}
	for _, request := range requests {
		input := strings.ReplaceAll(request, "HANDLE", s.Handle)
		var reply response
		if err := json.Unmarshal([]byte(b.Call(input)), &reply); err != nil {
			t.Fatal(err)
		}
		if reply.OK || reply.Error == nil || reply.Error.Code != "INVALID" {
			t.Fatalf("malformed numeric input must be invalid: %s: %+v", input, reply)
		}
		if len(b.shapes) != 1 || b.next != 1 {
			t.Fatalf("malformed input allocated or changed shapes: %s", input)
		}
	}
	// Rejecting malformed values must not change the original solid or prevent
	// an ordinary numeric transform from succeeding afterward.
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": s.Handle}).GeometryAssertions.Volume, 1000)
	moved := shape(t, b, map[string]any{"op": "transform", "handle": s.Handle, "matrix": [][]float64{{1, 0, 0, 5}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
	near(t, moved.Bounds.Min[0], 5)
	near(t, moved.GeometryAssertions.Volume, 1000)
}

func TestPlanarFaceMatchesCurrentGeometry(t *testing.T) {
	b := New()
	s := box(t, b, vector{}, vector{10, 8, 6})
	for _, normal := range []vector{{0, 0, 1}, {0, 0, -1}} {
		var face PlanarFace
		if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": s.Handle, "origin": vector{4, 3, 6}, "normal": normal}), &face); err != nil {
			t.Fatal(err)
		}
		if face.Face != 1 || face.Normal != (vector{0, 0, 1}) {
			t.Fatal(face)
		}
		near(t, face.Area, 80)
	}
	// The native contract compares supporting planes, not the location of the
	// arbitrary frame origin within a finite face boundary.
	invoke(t, b, map[string]any{"op": "planarFace", "handle": s.Handle, "origin": vector{100, -100, 6 + 5e-8}, "normal": vector{0, 0, 1}})
	for _, q := range []map[string]any{
		{"op": "planarFace", "handle": s.Handle, "origin": vector{0, 0, 6 + 2e-7}, "normal": vector{0, 0, 1}},
		{"op": "planarFace", "handle": s.Handle, "origin": vector{0, 0, 6}, "normal": vector{1, 0, 1}},
		{"op": "planarFace", "handle": s.Handle, "origin": vector{0, 0, 6}, "normal": vector{}},
	} {
		data, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		if reply := b.Call(string(data)); !strings.Contains(reply, `"ok":false`) {
			t.Fatal(reply)
		}
	}
	// A rigidly moved face must be measured from the current B-rep rather than
	// from the original primitive's authored cap coordinates.
	moved := shape(t, b, map[string]any{"op": "transform", "handle": s.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	var face PlanarFace
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": moved.Handle, "origin": vector{13, 8, 9}, "normal": vector{1, 0, 0}}), &face); err != nil {
		t.Fatal(err)
	}
	near(t, face.Area, 80)
	if face.Normal != (vector{1, 0, 0}) {
		t.Fatal(face)
	}
	// Two separate coplanar caps cannot be claimed as one retained face, even
	// when the authored origin lies over only one of them.
	far := box(t, b, vector{20, 0, 0}, vector{30, 8, 6})
	combined := shape(t, b, map[string]any{"op": "fuse", "a": s.Handle, "b": far.Handle})
	q := `{"op":"planarFace","handle":"` + combined.Handle + `","origin":[4,3,6],"normal":[0,0,1]}`
	if reply := b.Call(q); !strings.Contains(reply, `"ok":false`) || !strings.Contains(reply, "ambiguous") {
		t.Fatal(reply)
	}
}

func TestPlanarFaceRejectsNullWithoutMutation(t *testing.T) {
	b := New()
	s := box(t, b, vector{}, vector{10, 8, 6})
	for _, query := range []string{
		`{"op":"planarFace","handle":"HANDLE","origin":[null,0,6],"normal":[0,0,1]}`,
		`{"op":"planarFace","handle":"HANDLE","origin":[0,0,6],"normal":[null,0,1]}`,
		`{"op":"planarFace","handle":"HANDLE","origin":null,"normal":[0,0,1]}`,
		`{"op":"planarFace","handle":"HANDLE","origin":[0,0,6],"normal":null}`,
	} {
		if reply := b.Call(strings.ReplaceAll(query, "HANDLE", s.Handle)); !strings.Contains(reply, `"code":"INVALID"`) {
			t.Fatal(reply)
		}
		if len(b.shapes) != 1 || b.next != 1 {
			t.Fatal("face query changed registry")
		}
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": s.Handle}).GeometryAssertions.Volume, 480)
}

package exchange

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func sphereCylinderCommonFixture() SphereCylinderCommonRecipe {
	return SphereCylinderCommonRecipe{Name: "offset sphere", XAxis: geom.Vec3{X: 1}, ZAxis: geom.Vec3{Z: 1}, SphereRadius: 5, CylinderRadius: 1, Offset: 2, Lower: -2, Upper: 6, Tolerance: 1e-9, LengthUnit: .001}
}
func TestSphereCylinderCommonRecipeNative(t *testing.T) {
	r := sphereCylinderCommonFixture()
	r.Origin = geom.Vec3{X: 2, Y: -1, Z: 4}
	r.XAxis = geom.Vec3{Y: 1}
	r.ZAxis = geom.Vec3{X: 1}
	var b bytes.Buffer
	if e := WriteSphereCylinderCommonRecipeJSON(context.Background(), &b, r); e != nil {
		t.Fatal(e)
	}
	got, s, e := ReadSphereCylinderCommonRecipeJSON(context.Background(), bytes.NewReader(b.Bytes()), 0)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if len(s.LowerEndFaces) != 4 || len(s.CylindricalFaces) != 4 || len(s.UpperEndFaces) != 4 {
		t.Fatal("lost source face roles")
	}
	var native bytes.Buffer
	if e = parametric.WriteJSON(&native, s.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := parametric.ReadJSON(&native, parametric.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil {
		t.Fatal(e)
	}
	c, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || a != c {
		t.Fatal("changed native material", e)
	}
	var out bytes.Buffer
	if e = WriteSphereCylinderCommonRecipeJSON(context.Background(), &out, got); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
		t.Fatal("recipe changed", e)
	}
}
func TestSphereCylinderCommonRecipeRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSphereCylinderCommonRecipeJSON(context.Background(), &b, sphereCylinderCommonFixture()); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	changes := [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(raw, []byte(`"Offset":2`), []byte(`"Offset":1.25`), 1), bytes.Replace(raw, []byte(`"Reversed":false`), []byte(`"Reversed":true`), 1)}
	for i, data := range changes {
		if bytes.Equal(data, raw) {
			t.Fatalf("ineffective mutation %d", i)
		}
		if _, _, e := ReadSphereCylinderCommonRecipeJSON(context.Background(), bytes.NewReader(data), 0); e == nil {
			t.Fatal("accepted mutation", i)
		}
	}
	if _, _, e := ReadSphereCylinderCommonRecipeJSON(context.Background(), bytes.NewReader(raw), 32); e == nil {
		t.Fatal("ignored limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := ReadSphereCylinderCommonRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("ignored cancellation")
	}
	r := sphereCylinderCommonFixture()
	r.Offset = -1
	b.Reset()
	if e := WriteSphereCylinderCommonRecipeJSON(context.Background(), &b, r); e == nil || b.Len() != 0 {
		t.Fatal("invalid recipe published")
	}
}
func TestSphereCylinderCommonRecipeOCCT(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("optional OCCT oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "oracle.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	type record struct {
		Name         string
		Volume, Area float64
		Centroid     [3]float64
	}
	var wanted record
	for scan.Scan() {
		var x record
		if e = json.Unmarshal(scan.Bytes(), &x); e != nil {
			t.Fatal(e)
		}
		if x.Name == "offset-sphere-cylinder-common" {
			wanted = x
		}
	}
	if scan.Err() != nil || wanted.Name == "" {
		t.Fatal("missing OCCT record", scan.Err())
	}
	var b bytes.Buffer
	if e = WriteSphereCylinderCommonRecipeJSON(context.Background(), &b, sphereCylinderCommonFixture()); e != nil {
		t.Fatal(e)
	}
	_, s, e := ReadSphereCylinderCommonRecipeJSON(context.Background(), &b, 0)
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.Volume-wanted.Volume) > 2e-5 || math.Abs(m.Area-wanted.Area) > 2e-5 || m.Centroid.Sub(geom.Vec3{X: wanted.Centroid[0], Y: wanted.Centroid[1], Z: wanted.Centroid[2]}).Length() > 1e-6 {
		t.Fatal(m, wanted)
	}
}
func FuzzSphereCylinderCommonRecipe(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSphereCylinderCommonRecipeJSON(context.Background(), &b, sphereCylinderCommonFixture())
	f.Add(b.Bytes())
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, p []byte) {
		_, _, _ = ReadSphereCylinderCommonRecipeJSON(context.Background(), bytes.NewReader(p), 128<<10)
	})
}

func TestSphereCylinderCommonEndRoles(t *testing.T) {
	ctx := context.Background()
	for _, ends := range [][2]float64{{-6, 6}, {-2, 2}, {-6, 2}, {-2, 6}} {
		r := sphereCylinderCommonFixture()
		r.Lower, r.Upper = ends[0], ends[1]
		var b bytes.Buffer
		if e := WriteSphereCylinderCommonRecipeJSON(ctx, &b, r); e != nil {
			t.Fatal(ends, e)
		}
		got, s, e := ReadSphereCylinderCommonRecipeJSON(ctx, &b, 0)
		if e != nil || got != r || len(s.LowerEndFaces) != 4 || len(s.UpperEndFaces) != 4 {
			t.Fatal(ends, e)
		}
		// Planar ends are actual constant-height support patches. Spherical ends
		// reach a different height at the pole and the cylindrical rim.
		for side, ids := range [][]int{s.LowerEndFaces, s.UpperEndFaces} {
			f := s.Solid.Faces()[ids[0]]
			a, e := f.Surface.Eval(.3, .2)
			if e != nil {
				t.Fatal(e)
			}
			b, e := f.Surface.Eval(.3, .8)
			if e != nil {
				t.Fatal(e)
			}
			spherical := ends[side] == -6 || ends[side] == 6
			if spherical && a.Z == b.Z || !spherical && (a.Z != ends[side] || b.Z != ends[side]) {
				t.Fatal("incorrect end role", ends, side, a, b)
			}
		}
	}
	r := sphereCylinderCommonFixture()
	r.Lower = -4.5
	var b bytes.Buffer
	if e := WriteSphereCylinderCommonRecipeJSON(ctx, &b, r); e == nil || b.Len() != 0 {
		t.Fatal("partial cap escaped strict common API")
	}
	r = sphereCylinderCommonFixture()
	if e := WriteSphereCylinderCommonRecipeJSON(ctx, shortWriter{}, r); e == nil {
		t.Fatal("short write ignored")
	}
	if e := WriteSphereCylinderCommonRecipeJSON(ctx, &b, r); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	if _, _, e := ReadSphereThroughBoreRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("common accepted as through cut")
	}
	if _, _, e := ReadSphereBoreRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("common accepted as blind cut")
	}
}

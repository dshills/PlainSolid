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

func sphereBoreFixture() SphereBoreRecipe {
	return SphereBoreRecipe{Name: "offset sphere", XAxis: geom.Vec3{X: 1}, ZAxis: geom.Vec3{Z: 1}, SphereRadius: 5, BoreRadius: 2, Offset: 1, Floor: -.75, Tolerance: 1e-9, LengthUnit: .001}
}
func TestSphereBoreRecipeNative(t *testing.T) {
	r := sphereBoreFixture()
	r.Origin = geom.Vec3{X: 2, Y: -1, Z: 4}
	r.XAxis = geom.Vec3{Y: 1}
	r.ZAxis = geom.Vec3{X: 1}
	var b bytes.Buffer
	if e := WriteSphereBoreRecipeJSON(context.Background(), &b, r); e != nil {
		t.Fatal(e)
	}
	got, s, e := ReadSphereBoreRecipeJSON(context.Background(), bytes.NewReader(b.Bytes()), 0)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if len(s.SphericalFaces) != 4 || len(s.CylindricalFaces) != 4 || len(s.FloorFaces) != 4 {
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
	if e = WriteSphereBoreRecipeJSON(context.Background(), &out, got); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
		t.Fatal("recipe changed", e)
	}
}
func TestSphereBoreRecipeRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSphereBoreRecipeJSON(context.Background(), &b, sphereBoreFixture()); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	changes := [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(raw, []byte(`"Offset":1`), []byte(`"Offset":1.25`), 1), bytes.Replace(raw, []byte(`"Chart":0`), []byte(`"Chart":1`), 1)}
	for i, data := range changes {
		if bytes.Equal(data, raw) {
			t.Fatalf("ineffective mutation %d", i)
		}
		if _, _, e := ReadSphereBoreRecipeJSON(context.Background(), bytes.NewReader(data), 0); e == nil {
			t.Fatal("accepted mutation", i)
		}
	}
	if _, _, e := ReadSphereBoreRecipeJSON(context.Background(), bytes.NewReader(raw), 32); e == nil {
		t.Fatal("ignored limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := ReadSphereBoreRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("ignored cancellation")
	}
	r := sphereBoreFixture()
	r.Offset = -1
	b.Reset()
	if e := WriteSphereBoreRecipeJSON(context.Background(), &b, r); e == nil || b.Len() != 0 {
		t.Fatal("invalid recipe published")
	}
}
func TestSphereBoreRecipeOCCT(t *testing.T) {
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
		if x.Name == "off-axis-sphere-blind-bore" {
			wanted = x
		}
	}
	if scan.Err() != nil || wanted.Name == "" {
		t.Fatal("missing OCCT record", scan.Err())
	}
	var b bytes.Buffer
	if e = WriteSphereBoreRecipeJSON(context.Background(), &b, sphereBoreFixture()); e != nil {
		t.Fatal(e)
	}
	_, s, e := ReadSphereBoreRecipeJSON(context.Background(), &b, 0)
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.Volume-wanted.Volume) > 2e-5 || math.Abs(m.Area-wanted.Area) > 2e-5 {
		t.Fatal(m, wanted)
	}
}
func FuzzSphereBoreRecipe(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSphereBoreRecipeJSON(context.Background(), &b, sphereBoreFixture())
	f.Add(b.Bytes())
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, p []byte) {
		_, _, _ = ReadSphereBoreRecipeJSON(context.Background(), bytes.NewReader(p), 128<<10)
	})
}

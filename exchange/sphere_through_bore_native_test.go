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

func sphereThroughBoreFixture() SphereThroughBoreRecipe {
	return SphereThroughBoreRecipe{Name: "offset sphere", XAxis: geom.Vec3{X: 1}, ZAxis: geom.Vec3{Z: 1}, SphereRadius: 5, BoreRadius: 1, Offset: 2, Tolerance: 1e-9, LengthUnit: .001}
}
func TestSphereThroughBoreRecipeNative(t *testing.T) {
	r := sphereThroughBoreFixture()
	r.Origin = geom.Vec3{X: 2, Y: -1, Z: 4}
	r.XAxis = geom.Vec3{Y: 1}
	r.ZAxis = geom.Vec3{X: 1}
	var b bytes.Buffer
	if e := WriteSphereThroughBoreRecipeJSON(context.Background(), &b, r); e != nil {
		t.Fatal(e)
	}
	got, s, e := ReadSphereThroughBoreRecipeJSON(context.Background(), bytes.NewReader(b.Bytes()), 0)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if len(s.LowerSphericalFaces) != 4 || len(s.CylindricalFaces) != 4 || len(s.UpperSphericalFaces) != 4 {
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
	if e = WriteSphereThroughBoreRecipeJSON(context.Background(), &out, got); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
		t.Fatal("recipe changed", e)
	}
}
func TestSphereThroughBoreRecipeRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSphereThroughBoreRecipeJSON(context.Background(), &b, sphereThroughBoreFixture()); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	changes := [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(raw, []byte(`"Offset":2`), []byte(`"Offset":1.25`), 1), bytes.Replace(raw, []byte(`"Reversed":false`), []byte(`"Reversed":true`), 1)}
	for i, data := range changes {
		if bytes.Equal(data, raw) {
			t.Fatalf("ineffective mutation %d", i)
		}
		if _, _, e := ReadSphereThroughBoreRecipeJSON(context.Background(), bytes.NewReader(data), 0); e == nil {
			t.Fatal("accepted mutation", i)
		}
	}
	if _, _, e := ReadSphereThroughBoreRecipeJSON(context.Background(), bytes.NewReader(raw), 32); e == nil {
		t.Fatal("ignored limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := ReadSphereThroughBoreRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("ignored cancellation")
	}
	r := sphereThroughBoreFixture()
	r.Offset = -1
	b.Reset()
	if e := WriteSphereThroughBoreRecipeJSON(context.Background(), &b, r); e == nil || b.Len() != 0 {
		t.Fatal("invalid recipe published")
	}
}
func TestSphereThroughBoreRecipeOCCT(t *testing.T) {
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
		if x.Name == "offset-sphere-through-bore" {
			wanted = x
		}
	}
	if scan.Err() != nil || wanted.Name == "" {
		t.Fatal("missing OCCT record", scan.Err())
	}
	var b bytes.Buffer
	if e = WriteSphereThroughBoreRecipeJSON(context.Background(), &b, sphereThroughBoreFixture()); e != nil {
		t.Fatal(e)
	}
	_, s, e := ReadSphereThroughBoreRecipeJSON(context.Background(), &b, 0)
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
func FuzzSphereThroughBoreRecipe(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSphereThroughBoreRecipeJSON(context.Background(), &b, sphereThroughBoreFixture())
	f.Add(b.Bytes())
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, p []byte) {
		_, _, _ = ReadSphereThroughBoreRecipeJSON(context.Background(), bytes.NewReader(p), 128<<10)
	})
}

func TestSphereThroughBoreFormatSeparation(t *testing.T) {
	ctx := context.Background()
	var blind, through bytes.Buffer
	if e := WriteSphereBoreRecipeJSON(ctx, &blind, sphereBoreFixture()); e != nil {
		t.Fatal(e)
	}
	if e := WriteSphereThroughBoreRecipeJSON(ctx, &through, sphereThroughBoreFixture()); e != nil {
		t.Fatal(e)
	}
	if _, _, e := ReadSphereThroughBoreRecipeJSON(ctx, &blind, 0); e == nil {
		t.Fatal("blind recipe accepted as through")
	}
	if _, _, e := ReadSphereBoreRecipeJSON(ctx, &through, 0); e == nil {
		t.Fatal("through recipe accepted as blind")
	}
	if e := WriteSphereThroughBoreRecipeJSON(ctx, shortWriter{}, sphereThroughBoreFixture()); e == nil {
		t.Fatal("short write ignored")
	}
}

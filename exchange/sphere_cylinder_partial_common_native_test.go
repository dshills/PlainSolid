package exchange

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func sphereCylinderPartialCommonFixture() SphereCylinderPartialCommonRecipe {
	return SphereCylinderPartialCommonRecipe{Name: "offset sphere", XAxis: geom.Vec3{X: 1}, ZAxis: geom.Vec3{Z: 1}, SphereRadius: 5, CylinderRadius: 1, Offset: 2, Lower: -2, Upper: 4.25, Tolerance: 1e-9, LengthUnit: .001}
}
func TestSphereCylinderPartialCommonRecipeNative(t *testing.T) {
	r := sphereCylinderPartialCommonFixture()
	r.Origin = geom.Vec3{X: 2, Y: -1, Z: 4}
	r.XAxis = geom.Vec3{Y: 1}
	r.ZAxis = geom.Vec3{X: 1}
	var b bytes.Buffer
	if e := WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, r); e != nil {
		t.Fatal(e)
	}
	got, s, e := ReadSphereCylinderPartialCommonRecipeJSON(context.Background(), bytes.NewReader(b.Bytes()), 0)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if len(s.LowerCap) != 4 || len(s.CylindricalFaces) != 4 || len(s.UpperCap) != 4 {
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
	if e = WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &out, got); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
		t.Fatal("recipe changed", e)
	}
}
func TestSphereCylinderPartialCommonRecipeRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, sphereCylinderPartialCommonFixture()); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	changes := [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1), bytes.Replace(raw, []byte(`"Offset":2`), []byte(`"Offset":1.25`), 1), bytes.Replace(raw, []byte(`"Reversed":false`), []byte(`"Reversed":true`), 1)}
	for i, data := range changes {
		if bytes.Equal(data, raw) {
			t.Fatalf("ineffective mutation %d", i)
		}
		if _, _, e := ReadSphereCylinderPartialCommonRecipeJSON(context.Background(), bytes.NewReader(data), 0); e == nil {
			t.Fatal("accepted mutation", i)
		}
	}
	if _, _, e := ReadSphereCylinderPartialCommonRecipeJSON(context.Background(), bytes.NewReader(raw), 32); e == nil {
		t.Fatal("ignored limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := ReadSphereCylinderPartialCommonRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("ignored cancellation")
	}
	r := sphereCylinderPartialCommonFixture()
	r.Offset = -1
	b.Reset()
	if e := WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, r); e == nil || b.Len() != 0 {
		t.Fatal("invalid recipe published")
	}
}
func TestSphereCylinderPartialCommonRecipeOCCT(t *testing.T) {
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
		if x.Name == "offset-sphere-cylinder-partial-common" {
			wanted = x
		}
	}
	if scan.Err() != nil || wanted.Name == "" {
		t.Fatal("missing OCCT record", scan.Err())
	}
	var b bytes.Buffer
	if e = WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, sphereCylinderPartialCommonFixture()); e != nil {
		t.Fatal(e)
	}
	_, s, e := ReadSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, 0)
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
func FuzzSphereCylinderPartialCommonRecipe(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSphereCylinderPartialCommonRecipeJSON(context.Background(), &b, sphereCylinderPartialCommonFixture())
	f.Add(b.Bytes())
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, p []byte) {
		_, _, _ = ReadSphereCylinderPartialCommonRecipeJSON(context.Background(), bytes.NewReader(p), 128<<10)
	})
}

func TestSphereCylinderPartialCommonExactCapBinding(t *testing.T) {
	ctx := context.Background()
	r := sphereCylinderPartialCommonFixture()
	var b bytes.Buffer
	if e := WriteSphereCylinderPartialCommonRecipeJSON(ctx, &b, r); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	_, body, e := ReadSphereCylinderPartialCommonRecipeJSON(ctx, &b, 0)
	if e != nil {
		t.Fatal(e)
	}
	m := body.UpperCapMaterial
	if !m.Complete || m.Kind != geom.SphereCylinderCapLens || len(m.Events.Points) != 2 || len(m.Boundary) != 2 {
		t.Fatal("missing exact cap events")
	}
	for _, p := range m.Events.Points {
		if p.Base[0].Cmp(big.NewRat(159, 64)) != 0 || p.Base[2].Cmp(big.NewRat(17, 4)) != 0 {
			t.Fatal("incorrect exact radical-axis position", p)
		}
		ySquared := new(big.Rat).Mul(p.Vector[1], p.Vector[1])
		ySquared.Mul(ySquared, p.Radicand)
		if ySquared.Cmp(big.NewRat(3135, 4096)) != 0 || p.RootSign != 1 && p.RootSign != -1 {
			t.Fatal("incorrect exact algebraic event", p)
		}
	}
	if m.Events.Points[0].RootSign == m.Events.Points[1].RootSign {
		t.Fatal("collapsed events")
	}
	var envelope sphereCylinderPartialCommonEnvelope
	if e = json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	m.Events.Points[0].Radicand = big.NewRat(1, 1)
	envelope.CapMaterial, e = json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	changed, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = ReadSphereCylinderPartialCommonRecipeJSON(ctx, bytes.NewReader(changed), 0); e == nil {
		t.Fatal("accepted altered exact cap certificate")
	}
	if _, _, e = ReadSphereCylinderCommonRecipeJSON(ctx, bytes.NewReader(raw), 0); e == nil {
		t.Fatal("partial accepted as full common")
	}
	var full bytes.Buffer
	if e = WriteSphereCylinderCommonRecipeJSON(ctx, &full, sphereCylinderCommonFixture()); e != nil {
		t.Fatal(e)
	}
	if _, _, e = ReadSphereCylinderPartialCommonRecipeJSON(ctx, &full, 0); e == nil {
		t.Fatal("full common accepted as partial")
	}
	r.Upper = 6
	b.Reset()
	if e = WriteSphereCylinderPartialCommonRecipeJSON(ctx, &b, r); e == nil || b.Len() != 0 {
		t.Fatal("nonpartial cap accepted")
	}
	if e = WriteSphereCylinderPartialCommonRecipeJSON(ctx, shortWriter{}, sphereCylinderPartialCommonFixture()); e == nil {
		t.Fatal("short write ignored")
	}
}

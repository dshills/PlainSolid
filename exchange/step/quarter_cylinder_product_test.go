package step

import (
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
	"strings"
	"testing"
	"time"
)

func quarterProductDescriptor(t *testing.T) SplitCylinderDescriptor {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: -3, Y: 5, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	return SplitCylinderDescriptor{Name: "quarter posed", Frame: f, Radius: 2, Height: 3, LengthUnit: .001}
}
func TestQuarterCylinderProduct(t *testing.T) {
	d := quarterProductDescriptor(t)
	s, e := parametric.Cylinder(d.Frame, d.Radius, d.Height)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNativeQuarterCylinderProduct(&b, s, d.Name, d.LengthUnit); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	m, e := ReadQuarterCylinderProduct(&b, Limits{})
	if e != nil || m.Model.Frame != d.Frame || m.Model.Name != d.Name || m.Model.LengthUnit != d.LengthUnit || !reflect.DeepEqual(m.Model.Solid, s) {
		t.Fatal(m, e)
	}
	if m.Product == 0 || m.Definition == 0 || m.ShapeDefinition == 0 {
		t.Fatal("missing product ancestry")
	}
	if e = WriteQuarterCylinderProduct(&b, d); e != nil || !bytes.Equal(b.Bytes(), raw) {
		t.Fatal("descriptor/native mismatch", e)
	}
	if _, e = ReadSplitCylinderProduct(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("half product accepted quarters")
	}
	if _, e = ReadQuarterCylinderModel(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("bare reader broadened")
	}
	relabeled := bytes.Replace(raw, []byte("'quarter-cylinder'"), []byte("'split-cylinder'"), 1)
	if _, e = ReadSplitCylinderProduct(bytes.NewReader(relabeled), Limits{}); e == nil {
		t.Fatal("half coverage accepted relabeled quarters")
	}
	var half bytes.Buffer
	if e = WriteSplitCylinderProduct(&half, d); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuarterCylinderProduct(bytes.NewReader(half.Bytes()), Limits{}); e == nil {
		t.Fatal("quarters accepted half")
	}
	for _, bad := range []string{strings.Replace(string(raw), "'quarter-cylinder'", "'split-cylinder'", 1), strings.Replace(half.String(), "'split-cylinder'", "'quarter-cylinder'", 1), strings.Replace(string(raw), "'mechanical'", "'foreign'", 1), strings.Replace(string(raw), ".NOT_KNOWN.", ".MADE.", 1)} {
		if got, e := ReadQuarterCylinderProduct(strings.NewReader(bad), Limits{}); e == nil || got.Product != 0 || got.Model.Body != 0 {
			t.Fatal("accepted mismatched graph")
		}
	}
	faces := s.Faces()
	faces[1].Surface = splitForeignSurface{faces[1].Surface}
	foreign, e := parametric.NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = WriteNativeQuarterCylinderProduct(&b, foreign, d.Name, d.LengthUnit); e == nil || b.Len() != 0 {
		t.Fatal("foreign evaluator accepted")
	}
}
func TestQuarterCylinderProductOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	d := quarterProductDescriptor(t)
	solid, e := parametric.Cylinder(d.Frame, d.Radius, d.Height)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNativeQuarterCylinderProduct(&b, solid, d.Name, d.LengthUnit); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "quarter-product.step")
	if e = os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, p, "quarter-roots").CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	var v struct {
		Faces        int
		Volume, Area float64
		Center       [3]float64
	}
	if e = json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e, string(raw))
	}
	if v.Faces != 6 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
		t.Fatal(v)
	}
	for i, x := range []float64{-3, 5, .5} {
		if math.Abs(v.Center[i]-x) > 1e-10 {
			t.Fatal(v)
		}
	}
}

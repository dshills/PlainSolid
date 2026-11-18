package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
	"time"
)

func nativeMappedFixture(t *testing.T) NativeMappedPlanarModel {
	t.Helper()
	a, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	return NativeMappedPlanarModel{Name: "native mapped products", LengthUnit: .001, Products: []Body{{Name: "first", Solid: a}, {Name: "second", Solid: b}}, Uses: []NativeMappedPlanarUse{{Name: "first translated", Product: 0, Transform: geom.Translation(geom.Vec3{X: 10})}, {Name: "first rotated", Product: 0, Transform: geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 20}, {0, 1, 0, 0}}}, {Name: "second translated", Product: 1, Transform: geom.Translation(geom.Vec3{X: -10})}}}
}
func TestNativeMappedPlanar(t *testing.T) {
	t.Parallel()
	m := nativeMappedFixture(t)
	var b bytes.Buffer
	if e := WriteNativeMappedPlanarMaterials(&b, m, Limits{}); e != nil {
		t.Fatal(e)
	}
	a, e := ReadMappedPlanarMaterials(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Materials) != 2 {
		t.Fatal(a)
	}
}

func TestNativeMappedPlanarRejections(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"orphan", "product", "scale", "unit", "geometry", "noncardinal", "limit", "name"} {
		t.Run(kind, func(t *testing.T) {
			m := nativeMappedFixture(t)
			l := Limits{}
			switch kind {
			case "orphan":
				for i := range m.Uses {
					m.Uses[i].Product = 0
				}
			case "product":
				m.Uses[0].Product = 2
			case "scale":
				m.Uses[0].Transform[0][0] = 2
			case "unit":
				m.LengthUnit = -1
			case "geometry":
				m.Products[0].Solid = brep.Solid{}
			case "noncardinal":
				m.Products[0].Solid, _ = m.Products[0].Solid.Transformed(geom.Affine{{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
			case "limit":
				l.MaxEntities = 10
			case "name":
				m.Name = string([]byte{255})
			}
			var b bytes.Buffer
			if e := WriteNativeMappedPlanarMaterials(&b, m, l); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
}
func TestOCCTNativeMappedPlanar(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	m := nativeMappedFixture(t)
	var b bytes.Buffer
	if e := WriteNativeMappedPlanarMaterials(&b, m, Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "native-mapped.step")
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	at := bytes.LastIndex(raw, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(raw))
	}
	var out struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	if e = json.Unmarshal(raw[at:], &out); e != nil {
		t.Fatal(e)
	}
	if out.Faces != 18 || out.Solids != 3 || math.Abs(out.Volume-54) > 1e-9 || math.Abs(out.Area-126) > 1e-9 {
		t.Fatal(out)
	}
	for i, x := range []float64{85. / 18, 91. / 9, 31. / 18} {
		if math.Abs(out.Center[i]-x) > 1e-9 {
			t.Fatal(out)
		}
	}
}

func TestNativeMappedPlanarHolesAndOwnerProof(t *testing.T) {
	t.Parallel()
	m := nativeMappedFixture(t)
	rect := func(a, b, c, d float64) []geom.Vec3 {
		return []geom.Vec3{{X: a, Y: b}, {X: c, Y: b}, {X: c, Y: d}, {X: a, Y: d}}
	}
	h, e := brep.PrismWithHoles(rect(0, 0, 4, 4), [][]geom.Vec3{rect(1, 1, 3, 3)}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	m.Products[0] = Body{Name: "holed product", Solid: h}
	var b, again bytes.Buffer
	if e = WriteNativeMappedPlanarMaterials(&b, m, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteNativeMappedPlanarMaterials(&again, m, Limits{}); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic", e)
	}
	a, e := ReadMappedPlanarMaterials(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !nativeMappedSameBoundary(a.Materials[a.MapOrder[0]].Body.Solid, h) {
		t.Fatal("lost hole")
	}
	// Alter only an authored owner pcurve's source origin. Complete mapped proof
	// must reject even though the independent 3D model edge remains unchanged.
	for _, id := range a.Source.Order {
		en := a.Source.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "PCURVE" {
			rep := a.Source.Entities[en.Records[0].Args[2].Ref].Records[0]
			ln := a.Source.Entities[rep.Args[1].Items[0].Ref].Records[0]
			point := a.Source.Entities[ln.Args[1].Ref]
			point.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "999."}
			point.Records[0].Args[1].Items[1] = Value{Kind: Number, Text: "999."}
			a.Source.Entities[point.ID] = point
			break
		}
	}
	var dst bytes.Buffer
	if e = WriteMappedPlanarMaterialsSource(&dst, a.Source); e == nil || dst.Len() != 0 {
		t.Fatal("false pcurve accepted", e)
	}
}

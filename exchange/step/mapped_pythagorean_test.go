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

func pythagoreanMappedFixture(t *testing.T) (NativeMappedPlanarModel, []PythagoreanLocalFrame) {
	m := nativeMappedFixture(t)
	var e error
	m.Products[0].Solid, e = brep.Box(geom.Vec3{}, geom.Vec3{X: 10, Y: 15, Z: 20})
	if e != nil {
		t.Fatal(e)
	}
	return m, []PythagoreanLocalFrame{{Origin: geom.Vec3{X: 5, Y: -10, Z: 2}, XAxis: geom.Vec3{X: 3, Y: 4}, ZAxis: geom.Vec3{Z: 1}}, {XAxis: geom.Vec3{X: 1}, ZAxis: geom.Vec3{Z: 1}}}
}
func TestPythagoreanMapped(t *testing.T) {
	m, f := pythagoreanMappedFixture(t)
	var b bytes.Buffer
	if e := WritePythagoreanMappedPlanarMaterials(&b, m, f, Limits{}); e != nil {
		t.Fatal(e)
	}
	a, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadMappedPlanarMaterials(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("cardinal domain widened")
	}
	vs := a.Materials[a.MapOrder[0]].Body.Solid.Vertices()
	want := geom.Vec3{X: 11, Y: -2, Z: 2}
	found := false
	for _, v := range vs {
		if v == want {
			found = true
		}
	}
	if !found {
		t.Fatal(vs)
	}
}

func TestPythagoreanMappedIndependent(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-pythagorean-mapped-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Materials) != 2 {
		t.Fatal(a)
	}
	if _, e = ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old gate widened")
	}
}
func TestPythagoreanMappedReject(t *testing.T) {
	for _, kind := range []string{"irrational", "orthogonal", "rounding", "frame-limit", "limits"} {
		t.Run(kind, func(t *testing.T) {
			m, f := pythagoreanMappedFixture(t)
			l := Limits{}
			switch kind {
			case "irrational":
				f[0].XAxis = geom.Vec3{X: 1, Y: 1}
			case "orthogonal":
				f[0].ZAxis = geom.Vec3{X: 1, Z: 1}
			case "rounding":
				m.Products[0].Solid, _ = brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
			case "frame-limit":
				f[0].XAxis.X = 1e7
			case "limits":
				l.MaxEntities = 10
			}
			var out bytes.Buffer
			if e := WritePythagoreanMappedPlanarMaterials(&out, m, f, l); e == nil || out.Len() != 0 {
				t.Fatal(e)
			}
		})
	}
	raw, e := os.ReadFile("testdata/occt-pythagorean-mapped-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range a.Source.Order {
		en := a.Source.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "PCURVE" {
			rep := a.Source.Entities[en.Records[0].Args[2].Ref].Records[0]
			line := a.Source.Entities[rep.Args[1].Items[0].Ref].Records[0]
			p := a.Source.Entities[line.Args[1].Ref]
			p.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "99."}
			p.Records[0].Args[1].Items[1] = Value{Kind: Number, Text: "99."}
			a.Source.Entities[p.ID] = p
			break
		}
	}
	bad, e := mappedPlanarSourceBytes(a.Source)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("wrong owner line accepted")
	}
}
func TestOCCTPythagoreanMapped(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	m, f := pythagoreanMappedFixture(t)
	var out bytes.Buffer
	if e := WritePythagoreanMappedPlanarMaterials(&out, m, f, Limits{}); e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile("testdata/occt-pythagorean-mapped-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{source, out.Bytes()} {
		p := filepath.Join(t.TempDir(), "mapped.step")
		if e = os.WriteFile(p, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var v struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(data[at:], &v); e != nil {
			t.Fatal(e)
		}
		if v.Faces != 18 || v.Solids != 3 || math.Abs(v.Volume-6006) > 1e-7 || math.Abs(v.Area-2622) > 1e-7 {
			t.Fatal(v)
		}
		for i, x := range []float64{71943. / 6006, 61506. / 6006, 31509. / 6006} {
			if math.Abs(v.Center[i]-x) > 1e-8 {
				t.Fatal(v)
			}
		}
	}
}

func TestPythagoreanMappedSourceAdversaries(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-pythagorean-mapped-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{"3.0000000000000001", "1E999999999", "1E-999999999"} {
		t.Run(token, func(t *testing.T) {
			a, e := ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range a.Source.Order {
				en := a.Source.Entities[id]
				if len(en.Records) != 1 || en.Records[0].Type != "PLANE" {
					continue
				}
				placement := a.Source.Entities[en.Records[0].Args[1].Ref].Records[0]
				d := a.Source.Entities[placement.Args[3].Ref]
				items := d.Records[0].Args[1].Items
				if items[0].Text == "3." && items[1].Text == "4." {
					items[0] = Value{Kind: Number, Text: token}
					d.Records[0].Args[1].Items = items
					a.Source.Entities[d.ID] = d
					changed = true
					break
				}
			}
			if !changed {
				t.Fatal("fixture did not match")
			}
			data, e := mappedPlanarSourceBytes(a.Source)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(data), Limits{}); e == nil {
				t.Fatal("accepted invalid source frame")
			}
		})
	}
}
func FuzzPythagoreanMapped(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-pythagorean-mapped-planar.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 256})
	})
}

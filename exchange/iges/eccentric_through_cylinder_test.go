package iges

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
	"testing"
	"time"
)

func eccentricThroughSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/eccentric-through-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func eccentricThroughDescriptor() EccentricThroughCylinderDescriptor {
	return EccentricThroughCylinderDescriptor{Radius: 3, Height: 4, BoreRadius: 1, BoreOffset: geom.Vec2{X: 1, Y: .5}}
}
func TestEccentricThroughCylinder(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteEccentricThroughCylinderSolid(ctx, &out, eccentricThroughDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{eccentricThroughSource(t), out.Bytes()} {
		v, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if v.Descriptor != eccentricThroughDescriptor() || v.SourceSolidID != 73 || v.SourceFaceIDs[3] != 69 || len(v.Parameters) != 37 {
			t.Fatal(v)
		}
		if len(v.Native.Solids[1].Faces()) != 0 || len(v.Native.StockWallFaces[0]) == 0 || len(v.Native.CutterWallFaces[0]) == 0 || len(v.Native.StockCapFaces[0]) == 0 {
			t.Fatal("lost raw owner ancestry")
		}
		p, e := v.Native.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-32*math.Pi) > 1e-5 || math.Abs(p.Area-48*math.Pi) > 1e-5 || math.Abs(p.Centroid.Z-2) > 1e-6 || math.Abs(p.Centroid.X+.125) > 1e-8 || math.Abs(p.Centroid.Y+.0625) > 1e-8 {
			t.Fatal(p)
		}
		if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("old codec widened")
		}
	}
	d := eccentricThroughDescriptor()
	d.Origin = geom.Vec3{X: 2, Y: -3, Z: 4}
	out.Reset()
	if e := WriteEccentricThroughCylinderSolid(ctx, &out, d, 1); e != nil {
		t.Fatal(e)
	}
	v, e := ReadEccentricThroughCylinderSolid(ctx, &out, Limits{})
	if e != nil || v.Descriptor != d || v.LengthUnit != 1 {
		t.Fatal(v, e)
	}
}
func TestEccentricThroughCylinderRejects(t *testing.T) {
	ctx := context.Background()
	raw := eccentricThroughSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{13, 3, "2"}, {67, 5, "61"}, {57, 5, "1"}, {71, 5, "1"}, {69, 1, "15"}, {19, 12, "3"}, {51, 4, "359"}, {33, 17, "21"}, {5, 1, "1.00000000000000001"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true)
		if e != nil {
			t.Fatal(e)
		}
		for i := range doc.entities {
			if doc.entities[i].id == tc.id {
				doc.entities[i].values[tc.index].text = tc.value
			}
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		v, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || v.SourceSolidID != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 36}, {MaxValues: 20}} {
		if _, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-eccentric-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("rounded source promoted")
	}
	for _, d := range []EccentricThroughCylinderDescriptor{{Radius: 1, BoreRadius: 1, Height: 4}, {Radius: 3, BoreRadius: 1, Height: 0}, {Radius: 3, BoreRadius: 1, Height: -1}} {
		var out bytes.Buffer
		if e := WriteEccentricThroughCylinderSolid(ctx, &out, d, .001); e == nil || out.Len() != 0 {
			t.Fatal(d)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestOCCTEccentricThroughCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteEccentricThroughCylinderSolid(context.Background(), &out, eccentricThroughDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-eccentric-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{eccentricThroughSource(t), out.Bytes(), vendor} {
		path := filepath.Join(t.TempDir(), "blind.igs")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		b, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(b))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(b[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 4 || p.Solids != 1 || math.Abs(p.Volume-32*math.Pi) > 1e-8 || math.Abs(p.Area-48*math.Pi) > 1e-8 || math.Abs(p.Center[2]-2) > 1e-9 || math.Abs(p.Center[0]+.125) > 1e-9 || math.Abs(p.Center[1]+.0625) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func TestEccentricThroughBlindCircularDisjoint(t *testing.T) {
	ctx := context.Background()
	if _, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(blindSource(t)), Limits{}); e == nil {
		t.Fatal("blind admitted as through")
	}
	if _, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(eccentricThroughSource(t)), Limits{}); e == nil {
		t.Fatal("through admitted as blind")
	}
}

func FuzzEccentricThroughCylinder(f *testing.F) {
	f.Add(eccentricThroughSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadEccentricThroughCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 37, MaxValues: 4096})
	})
}

func TestEccentricThroughCylinderExactDomain(t *testing.T) {
	ctx := context.Background()
	for _, offset := range []geom.Vec2{{X: 1, Y: .5}, {X: -1, Y: -.5}, {Y: 1}} {
		d := eccentricThroughDescriptor()
		d.Origin = geom.Vec3{X: 4, Y: -3, Z: 2}
		d.BoreOffset = offset
		var raw bytes.Buffer
		if e := WriteEccentricThroughCylinderSolid(ctx, &raw, d, .001); e != nil {
			t.Fatal(e)
		}
		a, e := ReadEccentricThroughCylinderSolid(ctx, &raw, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = parametric.WriteJSON(&native, a.Native.Solids[0]); e != nil {
			t.Fatal(e)
		}
		s, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var again bytes.Buffer
		if e = parametric.WriteJSON(&again, s); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
			t.Fatal("native basis changed", e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || p.Centroid.Distance(geom.Vec3{X: 4 - offset.X/8, Y: -3 - offset.Y/8, Z: 4}) > 1e-8 {
			t.Fatal("displaced hole lost", p, e)
		}
	}
	for _, kind := range []string{"coaxial", "tangent", "outside", "rounded-tangency", "unrepresentable"} {
		t.Run(kind, func(t *testing.T) {
			d := eccentricThroughDescriptor()
			switch kind {
			case "coaxial":
				d.BoreOffset = geom.Vec2{}
			case "tangent":
				d.BoreOffset = geom.Vec2{X: 2}
			case "outside":
				d.BoreOffset = geom.Vec2{X: 3}
			case "rounded-tangency":
				d.BoreOffset = geom.Vec2{X: 2, Y: math.Ldexp(1, -50)}
			case "unrepresentable":
				d.Origin.X = math.Ldexp(1, 54)
			}
			var b bytes.Buffer
			if e := WriteEccentricThroughCylinderSolid(ctx, &b, d, .001); e == nil || b.Len() != 0 {
				t.Fatal("accepted unsupported/partial")
			}
		})
	}
	if _, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(throughSource(t)), Limits{}); e == nil {
		t.Fatal("coaxial dialect widened")
	}
	if _, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(eccentricThroughSource(t)), Limits{}); e == nil {
		t.Fatal("eccentric admitted as coaxial")
	}
	if _, e := ReadEccentricThroughCylinderSolid(ctx, nil, Limits{}); e == nil {
		t.Fatal("nil reader")
	}
}

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

func eccentricBlindSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/eccentric-blind-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func eccentricBlindDescriptor() EccentricBlindCylinderDescriptor {
	return EccentricBlindCylinderDescriptor{Radius: 3, Height: 4, BoreRadius: 1, Floor: 2, BoreOffset: geom.Vec2{X: 1, Y: .5}}
}
func TestEccentricBlindCylinder(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteEccentricBlindCylinderSolid(ctx, &out, eccentricBlindDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{eccentricBlindSource(t), out.Bytes()} {
		v, e := ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if v.Descriptor != eccentricBlindDescriptor() || v.SourceSolidID != 77 || v.SourceFaceIDs[4] != 73 || len(v.Parameters) != 39 {
			t.Fatal(v)
		}
		if len(v.Native.Solids[1].Faces()) != 0 || len(v.Native.StockWallFaces[0]) == 0 || len(v.Native.CutterWallFaces[0]) == 0 || len(v.Native.StockCapFaces[0]) == 0 || len(v.Native.CutterCapFaces[0]) == 0 {
			t.Fatal("lost material/floor ancestry")
		}
		p, e := v.Native.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-34*math.Pi) > 1e-5 || math.Abs(p.Area-46*math.Pi) > 1e-5 || math.Abs(p.Centroid.Z-33./17) > 1e-6 || math.Abs(p.Centroid.X+1./17) > 1e-8 || math.Abs(p.Centroid.Y+1./34) > 1e-8 {
			t.Fatal(p)
		}
		if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("old codec widened")
		}
	}
	d := eccentricBlindDescriptor()
	d.Origin = geom.Vec3{X: 2, Y: -3, Z: 4}
	out.Reset()
	if e := WriteEccentricBlindCylinderSolid(ctx, &out, d, 1); e != nil {
		t.Fatal(e)
	}
	v, e := ReadEccentricBlindCylinderSolid(ctx, &out, Limits{})
	if e != nil || v.Descriptor != d || v.LengthUnit != 1 {
		t.Fatal(v, e)
	}
}
func TestEccentricBlindCylinderRejects(t *testing.T) {
	ctx := context.Background()
	raw := eccentricBlindSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{13, 3, "2"}, {69, 5, "61"}, {59, 5, "1"}, {75, 5, "1"}, {73, 1, "15"}, {21, 12, "3"}, {53, 4, "359"}, {35, 17, "23"}, {75, 11, "0"}, {63, 5, "0"}, {5, 1, "1.00000000000000001"}} {
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
		v, e := ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || v.SourceSolidID != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 38}, {MaxValues: 20}} {
		if _, e := ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-eccentric-blind-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("rounded source promoted")
	}
	for _, d := range []EccentricBlindCylinderDescriptor{{Radius: 1, BoreRadius: 1, Height: 4, Floor: 2}, {Radius: 3, BoreRadius: 1, Height: 4, Floor: 4}, {Radius: 3, BoreRadius: 1, Height: 4, Floor: 0}} {
		var out bytes.Buffer
		if e := WriteEccentricBlindCylinderSolid(ctx, &out, d, .001); e == nil || out.Len() != 0 {
			t.Fatal(d)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestOCCTEccentricBlindCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteEccentricBlindCylinderSolid(context.Background(), &out, eccentricBlindDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-eccentric-blind-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{eccentricBlindSource(t), out.Bytes(), vendor} {
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
		if p.Faces != 5 || p.Solids != 1 || math.Abs(p.Volume-34*math.Pi) > 1e-8 || math.Abs(p.Area-46*math.Pi) > 1e-8 || math.Abs(p.Center[2]-33./17) > 1e-9 || math.Abs(p.Center[0]+1./17) > 1e-9 || math.Abs(p.Center[1]+1./34) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func FuzzEccentricBlindCylinder(f *testing.F) {
	f.Add(eccentricBlindSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadEccentricBlindCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 39, MaxValues: 4096})
	})
}

func TestEccentricBlindCylinderFloorAndReload(t *testing.T) {
	ctx := context.Background()
	for _, floor := range []float64{.5, 2, 3} {
		for _, offset := range []geom.Vec2{{X: 1, Y: .5}, {X: -1, Y: -.5}, {Y: 1}} {
			d := eccentricBlindDescriptor()
			d.Origin = geom.Vec3{X: 4, Y: -3, Z: 2}
			d.Floor = floor
			d.BoreOffset = offset
			var raw bytes.Buffer
			if e := WriteEccentricBlindCylinderSolid(ctx, &raw, d, .001); e != nil {
				t.Fatal(e)
			}
			a, e := ReadEccentricBlindCylinderSolid(ctx, &raw, Limits{})
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range a.Native.CutterCapFaces[0] {
				f := a.Native.Solids[0].Faces()[id]
				u, v := (f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2
				p, e := f.Surface.Eval(u, v)
				if e != nil || math.Abs(p.Z-(2+floor)) > 1e-12 {
					t.Fatal("wrong original floor", p, e)
				}
				du, dv, e := f.Surface.Partials(u, v)
				if e != nil {
					t.Fatal(e)
				}
				n := du.Cross(dv)
				if f.Reversed {
					n = n.Scale(-1)
				}
				if n.Z <= 0 || n.X != 0 || n.Y != 0 {
					t.Fatal("floor not reversed cutter-bottom orientation", n)
				}
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
				t.Fatal("native reload changed chart", e)
			}
			depth := 4 - floor
			v := 36 - depth
			want := geom.Vec3{X: 4 - depth*offset.X/v, Y: -3 - depth*offset.Y/v, Z: 2 + (72-depth*(4+floor)/2)/v}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(p.Volume-v*math.Pi) > 1e-8 || p.Centroid.Distance(want) > 1e-8 {
				t.Fatal("blind centroid", p, want, e)
			}
		}
	}
	for _, kind := range []string{"coaxial", "radial-touch", "radial-rounded", "floor-bottom", "floor-top", "floor-below", "floor-above", "unrepresentable"} {
		t.Run(kind, func(t *testing.T) {
			d := eccentricBlindDescriptor()
			switch kind {
			case "coaxial":
				d.BoreOffset = geom.Vec2{}
			case "radial-touch":
				d.BoreOffset = geom.Vec2{X: 2}
			case "radial-rounded":
				d.BoreOffset = geom.Vec2{X: 2, Y: math.Ldexp(1, -50)}
			case "floor-bottom":
				d.Floor = 0
			case "floor-top":
				d.Floor = 4
			case "floor-below":
				d.Floor = -1
			case "floor-above":
				d.Floor = 5
			case "unrepresentable":
				d.Origin.Z = math.Ldexp(1, 54)
			}
			var b bytes.Buffer
			if e := WriteEccentricBlindCylinderSolid(ctx, &b, d, .001); e == nil || b.Len() != 0 {
				t.Fatal("invalid clearances/partial output")
			}
		})
	}
	for _, raw := range [][]byte{blindSource(t), throughSource(t), eccentricThroughSource(t)} {
		if _, e := ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("other family accepted")
		}
	}
	raw := eccentricBlindSource(t)
	if _, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("coaxial API widened")
	}
	if _, e := ReadEccentricThroughCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("through API widened")
	}
}

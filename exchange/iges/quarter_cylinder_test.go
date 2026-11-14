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

func quarterCylinderSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/quarter-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestQuarterCylinder(t *testing.T) {
	ctx := context.Background()
	a, e := ReadQuarterCylinderSolid(ctx, bytes.NewReader(quarterCylinderSource(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Descriptor != (QuarterCylinderDescriptor{Radius: 2, Height: 3}) || len(a.Parameters) != 35 || a.NativeFaceSourceIDs != [5]int{57, 61, 59, 63, 65} {
		t.Fatal("ancestry")
	}
	for _, d := range []QuarterCylinderDescriptor{a.Descriptor, {Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 1.5}} {
		var b bytes.Buffer
		if e = WriteQuarterCylinderSolid(ctx, &b, d, .001); e != nil {
			t.Fatal(e)
		}
		q, e := ReadQuarterCylinderSolid(ctx, &b, Limits{})
		if e != nil || q.Descriptor != d || q.LengthUnit != .001 {
			t.Fatal(e)
		}
		var n bytes.Buffer
		if e = parametric.WriteJSON(&n, q.Solid); e != nil {
			t.Fatal(e)
		}
		raw := append([]byte(nil), n.Bytes()...)
		solid, e := parametric.ReadJSON(&n, parametric.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		n.Reset()
		if e = parametric.WriteJSON(&n, solid); e != nil || !bytes.Equal(raw, n.Bytes()) {
			t.Fatal("reload", e)
		}
		p, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		r, h := d.Radius, d.Height
		want := d.Origin.Add(geom.Vec3{X: 4 * r / (3 * math.Pi), Y: 4 * r / (3 * math.Pi), Z: h / 2})
		if e != nil || math.Abs(p.Volume-math.Pi*r*r*h/4) > 1e-8 || math.Abs(p.Area-(math.Pi*r*h/2+math.Pi*r*r/2+2*r*h)) > 1e-8 || p.Centroid.Distance(want) > 1e-8 {
			t.Fatal(p, e)
		}
	}
}
func TestQuarterCylinderReject(t *testing.T) {
	raw := quarterCylinderSource(t)
	ctx := context.Background()
	for _, m := range []struct {
		id, index int
		value     string
	}{{15, 3, "2.00000000000000001"}, {25, 5, "1.5"}, {29, 7, "-2"}, {43, 6, "2"}, {45, 5, "3"}, {47, 5, "0"}, {65, 1, "21"}, {67, 7, "1"}, {13, 1, "1"}, {15, 4, "9"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true)
		if e != nil {
			t.Fatal(e)
		}
		for i := range doc.entities {
			if doc.entities[i].id == m.id {
				doc.entities[i].values[m.index].text = m.value
			}
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		a, e := ReadQuarterCylinderSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || a.SourceSolidID != 0 || len(a.Solid.Faces()) != 0 {
			t.Fatal("accepted mutation", m, e)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-quarter-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuarterCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("vendor graph was silently normalized")
	}

	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 34}, {MaxValues: 20}} {
		if _, e := ReadQuarterCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadQuarterCylinderSolid(cancelled, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancellation")
	}
	for _, unit := range []float64{0, -1, math.NaN()} {
		var out bytes.Buffer
		if e := WriteQuarterCylinderSolid(ctx, &out, QuarterCylinderDescriptor{Radius: 2, Height: 3}, unit); e == nil || out.Len() != 0 {
			t.Fatal("unit failure was not atomic")
		}
	}

	var b bytes.Buffer
	if e := WriteQuarterCylinderSolid(ctx, &b, QuarterCylinderDescriptor{Origin: geom.Vec3{X: math.Exp2(53)}, Radius: 1, Height: 3}, .001); e == nil || b.Len() != 0 {
		t.Fatal("rounded output")
	}
}
func TestOCCTQuarterCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteQuarterCylinderSolid(context.Background(), &out, QuarterCylinderDescriptor{Radius: 2, Height: 3}, .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-quarter-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{quarterCylinderSource(t), out.Bytes(), vendor} {
		path := filepath.Join(t.TempDir(), "frustum.igs")
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var m struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(data[at:], &m); e != nil {
			t.Fatal(e)
		}
		if m.Faces != 5 || m.Solids != 1 || math.Abs(m.Volume-3*math.Pi) > 1e-8 || math.Abs(m.Area-(5*math.Pi+12)) > 1e-8 || math.Abs(m.Center[2]-1.5) > 1e-9 || math.Abs(m.Center[0]-8/(3*math.Pi)) > 1e-9 || math.Abs(m.Center[1]-8/(3*math.Pi)) > 1e-9 {
			t.Fatal(m)
		}
	}
}
func FuzzQuarterCylinder(f *testing.F) {
	f.Add(quarterCylinderSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		a, e := ReadQuarterCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 35, MaxValues: 2048})
		if e != nil && (a.SourceSolidID != 0 || len(a.Solid.Faces()) != 0) {
			t.Fatal("partial material")
		}
	})
}

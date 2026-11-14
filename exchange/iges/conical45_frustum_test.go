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

func conical45FrustumSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/conical45-frustum.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestConical45Frustum(t *testing.T) {
	ctx := context.Background()
	a, e := ReadConical45FrustumSolid(ctx, bytes.NewReader(conical45FrustumSource(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Descriptor != (Conical45FrustumDescriptor{Radius: 1, Height: 2}) || len(a.Parameters) != 20 || a.SourceSolidID != 39 || a.NativeFaceSourceIDs != [3]int{35, 31, 33} {
		t.Fatal("source identity")
	}
	for _, d := range []Conical45FrustumDescriptor{a.Descriptor, {Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 1.5}} {
		var out bytes.Buffer
		if e = WriteConical45FrustumSolid(ctx, &out, d, .001); e != nil {
			t.Fatal(e)
		}
		v, e := ReadConical45FrustumSolid(ctx, &out, Limits{})
		if e != nil || v.Descriptor != d {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = parametric.WriteJSON(&native, v.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var again bytes.Buffer
		_ = parametric.WriteJSON(&again, s)
		if !bytes.Equal(native.Bytes(), again.Bytes()) {
			t.Fatal("native support basis changed")
		}
		r, h := d.Radius, d.Height
		rt := r + h
		sum := r*r + r*rt + rt*rt
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := d.Origin.Add(geom.Vec3{Z: h * (r*r + 2*r*rt + 3*rt*rt) / (4 * sum)})
		if e != nil || math.Abs(p.Volume-math.Pi*h*sum/3) > 1e-8 || math.Abs(p.Area-math.Pi*(r*r+rt*rt+(r+rt)*h*math.Sqrt2)) > 1e-8 || p.Centroid.Distance(want) > 1e-8 {
			t.Fatal(p, want, e)
		}
		q, e := s.Faces()[1].Surface.Eval(0, .5)
		if e != nil || q != d.Origin.Add(geom.Vec3{X: r + h/2, Z: h / 2}) {
			t.Fatal("linear cone meridian drift", q, e)
		}
	}
}
func TestConical45FrustumReject(t *testing.T) {
	ctx := context.Background()
	raw := conical45FrustumSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{9, 4, "44.5"}, {9, 4, "45.000000000000001"}, {9, 3, "1.00000000000000001"}, {9, 5, "5"}, {15, 2, "2"}, {17, 2, "0.25"}, {21, 4, "2.9"}, {25, 5, "1"}, {37, 3, "0"}, {37, 7, "1"}, {33, 1, "13"}, {23, 5, "2"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for i := range doc.entities {
			if doc.entities[i].id == tc.id {
				doc.entities[i].values[tc.index].text = tc.value
				changed = true
			}
		}
		if !changed {
			t.Fatal(tc)
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadConical45FrustumSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || got.SourceSolidID != 0 {
			t.Fatal("accepted invalid", tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 19}, {MaxValues: 20}} {
		if _, e := ReadConical45FrustumSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, d := range []Conical45FrustumDescriptor{{Radius: 0, Height: 2}, {Radius: 1, Height: 0}, {Radius: 1, Height: -2}, {Origin: geom.Vec3{X: math.Ldexp(1, 54)}, Radius: 1, Height: 2}} {
		var b bytes.Buffer
		if e := WriteConical45FrustumSolid(ctx, &b, d, .001); e == nil || b.Len() != 0 {
			t.Fatal("invalid/partial", d)
		}
	}
	if _, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old cylinder widened")
	}
	if _, e := ReadConical45Supports(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("standalone support widened")
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-conical45-frustum.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConical45FrustumSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("rounded vendor graph promoted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadConical45FrustumSolid(cancelled, bytes.NewReader(raw), Limits{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestOCCTConical45Frustum(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteConical45FrustumSolid(context.Background(), &out, Conical45FrustumDescriptor{Radius: 1, Height: 2}, .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-conical45-frustum.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{conical45FrustumSource(t), out.Bytes(), vendor} {
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
		if m.Faces != 3 || m.Solids != 1 || math.Abs(m.Volume-26*math.Pi/3) > 1e-8 || math.Abs(m.Area-math.Pi*(10+8*math.Sqrt2)) > 1e-8 || math.Abs(m.Center[2]-17./13) > 1e-9 {
			t.Fatal(m)
		}
	}
}
func FuzzConical45Frustum(f *testing.F) {
	f.Add(conical45FrustumSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		a, e := ReadConical45FrustumSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 20, MaxValues: 2048})
		if e != nil && (a.SourceSolidID != 0 || len(a.Solid.Faces()) != 0) {
			t.Fatal("partial material")
		}
	})
}

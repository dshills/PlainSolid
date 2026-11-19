package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func weightedBoreAssemblyFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-weighted-station-bore-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func weightedBoreAssemblyModel(m WrappedRationalStationBoreAssembly) WrappedRationalStationBoreAssemblyModel {
	q := WrappedRationalStationBoreAssemblyModel{LengthUnit: m.Body.LengthUnit, Products: []WrappedRationalStationBoreAssemblyPart{{ID: "root", Name: "Bored assembly"}, {ID: "part", Name: m.Product.Name, Body: &m.Body}}}
	for i, u := range m.Assembly.Occurrences {
		q.Occurrences = append(q.Occurrences, AssemblyInstance{ID: string(rune('a' + i)), Name: u.Name, Parent: "root", Child: "part", Transform: u.Transform})
	}
	return q
}
func TestWrappedRationalStationBoreAssembly(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := weightedBoreAssemblyFixture(t)
	m, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.SourceFaceIDs) != 6 || len(m.SourceOuterWallFaceIDs) != 2 || len(m.SourceInnerWallFaceIDs) != 2 || len(m.Occurrences) != 2 {
		t.Fatal(m.SourceFaceIDs)
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("filled admitted bore")
	}
	if _, e = ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(weightedAssemblyFixture(t)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("bore admitted filled")
	}
	var b bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssembly(ctx, &b, weightedBoreAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationBoreAssembly(ctx, &b, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	var n bytes.Buffer
	if e = trimmed.WriteJSON(&n, m.Body.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSONContext(ctx, &n, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Body, e = WrappedRationalStationBoreFromSolid(ctx, s, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationBoreAssembly(ctx, &b, weightedBoreAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte("(10.,0.,0.)"), []byte("(10.00000000000000001,0.,0.)"), 1), bytes.Replace(raw, []byte("(3.25,-0.5,0.)"), []byte("(3.5,-0.5,0.)"), 1), bytes.Replace(raw, []byte("#296 = FACE_BOUND('',#297,.F.);"), []byte("#296 = FACE_BOUND('',#297,.T.);"), 1)} {
		if bytes.Equal(raw, bad) {
			t.Fatal("missing mutation")
		}
		got, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
		if e == nil || got.Assembly.Source != nil {
			t.Fatal("accepted malformed source")
		}
	}
}
func TestWrappedRationalStationBoreAssemblyLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_STATION_ASSEMBLY")
	if bin == "" {
		t.Skip("optional independent XCAF")
	}
	// The live test re-proves four separately ingested sources. Give it room
	// to run under race instrumentation alongside the kernel qualification.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	raw := weightedBoreAssemblyFixture(t)
	m, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadWrappedRationalStationBoreAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var generated, source bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssembly(ctx, &generated, weightedBoreAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationBoreAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	fresh := filepath.Join(t.TempDir(), "fresh.step")
	if b, e := exec.CommandContext(ctx, bin, "write-bore", fresh).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	freshRaw, e := os.ReadFile(fresh)
	if e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Faces int
		Shared        bool
		Occurrences   [][12]float64
	}
	var first measure
	for i, b := range [][]byte{raw, generated.Bytes(), source.Bytes(), freshRaw} {
		proof, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(b), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "bore.step")
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		reply, e := exec.CommandContext(ctx, bin, "read", p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(reply))
		}
		start := bytes.LastIndex(reply, []byte("{\"volume\""))
		if start < 0 {
			t.Fatal(string(reply))
		}
		var got measure
		if e = json.Unmarshal(reply[start:], &got); e != nil {
			t.Fatal(e)
		}
		if got.Solids != 2 || got.Faces != 12 || !got.Shared || len(got.Occurrences) != 2 || math.Abs(got.Volume-2*24.3558991065) > 1e-6 {
			t.Fatal(got)
		}
		if i == 0 {
			first = got
		} else {
			if math.Abs(got.Volume-first.Volume) > 1e-6 || math.Abs(got.Area-first.Area) > 1e-6 {
				t.Fatal("mass", got, first)
			}
			for j := range got.Centroid {
				if math.Abs(got.Centroid[j]-first.Centroid[j]) > 1e-7 {
					t.Fatal("centroid")
				}
			}
		}
		for j, u := range proof.Assembly.Occurrences {
			for r := 0; r < 3; r++ {
				for c := 0; c < 4; c++ {
					if got.Occurrences[j][4*r+c] != u.Transform[r][c] {
						t.Fatal("pose")
					}
				}
			}
		}
	}
}

func FuzzWrappedRationalStationBoreAssembly(f *testing.F) {
	f.Add(weightedBoreAssemblyFixture(f))
	f.Add(weightedAssemblyFixture(f))
	f.Add([]byte("ISO-10303-21;END-ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedRationalStationBoreAssembly(context.Background(), bytes.NewReader(b), Limits{}, WrappedSurfaceOptions{MaxOperations: 200000})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial proof")
		}
	})
}

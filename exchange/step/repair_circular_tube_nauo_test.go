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
	"testing"
	"time"
)

func circularTubeSource(t testing.TB) ([]byte, int64) {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-circular-tube-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadAssembly(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range a.Products {
		if p.Name == "Circular tube part" {
			return b, p.DefinitionID
		}
	}
	t.Fatal("missing product")
	return nil, 0
}
func TestRepairCircularTubeNAUO(t *testing.T) {
	t.Parallel()
	b, id := circularTubeSource(t)
	a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(b), Limits{}, id, CircularNAUORepairOptions{1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if a.Body.OuterRadius != 3 || a.Body.InnerRadius != 1 || a.Body.Height != 5 || a.LengthUnit != .001 || len(a.Solid.Faces()) != 16 {
		t.Fatal(a.Body, len(a.Solid.Faces()))
	}
	for _, w := range a.Walls {
		if len(w.Source.FailedConditions) != 4 {
			t.Fatal(w.Source)
		}
	}
	m, e := a.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-40*math.Pi) > 1e-8 || math.Abs(m.Area-56*math.Pi) > 1e-8 {
		t.Fatal(m, e)
	}
	if _, e = RepairCircularNAUOProduct(bytes.NewReader(b), Limits{}, id, CircularNAUORepairOptions{1e-9}); e == nil {
		t.Fatal("old family widened")
	}
}

func TestRepairCircularTubeNativeAndAuthoredSTEP(t *testing.T) {
	t.Parallel()
	raw, id := circularTubeSource(t)
	a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(raw), Limits{}, id, CircularNAUORepairOptions{1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = parametric.WriteJSON(&b, a.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := parametric.ReadJSON(bytes.NewReader(b.Bytes()), parametric.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var c bytes.Buffer
	if e = parametric.WriteJSON(&c, s); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
		t.Fatal("native reload changed geometry", e)
	}
	b.Reset()
	if e = WriteTubes(&b, TubeModel{Bodies: []TubeBody{a.Body}, LengthUnit: a.LengthUnit}); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(m.Bodies) != 1 {
		t.Fatal("new authored output", e)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-40*math.Pi) > 1e-8 {
		t.Fatal(p, e)
	}
	for i, w := range a.Walls {
		if w.SourceFaceSense != (i == 0) {
			t.Fatal("source wall orientation")
		}
		for j, r := range w.Source.Rims {
			want := r.CylinderPCurveID
			if i == 1 {
				want = r.PlanePCurveID
			}
			if w.RimMasterPCurveIDs[j] != want || r.CapFaceID != a.CapFaceIDs[j] {
				t.Fatal("original authority lost")
			}
		}
	}
}
func TestRepairCircularTubeNAUOReject(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"tolerance", "small", "faces", "vertices", "uses", "inner-positive", "cap-sense", "cap-loop-sense", "shared-loop", "wrong-master", "wrong-owner", "wrong-radius", "noncoaxial", "height", "seam-gap", "cross-product", "extra-face"} {
		t.Run(kind, func(t *testing.T) {
			raw, id := circularTubeSource(t)
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			l := Limits{}
			o := CircularNAUORepairOptions{1e-9}
			set := func(id int64, n int, v Value) { q := f.Entities[id]; q.Records[0].Args[n] = v; f.Entities[id] = q }
			switch kind {
			case "tolerance":
				o.MaxDistance = 1e-3
			case "small":
				o.MaxDistance = 1e-14
			case "faces":
				l.MaxFaces = 15
			case "vertices":
				l.MaxVertices = 15
			case "uses":
				l.MaxEdgeUses = 63
			case "inner-positive":
				set(550, 3, Value{Kind: Enumeration, Text: "T"})
			case "cap-sense":
				set(520, 3, Value{Kind: Enumeration, Text: "T"})
			case "cap-loop-sense":
				set(526, 4, Value{Kind: Enumeration, Text: "T"})
			case "shared-loop":
				set(524, 1, Value{Kind: Reference, Ref: 522})
			case "wrong-master":
				set(495, 3, Value{Kind: Enumeration, Text: "PCURVE_S2"})
			case "wrong-owner":
				set(508, 1, Value{Kind: Reference, Ref: 411})
			case "wrong-radius":
				set(509, 2, Value{Kind: Number, Text: "3."})
			case "noncoaxial":
				set(511, 1, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1e-20"}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}})
			case "height":
				set(516, 1, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "4."}}})
			case "seam-gap":
				for _, pid := range []int64{494, 529, 558} {
					set(pid, 1, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1."}, {Kind: Number, Text: "0.001"}, {Kind: Number, Text: "0."}}})
				}
			case "cross-product":
				set(70, 1, Value{Kind: Reference, Ref: 424})
			case "extra-face":
				q := f.Entities[396]
				q.Records[0].Args[1].Items = append(q.Records[0].Args[1].Items, Value{Kind: Reference, Ref: 44})
				f.Entities[396] = q
			}
			data, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := RepairCircularTubeNAUOProduct(bytes.NewReader(data), l, id, o)
			if e == nil || got.SourceBodyID != 0 || len(got.Solid.Faces()) != 0 || got.Walls[0].Source.SourceBodyID != 0 {
				t.Fatal("partial/accepted unsupported", kind, e)
			}
		})
	}
}
func TestOCCTRepairCircularTubeNAUO(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, id := circularTubeSource(t)
	a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(raw), Limits{}, id, CircularNAUORepairOptions{1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteTubes(&b, TubeModel{Bodies: []TubeBody{a.Body}, LengthUnit: a.LengthUnit}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "reconstructed-tube.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	run := func(path string) measure {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var m measure
		if e = json.Unmarshal(data[at:], &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	source, repaired := run("testdata/occt-circular-tube-nauo.step"), run(path)
	if source.Faces != 16 || source.Solids != 3 || repaired.Faces != 4 || repaired.Solids != 1 || math.Abs(source.Volume-(48+40*math.Pi)) > 1e-8 || math.Abs(source.Area-(104+56*math.Pi)) > 1e-8 || math.Abs(repaired.Volume-(source.Volume-48)) > 1e-8 || math.Abs(repaired.Area-(source.Area-104)) > 1e-8 || math.Abs(repaired.Center[2]-2.5) > 1e-9 {
		t.Fatal(source, repaired)
	}
}
func FuzzRepairCircularTubeNAUO(f *testing.F) {
	raw, id := circularTubeSource(f)
	f.Add(raw, 1e-9)
	for _, name := range []string{"occt-circular-tube-nauo-cm.step", "occt-circular-tube-nauo-cm-scaled.step"} {
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b, 1e-10)
	}
	f.Add(raw, 1e-14)
	f.Add([]byte("ISO-10303-21;"), 1e-9)
	f.Fuzz(func(t *testing.T, b []byte, tol float64) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 32, MaxVertices: 32, MaxEdgeUses: 128}, id, CircularNAUORepairOptions{tol})
		if e != nil && (a.SourceBodyID != 0 || len(a.Solid.Faces()) != 0) {
			t.Fatal("partial repair")
		}
	})
}

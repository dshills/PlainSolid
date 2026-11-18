package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func weightedAssemblyFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-weighted-station-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestWrappedRationalStationAssembly(t *testing.T) {
	raw := weightedAssemblyFixture(t)
	m, e := ReadWrappedRationalStationAssembly(context.Background(), bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Body.Walls) != 2 || len(m.Occurrences) != 2 || len(m.SourceWallFaceIDs) != 2 {
		t.Fatal(m)
	}
	t.Log(m.SourceFaceIDs, m.SourceWallFaceIDs)
}

func weightedAssemblyModel(m WrappedRationalStationAssembly) WrappedRationalStationAssemblyModel {
	out := WrappedRationalStationAssemblyModel{Products: []WrappedRationalStationAssemblyPart{{ID: "root", Name: "Weighted assembly"}, {ID: "part", Name: m.Product.Name, Body: &m.Body}}, LengthUnit: m.Assembly.LengthUnit}
	for _, u := range m.Assembly.Occurrences {
		out.Occurrences = append(out.Occurrences, AssemblyInstance{ID: u.Identifier, Name: u.Name, Parent: "root", Child: "part", Transform: u.Transform})
	}
	return out
}
func TestWrappedRationalStationAssemblyRoundTrip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := weightedAssemblyFixture(t)
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("standalone admitted assembly")
	}
	if _, e = ReadWrappedMaterialAssembly(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("old mixed route admitted weighted family")
	}
	if m.Product.DefinitionID != 31 || m.SourceBodyID != 37 || !reflect.DeepEqual(m.SourceWallFaceIDs, []int64{39, 174}) {
		t.Fatal(m)
	}
	ref := rationalStationSTEPFixture(t, 2)
	for i := range m.Body.Walls {
		a, _ := m.Body.Walls[i].Geometry.ExportRecipe()
		b, _ := ref.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("original support changed")
		}
	}
	var source, native, out bytes.Buffer
	if e = WriteWrappedRationalStationAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	re, e := ReadWrappedRationalStationAssembly(ctx, &source, Limits{}, WrappedSurfaceOptions{})
	if e != nil || !reflect.DeepEqual(m.SourceWallFaceIDs, re.SourceWallFaceIDs) || !reflect.DeepEqual(m.Occurrences, re.Occurrences) {
		t.Fatal(e)
	}
	if e = trimmed.WriteJSON(&native, m.Body.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	body, e := WrappedRationalStationLoftFromSolid(ctx, solid, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	body.CapNames = m.Body.CapNames
	m.Body = body
	if e = WriteWrappedRationalStationAssembly(ctx, &out, weightedAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	n, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range m.Body.Walls {
		a, _ := m.Body.Walls[i].Geometry.ExportRecipe()
		b, _ := n.Body.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("export flattened or replaced support")
		}
	}
	if len(n.Assembly.Products) != 2 || len(n.Occurrences) != 2 || n.Occurrences[0].ChildID != n.Occurrences[1].ChildID {
		t.Fatal("lost shared asset")
	}
	// Caches are not writer authority; source supports still reconstruct the body.
	bad := weightedAssemblyModel(m)
	bad.Products[1].Body = &WrappedRationalStationLoftBody{LengthUnit: m.Body.LengthUnit}
	var buf bytes.Buffer
	if e = WriteWrappedRationalStationAssembly(ctx, &buf, bad, Limits{}, WrappedSurfaceOptions{}); e == nil || buf.Len() != 0 {
		t.Fatal("partial invalid write")
	}
	if e = WriteWrappedRationalStationAssembly(ctx, occurrenceShortWriter{}, weightedAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
func TestWrappedRationalStationAssemblyReject(t *testing.T) {
	raw := string(weightedAssemblyFixture(t))
	ctx := context.Background()
	for _, c := range [][2]string{
		{"#16 = CARTESIAN_POINT('',(10.,0.,0.))", "#16 = CARTESIAN_POINT('',(10.00000000000000001,0.,0.))"},
		{"#12 = CARTESIAN_POINT('',(0.,0.,0.))", "#12 = CARTESIAN_POINT('',(1.,0.,0.))"},
		{"#45 = CARTESIAN_POINT('',(3.25,-0.5,0.))", "#45 = CARTESIAN_POINT('',(3.5,-0.5,0.))"},
		{"#5,#31,$", "#31,#31,$"},
		{"#28 = PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#7))", "#28 = PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#33))"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=MANIFOLD_SOLID_BREP('',#38);\nENDSEC;\nEND-ISO-10303-21;"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(raw, c[0], c[1], 1)
		if bad == raw {
			t.Fatal("stale", c)
		}
		m, e := ReadWrappedRationalStationAssembly(ctx, strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
		if e == nil || m.SourceBodyID != 0 {
			t.Fatal("accepted", c, e)
		}
		f, e := Parse(strings.NewReader(bad), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteWrappedRationalStationAssemblySource(ctx, &out, f, Limits{}, WrappedSurfaceOptions{}); e == nil || out.Len() != 0 {
			t.Fatal("partial")
		}
	}
	if _, e := ReadWrappedRationalStationAssembly(ctx, strings.NewReader(raw), Limits{MaxEntities: 257}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("entity limit")
	}
	if _, e := ReadWrappedRationalStationAssembly(ctx, strings.NewReader(raw), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("work limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadWrappedRationalStationAssembly(canceled, strings.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("cancel")
	}
	metre := strings.ReplaceAll(raw, "SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT($,.METRE.)")
	m, e := ReadWrappedRationalStationAssembly(ctx, strings.NewReader(metre), Limits{}, WrappedSurfaceOptions{})
	if e != nil || m.Body.LengthUnit != 1 {
		t.Fatal(e)
	}
}
func TestWrappedRationalStationAssemblyLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_STATION_ASSEMBLY")
	if bin == "" {
		t.Skip("optional independent XCAF author/observer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := weightedAssemblyFixture(t)
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var envelope bytes.Buffer
	if e = WriteWrappedRationalStationAssemblyJSON(ctx, &envelope, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadWrappedRationalStationAssemblyJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var generated, source, native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Body.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Body, e = WrappedRationalStationLoftFromSolid(ctx, solid, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationAssembly(ctx, &generated, weightedAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	fresh := filepath.Join(t.TempDir(), "fresh.step")
	if b, e := exec.CommandContext(ctx, bin, "write", fresh).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	freshRaw, e := os.ReadFile(fresh)
	if e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Faces int
		Shared        bool
		Occurrences   [][12]float64
	}
	var original mass
	for k, b := range [][]byte{raw, generated.Bytes(), source.Bytes(), freshRaw} {
		proof, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(b), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "body.step")
		os.WriteFile(path, b, 0600)
		reply, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(reply))
		}
		start := bytes.LastIndex(reply, []byte("{\"volume\""))
		if start < 0 {
			t.Fatal(string(reply))
		}
		var got mass
		if e = json.Unmarshal(reply[start:], &got); e != nil {
			t.Fatal(e)
		}
		if got.Solids != 2 || got.Faces != 8 || !got.Shared || len(got.Occurrences) != 2 || math.Abs(got.Volume-18*rationalStationSTEPArea()) > 1e-6 {
			t.Fatal(k, got)
		}
		if k == 0 {
			original = got
		} else {
			if math.Abs(got.Volume-original.Volume) > 1e-6 || math.Abs(got.Area-original.Area) > 1e-6 {
				t.Fatal("measure", k, got, original)
			}
			for j := range got.Centroid {
				if math.Abs(got.Centroid[j]-original.Centroid[j]) > 1e-7 {
					t.Fatal("centroid", k, got, original)
				}
			}
		}
		for i, u := range proof.Assembly.Occurrences {
			for r := 0; r < 3; r++ {
				for c := 0; c < 4; c++ {
					if got.Occurrences[i][r*4+c] != u.Transform[r][c] {
						t.Fatal("pose", i, got)
					}
				}
			}
		}
	}
}
func FuzzWrappedRationalStationAssembly(f *testing.F) {
	f.Add(weightedAssemblyFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadWrappedRationalStationAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 512}, WrappedSurfaceOptions{MaxOperations: 1000000})
		if e != nil {
			if m.SourceBodyID != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteWrappedRationalStationAssemblySource(context.Background(), &out, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{MaxOperations: 1000000}); e != nil {
			t.Fatal(e)
		}
	})
}

func TestWrappedRationalStationAssemblyReordered(t *testing.T) {
	ctx := context.Background()
	f, e := Parse(bytes.NewReader(weightedAssemblyFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil || !reflect.DeepEqual(m.SourceWallFaceIDs, []int64{39, 174}) || m.SourceBodyID != 37 {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteWrappedRationalStationAssemblySource(ctx, &out, f, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}

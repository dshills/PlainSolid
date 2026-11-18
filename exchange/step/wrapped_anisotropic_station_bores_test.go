package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
	"time"
)

func anisotropicBoreSTEPFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-weighted-station-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestWrappedAnisotropicStationBoreSTEP(t *testing.T) {
	slowtest.Skip(t)
	raw := anisotropicBoreSTEPFixture(t)
	m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Outer.Walls) != 2 || len(m.Inner.Walls) != 2 {
		t.Fatal("original bands")
	}
	proof, e := m.Solid.ValidateWrappedAnisotropicStationBore(context.Background(), geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	if m.Solid.Topology().EulerCharacteristic != 0 || m.Solid.Topology().InnerWires != 2 {
		t.Fatal("annular cap topology")
	}
	if _, e = ReadWrappedRationalStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("homothetic widened")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(context.Background(), &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := WrappedAnisotropicStationBoreFromSolid(context.Background(), solid, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&out, recovered); e != nil {
		t.Fatal(e)
	}
	again, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for k, a := range []WrappedAnisotropicStationLoftBody{m.Outer, m.Inner} {
		b := []WrappedAnisotropicStationLoftBody{again.Outer, again.Inner}[k]
		for i, w := range a.Walls {
			x, _ := w.Geometry.ExportRecipe()
			y, _ := b.Walls[i].Geometry.ExportRecipe()
			if !reflect.DeepEqual(x.Surface.Data(), y.Surface.Data()) {
				t.Fatal("original support")
			}
			if i > 0 && b.Walls[i-1].SideEdges[2] != b.Walls[i].SideEdges[0] {
				t.Fatal("shared rim")
			}
		}
	}
	var stable bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&stable, again); e != nil || !bytes.Equal(out.Bytes(), stable.Bytes()) {
		t.Fatal("stable", e)
	}
}

func TestWrappedAnisotropicStationBoreSTEPReject(t *testing.T) {
	slowtest.Skip(t)
	raw := anisotropicBoreSTEPFixture(t)
	for index, edit := range []func(*File){
		func(f *File) { sideDiskEditRecord(f, "PCURVE", func(r *Record) { r.Args[1] = r.Args[2] }) },
		func(f *File) {
			sideDiskEditRecord(f, "ADVANCED_FACE", func(r *Record) {
				if r.Args[3].Text == "T" {
					r.Args[3].Text = "F"
				} else {
					r.Args[3].Text = "T"
				}
			})
		},
		func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_SURFACE", func(r *Record) { r.Args[0].Items[1].Items[0] = Value{Kind: Number, Text: "0.50000000000000001"} })
		},
	} {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
		if e == nil || len(m.Outer.Walls) != 0 {
			t.Fatal("bad original", index)
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxFaces: 2}, {MaxVertices: 4}} {
		if _, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), l, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("limits")
		}
	}
	if _, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("work")
	}
	m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Inner = m.Outer
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("touching walls")
	}
}
func TestWrappedAnisotropicStationBoreSTEPLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional independent OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dir := t.TempDir()
	source, report := filepath.Join(dir, "original.step"), filepath.Join(dir, "report.json")
	if b, e := exec.CommandContext(ctx, bin, source, report, "bore").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area   float64
		Centroid       [3]float64
		Solids, Shells int
	}
	data, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var original mass
	if e = json.Unmarshal(data, &original); e != nil {
		t.Fatal(e)
	}
	options := trimmed.WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 1000)}
	outer, e := m.Outer.Solid.BoundWrappedAnisotropicStationMass(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := m.Inner.Solid.BoundWrappedAnisotropicStationMass(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	factor := new(big.Rat).Sub(outer.AxialVolumeFactor, inner.AxialVolumeFactor)
	zfactor := new(big.Rat).Sub(outer.AxialFirstZFactor, inner.AxialFirstZFactor)
	if factor.Cmp(big.NewRat(217, 32)) != 0 || new(big.Rat).Quo(zfactor, factor).Cmp(big.NewRat(39, 28)) != 0 {
		t.Fatal("independent axial integrals")
	}
	lo, _ := new(big.Rat).Sub(outer.Volume.Lower, inner.Volume.Upper).Float64()
	hi, _ := new(big.Rat).Sub(outer.Volume.Upper, inner.Volume.Lower).Float64()
	if original.Volume < lo-1e-9 || original.Volume > hi+1e-9 || math.Abs(original.Centroid[2]-39.0/28) > 1e-8 {
		t.Fatal("independent material", original, lo, hi)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := WrappedAnisotropicStationBoreFromSolid(ctx, solid, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&output, recovered); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "generated.step")
	if e = os.WriteFile(path, output.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if b, e := exec.CommandContext(ctx, bin, path, report, "read").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	data, e = os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var q mass
	if e = json.Unmarshal(data, &q); e != nil {
		t.Fatal(e)
	}
	if q.Solids != 1 || q.Shells != 1 || math.Abs(q.Volume-original.Volume) > 1e-8 || math.Abs(q.Area-original.Area) > 1e-8 {
		t.Fatal(q, original)
	}
	for i := range q.Centroid {
		if math.Abs(q.Centroid[i]-original.Centroid[i]) > 1e-8 {
			t.Fatal(q, original)
		}
	}
}

func FuzzWrappedAnisotropicStationBoreSTEP(f *testing.F) {
	f.Add(anisotropicBoreSTEPFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}, WrappedSurfaceOptions{MaxSurfaces: 32, MaxOperations: 4000000})
		if e != nil && len(m.Outer.Walls) != 0 {
			t.Fatal("partial material")
		}
	})
}

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func scaledNURBSFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-scaled-nurbs-planar-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestScaledNURBSPlanarAssembly(t *testing.T) {
	raw := scaledNURBSFixture(t)
	ctx := context.Background()
	m, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.ChartScalings()) != 12 || len(m.PCurveScalings()) != 48 || len(m.ModelCurves()) != 24 || len(m.NormalizedPCurves()) != 48 || len(m.Geometry.Assembly.Occurrences) != 3 {
		t.Fatal("source role loss")
	}
	if _, e = ReadNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old unit derivative mode widened")
	}
	unit, e := ReadNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.NormalizedSTEP()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(unit.Geometry.Assembly.Occurrences, m.Geometry.Assembly.Occurrences) {
		t.Fatal("placement changed")
	}
	originalFile, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	normalizedFile, e := Parse(bytes.NewBufferString(m.NormalizedSTEP()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	changed := map[int64]bool{}
	for _, c := range m.ChartScalings() {
		changed[c.Source.SurfaceID] = true
	}
	for _, p := range m.PCurveScalings() {
		changed[p.PoleIDs[0]] = true
		changed[p.PoleIDs[1]] = true
	}
	if !reflect.DeepEqual(originalFile.Order, normalizedFile.Order) || !reflect.DeepEqual(originalFile.Header, normalizedFile.Header) {
		t.Fatal("metadata/record order changed")
	}
	for id, en := range originalFile.Entities {
		if !changed[id] && !reflect.DeepEqual(en, normalizedFile.Entities[id]) {
			t.Fatal("unrelated record changed", id)
		}
	}
	nonunit := 0
	byID := map[int64]NURBSPlanarChartScaling{}
	for _, c := range m.ChartScalings() {
		byID[c.Source.SurfaceID] = c
		for _, s := range c.Scale {
			if s != "1" {
				nonunit++
			}
		}
		if c.Source.FaceID == 0 || c.Source.U != [2]string{"0", "1"} || c.Source.V != [2]string{"0", "1"} {
			t.Fatal(c)
		}
	}
	if nonunit < 12 {
		t.Fatal("weak fixture")
	}
	rat := func(s string) *big.Rat {
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return r
	}
	for _, p := range m.PCurveScalings() {
		c := byID[p.SurfaceID]
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				want := new(big.Rat).Mul(rat(p.SourcePoles[i][j]), rat(c.Scale[j]))
				if want.Cmp(rat(p.ProofPoles[i][j])) != 0 {
					t.Fatal("incorrect exact scaling")
				}
			}
		}
	}
	vols := map[float64]int{}
	for _, bs := range m.Geometry.ProductBodies {
		for _, b := range bs {
			mass, e := b.Body.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			vols[mass.Volume]++
		}
	}
	if vols[64] != 1 || vols[8] != 1 {
		t.Fatal(vols)
	}
	c := m.ChartScalings()[0]
	copy := m.ChartScalings()
	copy[0].Scale[0] = "forged"
	if m.ChartScalings()[0] != c {
		t.Fatal("mutable evidence")
	}
	var out bytes.Buffer
	if e = WriteScaledNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil || !bytes.Equal(out.Bytes(), raw) {
		t.Fatal("source changed", e)
	}
	n, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(n.ChartScalings(), m.ChartScalings()) || !reflect.DeepEqual(n.PCurveScalings(), m.PCurveScalings()) {
		t.Fatal("source evidence changed", e)
	}
	// Existing unit charts are still a valid special case, without changing the old mode.
	if _, e = ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(nurbsPlanarFixture(t)), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestScaledNURBSPlanarAssemblyReject(t *testing.T) {
	raw := scaledNURBSFixture(t)
	ctx := context.Background()
	m, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	c, p := m.ChartScalings()[0], m.PCurveScalings()[0]
	mutations := map[string]func(*File){
		"rounded UV": func(f *File) { f.Entities[p.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "0.00000000000000000001" },
		"rounded pole": func(f *File) {
			f.Entities[c.Source.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "0.00000000000000000001"
		},
		"nonaffine pole":      func(f *File) { f.Entities[c.Source.PoleIDs[3]].Records[0].Args[1].Items[0].Text = "1." },
		"wrong chart owner":   func(f *File) { f.Entities[p.PCurveID].Records[0].Args[1].Ref = m.ChartScalings()[1].Source.SurfaceID },
		"UV domain":           func(f *File) { f.Entities[p.PoleIDs[1]].Records[0].Args[1].Items[0].Text = "2." },
		"reversed knot range": func(f *File) { f.Entities[c.Source.SurfaceID].Records[0].Args[10].Items[0].Text = "2." },
		"unrepresentable scaled knot": func(f *File) {
			r := f.Entities[c.Source.SurfaceID].Records[0]
			r.Args[10].Items[0].Text = ".5"
			r.Args[10].Items[1].Text = "2."
		},
		"shared UV pole": func(f *File) {
			f.Entities[p.CurveID].Records[0].Args[2].Items[0].Ref = m.PCurveScalings()[1].PoleIDs[0]
		},
		"underflow": func(f *File) { f.Entities[c.Source.SurfaceID].Records[0].Args[10].Items[0].Text = "1.E-999999999" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || got.original != "" || len(got.Geometry.ProductBodies) != 0 {
				t.Fatal("accepted mutation", e)
			}
		})
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{}); e != nil {
		t.Fatal("reordered exact source", e)
	}
}
func TestScaledNURBSPlanarAssemblyLimits(t *testing.T) {
	raw := scaledNURBSFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 600}, {MaxSolids: 1}, {MaxValues: 128}} {
		if m, e := ReadScaledNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), l); e == nil || m.original != "" {
			t.Fatal("accepted limit", l)
		}
	}
	var out bytes.Buffer
	if e := WriteScaledNURBSPlanarAssemblySource(context.Background(), &out, ScaledNURBSPlanarAssembly{}, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("invalid write touched output")
	}
}
func TestScaledNURBSPlanarAssemblyLive(t *testing.T) {
	raw := scaledNURBSFixture(t)
	m, e := ReadScaledNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	model := ColoredAssemblyModel{LengthUnit: m.Geometry.Assembly.LengthUnit}
	key := func(id int64) string { return strconv.FormatInt(id, 10) }
	for _, p := range m.Geometry.Assembly.Products {
		model.Products = append(model.Products, ColoredAssemblyPart{ID: key(p.DefinitionID), Name: p.Name, Bodies: m.Geometry.ProductBodies[p.DefinitionID]})
	}
	for _, o := range m.Geometry.Assembly.Occurrences {
		model.Occurrences = append(model.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: key(o.Parent), Child: key(o.Child), Transform: o.Transform})
	}
	var generated, retained bytes.Buffer
	if e = WriteColoredAssembly(&generated, model); e != nil {
		t.Fatal(e)
	}
	if e = WriteScaledNURBSPlanarAssemblySource(context.Background(), &retained, m, Limits{}); e != nil {
		t.Fatal(e)
	}
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_SCALED_NURBS_PLANAR")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_SCALED_NURBS_PLANAR")
	}
	shiftedPath := filepath.Join(t.TempDir(), "shifted-original.step")
	authorCtx, authorCancel := context.WithTimeout(context.Background(), 30*time.Second)
	authored, err := exec.CommandContext(authorCtx, bin, "write-shifted", shiftedPath).CombinedOutput()
	authorCancel()
	if err != nil {
		t.Fatalf("%v: %s", err, authored)
	}
	shifted, e := os.ReadFile(shiftedPath)
	if e != nil {
		t.Fatal(e)
	}
	shiftedModel, e := ReadScaledNURBSPlanarAssembly(context.Background(), bytes.NewReader(shifted), Limits{})
	if e != nil {
		t.Fatal("genuine shifted chart", e)
	}
	for _, c := range shiftedModel.ChartScalings() {
		if c.Source.U != [2]string{"-2", "-1"} || c.Source.V != [2]string{"3", "4"} {
			t.Fatal("shifted fixture did not retain source basis", c)
		}
	}
	for _, b := range [][]byte{raw, retained.Bytes(), []byte(m.NormalizedSTEP()), generated.Bytes(), shifted, []byte(shiftedModel.NormalizedSTEP())} {
		path := filepath.Join(t.TempDir(), "assembly.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v: %s", e, out)
		}
		var q struct {
			Volume, Area  float64
			Solids, Faces int
			Centroid      [3]float64
			Colors        [][3]float64
		}
		at := bytes.IndexByte(out, '{')
		if at < 0 || json.Unmarshal(out[at:], &q) != nil {
			t.Fatal(string(out))
		}
		colors := map[[3]float64]int{}
		for _, c := range q.Colors {
			colors[c]++
		}
		if colors[[3]float64{1, 0, 0}] != 1 || colors[[3]float64{0, 0, 1}] != 1 {
			t.Fatal("changed product colors", q)
		}
		if math.Abs(q.Volume-136) > 1e-9 || math.Abs(q.Area-252) > 1e-9 || math.Abs(q.Centroid[0]-6.5) > 1e-9 || math.Abs(q.Centroid[1]-180.0/17) > 1e-9 || math.Abs(q.Centroid[2]-57.0/17) > 1e-9 || len(q.Colors) != 2 || q.Solids != 3 || q.Faces != 18 {
			t.Fatal(q)
		}
	}
}
func FuzzScaledNURBSPlanarAssembly(f *testing.F) {
	f.Add(scaledNURBSFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadScaledNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024})
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial failure")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteScaledNURBSPlanarAssemblySource(context.Background(), &out, m, Limits{}); e != nil || !bytes.Equal(b, out.Bytes()) {
			t.Fatal("unstable source", e)
		}
	})
}

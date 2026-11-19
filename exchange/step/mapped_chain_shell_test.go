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
	"slices"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func mappedChainSTEPFixture(t *testing.T) (trimmed.MappedSplineChainShell, []byte) {
	t.Helper()
	ctx := context.Background()
	b, err := os.ReadFile("testdata/mapped_chain_rectangular_source.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(b), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var file bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &file, source, "Mapped chain", .001, Limits{}, MappedChainShellSTEPOptions{}); err != nil {
		t.Fatal(err)
	}
	return source, file.Bytes()
}

func TestMappedChainRectangularSTEP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source, file := mappedChainSTEPFixture(t)
	loaded, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Mapped chain" || loaded.LengthUnit != .001 || loaded.OriginalSTEP() != string(file) ||
		len(loaded.Native.Solid.Vertices()) != 12 || len(loaded.Native.Solid.Edges()) != 17 || len(loaded.FaceIDs) != 7 || len(loaded.Chain) != 3 {
		t.Fatal("loaded STEP topology or source graph")
	}
	if x := loaded.Shell.Topology(); x.Faces != 7 || x.Edges != 15 || x.Vertices != 10 || x.EulerCharacteristic != 2 {
		t.Fatal("original whole-chain topology", x)
	}
	wantStations := []string{"0", "1/4", "1/2", "1"}
	for i, p := range loaded.Native.ChainStations {
		if p.RatString() != wantStations[i] {
			t.Fatal("wrong exact global model station", i, p)
		}
	}
	wantRaw := [2][3]trimmed.UVInterval{
		{{Min: 2, Max: 3}, {Min: 3, Max: 5}, {Min: 5, Max: 9}},
		{{Min: 23, Max: 28}, {Min: 17, Max: 23}, {Min: 10, Max: 17}},
	}
	wantWeights := [2][3]string{{"3/4", "2/3", "2"}, {"1/4", "1/2", "4"}}
	wantKnots := [2][]float64{{2, 2, 2, 2, 5, 5, 5, 9, 9, 9, 9}, {10, 10, 10, 10, 17, 17, 17, 23, 23, 23, 28, 28, 28, 28}}
	faces := loaded.Native.Solid.Faces()
	expected, err := source.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	original := expected.Solid.Faces()
	for i, f := range faces {
		if !geom.SameSplineSurface(f.Surface.(*geom.BSplineSurface), original[i].Surface.(*geom.BSplineSurface)) ||
			f.Reversed != original[i].Reversed {
			t.Fatal("changed original support basis or face direction", i)
		}
	}
	for owner := range 2 {
		first := loaded.Chain[0].Owners[owner]
		for i, cell := range loaded.Chain {
			got := cell.Owners[owner]
			u := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			s := u.Source()
			want := original[owner].Outer[i].Curve.(trimmed.MappedSplineUV).Source()
			if !slices.Equal(s.Curve.Poles(), want.Curve.Poles()) || !slices.Equal(s.Curve.Weights(), want.Curve.Weights()) {
				t.Fatal("changed original spline poles or weights", owner, i)
			}
			if first.BasisID != got.BasisID || got.FaceID != loaded.FaceIDs[owner] || s.Range != wantRaw[owner][i] ||
				s.Reversed != (owner == 1) || u.ParameterWeight().RatString() != wantWeights[owner][i] ||
				!slices.Equal(s.Curve.Knots(), wantKnots[owner]) {
				t.Fatal("lost exact original pcurve source or parameter map", owner, i)
			}
		}
	}
	if loaded.Chain[0].Owners[0].BasisID == loaded.Chain[0].Owners[1].BasisID {
		t.Fatal("distinct original spline bases must stay distinct")
	}
	p, err := loaded.Native.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(p.Volume-57.0/64) > 1e-8 {
		t.Fatal("native source mass integration", p.Volume, err)
	}
	var second bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &second, loaded.Shell, loaded.Name, loaded.LengthUnit, Limits{}, MappedChainShellSTEPOptions{}); err != nil || !bytes.Equal(file, second.Bytes()) {
		t.Fatal("canonical original coefficient roundtrip", err)
	}
	t.Logf("STEP bytes=%d source roof segments=%d solid volume=%.12g", len(file), len(loaded.Chain), p.Volume)
}

func TestMappedChainRectangularSTEPRejectsChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source, file := mappedChainSTEPFixture(t)
	loaded, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cell := loaded.Chain[0]
	record := func(f *File, id int64, index int, val Value) {
		e := f.Entities[id]
		e.Records[0].Args[index] = val
		f.Entities[id] = e
	}
	mutations := []struct {
		name   string
		mutate func(*File)
	}{
		{"same endpoints and tangent changed model interior", func(f *File) {
			b := f.Entities[cell.ModelCurveID].Records[0]
			pole := b.Args[2].Items[3].Ref
			point := f.Entities[pole].Records[0].Args[1]
			value, err := affineSplineSourceRat(point.Items[2])
			if err != nil {
				t.Fatal(err)
			}
			value.Add(value, big.NewRat(1, 1024))
			raw, err := mappedChainStepDecimal(value)
			if err != nil {
				t.Fatal(err)
			}
			point.Items[2].Text = raw
			record(f, pole, 1, point)
		}},
		{"borrowed pcurve support", func(f *File) {
			side := f.Entities[cell.Owners[0].PCurveID].Records[0].Args[1]
			record(f, cell.Owners[1].PCurveID, 1, side)
		}},
		{"wrong original raw endpoint", func(f *File) {
			id := cell.Owners[0].TrimmedCurveID
			v := f.Entities[id].Records[0].Args[2]
			v.Items[0].Items[0].Text = "2.125"
			record(f, id, 2, v)
		}},
		{"copied source vertex differs by topology", func(f *File) {
			a := f.Entities[cell.EdgeID].Records[0].Args[1]
			original := f.Entities[a.Ref]
			original.ID = f.Order[len(f.Order)-1] + 1
			f.Entities[original.ID] = original
			f.Order = append(f.Order, original.ID)
			record(f, cell.EdgeID, 1, Value{Kind: Reference, Ref: original.ID})
		}},
		{"different source loop orientation", func(f *File) {
			id := loaded.FaceIDs[1]
			bound := f.Entities[id].Records[0].Args[1].Items[0].Ref
			record(f, bound, 2, Value{Kind: Enumeration, Text: "T"})
		}},
		{"changed model station order", func(f *File) {
			node := f.Entities[cell.ModelCurveID].Records[0].Args[7]
			node.Items[1].Text = "0.3125"
			record(f, cell.ModelCurveID, 7, node)
		}},
	}
	for _, attack := range mutations {
		t.Run(attack.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(file), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			attack.mutate(f)
			changed := serializeWrappedProofFile(f)
			bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(changed), Limits{}, MappedChainShellSTEPOptions{})
			if err == nil || len(bad.Native.Solid.Faces()) != 0 || bad.OriginalSTEP() != "" {
				t.Fatal("changed STEP source escaped the original geometry proof")
			}
		})
	}
	for _, limits := range []Limits{{MaxBytes: int64(len(file) - 1)}, {MaxEntities: 1}, {MaxEdgeUses: 33}} {
		bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), limits, MappedChainShellSTEPOptions{})
		if err == nil || len(bad.Chain) != 0 {
			t.Fatal("STEP read quotas must discard proof")
		}
		var out bytes.Buffer
		if err := WriteMappedChainRectangularShell(ctx, &out, source, loaded.Name, loaded.LengthUnit, limits, MappedChainShellSTEPOptions{}); err == nil || out.Len() != 0 {
			t.Fatal("STEP write quotas must discard complete output")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ReadMappedChainRectangularShell(cancelled, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal("reader cancellation", err)
	}
	var out bytes.Buffer
	if err := WriteMappedChainRectangularShell(cancelled, &out, source, loaded.Name, loaded.LengthUnit, Limits{}, MappedChainShellSTEPOptions{}); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("writer cancellation", err)
	}
}

func TestMappedChainRectangularSTEPOCCT(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP solid transfer")
	}
	_, file := mappedChainSTEPFixture(t)
	path := filepath.Join(t.TempDir(), "mapped-chain.step")
	if err := os.WriteFile(path, file, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("OCCT STEP transfer: %v\n%s", err, out)
	}
	var ref struct {
		Volume, Area  float64
		Faces, Solids int
		Centroid      [3]float64
	}
	if err := json.Unmarshal(out, &ref); err != nil {
		t.Fatal(err, strings.TrimSpace(string(out)))
	}
	loaded, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := loaded.Native.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil {
		t.Fatal(err)
	}
	c := p.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length()
	if ref.Solids != 1 || ref.Faces != 7 || math.Abs(ref.Volume-p.Volume) > 1e-8 || math.Abs(ref.Area-p.Area) > 1e-8 || c > 1e-8 {
		t.Fatal("independent STEP transfer", ref, p)
	}
	t.Logf("OCCT STEP volume=%.12g area=%.13g center=%v", ref.Volume, ref.Area, ref.Centroid)
}

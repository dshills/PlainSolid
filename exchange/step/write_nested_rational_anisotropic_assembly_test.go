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
	"strconv"
	"testing"
	"time"
)

func nestedRationalWriteModels(t testing.TB, bore bool, raw []byte) (NestedRationalWrappedAnisotropicStationAssemblyModel, NestedRationalWrappedAnisotropicStationBoreAssemblyModel) {
	t.Helper()
	ctx := context.Background()
	var a NestedRationalWrappedAnisotropicStationAssemblyModel
	var b NestedRationalWrappedAnisotropicStationBoreAssemblyModel
	var uses []ExactNAUOOccurrence
	var root, group int64
	if bore {
		m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b = NestedRationalWrappedAnisotropicStationBoreAssemblyModel{Products: []WrappedAnisotropicStationBoreAssemblyPart{{ID: "root", Name: "Root"}, {ID: "group", Name: "Shared subgroup"}, {ID: "leaf", Name: "Local bore", Body: &m.Body}}, LengthUnit: m.Body.LengthUnit}
		uses, root, group = m.Occurrences, m.Roots[0], m.GroupID
	} else {
		m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a = NestedRationalWrappedAnisotropicStationAssemblyModel{Products: []WrappedAnisotropicStationAssemblyPart{{ID: "root", Name: "Root"}, {ID: "group", Name: "Shared subgroup"}, {ID: "leaf", Name: "Local filled", Body: &m.Body}}, LengthUnit: m.Body.LengthUnit}
		uses, root, group = m.Occurrences, m.Roots[0], m.GroupID
	}
	for _, u := range uses {
		p := RationalAnisotropicAssemblyInstance{ID: strconv.FormatInt(u.NAUOID, 10), Name: u.Name, Parent: "group", Child: "leaf"}
		if u.ParentID == root {
			p.Parent = "root"
		}
		if u.ChildID == group {
			p.Child = "group"
		}
		m := u.Matrix()
		for i := 0; i < 3; i++ {
			p.Origin[i], p.XAxis[i], p.ZAxis[i] = m[i][3], m[i][0], m[i][2]
		}
		if bore {
			b.Occurrences = append(b.Occurrences, p)
		} else {
			a.Occurrences = append(a.Occurrences, p)
		}
	}
	return a, b
}
func writeNestedRationalTest(ctx context.Context, out *bytes.Buffer, a NestedRationalWrappedAnisotropicStationAssemblyModel, b NestedRationalWrappedAnisotropicStationBoreAssemblyModel, bore bool) error {
	if bore {
		return WriteNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, out, b, Limits{}, WrappedSurfaceOptions{})
	}
	return WriteNestedRationalWrappedAnisotropicStationAssembly(ctx, out, a, Limits{}, WrappedSurfaceOptions{})
}
func TestWriteNestedRationalAnisotropic(t *testing.T) {
	for _, bore := range []bool{false, true} {
		raw := nestedRationalAnisotropicFixture(t)
		if bore {
			raw = nestedRationalAnisotropicBoreFixture(t)
		}
		a, b := nestedRationalWriteModels(t, bore, raw)
		var out, again bytes.Buffer
		if e := writeNestedRationalTest(context.Background(), &out, a, b, bore); e != nil {
			t.Fatal(e)
		}
		if e := writeNestedRationalTest(context.Background(), &again, a, b, bore); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
			t.Fatal("unstable", e)
		}
		if !bytes.Contains(out.Bytes(), []byte("AP242_MANAGED_MODEL_BASED_3D_ENGINEERING")) {
			t.Fatal("schema")
		}
		if bore {
			if _, e := ReadRationalWrappedAnisotropicStationBoreAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat gate")
			}
		} else {
			if _, e := ReadRationalWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat gate")
			}
		}
	}
}
func TestWriteNestedRationalAnisotropicReject(t *testing.T) {
	raw := nestedRationalAnisotropicFixture(t)
	for _, edit := range []func(*NestedRationalWrappedAnisotropicStationAssemblyModel){
		func(m *NestedRationalWrappedAnisotropicStationAssemblyModel) {
			m.Occurrences[0].Origin[0] = big.NewRat(1, 3)
		},
		func(m *NestedRationalWrappedAnisotropicStationAssemblyModel) { m.Occurrences[0].Child = "root" },
		func(m *NestedRationalWrappedAnisotropicStationAssemblyModel) { m.Occurrences = m.Occurrences[:2] },
		func(m *NestedRationalWrappedAnisotropicStationAssemblyModel) {
			for i := range m.Occurrences {
				m.Occurrences[i].XAxis = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}
				m.Occurrences[i].Origin[0] = new(big.Rat).SetInt(new(big.Int).Mul(big.NewInt(9), new(big.Int).Exp(big.NewInt(10), big.NewInt(62), nil)))
			}
		},
	} {
		a, b := nestedRationalWriteModels(t, false, raw)
		edit(&a)
		var out bytes.Buffer
		if e := writeNestedRationalTest(context.Background(), &out, a, b, false); e == nil || out.Len() != 0 {
			t.Fatal("accepted", e)
		}
	}
	a, b := nestedRationalWriteModels(t, false, raw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if e := writeNestedRationalTest(ctx, &out, a, b, false); e == nil || out.Len() != 0 {
		t.Fatal("cancel")
	}
}
func TestWriteNestedRationalAnisotropicLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	for _, bore := range []bool{false, true} {
		ctx := context.Background()
		authorCtx, cancel := context.WithTimeout(ctx, time.Minute)
		dir := t.TempDir()
		src := filepath.Join(dir, "source.step")
		report := filepath.Join(dir, "report.json")
		mode := "filled-nested-oblique"
		if bore {
			mode = "bore-nested-oblique"
		}
		reply, err := exec.CommandContext(authorCtx, bin, src, report, mode).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatal(err, string(reply))
		}
		type mass struct {
			Volume, Area               float64
			Centroid                   [3]float64
			Solids, Shells             int
			Shared                     bool
			Placements, LeafPlacements [][3][4]float64
		}
		data, e := os.ReadFile(report)
		if e != nil {
			t.Fatal(e)
		}
		var original mass
		if e = json.Unmarshal(data, &original); e != nil {
			t.Fatal(e)
		}
		raw, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		for _, shift := range []int64{0, 7} {
			a, b := nestedRationalWriteModels(t, bore, raw)
			uses := a.Occurrences
			if bore {
				uses = b.Occurrences
			}
			for i := range uses {
				if uses[i].Parent == "root" && uses[i].Origin[0].Sign() != 0 {
					uses[i].Origin[0].Add(uses[i].Origin[0], big.NewRat(shift, 1))
				}
			}
			var generated bytes.Buffer
			if e = writeNestedRationalTest(ctx, &generated, a, b, bore); e != nil {
				t.Fatal(e)
			}
			var instances []RationalHierarchyInstance
			var source *File
			var native bytes.Buffer
			if bore {
				m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(generated.Bytes()), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &native, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				instances, source = m.Instances, m.Source
			} else {
				m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(generated.Bytes()), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				instances, source = m.Instances, m.Source
			}
			regenerated, e := mappedPlanarSourceBytes(source)
			if e != nil {
				t.Fatal(e)
			}
			dst := filepath.Join(dir, "generated.step")
			if e = os.WriteFile(dst, regenerated, 0600); e != nil {
				t.Fatal(e)
			}
			if reply, e := exec.CommandContext(ctx, bin, dst, report, "read-nested-assembly").CombinedOutput(); e != nil {
				t.Fatal(e, string(reply))
			}
			data, e = os.ReadFile(report)
			if e != nil {
				t.Fatal(e)
			}
			var q mass
			if e = json.Unmarshal(data, &q); e != nil {
				t.Fatal(e)
			}
			if !q.Shared || q.Solids != 2 || q.Shells != 2 || len(q.LeafPlacements) != 2 || len(q.Placements) != 2 || math.Abs(q.Volume-original.Volume) > 1e-6 || math.Abs(q.Area-original.Area) > 1e-7 {
				t.Fatal(q, original)
			}
			for i := range q.Centroid {
				want := original.Centroid[i]
				if i == 0 {
					want += float64(shift) / 2
				}
				if math.Abs(q.Centroid[i]-want) > 1e-7 {
					t.Fatal(q, original)
				}
			}
			for i, u := range instances {
				matrix := u.Matrix()
				for r := 0; r < 3; r++ {
					for c := 0; c < 4; c++ {
						v, _ := matrix[r][c].Float64()
						if math.Abs(q.LeafPlacements[i][r][c]-v) > 1e-14 {
							t.Fatal("composed leaf", q.LeafPlacements[i], matrix)
						}
					}
				}
			}
			j := 0
			for _, u := range uses {
				if u.Parent != "root" {
					continue
				}
				pose, e := mappedRationalPose(RationalMappedPlacement{Origin: u.Origin, XAxis: u.XAxis, ZAxis: u.ZAxis})
				if e != nil {
					t.Fatal(e)
				}
				matrix := (ExactMappedOccurrence{pose: pose}).Matrix()
				for r := 0; r < 3; r++ {
					for c := 0; c < 4; c++ {
						v, _ := matrix[r][c].Float64()
						if math.Abs(q.Placements[j][r][c]-v) > 1e-14 {
							t.Fatal("subgroup", q.Placements[j], matrix)
						}
					}
				}
				j++
			}
		}
	}
}

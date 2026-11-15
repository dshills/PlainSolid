package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func rationalAnisotropicWriteModel(t testing.TB, bore bool, raw []byte) (RationalWrappedAnisotropicStationAssemblyModel, RationalWrappedAnisotropicStationBoreAssemblyModel) {
	t.Helper()
	ctx := context.Background()
	var a RationalWrappedAnisotropicStationAssemblyModel
	var b RationalWrappedAnisotropicStationBoreAssemblyModel
	var uses []ExactNAUOOccurrence
	if bore {
		m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b = RationalWrappedAnisotropicStationBoreAssemblyModel{Products: []WrappedAnisotropicStationBoreAssemblyPart{{ID: "root", Name: "Root"}, {ID: "leaf", Name: "Weighted bore", Body: &m.Body}}, LengthUnit: m.Body.LengthUnit}
		uses = m.Occurrences
	} else {
		m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a = RationalWrappedAnisotropicStationAssemblyModel{Products: []WrappedAnisotropicStationAssemblyPart{{ID: "root", Name: "Root"}, {ID: "leaf", Name: "Weighted filled", Body: &m.Body}}, LengthUnit: m.Body.LengthUnit}
		uses = m.Occurrences
	}
	for i, u := range uses {
		matrix := u.Matrix()
		p := RationalAnisotropicAssemblyInstance{ID: string(rune('a' + i)), Name: "Original use", Parent: "root", Child: "leaf"}
		for j := 0; j < 3; j++ {
			p.Origin[j] = matrix[j][3]
			p.XAxis[j] = matrix[j][0]
			p.ZAxis[j] = matrix[j][2]
		}
		if bore {
			b.Occurrences = append(b.Occurrences, p)
		} else {
			a.Occurrences = append(a.Occurrences, p)
		}
	}
	return a, b
}
func writeRationalAnisotropicTest(ctx context.Context, w io.Writer, a RationalWrappedAnisotropicStationAssemblyModel, b RationalWrappedAnisotropicStationBoreAssemblyModel, bore bool) error {
	if bore {
		return WriteRationalWrappedAnisotropicStationBoreAssembly(ctx, w, b, Limits{}, WrappedSurfaceOptions{})
	}
	return WriteRationalWrappedAnisotropicStationAssembly(ctx, w, a, Limits{}, WrappedSurfaceOptions{})
}
func TestWriteRationalAnisotropicAssembly(t *testing.T) {
	for _, bore := range []bool{false, true} {
		raw := rationalAnisotropicFixture(t)
		if bore {
			raw = rationalAnisotropicBoreFixture(t)
		}
		a, b := rationalAnisotropicWriteModel(t, bore, raw)
		var out bytes.Buffer
		if e := writeRationalAnisotropicTest(context.Background(), &out, a, b, bore); e != nil {
			t.Fatal(e)
		}
		if !bytes.Contains(out.Bytes(), []byte("AP242_MANAGED_MODEL_BASED_3D_ENGINEERING")) || !bytes.Contains(out.Bytes(), []byte("ap242_managed_model_based_3d_engineering")) {
			t.Fatal("schema")
		}
		// Raw 3:4 direction ratios normalize exactly; caller values stay owned.
		uses := a.Occurrences
		if bore {
			uses = b.Occurrences
		}
		uses[1].XAxis[0] = big.NewRat(3, 1)
		uses[1].XAxis[1] = big.NewRat(4, 1)
		var stable bytes.Buffer
		if e := writeRationalAnisotropicTest(context.Background(), &stable, a, b, bore); e != nil || !bytes.Equal(out.Bytes(), stable.Bytes()) {
			t.Fatal("unstable", e)
		}
		if uses[1].XAxis[0].Cmp(big.NewRat(3, 1)) != 0 || uses[1].XAxis[1].Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("caller ratio changed")
		}
		if e := writeRationalAnisotropicTest(context.Background(), sideDiskShortWriter{}, a, b, bore); !errors.Is(e, io.ErrShortWrite) {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if bore {
			m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteRationalWrappedAnisotropicStationBoreAssemblyJSON(context.Background(), &native, m); e != nil {
				t.Fatal(e)
			}
			if _, e = ReadRationalWrappedAnisotropicStationBoreAssemblyJSON(context.Background(), &native, IndependentMaterialJSONLimits{}); e != nil {
				t.Fatal(e)
			}
		} else {
			m, e := ReadRationalWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), &native, m); e != nil {
				t.Fatal(e)
			}
			if _, e = ReadRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), &native, IndependentMaterialJSONLimits{}); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestWriteRationalAnisotropicReject(t *testing.T) {
	for _, bore := range []bool{false, true} {
		raw := rationalAnisotropicFixture(t)
		if bore {
			raw = rationalAnisotropicBoreFixture(t)
		}
		for _, edit := range []func([]RationalAnisotropicAssemblyInstance){
			func(p []RationalAnisotropicAssemblyInstance) { p[0].Origin[0] = nil },
			func(p []RationalAnisotropicAssemblyInstance) { p[0].Origin[0] = big.NewRat(1, 3) },
			func(p []RationalAnisotropicAssemblyInstance) {
				p[0].XAxis = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(0, 1)}
			},
			func(p []RationalAnisotropicAssemblyInstance) {
				p[0].XAxis = [3]*big.Rat{big.NewRat(5, 1), big.NewRat(12, 1), big.NewRat(0, 1)}
			},
			func(p []RationalAnisotropicAssemblyInstance) { p[0].XAxis[2] = big.NewRat(1, 1) },
			func(p []RationalAnisotropicAssemblyInstance) { p[0].ID = p[1].ID },
			func(p []RationalAnisotropicAssemblyInstance) { p[0].Child = "foreign" },
			func(p []RationalAnisotropicAssemblyInstance) {
				p[0].Origin[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 257))
			},
		} {
			a, b := rationalAnisotropicWriteModel(t, bore, raw)
			if bore {
				edit(b.Occurrences)
			} else {
				edit(a.Occurrences)
			}
			var out bytes.Buffer
			if e := writeRationalAnisotropicTest(context.Background(), &out, a, b, bore); e == nil || out.Len() != 0 {
				t.Fatal("accepted", e)
			}
		}
		a, b := rationalAnisotropicWriteModel(t, bore, raw)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var out bytes.Buffer
		if e := writeRationalAnisotropicTest(ctx, &out, a, b, bore); e == nil || out.Len() != 0 {
			t.Fatal("cancel", e)
		}
	}
}
func TestWriteRationalAnisotropicLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	for _, bore := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		dir := t.TempDir()
		src := filepath.Join(dir, "original.step")
		report := filepath.Join(dir, "report.json")
		mode := "filled-assembly-oblique"
		if bore {
			mode = "bore-assembly-oblique"
		}
		if reply, e := exec.CommandContext(ctx, bin, src, report, mode).CombinedOutput(); e != nil {
			t.Fatal(e, string(reply))
		}
		type mass struct {
			Volume, Area   float64
			Centroid       [3]float64
			Solids, Shells int
			Shared         bool
			Placements     [][3][4]float64
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
			a, b := rationalAnisotropicWriteModel(t, bore, raw)
			uses := a.Occurrences
			if bore {
				uses = b.Occurrences
			}
			uses[1].Origin[0].Add(uses[1].Origin[0], big.NewRat(shift, 1))
			var generated bytes.Buffer
			if e = writeRationalAnisotropicTest(ctx, &generated, a, b, bore); e != nil {
				t.Fatal(e)
			}
			dst := filepath.Join(dir, "generated.step")
			if e = os.WriteFile(dst, generated.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			if reply, e := exec.CommandContext(ctx, bin, dst, report, "read-assembly").CombinedOutput(); e != nil {
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
			if !q.Shared || q.Solids != 2 || q.Shells != 2 || len(q.Placements) != 2 || math.Abs(q.Volume-original.Volume) > 1e-6 || math.Abs(q.Area-original.Area) > 1e-7 {
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
			for i, p := range uses {
				pose, e := mappedRationalPose(RationalMappedPlacement{Origin: p.Origin, XAxis: p.XAxis, ZAxis: p.ZAxis})
				if e != nil {
					t.Fatal(e)
				}
				want := (ExactMappedOccurrence{pose: pose}).Matrix()
				for r := 0; r < 3; r++ {
					for c := 0; c < 4; c++ {
						v, _ := want[r][c].Float64()
						if math.Abs(q.Placements[i][r][c]-v) > 1e-14 {
							t.Fatal("actual XCAF pose", q.Placements[i], want)
						}
					}
				}
			}
		}
	}
}

func TestRationalAnisotropicUseCopies(t *testing.T) {
	p := RationalAnisotropicAssemblyInstance{ID: "a", Parent: "root", Child: "leaf", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(0, 1), big.NewRat(0, 1)}, XAxis: [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), big.NewRat(0, 1)}, ZAxis: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
	q := p
	q.ID = "b"
	_, poses, e := stageRationalAnisotropicUses(context.Background(), []RationalAnisotropicAssemblyInstance{p, q})
	if e != nil {
		t.Fatal(e)
	}
	p.Origin[0].SetInt64(99)
	p.XAxis[0].SetInt64(99)
	for _, id := range []string{"a", "b"} {
		if poses[id].Origin[0].Cmp(big.NewRat(1, 10)) != 0 || poses[id].X[0].Cmp(big.NewRat(3, 5)) != 0 {
			t.Fatal("borrowed input")
		}
	}
}
func FuzzRationalAnisotropicUseRatios(f *testing.F) {
	f.Add(int64(3), int64(4), int64(10))
	f.Add(int64(5), int64(12), int64(3))
	f.Fuzz(func(t *testing.T, x, y, d int64) {
		if d == 0 {
			return
		}
		p := RationalAnisotropicAssemblyInstance{ID: "a", Parent: "root", Child: "leaf", Origin: [3]*big.Rat{big.NewRat(1, d), big.NewRat(0, 1), big.NewRat(0, 1)}, XAxis: [3]*big.Rat{new(big.Rat).SetInt64(x), new(big.Rat).SetInt64(y), big.NewRat(0, 1)}, ZAxis: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
		q := p
		q.ID = "b"
		_, poses, e := stageRationalAnisotropicUses(context.Background(), []RationalAnisotropicAssemblyInstance{p, q})
		if e == nil {
			for _, b := range poses {
				if mappedRationalDot(b.X, b.X).Cmp(big.NewRat(1, 1)) != 0 || mappedRationalDot(b.X, b.Z).Sign() != 0 || rationalHierarchyFrameBound(b) != nil {
					t.Fatal("invalid admitted pose")
				}
			}
		}
	})
}

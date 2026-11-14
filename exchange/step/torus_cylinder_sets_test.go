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
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strconv"
	"strings"
	"testing"
	"time"
)

func torusCylinderSTEPBody(op geom.TorusCylinderOperation) TorusCylinderBody {
	return TorusCylinderBody{"torus cylinder", 5, 2, 6, -3, 4, op}
}
func TestTorusCylinderSTEP(t *testing.T) {
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		b := torusCylinderSTEPBody(op)
		proof, e := CertifyTorusCylinderExport(b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		if proof.HeightSquared.Cmp(big.NewRat(3, 1)) != 0 || proof.ErrorUpper.Sign() <= 0 || proof.ErrorUpper.Cmp(new(big.Rat).SetFloat64(1e-8)) > 0 {
			t.Fatal("residual proof")
		}
		var buf bytes.Buffer
		if e = WriteTorusCylinderSets(&buf, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(buf.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, ent := range f.Entities {
			for _, r := range ent.Records {
				counts[r.Type]++
			}
		}
		want := 2
		if op == geom.TorusCylinderUnion {
			want = 5
		}
		if counts["ADVANCED_FACE"] != want || counts["TOROIDAL_SURFACE"] != 1 || counts["CYLINDRICAL_SURFACE"] != 1 || counts["MANIFOLD_SOLID_BREP"] != 1 {
			t.Fatal(counts)
		}
		if strings.Contains(buf.String(), "B_SPLINE") || strings.Contains(buf.String(), "POLYLINE") {
			t.Fatal("sampled export")
		}
		var repeated bytes.Buffer
		if e = WriteTorusCylinderSets(&repeated, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e != nil || !bytes.Equal(buf.Bytes(), repeated.Bytes()) {
			t.Fatal("determinism", e)
		}
	}
	b := torusCylinderSTEPBody(geom.TorusCylinderCommon)
	if _, e := CertifyTorusCylinderExport(b, 1e-30); e == nil {
		t.Fatal("impossible numeric tolerance")
	}
	b.CylinderRadius = 7
	var out bytes.Buffer
	if e := WriteTorusCylinderSets(&out, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e == nil || out.Len() != 0 {
		t.Fatal("atomic invalid")
	}
}
func TestOCCTTorusCylinderSTEP(t *testing.T) {
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		b := torusCylinderSTEPBody(op)
		var buf bytes.Buffer
		if e := WriteTorusCylinderSets(&buf, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "shape.step")
		if e := os.WriteFile(path, buf.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "readback.json")
		stepOracle(t, out, path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var got struct {
			Area, Volume float64
			Centroid     [3]float64
		}
		if e = json.Unmarshal(data, &got); e != nil {
			t.Fatal(e)
		}
		source, e := geom.NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		if e != nil {
			t.Fatal(e)
		}
		v, e := source.BoundVolume(context.Background(), geom.TorusCylinderVolumeOptions{VolumeTolerance: big.NewRat(1, 100)})
		if e != nil {
			t.Fatal(e)
		}
		a, e := source.BoundArea(context.Background(), geom.TorusCylinderAreaOptions{AreaTolerance: big.NewRat(1, 1000000)})
		if e != nil {
			t.Fatal(e)
		}
		for _, x := range []struct {
			value float64
			l, h  *big.Rat
		}{{got.Volume, v.Lower, v.Upper}, {got.Area, a.Lower, a.Upper}} {
			lo, _ := x.l.Float64()
			hi, _ := x.h.Float64()
			if x.value < lo-1e-5 || x.value > hi+1e-5 {
				t.Fatal(op, "OCCT", got, lo, hi)
			}
		}
		wantFirst := 0.
		if op == geom.TorusCylinderUnion {
			wantFirst = 126 * math.Pi
		}
		if math.Abs(got.Centroid[2]*got.Volume-wantFirst) > 1e-5 {
			t.Fatal("centroid", got)
		}
		t.Log(op, got)
	}
}

func TestOCCTTorusCylinderSTEPOriginalBoolean(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_TORUS_CYLINDER_STEP")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_TORUS_CYLINDER_STEP to pinned reader")
	}
	for _, c := range []float64{4, 6} {
		for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
			for _, caps := range [][2]float64{{-3, 4}, {-4, 3}} {
				if caps[0] == -4 && op != geom.TorusCylinderUnion {
					continue
				}
				b := torusCylinderSTEPBody(op)
				b.CylinderRadius, b.Lower, b.Upper = c, caps[0], caps[1]
				var buf bytes.Buffer
				if e := WriteTorusCylinderSets(&buf, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e != nil {
					t.Fatal(e)
				}
				imported, err := ReadTorusCylinderSet(bytes.NewReader(buf.Bytes()), Limits{}, 1e-8)
				if err != nil {
					t.Fatal(err)
				}
				var reexport bytes.Buffer
				if err = imported.WriteSTEP(&reexport); err != nil || !bytes.Equal(buf.Bytes(), reexport.Bytes()) {
					t.Fatal("paired reexport", err)
				}
				native, err := imported.Solid().MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if err != nil {
					t.Fatal(err)
				}

				path := filepath.Join(t.TempDir(), "shape.step")
				if e := os.WriteFile(path, buf.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				output := filepath.Join(t.TempDir(), "read.json")
				args := []string{output, path}
				for _, v := range []float64{5, 2, c, caps[0], caps[1]} {
					args = append(args, strconv.FormatFloat(v, 'g', -1, 64))
				}
				args = append(args, strconv.Itoa(int(op)))
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				log, e := exec.CommandContext(ctx, oracle, args...).CombinedOutput()
				cancel()
				if e != nil {
					t.Fatalf("original Boolean %v: %v %s", b, e, log)
				}
				data, e := os.ReadFile(output)
				if e != nil {
					t.Fatal(e)
				}
				var got struct {
					Volume, Area, CentroidZ, ReferenceVolume, ReferenceArea, ReferenceCentroidZ, EdgeResidual float64
					Solids, Shells, Faces                                                                     int
				}
				if e = json.Unmarshal(data, &got); e != nil {
					t.Fatal(e)
				}
				wantFaces := 2
				if op == geom.TorusCylinderUnion {
					wantFaces = 5
				}
				if got.Solids != 1 || got.Shells != 1 || got.Faces != wantFaces || got.EdgeResidual > 1e-8 || math.Abs(got.Volume-got.ReferenceVolume) > 1e-5 || math.Abs(got.Area-got.ReferenceArea) > 1e-5 || math.Abs(got.CentroidZ-got.ReferenceCentroidZ) > 1e-7 {
					t.Fatal(b, got)
				}
				if math.Abs(native.Volume-got.ReferenceVolume) > 1e-5 || math.Abs(native.Area-got.ReferenceArea) > 1e-5 || math.Abs(native.Centroid.Z-got.ReferenceCentroidZ) > 1e-7 {
					t.Fatal("native reconstructed vs original OCCT", native, got)
				}

				t.Log(c, op, caps, got.EdgeResidual)
			}
		}
	}
}

func TestTorusCylinderSTEPExactDecimal(t *testing.T) {
	for _, x := range []float64{.1, -.1, 1.0 / 3, math.SmallestNonzeroFloat64, math.MaxFloat64, 2 * math.Pi} {
		v, ok := new(big.Rat).SetString(torusCylinderSTEPReal(x))
		if !ok || v.Cmp(new(big.Rat).SetFloat64(x)) != 0 {
			t.Fatal("decimal changed represented scalar", x)
		}
	}
}

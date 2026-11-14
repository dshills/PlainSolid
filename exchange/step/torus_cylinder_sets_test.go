package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
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

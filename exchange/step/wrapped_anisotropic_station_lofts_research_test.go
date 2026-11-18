//go:build research

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
	"testing"
	"time"
)

func TestWrappedAnisotropicStationSTEPLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional independent OCCT author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dir := t.TempDir()
	source, report := filepath.Join(dir, "original.step"), filepath.Join(dir, "original.json")
	if b, e := exec.CommandContext(ctx, bin, source, report, "filled").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
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
	bounds, e := m.Solid.BoundWrappedAnisotropicStationMass(ctx, trimmed.WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 1000)})
	if e != nil {
		t.Fatal(e)
	}
	if bounds.AxialVolumeFactor.Cmp(big.NewRat(43, 6)) != 0 || bounds.AxialFirstZFactor.Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal("independent station integral")
	}
	lo, _ := bounds.Volume.Lower.Float64()
	hi, _ := bounds.Volume.Upper.Float64()
	if original.Volume < lo-1e-10 || original.Volume > hi+1e-10 {
		t.Fatal("volume", original, bounds.Volume)
	}
	for i, c := range bounds.Centroid {
		lo, _ = c.Lower.Float64()
		hi, _ = c.Upper.Float64()
		if original.Centroid[i] < lo-1e-9 || original.Centroid[i] > hi+1e-9 {
			t.Fatal("centroid", i, original)
		}
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, e := WrappedAnisotropicStationLoftFromSolid(ctx, s, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, model := range []WrappedAnisotropicStationLoftBody{m, rebuilt} {
		var output bytes.Buffer
		if e = WriteWrappedAnisotropicStationLoft(&output, model); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, string(rune('a'+i))+".step")
		if e = os.WriteFile(p, output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		if b, e := exec.CommandContext(ctx, bin, p, report, "read").CombinedOutput(); e != nil {
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
			t.Fatal("OCCT", q, original)
		}
		for j := range q.Centroid {
			if math.Abs(q.Centroid[j]-original.Centroid[j]) > 1e-8 {
				t.Fatal("OCCT centroid", q, original)
			}
		}
	}
}

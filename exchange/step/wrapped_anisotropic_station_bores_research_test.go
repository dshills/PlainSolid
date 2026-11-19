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

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepairCircularTubeCentimeterSource(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-circular-tube-nauo-cm-scaled.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(raw), Limits{}, 389, CircularNAUORepairOptions{1e-10})
	if e != nil {
		t.Fatal(e)
	}
	if a.LengthUnit != .01 || a.Body.OuterRadius != 3 || a.Body.InnerRadius != 1 || a.Body.Height != 5 {
		t.Fatal("source coordinates/units changed", a.Body, a.LengthUnit)
	}
	if a.Body.OuterRadius*10 != 30 || a.Body.InnerRadius*10 != 10 || a.Body.Height*10 != 50 {
		t.Fatal("physical millimetre dimensions")
	}
	for _, w := range a.Walls {
		if w.ModelSeamMagnitude.RatString() != "1/10" || w.ModelSeamParameterEnd.RatString() != "50" || len(w.Source.FailedConditions) != 4 || w.Bounds.Tolerance.FloatString(12) != "0.000000000100" {
			t.Fatal("source evidence units")
		}
	}
	// The same small physical tube is a legitimate centimeter source, but its
	// 0.3 and 0.1 support radii cannot satisfy this native exact scalar gate.
	small, e := os.ReadFile("testdata/occt-circular-tube-nauo-cm.step")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := RepairCircularTubeNAUOProduct(bytes.NewReader(small), Limits{}, 389, CircularNAUORepairOptions{1e-10}); e == nil || got.SourceBodyID != 0 {
		t.Fatal("rounded centimeter support accepted")
	}
	for _, kind := range []string{"mixed-length-contexts", "wrong-angle", "small-tolerance", "negative-line-scale", "zero-line-scale", "shared-line-vector"} {
		t.Run(kind, func(t *testing.T) {
			input := string(raw)
			o := CircularNAUORepairOptions{1e-10}
			switch kind {
			case "mixed-length-contexts":
				input = strings.Replace(input, "SI_UNIT(.CENTI.,.METRE.)", "SI_UNIT(.MILLI.,.METRE.)", 1)
			case "wrong-angle":
				input = strings.ReplaceAll(input, "SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)")
			case "small-tolerance":
				o.MaxDistance = 1e-14
			case "negative-line-scale":
				input = strings.Replace(input, "#441 = VECTOR('',#442,0.1);", "#441 = VECTOR('',#442,-0.1);", 1)
			case "zero-line-scale":
				input = strings.Replace(input, "#441 = VECTOR('',#442,0.1);", "#441 = VECTOR('',#442,0.);", 1)
			case "shared-line-vector":
				input = strings.Replace(input, "#445 = LINE('',#446,#447);", "#445 = LINE('',#446,#441);", 1)
			}
			if got, e := RepairCircularTubeNAUOProduct(strings.NewReader(input), Limits{}, 389, o); e == nil || got.SourceBodyID != 0 || len(got.Solid.Faces()) != 0 {
				t.Fatal("unsupported units/repair accepted")
			}
		})
	}
}
func TestOCCTRepairCircularTubeCentimeterSource(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/occt-circular-tube-nauo-cm-scaled.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := RepairCircularTubeNAUOProduct(bytes.NewReader(raw), Limits{}, 389, CircularNAUORepairOptions{1e-10})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteTubes(&b, TubeModel{Bodies: []TubeBody{a.Body}, LengthUnit: a.LengthUnit}); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || m.LengthUnit != .01 {
		t.Fatal("new output unit", e)
	}
	path := filepath.Join(t.TempDir(), "reconstructed-centimeter-tube.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	run := func(path string) measure {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var m measure
		if e = json.Unmarshal(data[at:], &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	original, repaired := run("testdata/occt-circular-tube-nauo-cm-scaled.step"), run(path)
	// OCCT's reader returns millimetres: source geometry was authored R30/R10/H50.
	if original.Solids != 3 || original.Faces != 16 || repaired.Solids != 1 || repaired.Faces != 4 || math.Abs(original.Volume-(48+40000*math.Pi)) > 1e-6 || math.Abs(original.Area-(104+5600*math.Pi)) > 1e-6 || math.Abs(repaired.Volume-(original.Volume-48)) > 1e-6 || math.Abs(repaired.Area-(original.Area-104)) > 1e-6 || math.Abs(repaired.Center[2]-25) > 1e-8 {
		t.Fatal(original, repaired)
	}
}

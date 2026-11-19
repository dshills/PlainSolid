//go:build research

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

// This oracle builds original weighted supports in OCCT, never from Go output.
func TestIndependentWeightedStationSTEP(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_STATION_AUTHOR")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_WEIGHTED_STATION_AUTHOR")
	}
	for _, mode := range []string{"filled", "bore"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path, measure := dir+"/original.step", dir+"/original.json"
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if b, e := exec.CommandContext(ctx, oracle, path, measure, mode).CombinedOutput(); e != nil {
				t.Fatalf("independent author: %v\n%s", e, b)
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			mb, e := os.ReadFile(measure)
			if e != nil {
				t.Fatal(e)
			}
			var mass struct {
				Volume         float64
				Centroid       [3]float64
				Solids, Shells int
			}
			if e = json.Unmarshal(mb, &mass); e != nil {
				t.Fatal(e)
			}
			factor, cz := 9., 71./54
			var got, want []WrappedRationalStationLoftBody
			var canonical bytes.Buffer
			if mode == "filled" {
				m, e := ReadWrappedRationalStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				got = []WrappedRationalStationLoftBody{m}
				want = []WrappedRationalStationLoftBody{rationalStationSTEPFixture(t, 2)}
				if e = WriteWrappedRationalStationLoft(&canonical, m); e != nil {
					t.Fatal(e)
				}
			} else {
				m, e := ReadWrappedRationalStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				ref := rationalStationBoreSTEPFixture(t)
				got = []WrappedRationalStationLoftBody{m.Outer, m.Inner}
				want = []WrappedRationalStationLoftBody{ref.Outer, ref.Inner}
				factor, cz = 125./16, 383./300
				if e = WriteWrappedRationalStationBore(&canonical, m); e != nil {
					t.Fatal(e)
				}
			}
			var replay bytes.Buffer
			if mode == "filled" {
				reread, e := ReadWrappedRationalStationLoft(bytes.NewReader(canonical.Bytes()), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if e = WriteWrappedRationalStationLoft(&replay, reread); e != nil {
					t.Fatal(e)
				}
			} else {
				reread, e := ReadWrappedRationalStationBore(bytes.NewReader(canonical.Bytes()), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if e = WriteWrappedRationalStationBore(&replay, reread); e != nil {
					t.Fatal(e)
				}
			}
			if !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
				t.Fatal("canonical reexport unstable")
			}
			if os.Getenv("PLAINSOLID_OCCT_ORACLE") != "" && os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
				exported := dir + "/canonical.step"
				if e = os.WriteFile(exported, canonical.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				out := dir + "/canonical.json"
				stepOracle(t, out, "step-mass-precise", exported)
				b, e := os.ReadFile(out)
				if e != nil {
					t.Fatal(e)
				}
				var readback struct {
					Volume   float64
					Centroid [3]float64
				}
				if e = json.Unmarshal(b, &readback); e != nil {
					t.Fatal(e)
				}
				if math.Abs(readback.Volume-mass.Volume) > 1e-6 {
					t.Fatal("readback volume changed", readback, mass)
				}
				for i := range readback.Centroid {
					if math.Abs(readback.Centroid[i]-mass.Centroid[i]) > 1e-7 {
						t.Fatal("readback centroid changed", readback, mass)
					}
				}
			}
			for i := range got {
				if len(got[i].Walls) != len(want[i].Walls) {
					t.Fatal("original wall count changed")
				}
				for j := range got[i].Walls {
					a, _ := got[i].Walls[j].Geometry.ExportRecipe()
					b, _ := want[i].Walls[j].Geometry.ExportRecipe()
					if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
						t.Fatalf("original support %d/%d differs", i, j)
					}
				}
			}
			if mass.Solids != 1 || mass.Shells != 1 || math.Abs(mass.Volume-factor*rationalStationSTEPArea()) > 1e-6 || math.Abs(mass.Centroid[2]-cz) > 1e-7 {
				t.Fatal(mass)
			}
			t.Logf("independently authored %s volume %.12g", mode, mass.Volume)
		})
	}
}

package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/step"
	"reflect"
	"testing"
	"time"
)

func editedExactNAUODAG(t *testing.T) *ExactDAGPlanarNAUODocument {
	t.Helper()
	old := exactDAGNAUONative(t)
	p := step.ExactNAUOPlacement{NAUOID: old.Occurrences()[0].Path[0].NAUOID, Origin: [3]*big.Rat{big.NewRat(200, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, XAxis: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, ZAxis: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
	var before bytes.Buffer
	if e := old.Write(&before); e != nil {
		t.Fatal(e)
	}
	edited, e := old.WithRootPlacements([]step.ExactNAUOPlacement{p}, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var after bytes.Buffer
	if e = old.Write(&after); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) || !bytes.Equal(old.native, edited.native) {
		t.Fatal("old source/local assets mutated")
	}
	if !reflect.DeepEqual(old.Groups(), edited.Groups()) || !reflect.DeepEqual(old.Parts(), edited.Parts()) {
		t.Fatal("product identity changed")
	}
	var save bytes.Buffer
	if e = edited.Write(&save); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadExactDAGPlanarNAUODocument(&save, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(edited.Occurrences(), reloaded.Occurrences()) || !reflect.DeepEqual(edited.Relationships(), reloaded.Relationships()) {
		t.Fatal("edited paths changed on reload")
	}
	return reloaded
}
func TestExactNAUODAGRootEditNative(t *testing.T) {
	d := editedExactNAUODAG(t)
	bounds, e := d.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range []int64{212, 112, 72} {
		if bounds.Max[i].Cmp(big.NewRat(x, 1)) != 0 || bounds.Min[i].Cmp(big.NewRat([]int64{0, -5, 0}[i], 1)) != 0 {
			t.Fatal(bounds)
		}
	}
	bad := step.ExactNAUOPlacement{NAUOID: d.Occurrences()[0].Path[1].NAUOID}
	if n, e := d.WithRootPlacements([]step.ExactNAUOPlacement{bad}, step.Limits{}); e == nil || n != nil {
		t.Fatal("internal edit accepted")
	}
}
func TestOCCTExactNAUODAGRootEdit(t *testing.T) {
	boundsBin, massBin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS"), os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if boundsBin == "" || massBin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := editedExactNAUODAG(t)
	var output bytes.Buffer
	if e := d.WriteSTEP(&output, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "root-edit.step")
	if e := os.WriteFile(file, output.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	run := func(bin, marker string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, bin, file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		i := bytes.LastIndex(data, []byte(marker))
		if i < 0 {
			t.Fatal(string(data))
		}
		return data[i:]
	}
	var b struct{ Min, Max [3]float64 }
	if e := json.Unmarshal(run(boundsBin, `{"Min"`), &b); e != nil {
		t.Fatal(e)
	}
	for i, x := range []float64{212, 112, 72} {
		if math.Abs(b.Max[i]-x) > 1e-6 || math.Abs(b.Min[i]-[]float64{0, -5, 0}[i]) > 1e-6 {
			t.Fatal(b)
		}
	}
	var m struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	if e := json.Unmarshal(run(massBin, `{"Faces"`), &m); e != nil {
		t.Fatal(e)
	}
	if m.Faces != 42 || m.Solids != 7 || math.Abs(m.Volume-114) > 1e-9 || math.Abs(m.Area-274) > 1e-9 {
		t.Fatal(m)
	}
	for i, x := range []float64{3793.0 / 38, 1129.0 / 19, 1119.0 / 38} {
		if math.Abs(m.Center[i]-x) > 1e-9 {
			t.Fatal(m)
		}
	}
}

func TestOCCTExactNAUODAGRootRationalRotation(t *testing.T) {
	boundsBin, massBin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS"), os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if boundsBin == "" || massBin == "" {
		t.Skip("optional pinned OCCT")
	}
	old := exactDAGNAUONative(t)
	p := step.ExactNAUOPlacement{NAUOID: old.Occurrences()[0].Path[0].NAUOID, Origin: [3]*big.Rat{big.NewRat(200, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, XAxis: [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), big.NewRat(0, 1)}, ZAxis: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
	d, e := old.WithRootPlacements([]step.ExactNAUOPlacement{p}, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = ReadExactDAGPlanarNAUODocument(&native, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	if e = d.WriteSTEP(&output, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "rational-root-edit.step")
	if e = os.WriteFile(file, output.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, bin := range []string{boundsBin, massBin} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, e := exec.CommandContext(ctx, bin, file).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(data))
		}
		if bin == boundsBin {
			at := bytes.LastIndex(data, []byte(`{"Min"`))
			if at < 0 {
				t.Fatal(string(data))
			}
			var b struct{ Min, Max [3]float64 }
			if e = json.Unmarshal(data[at:], &b); e != nil {
				t.Fatal(e)
			}
			for i, x := range []float64{207.2, 112, 72} {
				if math.Abs(b.Max[i]-x) > 1e-6 || math.Abs(b.Min[i]-[]float64{0, -11, 0}[i]) > 1e-6 {
					t.Fatal(b)
				}
			}
		} else {
			at := bytes.LastIndex(data, []byte(`{"Faces"`))
			if at < 0 {
				t.Fatal(string(data))
			}
			var m struct {
				Faces, Solids int
				Volume, Area  float64
				Center        [3]float64
			}
			if e = json.Unmarshal(data[at:], &m); e != nil {
				t.Fatal(e)
			}
			if m.Faces != 42 || m.Solids != 7 || math.Abs(m.Volume-114) > 1e-9 || math.Abs(m.Area-274) > 1e-9 {
				t.Fatal(m)
			}
			for i, x := range []float64{95.3, 59.4, 1119.0 / 38} {
				if math.Abs(m.Center[i]-x) > 1e-9 {
					t.Fatal(m)
				}
			}
		}
	}
}

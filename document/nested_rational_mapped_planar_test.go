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

func nestedMappedNative(t *testing.T) *RationalMappedPlanarDocument {
	t.Helper()
	b, e := os.ReadFile("../exchange/step/testdata/occt-nested-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadNestedRationalMappedPlanarSTEP(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestNestedMappedNative(t *testing.T) {
	d := nestedMappedNative(t)
	if len(d.MapIDs()) != 2 || len(d.Occurrences()) != 6 || len(d.OccurrencePaths()) != 6 {
		t.Fatal(d)
	}
	paths := d.OccurrencePaths()
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadRationalMappedPlanarDocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("flat native widened")
	}
	n, e := ReadNestedRationalMappedPlanarDocument(&raw, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(paths, n.OccurrencePaths()) {
		t.Fatal("lost ancestry")
	}
	paths[0][0].ItemID = -1
	if reflect.DeepEqual(paths, n.OccurrencePaths()) {
		t.Fatal("mutable ancestry")
	}
	b, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range []int64{-10, -8, 0} {
		if b.Min[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	for i, v := range []int64{106, 22, 4} {
		if b.Max[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	if _, e = n.BakeOccurrence(n.Occurrences()[0].ItemID); e == nil {
		t.Fatal("ambiguous leaf selected")
	}
	if _, e = n.BakeOccurrenceIndex(1); e != nil {
		t.Fatal(e)
	}
	if _, e = n.BakeOccurrenceIndex(3); e == nil {
		t.Fatal("rounded nested point")
	}
	var source bytes.Buffer
	if e = n.WriteSTEP(&source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadNestedRationalMappedPlanarMaterials(&source, step.Limits{})
	if e != nil || len(a.Occurrences) != 6 {
		t.Fatal(e)
	}
}
func TestOCCTNestedMappedNative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := nestedMappedNative(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e := ReadNestedRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var source bytes.Buffer
	if e = d.WriteSTEP(&source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "nested.step")
	if e = os.WriteFile(p, source.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(data))
	}
	at := bytes.LastIndex(data, []byte(`{"Min"`))
	if at < 0 {
		t.Fatal(string(data))
	}
	var b struct{ Min, Max [3]float64 }
	if e = json.Unmarshal(data[at:], &b); e != nil {
		t.Fatal(e)
	}
	for i, v := range []float64{-10, -8, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{106, 22, 4} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}

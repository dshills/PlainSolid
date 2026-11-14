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

func dagMappedNative(t *testing.T) *RationalMappedPlanarDocument {
	t.Helper()
	b, e := os.ReadFile("../exchange/step/testdata/occt-dag-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadDAGRationalMappedPlanarSTEP(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestDAGMappedNative(t *testing.T) {
	d := dagMappedNative(t)
	paths := d.DAGOccurrencePaths()
	if len(paths) != 11 || len(d.MapIDs()) != 2 {
		t.Fatal("sharing")
	}
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadTwoLevelRationalMappedPlanarDocument(bytes.NewReader(saved.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("fixed format widened")
	}
	n, e := ReadDAGRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(paths, n.DAGOccurrencePaths()) {
		t.Fatal("lost paths")
	}
	paths[0][0].ItemID = -1
	if reflect.DeepEqual(paths, n.DAGOccurrencePaths()) {
		t.Fatal("mutable paths")
	}
	b, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range []int64{-10, -10, 0} {
		if b.Min[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	for i, v := range []int64{204, 22, 54} {
		if b.Max[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	if _, e = n.BakeOccurrence(n.Occurrences()[0].ItemID); e == nil {
		t.Fatal("ambiguous leaf")
	}
	if _, e = n.BakeOccurrenceIndex(1); e != nil {
		t.Fatal(e)
	}
	if _, e = n.WithOuterPlacements(nil, step.Limits{}); e == nil {
		t.Fatal("fixed edit widened")
	}
}
func TestOCCTDAGMappedNative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := dagMappedNative(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e := ReadDAGRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var source bytes.Buffer
	if e = d.WriteSTEP(&source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "dag.step")
	if e = os.WriteFile(p, source.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	at := bytes.LastIndex(raw, []byte(`{"Min"`))
	if at < 0 {
		t.Fatal(string(raw))
	}
	var b struct{ Min, Max [3]float64 }
	if e = json.Unmarshal(raw[at:], &b); e != nil {
		t.Fatal(e)
	}
	for i, v := range []float64{-10, -10, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{204, 22, 54} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}

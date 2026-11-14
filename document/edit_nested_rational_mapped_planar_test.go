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
	"testing"
	"time"
)

func editedNestedNative(t *testing.T) (*RationalMappedPlanarDocument, *RationalMappedPlanarDocument) {
	t.Helper()
	d := nestedMappedNative(t)
	paths := d.OccurrencePaths()
	ids := []int64{paths[0][0].ItemID, paths[3][0].ItemID}
	v := func(a, b, c int64) [3]*big.Rat {
		return [3]*big.Rat{big.NewRat(a, 1), big.NewRat(b, 1), big.NewRat(c, 1)}
	}
	ps := []step.RationalMappedPlacement{{ItemID: ids[0], Origin: v(0, 0, 5), XAxis: v(1, 0, 0), ZAxis: v(0, 0, 1)}, {ItemID: ids[1], Origin: v(200, 0, 0), XAxis: v(3, 4, 0), ZAxis: v(0, 0, 1)}}
	next, e := d.WithOuterPlacements(ps, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ps[1].Origin[0].SetInt64(999)
	return d, next
}
func TestEditedNestedMappedNative(t *testing.T) {
	old, d := editedNestedNative(t)
	b, e := old.ExactWorldBounds()
	if e != nil || b.Max[0].Cmp(big.NewRat(106, 1)) != 0 {
		t.Fatal("old document changed")
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e = ReadNestedRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = d.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range []int64{-10, -8, 0} {
		if b.Min[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	for i, v := range []int64{206, 22, 9} {
		if b.Max[i].Cmp(big.NewRat(v, 1)) != 0 {
			t.Fatal(b)
		}
	}
	if len(d.MapIDs()) != 2 || len(d.Occurrences()) != 6 {
		t.Fatal("sharing lost")
	}
}
func TestOCCTEditedNestedMappedNative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	_, d := editedNestedNative(t)
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
	p := filepath.Join(t.TempDir(), "edited.step")
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
	for i, v := range []float64{-10, -8, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{206, 22, 9} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}

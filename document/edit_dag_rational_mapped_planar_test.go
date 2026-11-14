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

func editedDAGNative(t *testing.T) (*RationalMappedPlanarDocument, *RationalMappedPlanarDocument) {
	t.Helper()
	old := dagMappedNative(t)
	paths := old.DAGOccurrencePaths()
	v := func(x, y, z int64) [3]*big.Rat {
		return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
	}
	root := step.RationalMappedPlacement{ItemID: paths[8][0].ItemID, Origin: v(0, 0, 100), XAxis: v(1, 0, 0), ZAxis: v(0, 0, 1)}
	d, e := old.WithDAGRootPlacements([]step.RationalMappedPlacement{root}, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	leaf := paths[0][len(paths[0])-1]
	shared := step.RationalMappedPlacement{ItemID: leaf.ItemID, Origin: v(30, 0, 0), XAxis: v(3, 4, 0), ZAxis: v(0, 0, 1)}
	d, e = d.WithDAGSharedPlacements([]step.RationalMappedPlacement{shared}, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	shared.Origin[0].SetInt64(999)
	return old, d
}
func TestEditedDAGMappedNative(t *testing.T) {
	old, d := editedDAGNative(t)
	b, e := old.ExactWorldBounds()
	if e != nil || b.Max[2].Cmp(big.NewRat(54, 1)) != 0 {
		t.Fatal("old changed")
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e = ReadDAGRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = d.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range []*big.Rat{big.NewRat(204, 1), big.NewRat(156, 5), big.NewRat(104, 1)} {
		if b.Max[i].Cmp(v) != 0 {
			t.Fatal(b)
		}
	}
	if len(d.MapIDs()) != 2 || len(d.Occurrences()) != 11 {
		t.Fatal("sharing")
	}
}
func TestOCCTEditedDAGMappedNative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	_, d := editedDAGNative(t)
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
	for i, v := range []float64{-10, -10, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{204, 31.2, 104} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}

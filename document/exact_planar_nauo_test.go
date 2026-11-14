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

func exactNAUONative(t *testing.T) *ExactPlanarNAUODocument {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/occt-exact-nauo-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadExactPlanarNAUOSTEP(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestExactNAUONative(t *testing.T) {
	d := exactNAUONative(t)
	uses := d.Occurrences()
	part := d.PartDefinition()
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	n, e := ReadExactPlanarNAUODocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if n.Name() != "Two-use NAUO assembly" || n.LengthUnit() != .001 || !reflect.DeepEqual(part, n.PartDefinition()) || !reflect.DeepEqual(uses, n.Occurrences()) {
		t.Fatal("identity lost")
	}
	if uses[0].ChildID != uses[1].ChildID || uses[0].RelationshipID == uses[1].RelationshipID || uses[0].TransformationID == uses[1].TransformationID {
		t.Fatal("owner binding")
	}
	b, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range []int64{12, 22, 4} {
		if b.Max[i].Cmp(big.NewRat(x, 1)) != 0 || b.Min[i].Sign() != 0 {
			t.Fatal(b)
		}
	}
	if _, e = ReadRationalMappedPlanarDocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("mapped format accepted NAUO")
	}
	var env rationalMappedEnvelope
	if e = json.Unmarshal(raw.Bytes(), &env); e != nil {
		t.Fatal(e)
	}
	env.Local = json.RawMessage(`{}`)
	bad, _ := json.Marshal(env)
	if n, e = ReadExactPlanarNAUODocument(bytes.NewReader(bad), 4<<20, step.Limits{}); e == nil || n != nil {
		t.Fatal("stale local assets")
	}
}
func TestOCCTExactNAUONative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := exactNAUONative(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e := ReadExactPlanarNAUODocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = d.WriteSTEP(&raw, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "nauo.step")
	if e = os.WriteFile(p, raw.Bytes(), 0600); e != nil {
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
	for i, x := range []float64{12, 22, 4} {
		if math.Abs(b.Min[i]) > 1e-6 || math.Abs(b.Max[i]-x) > 1e-6 {
			t.Fatal(b)
		}
	}
}

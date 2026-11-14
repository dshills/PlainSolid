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

func exactDAGNAUONative(t *testing.T) *ExactDAGPlanarNAUODocument {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/occt-exact-dag-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadExactDAGPlanarNAUOSTEP(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestExactDAGNAUONative(t *testing.T) {
	d := exactDAGNAUONative(t)
	uses := d.Occurrences()
	part := d.Parts()
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	n, e := ReadExactDAGPlanarNAUODocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if n.Name() != "DAG planar NAUO assembly" || n.LengthUnit() != .001 || !reflect.DeepEqual(part, n.Parts()) || !reflect.DeepEqual(uses, n.Occurrences()) || !reflect.DeepEqual(d.Groups(), n.Groups()) || !reflect.DeepEqual(d.Relationships(), n.Relationships()) {
		t.Fatal("identity lost")
	}
	if uses[0].Path[1].ChildID != uses[1].Path[1].ChildID || uses[0].Path[1].RelationshipID == uses[1].Path[1].RelationshipID || uses[0].Path[1].TransformationID == uses[1].Path[1].TransformationID {
		t.Fatal("owner binding")
	}
	group := n.Groups()
	group[0].Representations[0] = -1
	if n.Groups()[0].Representations[0] < 0 {
		t.Fatal("mutable group definition")
	}
	uses[0].Path[0].NAUOID = -1
	if n.Occurrences()[0].Path[0].NAUOID < 0 {
		t.Fatal("mutable path")
	}
	matrix := uses[0].Matrix()
	matrix[0][0].SetInt64(999)
	if n.Occurrences()[0].Matrix()[0][0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable exact pose")
	}
	if _, e = ReadExactPlanarNAUOProductsDocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("flat native schema widened")
	}
	b, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range []int64{112, 112, 72} {
		if b.Max[i].Cmp(big.NewRat(x, 1)) != 0 || b.Min[i].Cmp(big.NewRat([]int64{0, -5, 0}[i], 1)) != 0 {
			t.Fatal(b)
		}
	}
	if _, e = ReadRationalMappedPlanarDocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("mapped format accepted NAUO")
	}
	if _, e = ReadExactPlanarNAUODocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{}); e == nil {
		t.Fatal("old native schema widened")
	}
	var env rationalMappedEnvelope
	if e = json.Unmarshal(raw.Bytes(), &env); e != nil {
		t.Fatal(e)
	}
	env.Local = json.RawMessage(`{}`)
	bad, _ := json.Marshal(env)
	if n, e = ReadExactDAGPlanarNAUODocument(bytes.NewReader(bad), 4<<20, step.Limits{}); e == nil || n != nil {
		t.Fatal("stale local assets")
	}
}
func TestOCCTExactDAGNAUONative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := exactDAGNAUONative(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e := ReadExactDAGPlanarNAUODocument(&saved, 4<<20, step.Limits{})
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
	for i, x := range []float64{112, 112, 72} {
		if math.Abs(b.Min[i]-[]float64{0, -5, 0}[i]) > 1e-6 || math.Abs(b.Max[i]-x) > 1e-6 {
			t.Fatal(b)
		}
	}
}

func TestOCCTExactDAGNAUOMass(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := exactDAGNAUONative(t)
	var native, raw bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	n, e := ReadExactDAGPlanarNAUODocument(&native, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = n.WriteSTEP(&raw, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(t.TempDir(), "reloaded.step")
	if e = os.WriteFile(output, raw.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, file := range []string{"../exchange/step/testdata/occt-exact-dag-nauo.step", output} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, e := exec.CommandContext(ctx, bin, file).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(data))
		}
		var got struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(data[at:], &got); e != nil {
			t.Fatal(e)
		}
		if got.Faces != 42 || got.Solids != 7 || math.Abs(got.Volume-114) > 1e-9 || math.Abs(got.Area-274) > 1e-9 {
			t.Fatal(got)
		}
		for i, x := range []float64{1993.0 / 38, 1129.0 / 19, 1119.0 / 38} {
			if math.Abs(got.Center[i]-x) > 1e-9 {
				t.Fatal(got)
			}
		}
	}
}

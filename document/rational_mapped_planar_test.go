package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func rationalMappedNativeFixture(t *testing.T) *RationalMappedPlanarDocument {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/occt-rational-mapped-occurrence.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadRationalMappedPlanarSTEP(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestRationalMappedNativeRoundtrip(t *testing.T) {
	d := rationalMappedNativeFixture(t)
	if len(d.MapIDs()) != 2 || len(d.Occurrences()) != 3 || d.LengthUnit() != .001 {
		t.Fatal(d)
	}
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	n, e := ReadRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range []*big.Rat{big.NewRat(-10, 1), new(big.Rat), new(big.Rat)} {
		if b.Min[i].Cmp(q) != 0 {
			t.Fatal(b)
		}
	}
	for i, q := range []*big.Rat{big.NewRat(56, 5), big.NewRat(22, 1), big.NewRat(4, 1)} {
		if b.Max[i].Cmp(q) != 0 {
			t.Fatal(b)
		}
	}
	b.Max[0].SetInt64(999)
	again, e := n.ExactWorldBounds()
	if e != nil || again.Max[0].Cmp(big.NewRat(56, 5)) != 0 {
		t.Fatal("mutable bounds")
	}
	uses := n.Occurrences()
	if _, e = n.BakeOccurrence(uses[0].ItemID); e == nil {
		t.Fatal("baked rounded points")
	}
	placed, e := n.BakeOccurrence(uses[1].ItemID)
	if e != nil || placed.Name != uses[1].Name {
		t.Fatal(e)
	}
	if len(placed.Solid.Vertices()) != 8 {
		t.Fatal(placed)
	}
	var source bytes.Buffer
	if e = n.WriteSTEP(&source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadRationalMappedPlanarMaterials(&source, step.Limits{})
	if e != nil || len(a.Occurrences) != 3 {
		t.Fatal(e)
	}
	if _, e = n.BakeOccurrence(-1); e == nil {
		t.Fatal("foreign item")
	}
}
func TestRationalMappedNativeTamper(t *testing.T) {
	d := rationalMappedNativeFixture(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"version", "format", "local", "source", "unknown", "limit", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			var env map[string]json.RawMessage
			if e := json.Unmarshal(saved.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "version":
				env["version"] = json.RawMessage("2")
			case "format":
				env["format"] = json.RawMessage(`"other"`)
			case "local":
				s := State{LengthUnit: .001, Planar: map[string]brep.Solid{}}
				b, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 9, Y: 9, Z: 9})
				s.Planar["forged"] = b
				local, e := New(s, 0)
				if e != nil {
					t.Fatal(e)
				}
				var raw bytes.Buffer
				if e = local.Write(&raw); e != nil {
					t.Fatal(e)
				}
				env["local"] = raw.Bytes()
			case "source":
				env["source"] = json.RawMessage(`"invalid"`)
			case "unknown":
				env["mystery"] = json.RawMessage("true")
			}
			raw, e := json.Marshal(env)
			if e != nil {
				t.Fatal(e)
			}
			limit := int64(4 << 20)
			if kind == "limit" {
				limit = 100
			}
			if kind == "trailing" {
				raw = append(raw, []byte("{}")...)
			}
			n, e := ReadRationalMappedPlanarDocument(bytes.NewReader(raw), limit, step.Limits{})
			if e == nil || n != nil {
				t.Fatal("accepted forged envelope", e)
			}
		})
	}
}

func rationalNativePlacements(t *testing.T, source []byte) (*step.File, []step.RationalMappedPlacement) {
	t.Helper()
	a, e := step.ReadRationalMappedPlanarMaterials(bytes.NewReader(source), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ps := []step.RationalMappedPlacement{}
	for _, u := range a.Occurrences {
		m := u.Matrix()
		ps = append(ps, step.RationalMappedPlacement{ItemID: u.ItemID, Origin: [3]*big.Rat{m[0][3], m[1][3], m[2][3]}, XAxis: [3]*big.Rat{m[0][0], m[1][0], m[2][0]}, ZAxis: [3]*big.Rat{m[0][2], m[1][2], m[2][2]}})
	}
	return a.Source, ps
}
func TestRationalMappedNativePreciseAndBake(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/occt-rational-mapped-occurrence.step")
	if e != nil {
		t.Fatal(e)
	}
	source, poses := rationalNativePlacements(t, raw)
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(40), nil))
	poses[0].Origin[0].Add(poses[0].Origin[0], epsilon)
	var out bytes.Buffer
	if e = step.WriteRationalMappedPlanarPlacements(&out, source, poses, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	d, e := ReadRationalMappedPlanarSTEP(&out, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e = ReadRationalMappedPlanarDocument(&saved, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := d.ExactWorldBounds()
	want := new(big.Rat).Add(big.NewRat(56, 5), epsilon)
	if e != nil || b.Max[0].Cmp(want) != 0 {
		t.Fatal("tiny component lost", b, e)
	}
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 10, Y: 15, Z: 20})
	small, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	model := step.NativeMappedPlanarModel{Name: "bake", LengthUnit: .001, Products: []step.Body{{Name: "large", Solid: a}, {Name: "small", Solid: small}}, Uses: []step.NativeMappedPlanarUse{{Name: "first", Product: 0, Transform: geom.Identity()}, {Name: "second", Product: 0, Transform: geom.Translation(geom.Vec3{X: 100})}, {Name: "third", Product: 1, Transform: geom.Identity()}}}
	var base bytes.Buffer
	if e = step.WriteNativeMappedPlanarMaterials(&base, model, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	source, poses = rationalNativePlacements(t, base.Bytes())
	poses[0].XAxis = [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), new(big.Rat)}
	out.Reset()
	if e = step.WriteRationalMappedPlanarPlacements(&out, source, poses, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	d, e = ReadRationalMappedPlanarSTEP(&out, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	body, e := d.BakeOccurrence(d.Occurrences()[0].ItemID)
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(body.Solid.Tessellate().SignedVolume()-3000) > 1e-8 {
		t.Fatal("baked volume")
	}
	if len(d.MapIDs()) != 2 || d.Occurrences()[0].MapID != d.Occurrences()[1].MapID {
		t.Fatal("sharing lost")
	}
}
func TestOCCTRationalMappedNativeBounds(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d := rationalMappedNativeFixture(t)
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := ReadRationalMappedPlanarDocument(&native, 4<<20, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = d.WriteSTEP(&raw, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "reload.step")
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
	for i, v := range []float64{-10, 0, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{11.2, 22, 4} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}

func TestRationalMappedNativeFormatIsolation(t *testing.T) {
	d := rationalMappedNativeFixture(t)
	if e := d.Write(shortWriter{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	if _, e := Read(bytes.NewReader(raw.Bytes()), 4<<20, 0); e == nil {
		t.Fatal("generic floating document accepted rational envelope")
	}
	if n, e := ReadRationalMappedPlanarDocument(bytes.NewReader(raw.Bytes()), 4<<20, step.Limits{MaxEntities: 10}); e == nil || n != nil {
		t.Fatal("inner STEP budget ignored")
	}
	if n, e := ReadRationalMappedPlanarSTEP(nil, step.Limits{}); e == nil || n != nil {
		t.Fatal("nil source")
	}
}

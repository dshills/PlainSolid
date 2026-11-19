package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	rigidplacement "plainsolid/brep/placement"
	"testing"
)

func TestTranslatedCylinderTorusCavityObliqueOccurrence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var raw bytes.Buffer
	WriteTranslatedCylinderTorusCavity(&raw, TranslatedCylinderTorusCavityBody{Name: "placed", MajorRadius: 5, MinorRadius: 2, CylinderRadius: 10, Lower: 0, Upper: 7, OffsetX: 1, OffsetY: -1, OffsetZ: 3}, .001)
	source, e := ReadTranslatedCylinderTorusCavity(&raw, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	m := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	pose, e := rigidplacement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := source.PlaceSTEP(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	placed.WriteSTEP(&out)
	got, e := ReadTranslatedCylinderTorusCavityOccurrence(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Original().Source() != source.Source() {
		t.Fatal("source")
	}
	actual, _ := got.Pose()
	for i := range m {
		for j := range m[i] {
			if m[i][j].Cmp(actual[i][j]) != 0 {
				t.Fatal("pose")
			}
		}
	}
	var again bytes.Buffer
	got.WriteSTEP(&again)
	if !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal("determinism")
	}
	if _, e = ReadTranslatedCylinderTorusCavity(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("canonical accepted world")
	}
	m[0][3] = r(1, 7)
	bad, _ := rigidplacement.NewRationalRigid(m, 4096)
	if _, e = source.PlaceSTEP(ctx, bad); e == nil {
		t.Fatal("nondecimal")
	}
	f, _ := Parse(bytes.NewReader(out.Bytes()), Limits{})
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "CARTESIAN_POINT" && len(en.Records[0].Args[1].Items) == 3 {
			en.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "999."}
			f.Entities[id] = en
			break
		}
	}
	if _, e = ReadTranslatedCylinderTorusCavityOccurrence(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("altered world")
	}
	for _, duplicate := range []bool{false, true} {
		f, _ := Parse(bytes.NewReader(out.Bytes()), Limits{})
		datum, e := cavitySourceFrame(f, false, false)
		if e != nil {
			t.Fatal(e)
		}
		if duplicate {
			for id, en := range f.Entities {
				if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_REPRESENTATION" {
					en.Records[0].Args[1].Items = append(en.Records[0].Args[1].Items, Value{Kind: Reference, Ref: datum})
					f.Entities[id] = en
					break
				}
			}
		} else {
			cavitySourceFrame(f, false, true)
		}
		if _, e := ReadTranslatedCylinderTorusCavityOccurrence(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("datum missing/duplicate", duplicate)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_TRANSLATED_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "placed.step")
		os.WriteFile(path, out.Bytes(), 0600)
		if log, e := exec.Command(oracle, path, "oblique").CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

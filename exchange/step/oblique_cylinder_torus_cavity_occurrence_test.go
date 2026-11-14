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

func TestObliqueCylinderTorusCavityObliqueOccurrence(t *testing.T) {
	ctx := context.Background()
	var raw bytes.Buffer
	WriteObliqueCylinderTorusCavity(&raw, ObliqueCylinderTorusCavityBody{Name: "placed", MajorRadius: 3, MinorRadius: 1, CylinderRadius: 8, Lower: -6, Upper: 6, Frame: [3][4]*big.Rat{{big.NewRat(3, 5), big.NewRat(0, 1), big.NewRat(4, 5), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(-4, 5), big.NewRat(0, 1), big.NewRat(3, 5), big.NewRat(0, 1)}}}, .001)
	source, e := ReadObliqueCylinderTorusCavity(&raw, Limits{})
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
	got, e := ReadObliqueCylinderTorusCavityOccurrence(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Original().Source().CylinderRadius != source.Source().CylinderRadius {
		t.Fatal("source")
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			if got.Original().Source().Frame[i][j].Cmp(source.Source().Frame[i][j]) != 0 {
				t.Fatal("local rational frame")
			}
		}
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
	if _, e = ReadObliqueCylinderTorusCavity(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
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
	if _, e = ReadObliqueCylinderTorusCavityOccurrence(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
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
		if _, e := ReadObliqueCylinderTorusCavityOccurrence(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("datum missing/duplicate", duplicate)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "placed.step")
		os.WriteFile(path, out.Bytes(), 0600)
		if log, e := exec.Command(oracle, path, "oblique").CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

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

func TestEccentricCylinderTorusCavityObliqueOccurrence(t *testing.T) {
	ctx := context.Background()
	var raw bytes.Buffer
	WriteEccentricCylinderTorusCavity(&raw, EccentricCylinderTorusCavityBody{Name: "placed", MajorRadius: 5, MinorRadius: 2, CylinderRadius: 10, Lower: -3, Upper: 4, OffsetX: 1, OffsetY: -1}, .001)
	source, e := ReadEccentricCylinderTorusCavity(&raw, Limits{})
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
	got, e := ReadEccentricCylinderTorusCavityOccurrence(ctx, bytes.NewReader(out.Bytes()), Limits{})
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
	if _, e = ReadEccentricCylinderTorusCavity(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
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
	if _, e = ReadEccentricCylinderTorusCavityOccurrence(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("altered world")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "placed.step")
		os.WriteFile(path, out.Bytes(), 0600)
		if log, e := exec.Command(oracle, path, "oblique").CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

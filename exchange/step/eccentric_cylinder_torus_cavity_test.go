package step

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEccentricCylinderTorusCavitySTEP(t *testing.T) {
	t.Parallel()
	b := EccentricCylinderTorusCavityBody{Name: "eccentric", MajorRadius: 5, MinorRadius: 2, CylinderRadius: 10, Lower: -3, Upper: 4, OffsetX: 1, OffsetY: -1}
	var raw bytes.Buffer
	if e := WriteEccentricCylinderTorusCavity(&raw, b, .001); e != nil {
		t.Fatal(e)
	}
	got, e := ReadEccentricCylinderTorusCavity(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil || got.Source() != b {
		t.Fatal(got.Source(), e)
	}
	var round bytes.Buffer
	got.WriteSTEP(&round)
	if !bytes.Equal(round.Bytes(), raw.Bytes()) {
		t.Fatal("canonical")
	}
	for _, x := range []float64{3, 4} {
		bad := b
		bad.OffsetX = x
		bad.OffsetY = 0
		var dst bytes.Buffer
		if e := WriteEccentricCylinderTorusCavity(&dst, bad, .001); e == nil || dst.Len() != 0 {
			t.Fatal("touch/overlap")
		}
	}
	for _, typ := range []string{"TOROIDAL_SURFACE", "ORIENTED_CLOSED_SHELL", "CARTESIAN_POINT", "PCURVE"} {
		f, _ := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != typ {
				continue
			}
			r := en.Records[0]
			switch typ {
			case "TOROIDAL_SURFACE":
				r.Args[2] = Value{Kind: Number, Text: "6."}
			case "ORIENTED_CLOSED_SHELL":
				r.Args[3].Text = "T"
			case "CARTESIAN_POINT":
				r.Args[1].Items[0] = Value{Kind: Number, Text: "99."}
			case "PCURVE":
				r.Args[1] = Value{Kind: Reference, Ref: id}
			}
			en.Records[0] = r
			f.Entities[id] = en
			break
		}
		if _, e = ReadEccentricCylinderTorusCavity(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("altered", typ)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "cavity.step")
		os.WriteFile(path, round.Bytes(), 0600)
		if log, e := exec.Command(oracle, path).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
	b.OffsetX = .5
	b.OffsetY = -.25
	raw.Reset()
	if e = WriteEccentricCylinderTorusCavity(&raw, b, .001); e != nil {
		t.Fatal(e)
	}
	if got, e = ReadEccentricCylinderTorusCavity(&raw, Limits{}); e != nil || got.Source() != b {
		t.Fatal("dyadic", e)
	}
}

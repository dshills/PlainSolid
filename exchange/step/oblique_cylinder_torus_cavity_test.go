package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestObliqueCylinderTorusCavitySTEP(t *testing.T) {
	t.Parallel()
	r := big.NewRat
	b := ObliqueCylinderTorusCavityBody{Name: "oblique", MajorRadius: 3, MinorRadius: 1, CylinderRadius: 8, Lower: -6, Upper: 6, Frame: [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}}
	var raw bytes.Buffer
	if e := WriteObliqueCylinderTorusCavity(&raw, b, .001); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadObliqueCylinderTorusCavity(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	out := loaded.Source()
	if out.Frame[0][0].Cmp(r(3, 5)) != 0 || out.Lower != -6 || out.CylinderRadius != 8 {
		t.Fatal("source")
	}
	out.Frame[0][0].SetInt64(99)
	var round bytes.Buffer
	if e = loaded.WriteSTEP(&round); e != nil || !bytes.Equal(round.Bytes(), raw.Bytes()) {
		t.Fatal("immutable canonical", e)
	}
	l, m, e := loaded.Query().ClassifyRational(context.Background(), [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}, geom.SurfacePatchOptions{})
	if e != nil || l != geom.BoreBoundary || m != geom.TorusCylinderTorus {
		t.Fatal(l, m, e)
	}
	for _, typ := range []string{"DIRECTION", "PCURVE", "TOROIDAL_SURFACE", "ORIENTED_CLOSED_SHELL"} {
		f, _ := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != typ {
				continue
			}
			rec := en.Records[0]
			switch typ {
			case "DIRECTION":
				rec.Args[1].Items[0] = Value{Kind: Number, Text: "0.123"}
			case "PCURVE":
				rec.Args[1] = Value{Kind: Reference, Ref: id}
			case "TOROIDAL_SURFACE":
				rec.Args[2] = Value{Kind: Number, Text: "3.5"}
			case "ORIENTED_CLOSED_SHELL":
				rec.Args[3].Text = "T"
			}
			en.Records[0] = rec
			f.Entities[id] = en
			break
		}
		if _, e = ReadObliqueCylinderTorusCavity(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("mutation", typ)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "cavity.step")
		os.WriteFile(path, round.Bytes(), 0600)
		if log, e := exec.Command(oracle, path).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
	b.Frame[0][3] = r(1, 10)
	b.Frame[1][3] = r(1, 5)
	b.Frame[2][3] = r(1, 8)
	b.Lower = -5.5
	b.Upper = 6.25
	var finite bytes.Buffer
	if e := WriteObliqueCylinderTorusCavity(&finite, b, .001); e != nil {
		t.Fatal("finite source", e)
	}
	recovered, e := ReadObliqueCylinderTorusCavity(&finite, Limits{})
	if e != nil || recovered.Source().Frame[0][3].Cmp(r(1, 10)) != 0 || recovered.Source().Upper != 6.25 {
		t.Fatal("finite recovery", e)
	}
	b.Frame[0][3] = r(1, 7)
	var bad bytes.Buffer
	if e := WriteObliqueCylinderTorusCavity(&bad, b, .001); e == nil || bad.Len() != 0 {
		t.Fatal("nondecimal atomic")
	}
}
